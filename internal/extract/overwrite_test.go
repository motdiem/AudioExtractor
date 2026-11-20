package extract

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The Save dialog confirmed "typed name", not "typed name.m4a": an existing
// "typed name.m4a" must not be overwritten silently.
func TestAddedExtensionNeverOverwritesAnUnconfirmedFile(t *testing.T) {
	src := makeClip(t, "clip.mp4", "aac")
	dir := t.TempDir()
	existing := filepath.Join(dir, "typed name.m4a")
	os.WriteFile(existing, []byte("keep me"), 0o644)
	svc, _ := newService(t, &fakePort{saveTo: filepath.Join(dir, "typed name")})

	res := svc.Run([]string{src}, "m4a")

	if got := res.Outcomes[0].Output; filepath.Base(got) != "typed name (2).m4a" {
		t.Errorf("output = %q, want typed name (2).m4a", got)
	}
	if data, _ := os.ReadFile(existing); string(data) != "keep me" {
		t.Error("the existing file was overwritten")
	}
}

// A changed extension ("notes.txt" in M4A mode) is the same situation.
func TestChangedExtensionNeverOverwritesAnUnconfirmedFile(t *testing.T) {
	src := makeClip(t, "clip.mp4", "aac")
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "notes.txt.m4a"), []byte("keep me"), 0o644)
	svc, _ := newService(t, &fakePort{saveTo: filepath.Join(dir, "notes.txt")})

	res := svc.Run([]string{src}, "m4a")

	if got := res.Outcomes[0].Output; filepath.Base(got) != "notes.txt (2).m4a" {
		t.Errorf("output = %q, want notes.txt (2).m4a", got)
	}
}

// When the dialog's own name has the right extension, the OS already asked
// "replace?": the file is replaced as the user confirmed.
func TestConfirmedReplaceStillReplaces(t *testing.T) {
	src := makeClip(t, "clip.mp4", "aac")
	dest := filepath.Join(t.TempDir(), "out.m4a")
	os.WriteFile(dest, []byte("old"), 0o644)
	svc, _ := newService(t, &fakePort{saveTo: dest})

	if res := svc.Run([]string{src}, "m4a"); res.Outcomes[0].Output != dest {
		t.Errorf("output = %q, want %q", res.Outcomes[0].Output, dest)
	}
	assertM4A(t, dest, "aac")
}

func TestSuggestionNeverEqualsTheSource(t *testing.T) {
	dir := t.TempDir()
	mp3 := touch(t, dir, "talk.mp3")
	m4a := touch(t, dir, "audio_only.m4a")
	svc := New(Config{Port: &fakePort{}})

	items, _, err := svc.accept([]string{mp3}, FormatMP3)
	if err != nil || items[0].SuggestedName != "talk (2).mp3" {
		t.Errorf("MP3 from an .mp3: items = %+v, err = %v, want talk (2).mp3", items, err)
	}
	items, _, _ = svc.accept([]string{m4a}, FormatM4A)
	if items[0].SuggestedName != "audio_only (2).m4a" {
		t.Errorf("M4A from audio_only.m4a: suggested %q, want audio_only (2).m4a", items[0].SuggestedName)
	}
	items, _, _ = svc.accept([]string{m4a}, FormatMP3)
	if items[0].SuggestedName != "audio_only.mp3" {
		t.Errorf("another format keeps the plain name: suggested %q", items[0].SuggestedName)
	}
}

// Choosing the source itself in the Save dialog is refused with a clear
// message, and the source is left intact.
func TestSavingOverTheSourceIsRefused(t *testing.T) {
	src := makeAudioWith(t, "talk.mp3", 2, 1, "libmp3lame", "44100")
	before, _ := os.ReadFile(src)
	cases := map[string]string{"same path": src}
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		cases["other case"] = filepath.Join(filepath.Dir(src), strings.ToUpper("talk.mp3"))
	}
	for name, chosen := range cases {
		t.Run(name, func(t *testing.T) {
			svc, rec := newService(t, &fakePort{saveTo: chosen})
			res := svc.Run([]string{src}, "mp3")
			if len(res.Outcomes) != 1 || res.Outcomes[0].Kind != KindSameAsSource {
				t.Fatalf("outcomes = %+v, want %q", res.Outcomes, KindSameAsSource)
			}
			last := rec.events[len(rec.events)-1]
			if last.Phase != PhaseError || last.Message != "Choose a different name: this is the original file." {
				t.Errorf("last event = %+v", last)
			}
			if after, _ := os.ReadFile(src); !bytes.Equal(before, after) {
				t.Error("the source was modified")
			}
			noStrays(t, filepath.Dir(src), "talk.mp3")
		})
	}
}

func TestSameFile(t *testing.T) {
	dir := t.TempDir()
	a := touch(t, dir, "a.mp3")
	if !sameFile(a, filepath.Join(dir, ".", "a.mp3")) {
		t.Error("same path, not cleaned")
	}
	if sameFile(a, filepath.Join(dir, "b.mp3")) {
		t.Error("different files")
	}
	missing := filepath.Join(dir, "Missing.mp3")
	insensitive := runtime.GOOS == "darwin" || runtime.GOOS == "windows"
	if got := sameFile(missing, filepath.Join(dir, "missing.mp3")); got != insensitive {
		t.Errorf("paths differing in case only: sameFile = %v, want %v on %s", got, insensitive, runtime.GOOS)
	}
}
