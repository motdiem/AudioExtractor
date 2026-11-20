package ffmpeg

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func newInstaller(t *testing.T, data []byte, prepare func(string) error) installer {
	t.Helper()
	return installer{root: t.TempDir(), name: "ffmpeg", data: data, prepare: prepare}
}

func TestInstallWritesExecutableUnderHashedDir(t *testing.T) {
	in := newInstaller(t, []byte("fake ffmpeg v1"), nil)

	p, err := in.install()
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, in.data) {
		t.Fatalf("content = %q, want %q", got, in.data)
	}
	dir := filepath.Base(filepath.Dir(p))
	if !strings.HasPrefix(dir, "ffmpeg-") || len(dir) != len("ffmpeg-")+12 {
		t.Fatalf("unexpected cache folder %q", dir)
	}
	if runtime.GOOS != "windows" {
		if fi, _ := os.Stat(p); fi.Mode().Perm()&0o111 == 0 {
			t.Fatalf("not executable: %v", fi.Mode())
		}
	}
	// No temp files left behind.
	entries, _ := os.ReadDir(filepath.Dir(p))
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Fatalf("leftover temp file %s", e.Name())
		}
	}
}

func TestDifferentBinariesGetDifferentPaths(t *testing.T) {
	root := t.TempDir()
	p1, err := installer{root: root, name: "ffmpeg", data: []byte("v1")}.install()
	if err != nil {
		t.Fatal(err)
	}
	p2, err := installer{root: root, name: "ffmpeg", data: []byte("v2")}.install()
	if err != nil {
		t.Fatal(err)
	}
	if p1 == p2 {
		t.Fatalf("a new binary must not reuse the old path %s", p1)
	}
}

func TestInstallReusesValidCopy(t *testing.T) {
	var prepared atomic.Int32
	in := newInstaller(t, []byte("fake ffmpeg"), func(string) error { prepared.Add(1); return nil })

	p1, err := in.install()
	if err != nil {
		t.Fatal(err)
	}
	fi1, _ := os.Stat(p1)
	p2, err := in.install()
	if err != nil {
		t.Fatal(err)
	}
	fi2, _ := os.Stat(p2)
	if p1 != p2 {
		t.Fatalf("paths differ: %s vs %s", p1, p2)
	}
	if prepared.Load() != 1 {
		t.Fatalf("prepare ran %d times, want 1 (second call should reuse)", prepared.Load())
	}
	if !os.SameFile(fi1, fi2) || !fi1.ModTime().Equal(fi2.ModTime()) {
		t.Fatal("file was rewritten instead of reused")
	}
}

// prepare (ad-hoc signing on macOS) changes the bytes; the modified copy must
// still be recognised as valid on the next launch.
func TestReuseAcceptsFileModifiedByPrepare(t *testing.T) {
	var prepared atomic.Int32
	sign := func(p string) error {
		prepared.Add(1)
		f, err := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = f.WriteString("+signature")
		return err
	}
	in := newInstaller(t, []byte("unsigned"), sign)
	p, err := in.install()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := in.install(); err != nil {
		t.Fatal(err)
	}
	if prepared.Load() != 1 {
		t.Fatalf("prepare ran %d times, want 1", prepared.Load())
	}
	if got, _ := os.ReadFile(p); string(got) != "unsigned+signature" {
		t.Fatalf("content = %q", got)
	}
}

func TestInstallRepairsCorruptedCopy(t *testing.T) {
	in := newInstaller(t, []byte("good binary"), nil)
	p, err := in.install()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("trunc"), 0o755); err != nil {
		t.Fatal(err)
	}
	p2, err := in.install()
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(p2); string(got) != "good binary" {
		t.Fatalf("not repaired: %q", got)
	}
}

func TestInstallRepairsMissingStamp(t *testing.T) {
	in := newInstaller(t, []byte("good binary"), nil)
	p, err := in.install()
	if err != nil {
		t.Fatal(err)
	}
	os.Remove(p + stampSuffix)
	if _, err := in.install(); err != nil {
		t.Fatal(err)
	}
	if !valid(p, p+stampSuffix) {
		t.Fatal("stamp not rewritten")
	}
}

func TestPrepareErrorIsReported(t *testing.T) {
	boom := errors.New("codesign failed")
	in := newInstaller(t, []byte("x"), func(string) error { return boom })
	if _, err := in.install(); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want %v", err, boom)
	}
}

func TestConcurrentInstalls(t *testing.T) {
	root := t.TempDir()
	data := bytes.Repeat([]byte("ffmpeg"), 200_000) // ~1.2 MB, makes races likelier
	const n = 16
	paths := make([]string, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			paths[i], errs[i] = installer{root: root, name: "ffmpeg", data: data}.install()
		}()
	}
	wg.Wait()
	for i := range n {
		if errs[i] != nil {
			t.Fatalf("install %d: %v", i, errs[i])
		}
		if paths[i] != paths[0] {
			t.Fatalf("install %d returned %s, want %s", i, paths[i], paths[0])
		}
	}
	got, err := os.ReadFile(paths[0])
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("final file corrupted (err=%v, len=%d)", err, len(got))
	}
	if !valid(paths[0], paths[0]+stampSuffix) {
		t.Fatal("final copy not valid")
	}
}

// TestBundledFFmpegRuns is the end-to-end smoke test: the real embedded binary
// is unpacked into the real user cache, signed, and executed.
//
//	go test ./internal/ffmpeg -run Bundled -v
func TestBundledFFmpegRuns(t *testing.T) {
	p, err := Path()
	if errors.Is(err, ErrNotBundled) {
		t.Skip("no ffmpeg bundled in this build; run `wails3 task ffmpeg:mac` or `ffmpeg:windows` first")
	}
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(p, "-hide_banner", "-version").CombinedOutput()
	if err != nil {
		t.Fatalf("%s -version: %v\n%s", p, err, out)
	}
	first, _, _ := strings.Cut(string(out), "\n")
	t.Logf("%s\n%s", p, first)
	if !strings.HasPrefix(first, "ffmpeg version ") {
		t.Fatalf("unexpected output: %s", out)
	}
	if strings.Contains(string(out), "--enable-gpl") || strings.Contains(string(out), "--enable-nonfree") {
		t.Fatal("bundled ffmpeg is not LGPL-only")
	}
	if runtime.GOOS == "darwin" {
		if out, err := exec.Command("/usr/bin/codesign", "--verify", "--strict", p).CombinedOutput(); err != nil {
			t.Fatalf("unpacked binary is not validly signed: %v\n%s", err, out)
		}
	}
	// Second call is served from memory / the existing copy.
	p2, err := Path()
	if err != nil || p2 != p {
		t.Fatalf("second Path() = %q, %v", p2, err)
	}
}
