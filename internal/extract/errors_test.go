package extract

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"

	"github.com/motdiem/AudioExtractor/internal/i18n"
)

// One service test per error kind, each with a fixture generated at test time.

// runExpecting runs src and asserts the single outcome's kind, that the window
// was told, and that nothing but want is left in the destination folder.
func runExpecting(t *testing.T, src, destDir string, want ErrorKind) (*Service, *fakePort, Outcome) {
	t.Helper()
	port := &fakePort{saveTo: filepath.Join(destDir, "out.m4a")}
	svc, rec := newService(t, port)
	res := svc.Run([]string{src}, "m4a")
	if len(res.Outcomes) != 1 || res.Outcomes[0].Kind != want {
		t.Fatalf("outcomes = %+v, want one of kind %q", res.Outcomes, want)
	}
	o := res.Outcomes[0]
	last := rec.events[len(rec.events)-1]
	if last.Phase != PhaseError || last.Kind != want || last.Message != i18n.Load(i18n.EN).T("error."+string(want)) {
		t.Errorf("last event = %+v, want an error event for %q with its catalog message", last, want)
	}
	if entries, _ := os.ReadDir(destDir); len(entries) != 0 {
		t.Errorf("%s should be empty, has %d entries (partial output left behind)", destDir, len(entries))
	}
	return svc, port, o
}

func cutInHalf(t *testing.T, src, name string) string {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	cut := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(cut, data[:len(data)/2], 0o644); err != nil {
		t.Fatal(err)
	}
	return cut
}

func TestNoAudioKind(t *testing.T) {
	runExpecting(t, makeClip(t, "silent.mp4", ""), t.TempDir(), KindNoAudio)
}

// A download cut off before the moov atom (normal MP4 layout: index at the end).
func TestTruncatedMP4WithMoovAtTheEndIsUnreadable(t *testing.T) {
	cut := cutInHalf(t, makeClip(t, "full.mp4", "aac"), "cut.mp4")
	_, _, o := runExpecting(t, cut, t.TempDir(), KindUnreadable)
	if !strings.Contains(o.Log, "moov atom not found") {
		t.Errorf("log should carry ffmpeg's own words:\n%s", o.Log)
	}
}

// A "faststart" file keeps its index in front, so ffmpeg opens it and would
// quietly write half of the audio. We must not call that a success.
func TestTruncatedFaststartMP4IsUnreadableNotHalfAnExtraction(t *testing.T) {
	full := filepath.Join(t.TempDir(), "fast.mp4")
	// Enough audio that cutting the file in half lands inside the media data.
	runFFmpegForTest(t, "-f", "lavfi", "-i", "sine=frequency=440:duration=20",
		"-f", "lavfi", "-i", "color=c=blue:s=64x64:r=10:d=20", "-c:v", "mpeg4", "-c:a", "aac",
		"-movflags", "+faststart", full)
	cut := cutInHalf(t, full, "cut.mp4")
	runExpecting(t, cut, t.TempDir(), KindUnreadable)
}

func TestEmptyFileIsUnreadable(t *testing.T) {
	src := filepath.Join(t.TempDir(), "empty.mp4")
	os.WriteFile(src, nil, 0o644)
	runExpecting(t, src, t.TempDir(), KindUnreadable)
}

func TestRawZoomFileIsUnreadable(t *testing.T) {
	src := filepath.Join(t.TempDir(), "zoom_0.zoom")
	os.WriteFile(src, []byte{0x00, 0x01, 0x02, 0xff, 0xfe, 0x10, 0x80, 0x00, 0x00, 0x11}, 0o644)
	runExpecting(t, src, t.TempDir(), KindUnreadable)
}

func TestTextFileRenamedMP4IsNotMedia(t *testing.T) {
	src := filepath.Join(t.TempDir(), "notes.mp4")
	os.WriteFile(src, []byte("hello, this is not a video\n"), 0o644)
	runExpecting(t, src, t.TempDir(), KindNotMedia)
}

func TestUnknownBinaryWithUnknownExtensionIsNotMedia(t *testing.T) {
	src := filepath.Join(t.TempDir(), "data.bin")
	os.WriteFile(src, []byte{0x00, 0x01, 0x02, 0xff, 0xfe, 0x10, 0x80, 0x00}, 0o644)
	runExpecting(t, src, t.TempDir(), KindNotMedia)
}

func TestMissingSourceIsUnreadable(t *testing.T) {
	// Run's accept() rejects a missing path itself; drive the pre-check directly.
	_, kind, err := extract(context.Background(), ffmpegPath(t), filepath.Join(t.TempDir(), "gone.mp4"),
		filepath.Join(t.TempDir(), "x.m4a"), FormatM4A, nil)
	if err == nil || kind != KindUnreadable {
		t.Errorf("kind = %q err = %v, want unreadable", kind, err)
	}
}

func TestReadOnlyDestinationFolderIsUnwritable(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("chmod does not stop root or Windows from writing")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	if f, err := os.Create(filepath.Join(dir, "probe")); err == nil {
		f.Close()
		t.Skip("directory is writable despite chmod 0555")
	}
	runExpecting(t, makeClip(t, "clip.mp4", "aac"), dir, KindDestinationUnwritable)
}

func TestMissingDestinationFolderIsUnwritable(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "unplugged")
	port := &fakePort{saveTo: filepath.Join(dir, "clip.m4a")}
	svc, _ := newService(t, port)
	res := svc.Run([]string{makeClip(t, "clip.mp4", "aac")}, "m4a")
	if res.Outcomes[0].Kind != KindDestinationUnwritable {
		t.Errorf("outcome = %+v", res.Outcomes[0])
	}
}

// Cancel is silent: no error screen, no message, nothing left on disk.
func TestCancelIsSilentAndLeavesNothing(t *testing.T) {
	src := makeClip(t, "clip.mp4", "aac")
	dir := t.TempDir()
	port := &fakePort{saveTo: filepath.Join(dir, "clip.m4a")}
	bin := ffmpegPath(t)
	rec := &recorder{}
	var svc *Service
	svc = New(Config{
		Locate: func() (string, error) { return bin, nil },
		Port:   port,
		Lang:   i18n.EN,
		Emit: func(name string, data any) {
			rec.emit(name, data)
			// Press Cancel the moment the run starts.
			if e, ok := data.(StateEvent); ok && e.Phase == PhaseWorking {
				svc.Cancel()
			}
		},
	})

	res := svc.Run([]string{src}, "mp3")

	if !res.Cancelled || len(res.Outcomes) != 1 || res.Outcomes[0].Kind != KindCancelled {
		t.Fatalf("result = %+v, want cancelled", res)
	}
	for _, e := range rec.events {
		if e.Phase == PhaseError {
			t.Errorf("cancel must not show an error: %+v", e)
		}
	}
	if p := rec.phases(); p[len(p)-1] != PhaseIdle {
		t.Errorf("phases = %v, want to end idle", p)
	}
	noStrays(t, dir)
}

func TestCancelledHasNoMessageRequirement(t *testing.T) {
	for _, k := range MessageKinds {
		if k == KindCancelled {
			t.Error("cancelled is silent and must not be in MessageKinds")
		}
	}
}

// --- classification as pure functions ---

func TestClassifyWriteError(t *testing.T) {
	cases := []struct {
		name      string
		err       error
		replacing bool
		want      ErrorKind
	}{
		{"permission", fs.ErrPermission, false, KindDestinationUnwritable},
		{"wrapped permission", &fs.PathError{Op: "open", Path: "x", Err: syscall.EACCES}, false, KindDestinationUnwritable},
		{"disk full", &os.LinkError{Op: "rename", Err: syscall.ENOSPC}, true, KindDestinationUnwritable},
		{"read-only volume", &fs.PathError{Err: syscall.EROFS}, false, KindDestinationUnwritable},
		{"folder gone", &fs.PathError{Err: syscall.ENOENT}, false, KindDestinationUnwritable},
		{"locked file when replacing", &os.LinkError{Op: "rename", Err: syntheticLockErrno}, true, KindOutputLocked},
		{"lock errno without a file to replace", syntheticLockErrno, false, KindFailed},
		{"something else", errors.New("boom"), false, KindFailed},
		{"nil", nil, false, ""},
	}
	for _, c := range cases {
		if got := classifyWriteError(c.err, c.replacing); got != c.want {
			t.Errorf("%s: classifyWriteError = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestClassifyBroken(t *testing.T) {
	mp4 := append([]byte{0, 0, 0, 0x20}, []byte("ftypisom")...)
	cases := []struct {
		name  string
		file  string
		bytes []byte
		want  ErrorKind
	}{
		{"mp4 box, cut off", "a.mp4", mp4, KindUnreadable},
		{"mp4 box under a wrong extension", "a.dat", mp4, KindUnreadable},
		{"text named mp4", "a.mp4", []byte("just some words\nand more"), KindNotMedia},
		{"text with accents cut mid-character", "a.mp4", append([]byte("café"), 0xc3), KindNotMedia},
		{"garbage named mp4", "a.mp4", []byte{0, 0x81, 0xfe, 0x00, 0x9c}, KindUnreadable},
		{"garbage named bin", "a.bin", []byte{0, 0x81, 0xfe, 0x00, 0x9c}, KindNotMedia},
		{"empty", "a.mp4", nil, KindUnreadable},
		{"zoom raw", "audio.zoom", []byte{0, 0x81, 0xfe}, KindUnreadable},
		{"zoom raw uppercase", "AUDIO.ZOOM", []byte("text"), KindUnreadable},
		{"wav header", "a.wav", []byte("RIFF....WAVEfmt "), KindUnreadable},
		{"png", "a.png", []byte("\x89PNG\r\n\x1a\n\x00\x00"), KindNotMedia},
	}
	for _, c := range cases {
		if got := classifyBroken(c.file, sniff(c.bytes)); got != c.want {
			t.Errorf("%s: = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestLogPhrases(t *testing.T) {
	if !logSaysInputBroken("[mov] moov atom not found\nError: Invalid data found when processing input") {
		t.Error("moov / invalid data not recognised")
	}
	if !logSaysPartial("stream 1, offset 0x16ddf: partial file") {
		t.Error("partial file not recognised")
	}
	if !logSaysWriteFailed("av_interleaved_write_frame(): No space left on device") {
		t.Error("disk full not recognised")
	}
	if logSaysInputBroken("Stream #0:0: Audio: aac") || logSaysPartial("") || logSaysWriteFailed("ok") {
		t.Error("false positive")
	}
}

// --- Copy details ---

func TestCopyDetailsHasHeaderAndRawLog(t *testing.T) {
	src := makeClip(t, "Weekly sync.mp4", "")
	port := &fakePort{saveTo: filepath.Join(t.TempDir(), "out.m4a")}
	svc, _ := newService(t, port)
	svc.cfg.Version = "1.2.3"
	res := svc.Run([]string{src}, "m4a")
	if res.Outcomes[0].Kind != KindNoAudio {
		t.Fatalf("setup: %+v", res)
	}
	if err := svc.CopyDetails(); err != nil {
		t.Fatal(err)
	}
	if len(port.copied) != 1 {
		t.Fatalf("copied %d times", len(port.copied))
	}
	text := port.copied[0]
	for _, want := range []string{
		"AudioExtractor 1.2.3", "OS: " + runtime.GOOS, "File: Weekly sync.mp4", "Error: no-audio",
		"--- ffmpeg log ---", "Stream #0:0", // raw ffmpeg output
	} {
		if !strings.Contains(text, want) {
			t.Errorf("details lack %q:\n%s", want, text)
		}
	}
}

func TestCopyDetailsWithoutAnErrorFails(t *testing.T) {
	svc, _ := newService(t, &fakePort{})
	if err := svc.CopyDetails(); err == nil {
		t.Error("expected an error when there is nothing to copy")
	}
}

// runFFmpegForTest generates a fixture with ffmpeg.
func runFFmpegForTest(t *testing.T, args ...string) {
	t.Helper()
	out, err := runFFmpeg(context.Background(), fixtureFFmpeg(t), append([]string{"-loglevel", "error", "-y"}, args...)...)
	if err != nil {
		t.Fatalf("generating fixture: %v\n%s", err, out)
	}
}
