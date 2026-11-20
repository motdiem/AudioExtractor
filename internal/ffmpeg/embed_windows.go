package ffmpeg

import "embed"

// bin/windows/ffmpeg.exe is the statically linked x64 build from
// `wails3 task ffmpeg:windows` (cross-compiled on the Mac with mingw-w64).
// The folder (not the file) is embedded so a checkout without the built exe
// still compiles: the committed README.md keeps the pattern non-empty, and
// embeddedBinary then returns nil, so Path returns ErrNotBundled.
//
//go:embed bin/windows
var binFS embed.FS

const binaryName = "ffmpeg.exe"

func embeddedBinary() []byte {
	data, err := binFS.ReadFile("bin/windows/" + binaryName)
	if err != nil {
		return nil
	}
	return data
}

// prepareBinary: nothing to do on Windows (no signing, no exec bit).
func prepareBinary(string) error { return nil }
