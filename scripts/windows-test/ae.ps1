<#
Drives AudioExtractor's UI in a Windows VM, one step per call, so the Mac
side can take a screenshot between steps (see docs/tutorial/10-testing-windows-from-a-mac.md).

The web UI is reached through UI Automation (WebView2 exposes the page's
buttons with their HTML ids as AutomationId). The native file dialogs are
driven with plain Win32 messages, because UIA cannot see inside them when an
ARM64 PowerShell talks to the emulated x64 app.

  ae.ps1 screen                      list what is on screen (buttons, texts, dialogs)
  ae.ps1 front                       bring the app window to the front (before a screenshot)
  ae.ps1 click   <id>               press a button by its HTML id (drop, reveal, cancel...)
  ae.ps1 format  M4A|MP3             pick the output format
  ae.ps1 open    <dir> <name>...     fill the Open dialog: go to <dir>, select the names
  ae.ps1 save    <path>              fill the Save dialog
  ae.ps1 cancel                      cancel the open dialog
  ae.ps1 ok                       accept the open dialog as it is (pre-filled name)
  ae.ps1 folder  <dir>              fill the folder dialog
  ae.ps1 replace yes|no              answer "Do you want to replace it?"
  ae.ps1 msgbox                      print and dismiss a message box (the Busy one)
  ae.ps1 wait    <text> [seconds]    wait until some on-screen text contains <text>
  ae.ps1 waitfile <path> [seconds]   wait until a file exists and stops growing
#>
param([Parameter(Mandatory)][string]$Cmd, [Parameter(ValueFromRemainingArguments)][string[]]$Rest)
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName UIAutomationClient, UIAutomationTypes
$AE = [System.Windows.Automation.AutomationElement]
$TS = [System.Windows.Automation.TreeScope]
$P  = [System.Windows.Automation.PropertyCondition]
$True_ = [System.Windows.Automation.Condition]::TrueCondition

Add-Type @"
using System; using System.Text; using System.Runtime.InteropServices;
public static class Dlg {
  delegate bool EnumProc(IntPtr h, IntPtr l);
  [DllImport("user32.dll")] static extern bool EnumChildWindows(IntPtr p, EnumProc f, IntPtr l);
  [DllImport("user32.dll", CharSet=CharSet.Unicode)] static extern int GetClassName(IntPtr h, StringBuilder s, int n);
  [DllImport("user32.dll", CharSet=CharSet.Unicode)] static extern int GetWindowText(IntPtr h, StringBuilder s, int n);
  [DllImport("user32.dll")] static extern int GetDlgCtrlID(IntPtr h);
  [DllImport("user32.dll")] static extern IntPtr GetParent(IntPtr h);
  [DllImport("user32.dll")] static extern bool IsWindowVisible(IntPtr h);
  [DllImport("user32.dll", CharSet=CharSet.Unicode)] static extern IntPtr SendMessage(IntPtr h, uint m, IntPtr w, string l);
  [DllImport("user32.dll")] static extern IntPtr SendMessage(IntPtr h, uint m, IntPtr w, IntPtr l);
  [DllImport("user32.dll")] static extern IntPtr GetDlgItem(IntPtr h, int id);
  [DllImport("user32.dll")] static extern bool PostMessage(IntPtr h, uint m, IntPtr w, IntPtr l);
  static string Cls(IntPtr h) { var s = new StringBuilder(256); GetClassName(h, s, 256); return s.ToString(); }
  // The file-name Edit: id 1001 in the Save dialog, or an Edit inside the
  // combo box with id 1148 (cmb13) in the Open and folder dialogs.
  public static IntPtr FileNameEdit(IntPtr dlg) {
    IntPtr found = IntPtr.Zero;
    EnumChildWindows(dlg, (h, l) => {
      if (Cls(h) == "Edit" && IsWindowVisible(h)) {
        // 1001: Save dialog; 1152 (edt1): the folder picker's "Folder:" box.
        if (GetDlgCtrlID(h) == 1001 || GetDlgCtrlID(h) == 1152) { found = h; return false; }
        for (IntPtr p = GetParent(h); p != IntPtr.Zero && p != dlg; p = GetParent(p))
          if (GetDlgCtrlID(p) == 1148) { found = h; return false; }
      }
      return true;
    }, IntPtr.Zero);
    return found;
  }
  // Debug: every Edit in a dialog with its control id and its parents' ids.
  public static string Edits(IntPtr dlg) {
    var sb = new StringBuilder();
    EnumChildWindows(dlg, (h, l) => {
      if (Cls(h) == "Edit") {
        sb.Append("Edit id=" + GetDlgCtrlID(h) + " visible=" + IsWindowVisible(h) + " parents:");
        for (IntPtr p = GetParent(h); p != IntPtr.Zero && p != dlg; p = GetParent(p)) sb.Append(" " + Cls(p) + "#" + GetDlgCtrlID(p));
        sb.AppendLine();
      }
      return true;
    }, IntPtr.Zero);
    return sb.ToString();
  }
  delegate bool TopProc(IntPtr h, IntPtr l);
  [DllImport("user32.dll")] static extern bool EnumWindows(TopProc f, IntPtr l);
  [DllImport("user32.dll")] static extern uint GetWindowThreadProcessId(IntPtr h, out uint pid);
  // A visible top-level dialog (#32770) of process pid whose title matches.
  public static IntPtr FindTop(int pid, string titleRegex) {
    IntPtr found = IntPtr.Zero;
    EnumWindows((h, l) => {
      uint p; GetWindowThreadProcessId(h, out p);
      if (p == pid && IsWindowVisible(h) && Cls(h) == "#32770") {
        var s = new StringBuilder(512); GetWindowText(h, s, 512);
        if (System.Text.RegularExpressions.Regex.IsMatch(s.ToString(), titleRegex)) { found = h; return false; }
      }
      return true;
    }, IntPtr.Zero);
    return found;
  }
  // All static texts of a message box (its message).
  public static string Texts(IntPtr dlg) {
    var sb = new StringBuilder();
    EnumChildWindows(dlg, (h, l) => {
      if (Cls(h) == "Static" || Cls(h) == "DirectUIHWND") { var s = new StringBuilder(2048); GetWindowText(h, s, 2048); if (s.Length > 0) sb.AppendLine(s.ToString()); }
      return true;
    }, IntPtr.Zero);
    return sb.ToString();
  }
  public static void ClickTaskButton(IntPtr h, int id) { SendMessage(h, 0x0466, (IntPtr)id, IntPtr.Zero); } // TDM_CLICK_BUTTON
  public static void SetText(IntPtr h, string t) { SendMessage(h, 0x000C, IntPtr.Zero, t); }              // WM_SETTEXT
  public static void Ok(IntPtr dlg) { PostMessage(dlg, 0x0111, (IntPtr)1, GetDlgItem(dlg, 1)); }          // WM_COMMAND IDOK
  [DllImport("user32.dll")] static extern bool ShowWindow(IntPtr h, int cmd);
  [DllImport("user32.dll")] static extern bool SetForegroundWindow(IntPtr h);
  [DllImport("user32.dll")] static extern bool SetWindowPos(IntPtr h, IntPtr after, int x, int y, int cx, int cy, uint flags);
  // Raising another process's window: restore it, make it topmost for a
  // moment (z-order only: SWP_NOMOVE|SWP_NOSIZE), then normal again.
  public static void Front(IntPtr h) {
    ShowWindow(h, 9);
    SetWindowPos(h, (IntPtr)(-1), 0, 0, 0, 0, 0x0003);
    SetWindowPos(h, (IntPtr)(-2), 0, 0, 0, 0, 0x0003);
    SetForegroundWindow(h);
  }
  public static void Cancel(IntPtr dlg) { PostMessage(dlg, 0x0111, (IntPtr)2, IntPtr.Zero); }             // WM_COMMAND IDCANCEL
  public static void Close(IntPtr dlg) { PostMessage(dlg, 0x0010, IntPtr.Zero, IntPtr.Zero); }           // WM_CLOSE
}
"@

function App-Process {
  # Only the copy setup.ps1 installed: never touch another AudioExtractor.
  $dir = Join-Path $env:USERPROFILE 'Downloads\AETest'
  $p = Get-Process AudioExtractor -ErrorAction SilentlyContinue |
    Where-Object { $_.MainWindowHandle -ne 0 -and $_.Path -like "$dir\*" } | Select-Object -First 1
  if (-not $p) { throw "AudioExtractor from $dir is not running (or has no window)" }
  $p
}
function App-Window {
  $cond = New-Object $P($AE::ProcessIdProperty, (App-Process).Id)
  $w = $AE::RootElement.FindFirst($TS::Children, $cond)
  if (-not $w) { throw 'no UIA window for AudioExtractor' }
  $w
}
# Native dialogs (#32770) of the app: children of its window in the UIA tree,
# or top-level for message boxes.
function App-Dialogs {
  $cond = New-Object $P($AE::ProcessIdProperty, (App-Process).Id)
  foreach ($top in $AE::RootElement.FindAll($TS::Children, $cond)) {
    foreach ($w in @($top) + @($top.FindAll($TS::Children, $True_))) {
      if ($w.Current.ClassName -eq '#32770') { $w }
    }
  }
}
function Wait-Dialog([int]$Seconds = 20) {
  for ($i = 0; $i -lt $Seconds * 4; $i++) {
    $d = App-Dialogs | Select-Object -First 1
    if ($d) { return $d }
    Start-Sleep -Milliseconds 250
  }
  throw 'no dialog appeared'
}
function Fill-Dialog($dlg, [string]$text) {
  $h = [IntPtr]$dlg.Current.NativeWindowHandle
  $edit = [Dlg]::FileNameEdit($h)
  if ($edit -eq [IntPtr]::Zero) { throw "no file-name box in '$($dlg.Current.Name)'" }
  [Dlg]::SetText($edit, $text)
  [Dlg]::Ok($h)
  "dialog '$($dlg.Current.Name)' <- $text"
}
function By-Id($id) {
  $e = (App-Window).FindFirst($TS::Descendants, (New-Object $P($AE::AutomationIdProperty, $id)))
  if (-not $e) { throw "no element with id '$id' on screen" }
  $e
}
function Screen-Texts {
  $w = App-Window
  "window: $($w.Current.Name)"
  foreach ($e in $w.FindAll($TS::Descendants, $True_)) {
    $c = $e.Current
    if ($c.IsOffscreen) { continue }
    $t = $c.ControlType.ProgrammaticName -replace 'ControlType.', ''
    if ($t -in 'Button', 'Text', 'RadioButton', 'ProgressBar', 'ListItem' -and ($c.Name -or $c.AutomationId)) {
      $extra = ''
      if ($t -eq 'RadioButton') {
        $extra = ' selected=' + $e.GetCurrentPattern([System.Windows.Automation.SelectionItemPattern]::Pattern).Current.IsSelected
      }
      "  $t id='$($c.AutomationId)' '$($c.Name)'$extra"
    }
  }
  foreach ($d in App-Dialogs) { "  DIALOG '$($d.Current.Name)'" }
}

switch ($Cmd) {
  'screen' { Screen-Texts }
  'edits'  { foreach ($d in App-Dialogs) { "$($d.Current.Name):"; [Dlg]::Edits([IntPtr]$d.Current.NativeWindowHandle) } }
  'front'  { [Dlg]::Front((App-Process).MainWindowHandle); 'raised the app window' }
  'click' {
    (By-Id $Rest[0]).GetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern).Invoke()
    "clicked $($Rest[0])"
  }
  'format' {
    $r = (App-Window).FindFirst($TS::Descendants, (New-Object $P($AE::NameProperty, $Rest[0])))
    $r.GetCurrentPattern([System.Windows.Automation.SelectionItemPattern]::Pattern).Select()
    "format $($Rest[0])"
  }
  'open' {
    $dlg = Wait-Dialog
    Fill-Dialog $dlg $Rest[0]   # navigates into the folder
    Start-Sleep -Milliseconds 1200
    $names = ($Rest | Select-Object -Skip 1 | ForEach-Object { '"' + $_ + '"' }) -join ' '
    Fill-Dialog (Wait-Dialog) $names
  }
  'save'   { Fill-Dialog (Wait-Dialog) $Rest[0] }
  'cancel' { $d = Wait-Dialog; [Dlg]::Cancel([IntPtr]$d.Current.NativeWindowHandle); "dialog '$($d.Current.Name)' cancelled" }
  'ok'     { $d = Wait-Dialog; [Dlg]::Ok([IntPtr]$d.Current.NativeWindowHandle); "dialog '$($d.Current.Name)' accepted as is" }
  'folder' {
    $dlg = Wait-Dialog
    Fill-Dialog $dlg $Rest[0]
    # Typing a folder path and pressing Select Folder may only navigate into it;
    # if the dialog is still there, confirm the (now current) folder.
    Start-Sleep -Milliseconds 1200
    $still = App-Dialogs | Select-Object -First 1
    if ($still) { [Dlg]::Ok([IntPtr]$still.Current.NativeWindowHandle); 'confirmed current folder' }
  }
  'replace' {
    # Plain Win32 here: UI Automation calls hang while this modal task dialog
    # (a #32770 window of the app's process, "Confirm Save As") is up.
    $c = [IntPtr]::Zero
    for ($i = 0; $i -lt 40 -and $c -eq [IntPtr]::Zero; $i++) {
      $c = [Dlg]::FindTop((App-Process).Id, 'Confirm|Confirmer')
      if ($c -eq [IntPtr]::Zero) { Start-Sleep -Milliseconds 250 }
    }
    if ($c -eq [IntPtr]::Zero) { throw 'no replace confirmation' }
    $id = if ($Rest[0] -eq 'yes') { 6 } else { 7 }   # IDYES / IDNO
    [Dlg]::ClickTaskButton($c, $id)
    "replace? -> $($Rest[0])"
  }
  'msgbox' {
    $d = Wait-Dialog 10
    "message box '$($d.Current.Name)':"
    foreach ($e in $d.FindAll($TS::Descendants, $True_)) {
      if ($e.Current.Name) { "  $($e.Current.ControlType.ProgrammaticName -replace 'ControlType.', ''): $($e.Current.Name)" }
    }
    # A MessageBox with a lone OK button closes on IDCANCEL (like Esc).
    $h = [IntPtr]$d.Current.NativeWindowHandle
    [Dlg]::Cancel($h)
    Start-Sleep -Milliseconds 500
    if (App-Dialogs | Where-Object { $_.Current.NativeWindowHandle -eq $h }) { [Dlg]::Close($h) }
    'dismissed'
  }
  'wait' {
    $secs = if ($Rest.Count -gt 1) { [int]$Rest[1] } else { 30 }
    for ($i = 0; $i -lt $secs * 2; $i++) {
      $s = (Screen-Texts) -join "`n"
      if ($s -like "*$($Rest[0])*") { $s; return }
      Start-Sleep -Milliseconds 500
    }
    Screen-Texts
    throw "'$($Rest[0])' did not appear in $secs s"
  }
  'waitfile' {
    $secs = if ($Rest.Count -gt 1) { [int]$Rest[1] } else { 60 }
    $last = -1
    for ($i = 0; $i -lt $secs * 2; $i++) {
      if (Test-Path -LiteralPath $Rest[0]) {
        $len = (Get-Item -LiteralPath $Rest[0]).Length
        if ($len -gt 0 -and $len -eq $last) { "$($Rest[0]): $len bytes"; return }
        $last = $len
      }
      Start-Sleep -Milliseconds 500
    }
    throw "$($Rest[0]) did not appear in $secs s"
  }
  default { throw "unknown command '$Cmd'" }
}
