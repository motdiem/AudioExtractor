package desktop

import (
	"os/exec"
	"syscall"
)

// explorer.exe returns exit status 1 even on success, so the error is ignored.
func reveal(path string) error {
	cmd := exec.Command("explorer.exe", "/select,", path)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = cmd.Run()
	return nil
}
