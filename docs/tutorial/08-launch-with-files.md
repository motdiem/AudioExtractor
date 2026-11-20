# 8. Launch with files: Dock icon, exe drop, "Open with"

Goal of this chapter: files that the operating system hands to the app, by dropping on the Dock icon or the `.exe`, or through "Open with...", take the same road as a window drop: straight to the Save or folder dialog. It works whether the app is already running or not.

## The idea

Every source ends in `svc.Run(paths, "")`. The only new work is collecting the paths, which arrive in four different ways:

| Source | How the paths arrive |
| --- | --- |
| macOS, cold or running | One `ApplicationOpenedWithFile` Wails event **per file** (the delegate's `application:openFile:`) |
| Windows, cold launch | `os.Args[1:]` |
| Windows, app already running | A second process starts, forwards its args to the first one, and exits (Wails `SingleInstance`) |
| Window drop | Unchanged from chapter 4 |

The new package `internal/launch` is three small pieces, all pure enough to unit test:

- `Filter(args, exists)`: drops flags (anything starting with `-`, such as macOS `-psn_...`) and paths that do not exist.
- `Coalescer`: merges `Add` calls made within 300 ms into one batch. macOS sends one event per file, and a batch must stay a batch: two files should give one folder dialog, not two competing ones.
- `Launcher`: a coalescer plus a gate. Batches wait until the frontend says it is ready, then go out together.

## Why a "ready" gate

On a cold launch the open-file event arrives almost at once, before the webview has loaded. `svc.Run` emits state events the page must be listening for, so delivering early would lose the first screens. The page calls `Events.Emit("frontend:ready")` at the very end of `main()`, after its listeners exist, and Go opens the gate on it. No generated binding is involved: a plain event keeps the dependency small.

## macOS: declaring document types

macOS only lists an app in "Open with" and accepts Dock drops if `Info.plist` declares `CFBundleDocumentTypes`. We declare the specific types we can read (MP4, M4V, MOV, M4A, MP3, WAV, AAC, plus `mkv` and `webm` by extension; chapter 11 explains why not the generic `public.movie` and `public.audio`), with `CFBundleTypeRole` Editor and `LSHandlerRank` **Alternate**: we show up in the menu but never become the default app for `.mp4`. A second entry with `public.folder` (role Viewer) lets whole folders be dropped on the Dock icon.

Wails has a `fileAssociations` section in `build/config.yml`, but it needs one entry per extension and regenerating the build assets overwrites our edited plists. Declaring UTIs by hand, in `Info.plist` and `Info.dev.plist`, is shorter and keeps control of the rank.

## Windows: arguments and a single instance

Dragging files onto an `.exe` passes them as arguments. `SingleInstanceOptions{UniqueID, OnSecondInstanceLaunch}` makes a second launch hand `SecondInstanceData.Args` (the program name first) to the running copy and exit. The callback filters the args, adds them to the launcher and brings the window to the front. Only Windows reads `os.Args` for its own launch: on macOS the arguments are not files and the open events carry them.

## Busy

If a job is running when files arrive, `Service.Run` would silently refuse. We chose the simplest robust behaviour over a queue: a native message "Busy: try again when the current job finishes." (EN and FR strings `busy.title`, `busy.message`). `Service.Busy()` is the check; the tiny race between check and run just ends in the same refusal.

## Try it

```sh
wails3 task darwin:package
ffmpeg -f lavfi -i sine=d=2 -f lavfi -i color=c=blue:s=64x64:d=2 -c:a aac -c:v libx264 -shortest t1.mp4
open -a bin/AudioExtractor.app t1.mp4
```

To see the paths without a dialog, temporarily log inside the `launch.New` callback in `main.go`.

## Pitfalls

- `open -a` with several files may deliver them as several events: that is the reason for the debounce.
- Do not give the app the "Owner" role or "Default" rank, or it would steal `.mp4` from QuickTime.
- Launch Services caches document types: after changing the plist, a freshly built app can need `lsregister -f` (or a reboot) before "Open with" updates. Dock drops of the running build work regardless.
- The Windows build is `-H windowsgui`: no console, so log to a file when debugging launch arguments.
