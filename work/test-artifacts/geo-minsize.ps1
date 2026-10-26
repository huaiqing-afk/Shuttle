$ErrorActionPreference = 'Stop'
$log = Join-Path $env:TEMP 'geo-minsize.log'
Remove-Item $log -ErrorAction SilentlyContinue
function Note($t) { $t | Out-File $log -Append -Encoding UTF8 }
Add-Type @'
using System; using System.Runtime.InteropServices;
public class MinProbe {
  [DllImport("user32.dll")] public static extern bool SetProcessDPIAware();
  [DllImport("user32.dll")] public static extern bool MoveWindow(IntPtr h, int x, int y, int w, int hgt, bool repaint);
  [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr h, out RECT r);
  [DllImport("user32.dll")] public static extern uint GetDpiForWindow(IntPtr hwnd);
  [StructLayout(LayoutKind.Sequential)] public struct RECT { public int Left, Top, Right, Bottom; }
}
'@
Add-Type -AssemblyName UIAutomationClient
[MinProbe]::SetProcessDPIAware() | Out-Null

$exe = 'D:\harry\youchui\Shuttle\build\bin\Shuttle.exe'
$root = Join-Path ([IO.Path]::GetTempPath()) ('shuttle-min-' + [guid]::NewGuid())
[IO.Directory]::CreateDirectory($root) | Out-Null
[IO.File]::WriteAllText((Join-Path $root 'settings.json'), '{"theme":"system"}')
$env:SHUTTLE_CONFIG_TEST_ROOT = $root
$process = Start-Process $exe -PassThru
Remove-Item Env:SHUTTLE_CONFIG_TEST_ROOT
try {
  Start-Sleep -Seconds 5
  $cond = [System.Windows.Automation.PropertyCondition]::new([System.Windows.Automation.AutomationElement]::ProcessIdProperty, $process.Id)
  $win = [System.Windows.Automation.AutomationElement]::RootElement.FindFirst([System.Windows.Automation.TreeScope]::Children, $cond)
  $handle = $win.Current.NativeWindowHandle
  $dpi = [MinProbe]::GetDpiForWindow($handle)
  $scale = $dpi / 96
  Note "dpi=$dpi scale=$scale"
  # targets are the DIP sizes a 1080x720 screen presents at 100/125/150%
  foreach ($target in @(
      @{'dipw'=1080;'diph'=720;'tag'='1080x720@100'},
      @{'dipw'=864;'diph'=576;'tag'='1080x720@125'},
      @{'dipw'=720;'diph'=480;'tag'='1080x720@150'})) {
    [MinProbe]::MoveWindow($handle, 0, 0, [int]($target.dipw * $scale), [int]($target.diph * $scale), $true) | Out-Null
    Start-Sleep -Milliseconds 1500
    $r = New-Object MinProbe+RECT
    [MinProbe]::GetWindowRect($handle, [ref]$r) | Out-Null
    $gotW = [int](($r.Right - $r.Left) / $scale)
    $gotH = [int](($r.Bottom - $r.Top) / $scale)
    Note "$($target.tag): asked $($target.dipw)x$($target.diph) DIP, got ${gotW}x${gotH} DIP $(if ($gotW -ne $target.dipw -or $gotH -ne $target.diph) { '<-- CLAMPED' } else { 'ok' })"
  }
} finally {
  $process.Refresh()
  if (-not $process.HasExited) { $process.CloseMainWindow() | Out-Null; if (-not $process.WaitForExit(8000)) { $process.Kill() } }
}
Get-Content $log -Encoding UTF8
