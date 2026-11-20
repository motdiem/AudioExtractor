# Release checklist

Run this on the release build (`env -u GOPATH wails3 task release`) before sending it out: about 10 minutes per OS. Automated tests (`env -u GOPATH go test ./...`, `npm run check` in `frontend/`) must pass first.

**[VM]** marks the items `scripts/windows-test/` already drives in the Windows VM (see `docs/tutorial/10-testing-windows-from-a-mac.md`). Look at their screenshots rather than redoing them. Everything else is done by hand.

## Test files

On the Mac (Homebrew ffmpeg), in a folder you can reach from the VM (`~/AEWinTest/media` is `\\Mac\Home\AEWinTest\media`):

```sh
ffmpeg -f lavfi -i sine=d=120 -f lavfi -i color=c=blue:s=320x240:r=10:d=120 -c:a aac -c:v libx264 -pix_fmt yuv420p -shortest call.mp4
ffmpeg -f lavfi -i sine=d=1800 -f lavfi -i color=c=green:s=64x64:r=1:d=1800 -c:a aac -c:v libx264 -pix_fmt yuv420p -shortest long.mp4
ffmpeg -f lavfi -i color=c=red:s=320x240:r=10:d=5 -c:v libx264 -pix_fmt yuv420p silent.mp4
head -c 60000 call.mp4 > truncated.mp4; echo "not a video" > notes.mp4
mkdir batch; cp call.mp4 batch/a.mp4; cp call.mp4 batch/b.mp4; cp truncated.mp4 batch/c-broken.mp4
mkdir "2026-11-20 10.00.00 Weekly sync"; cp call.mp4 "2026-11-20 10.00.00 Weekly sync/video1234567890.mp4"
```

## Both OSes

- [ ] First launch from a download (zip on macOS, exe in Downloads on Windows) with the first-launch note: macOS right-click > Open; Windows "More info" > "Run anyway". The window opens, about 420×320, with the version shown.
- [ ] **[VM]** Drop zone click > `call.mp4` > Save dialog in the source folder, named `call.m4a` > Done in about a second. The file plays.
- [ ] **[VM]** Same with MP3: the progress bar appears only on long files, and the MP3 plays.
- [ ] Drag `call.mp4` from Finder / Explorer onto the window: the zone highlights, and the Save dialog follows.
- [ ] **[VM]** `long.mp4` to MP3, then Cancel: back to the drop zone, no output and no hidden `.audioextractor-*.part` file left.
- [ ] **[VM]** Batch `a.mp4`, `b.mp4`, `c-broken.mp4`: one folder dialog, then "2 done, 1 failed" with the damaged-file sentence for `c-broken.mp4`.
- [ ] **[VM]** Zoom folder: the name is `2026-11-20 10.00.00 Weekly sync.m4a`. By hand: drop the folder itself on the window.
- [ ] `silent.mp4` says there is no sound; `notes.mp4` says it is not a recording.
- [ ] **[VM]** Show in Finder / Show in Explorer opens the folder with the output selected, in front.
- [ ] **[VM]** Copy details, then paste: version, OS, file, error kind and the ffmpeg log.
- [ ] **[VM]** Cancel each dialog (picker, Save, folder): back to the drop zone, no error.
- [ ] Save over an existing file: the system asks "Replace?"; No changes nothing.
- [ ] French system language: every screen and the busy message are in French.
- [ ] Dark mode: the window follows it.
- [ ] Quit, relaunch: the format choice and the window position are remembered.

## macOS only

- [ ] App not running: `open -a AudioExtractor.app t1.mp4` opens the Save dialog; two files give one folder dialog.
- [ ] Drag a file onto the Dock icon, app running and not running: same result. A folder on the Dock icon is expanded.
- [ ] Finder > right-click a `.mp4` > Open With lists AudioExtractor, which is not the default app.
- [ ] A job running: drop another file on the Dock icon: "Busy" message, the job continues.
- [ ] Runs on Apple Silicon and on Intel (or Rosetta: `arch -x86_64 open ...`).

## Windows only

- [ ] Run as a standard (non-admin) user, from Downloads. (The VM user is an admin with a filtered token: **[VM]** covers standard rights, not a separate account.)
- [ ] **[VM]** Lock the output (`scripts/windows-test/lock.ps1`, or open the M4A in a player that locks it), overwrite it through the app: "A file with that name is open in another program...".
- [ ] **[VM]** No console window flashes while ffmpeg runs (look at the MP3 progress screen).
- [ ] App not running: drag `call.mp4` onto `AudioExtractor.exe`: the window opens, then the Save dialog. Two files: one folder dialog. (**[VM]** covers the same path with command-line arguments.)
- [ ] Open with > Choose another app > AudioExtractor: same.
- [ ] **[VM]** App running: launch it again with a file: no second window, the running one shows the Save dialog.
- [ ] **[VM]** App busy: launch it again with a file: "Busy" message, the second process exits, the job continues.
- [ ] **[VM]** Close the window during an MP3: no `ffmpeg.exe` left, no `.part` file left. `taskkill /F` on the app also ends ffmpeg; only a hidden `.part` remains.
- [ ] WebView2 missing (dev build only): `scripts/windows-test/build-test-exe.sh --dev`, then `vm.sh start -Env AUDIOEXTRACTOR_FAKE_NO_WEBVIEW2=1`: a French or English message box with the download link; Yes opens it; the app exits.
