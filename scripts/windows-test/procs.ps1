# Lists running AudioExtractor and ffmpeg processes, and the PowerShell
# processes running these test scripts (a hung UI Automation call shows here).
Get-CimInstance Win32_Process |
  Where-Object { $_.Name -in 'AudioExtractor.exe', 'ffmpeg.exe' -or ($_.Name -eq 'powershell.exe' -and $_.CommandLine -like '*windows-test*' -and $_.ProcessId -ne $PID) } |
  Select-Object ProcessId, ParentProcessId, CreationDate, ExecutablePath, CommandLine |
  Format-List | Out-String -Width 300
