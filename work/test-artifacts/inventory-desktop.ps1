$ErrorActionPreference = 'Stop'
$out = 'D:\harry\youchui\Shuttle\work\test-artifacts'
$log = Join-Path $env:TEMP 'inventory-desktop.log'
Remove-Item $log -ErrorAction SilentlyContinue
function Note($t) { $t | Out-File $log -Append -Encoding UTF8 }

Add-Type -AssemblyName UIAutomationClient,System.Drawing
Add-Type @'
using System; using System.Runtime.InteropServices;
public class InvWin {
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
[InvWin]::SetProcessDPIAware() | Out-Null

# read-only panel: detection never writes the real ledger, so no isolation root is needed
$process = Start-Process 'D:\harry\youchui\Shuttle\build\bin\Shuttle.exe' -PassThru

function Window {
  $c = [System.Windows.Automation.PropertyCondition]::new([System.Windows.Automation.AutomationElement]::ProcessIdProperty, $process.Id)
  return [System.Windows.Automation.AutomationElement]::RootElement.FindFirst([System.Windows.Automation.TreeScope]::Children, $c)
}
function Find($win, $name) {
  return $win.FindFirst([System.Windows.Automation.TreeScope]::Descendants,
    [System.Windows.Automation.PropertyCondition]::new([System.Windows.Automation.AutomationElement]::NameProperty, $name))
}
function Capture($handle, $name) {
  $r = New-Object InvWin+RECT
  [InvWin]::GetWindowRect($handle, [ref]$r) | Out-Null
  $w = $r.Right - $r.Left; $hgt = $r.Bottom - $r.Top
  $bitmap = [System.Drawing.Bitmap]::new($w, $hgt)
  $g = [System.Drawing.Graphics]::FromImage($bitmap)
  $hdc = $g.GetHdc()
  # PW_RENDERFULLCONTENT captures the WebView2 surface even off-screen
  [InvWin]::PrintWindow($handle, $hdc, 2) | Out-Null
  $g.ReleaseHdc($hdc); $g.Dispose()
  $bitmap.Save((Join-Path $out $name)); $bitmap.Dispose()
  return "${w}x${hgt}"
}
function Click($win, $element) {
  $r = $element.Current.BoundingRectangle
  $win.SetFocus()
  [InvWin]::SetCursorPos([int]($r.X + $r.Width / 2), [int]($r.Y + $r.Height / 2)) | Out-Null
  [InvWin]::mouse_event(2,0,0,0,[UIntPtr]::Zero)
  [InvWin]::mouse_event(4,0,0,0,[UIntPtr]::Zero)
}

# names are built from code points so this file stays ASCII and cannot be
# misencoded as GBK when PowerShell reads it
$inventoryLabel = -join @([char]0x5B89, [char]0x88C5, [char]0x5F52, [char]0x5C5E)
$noteNeedle = 'ARP/MSIX'
$blockedLabel = -join @([char]0x4E0D, [char]0x53EF, [char]0x5378, [char]0x8F7D)
$pkgSuffix = -join @([char]0x4E2A, [char]0x5305)

try {
  for ($i = 0; $i -lt 60; $i++) { if (Window) { break }; Start-Sleep -Milliseconds 250 }
  $win = Window
  if (-not $win) { throw 'window not found' }
  $handle = $win.Current.NativeWindowHandle
  $scale = [InvWin]::GetDpiForWindow($handle) / 96
  [InvWin]::SetForegroundWindow($handle) | Out-Null
  # 1080x720 at 150% scaling is the tightest supported layout
  [InvWin]::MoveWindow($handle, 0, 0, [int](1080 * $scale), [int](720 * $scale), $true) | Out-Null
  Start-Sleep -Milliseconds 2000

  $win = Window
  $button = Find $win $inventoryLabel
  if (-not $button) { throw "inventory button not found: $inventoryLabel" }
  Click $win $button
  Start-Sleep -Milliseconds 1500

  # real detection runs npm + winget; wait until the source rows or the note appear
  $ready = $false
  for ($i = 0; $i -lt 120; $i++) {
    Start-Sleep -Milliseconds 500
    $win = Window
    if (-not $win) { throw 'window disappeared during detection' }
    $all = $win.FindAll([System.Windows.Automation.TreeScope]::Descendants, [System.Windows.Automation.Condition]::TrueCondition)
    foreach ($c in $all) { if ($c.Current.Name -like "*$noteNeedle*") { $ready = $true; break } }
    if ($ready) { break }
  }
  if (-not $ready) { throw 'detection never produced a report' }

  $paths = @()
  $counts = @()
  $blocked = 0
  $all = $win.FindAll([System.Windows.Automation.TreeScope]::Descendants, [System.Windows.Automation.Condition]::TrueCondition)
  foreach ($c in $all) {
    $n = $c.Current.Name
    if ($n -match 'install-ledger\.json$') { $paths += $n }
    # UIA splits a source row into three nodes: name, separator, "N 个包"
    if ($n.EndsWith($pkgSuffix)) { $counts += $n }
    if ($n -eq $blockedLabel) { $blocked++ }
  }
  Note "ledger=$($paths -join '|')"
  Note "sourceCounts=$($counts -join '|') nonRemovableRows=$blocked"
  $all | ForEach-Object { $_.Current.Name } | Where-Object { $_ } |
    Set-Content (Join-Path $out 'inventory-uia-names.txt') -Encoding UTF8
  if (-not $paths) { throw 'ledger path row missing' }
  if ($counts.Count -lt 2) { throw "two source rows expected, got: $($counts -join '|')" }
  if ($blocked -lt 1) { throw 'no item was marked non-removable' }

  # smallest supported viewport, then the tightest one
  [InvWin]::MoveWindow($handle, 0, 0, [int](720 * $scale), [int](480 * $scale), $true) | Out-Null
  Start-Sleep -Milliseconds 2000
  Note "720x480 physical=$(Capture $handle 'inventory-720x480.png')"
  [InvWin]::MoveWindow($handle, 0, 0, [int](1080 * $scale), [int](720 * $scale), $true) | Out-Null
  Start-Sleep -Milliseconds 2000
  Note "1080x720 physical=$(Capture $handle 'inventory-1080x720.png')"

  $wr = (Window).Current.BoundingRectangle
  $rows = @((Window).FindAll([System.Windows.Automation.TreeScope]::Descendants, [System.Windows.Automation.Condition]::TrueCondition) | ForEach-Object {
      $r = $_.Current.BoundingRectangle
      if ($r.Width -lt 1 -or $r.Height -lt 1) { return }
      [pscustomobject]@{ name = $_.Current.Name; x = $r.X; y = $r.Y; w = $r.Width; h = $r.Height }
    })
  $outside = @($rows | Where-Object {
      $beyond = ($_.x -lt $wr.X - 0.5) -or (($_.x + $_.w) -gt ($wr.X + $wr.Width) + 0.5)
      $isContentPane = ($_.h -gt $wr.Height * 0.9) -and ($_.w -gt $wr.Width * 0.9)
      $_.w -gt 0 -and $beyond -and (-not $isContentPane) })
  Note "elements=$($rows.Count) horizontalOverflow=$($outside.Count) names=$(@($outside | ForEach-Object { $_.name }) -join ',')"
  if ($outside.Count) { throw 'inventory panel overflowed horizontally' }
  Note 'PASS'
} catch {
  Note "ERROR: $($_.Exception.Message)"
} finally {
  $process.Refresh()
  if (-not $process.HasExited) { $process.CloseMainWindow() | Out-Null; if (-not $process.WaitForExit(8000)) { $process.Kill() } }
}
Get-Content $log -Encoding UTF8
