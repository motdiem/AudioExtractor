`ffmpeg` (the arm64 slice, ad-hoc signed) is written here by
`wails3 task ffmpeg:mac`, next to the universal `bin/darwin/ffmpeg`, and
embedded only by the arm64 Go build (`embed_darwin_arm64.go`).
It is git-ignored: rebuild it from the pinned sources in `scripts/ffmpeg/`.
