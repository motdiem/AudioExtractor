package webview2

import (
	"strings"
	"testing"

	"github.com/motdiem/AudioExtractor/internal/i18n"
)

type fakeRegistry map[string]string // "hive|path|name" -> value

func (f fakeRegistry) lookup(h Hive, path, name string) (string, bool) {
	v, ok := f[key(h, path, name)]
	return v, ok
}

func key(h Hive, path, name string) string {
	return string(rune('0'+h)) + "|" + path + "|" + name
}

const runtimeGUID = "{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}"

func noEnv(string) string { return "" }

func TestMissing(t *testing.T) {
	clients := `Software\Microsoft\EdgeUpdate\Clients\` + runtimeGUID
	state := `Software\Microsoft\EdgeUpdate\ClientState\` + runtimeGUID
	cases := []struct {
		name string
		reg  fakeRegistry
		want bool
	}{
		{"nothing registered", fakeRegistry{}, true},
		{"per-machine runtime", fakeRegistry{key(LocalMachine, clients, "pv"): "131.0.2903.70"}, false},
		{"per-user runtime", fakeRegistry{key(CurrentUser, clients, "pv"): "131.0.2903.70"}, false},
		{"uninstalled leaves 0.0.0.0", fakeRegistry{key(LocalMachine, clients, "pv"): "0.0.0.0"}, true},
		{"empty pv", fakeRegistry{key(CurrentUser, clients, "pv"): " "}, true},
		{"only the Wails loader's value", fakeRegistry{key(LocalMachine, state, "EBWebView"): `C:\Program Files (x86)\Microsoft\EdgeWebView\Application\131.0.2903.70`}, false},
		{"Edge Canary only", fakeRegistry{key(CurrentUser, `Software\Microsoft\EdgeUpdate\Clients\{65C35B14-6C1D-4122-AC46-7148CC9D6497}`, "pv"): "133.0.1.0"}, false},
	}
	for _, c := range cases {
		if got := Missing(c.reg.lookup, noEnv); got != c.want {
			t.Errorf("%s: Missing = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestForceMissingOnlyInDevBuilds(t *testing.T) {
	installed := fakeRegistry{key(LocalMachine, `Software\Microsoft\EdgeUpdate\Clients\`+runtimeGUID, "pv"): "131.0.2903.70"}
	env := func(k string) string {
		if k == ForceMissingEnv {
			return "1"
		}
		return ""
	}
	defer func(old bool) { allowForce = old }(allowForce)

	allowForce = true
	if !Missing(installed.lookup, env) {
		t.Error("dev build: the override should report the runtime as missing")
	}
	allowForce = false
	if Missing(installed.lookup, env) {
		t.Error("production build: the override must be ignored")
	}
}

func TestMessageHasLinkInBothLanguages(t *testing.T) {
	for _, lang := range i18n.Langs {
		msgs := i18n.Load(lang)
		if got := msgs.T("webview2.message", "url", DownloadURL); !strings.Contains(got, DownloadURL) {
			t.Errorf("%s: message has no download link: %q", lang, got)
		}
		if msgs.T("webview2.title") == "webview2.title" {
			t.Errorf("%s: no title", lang)
		}
	}
}
