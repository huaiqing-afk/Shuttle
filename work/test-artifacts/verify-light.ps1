# Light-theme terminal readability check: switch to light, open a PowerShell
# tab, print the ANSI grays that previously washed out, capture one screenshot.
$ErrorActionPreference = 'Stop'
$out = $PSScriptRoot
$project = Split-Path (Split-Path $out -Parent) -Parent
$exe = Join-Path $project 'build\bin\Shuttle.exe'
$log = Join-Path $out 'verify-light.log'
$process = $null
function Note($t) { $t | Out-File $log -Append -Encoding UTF8; Write-Host $t }
function U($p) { return -join @($p | ForEach-Object { [char]$_ }) }
$lightName = U @(0x6D45,0x8272)  # 浅色
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
function ClickName($name) {
  $win = Window
  if (-not $win) { throw 'window missing' }
  $el = $win.FindFirst([System.Windows.Automation.TreeScope]::Descendants,
    [System.Windows.Automation.PropertyCondition]::new(
      [System.Windows.Automation.AutomationElement]::NameProperty, $name))
  if (-not $el) { throw "not found: $name" }
  try { $win.SetFocus() } catch {}
  try { $el.SetFocus() } catch {}
  Start-Sleep -Milliseconds 400
  $r = $el.Current.BoundingRectangle
  [VWin]::SetCursorPos([int]($r.X + $r.Width/2), [int]($r.Y + $r.Height/2)) | Out-Null
  [VWin]::mouse_event(2,0,0,0,[UIntPtr]::Zero)
  [VWin]::mouse_event(4,0,0,0,[UIntPtr]::Zero)
  Start-Sleep -Milliseconds 600
}
try {
  Set-Content $log 'light theme verify start' -Encoding UTF8
  [Environment]::SetEnvironmentVariable('WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS', '--force-renderer-accessibility', 'Process')
  $process = Start-Process -FilePath $exe -WorkingDirectory $project -PassThru
  Remove-Item Env:WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS
  $win = $null
  for ($i = 0; $i -lt 60 -and -not $win; $i++) { Start-Sleep -Milliseconds 500; $win = Window }
  if (-not $win) { throw 'window not found' }
  $handle = [IntPtr]$win.Current.NativeWindowHandle
  Start-Sleep -Seconds 2
  ClickName $lightName
  $win.SetFocus()
  [System.Windows.Forms.SendKeys]::SendWait('^+t')
  Start-Sleep -Seconds 3
  # type the ANSI gray probes into the terminal
  # paste via clipboard: SendKeys keystrokes get eaten by the Sogou IME
  Set-Clipboard -Value 'Write-Host DarkGray-dim-text -ForegroundColor DarkGray; Write-Host Black-gray-text -ForegroundColor Black; Write-Host Normal-text; Write-Host Blue-text -ForegroundColor Blue'
  [System.Windows.Forms.SendKeys]::SendWait('^v')
  Start-Sleep -Milliseconds 500
  [System.Windows.Forms.SendKeys]::SendWait('{ENTER}')
  Start-Sleep -Seconds 2
  $rect = New-Object VWin+RECT
  if (-not [VWin]::GetWindowRect($handle, [ref]$rect)) { throw 'GetWindowRect failed' }
  $bitmap = [System.Drawing.Bitmap]::new($rect.Right - $rect.Left, $rect.Bottom - $rect.Top)
  $graphics = [System.Drawing.Graphics]::FromImage($bitmap)
  $dc = $graphics.GetHdc()
  if (-not [VWin]::PrintWindow($handle, $dc, 2)) { throw 'PrintWindow failed' }
  $graphics.ReleaseHdc($dc); $graphics.Dispose()
  $bitmap.Save((Join-Path $out 'verify-light-terminal.png'), [System.Drawing.Imaging.ImageFormat]::Png)
  $bitmap.Dispose()
  Note "screenshot saved physical=$($rect.Right - $rect.Left)x$($rect.Bottom - $rect.Top)"
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
