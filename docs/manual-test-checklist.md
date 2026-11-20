# Manual test checklist

Things automated tests cannot prove. Run them on a real build before a release. Tick them per platform.

## Launch with files (ticket #9)

Make a few small test files first: `ffmpeg -f lavfi -i sine=d=2 -f lavfi -i color=c=blue:s=64x64:d=2 -c:a aac -c:v libx264 -shortest t1.mp4` (and copy it to `t2.mp4`).

### macOS
- [ ] App not running: `open -a AudioExtractor.app t1.mp4` launches it and the Save dialog opens, pre-filled for `t1`.
- [ ] App not running: `open -a AudioExtractor.app t1.mp4 t2.mp4` gives one folder dialog (one batch), not two.
- [ ] App not running: drag a file onto the Dock icon (or the `.app` in Finder): same result.
- [ ] App running and idle: drop a file on the Dock icon: the Save dialog opens.
- [ ] App running and idle: Finder, right-click a `.mp4` > Open With > AudioExtractor. It is listed (under "Other" or the list) but is never the default app for `.mp4`.
- [ ] Drop a folder on the Dock icon: its media files are picked up.
- [ ] App busy (a job running, or a dialog open): drop another file: a "Busy" message appears and the current job is not disturbed.
- [ ] French system language: the busy message is in French.

### Windows
- [ ] App not running: drag `t1.mp4` onto `AudioExtractor.exe`: the window opens and the Save dialog follows.
- [ ] App not running: select two files, drag them onto the exe: one folder dialog.
- [ ] App not running: Open with > Choose another app > AudioExtractor (`.exe` anywhere): same.
- [ ] App already running (idle): drag another file onto the exe: no second window, the running window gets the Save dialog and comes to the front.
- [ ] App running and busy: drag another file onto the exe: "Busy" message, the second process exits.
