# Verify inventory removal preview on real running Shuttle.exe
$ErrorActionPreference = 'Stop'
$out = $PSScriptRoot
$project = Split-Path (Split-Path $out -Parent) -Parent
$exe = Join-Path $project 'build\bin\Shuttle.exe'
$log = Join-Path $out 'verify-removal.log'
$process = $null

function Note($t) { $t | Out-File $log -Append -Encoding UTF8; Write-Host $t }

Add-Type -AssemblyName UIAutomationClient,UIAutomationTypes,System.Drawing,System.Windows.Forms
Add-Type @'
using System; using System.Runtime.InteropServices;
public class VRemWin {
  [DllImport("user32.dll")] public static extern bool SetProcessDPIAware();
  [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr h, out RECT r);
  [DllImport("user32.dll")] public static extern bool PrintWindow(IntPtr h, IntPtr dc, uint flags);
  [DllImport("user32.dll")] public static extern bool SetCursorPos(int x, int y);
  [DllImport("user32.dll")] public static extern void mouse_event(uint f, uint x, uint y, int d, UIntPtr e);
  [StructLayout(LayoutKind.Sequential)] public struct RECT { public int Left, Top, Right, Bottom; }
}
'@
[VRemWin]::SetProcessDPIAware() | Out-Null

function Window {
  $condition = [System.Windows.Automation.PropertyCondition]::new(
    [System.Windows.Automation.AutomationElement]::ProcessIdProperty, $process.Id)
  $wins = @()
  try { $wins = @([System.Windows.Automation.AutomationElement]::RootElement.FindAll(
    [System.Windows.Automation.TreeScope]::Children, $condition)) } catch { return $null }
  foreach ($w in $wins) { try { if ($w.Current.Name -like 'Shuttle*' -and $w.Current.BoundingRectangle.Width -gt 500) { return $w } } catch {} }
  return $null
}

function FindByName($root, $name) {
  if (-not $root) { return $null }
  return $root.FindFirst([System.Windows.Automation.TreeScope]::Descendants,
    [System.Windows.Automation.PropertyCondition]::new(
      [System.Windows.Automation.AutomationElement]::NameProperty, $name))
}

function ClickElement($el) {
  try {
    $inv = $el.GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern)
    if ($inv) { $inv.Invoke() }
  } catch {}
  try {
    $win = Window
    if ($win) { $win.SetFocus() }
    $el.SetFocus()
    [System.Windows.Forms.SendKeys]::SendWait('{ENTER}')
  } catch {}
  try {
    $r = $el.Current.BoundingRectangle
    [VRemWin]::SetCursorPos([int]($r.X + $r.Width/2), [int]($r.Y + $r.Height/2)) | Out-Null
    Start-Sleep -Milliseconds 100
    [VRemWin]::mouse_event(2,0,0,0,[UIntPtr]::Zero)
    Start-Sleep -Milliseconds 100
    [VRemWin]::mouse_event(4,0,0,0,[UIntPtr]::Zero)
  } catch {}
  Start-Sleep -Milliseconds 800
}

try {
  Set-Content $log 'removal verify start' -Encoding UTF8
  $testRoot = Join-Path $env:TEMP ('shuttle-verify-rem-' + [Guid]::NewGuid().ToString('N'))
  New-Item -ItemType Directory -Path $testRoot -Force | Out-Null
  $wv2Folder = Join-Path $testRoot 'wv2'
  New-Item -ItemType Directory -Path $wv2Folder -Force | Out-Null

  # Seed an isolated install-ledger.json with managed package 'defuddle'
  $ledgerJson = @'
{
  "schemaVersion": 2,
  "records": [
    {
      "name": "defuddle",
      "source": "npm",
      "root": "C:\\Users\\admin\\AppData\\Roaming\\npm\\node_modules",
      "version": "0.18.1",
      "state": "installed"
    }
  ]
}
'@
  Set-Content -Path (Join-Path $testRoot 'install-ledger.json') -Value $ledgerJson -Encoding UTF8
  Note "Isolated ledger seeded with managed defuddle in $testRoot"

  [Environment]::SetEnvironmentVariable('SHUTTLE_CONFIG_TEST_ROOT', $testRoot, 'Process')
  [Environment]::SetEnvironmentVariable('WEBVIEW2_USER_DATA_FOLDER', $wv2Folder, 'Process')
  [Environment]::SetEnvironmentVariable('WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS', '--force-renderer-accessibility', 'Process')
  
  $process = Start-Process -FilePath $exe -WorkingDirectory $project -PassThru
  Remove-Item Env:WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS
  Remove-Item Env:WEBVIEW2_USER_DATA_FOLDER
  Remove-Item Env:SHUTTLE_CONFIG_TEST_ROOT

  $win = $null
  for ($i = 0; $i -lt 60 -and -not $win; $i++) { Start-Sleep -Milliseconds 500; $win = Window }
  if (-not $win) { throw 'window not found' }
  $handle = [IntPtr]$win.Current.NativeWindowHandle
  Start-Sleep -Seconds 2

  function CaptureShot($name) {
    $rect = New-Object VRemWin+RECT
    if (-not [VRemWin]::GetWindowRect($handle, [ref]$rect)) { throw 'GetWindowRect failed' }
    $bitmap = [System.Drawing.Bitmap]::new($rect.Right - $rect.Left, $rect.Bottom - $rect.Top)
    $graphics = [System.Drawing.Graphics]::FromImage($bitmap)
    $dc = $graphics.GetHdc()
    if (-not [VRemWin]::PrintWindow($handle, $dc, 2)) { throw 'PrintWindow failed' }
    $graphics.ReleaseHdc($dc); $graphics.Dispose()
    $shotPath = Join-Path $out $name
    $bitmap.Save($shotPath, [System.Drawing.Imaging.ImageFormat]::Png)
    $bitmap.Dispose()
    Note "screenshot saved $name physical=$($rect.Right - $rect.Left)x$($rect.Bottom - $rect.Top)"
  }

  # 1. Click '安装归属' button in header with retry
  $opened = $false
  for ($attempt = 0; $attempt -lt 5 -and -not $opened; $attempt++) {
    $invBtn = FindByName $win '安装归属'
    if ($invBtn) {
      ClickElement $invBtn
      for ($j = 0; $j -lt 10; $j++) {
        Start-Sleep -Milliseconds 500
        $redetectBtn = FindByName $win '重新检测'
        if ($redetectBtn) { $opened = $true; break }
      }
    }
  }
  if (-not $opened) { throw '重新检测 button not found (dialog did not open after retries)' }
  Note "Dialog opened, waiting for detection ready..."

  # Wait until detection completes (重新检测 button is enabled)
  for ($i = 0; $i -lt 40; $i++) {
    Start-Sleep -Milliseconds 500
    if ($redetectBtn.Current.IsEnabled) { break }
  }

  # 3. Find defuddle checkbox
  $allNodes = $win.FindAll([System.Windows.Automation.TreeScope]::Descendants, [System.Windows.Automation.Condition]::TrueCondition)
  $cb = $null
  foreach ($node in $allNodes) {
    if ($node.Current.Name -like '*defuddle*' -and $node.Current.ControlType -eq [System.Windows.Automation.ControlType]::CheckBox) {
      $cb = $node
      break
    }
  }
  if (-not $cb) {
    $cb = FindByName $win '选择 defuddle'
  }
  if (-not $cb) {
    $names = @()
    foreach ($n in $allNodes) { if ($n.Current.Name) { $names += $n.Current.Name } }
    $joined = $names -join ' | '
    Note "Nodes in tree: $joined"
    throw 'defuddle checkbox not found'
  }
  Note "Found defuddle checkbox, IsEnabled=$($cb.Current.IsEnabled)"
  $parent = [System.Windows.Automation.TreeWalker]::ControlViewWalker.GetParent($cb)
  if ($parent) { foreach ($pn in $parent.FindAll([System.Windows.Automation.TreeScope]::Descendants, [System.Windows.Automation.Condition]::TrueCondition)) { Note "ItemText: '$($pn.Current.Name)'" } }
  if (-not $cb.Current.IsEnabled) { throw 'defuddle checkbox is disabled (should be managed)' }

  # 4. Scroll into view and toggle checkbox
  try { $win.SetFocus() } catch {}
  try { $cb.SetFocus() } catch {}
  Start-Sleep -Milliseconds 300
  # Try Toggle pattern first
  try {
    $togglePattern = $cb.GetCurrentPattern([System.Windows.Automation.TogglePattern]::Pattern)
    if ($togglePattern) {
      Note "Invoking TogglePattern on defuddle checkbox"
      $togglePattern.Toggle()
    }
  } catch { Note "TogglePattern not supported: $($_.Exception.Message)" }

  # Also send Space key and physical click to ensure React onChange fires
  [System.Windows.Forms.SendKeys]::SendWait(' ')
  Start-Sleep -Milliseconds 400
  ClickElement $cb
  Start-Sleep -Seconds 1

# 5. Find and click '生成移除计划' button
  $planBtn = FindByName $win '生成移除计划'
  if (-not $planBtn) { throw '生成移除计划 button not found' }
  if (-not $planBtn.Current.IsEnabled) { throw '生成移除计划 button is not enabled after selection' }
  Note "Clicking 生成移除计划..."
  try {
    $invPattern = $planBtn.GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern)
    if ($invPattern) {
      Note "Invoking InvokePattern on planBtn"
      $invPattern.Invoke()
    }
  } catch { Note "InvokePattern failed: $($_.Exception.Message)" }
  try { $planBtn.SetFocus(); [System.Windows.Forms.SendKeys]::SendWait('{ENTER}') } catch {}
  ClickElement $planBtn
  Start-Sleep -Seconds 2

  # 6. Verify removal preview card (plan takes 3-8s to re-detect inventory)
  $previewTitle = $null
  for ($i = 0; $i -lt 60 -and -not $previewTitle; $i++) {
    Start-Sleep -Milliseconds 500
    $previewTitle = FindByName $win '卸载预览'
  }
  if (-not $previewTitle) {
    # Print all names in tree to see what is shown
    $nodesNow = $win.FindAll([System.Windows.Automation.TreeScope]::Descendants, [System.Windows.Automation.Condition]::TrueCondition)
    $textList = @()
    foreach ($tn in $nodesNow) { if ($tn.Current.Name) { $textList += $tn.Current.Name } }
    Note "Tree after plan: $($textList -join ' | ')"
    throw '卸载预览 heading not found after planning removal'
  }
  Note "Found 卸载预览 heading!"

  # 7. Check for uninstall command and confirm button
  $confirmBtn = FindByName $win '确认并执行卸载'
  if (-not $confirmBtn) { throw '确认并执行卸载 button not found in preview' }
  Note "Found 确认并执行卸载 button, IsEnabled=$($confirmBtn.Current.IsEnabled)"
  if (-not $confirmBtn.Current.IsEnabled) { throw '确认并执行卸载 button should be enabled' }

  # Scroll preview into view so the screenshot captures the preview card clearly
  try {
    $confirmBtn.SetFocus()
    Start-Sleep -Milliseconds 600
  } catch {}

  CaptureShot 'verify-removal-preview.png'
  Note "PASS: verify-removal completed successfully; execution not invoked on real machine."

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
    Note "process exited: $($process.HasExited)"
    $process.Dispose()
  }
}
