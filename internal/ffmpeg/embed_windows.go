package ffmpeg

import "embed"

// Placeholder until ticket #4 cross-compiles ffmpeg for Windows: the folder
// holds only README.md, so embeddedBinary returns nil and Path returns
// ErrNotBundled. Once bin/windows/ffmpeg.exe exists at build time it is
// embedded and unpacked with no code change here.
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
