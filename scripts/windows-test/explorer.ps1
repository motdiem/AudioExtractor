<#
Lists open File Explorer windows with their folder and selected items, to
check "Show in Explorer". -Close closes those showing Downloads\AETest.
#>
param([switch]$Close)
$dir = Join-Path $env:USERPROFILE 'Downloads\AETest'
foreach ($w in (New-Object -ComObject Shell.Application).Windows()) {
  try { $folder = $w.Document.Folder.Self.Path } catch { continue }
  $sel = @($w.Document.SelectedItems() | ForEach-Object { $_.Name }) -join ', '
  "explorer: $folder | selected: $sel | visible: $($w.Visible)"
  if ($Close -and $folder -like "$dir*") { $w.Quit() }
}
