# 5. MP3 output, a progress bar and Cancel

Goal of this chapter: an **M4A | MP3** toggle that is remembered between launches, a progress bar with a **Cancel** button for jobs that take a while, and the guarantee that a cancelled job leaves nothing behind. M4A stays instant (a stream copy); MP3 is a real re-encode, so for the first time a job can take seconds or minutes.

The design from chapter 4 does not change: the Go `extract.Service` owns the behaviour, the window only shows `extract:state` events. We add to it.

## MP3 with ffmpeg

```sh
ffmpeg -i call.mp4 -vn -sn -dn -map 0:a:0 -c:a libmp3lame -b:a 128k -f mp3 out.mp3
```

- `-c:a libmp3lame -b:a 128k` is constant-bitrate 128 kbps.
- We pass no `-ac`: libmp3lame accepts mono and stereo, and ffmpeg keeps whichever the source has. The test generates a mono and a stereo clip and checks the output keeps each.
- `-f mp3` is explicit because the output file is a temporary `.part` file, whose extension ffmpeg cannot guess.

**Atomic write.** Both formats write to a temporary file in the destination folder (`.audioextractor-*.part`) and `os.Rename` it into place only when ffmpeg exits successfully. The destination therefore never holds a half-written file, whatever goes wrong. The same `defer os.Remove(tmp)` that cleans up after errors cleans up after Cancel.

Verification stays free of `ffprobe`: the test runs `ffmpeg -i out.mp3` and matches `Audio: mp3, 44100 Hz, stereo, ... 128 kb/s` in the stream listing.

## Reading progress from ffmpeg

Two sources, both plain text:

1. **Total duration**, from the probe we already run (`ffmpeg -i file`): a line like `Duration: 00:01:02.50, start: 0.0, ...`. A regexp turns it into a `time.Duration`. It may be `N/A` (some streams, broken files); then we report an indeterminate bar.
2. **Position**, from `-progress pipe:1`. This makes ffmpeg print `key=value` blocks on stdout about every half second:

```
out_time_us=15000000
...
progress=continue
```

and a final `progress=end`. We also pass `-nostats` so stderr stays a clean log (that is what "Copy details" will show in ticket #7).

`runWithProgress` starts ffmpeg, reads stdout line by line with a `bufio.Scanner`, and `Wait()`s when the pipe closes. Percent is `out_time_us / duration`, capped at 99 until `progress=end` arrives, so the bar never reads 100% while ffmpeg is still finalising. Pitfalls:

- `out_time_us` is `N/A` (or a huge negative number) before the first packet. Ignore anything that does not parse to a non-negative integer. (`out_time_ms` is a misnomer: it is also microseconds. Use `out_time_us`.)
- Do not call `cmd.Wait()` before you have finished reading from `StdoutPipe`: Wait closes the pipe.

**Throttling.** ffmpeg already reports about twice a second, but nothing guarantees that, and a Wails event is not free. The service emits at most one working event per 100 ms (`progressInterval`), except the final 100, which is never dropped.

## Cancel and cleanup

`Run` creates a `context.WithCancel`; `Cancel()` calls the cancel function (guarded by a mutex, because the window calls it from another goroutine). ffmpeg is started with `exec.CommandContext(ctx, ...)`, so cancelling the context kills the process, which closes stdout and ends the read loop. `extract` then sees `ctx.Err() != nil` and returns `KindCancelled`, the deferred `os.Remove` deletes the `.part` file, and the service emits `idle`: no error screen, no message. Nothing was ever written to the real destination, so there is nothing to remove there.

The service test cancels at the first progress event of a 20-minute generated clip (cheap to make as 16 kHz PCM, but about three seconds to encode), then asserts the result is `cancelled`, the last event is `idle`, and the folder is empty. Generating inputs that are *long enough to still be running* is the whole trick for testing cancellation without sleeps.

## The 500 ms rule

A progress bar that flashes for 80 ms on an M4A copy is worse than no bar. The service does not decide when to show it; the **window** does:

- On a `working` event it records the percent but stays on the current screen, and starts a 500 ms timer (once).
- If `done`, `error` or `idle` arrives first, the timer is cleared and the window goes straight there. The quick job never shows the bar.
- If the timer fires, the working screen appears with the bar (indeterminate while percent is -1) and the Cancel button.

So the idle screen is simply still visible for the first half second. All of this is about 20 lines in `frontend/src/main.ts`.

## Remembering the format: the settings store

`internal/settings` is deliberately tiny: a JSON file at `os.UserConfigDir()/AudioExtractor/settings.json` (`~/Library/Application Support/...` on macOS, `%AppData%\...` on Windows), read and written through typed keys:

```go
store := settings.New(path)
store.Set("format", "mp3")
var f string
ok := store.Get("format", &f)
```

Values are kept as `json.RawMessage`, so each feature owns its key and its Go type and the store never needs to know them. Ticket #10 will add a `"window"` key for the window position without touching the package. Choices worth copying:

- A missing or corrupt file reads as empty. Settings are a convenience; they must never stop the app from starting.
- `Set` rewrites the whole file through a temp file and a rename, like the audio output.
- The service takes the store through `Config.Settings` (nil is fine: the choice then lasts for the session), loads the format in `New`, and exposes `Format()` / `SetFormat()` to the frontend. The Go side stays the single source of truth; the toggle just mirrors it. Drag-and-drop and the picker call `Run(paths, "")`, where an empty format means "the remembered one". The format also decides the Save dialog's pre-filled name, extension and filter.

## Frontend chore: type-checking

`tsc --noEmit` failed because the Wails template pins TypeScript 4.9 while its `tsconfig.json` uses `"moduleResolution": "bundler"` and `"verbatimModuleSyntax"`, which need 5.x. We bumped the devDependency to `^5.9` and added `npm run check` (`tsc --noEmit`).

## Try it

```sh
env -u GOPATH go test ./...      # needs ffmpeg with libmp3lame on PATH, or AUDIOEXTRACTOR_FFMPEG
wails3 dev
```
