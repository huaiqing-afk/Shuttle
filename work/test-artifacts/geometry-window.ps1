$ErrorActionPreference = 'Stop'
$out = 'D:\harry\youchui\Shuttle\work\test-artifacts'
$log = Join-Path $env:TEMP 'geo-window.log'
Remove-Item $log -ErrorAction SilentlyContinue
function Note($text) { $text | Out-File $log -Append -Encoding UTF8 }

Add-Type -AssemblyName UIAutomationClient,UIAutomationTypes,System.Drawing,System.Windows.Forms
Add-Type @'
using System; using System.Runtime.InteropServices;
public class GeoWin3 {
  [DllImport("user32.dll")] public static extern bool SetProcessDPIAware();
  [DllImport("user32.dll")] public static extern bool MoveWindow(IntPtr h, int x, int y, int w, int hgt, bool repaint);
  [DllImport("user32.dll")] public static extern bool SetCursorPos(int x, int y);
  [DllImport("user32.dll")] public static extern void mouse_event(uint f, uint x, uint y, uint d, UIntPtr e);
  [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr h);
}
'@
[GeoWin3]::SetProcessDPIAware() | Out-Null

$root = Join-Path ([IO.Path]::GetTempPath()) ('shuttle-geo-' + [guid]::NewGuid())
[IO.Directory]::CreateDirectory($root) | Out-Null
[IO.File]::WriteAllText((Join-Path $root 'settings.json'), '{"theme":"system"}')
$env:SHUTTLE_CONFIG_TEST_ROOT = $root
$process = Start-Process 'D:\harry\youchui\Shuttle\build\bin\Shuttle.exe' -PassThru
Remove-Item Env:SHUTTLE_CONFIG_TEST_ROOT
Note "launched $($process.Id)"

function Window {
  $condition = [System.Windows.Automation.PropertyCondition]::new([System.Windows.Automation.AutomationElement]::ProcessIdProperty, $process.Id)
  return [System.Windows.Automation.AutomationElement]::RootElement.FindFirst([System.Windows.Automation.TreeScope]::Children, $condition)
}
function Controls($win) {
  $all = $win.FindAll([System.Windows.Automation.TreeScope]::Descendants, [System.Windows.Automation.Condition]::TrueCondition)
  $rows = foreach ($c in $all) {
    $r = $c.Current.BoundingRectangle
    if ($r.Width -lt 1 -or $r.Height -lt 1) { continue }
    $name = $c.Current.Name
    if ([string]::IsNullOrWhiteSpace($name)) { $name = $c.Current.ControlType.ProgrammaticName }
    [pscustomobject]@{ name = $name; x = [math]::Round($r.X,1); y = [math]::Round($r.Y,1)
      w = [math]::Round($r.Width,1); h = [math]::Round($r.Height,1) }
  }
  return [pscustomobject]@{ all = $all.Count; rows = @($rows) }
}
function Shot($win, $name) {
  $r = $win.Current.BoundingRectangle
  $bitmap = [System.Drawing.Bitmap]::new([int]$r.Width, [int]$r.Height)
  $graphics = [System.Drawing.Graphics]::FromImage($bitmap)
  $graphics.CopyFromScreen([int]$r.X, [int]$r.Y, 0, 0, $bitmap.Size)
  $graphics.Dispose()
  $bitmap.Save((Join-Path $out $name))
  $bitmap.Dispose()
  Note "shot $name $([int]$r.Width)x$([int]$r.Height)"
}
function Report($win, $label) {
  $c = Controls $win
  $wr = $win.Current.BoundingRectangle
  # the WebView2 child pane lags a resize; judge against the union of top-level
  # and content bounds so a stale child does not read as a clipped control
  $clipped = @($c.rows | Where-Object {
    $_.w -gt 0 -and ($_.x -lt $wr.X - 0.5 -or ($_.x + $_.w) -gt ($wr.X + $wr.Width) + 0.5) -and
    -not ($_.h -gt $wr.Height * 0.9) })  # skip the content pane itself
  $dupes = @($c.rows | Group-Object name | Where-Object {
    $_.Count -gt 1 -and $_.Group[0].w -gt 20 -and $_.Group[0].h -gt 20 })
  [pscustomobject]@{ label = $label; window = [pscustomobject]@{ w=[math]::Round($wr.Width,1); h=[math]::Round($wr.Height,1) }
    elements = $c.all; measurable = $c.rows.Count; clipped = $clipped.Count; duplicateNames = $dupes.Count
    clippedNames = @($clipped | ForEach-Object { $_.name }) }
  $c.rows | ConvertTo-Json -Depth 3 | Set-Content (Join-Path $out "uia-$label.json") -Encoding UTF8
}

try {
  for ($i = 0; $i -lt 40; $i++) { if (Window) { break }; Start-Sleep -Milliseconds 250 }
  $win = Window
  if (-not $win) { throw 'window not found' }
  [GeoWin3]::SetForegroundWindow($win.Current.NativeWindowHandle) | Out-Null
  Start-Sleep -Milliseconds 500
  Note "window handle $($win.Current.NativeWindowHandle) rect $($win.Current.BoundingRectangle)"

  $report = @()
  # the app's minimum is 960x600 DIP; Windows sizes are physical pixels, so
  # convert from DIP with this monitor's scale factor
  $scale = [System.Windows.Forms.Screen]::PrimaryScreen.Bounds.Width / 1536.0
  Note "monitor scale factor $scale"
  $dips = @(@{w=960;h=600}, @{w=1080;h=720}, @{w=1440;h=900})
  foreach ($dip in $dips) {
    $h = $win.Current.NativeWindowHandle
    $ok = [GeoWin3]::MoveWindow($h, 0, 0, [int]($dip.w * $scale), [int]($dip.h * $scale), $true)
    # WebView2 resizes its child window asynchronously; measure after it settles
    Start-Sleep -Milliseconds 2600
    $win = Window
    $label = "$($dip.w)x$($dip.h)dip"
    $actual = $win.Current.BoundingRectangle
    Note "move $label ok=$ok physical=$([int]$actual.Width)x$([int]$actual.Height)"
    Shot $win "window-$label.png"
    $report += Report $win $label
  }

  $win = Window
  [GeoWin3]::MoveWindow($win.Current.NativeWindowHandle, 0, 0, [int](1440 * $scale), [int](900 * $scale), $true) | Out-Null
  Start-Sleep -Milliseconds 900
  $win = Window
  $name = [string]::Concat([char]0x914D, [char]0x7F6E, [char]0x4E0E, [char]0x8FD8, [char]0x539F)
  $button = $win.FindFirst([System.Windows.Automation.TreeScope]::Descendants,
    [System.Windows.Automation.PropertyCondition]::new([System.Windows.Automation.AutomationElement]::NameProperty, $name))
  if (-not $button) { throw 'config button not found' }
  $r = $button.Current.BoundingRectangle
  [GeoWin3]::SetCursorPos([int]($r.X + $r.Width / 2), [int]($r.Y + $r.Height / 2)) | Out-Null
  [GeoWin3]::mouse_event(2,0,0,0,[UIntPtr]::Zero)
  [GeoWin3]::mouse_event(4,0,0,0,[UIntPtr]::Zero)
  Start-Sleep -Milliseconds 900
  $win = Window
  Shot $win 'window-config-1440x900.png'
  $report += Report $win 'config-1440x900'

  $report | ConvertTo-Json -Depth 4 | Set-Content (Join-Path $out 'uia-window-report.json') -Encoding UTF8
  $report | ForEach-Object { Note "$($_.label): window=$($_.window.w)x$($_.window.h) elements=$($_.elements) measurable=$($_.measurable) clipped=$($_.clipped) [$($_.clippedNames -join ',')] dupNames=$($_.duplicateNames)" }
} catch {
  Note "ERROR: $($_.Exception.GetType().Name): $($_.Exception.Message)"
  throw
} finally {
  $process.Refresh()
  if (-not $process.HasExited) { $process.CloseMainWindow() | Out-Null; if (-not $process.WaitForExit(8000)) { $process.Kill() } }
  Note 'closed'
}
