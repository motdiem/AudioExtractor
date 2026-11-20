<#
Closes the AudioExtractor started from Downloads\AETest (and only that one),
ends leftover PowerShell processes of these test scripts (an interrupted
prlctl call can leave one hanging in a UI Automation call), and optionally
deletes the test folder.

  stop.ps1 [-Clean] [-ScriptsOnly]
#>
param([switch]$Clean, [switch]$ScriptsOnly)
foreach ($p in Get-CimInstance Win32_Process | Where-Object {
    $_.Name -eq 'powershell.exe' -and $_.CommandLine -like '*windows-test*' -and $_.ProcessId -ne $PID }) {
  Stop-Process -Id $p.ProcessId -Force -ErrorAction SilentlyContinue
  "ended script pid $($p.ProcessId)"
}
if ($ScriptsOnly) { return }
$dir = Join-Path $env:USERPROFILE 'Downloads\AETest'
foreach ($p in Get-Process AudioExtractor -ErrorAction SilentlyContinue | Where-Object { $_.Path -like "$dir\*" }) {
  if (-not $p.CloseMainWindow() -or -not $p.WaitForExit(5000)) { $p.Kill() }
  "stopped pid $($p.Id)"
}
if ($Clean -and (Test-Path $dir)) { Start-Sleep 1; Remove-Item -Recurse -Force $dir; "removed $dir" }
