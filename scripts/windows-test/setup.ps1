<#
Copies the exe and the test media from the Mac share into a normal user
folder (Downloads\AETest), the way a user would run the app. Outputs then land
on a local NTFS disk, where file locks behave as on a real PC.

  setup.ps1 -Exe \\Mac\Home\...\bin\AudioExtractor.exe -Media \\Mac\Home\AEWinTest\media
#>
param([Parameter(Mandatory)][string]$Exe, [Parameter(Mandatory)][string]$Media)
$ErrorActionPreference = 'Stop'
$dir = Join-Path $env:USERPROFILE 'Downloads\AETest'
if (Get-Process AudioExtractor -ErrorAction SilentlyContinue | Where-Object { $_.Path -like "$dir\*" }) {
  throw 'AudioExtractor from AETest is running: run stop.ps1 first'
}
if (Test-Path $dir) { Remove-Item -Recurse -Force $dir }
New-Item -ItemType Directory $dir | Out-Null
Copy-Item $Exe (Join-Path $dir 'AudioExtractor.exe')
Copy-Item -Recurse $Media (Join-Path $dir 'media')
# Files copied from a network share carry no Mark of the Web; a downloaded exe
# would (and SmartScreen would then ask once). Report it either way.
$zone = Get-Content -LiteralPath (Join-Path $dir 'AudioExtractor.exe') -Stream Zone.Identifier -ErrorAction SilentlyContinue
"Mark of the Web: " + ($(if ($zone) { $zone -join ' ' } else { 'none' }))
Get-ChildItem -Recurse $dir | Select-Object FullName, Length | Format-Table -AutoSize | Out-String -Width 200
