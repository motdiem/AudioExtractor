<#
Starts Downloads\AETest\AudioExtractor.exe (with optional file or folder
arguments, as if they were dropped on the exe) and returns at once.
Run it in the background on the Mac side: prlctl exec can wait for GUI apps.

  start.ps1 [-Env NAME=VALUE] [-Exe other.exe] [path...]

-Exe copies another exe (a share path, say the --dev build) into AETest and
starts that instead.
#>
[CmdletBinding(PositionalBinding = $false)] # paths must not bind to -Env
param([string]$Env, [string]$Exe, [Parameter(ValueFromRemainingArguments)][string[]]$Paths)
$dir = Join-Path $env:USERPROFILE 'Downloads\AETest'
$path = Join-Path $dir 'AudioExtractor.exe'
if ($Exe) { $path = Join-Path $dir (Split-Path $Exe -Leaf); Copy-Item -Force $Exe $path }
if ($Env) { $k, $v = $Env -split '=', 2; [Environment]::SetEnvironmentVariable($k, $v, 'Process') }
$args_ = @($Paths | Where-Object { $_ } | ForEach-Object { '"' + $_ + '"' })
if ($args_.Count) {
  $p = Start-Process -FilePath $path -ArgumentList $args_ -WorkingDirectory $dir -PassThru
} else {
  $p = Start-Process -FilePath $path -WorkingDirectory $dir -PassThru
}
"started pid $($p.Id)"
