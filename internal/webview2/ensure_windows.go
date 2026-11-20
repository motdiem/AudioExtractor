package webview2

import (
	"os"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"github.com/motdiem/AudioExtractor/internal/i18n"
)

// Ensure returns true when the WebView2 Runtime is installed. Otherwise it
// shows a native message box (user32 MessageBoxW, no WebView needed) with the
// download link, opens the link if the user says Yes, and returns false: the
// caller should exit.
func Ensure(lang i18n.Lang) bool {
	if !Missing(registryLookup, os.Getenv) {
		return true
	}
	msgs := i18n.Load(lang)
	title, _ := windows.UTF16PtrFromString(msgs.T("webview2.title"))
	text, _ := windows.UTF16PtrFromString(msgs.T("webview2.message", "url", DownloadURL))
	const (
		mbYesNo         = 0x00000004
		mbIconError     = 0x00000010
		mbSetForeground = 0x00010000
		idYes           = 6
	)
	ret, _ := windows.MessageBox(0, text, title, mbYesNo|mbIconError|mbSetForeground)
	if ret == idYes {
		verb, _ := windows.UTF16PtrFromString("open")
		url, _ := windows.UTF16PtrFromString(DownloadURL)
		_ = windows.ShellExecute(0, verb, url, nil, nil, windows.SW_SHOWNORMAL)
	}
	return false
}

func registryLookup(hive Hive, path, name string) (string, bool) {
	root, access := registry.CURRENT_USER, uint32(registry.QUERY_VALUE)
	if hive == LocalMachine {
		// EdgeUpdate is a 32-bit program: its machine keys live under WOW6432Node.
		root, access = registry.LOCAL_MACHINE, registry.QUERY_VALUE|registry.WOW64_32KEY
	}
	k, err := registry.OpenKey(root, path, access)
	if err != nil {
		return "", false
	}
	defer k.Close()
	v, _, err := k.GetStringValue(name)
	if err != nil {
		return "", false
	}
	return v, true
}
