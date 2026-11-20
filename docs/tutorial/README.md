# AudioExtractor tutorial

A step-by-step build log of AudioExtractor, a tiny desktop app (macOS + Windows) that extracts the audio from a Zoom recording. Each chapter matches one ticket and is written to be readable on its own.

1. [Scaffold and local builds](01-scaffold-and-local-builds.md): a Wails v3 app, dev mode, and a one-command release that builds a universal macOS app and a Windows exe from a single Mac, with no CI.
2. [A slim ffmpeg for macOS, embedded in the app](02-slim-ffmpeg-for-macos.md): a 6 MB universal, LGPL-only ffmpeg built from pinned sources with one task, embedded with `go:embed`, unpacked to the user cache and ad-hoc signed on first use.
3. [Cross-compiling ffmpeg for Windows, from a Mac](03-cross-compiling-ffmpeg-for-windows.md): the same ffmpeg as a 3.5 MB static `ffmpeg.exe` built with mingw-w64, DLL and licence checks without running it, reproducible PE builds, and testing in a Parallels VM from the command line.
4. [First extraction: drop an MP4, get an M4A](04-first-extraction-m4a.md): the extraction service and its test seam, Wails file drop and dialogs, EN/FR localization.
5. [MP3 output, progress and cancel](05-mp3-progress-cancel.md): libmp3lame at 128 kbps, parsing `-progress pipe:1`, cancelling by context with clean-up, the 500 ms "never flash" rule, and a tiny settings store.
6. [Errors people understand, and Copy details](06-errors-people-understand.md): classifying failures from Go pre-checks, ffmpeg output and file sniffing, bilingual messages for non-technical users, and a Copy details button that copies the raw ffmpeg log.
7. [Batches, folder drops and Zoom naming](07-batches-and-zoom-names.md): expanding folders, a folder dialog for several inputs, collision suffixes, Zoom meeting-folder names, sequential runs with a failure summary, and cancel mid-batch.
8. [Launch with files](08-launch-with-files.md): Dock-icon drops, exe drops and "Open with" through macOS open-file events, Windows arguments and a single instance, with a debounce that makes one batch and a gate that waits for the frontend.
9. [Window polish and a custom icon](09-window-polish-and-icon.md): a hand-drawn SVG icon with hinted small sizes turned into `.icns`, Liquid Glass `Assets.car` and `.ico` by one task (and why builds kept rewriting `Assets.car`), native light/dark backgrounds, a remembered window position clamped to the connected screens, and a version stamped from `git describe`.
11. [Hardening from a code review](11-hardening-from-a-code-review.md): eight bugs a review found behind green tests (quitting mid-job, a folder named "Partial file reviews", silent overwrites, unsupported formats called damaged, a crash blamed on the file, a launch race, French plurals), why each happened, and the fix.
