package extract

import (
	"os/exec"
	"syscall"
)

// hideWindow stops a console window flashing up when a GUI app starts ffmpeg.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
}
