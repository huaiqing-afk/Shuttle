# Verify terminal palette switcher on real running Shuttle.exe
$ErrorActionPreference = 'Stop'
$out = $PSScriptRoot
$project = Split-Path (Split-Path $out -Parent) -Parent
$exe = Join-Path $project 'build\bin\Shuttle.exe'
$log = Join-Path $out 'verify-palette.log'
$process = $null

function Note($t) { $t | Out-File $log -Append -Encoding UTF8; Write-Host $t }

Add-Type -AssemblyName UIAutomationClient,UIAutomationTypes,System.Drawing,System.Windows.Forms
Add-Type @'
using System; using System.Runtime.InteropServices;
public class VWin {
  [DllImport("user32.dll")] public static extern bool SetProcessDPIAware();
  [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr h, out RECT r);
  [DllImport("user32.dll")] public static extern bool PrintWindow(IntPtr h, IntPtr dc, uint flags);
  [DllImport("user32.dll")] public static extern bool SetCursorPos(int x, int y);
  [DllImport("user32.dll")] public static extern void mouse_event(uint f, uint x, uint y, int d, UIntPtr e);
  [StructLayout(LayoutKind.Sequential)] public struct RECT { public int Left, Top, Right, Bottom; }
}
'@
[VWin]::SetProcessDPIAware() | Out-Null

function Window {
  $condition = [System.Windows.Automation.PropertyCondition]::new(
    [System.Windows.Automation.AutomationElement]::ProcessIdProperty, $process.Id)
  $wins = @()
  try { $wins = @([System.Windows.Automation.AutomationElement]::RootElement.FindAll(
    [System.Windows.Automation.TreeScope]::Children, $condition)) } catch { return $null }
  foreach ($w in $wins) { try { if ($w.Current.Name -like 'Shuttle*') { return $w } } catch {} }
  return $null
}

try {
  Set-Content $log 'palette verify start' -Encoding UTF8
  $testRoot = Join-Path $env:TEMP ('shuttle-verify-palette-' + [Guid]::NewGuid().ToString('N'))
  New-Item -ItemType Directory -Path $testRoot -Force | Out-Null
  [Environment]::SetEnvironmentVariable('SHUTTLE_CONFIG_TEST_ROOT', $testRoot, 'Process')
  [Environment]::SetEnvironmentVariable('WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS', '--force-renderer-accessibility', 'Process')
  
  $process = Start-Process -FilePath $exe -WorkingDirectory $project -PassThru
  Remove-Item Env:WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS
  Remove-Item Env:SHUTTLE_CONFIG_TEST_ROOT

  $win = $null
  for ($i = 0; $i -lt 60 -and -not $win; $i++) { Start-Sleep -Milliseconds 500; $win = Window }
  if (-not $win) { throw 'window not found' }
  $handle = [IntPtr]$win.Current.NativeWindowHandle
  Start-Sleep -Seconds 2

  # Open a terminal with Ctrl+Shift+T
  $win.SetFocus()
  [System.Windows.Forms.SendKeys]::SendWait('^+t')
  Start-Sleep -Seconds 3

  # Paste ANSI colors probe into terminal
  Set-Clipboard -Value 'Write-Host Palette-Test: -NoNewline; Write-Host " Red" -ForegroundColor Red -NoNewline; Write-Host " Green" -ForegroundColor Green -NoNewline; Write-Host " Cyan" -ForegroundColor Cyan; Write-Host Tokyo-Night-or-Custom-Palette'
  [System.Windows.Forms.SendKeys]::SendWait('^v')
  Start-Sleep -Milliseconds 500
  [System.Windows.Forms.SendKeys]::SendWait('{ENTER}')
  Start-Sleep -Seconds 2

  function CaptureShot($name) {
    $rect = New-Object VWin+RECT
    if (-not [VWin]::GetWindowRect($handle, [ref]$rect)) { throw 'GetWindowRect failed' }
    $bitmap = [System.Drawing.Bitmap]::new($rect.Right - $rect.Left, $rect.Bottom - $rect.Top)
    $graphics = [System.Drawing.Graphics]::FromImage($bitmap)
    $dc = $graphics.GetHdc()
    if (-not [VWin]::PrintWindow($handle, $dc, 2)) { throw 'PrintWindow failed' }
    $graphics.ReleaseHdc($dc); $graphics.Dispose()
    $shotPath = Join-Path $out $name
    $bitmap.Save($shotPath, [System.Drawing.Imaging.ImageFormat]::Png)
    $bitmap.Dispose()
    Note "screenshot saved $name physical=$($rect.Right - $rect.Left)x$($rect.Bottom - $rect.Top)"
  }

  CaptureShot 'verify-palette-default.png'

  # Select Dracula from ComboBox
  $combo = $win.FindFirst([System.Windows.Automation.TreeScope]::Descendants,
    [System.Windows.Automation.PropertyCondition]::new([System.Windows.Automation.AutomationElement]::AutomationIdProperty, 'terminal-palette'))
  if (-not $combo) { throw 'terminal-palette ComboBox not found' }

  $combo.SetFocus()
  Start-Sleep -Milliseconds 400
  [System.Windows.Forms.SendKeys]::SendWait('{DOWN}{DOWN}{DOWN}{ENTER}')
  Start-Sleep -Seconds 1
  $valPattern = $combo.GetCurrentPattern([System.Windows.Automation.ValuePattern]::Pattern)
  Note "Selected: '$($valPattern.Current.Value)'"
  CaptureShot 'verify-palette-dracula.png'

  # Select Solarized Light (down 5 more times)
  $combo.SetFocus()
  Start-Sleep -Milliseconds 400
  [System.Windows.Forms.SendKeys]::SendWait('{DOWN}{DOWN}{DOWN}{DOWN}{DOWN}{ENTER}')
  Start-Sleep -Seconds 1
  Note "Selected: '$($valPattern.Current.Value)'"
  CaptureShot 'verify-palette-light.png'

  Note 'VERIFY DONE'
} catch {
  Note "ERROR: $($_.Exception.ToString())"
  throw
} finally {
  if ($process) {
    $process.Refresh()
    if (-not $process.HasExited) {
      $process.CloseMainWindow() | Out-Null
      if (-not $process.WaitForExit(8000)) { try { $process.Kill() } catch {}; $process.WaitForExit(8000) | Out-Null }
    }
    Note "exited=$($process.HasExited)"
    $process.Dispose()
  }
}
