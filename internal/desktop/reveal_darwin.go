package desktop

import "os/exec"

// `open -R` opens Finder with the file selected.
func reveal(path string) error { return exec.Command("open", "-R", path).Run() }
