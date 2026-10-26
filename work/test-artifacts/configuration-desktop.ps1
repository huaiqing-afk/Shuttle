$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName UIAutomationClient,System.Drawing,System.Windows.Forms
Add-Type 'using System; using System.Runtime.InteropServices; public class ConfigMouse { [DllImport("user32.dll")] public static extern bool SetProcessDPIAware(); [DllImport("user32.dll")] public static extern bool SetCursorPos(int x,int y); [DllImport("user32.dll")] public static extern void mouse_event(uint f,uint x,uint y,uint d,UIntPtr e); }'
[ConfigMouse]::SetProcessDPIAware() | Out-Null
$root = Join-Path ([IO.Path]::GetTempPath()) ('shuttle-config-' + [guid]::NewGuid())
[IO.Directory]::CreateDirectory($root) | Out-Null
[IO.File]::WriteAllText((Join-Path $root 'settings.json'), '{"theme":"system","userOwned":true}')
$env:SHUTTLE_CONFIG_TEST_ROOT = $root
$process = Start-Process 'D:\harry\youchui\Shuttle\build\bin\Shuttle.exe' -PassThru
Remove-Item Env:SHUTTLE_CONFIG_TEST_ROOT
function Window {
 $condition = [System.Windows.Automation.PropertyCondition]::new([System.Windows.Automation.AutomationElement]::ProcessIdProperty, $process.Id)
 return [System.Windows.Automation.AutomationElement]::RootElement.FindFirst([System.Windows.Automation.TreeScope]::Children,$condition)
}
function ClickName($name) {
 $win=Window
 $win.SetFocus()
 $condition=[System.Windows.Automation.PropertyCondition]::new([System.Windows.Automation.AutomationElement]::NameProperty,$name)
 $element=$win.FindFirst([System.Windows.Automation.TreeScope]::Descendants,$condition)
 if(-not $element){throw "control not found: $name"}
 $r=$element.Current.BoundingRectangle
 [ConfigMouse]::SetCursorPos([int]($r.X+$r.Width/2),[int]($r.Y+$r.Height/2))|Out-Null
 [ConfigMouse]::mouse_event(2,0,0,0,[UIntPtr]::Zero)
 [ConfigMouse]::mouse_event(4,0,0,0,[UIntPtr]::Zero)
 Start-Sleep -Milliseconds 700
}
try {
 Start-Sleep -Seconds 3
 ClickName '配置与还原'
 ClickName '启用 PowerShell 历史预测'
 ClickName '预览差异'
 if(Test-Path (Join-Path $root 'profile.ps1')){throw 'preview wrote profile'}
 ClickName '确认应用所示差异'
 $records=Get-ChildItem (Join-Path $root 'operations') -Filter record.json -Recurse
 if($records.Count -ne 1){throw 'missing operation record'}
 $record=[IO.File]::ReadAllText($records[0].FullName)|ConvertFrom-Json
 if($record.Operation.State -ne 'succeeded'){throw 'apply not succeeded'}
 ClickName '核对并预览还原'
 ClickName '确认应用所示差异'
 if([IO.File]::ReadAllText((Join-Path $root 'profile.ps1')) -match 'shuttle managed'){throw 'restore left managed block'}
 $screen=[System.Windows.Forms.Screen]::PrimaryScreen.Bounds
 $bitmap=[System.Drawing.Bitmap]::new($screen.Width,$screen.Height)
 $graphics=[System.Drawing.Graphics]::FromImage($bitmap)
 $graphics.CopyFromScreen($screen.Location,[System.Drawing.Point]::Empty,$screen.Size)
 $graphics.Dispose()
 $bitmap.Save('D:\harry\youchui\Shuttle\work\test-artifacts\configuration-desktop.png')
 $bitmap.Dispose()
 "PASS: desktop preview/apply/record/restore; isolation=$root"
} finally {
 $process.Refresh()
 if(-not $process.HasExited){$process.CloseMainWindow()|Out-Null; if(-not $process.WaitForExit(5000)){throw 'acceptance app did not exit'}}
}
