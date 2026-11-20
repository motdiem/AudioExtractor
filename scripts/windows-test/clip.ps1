# Prints the clipboard text (to check "Copy details"). -Clear empties it first.
param([switch]$Clear)
if ($Clear) { Set-Clipboard -Value $null; 'clipboard cleared'; return }
$t = Get-Clipboard -Raw
if (-not $t) { 'clipboard is empty'; return }
"clipboard: $($t.Length) chars"
$t
