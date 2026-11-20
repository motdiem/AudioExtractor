# Lists a folder including hidden files, with attributes (to see the
# temporary .audioextractor-*.part file and its Hidden flag).
param([Parameter(Mandatory)][string]$Path)
Get-ChildItem -Force -LiteralPath $Path -File |
  Select-Object Name, Length, Attributes | Format-Table -AutoSize | Out-String -Width 200
