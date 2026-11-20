package i18n

import "golang.org/x/sys/windows"

// GetUserDefaultUILanguage returns a LANGID whose low 10 bits are the primary
// language (0x0c = French). Calling kernel32 through x/sys keeps the build pure Go.
func detect() Lang {
	proc := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetUserDefaultUILanguage")
	id, _, _ := proc.Call()
	if id&0x3ff == 0x0c {
		return FR
	}
	return EN
}
