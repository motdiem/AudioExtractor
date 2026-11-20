# AudioExtractor tutorial

A step-by-step build log of AudioExtractor, a tiny desktop app (macOS + Windows) that extracts the audio from a Zoom recording. Each chapter matches one ticket and is written to be readable on its own.

1. [Scaffold and local builds](01-scaffold-and-local-builds.md): a Wails v3 app, dev mode, and a one-command release that builds a universal macOS app and a Windows exe from a single Mac, with no CI.
2. [A slim ffmpeg for macOS, embedded in the app](02-slim-ffmpeg-for-macos.md): a 6 MB universal, LGPL-only ffmpeg built from pinned sources with one task, embedded with `go:embed`, unpacked to the user cache and ad-hoc signed on first use.
