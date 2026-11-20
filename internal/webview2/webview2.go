// Package webview2 checks, on Windows, that the Microsoft WebView2 Runtime is
// installed before the app tries to open its window.
//
// Without the runtime, Wails cannot create the window and the app would
// simply not appear. Ensure shows a native FR/EN message box with Microsoft's
// download link instead, and tells main to exit.
package webview2

import "strings"

// DownloadURL is Microsoft's evergreen bootstrapper page for the runtime.
const DownloadURL = "https://go.microsoft.com/fwlink/p/?LinkId=2124703"

// ForceMissingEnv, set to "1", makes a non-production build behave as if the
// runtime were missing, so the message box can be tested on a machine that
// has it. Release builds (built with -tags production) ignore it.
const ForceMissingEnv = "AUDIOEXTRACTOR_FAKE_NO_WEBVIEW2"

// Hive is a registry root.
type Hive int

const (
	LocalMachine Hive = iota // 32-bit view (WOW6432Node), where EdgeUpdate writes
	CurrentUser
)

// Lookup reads a string value from the registry; ok is false when the key or
// value does not exist.
type Lookup func(hive Hive, path, name string) (value string, ok bool)

// Channel GUIDs: the WebView2 Runtime, then Edge Beta, Dev and Canary, which
// also provide WebView2 (same list as the Wails loader).
var channels = []string{
	"{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}",
	"{2CD8A007-E189-409D-A2C8-9AF4EF3C72AA}",
	"{0D50BFEC-CD6A-4F9A-964C-C7416E3ACB10}",
	"{65C35B14-6C1D-4122-AC46-7148CC9D6497}",
}

// Installed reports whether any WebView2 install is registered, per machine
// or per user. It checks both the "pv" value Microsoft documents and the
// "EBWebView" value the Wails loader reads, so it never says "missing" when
// Wails would find a runtime.
func Installed(look Lookup) bool {
	for _, guid := range channels {
		for _, hive := range []Hive{LocalMachine, CurrentUser} {
			if v, ok := look(hive, `Software\Microsoft\EdgeUpdate\Clients\`+guid, "pv"); ok && validVersion(v) {
				return true
			}
			if v, ok := look(hive, `Software\Microsoft\EdgeUpdate\ClientState\`+guid, "EBWebView"); ok && strings.TrimSpace(v) != "" {
				return true
			}
		}
	}
	return false
}

// validVersion: an uninstall can leave "pv" behind as "0.0.0.0" or empty.
func validVersion(v string) bool {
	v = strings.TrimSpace(v)
	return v != "" && v != "0.0.0.0"
}

// Missing decides whether to show the "install WebView2" message.
func Missing(look Lookup, getenv func(string) string) bool {
	if allowForce && getenv(ForceMissingEnv) == "1" {
		return true
	}
	return !Installed(look)
}
