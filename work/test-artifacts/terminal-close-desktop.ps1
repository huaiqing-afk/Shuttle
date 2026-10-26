$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName UIAutomationClient,System.Windows.Forms
Add-Type 'using System; using System.Runtime.InteropServices; public class K4 { [DllImport("user32.dll")] public static extern bool SetProcessDPIAware(); [DllImport("user32.dll")] public static extern bool SetCursorPos(int x,int y); [DllImport("user32.dll")] public static extern void mouse_event(uint f,uint x,uint y,uint d,UIntPtr e); [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr h); [DllImport("user32.dll")] public static extern bool ShowWindow(IntPtr h,int n); [DllImport("user32.dll")] public static extern IntPtr GetForegroundWindow(); }'
[K4]::SetProcessDPIAware() | Out-Null
$UIA=[System.Windows.Automation.AutomationElement]; $Scope=[System.Windows.Automation.TreeScope]
function GetWin($p) { $UIA::RootElement.FindFirst($Scope::Children,[System.Windows.Automation.PropertyCondition]::new($UIA::ProcessIdProperty,$p)) }
function Foreground($w) {
 $h=[IntPtr]$w.Current.NativeWindowHandle
 for($i=0;$i -lt 10;$i++){
  [K4]::ShowWindow($h,9)|Out-Null; [K4]::SetForegroundWindow($h)|Out-Null; Start-Sleep -Milliseconds 300
  if([K4]::GetForegroundWindow() -eq $h){ return $true }
  $r=$w.Current.BoundingRectangle
  [K4]::SetCursorPos([int]($r.X+$r.Width/2),[int]($r.Y+18))|Out-Null
  [K4]::mouse_event(2,0,0,0,[UIntPtr]::Zero); [K4]::mouse_event(4,0,0,0,[UIntPtr]::Zero); Start-Sleep -Milliseconds 300
  if([K4]::GetForegroundWindow() -eq $h){ return $true }
 }
 return $false
}
function ClickCenter($w,$r) {
 if(-not (Foreground $w)){ throw 'shuttle cannot take foreground' }
 [K4]::SetCursorPos([int]($r.X+$r.Width/2),[int]($r.Y+$r.Height/2))|Out-Null
 Start-Sleep -Milliseconds 120
 [K4]::mouse_event(2,0,0,0,[UIntPtr]::Zero); Start-Sleep -Milliseconds 60; [K4]::mouse_event(4,0,0,0,[UIntPtr]::Zero)
 Start-Sleep -Milliseconds 600
}
$before=(Get-CimInstance Win32_Process -Filter "Name='pwsh.exe'").ProcessId
$app=Start-Process 'D:\harry\youchui\Shuttle\build\bin\Shuttle.exe' -PassThru
try {
 Start-Sleep -Seconds 4
 $win=GetWin $app.Id
 if(-not (Foreground $win)){ throw 'shuttle never took foreground' }
 $new=$win.FindFirst($Scope::Descendants,[System.Windows.Automation.PropertyCondition]::new($UIA::NameProperty,'新建终端'))
 ClickCenter $win $new.Current.BoundingRectangle
 $items=$win.FindAll($Scope::Descendants,[System.Windows.Automation.PropertyCondition]::new($UIA::ControlTypeProperty,[System.Windows.Automation.ControlType]::MenuItem))
 if($items.Count -eq 0){ throw 'menu did not open' }
 ClickCenter $win $items[0].Current.BoundingRectangle     # PowerShell
 Start-Sleep -Seconds 3
 $after=(Get-CimInstance Win32_Process -Filter "Name='pwsh.exe'").ProcessId
 $created=$after | Where-Object { $before -notcontains $_ }
 if(-not $created){ throw 'terminal child not found' }
 "terminal-pids: $($created -join ',')"
 # focus the terminal surface, then use the app's own Ctrl+Shift+W
 $win=GetWin $app.Id
 $surface=$win.FindFirst($Scope::Descendants,[System.Windows.Automation.PropertyCondition]::new($UIA::NameProperty,'终端 PowerShell'))
 if($surface){ ClickCenter $win $surface.Current.BoundingRectangle } else { if(-not (Foreground $win)){ throw 'refocus failed' } }
 Start-Sleep -Milliseconds 500
 [System.Windows.Forms.SendKeys]::SendWait('^+w')
 Start-Sleep -Seconds 1
 [System.Windows.Forms.SendKeys]::SendWait('{ENTER}')      # native confirm defaults to the affirmative button
 $closed=$false
 for($i=0;$i -lt 30;$i++){
  Start-Sleep -Milliseconds 300
  if(-not (Get-Process -Id $created[0] -ErrorAction SilentlyContinue)){ $closed=$true; break }
 }
 if(-not $closed){ throw "child $($created[0]) survived close" }
 'terminal-close: child exited'
 $app.CloseMainWindow()|Out-Null
 if(-not $app.WaitForExit(15000)){ throw 'app did not exit' }
 'app-exit: clean'
 if(Get-Process -Id $created[0] -ErrorAction SilentlyContinue){ throw 'residual child' }
 'no-residual'
} finally { $app.Refresh(); if(-not $app.HasExited){ Stop-Process -Id $app.Id -Force -ErrorAction SilentlyContinue } }
