package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRoundTripAcrossStores(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "settings.json") // folder does not exist yet
	if err := New(path).Set("format", "mp3"); err != nil {
		t.Fatal(err)
	}
	var got string
	if !New(path).Get("format", &got) || got != "mp3" { // a fresh store = a new launch
		t.Errorf("format = %q, want mp3", got)
	}
}

func TestKeysAreIndependent(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "s.json"))
	type pos struct{ X, Y int }
	s.Set("format", "mp3")
	s.Set("window", pos{3, 4})
	var f string
	var p pos
	if !s.Get("format", &f) || !s.Get("window", &p) || f != "mp3" || p != (pos{3, 4}) {
		t.Errorf("got %q %+v", f, p)
	}
}

func TestMissingOrCorruptFileReadsAsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.json")
	var v string
	if New(path).Get("format", &v) {
		t.Error("missing file should read as empty")
	}
	os.WriteFile(path, []byte("{not json"), 0o644)
	s := New(path)
	if s.Get("format", &v) {
		t.Error("corrupt file should read as empty")
	}
	if err := s.Set("format", "m4a"); err != nil || !s.Get("format", &v) || v != "m4a" {
		t.Errorf("set after corrupt: %v %q", err, v)
	}
}
