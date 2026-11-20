package ffmpeg

import "embed"

// Only this architecture's slice is embedded: a universal app is two Go
// binaries, and embedding the universal ffmpeg in both would ship it twice.
// The folder (not the file) is embedded so a fresh checkout without the built
// binary still compiles; the committed README.md keeps the pattern non-empty.
//
//go:embed bin/darwin/arm64
var binFS embed.FS

const binDir = "bin/darwin/arm64"
