# Preview-only desktop acceptance for the install flow, modeled on the
# proven configuration-desktop.ps1 driver: window SetFocus + physical click,
# bounded waits, verified cleanup. Execution (确认并执行安装) is never clicked.
$ErrorActionPreference = 'Stop'
$out = $PSScriptRoot
$project = Split-Path (Split-Path $out -Parent) -Parent
$exe = Join-Path $project 'build\bin\Shuttle.exe'
$log = Join-Path $out 'preview-install.log'
$process = $null
$oldRoot = [Environment]::GetEnvironmentVariable('SHUTTLE_CONFIG_TEST_ROOT', 'Process')
$oldWebview = [Environment]::GetEnvironmentVariable('WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS', 'Process')

function Note($text) {
  $text | Out-File $log -Append -Encoding UTF8
  Write-Host $text
}
function U($points) { return -join @($points | ForEach-Object { [char]$_ }) }
$inventoryName = U @(0x5B89,0x88C5,0x5F52,0x5C5E)          # 安装归属
$autoName = U @(0x81EA,0x52A8,0x5B89,0x88C5)               # 自动安装
$detectName = U @(0x91CD,0x65B0,0x68C0,0x6D4B)             # 重新检测
$closeName = U @(0x5173,0x95ED)                            # 关闭
$generateName = U @(0x751F,0x6210,0x5B89,0x88C5,0x9884,0x89C8)  # 生成安装预览
$previewName = U @(0x5B89,0x88C5,0x9884,0x89C8)            # 安装预览
$executeName = U @(0x786E,0x8BA4,0x5E76,0x6267,0x884C,0x5B89,0x88C5)  # 确认并执行安装
$selectPrefix = (U @(0x9009,0x62E9,0x7EC4,0x4EF6)) + ' '   # 选择组件
$components = @('PowerShell 7', 'Git', 'Claude Code', 'Codex')
$packages = @('Microsoft.PowerShell', 'Git.Git', '@anthropic-ai/claude-code', '@openai/codex')
$colon = U @(0xFF1A)                    # ：
$installWord = U @(0x5B89,0x88C5)       # 安装
$skipWord = U @(0x8DF3,0x8FC7)          # 跳过

Add-Type -AssemblyName UIAutomationClient,UIAutomationTypes,System.Drawing,System.Windows.Forms
Add-Type @'
using System; using System.Runtime.InteropServices;
public class InstallWin {
  [DllImport("user32.dll")] public static extern bool SetProcessDPIAware();
  [DllImport("user32.dll")] public static extern bool MoveWindow(IntPtr h, int x, int y, int w, int height, bool repaint);
  [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr h, out RECT r);
  [DllImport("user32.dll")] public static extern bool PrintWindow(IntPtr h, IntPtr dc, uint flags);
  [DllImport("user32.dll")] public static extern uint GetDpiForWindow(IntPtr h);
  [DllImport("user32.dll")] public static extern bool SetCursorPos(int x, int y);
  [DllImport("user32.dll")] public static extern void mouse_event(uint flags, uint x, uint y, int data, UIntPtr extra);
  [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr h);
  [DllImport("user32.dll")] public static extern IntPtr GetForegroundWindow();
  [DllImport("user32.dll")] public static extern void keybd_event(byte key, byte scan, uint flags, UIntPtr extra);
  [DllImport("user32.dll")] public static extern IntPtr WindowFromPoint(POINT p);
  [DllImport("user32.dll")] public static extern IntPtr GetAncestor(IntPtr h, uint flags);
  [StructLayout(LayoutKind.Sequential)] public struct POINT { public int X, Y; }
  [StructLayout(LayoutKind.Sequential)] public struct RECT { public int Left, Top, Right, Bottom; }
}
'@
[InstallWin]::SetProcessDPIAware() | Out-Null

function Window {
  if ($process.HasExited) { throw "Shuttle exited early: $($process.ExitCode)" }
  $condition = [System.Windows.Automation.PropertyCondition]::new(
    [System.Windows.Automation.AutomationElement]::ProcessIdProperty, $process.Id)
  # The IME (Sogou) creates its own top-level windows inside the Shuttle
  # process, and FindFirst by PID can return that 122x64 indicator instead of
  # the main window — which once got resized to cover the screen and swallowed
  # every click. Match the real window by title, else take the largest visible.
  $wins = @()
  try { $wins = @([System.Windows.Automation.AutomationElement]::RootElement.FindAll(
    [System.Windows.Automation.TreeScope]::Children, $condition)) } catch { return $null }
  foreach ($w in $wins) {
    try { if ($w.Current.Name -like 'Shuttle*') { return $w } } catch {}
  }
  $best = $null; $bestArea = [double]0
  foreach ($w in $wins) {
    try {
      $r = $w.Current.BoundingRectangle
      if (-not $w.Current.IsOffscreen -and ($r.Width * $r.Height) -gt $bestArea) {
        $best = $w; $bestArea = $r.Width * $r.Height
      }
    } catch {}
  }
  return $best
}
function Find($root, $name) {
  if (-not $root) { return $null }
  return $root.FindFirst([System.Windows.Automation.TreeScope]::Descendants,
    [System.Windows.Automation.PropertyCondition]::new(
      [System.Windows.Automation.AutomationElement]::NameProperty, $name))
}
function Nodes($root) {
  return $root.FindAll([System.Windows.Automation.TreeScope]::Descendants,
    [System.Windows.Automation.Condition]::TrueCondition)
}
function Dialog {
  if ($process.HasExited) { throw "Shuttle exited early: $($process.ExitCode)" }
  $window = Window
  if (-not $window) { return $null }
  $dialog = $null
  try { $nodes = @(Nodes $window) } catch { return $null }
  foreach ($node in $nodes) {
    if ($node.Current.Name -ne $inventoryName) { continue }
    if ($node.Current.ControlType -ne [System.Windows.Automation.ControlType]::Header -and
        $node.Current.ControlType -ne [System.Windows.Automation.ControlType]::Text) { continue }
    $candidate = [System.Windows.Automation.TreeWalker]::ControlViewWalker.GetParent($node)
    while ($candidate -and $candidate -ne $window) {
      if ((Find $candidate $detectName) -and (Find $candidate $closeName)) { $dialog = $candidate; break }
      $candidate = [System.Windows.Automation.TreeWalker]::ControlViewWalker.GetParent($candidate)
    }
    if ($dialog) { break }
  }
  if ($dialog) { return $dialog }
  # Fallback: the h2 ancestor walk can stall in the WebView2 control view;
  # anchor on the 重新检测 button instead, else use the whole window.
  try { $detect = Find $window $detectName } catch { return $null }
  if (-not $detect) { return $null }
  $candidate = $detect
  while ($candidate -and $candidate -ne $window) {
    if ((Find $candidate $closeName) -and (Find $candidate $inventoryName)) { return $candidate }
    try { $candidate = [System.Windows.Automation.TreeWalker]::ControlViewWalker.GetParent($candidate) } catch { break }
  }
  return $window
}
function Invoke-ElementClick($win, $element, $key) {
  # Keyboard-first: UIA SetFocus scrolls the element into the dialog viewport
  # and is immune to scroll clipping, IME occlusion and window moves. One
  # physical fallback click after — the backdrop is inert, and these buttons
  # are idempotent (open/switch/replan), so a redundant hit is harmless.
  try { $win.SetFocus() } catch {}
  try { $element.SetFocus() } catch {}
  [System.Windows.Forms.SendKeys]::SendWait($key)
  Start-Sleep -Milliseconds 700
  try {
    $r = $element.Current.BoundingRectangle
    $pt = New-Object InstallWin+POINT
    $pt.X = [int]($r.X + $r.Width/2); $pt.Y = [int]($r.Y + $r.Height/2)
    if ([InstallWin]::GetAncestor([InstallWin]::WindowFromPoint($pt), 2) -eq $script:handle) {
      [InstallWin]::SetCursorPos($pt.X, $pt.Y) | Out-Null
      [InstallWin]::mouse_event(2,0,0,0,[UIntPtr]::Zero)
      [InstallWin]::mouse_event(4,0,0,0,[UIntPtr]::Zero)
    } else {
      Note "physical click skipped (window check failed) for: $($element.Current.Name)"
    }
  } catch {}
  Start-Sleep -Milliseconds 400
}
function ClickName($name) {
  # Exactly the sequence proven by probe-nav.ps1: window SetFocus, immediate
  # down/up. The earlier ALT-tap foreground dance made Chromium swallow clicks.
  if ($name -eq $executeName) { throw 'execution click is forbidden in acceptance' }
  $win = Window
  if (-not $win) { throw 'Shuttle window missing' }
  $element = Find $win $name
  if (-not $element) { throw "control not found: $name" }
  if (-not $element.Current.IsEnabled) { throw "control disabled: $name" }
  Invoke-ElementClick $win $element '{ENTER}'
}
function WaitForControl($name, $seconds) {
  for ($i = 0; $i -lt ($seconds * 2); $i++) {
    Start-Sleep -Milliseconds 500
    $root = Dialog
    $element = Find $root $name
    if ($element) { return $root }
  }
  throw "wait timed out for: $name"
}
function SelectComponent($component) {
  # Physical click: the four checkboxes sit in view and a plain click fired
  # React onChange 4/4 (keyboard Space silently missed). Per-element clicks
  # still flake occasionally, so the plan is verified afterwards and missing
  # components are re-clicked (the plan generation itself is read-only).
  $name = $selectPrefix + $component
  $win = Window
  if (-not $win) { throw 'Shuttle window missing' }
  $win.SetFocus()
  $checkbox = Find $win $name
  if (-not $checkbox) { throw "checkbox missing: $name" }
  if (-not $checkbox.Current.IsEnabled) { throw "checkbox disabled: $name" }
  $r = $checkbox.Current.BoundingRectangle
  [InstallWin]::SetCursorPos([int]($r.X + $r.Width/2), [int]($r.Y + $r.Height/2)) | Out-Null
  [InstallWin]::mouse_event(2,0,0,0,[UIntPtr]::Zero)
  [InstallWin]::mouse_event(4,0,0,0,[UIntPtr]::Zero)
  Start-Sleep -Milliseconds 500
}
function Visible($element, $root) {
  $r = $element.Current.BoundingRectangle
  $b = $root.Current.BoundingRectangle
  return (-not $element.Current.IsOffscreen -and $r.Width -gt 0 -and $r.Height -gt 0 -and
    $r.Top -ge $b.Top -and $r.Bottom -le $b.Bottom -and $r.Left -ge $b.Left -and $r.Right -le $b.Right)
}
function Reveal($root, $element) {
  if (Visible $element $root) { return }
  $b = $root.Current.BoundingRectangle
  [InstallWin]::SetCursorPos([int]($b.Left + $b.Width / 2), [int]($b.Top + $b.Height / 2)) | Out-Null
  # Reset to the top first, then walk down — avoids direction confusion when
  # the previous scroll position is unknown after a resize.
  for ($i = 0; $i -lt 3; $i++) {
    [InstallWin]::mouse_event(0x0800, 0, 0, 120, [UIntPtr]::Zero)
    Start-Sleep -Milliseconds 120
  }
  for ($i = 0; $i -lt 40; $i++) {
    if (Visible $element $root) { return }
    $r = $element.Current.BoundingRectangle
    $delta = -120
    if ($r.Top -lt ($b.Top + 65)) { $delta = 120 }
    [InstallWin]::mouse_event(0x0800, 0, 0, $delta, [UIntPtr]::Zero)
    Start-Sleep -Milliseconds 120
  }
  throw "could not scroll into view: $($element.Current.Name)"
}
function Dump($name) {
  $lines = @()
  $root = Dialog
  if ($root) {
    foreach ($node in (Nodes $root)) {
      if ($node.Current.Name) { $lines += $node.Current.Name }
    }
  }
  $lines | Set-Content (Join-Path $out $name) -Encoding UTF8
  return $lines
}
function HasPlanRow($names, $package) {
  # React renders each preview row as separate text nodes: <id>|<：>|<动作>|…
  # A bare id match would also hit detection rows (id|version|·), so require
  # the full id：动作 shape.
  for ($j = 0; $j -lt ($names.Count - 2); $j++) {
    if ($names[$j] -eq $package -and $names[$j + 1] -eq $colon -and
        ($names[$j + 2] -eq $installWord -or $names[$j + 2] -eq $skipWord)) { return $j }
  }
  return -1
}
function GeneratePreview {
  # 生成安装预览 sits below the dialog fold. Wheel scrolling overshot; SetFocus
  # makes Chromium scroll the focused button into view — re-query the rect
  # afterwards and physical-click the fresh position.
  $generate = Find (Window) $generateName
  if (-not $generate) { throw 'generate preview button missing' }
  if (-not $generate.Current.IsEnabled) { throw 'generate preview still disabled; selection clicks failed' }
  try { $generate.SetFocus() } catch {}
  Start-Sleep -Milliseconds 500
  $generate = Find (Window) $generateName
  $r = $generate.Current.BoundingRectangle
  [InstallWin]::SetCursorPos([int]($r.X + $r.Width/2), [int]($r.Y + $r.Height/2)) | Out-Null
  [InstallWin]::mouse_event(2,0,0,0,[UIntPtr]::Zero)
  [InstallWin]::mouse_event(4,0,0,0,[UIntPtr]::Zero)
  Start-Sleep -Milliseconds 700
}
function AssertPreview($root) {
  $heading = Find $root $previewName
  if (-not $heading) { throw 'install preview heading missing' }
  # Wheel scrolling is timing-sensitive right after a resize (relayout plus
  # UIA rect caching). Fallback: SetFocus on the preview's own execute button
  # makes Chromium scroll the section into view — focus only, never clicked.
  try { Reveal $root $heading } catch {
    Note "reveal fallback: $($_.Exception.Message)"
    $execute = Find $root $executeName
    if ($execute) { try { $execute.SetFocus() } catch {} }
    Start-Sleep -Milliseconds 800
  }
  $names = @(Nodes $root | ForEach-Object { $_.Current.Name })
  # Installed components legitimately show 跳过 with no command, so ids are
  # only asserted for rows the plan actually installs.
  $installIds = @()
  foreach ($package in $packages) {
    $idx = HasPlanRow $names $package
    if ($idx -lt 0) { throw "component absent from preview rows: $package" }
    $action = $names[$idx + 2]
    Note "previewRow=$package$colon$action"
    if ($action -eq $installWord) { $installIds += $package }
  }
  foreach ($id in $installIds) {
    $cmd = @($names | Where-Object { $_ -and ($_.Contains("winget install --id $id") -or $_.Contains("npm install -g $id")) })
    if ($cmd.Count -eq 0) { throw "install command missing package id: $id" }
  }
  Note "installIdsChecked=$($installIds -join '|')"
}
function AssertHorizontal($root) {
  $bounds = $root.Current.BoundingRectangle
  $outside = @()
  $scrollable = @()
  foreach ($node in (Nodes $root)) {
    $r = $node.Current.BoundingRectangle
    $pattern = $null
    if ($node.TryGetCurrentPattern([System.Windows.Automation.ScrollPattern]::Pattern, [ref]$pattern)) {
      # Chromium exposes ScrollPattern(HorizontallyScrollable) on any element
      # with overflow:hidden — including the sanctioned `truncate` ellipsis
      # (AGENTS.md: overflow is handled by truncating text, e.g. the sidebar
      # session titles). Those cannot actually scroll; only real containers fail.
      if ($pattern.Current.HorizontallyScrollable -and $node.Current.ClassName -notlike '*truncate*') {
        $scrollable += ("name=[{0}] type={1} class={2} rect=x{3} y{4} w{5} h{6}" -f `
          $node.Current.Name, $node.Current.ControlType.ProgrammaticName, $node.Current.ClassName,
          [int]$r.X, [int]$r.Y, [int]$r.Width, [int]$r.Height)
      }
    }
    if ($node.Current.IsOffscreen -or $r.Width -le 0 -or $r.Height -le 0) { continue }
    # The WebView2 "Intermediate D3D Window" is a native rendering surface, not
    # page content; after a programmatic resize its UIA rect keeps the stale
    # launch geometry long past the real relayout (screenshots confirm).
    if ($node.Current.ClassName -eq 'Intermediate D3D Window') { continue }
    if ($r.Right -lt $bounds.Left -or $r.Left -gt $bounds.Right) { $outside += $node.Current.Name; continue }
    if ($r.Left -lt $bounds.Left - 1 -or $r.Right -gt $bounds.Right + 1) {
      $outside += $node.Current.Name
      Note ("outside name=[{0}] type={1} class={2} rect=x{3} y{4} w{5} h{6} win=x{7} y{8} w{9} h{10}" -f `
        $node.Current.Name, $node.Current.ControlType.ProgrammaticName, $node.Current.ClassName,
        [int]$r.X, [int]$r.Y, [int]$r.Width, [int]$r.Height,
        [int]$bounds.X, [int]$bounds.Y, [int]$bounds.Width, [int]$bounds.Height)
    }
  }
  Note "horizontalOverflow=$($outside.Count)"
  Note "hScrollableCount=$($scrollable.Count)"
  foreach ($s in $scrollable) { Note "hScrollable $s" }
  if ($outside.Count) { throw 'install panel overflowed horizontally' }
  if ($scrollable.Count) { throw 'horizontal scrolling exposed at this size' }
}
function Capture($name) {
  $rect = New-Object InstallWin+RECT
  if (-not [InstallWin]::GetWindowRect($script:handle, [ref]$rect)) { throw 'GetWindowRect failed' }
  $bitmap = [System.Drawing.Bitmap]::new($rect.Right - $rect.Left, $rect.Bottom - $rect.Top)
  $graphics = $null
  $dc = [IntPtr]::Zero
  try {
    $graphics = [System.Drawing.Graphics]::FromImage($bitmap)
    $dc = $graphics.GetHdc()
    if (-not [InstallWin]::PrintWindow($script:handle, $dc, 2)) { throw 'PrintWindow failed' }
    $graphics.ReleaseHdc($dc); $dc = [IntPtr]::Zero
    $bitmap.Save((Join-Path $out $name), [System.Drawing.Imaging.ImageFormat]::Png)
    Note "screenshot=$name physical=$($bitmap.Width)x$($bitmap.Height)"
  } finally {
    if ($dc -ne [IntPtr]::Zero) { $graphics.ReleaseHdc($dc) }
    if ($graphics) { $graphics.Dispose() }
    $bitmap.Dispose()
  }
}

try {
  if (-not (Test-Path -LiteralPath $exe -PathType Leaf)) { throw "Build Shuttle first: $exe" }
  Set-Content $log 'Preview-only desktop acceptance; execution is forbidden.' -Encoding UTF8
  $testRoot = Join-Path ([IO.Path]::GetTempPath()) ('Shuttle-install-' + [guid]::NewGuid().ToString('N'))
  if (Test-Path -LiteralPath $testRoot) { throw "Isolation root already exists: $testRoot" }
  New-Item -ItemType Directory -Path $testRoot | Out-Null
  Note "isolationRoot=$testRoot"
  # Force the WebView2 renderer accessibility tree on from launch (process-
  # scoped; the WM_GETOBJECT wake race intermittently leaves the tree empty).
  [Environment]::SetEnvironmentVariable('WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS', '--force-renderer-accessibility', 'Process')
  [Environment]::SetEnvironmentVariable('SHUTTLE_CONFIG_TEST_ROOT', $testRoot, 'Process')
  try { $process = Start-Process -FilePath $exe -WorkingDirectory $project -PassThru }
  finally {
    [Environment]::SetEnvironmentVariable('SHUTTLE_CONFIG_TEST_ROOT', $oldRoot, 'Process')
    [Environment]::SetEnvironmentVariable('WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS', $oldWebview, 'Process')
  }
  $win = $null
  for ($i = 0; $i -lt 60 -and -not $win; $i++) { Start-Sleep -Milliseconds 500; $win = Window }
  if (-not $win) { throw 'Shuttle window not found' }
  $script:handle = [IntPtr]$win.Current.NativeWindowHandle
  $scale = [InstallWin]::GetDpiForWindow($handle) / 96.0
  if ($scale -le 0) { throw 'Invalid window DPI' }
  # No window move before the clicks: after a programmatic move Chromium's
  # hit-test mapping refreshes asynchronously and clicks silently miss (probe
  # without a move worked first try). Resizing happens later, for screenshots
  # only, where nothing needs clicking.
  Start-Sleep -Seconds 2

  # First click is occasionally swallowed outright (focus/IME race); retry.
  $opened = $false
  for ($i = 0; $i -lt 3 -and -not $opened; $i++) {
    ClickName $inventoryName
    try { WaitForControl $detectName 15 | Out-Null; $opened = $true }
    catch { Note "openDialogRetry=$($i + 1)" }
  }
  if (-not $opened) { throw 'inventory dialog did not open after retries' }
  $ready = $false
  for ($i = 0; $i -lt 180; $i++) {
    Start-Sleep -Milliseconds 500
    $detect = Find (Dialog) $detectName
    if ($detect -and $detect.Current.IsEnabled) { $ready = $true; break }
  }
  $names = @(Dump 'preview-install-detection-uia.txt')
  Note "detectionUIA=$($names[0..([Math]::Min(39,$names.Count-1))] -join '|')"
  if (-not $ready) { throw 'inventory detection timed out' }
  $ledgerPath = Join-Path $testRoot 'install-ledger.json'
  if (-not ($names | Where-Object { $_.Contains($ledgerPath) })) { throw "isolated ledger path not displayed: $ledgerPath" }
  if (-not ($names | Where-Object { $_.Contains('ARP/MSIX') })) { throw 'detection report missing' }

  ClickName $autoName
  WaitForControl $generateName 20 | Out-Null
  # Checkbox physical clicks flake occasionally (one in four missed in an
  # earlier run). The generated plan is the verifier and generating is
  # read-only, so re-click any missing component and re-plan, bounded.
  $missing = @($components)
  for ($attempt = 1; ($attempt -le 3) -and ($missing.Count -gt 0); $attempt++) {
    foreach ($component in $missing) { SelectComponent $component }
    GeneratePreview
    WaitForControl $previewName 20 | Out-Null
    $planNames = @(Nodes (Window) | ForEach-Object { $_.Current.Name })
    $missing = @()
    for ($p = 0; $p -lt $packages.Count; $p++) {
      if ((HasPlanRow $planNames $packages[$p]) -lt 0) { $missing += $components[$p] }
    }
    if ($missing.Count) { Note "planMissing=$($missing -join '|') attempt=$attempt" }
  }
  if ($missing.Count -gt 0) { throw "preview rows missing after retries: $($missing -join '|')" }
  $names = @(Dump 'preview-install-plan-uia.txt')
  Note "planUIA=$($names[0..([Math]::Min(59,$names.Count-1))] -join '|')"

  foreach ($size in @(@(720,480), @(1080,720))) {
    $width = [int]($size[0] * $scale); $height = [int]($size[1] * $scale)
    if (-not [InstallWin]::MoveWindow($handle, 0, 0, $width, $height, $true)) { throw 'MoveWindow failed' }
    Start-Sleep -Seconds 2
    $rect = New-Object InstallWin+RECT
    if (-not [InstallWin]::GetWindowRect($handle, [ref]$rect)) { throw 'GetWindowRect failed' }
    if ([Math]::Abs(($rect.Right-$rect.Left)-$width) -gt 1 -or [Math]::Abs(($rect.Bottom-$rect.Top)-$height) -gt 1) {
      throw "window did not accept requested DIP size: $($size -join 'x')"
    }
    $root = $null
    for ($i = 0; $i -lt 20 -and -not $root; $i++) { Start-Sleep -Milliseconds 500; $root = Dialog }
    if (-not $root) { throw 'inventory dialog lost before preview assertions' }
    Note "DIP=$($size -join 'x') scale=$scale"
    AssertPreview $root
    AssertHorizontal $root
    Capture "preview-install-$($size[0])x$($size[1]).png"
  }
  if (Test-Path -LiteralPath $ledgerPath) { throw 'preview-only run unexpectedly created an install ledger' }
  Note 'PASS: four real preview rows checked; execution never invoked.'
} catch {
  Note "ERROR: $($_.Exception.ToString())"
  if ($process -and -not $process.HasExited) {
    try { Capture 'preview-install-debug.png' } catch { Note "debug capture failed: $($_.Exception.Message)" }
    try { Dump 'preview-install-failure-uia.txt' | ForEach-Object { Note $_ } }
    catch { Note "UIA diagnostic failed: $($_.Exception.Message)" }
  }
  throw
} finally {
  if ($process) {
    $process.Refresh()
    if (-not $process.HasExited) {
      $process.CloseMainWindow() | Out-Null
      if (-not $process.WaitForExit(8000)) {
        Note "graceful close timed out; terminating owned PID=$($process.Id)"
        try { $process.Kill() } catch { if (-not $process.HasExited) { throw } }
        if (-not $process.WaitForExit(8000)) { throw "owned PID did not exit: $($process.Id)" }
      }
    }
    $process.Refresh()
    if (-not $process.HasExited) { throw "owned PID still running: $($process.Id)" }
    Note "ownedProcessExited=$($process.Id) exitCode=$($process.ExitCode)"
    $process.Dispose()
  }
}
