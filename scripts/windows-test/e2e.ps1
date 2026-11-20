<#
Runs the scripted end-to-end checks in one go, inside the VM (one prlctl call
instead of dozens: each prlctl exec costs seconds, and minutes when the VM is
busy). Assumes setup.ps1 has run. Copies outputs to -Collect (a share folder)
so the Mac can probe them with ffprobe. Prints PASS/FAIL lines.

  e2e.ps1 [-Collect \\Mac\Home\AEWinTest\out] [-Only m4a,mp3,...]
#>
param([string]$Collect, [string[]]$Only)
$ErrorActionPreference = 'Continue'
$ae = Join-Path $PSScriptRoot 'ae.ps1'
$base = Join-Path $env:USERPROFILE 'Downloads\AETest'
$D = Join-Path $base 'media'
$exe = Join-Path $base 'AudioExtractor.exe'
function AE { & powershell -NoProfile -ExecutionPolicy Bypass -File $ae @args 2>&1 | ForEach-Object { "    $_" } }
function Check($name, [bool]$ok, $detail = '') { "{0} {1} {2}" -f $(if ($ok) { 'PASS' } else { 'FAIL' }), $name, $detail }
function Want($t) { -not $Only -or $Only -contains $t }
function Parts { @(Get-ChildItem -Force -LiteralPath $D -Filter '.audioextractor-*' -ErrorAction SilentlyContinue) }
function Ffmpegs { @(Get-Process ffmpeg -ErrorAction SilentlyContinue | Where-Object { $_.Path -like "$env:LOCALAPPDATA\AudioExtractor\*" }) }
function Start-App { Start-Process $exe -WorkingDirectory $base | Out-Null; Start-Sleep 6 }
function App { Get-Process AudioExtractor -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq $exe } }
# Starts a long MP3 into $out and returns once ffmpeg is running.
function Start-LongMp3($out) {
  Remove-Item -Force -LiteralPath $out -ErrorAction SilentlyContinue
  AE format MP3; AE click drop; AE open $D long.mp4; AE save $out
  for ($i = 0; $i -lt 40 -and -not (Ffmpegs); $i++) { Start-Sleep -Milliseconds 250 }
  Start-Sleep 1
}

if (-not (App)) { Start-App }

if (Want 'm4a') {
  '== single file to M4A'
  Remove-Item -Force "$D\call.m4a" -ErrorAction SilentlyContinue
  AE format M4A; AE click drop; AE open $D call.mp4; AE save "$D\call.m4a"; AE waitfile "$D\call.m4a"
  $s = (AE wait 'Saved as call.m4a' 10) -join "`n"
  Check 'm4a-done' ($s -like '*Saved as call.m4a*')
  AE click again
}
if (Want 'mp3') {
  '== single file to MP3'
  Remove-Item -Force "$D\call.mp3" -ErrorAction SilentlyContinue
  AE format MP3; AE click drop; AE open $D call.mp4; AE save "$D\call.mp3"; AE waitfile "$D\call.mp3"
  $s = (AE wait 'Saved as call.mp3' 20) -join "`n"
  Check 'mp3-done' ($s -like '*Saved as call.mp3*')
  AE click again
}
if (Want 'hidden') {
  '== (a) the temporary .part is hidden while encoding, the result is not'
  Start-LongMp3 "$D\long.mp3"
  $p = Parts
  Check 'part-exists-while-encoding' ($p.Count -eq 1) ($p | ForEach-Object { "$($_.Name) [$($_.Attributes)]" })
  Check 'part-is-hidden' ($p.Count -eq 1 -and ($p[0].Attributes -band [IO.FileAttributes]::Hidden))
  AE waitfile "$D\long.mp3" 240 | Out-Null
  AE wait 'Saved as long.mp3' 240 | Out-Null
  $f = Get-Item -Force -LiteralPath "$D\long.mp3" -ErrorAction SilentlyContinue
  Check 'result-not-hidden' ($f -and -not ($f.Attributes -band [IO.FileAttributes]::Hidden)) "$($f.Attributes)"
  Check 'no-part-after-rename' ((Parts).Count -eq 0)
  AE click again
}
if (Want 'cancel') {
  '== cancel mid-MP3'
  Start-LongMp3 "$D\cancel.mp3"
  AE click cancel; Start-Sleep 2
  Check 'cancel-no-output' (-not (Test-Path "$D\cancel.mp3"))
  Check 'cancel-no-part' ((Parts).Count -eq 0)
  Check 'cancel-no-ffmpeg' ((Ffmpegs).Count -eq 0)
}
if (Want 'busy') {
  '== (e) a second launch with a file while a job runs: Busy'
  Start-LongMp3 "$D\busy.mp3"
  $second = Start-Process $exe -ArgumentList "`"$D\call.mp4`"" -PassThru
  $box = (AE msgbox) -join "`n"
  $box
  Check 'busy-message' ($box -like '*Busy*')
  Check 'second-process-exited' ($second.WaitForExit(5000))
  Check 'job-still-running' ((Ffmpegs).Count -eq 1)
  AE click cancel; Start-Sleep 2
}
if (Want 'close') {
  '== (c) close the window mid-encode'
  Start-LongMp3 "$D\close.mp3"
  $a = App; $a.CloseMainWindow() | Out-Null
  $exited = $a.WaitForExit(10000); Start-Sleep 2
  Check 'close-app-exited' $exited
  Check 'close-no-ffmpeg' ((Ffmpegs).Count -eq 0)
  Check 'close-no-part' ((Parts).Count -eq 0) ((Parts) | ForEach-Object Name)
  Check 'close-no-output' (-not (Test-Path "$D\close.mp3"))
  Start-App
}
if (Want 'kill') {
  '== (b) taskkill /F mid-encode'
  Start-LongMp3 "$D\kill.mp3"
  $ff = Ffmpegs
  taskkill /F /PID (App).Id | Out-Null
  Start-Sleep 3
  Check 'kill-ffmpeg-gone' ((Ffmpegs).Count -eq 0) "ffmpeg pids before: $($ff.Id -join ',')"
  $p = Parts
  Check 'kill-only-residue-is-part' (-not (Test-Path "$D\kill.mp3")) ("residue: " + (($p | ForEach-Object { "$($_.Name) [$($_.Attributes)]" }) -join ', '))
  $p | Remove-Item -Force
  Start-App
}
if ($Collect) {
  New-Item -ItemType Directory -Force $Collect | Out-Null
  Copy-Item -Force "$D\call.m4a", "$D\call.mp3", "$D\long.mp3" $Collect -ErrorAction SilentlyContinue
  "collected outputs in $Collect"
}
