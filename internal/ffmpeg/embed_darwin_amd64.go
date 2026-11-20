package ffmpeg

import "embed"

// See embed_darwin_arm64.go.
//
//go:embed bin/darwin/amd64
var binFS embed.FS

const binDir = "bin/darwin/amd64"
