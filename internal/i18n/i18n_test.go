package i18n

import (
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
	en, fr := Load(EN), Load(FR)
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
