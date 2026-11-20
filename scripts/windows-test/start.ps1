<#
Starts Downloads\AETest\AudioExtractor.exe (with optional file or folder
arguments, as if they were dropped on the exe) and returns at once.
Run it in the background on the Mac side: prlctl exec can wait for GUI apps.

  start.ps1 [-Env NAME=VALUE] [path...]
#>
[CmdletBinding(PositionalBinding = $false)] # paths must not bind to -Env
param([string]$Env,[Parameter(ValueFromRemainingArguments)][string[]]$Paths)
$exe = Join-Path $env:USERPROFILE 'Downloads\AETest\AudioExtractor.exe'
if ($Env) { $k, $v = $Env -split '=', 2; [Environment]::SetEnvironmentVariable($k, $v, 'Process') }
$args_ = @($Paths | Where-Object { $_ } | ForEach-Object { '"' + $_ + '"' })
if ($args_.Count) {
  $p = Start-Process -FilePath $exe -ArgumentList $args_ -WorkingDirectory (Split-Path $exe) -PassThru
} else {
  $p = Start-Process -FilePath $exe -WorkingDirectory (Split-Path $exe) -PassThru
}
"started pid $($p.Id)"
