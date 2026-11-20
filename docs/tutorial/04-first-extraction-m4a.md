# 4. First extraction: drop an MP4, get an M4A

Goal of this chapter: the first complete flow on macOS. Drop a file on the window (or click to pick one), a native Save dialog opens next to the source with the name pre-filled, the audio track is copied into an `.m4a`, and the window says "Done" with a **Show in Finder** button. The UI speaks French or English following the OS.

(Chapter 3 is the embedded Windows ffmpeg; chapters 3 and 4 were built in parallel, hence the gap.)

## The design in one picture

```
 window (TypeScript)           Go
 ───────────────────           ──────────────────────────────────────────
 click zone ──Pick()────────►  extract.Service ──► Port.ChooseInputs
 drop file ──(Wails event)──►  Service.Run     ──► Port.ChooseSaveFile
                                     │           └► ffmpeg (injected locator)
 show screen ◄──"extract:state"──────┘
 Show in Finder ──Reveal()──►  Port.Reveal
```

All the behaviour lives in one Go type, `extract.Service` (`internal/extract`). The frontend is a dumb view: it calls `Pick()` / `Reveal()` and swaps screens when the service emits an `extract:state` event (`working`, `done`, `error`, `idle`). Dropped files never go through JavaScript: Wails hands them to Go, which calls `Service.Run` directly.

## The test seam

The service is the one thing we test, and it needs two things it must not hard-code:

1. **Native dialogs and "reveal"**: a small `Port` interface (`ChooseInputs`, `ChooseSaveFile`, `Reveal`). `internal/desktop` implements it with Wails dialogs; the tests use a fake that returns canned paths and records what it was asked (which folder, which pre-filled name).
2. **ffmpeg's location**: a `Locate func() (string, error)` passed to `extract.New`. Production will pass the embedded build (`ffmpeg.Path()`, chapter 3); until that lands, `main.go` uses `exec.LookPath("ffmpeg")`. *Update: released apps always use the embedded ffmpeg; `PATH` is only the fallback of a dev build made without it. The tests still use `$AUDIOEXTRACTOR_FFMPEG` or `PATH` (Homebrew's ffmpeg), and chapter 11 runs them a second time against the slim build.* Tests use `$AUDIOEXTRACTOR_FFMPEG` or `PATH`, and skip with a clear message if neither exists.

Tests run the **real ffmpeg** on a 2-second clip generated at test time:

```sh
ffmpeg -f lavfi -i sine=frequency=440:duration=2 \
       -f lavfi -i color=c=blue:s=64x64:r=10:d=2 \
       -c:v mpeg4 -c:a aac clip.mp4
```

`mpeg4` and `aac` are built into every ffmpeg, so no extra codec libraries are needed. We verify the output by probing it with `ffmpeg -i out.m4a` (no `ffprobe` needed) and reading the stream listing: one `Audio: aac` stream, no `Video:`, brand `M4A`, about 2 seconds. A second test asserts the log contains `(copy)`, proving the AAC track is remuxed rather than re-encoded.

```sh
go test ./internal/...
```

## Extracting

```sh
ffmpeg -i src -vn -sn -dn -map 0:a:0 -c:a copy -f ipod out.m4a.part
```

- `-vn -sn -dn` drop video, subtitles and data; `-map 0:a:0` keeps the first audio track.
- `-c:a copy` is the remux: no decoding, so an hour of audio takes about a second.
- If the first audio stream is not AAC (we probe with `ffmpeg -i` first and read the codec), we use `-c:a aac -b:a 160k` instead.
- `-f ipod` forces the M4A-flavoured MP4 muxer, because the temporary file does not end in `.m4a`.
- The output goes to a temp file in the destination folder (`.audioextractor-*.part`) and is renamed into place on success, so a crash or failure never leaves a half-written `.m4a`. A deferred remove cleans the temp file on every failure path.

**Pitfall:** `ffmpeg -i file` with no output prints the streams and then exits with an error ("At least one output file must be specified"). That is expected; we only parse the text.

## Wails v3 APIs used

- **File drop.** Create the window with `EnableFileDrop: true`, mark the target element with `data-file-drop-target`, and listen in Go:
  ```go
  win.OnWindowEvent(events.Common.WindowFilesDropped, func(e *application.WindowEvent) {
      go svc.Run(e.Context().DroppedFiles(), "m4a")
  })
  ```
  While a file hovers, Wails adds the class `file-drop-target-active` to the element; we style it for the highlight. **Pitfall:** run the work in a goroutine, as the handler is called on the UI path and a dialog or ffmpeg would freeze the window.
- **Dialogs.** `app.Dialog.OpenFile()...PromptForSingleSelection()` and `app.Dialog.SaveFile().SetDirectory(dir).SetFilename(name).AddFilter("M4A audio", "*.m4a").PromptForSingleSelection()`. Cancelling returns an empty string and no error. macOS shows its own "replace?" confirmation for the Save panel, so we write no overwrite logic.
- **Services and bindings.** `application.NewService(svc)` in `Options.Services`; every exported method is callable from TypeScript. `wails3 generate bindings -ts` (run by `wails3 task build`) writes typed wrappers to `frontend/bindings/` (git-ignored).
- **Events to the frontend.** `app.Event.Emit("extract:state", payload)` in Go, `Events.On("extract:state", ev => ev.data)` in TypeScript. The service receives an `Emit func(name string, data any)` so tests can record events too.

**Pitfall:** the native window is black behind the webview, so `style.css` sets an explicit background for light and dark (`prefers-color-scheme`).

## Localization from day one

`internal/i18n` embeds `locales/en.json` and `locales/fr.json`. A test (`TestCatalogsHaveIdenticalKeys`) fails if any key exists in only one language, is empty, or has different `{placeholders}`; another checks that every error kind has an `error.<kind>` message. The service serves its catalog to the frontend (`Messages()`), and the HTML marks translatable nodes with `data-i18n="key"`. Go also uses the catalog for error messages and dialog titles.

Language detection happens in Go, once, at startup:

- **macOS:** `defaults read -g AppleLanguages` (the first entry wins), then `AppleLocale`, then `LC_ALL`/`LANG`. **Pitfall:** an app launched from Finder has no `LANG`, which is why the preferences come first.
- **Windows:** `GetUserDefaultUILanguage` from `kernel32.dll` through `golang.org/x/sys/windows`; the low 10 bits `0x0c` mean French. This keeps the Windows cross-compile pure Go.
- Anything else: English.

## What is deliberately not here yet

One file at a time (dropping several shows a message), no MP3, no progress bar, a single generic error plus "no audio". Those arrive with the batch (#8), MP3/progress (#6) and error-kind (#7) tickets; the service already carries `Index`/`Total`/`Percent` on its state event and an `ErrorKind` on every outcome so they slot in without reshaping it.

> **Update:** all of it arrived: MP3 and progress in chapter 5, error kinds in chapter 6, batches in chapter 7.
