# Architecture

AudioExtractor is a small desktop app for macOS and Windows: drop a Zoom recording on the window, get its audio as M4A or MP3. This document is the map of the code: what lives where, how a job flows through it, and the few rules that hold it together. For the story of *why* each piece looks the way it does, follow the links to the [tutorial](docs/tutorial/README.md).

## The big picture

```
 OS hands us files                 internal/launch
 (Dock drop, Open with,  ───────►  Filter → 300 ms debounce ──┐
  exe args, 2nd launch)            → wait for "ready"         │
                                                              ▼
 window drop (Wails event) ────────────────────────►  extract.Service  ───►  Port (internal/desktop)
                                                      all the behaviour       dialogs, reveal, clipboard
 frontend (TypeScript)  ── bound methods ──────────►  Pick, Run, Cancel  ───►  ffmpeg, a separate process
 a dumb view            ◄─ "extract:state" events ──  ...                      embedded, unpacked once
                                                          │
                                         i18n catalogs ◄──┴──► settings.json
```

Three rules shape everything:

1. **Go owns the behaviour, the window only shows it.** `extract.Service` decides what happens; the frontend calls a handful of bound methods and swaps screens when the service emits an `extract:state` event. There is no logic in TypeScript worth testing.
2. **Every way of giving the app files ends in the same call.** A window drop, the file picker, a Dock drop, "Open with", files dropped on the exe and a second launch all reach `Service.TryRun` / `Service.Run` with a list of paths.
3. **ffmpeg is a separate process we ship ourselves.** A slim, LGPL-only build is embedded in the Go binary, unpacked to the user cache on first use, and run with `os/exec`. Nothing is installed, nothing needs admin rights.

## Repository layout

| Path | What it is |
| --- | --- |
| [`main.go`](main.go) | Wiring only: builds the service, the launcher and the window, and connects Wails events to them. |
| [`notices.go`](notices.go) | Embeds `THIRD_PARTY_NOTICES.txt` and `third_party/licenses/` (it must sit at the root because `go:embed` cannot reach up). |
| [`internal/extract`](internal/extract) | The application service: inputs, naming, running ffmpeg, progress, cancel, error classification, batch summaries. The core of the app and of the tests. |
| [`internal/desktop`](internal/desktop) | The real `extract.Port`: Wails native dialogs, "Show in Finder / Explorer", clipboard, message box. |
| [`internal/ffmpeg`](internal/ffmpeg) | Embeds the platform's ffmpeg and unpacks it to `ffmpeg-<sha256>/` in the user cache (ad-hoc signing it on macOS). |
| [`internal/launch`](internal/launch) | Turns OS-delivered files into batches: `Filter`, a 300 ms `Coalescer`, and a `Launcher` gate that waits for the frontend. |
| [`internal/i18n`](internal/i18n) | EN and FR catalogs (`locales/*.json`), locale detection per OS, plurals, per-OS key variants (`key@mac`, `key@windows`). |
| [`internal/settings`](internal/settings) | A tiny JSON key/value store in the user config folder. A missing or corrupt file reads as empty. |
| [`internal/window`](internal/window) | Remembers the window position (clamped to the screens connected now) and follows system light/dark. |
| [`internal/webview2`](internal/webview2) | Windows only: checks the WebView2 Runtime before opening a window, and explains with a native message box if it is missing. |
| [`internal/notices`](internal/notices) | The "Licences" link: writes the notice and every licence text to a temp file and opens it. |
| [`frontend/`](frontend) | Plain TypeScript + Vite, no framework. `src/main.ts` renders the five screens; `src/preview.ts` is a dev-only screen preview. |
| [`build/`](build) | Wails build config, per-OS Taskfiles, `Info.plist`, icons, Windows manifest. |
| [`scripts/ffmpeg/`](scripts/ffmpeg) | Builds the slim ffmpeg from pinned, checksummed sources (macOS universal and Windows x64, both from a Mac). |
| [`scripts/release/`](scripts/release) | Packs the two zips, checks the Windows PE imports, verifies and checksums the release. |
| [`scripts/windows-test/`](scripts/windows-test) | Drives the real exe in a Parallels Windows VM from the Mac, one UI step per call. |
| [`scripts/screenshots/`](scripts/screenshots) | Regenerates the README screenshots from the dev-only screen previews, in light and dark. |
| [`docs/`](docs) | The tutorial, the release checklist, the first-launch notes shipped in the zips, the README screenshots, and agent docs. |
| [`third_party/licenses/`](third_party/licenses) | Full licence texts of everything compiled or bundled in. |

## Life of a job

```
paths (drop, picker, launch args)
  → expandInputs        folders → their top-level media files, deduplicated, sorted
  → 0 items             "empty folder" error
    1 item              native Save dialog, next to the source, name pre-filled
    2+ items            native folder dialog; Zoom names and collision suffixes
  → for each item       probe (ffmpeg -i) → extract to a hidden .part file → rename
  → done screen         or an error screen, or a summary for a batch
```

- **One job at a time.** `Service` holds a `busy` mutex for the whole job, dialogs included. A drop while busy shows a short native notice instead of queueing. After `ServiceShutdown` (quit) nothing new starts.
- **Extraction** runs `ffmpeg -vn -sn -dn -map 0:a:0` and either copies the AAC track into an M4A (`-c:a copy`, near instant), re-encodes non-AAC audio to AAC, or encodes MP3 with LAME at 128 kbps. See [`internal/extract/ffmpeg.go`](internal/extract/ffmpeg.go).
- **Progress** comes from `-progress pipe:1` against the duration from the probe; `-1` means indeterminate. The working screen only appears after 500 ms so quick jobs never flash.
- **Atomic output.** ffmpeg writes to `.audioextractor-*.part` in the destination folder, which is renamed into place only on success. Cancel, an error or a quit removes it, so a destination never holds a half-written file.
- **Cancel and quit** cancel the job's context, which kills ffmpeg. On quit, `ServiceShutdown` waits up to 2 s for clean-up. On Windows every ffmpeg also joins a Job Object that kills it if the app dies hard.
- **Errors are classified, never just "failed".** [`classify.go`](internal/extract/classify.go) combines Go pre-checks (can we read the source, can we write next to the destination), ffmpeg's diagnostic lines, and sniffing the first 4 KB, into one `ErrorKind` (`no-audio`, `unreadable`, `not-media`, `destination-unwritable`, `output-locked`, `unsupported-format`, ...). Each kind has a plain message in both languages, and **Copy details** puts the raw ffmpeg log and the Go error on the clipboard.

## The boundary with the window

The frontend talks to Go in two ways, both from Wails:

- **Bound methods**, through generated bindings in `frontend/bindings/` (git-ignored, regenerated by `wails3 generate bindings`): `Pick`, `SetFormat`, `Cancel`, `Reveal`, `CopyDetails`, `Messages`, `Version`, and `notices.Service.Open`.
- **Events**: Go emits `extract:state` with a `StateEvent` (`phase` is `idle`, `working`, `done`, `error` or `summary`), and the page emits `frontend:ready` once its listeners exist, which opens the launch gate.

Dropped files never go through JavaScript: Wails delivers them to Go (`WindowFilesDropped`), which calls the service directly. Text is never built in TypeScript either: the service sends the catalog and already-resolved strings (plurals included), so the two languages are handled in one place and tested there.

## Seams and tests

`extract.Service` gets everything it must not hard-code through `extract.Config`:

| Collaborator | Production | Tests |
| --- | --- | --- |
| `Port` (dialogs, reveal, clipboard, notices) | `internal/desktop` on Wails | a fake that returns canned paths and records what it was asked |
| `Locate` (where ffmpeg is) | the embedded build; `PATH` in a dev build made without it | `$AUDIOEXTRACTOR_FFMPEG` or `PATH`, skipped with a message if neither exists |
| `Emit` (events to the window) | `application.Event.Emit` | a recorder |
| `Settings` | `settings.json` in the user config folder | a temp file, or `nil` |

The tests run the **real ffmpeg** on short clips generated at test time, so they check real output, real errors and real cancellation, not mocks of them. Pure pieces (input expansion, naming, classification, window placement, launch coalescing, plurals, catalog parity) have plain unit tests. Platform behaviour lives in build-tagged files (`*_windows.go`, `*_darwin.go`, `*_other.go`), each with its own tests where it matters.

What Go tests cannot see is covered by [`scripts/windows-test/`](scripts/windows-test), which drives the real exe in a Windows VM, and by the [release checklist](docs/manual-test-checklist.md).

## Platform differences

| | macOS | Windows |
| --- | --- | --- |
| Webview | WKWebView | WebView2 (checked at start by `internal/webview2`) |
| ffmpeg | universal `ffmpeg`, the arm64 or amd64 slice embedded per build, ad-hoc signed when unpacked | static x64 `ffmpeg.exe`, cross-compiled with mingw-w64 |
| Files from the OS | one `ApplicationOpenedWithFile` event per file; `CFBundleDocumentTypes` in `Info.plist` | `os.Args`, plus a single-instance handoff from a second launch |
| Hidden `.part` file | leading dot | `FILE_ATTRIBUTE_HIDDEN`, cleared before the rename |
| Killing ffmpeg on a crash | not available | Job Object with `KILL_ON_JOB_CLOSE` |
| Licences | copied into the `.app` before signing, and embedded | embedded in the exe (single file), and next to it in the zip |

## Build and release

Everything is built on one Mac, with no CI. [`Taskfile.yml`](Taskfile.yml) is the single entry point:

- `wails3 task dev`: the app with hot reload.
- `wails3 task ffmpeg:mac` / `ffmpeg:windows`: build the slim ffmpeg from the pins in [`scripts/ffmpeg/config.sh`](scripts/ffmpeg/config.sh). The `:ensure` variants rebuild only when the scripts changed.
- `wails3 task release`: both ffmpegs if needed, the universal ad-hoc-signed `.app`, the Windows exe (`CGO_ENABLED=0`), both zips with their first-launch notes and notices, a verification pass, and `dist/SHA256SUMS.txt`. The version comes from `git describe`.

## Where to read more

The [tutorial](docs/tutorial/README.md) explains each part in depth, one chapter per ticket:

- Build pipeline and ffmpeg: chapters [1](docs/tutorial/01-scaffold-and-local-builds.md), [2](docs/tutorial/02-slim-ffmpeg-for-macos.md), [3](docs/tutorial/03-cross-compiling-ffmpeg-for-windows.md) and [12](docs/tutorial/12-shipping-it.md).
- The extraction service: chapters [4](docs/tutorial/04-first-extraction-m4a.md) (design and test seam), [5](docs/tutorial/05-mp3-progress-cancel.md) (progress, cancel, atomic write), [6](docs/tutorial/06-errors-people-understand.md) (error classification) and [7](docs/tutorial/07-batches-and-zoom-names.md) (batches and naming).
- The desktop shell: chapters [8](docs/tutorial/08-launch-with-files.md) (launch with files) and [9](docs/tutorial/09-window-polish-and-icon.md) (window, theme, icon).
- Testing and hardening: chapters [10](docs/tutorial/10-testing-windows-from-a-mac.md) and [11](docs/tutorial/11-hardening-from-a-code-review.md).
