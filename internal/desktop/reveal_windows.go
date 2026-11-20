package desktop

import "os/exec"

// explorer.exe returns exit status 1 even on success, so the error is ignored.
//
// No HideWindow here: explorer.exe is a GUI program (no console to hide), and
// it honours the SW_HIDE that HideWindow puts in STARTUPINFO, so the folder
// window opened, with the file selected, but invisible.
func reveal(path string) error {
	_ = exec.Command("explorer.exe", "/select,", path).Run()
	return nil
}
