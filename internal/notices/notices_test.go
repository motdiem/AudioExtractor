package notices

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func fixture() fstest.MapFS {
	return fstest.MapFS{
		Name:                                  {Data: []byte("NOTICE BODY")},
		"third_party/licenses/README.md":      {Data: []byte("maintainer notes")},
		"third_party/licenses/lame/COPYING":   {Data: []byte("LAME TEXT")},
		"third_party/licenses/ffmpeg/LICENSE": {Data: []byte("FFMPEG TEXT")},
	}
}

func TestTextHasNoticeAndEveryLicence(t *testing.T) {
	text, err := Text(fixture())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"NOTICE BODY", "ffmpeg/LICENSE", "FFMPEG TEXT", "lame/COPYING", "LAME TEXT"} {
		if !strings.Contains(text, want) {
			t.Errorf("text lacks %q", want)
		}
	}
	if strings.Contains(text, "maintainer notes") {
		t.Error("the folder README is not a licence and must be left out")
	}
}

func TestOpenWritesTheTextAndOpensIt(t *testing.T) {
	dir := t.TempDir()
	var opened string
	s := &Service{FS: fixture(), Dir: dir, OpenFile: func(p string) error { opened = p; return nil }}
	if err := s.Open(); err != nil {
		t.Fatal(err)
	}
	if opened != filepath.Join(dir, Name) {
		t.Fatalf("opened %q", opened)
	}
	got, err := os.ReadFile(opened)
	if err != nil || !strings.Contains(string(got), "LAME TEXT") {
		t.Fatalf("file not written: %v", err)
	}
}

// The real files must exist and say what the release promises.
func TestRepoNoticeIsComplete(t *testing.T) {
	text, err := Text(os.DirFS("../.."))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"FFmpeg 9.0.2", "LAME 3.100", "GNU Lesser General Public", "Wails", "Permission is hereby granted"} {
		if !strings.Contains(text, want) {
			t.Errorf("repo notice lacks %q", want)
		}
	}
}
