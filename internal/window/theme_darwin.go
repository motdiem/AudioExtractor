package window

import (
	"os/exec"
	"strings"
)

// systemDark reads the same preference AppKit does: AppleInterfaceStyle is
// "Dark" in dark mode and absent in light mode (including Auto while light).
func systemDark() bool {
	out, err := exec.Command("defaults", "read", "-g", "AppleInterfaceStyle").Output()
	return err == nil && strings.EqualFold(strings.TrimSpace(string(out)), "dark")
}
