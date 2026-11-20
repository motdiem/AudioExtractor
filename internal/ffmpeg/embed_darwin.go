package ffmpeg

import (
	"embed"
	"fmt"
	"os/exec"
)

// The whole folder is embedded (not bin/darwin/ffmpeg directly) so the package
// still compiles on a fresh checkout where the binary has not been built yet:
// the committed README.md keeps the pattern non-empty.
//
//go:embed bin/darwin
var binFS embed.FS

const binaryName = "ffmpeg"

func embeddedBinary() []byte {
	data, err := binFS.ReadFile("bin/darwin/" + binaryName)
	if err != nil {
		return nil
	}
	return data
}

// prepareBinary makes the unpacked file runnable on macOS: Apple Silicon
// refuses to execute unsigned code, so ad-hoc sign it unless it already
// carries a valid signature (the build script signs it, so this is a safety
// net), and drop any quarantine flag.
func prepareBinary(path string) error {
	_ = exec.Command("/usr/bin/xattr", "-d", "com.apple.quarantine", path).Run()
	if exec.Command("/usr/bin/codesign", "--verify", path).Run() == nil {
		return nil
	}
	out, err := exec.Command("/usr/bin/codesign", "--force", "--sign", "-", path).CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg: ad-hoc sign %s: %w: %s", path, err, out)
	}
	return nil
}
