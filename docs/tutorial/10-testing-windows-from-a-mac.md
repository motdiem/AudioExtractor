# 10. Testing on Windows from a Mac

Goal of this chapter: run the real Windows exe through every flow (single file, batch, Zoom folder, cancel, Show in Explorer, Copy details, locked output, launch with files) in a Windows 11 VM, driven from the Mac's terminal, with a screenshot of each screen. And a short manual checklist for what a script can't prove.

This found three bugs that every Go test had missed, all three Windows-only, and one missing feature (the WebView2 message).

## The setup

- Parallels Desktop with a Windows 11 ARM VM named "Windows 11". It runs our x64 exe through Windows' emulation.
- The Mac home folder is visible in the VM as `\\Mac\Home`, so the repo's scripts and the built exe need no copying step.
- `prlctl exec "Windows 11" --current-user ...` runs a command as the logged-in user, so windows appear on the VM's desktop. Without `--current-user` it runs as SYSTEM in session 0, where nothing is visible.
- `prlctl capture "Windows 11" --file x.png` takes a screenshot of the VM's screen.

Everything lives in `scripts/windows-test/`:

| Script | What it does |
| --- | --- |
| `vm.sh <script> args...` | Runs `<script>.ps1` from this folder in the VM through `\\Mac\Home`. `vm.sh shot x.png` takes a screenshot. |
| `build-test-exe.sh [--dev]` | Builds the release exe with its own single-instance ID (see the pitfalls). `--dev` drops `-tags production`. |
| `setup.ps1` | Copies the exe and the test media into `Downloads\AETest`, the way a user would run it. |
| `start.ps1 [paths]` | Starts the exe, with files or folders as arguments, as if they were dropped on it. |
| `ae.ps1 <step>` | One UI step: `screen`, `click <id>`, `format MP3`, `open <dir> <names>`, `save <path>`, `folder <dir>`, `ok`, `cancel`, `replace yes`, `msgbox`, `wait <text>`, `waitfile <path>`, `front`. |
| `lock.ps1`, `clip.ps1`, `explorer.ps1`, `ls.ps1`, `procs.ps1` | Hold a file open, read the clipboard, list Explorer windows and their selection, list hidden files, list processes. |
| `stop.ps1 [-Clean]` | Closes our copy of the app (never another one) and ends leftover script processes. |

One step per call is deliberate: the Mac side decides what comes next and takes a screenshot in between. A flow reads like this:

```sh
V=scripts/windows-test/vm.sh
D='C:\Users\ziad\Downloads\AETest\media'
$V ae format MP3
$V ae click drop                       # the drop zone opens the file picker
$V ae open "$D" call.mp4               # go to the folder, select the file
$V ae save "$D\call.mp3"               # Save dialog
$V ae waitfile "$D\call.mp3"
$V ae wait Explorer                    # "Show in Explorer" is on screen: done
$V shot /tmp/done.png
```

## Two ways into the app

**The web UI, through UI Automation.** WebView2 exposes the page to Windows' accessibility API. A `<button id="drop">` becomes an element with `AutomationId` `drop` and the `Invoke` pattern; the format buttons (`role="radio"`) get `SelectionItem`. Elements of hidden screens (`hidden` attribute) are not in the tree, so `ae screen` lists exactly what is on screen. That makes UIA a good assertion tool too: "the error screen shows the output-locked sentence" is a text search.

**Native dialogs, through Win32 messages.** Under emulation, UIA sees the common file dialogs as anonymous panes. Plain window messages work:

- the file-name box is an `Edit`: control id 1001 in the Save dialog, an `Edit` inside the combo box 1148 in the Open dialog, and id 1152 in the folder picker ("Folder:");
- `WM_SETTEXT` fills it, `WM_COMMAND IDOK` presses the default button, `IDCANCEL` cancels;
- a folder path in the Open dialog plus OK navigates into the folder; then `"a.mp4" "b.mp4"` selects several files;
- in the folder picker, the first OK navigates into the folder you typed and a second one selects it;
- "Do you want to replace it?" is a task dialog: `TDM_CLICK_BUTTON` with `IDYES` (6) or `IDNO` (7);
- a message box with a lone OK button closes on `IDCANCEL`, not `IDOK`.

## What we checked, and what we found

| Flow | How it is checked |
| --- | --- |
| Single file to M4A, then MP3 | `waitfile`, then the done screen's text |
| Batch of three, one truncated | the summary text, `dir` of the output folder |
| Zoom folder | `ok` accepts the pre-filled name; the file must be `2026-11-20 10.00.00 Weekly sync.m4a` |
| Cancel a long MP3 | `click cancel`, then `ls.ps1`: no `.part` left |
| Show in Explorer | `explorer.ps1`: folder, selected item, and **visible** |
| Copy details | `clip.ps1` (`Get-Clipboard`) |
| Locked output | `lock.ps1`, then overwrite through the app: the error text |
| Launch with a file, second launch forwarding, Busy | `start.ps1 <path>`, `procs.ps1` (the second process exits), `msgbox` |
| Console window flash | a screenshot while ffmpeg runs, plus `procs.ps1` showing ffmpeg's command line |

What we found:

1. **"Show in Finder" on Windows.** The catalogs now allow per-OS variants, `done.show@mac` and `done.show@windows`; `i18n.Load` keeps the one for the running OS under the plain key, so the frontend did not change. A test compares the raw files so both variants exist in both languages.
2. **Show in Explorer opened an invisible window.** We started `explorer.exe /select,<file>` with `HideWindow: true`, copied from the ffmpeg code to avoid a console flash. Explorer honours the `SW_HIDE` this puts in `STARTUPINFO`: the folder opened, the file was selected, and the window stayed hidden. `explorer.ps1` showed `visible: False`. Explorer is a GUI program, so the flag goes.
3. **Cancelling any dialog showed an error.** On macOS, Wails returns an empty path when you cancel. On Windows it returns an error, "cancelled by user", from an internal package we can't import. The service saw an error and said "The audio could not be extracted from this file". `desktop.noCancelError` maps that error back to an empty result.
4. **A missing WebView2 runtime failed silently.** See below.
5. Not an app bug, but worth knowing: our own `start.ps1` first passed the file to `-Env`, its first positional parameter, so "launch with a file" seemed broken until `procs.ps1` showed the command line had no argument. Check the process's command line before you blame the app.

## WebView2 missing

Windows 11 ships the WebView2 Runtime, but Windows 10 machines and some locked-down PCs don't have it. Without it Wails can't create the window, and the app just doesn't appear.

We can't uninstall the runtime from the VM safely, so the code is split so that the decision can be tested without it:

- `webview2.Installed(lookup)` checks the registry keys Microsoft documents, `Software\Microsoft\EdgeUpdate\Clients\{GUID}` value `pv`, and the `ClientState\{GUID}` value `EBWebView` that the Wails loader itself reads, for the runtime and the Beta/Dev/Canary channels, per machine (32-bit view) and per user. `lookup` is a function, so unit tests use a map.
- `webview2.Missing(lookup, getenv)` adds an override: `AUDIOEXTRACTOR_FAKE_NO_WEBVIEW2=1` reports "missing". It is honoured only without `-tags production`, through a variable set in two build-tagged files. Release builds ignore it.
- `webview2.Ensure(lang)`, Windows only, runs first in `main`. If the runtime is missing, it shows `MessageBoxW` (through `golang.org/x/sys/windows`, no cgo) in French or English with the link `https://go.microsoft.com/fwlink/p/?LinkId=2124703`, opens the link with `ShellExecute` if you say Yes, and `main` returns.

To see the box: `build-test-exe.sh --dev`, then `start.ps1 -Env AUDIOEXTRACTOR_FAKE_NO_WEBVIEW2=1`.

## What can't be automated (here)

- **Real drag and drop.** UIA and window messages can't fake an OLE drag from Explorer. The scripts use the same code paths instead: the file picker for the window, and command-line arguments for drops on the exe. A human still drags once.
- **SmartScreen.** A file copied from the `\\Mac\Home` share has no Mark of the Web (`setup.ps1` prints it), so no warning appears. A downloaded exe gets "Windows protected your PC" once, with "More info", then "Run anyway". That needs a real download, so it's on the checklist.
- **A real non-admin account.** Our VM user is an administrator running with UAC's filtered token (`whoami /groups` shows `BUILTIN\Administrators ... Group used for deny only` and `Medium Mandatory Level`). The process has standard-user rights, which is what matters for writing to `%LocalAppData%` and the Downloads folder, but a separate standard account is still worth one manual run.
- **Look and feel.** A screenshot shows the screen; a person decides whether it looks right.

## Pitfalls

- **Each `prlctl exec` is slow.** A call costs seconds, and each `ae.ps1` call compiles its small C# helper again under x64 emulation. `e2e.ps1` runs the scripted checks inside the VM in a single call: single file to M4A and MP3, the hidden `.part` file, cancel, Busy, closing the window mid-encode, and `taskkill /F` mid-encode. It prints `PASS`/`FAIL` lines and copies the outputs back for `ffprobe` on the Mac. `-Only close,kill` picks sections.
- **Don't wait with `pgrep -f "prlctl exec"`.** The waiting shell's own command line contains the pattern, so the loop never ends.
- **One `prlctl exec` at a time.** While a long `prlctl exec` runs (our first `lock.ps1` held a file for 60 s), other calls queue behind it and seem to hang. Stopping the Mac-side `prlctl` cancels the job in the VM ("Canceling the job..."). `lock.ps1` now starts a hidden, detached PowerShell to hold the file and returns at once. Another agent or person using the VM at the same time has the same effect.
- **A leftover copy of the app steals the single instance.** A test build from an earlier ticket was still running with a Save dialog open. Every launch of our exe forwarded its arguments to that copy and exited. We don't close windows we didn't open, so `build-test-exe.sh` gives the test build its own ID with `-X main.singleInstanceID=...`.
- **Raise the app before a screenshot.** Other windows end up on top. `ae front` makes our window topmost for a moment, then normal again.
- **Don't use `start` through `prlctl exec`.** Chapter 3 explains why. `Start-Process -PassThru` returns at once.
- **PowerShell positional parameters.** See bug 5. `[CmdletBinding(PositionalBinding = $false)]` fixes it.
- **Quoting.** Pass paths as separate arguments to `powershell -File`, never through `cmd /c "..."`.

## Result

`scripts/windows-test/` drives every flow of the Windows build from the Mac, and `docs/manual-test-checklist.md` lists what remains manual, about 10 minutes per OS.
