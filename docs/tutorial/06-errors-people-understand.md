# 6. Errors people understand, and Copy details

Goal of this chapter: when extraction fails, the window says in one plain sentence what is wrong and what to do next, in English or French, and a **Copy details** button gives us everything we need to debug it from a pasted email. Before this chapter every failure was "failed" (plus `no-audio`); ffmpeg's own words were thrown away.

The design stays the same: the Go `extract.Service` decides, the window shows. We add a classifier and a message per kind.

## The kinds

| Kind | What the user sees happened | Typical cause |
|---|---|---|
| `no-audio` | no sound | a video recorded without audio |
| `unreadable` | damaged or not finished | Zoom still converting, a raw `.zoom` file, a truncated download, an empty file |
| `not-media` | not a recording | a text file, an image, anything else |
| `destination-unwritable` | cannot save there | permission denied, disk full, read-only or unplugged volume, offline cloud folder |
| `output-locked` | name is in use | replacing a file another program has open (Windows sharing violation) |
| `cancelled` | nothing, silently | the user pressed Cancel |
| `failed` | fallback | anything we could not classify (ffmpeg missing, a bug) |

`failed` is deliberately the last resort, not a default: every time it shows up in a pasted log, that is a missing classification to add.

## Classification: evidence in a sensible order

`internal/extract/classify.go` holds pure functions; `extract()` in `ffmpeg.go` calls them in this order, cheapest and most certain first:

1. **Go pre-checks.** Can we open the source? Can we create the temporary file in the destination folder? Trying to create the real temp file beats `stat` plus permission bits, which lie on network shares, ACLs and read-only mounts. This runs before ffmpeg starts, so a bad destination never costs a probe.
2. **ffmpeg's verdict.** The probe (`ffmpeg -i file`) lists streams. No audio stream means `no-audio`. No streams at all, with `Invalid data found when processing input` or `moov atom not found` in the log, means ffmpeg could not parse the file.
3. **Sniffing, to split "broken" from "not media".** ffmpeg says the same thing for a truncated MP4 and for a text file. We read the first 4 KB ourselves:
   - a known signature (an `ftyp`/`moov`/`mdat` box, `RIFF`, `ID3`, `OggS`, `fLaC`, Matroska) means it is real media that is damaged: `unreadable`;
   - zero bytes, or a `.zoom` extension, is also `unreadable`;
   - valid UTF-8 text is `not-media`, whatever the extension (a text file renamed `.mp4`);
   - binary garbage is `unreadable` when the extension says audio or video, `not-media` otherwise.
4. **Go errors from writing.** `classifyWriteError(err, replacing)` uses `errors.Is`: `os.ErrPermission`, `os.ErrNotExist` (the folder disappeared), `syscall.ENOSPC`, `syscall.EROFS` mean `destination-unwritable`. A lock error while replacing an existing file means `output-locked`.

### A subtle one: ffmpeg succeeds on half a file

Cut an MP4 in half and ffmpeg's behaviour depends on where its index (the `moov` atom) is:

- Normal MP4 (index at the end): `moov atom not found`, exit 183. Easy.
- "Faststart" MP4 (index in front, which web-optimised files and many downloads are): ffmpeg opens it, logs `partial file` and `corrupt input packet`, **exits 0**, and writes the first half of the audio. Reporting success would hand someone half a meeting without telling them. So a successful run whose log contains those phrases is turned into `unreadable` and the output is discarded. The tests generate both layouts.

### Windows locks, without a Windows machine

`output-locked` is hard to produce on macOS. So the decision is a pure function and the test feeds it a synthetic errno (`syscall.EBUSY` on Unix, `Errno(32)`, `ERROR_SHARING_VIOLATION`, on Windows; a tiny `_test` file per OS defines which). Two build-tagged files keep the OS knowledge out of the logic and the Windows cross-compile pure Go (`syscall.Errno` needs no cgo):

- `lock_windows.go`: sharing violation (32), lock violation (33), and access denied (5), which is what `MoveFileEx` returns when the target is open elsewhere. Also `ERROR_DISK_FULL` (112).
- `lock_other.go`: `ETXTBSY` and `EBUSY`.

Access denied only counts as a lock when we were *replacing a file that existed*; creating the temp file in a read-only folder is `destination-unwritable`. That is why `classifyWriteError` takes a `replacing` flag.

## Writing error messages for non-technical people, twice

Rules we used, in both languages:

- **One sentence for what, one for what next.** "This file is damaged or not finished yet. If it comes from Zoom, wait for Zoom to finish converting the recording, then try again." The ticket's key case is Zoom: a recording is often opened seconds after the meeting, while Zoom is still converting.
- **No jargon.** No "stream", "codec", "moov", "errno". Those belong in Copy details.
- **Say what to do, not what failed**, and offer something they can do now (choose another folder such as Documents, close the other program, download again).
- **Do not blame.** "A file with that name is open in another program" rather than "you left it open".
- **Write French in French**, not translated English: `abîmé`, `pas encore terminé`, `Réessayez`; the typographic apostrophe `’` as in the existing catalog.
- **Keep them short.** The window is 420 x 320; the longest message must fit with its two buttons. Check the longest of both languages.

Each kind is a key `error.<kind>` in `locales/en.json` and `fr.json`. `MessageKinds` lists every kind that must have one, and the existing catalog test fails if a language is missing any (`cancelled` is silent, so it is not in the list).

## Copy details

The error screen gets a quiet link-style button next to **Try again**. It puts this on the clipboard:

```
AudioExtractor 1.2.3
OS: windows/amd64
File: Weekly sync.mp4
Error: unreadable
Cause: ...

--- ffmpeg log ---
<ffmpeg's raw output, probe and run>
```

Decisions:

- **The clipboard goes through Go**, not `navigator.clipboard`. The webview clipboard needs a user gesture, a secure context and sometimes a permission prompt, and behaves differently in WKWebView and WebView2. Wails v3 has `app.Clipboard.SetText`, which uses the native clipboard on each OS. It sits behind a new `Port.CopyText` method, so the service stays testable (the fake port records the text).
- **Go remembers the text**, set whenever an error is shown. `CopyDetails()` takes no argument; the frontend cannot get it wrong.
- **The header only names the file**, not its folder. The raw ffmpeg log is left untouched and may contain the path; it is what we need.
- **The app version** comes from `Config.Version`, set from `main.version` (default `dev`; release builds can pass `-ldflags "-X main.version=1.2.3"`). *Update: chapter 9 wired this to `git describe` in the build tasks.*
- **"Copied"** replaces the button label for two seconds, then it returns. If the copy fails there is nothing useful to tell the user, so we stay quiet.

## Tests

One service test per kind, each with a fixture generated at test time by ffmpeg itself (nothing binary is committed): a video with no audio, a truncated MP4 (index at the end, and faststart), an empty file, a raw `.zoom` file, a text file renamed `.mp4`, a read-only destination folder (`chmod 0555`; skipped as root and on Windows), a vanished destination folder, and a cancel that fires the moment work starts. Pure tests cover the errno mapping, the sniffing table and the log phrases.

```sh
env -u GOPATH go test ./...
```

## Pitfalls

- `chmod 0555` does not stop root. The test skips itself when it can still create a file.
- On macOS `os.CreateTemp` into a missing folder gives `ENOENT`, not `EACCES`; treat both as "cannot save there".
- Do not classify by ffmpeg exit code alone: 183 (`AVERROR_INVALIDDATA`) is also what a text file gets.
- Keep the Windows cross-compile green: `GOOS=windows CGO_ENABLED=0 go build .`
