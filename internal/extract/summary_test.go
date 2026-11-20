package extract

import (
	"testing"

	"github.com/motdiem/AudioExtractor/internal/i18n"
)

func TestSummaryTextResolvesPlurals(t *testing.T) {
	cases := []struct {
		lang                 i18n.Lang
		done, failed, total  int
		stopped              bool
		wantTitle, wantCount string
	}{
		{i18n.EN, 3, 0, 3, false, "3 files done", ""},
		{i18n.EN, 1, 1, 2, false, "1 done, 1 failed", ""},
		{i18n.EN, 1, 0, 5, true, "Stopped after 1 of 5", "1 done, 0 failed"},
		{i18n.FR, 1, 1, 2, false, "1 réussi, 1 en échec", ""},
		{i18n.FR, 2, 3, 5, false, "2 réussis, 3 en échec", ""},
		{i18n.FR, 4, 0, 4, false, "4 fichiers terminés", ""},
		{i18n.FR, 0, 1, 3, true, "Arrêté après 1 sur 3", "0 réussi, 1 en échec"},
	}
	for _, c := range cases {
		svc := New(Config{Port: &fakePort{}, Lang: c.lang})
		title, counts := svc.summaryText(c.done, c.failed, c.total, c.stopped)
		if title != c.wantTitle || counts != c.wantCount {
			t.Errorf("%s %+v: got (%q, %q), want (%q, %q)", c.lang, c, title, counts, c.wantTitle, c.wantCount)
		}
	}
}
