package i18n

import (
	"encoding/json"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var placeholderRe = regexp.MustCompile(`\{[a-z]+\}`)

func placeholders(s string) string {
	p := placeholderRe.FindAllString(s, -1)
	sort.Strings(p)
	return strings.Join(p, ",")
}

func TestCatalogsHaveIdenticalKeys(t *testing.T) {
	// The raw files, so that per-OS variants are compared too.
	en, fr := rawCatalog(t, EN), rawCatalog(t, FR)
	for k, v := range en {
		f, ok := fr[k]
		if !ok {
			t.Errorf("key %q missing from fr.json", k)
			continue
		}
		if v == "" || f == "" {
			t.Errorf("key %q has an empty message", k)
		}
		if placeholders(v) != placeholders(f) {
			t.Errorf("key %q has different placeholders in en and fr", k)
		}
	}
	for k := range fr {
		if _, ok := en[k]; !ok {
			t.Errorf("key %q missing from en.json", k)
		}
	}
}

func TestFromTags(t *testing.T) {
	cases := []struct {
		tags []string
		want Lang
	}{
		{[]string{"fr-FR", "en-US"}, FR},
		{[]string{"fr_CA"}, FR},
		{[]string{"en-US", "fr"}, EN},
		{[]string{"", "C", "fr_FR.UTF-8"}, FR},
		{[]string{"de-DE"}, EN},
		{nil, EN},
	}
	for _, c := range cases {
		if got := FromTags(c.tags...); got != c.want {
			t.Errorf("FromTags(%q) = %s, want %s", c.tags, got, c.want)
		}
	}
}

func TestFirstTagParsesDefaultsOutput(t *testing.T) {
	if got := firstTag("(\n    \"fr-FR\",\n    \"en-US\"\n)\n"); got != "fr-FR" {
		t.Errorf("got %q", got)
	}
}

func TestTReplacesPlaceholders(t *testing.T) {
	if got := Load(EN).T("done.hint", "name", "a.m4a"); got != "Saved as a.m4a" {
		t.Errorf("got %q", got)
	}
}

func TestPerOSVariants(t *testing.T) {
	cases := []struct {
		lang Lang
		goos string
		want string
	}{
		{EN, "darwin", "Show in Finder"},
		{EN, "windows", "Show in Explorer"},
		{FR, "darwin", "Afficher dans le Finder"},
		{FR, "windows", "Afficher dans l’Explorateur"},
	}
	for _, c := range cases {
		got := forOS(rawCatalog(t, c.lang), c.goos)
		if got["done.show"] != c.want {
			t.Errorf("%s/%s: done.show = %q, want %q", c.lang, c.goos, got["done.show"], c.want)
		}
		for k := range got {
			if strings.Contains(k, "@") {
				t.Errorf("%s/%s: variant key %q left in the catalog", c.lang, c.goos, k)
			}
		}
	}
}

// rawCatalog reads a catalog file as is, variants included.
func rawCatalog(t *testing.T, lang Lang) Catalog {
	t.Helper()
	data, err := files.ReadFile("locales/" + string(lang) + ".json")
	if err != nil {
		t.Fatal(err)
	}
	c := Catalog{}
	if err := json.Unmarshal(data, &c); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestEveryVariantHasBothOSes(t *testing.T) {
	for _, lang := range Langs {
		c := rawCatalog(t, lang)
		for k := range c {
			base, os, ok := strings.Cut(k, "@")
			if !ok {
				continue
			}
			if _, plain := c[base]; plain {
				t.Errorf("%s: %q has both a plain key and variants", lang, base)
			}
			other := map[string]string{"mac": "windows", "windows": "mac"}[os]
			if other == "" {
				t.Errorf("%s: %q: unknown OS suffix", lang, k)
			} else if _, ok := c[base+"@"+other]; !ok {
				t.Errorf("%s: %q has no @%s variant", lang, k, other)
			}
		}
	}
}
