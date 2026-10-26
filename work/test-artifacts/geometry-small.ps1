$ErrorActionPreference = 'Stop'
$out = 'D:\harry\youchui\Shuttle\work\test-artifacts'
$log = Join-Path $env:TEMP 'geo-small.log'
Remove-Item $log -ErrorAction SilentlyContinue
function Note($t) { $t | Out-File $log -Append -Encoding UTF8 }

Add-Type -AssemblyName UIAutomationClient,System.Drawing,System.Windows.Forms
Add-Type @'
using System; using System.Runtime.InteropServices;
public class SmallWin {
  [DllImport("user32.dll")] public static extern bool SetProcessDPIAware();
  [DllImport("user32.dll")] public static extern bool MoveWindow(IntPtr h, int x, int y, int w, int hgt, bool repaint);
  [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr h, out RECT r);
  [DllImport("user32.dll")] public static extern bool PrintWindow(IntPtr hwnd, IntPtr hdc, uint flags);
  [DllImport("user32.dll")] public static extern uint GetDpiForWindow(IntPtr hwnd);
  [DllImport("user32.dll")] public static extern bool SetCursorPos(int x, int y);
  [DllImport("user32.dll")] public static extern void mouse_event(uint f, uint x, uint y, uint d, UIntPtr e);
  [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr h);
  [StructLayout(LayoutKind.Sequential)] public struct RECT { public int Left, Top, Right, Bottom; }
}
'@
[SmallWin]::SetProcessDPIAware() | Out-Null

$exe = 'D:\harry\youchui\Shuttle\build\bin\Shuttle.exe'
$root = Join-Path ([IO.Path]::GetTempPath()) ('shuttle-small-' + [guid]::NewGuid())
[IO.Directory]::CreateDirectory($root) | Out-Null
[IO.File]::WriteAllText((Join-Path $root 'settings.json'), '{"theme":"system"}')
$env:SHUTTLE_CONFIG_TEST_ROOT = $root
$process = Start-Process $exe -PassThru
Remove-Item Env:SHUTTLE_CONFIG_TEST_ROOT

function Window {
  $c = [System.Windows.Automation.PropertyCondition]::new([System.Windows.Automation.AutomationElement]::ProcessIdProperty, $process.Id)
  return [System.Windows.Automation.AutomationElement]::RootElement.FindFirst([System.Windows.Automation.TreeScope]::Children, $c)
}
function Capture($handle, $name) {
  $r = New-Object SmallWin+RECT
  [SmallWin]::GetWindowRect($handle, [ref]$r) | Out-Null
  $w = $r.Right - $r.Left; $hgt = $r.Bottom - $r.Top
  $bitmap = [System.Drawing.Bitmap]::new($w, $hgt)
  $g = [System.Drawing.Graphics]::FromImage($bitmap)
  $hdc = $g.GetHdc()
  # PW_RENDERFULLCONTENT captures the WebView2 surface even off-screen
  [SmallWin]::PrintWindow($handle, $hdc, 2) | Out-Null
  $g.ReleaseHdc($hdc); $g.Dispose()
  $bitmap.Save((Join-Path $out $name)); $bitmap.Dispose()
  return "${w}x${hgt}"
}
function Check($win, $label) {
  $all = $win.FindAll([System.Windows.Automation.TreeScope]::Descendants, [System.Windows.Automation.Condition]::TrueCondition)
  $rows = foreach ($c in $all) {
    $r = $c.Current.BoundingRectangle
    if ($r.Width -lt 1 -or $r.Height -lt 1) { continue }
    [pscustomobject]@{ name = $c.Current.Name; x = $r.X; y = $r.Y; w = $r.Width; h = $r.Height }
  }
  $rows = @($rows)
  $wr = $win.Current.BoundingRectangle
  $outside = @($rows | Where-Object {
    # vertical overflow inside a scroll container is normal; the geometry line
    # only forbids unexpected horizontal overflow
    $beyond = ($_.x -lt $wr.X - 0.5) -or (($_.x + $_.w) -gt ($wr.X + $wr.Width) + 0.5)
    $isContentPane = ($_.h -gt $wr.Height * 0.9) -and ($_.w -gt $wr.Width * 0.9)
    $_.w -gt 0 -and $beyond -and (-not $isContentPane) })
  [pscustomobject]@{ label = $label; elements = $all.Count; measurable = $rows.Count
    outside = $outside.Count; outsideNames = @($outside | ForEach-Object { $_.name }) }
  $rows | ConvertTo-Json -Depth 3 | Set-Content (Join-Path $out "uia-$label.json") -Encoding UTF8
}

try {
  for ($i = 0; $i -lt 40; $i++) { if (Window) { break }; Start-Sleep -Milliseconds 250 }
  $win = Window
  if (-not $win) { throw 'window not found' }
  $handle = $win.Current.NativeWindowHandle
  $dpi = [SmallWin]::GetDpiForWindow($handle)
  $scale = $dpi / 96
  [SmallWin]::SetForegroundWindow($handle) | Out-Null
  Note "dpi=$dpi scale=$scale"

  $report = @()
  foreach ($dip in @(
      @{'w'=1080;'h'=720;'tag'='1080x720-100'},
      @{'w'=864;'h'=576;'tag'='1080x720-125'},
      @{'w'=720;'h'=480;'tag'='1080x720-150'})) {
    [SmallWin]::MoveWindow($handle, 0, 0, [int]($dip.w * $scale), [int]($dip.h * $scale), $true) | Out-Null
    Start-Sleep -Milliseconds 2800
    $win = Window
    $actual = Capture $handle "window-$($dip.tag).png"
    $report += Check $win $dip.tag
    Note "$($dip.tag): physical=$actual"
  }

  # open the config panel at the smallest size, the densest surface
  $win = Window
  [SmallWin]::MoveWindow($handle, 0, 0, [int](720 * $scale), [int](480 * $scale), $true) | Out-Null
  Start-Sleep -Milliseconds 2000
  $win = Window
  $name = [string]::Concat([char]0x914D, [char]0x7F6E, [char]0x4E0E, [char]0x8FD8, [char]0x539F)
  $button = $win.FindFirst([System.Windows.Automation.TreeScope]::Descendants,
    [System.Windows.Automation.PropertyCondition]::new([System.Windows.Automation.AutomationElement]::NameProperty, $name))
  if (-not $button) { throw 'config button not found' }
  $br = $button.Current.BoundingRectangle
  $win.SetFocus()
  [SmallWin]::SetCursorPos([int]($br.X + $br.Width / 2), [int]($br.Y + $br.Height / 2)) | Out-Null
  [SmallWin]::mouse_event(2,0,0,0,[UIntPtr]::Zero)
  [SmallWin]::mouse_event(4,0,0,0,[UIntPtr]::Zero)
  Start-Sleep -Milliseconds 1500
  $actual = Capture $handle 'window-720x480-config.png'
  $report += Check (Window) '720x480-config'
  Note "720x480-config: physical=$actual"

  $report | ConvertTo-Json -Depth 4 | Set-Content (Join-Path $out 'uia-small-report.json') -Encoding UTF8
  $report | ForEach-Object { Note "$($_.label): elements=$($_.elements) measurable=$($_.measurable) horizontalOverflow=$($_.outside) [$($_.outsideNames -join ',')]" }
} catch {
  Note "ERROR: $($_.Exception.Message)"
} finally {
  $process.Refresh()
  if (-not $process.HasExited) { $process.CloseMainWindow() | Out-Null; if (-not $process.WaitForExit(8000)) { $process.Kill() } }
}
Get-Content $log -Encoding UTF8
