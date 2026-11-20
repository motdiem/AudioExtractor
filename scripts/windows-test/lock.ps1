<#
Holds a file open with no sharing for a while, as a media player or editor
would, to test the output-locked message. It returns at once: the lock is held
by a detached, hidden PowerShell. (A long-running prlctl exec session made the
other prlctl calls hang while it lasted.)

  lock.ps1 -Path C:\...\call.m4a [-Seconds 60]
#>
param([Parameter(Mandatory)][string]$Path, [int]$Seconds = 60, [switch]$Hold)
if ($Hold) {
  $f = [System.IO.File]::Open($Path, 'Open', 'ReadWrite', 'None')
  try { Start-Sleep -Seconds $Seconds } finally { $f.Close() }
  return
}
$p = Start-Process powershell -WindowStyle Hidden -PassThru -ArgumentList @(
  '-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', "`"$PSCommandPath`"", '-Path', "`"$Path`"", '-Seconds', $Seconds, '-Hold')
Start-Sleep -Seconds 2
try { [System.IO.File]::Open($Path, 'Open', 'Read', 'ReadWrite').Close(); "NOT locked (pid $($p.Id))" }
catch { "locked $Path for $Seconds s (pid $($p.Id))" }
