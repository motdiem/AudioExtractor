// Package i18n holds the EN and FR message catalogs and picks one from the OS locale.
//
// Both catalogs live in locales/*.json and must have identical keys (enforced by a test).
// Placeholders look like {name}.
package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

//go:embed locales/*.json
var files embed.FS

// Lang is a supported UI language.
type Lang string

const (
	EN Lang = "en"
	FR Lang = "fr"
)

// Langs lists every supported language.
var Langs = []Lang{EN, FR}

// Catalog maps message keys to text in one language.
type Catalog map[string]string

// Load returns the catalog for lang (English for anything unknown).
func Load(lang Lang) Catalog {
	if lang != FR {
		lang = EN
	}
	data, err := files.ReadFile("locales/" + string(lang) + ".json")
	if err != nil {
		panic(fmt.Sprintf("i18n: missing embedded catalog %q: %v", lang, err))
	}
	c := Catalog{}
	if err := json.Unmarshal(data, &c); err != nil {
		panic(fmt.Sprintf("i18n: bad catalog %q: %v", lang, err))
	}
	return c
}

// T returns the message for key with {placeholders} replaced by the
// key/value pairs in args. A missing key returns the key itself.
func (c Catalog) T(key string, args ...string) string {
	s, ok := c[key]
	if !ok {
		return key
	}
	for i := 0; i+1 < len(args); i += 2 {
		s = strings.ReplaceAll(s, "{"+args[i]+"}", args[i+1])
	}
	return s
}

// FromTags returns FR if the first non-empty tag is a French locale ("fr",
// "fr-FR", "fr_CA.UTF-8"...), else EN.
func FromTags(tags ...string) Lang {
	for _, t := range tags {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" || t == "c" || t == "posix" {
			continue
		}
		if strings.HasPrefix(t, "fr") {
			return FR
		}
		return EN
	}
	return EN
}

var tagRe = regexp.MustCompile(`[A-Za-z]{2,3}(?:[-_][A-Za-z0-9]+)*`)

// firstTag extracts the first language tag from text such as the output of
// `defaults read -g AppleLanguages` ("(\n    \"fr-FR\",\n    \"en-US\"\n)").
func firstTag(s string) string {
	return tagRe.FindString(s)
}

// Detect returns the UI language for the current OS locale.
func Detect() Lang { return detect() }
