//go:build !windows

package extract

import (
	"os"
	"os/exec"
)

func hideWindow(*exec.Cmd) {}

// killWithUs is a no-op here: ffmpeg is killed through its context, and the
// temporary file's leading dot already hides it.
func killWithUs(*os.Process) {}

func setHidden(string, bool) error { return nil }
