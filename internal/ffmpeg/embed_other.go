//go:build !darwin && !windows

package ffmpeg

// No ffmpeg is bundled for other operating systems; Path returns ErrNotBundled.

const binaryName = "ffmpeg"

func embeddedBinary() []byte { return nil }

func prepareBinary(string) error { return nil }
