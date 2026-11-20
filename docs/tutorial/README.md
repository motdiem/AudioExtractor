# AudioExtractor tutorial

A step-by-step build log of AudioExtractor, a tiny desktop app (macOS + Windows) that extracts the audio from a Zoom recording. Each chapter matches one ticket and is written to be readable on its own.

1. [Scaffold and local builds](01-scaffold-and-local-builds.md): a Wails v3 app, dev mode, and a one-command release that builds a universal macOS app and a Windows exe from a single Mac, with no CI.
4. [First extraction: drop an MP4, get an M4A](04-first-extraction-m4a.md): the extraction service and its test seam, Wails file drop and dialogs, EN/FR localization.
5. [MP3 output, progress and cancel](05-mp3-progress-cancel.md): libmp3lame at 128 kbps, parsing `-progress pipe:1`, cancelling by context with clean-up, the 500 ms "never flash" rule, and a tiny settings store.
