package extract

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/motdiem/AudioExtractor/internal/i18n"
)

// ffmpegPath finds ffmpeg for the tests: $AUDIOEXTRACTOR_FFMPEG, else PATH.
// (The bundled build is wired in once internal/ffmpeg lands.)
func ffmpegPath(t *testing.T) string {
	t.Helper()
	if p := os.Getenv("AUDIOEXTRACTOR_FFMPEG"); p != "" {
		return p
	}
	p, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not found: set AUDIOEXTRACTOR_FFMPEG or put ffmpeg on PATH")
	}
	return p
}

// fakePort records what the service asked the shell to do.
type fakePort struct {
	mu        sync.Mutex
	picks     []string // returned by ChooseInputs
	saveTo    string   // returned by ChooseSaveFile; "" = user cancels
	saveReqs  []SaveRequest
	revealed  []string
	saveError error
}

func (f *fakePort) ChooseInputs(PickRequest) ([]string, error) { return f.picks, nil }
func (f *fakePort) ChooseSaveFile(r SaveRequest) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.saveReqs = append(f.saveReqs, r)
	return f.saveTo, f.saveError
}
func (f *fakePort) Reveal(p string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.revealed = append(f.revealed, p)
	return nil
}

type recorder struct {
	mu     sync.Mutex
	events []StateEvent
}

func (r *recorder) emit(name string, data any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if name == EventState {
		r.events = append(r.events, data.(StateEvent))
	}
}

func (r *recorder) phases() []Phase {
	var p []Phase
	for _, e := range r.events {
		p = append(p, e.Phase)
	}
	return p
}

func newService(t *testing.T, port *fakePort) (*Service, *recorder) {
	t.Helper()
	bin := ffmpegPath(t)
	rec := &recorder{}
	return New(Config{
		Locate: func() (string, error) { return bin, nil },
		Port:   port,
		Emit:   rec.emit,
		Lang:   i18n.EN,
	}), rec
}

// makeClip generates a 2 s test clip. audio is an ffmpeg audio codec name, or ""
// for a video with no audio track.
func makeClip(t *testing.T, name, audio string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	args := []string{"-hide_banner", "-loglevel", "error", "-y"}
	if audio != "" {
		args = append(args, "-f", "lavfi", "-i", "sine=frequency=440:duration=2")
	}
	args = append(args, "-f", "lavfi", "-i", "color=c=blue:s=64x64:r=10:d=2", "-c:v", "mpeg4")
	if audio != "" {
		args = append(args, "-c:a", audio)
	}
	args = append(args, path)
	if out, err := exec.Command(ffmpegPath(t), args...).CombinedOutput(); err != nil {
		t.Fatalf("generating %s: %v\n%s", name, err, out)
	}
	return path
}

var (
	audioRe = regexp.MustCompile(`Audio: (\w+)`)
	videoRe = regexp.MustCompile(`Stream #.*Video:`)
	brandRe = regexp.MustCompile(`major_brand\s*:\s*(\S+)`)
	durRe   = regexp.MustCompile(`Duration: (\d+):(\d+):(\d+\.\d+)`)
)

// probeOut inspects a file with `ffmpeg -i` (no ffprobe needed).
func probeOut(t *testing.T, path string) (log string) {
	t.Helper()
	out, _ := exec.Command(ffmpegPath(t), "-hide_banner", "-i", path).CombinedOutput()
	return string(out)
}

func assertM4A(t *testing.T, path, wantCodec string) {
	t.Helper()
	log := probeOut(t, path)
	if m := audioRe.FindStringSubmatch(log); m == nil || m[1] != wantCodec {
		t.Errorf("audio codec = %v, want %s\n%s", m, wantCodec, log)
	}
	if videoRe.MatchString(log) {
		t.Errorf("output still has a video stream\n%s", log)
	}
	if m := brandRe.FindStringSubmatch(log); m == nil || m[1] != "M4A" {
		t.Errorf("major_brand = %v, want M4A\n%s", m, log)
	}
	if m := durRe.FindStringSubmatch(log); m == nil || !strings.HasPrefix(m[3], "01") && !strings.HasPrefix(m[3], "02") {
		t.Errorf("duration = %v, want about 2 s\n%s", m, log)
	}
}

func noStrays(t *testing.T, dir string, want ...string) {
	t.Helper()
	entries, _ := os.ReadDir(dir)
	var got []string
	for _, e := range entries {
		got = append(got, e.Name())
	}
	if len(got) != len(want) {
		t.Errorf("%s contains %v, want %v (no temp files left behind)", dir, got, want)
	}
}

func TestSuggestNameKeepsSourceNameWithNewExtension(t *testing.T) {
	cases := map[string]string{
		"/a/b/Weekly sync.mp4": "Weekly sync.m4a",
		"/a/b/video123.mov":    "video123.m4a",
		"/a/b/noext":           "noext.m4a",
		"/a/b/a.b.c.mkv":       "a.b.c.m4a",
	}
	for in, want := range cases {
		if got := SuggestName(in, FormatM4A); got != want {
			t.Errorf("SuggestName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDropAsksSaveDialogInSourceFolderThenExtractsAAC(t *testing.T) {
	src := makeClip(t, "Weekly sync.mp4", "aac")
	dest := filepath.Join(t.TempDir(), "out.m4a")
	port := &fakePort{saveTo: dest}
	svc, rec := newService(t, port)

	res := svc.Run([]string{src}, "m4a")

	if res.Cancelled || len(res.Outcomes) != 1 || res.Outcomes[0].Kind != "" {
		t.Fatalf("unexpected result: %+v", res)
	}
	if res.Outcomes[0].Output != dest {
		t.Errorf("output = %q, want %q", res.Outcomes[0].Output, dest)
	}
	req := port.saveReqs[0]
	if req.Dir != filepath.Dir(src) || req.Name != "Weekly sync.m4a" {
		t.Errorf("save dialog opened with %+v, want dir %q and name %q", req, filepath.Dir(src), "Weekly sync.m4a")
	}
	assertM4A(t, dest, "aac")
	noStrays(t, filepath.Dir(dest), "out.m4a")

	phases := rec.phases()
	if len(phases) != 2 || phases[0] != PhaseWorking || phases[1] != PhaseDone {
		t.Errorf("phases = %v, want [working done]", phases)
	}
	if last := rec.events[len(rec.events)-1]; last.Output != dest || last.Name != "out.m4a" {
		t.Errorf("done event = %+v", last)
	}
}

func TestAACIsStreamCopiedNotReencoded(t *testing.T) {
	src := makeClip(t, "clip.mp4", "aac")
	dest := filepath.Join(t.TempDir(), "clip.m4a")
	svc, _ := newService(t, &fakePort{saveTo: dest})

	res := svc.Run([]string{src}, "m4a")
	if res.Outcomes[0].Kind != "" {
		t.Fatalf("failed: %+v", res)
	}
	// A stream copy is logged as "Stream #0:0 -> #0:0 (copy)".
	if !strings.Contains(res.Outcomes[0].Log, "(copy)") {
		t.Errorf("expected a stream copy in the log:\n%s", res.Outcomes[0].Log)
	}
}

func TestNonAACAudioFallsBackToAACEncoding(t *testing.T) {
	src := makeClip(t, "clip.mkv", "pcm_s16le")
	dest := filepath.Join(t.TempDir(), "clip.m4a")
	svc, _ := newService(t, &fakePort{saveTo: dest})

	res := svc.Run([]string{src}, "m4a")
	if res.Outcomes[0].Kind != "" {
		t.Fatalf("failed: %+v", res)
	}
	assertM4A(t, dest, "aac")
}

func TestCancellingTheSaveDialogReturnsToIdleAndWritesNothing(t *testing.T) {
	src := makeClip(t, "clip.mp4", "aac")
	port := &fakePort{saveTo: ""}
	svc, rec := newService(t, port)

	res := svc.Run([]string{src}, "m4a")

	if !res.Cancelled || len(res.Outcomes) != 0 {
		t.Errorf("result = %+v, want cancelled", res)
	}
	if p := rec.phases(); len(p) != 1 || p[0] != PhaseIdle {
		t.Errorf("phases = %v, want [idle]", p)
	}
	noStrays(t, filepath.Dir(src), "clip.mp4")
}

func TestMissingExtensionIsAdded(t *testing.T) {
	src := makeClip(t, "clip.mp4", "aac")
	dir := t.TempDir()
	svc, _ := newService(t, &fakePort{saveTo: filepath.Join(dir, "typed name")})

	res := svc.Run([]string{src}, "m4a")
	if got := res.Outcomes[0].Output; filepath.Base(got) != "typed name.m4a" {
		t.Errorf("output = %q", got)
	}
}

func TestPickUsesTheFilePickerThenRuns(t *testing.T) {
	src := makeClip(t, "clip.mp4", "aac")
	dest := filepath.Join(t.TempDir(), "clip.m4a")
	svc, _ := newService(t, &fakePort{picks: []string{src}, saveTo: dest})

	if res := svc.Pick(); res.Outcomes[0].Output != dest {
		t.Fatalf("result = %+v", res)
	}
	assertM4A(t, dest, "aac")
}

func TestCancellingThePickerDoesNothing(t *testing.T) {
	port := &fakePort{}
	svc, rec := newService(t, port)
	if res := svc.Pick(); !res.Cancelled {
		t.Errorf("result = %+v, want cancelled", res)
	}
	if len(port.saveReqs) != 0 || len(rec.events) != 0 {
		t.Errorf("expected no dialog and no events")
	}
}

func TestVideoWithoutAudioReportsNoAudioAndLeavesNoFile(t *testing.T) {
	src := makeClip(t, "silent.mp4", "")
	dir := t.TempDir()
	svc, rec := newService(t, &fakePort{saveTo: filepath.Join(dir, "silent.m4a")})

	res := svc.Run([]string{src}, "m4a")

	if res.Outcomes[0].Kind != KindNoAudio {
		t.Fatalf("kind = %q, want no-audio", res.Outcomes[0].Kind)
	}
	if last := rec.events[len(rec.events)-1]; last.Phase != PhaseError || last.Message != i18n.Load(i18n.EN).T("error.no-audio") {
		t.Errorf("last event = %+v", last)
	}
	noStrays(t, dir)
}

func TestNonMediaFileFailsCleanly(t *testing.T) {
	src := filepath.Join(t.TempDir(), "notes.mp4")
	os.WriteFile(src, []byte("hello, not a video"), 0o644)
	dir := t.TempDir()
	svc, _ := newService(t, &fakePort{saveTo: filepath.Join(dir, "notes.m4a")})

	res := svc.Run([]string{src}, "m4a")

	if res.Outcomes[0].Kind != KindFailed || res.Outcomes[0].Log == "" {
		t.Errorf("outcome = %+v, want failed with a log", res.Outcomes[0])
	}
	noStrays(t, dir)
}

func TestUnwritableDestinationFailsWithoutPartialFile(t *testing.T) {
	src := makeClip(t, "clip.mp4", "aac")
	svc, _ := newService(t, &fakePort{saveTo: filepath.Join(t.TempDir(), "missing", "clip.m4a")})

	if res := svc.Run([]string{src}, "m4a"); res.Outcomes[0].Kind != KindFailed {
		t.Errorf("outcome = %+v, want failed", res.Outcomes[0])
	}
}

func TestSeveralFilesAreNotSupportedYet(t *testing.T) {
	a, b := makeClip(t, "a.mp4", "aac"), makeClip(t, "b.mp4", "aac")
	port := &fakePort{saveTo: "x.m4a"}
	svc, _ := newService(t, port)

	res := svc.Run([]string{a, b}, "m4a")
	if res.Outcomes[0].Kind != KindBatchUnsupported || len(port.saveReqs) != 0 {
		t.Errorf("result = %+v", res)
	}
}

func TestMissingFFmpegIsReportedAsFailure(t *testing.T) {
	src := makeClip(t, "clip.mp4", "aac")
	dir := t.TempDir()
	svc := New(Config{
		Locate: func() (string, error) { return "", errors.New("no ffmpeg") },
		Port:   &fakePort{saveTo: filepath.Join(dir, "clip.m4a")},
		Lang:   i18n.EN,
	})
	if res := svc.Run([]string{src}, "m4a"); res.Outcomes[0].Kind != KindFailed {
		t.Errorf("outcome = %+v", res.Outcomes[0])
	}
}

func TestRevealGoesThroughThePort(t *testing.T) {
	port := &fakePort{}
	svc, _ := newService(t, port)
	if err := svc.Reveal("/x/y.m4a"); err != nil || len(port.revealed) != 1 || port.revealed[0] != "/x/y.m4a" {
		t.Errorf("revealed = %v, err = %v", port.revealed, err)
	}
}

func TestCancelledContextLeavesNoPartialFile(t *testing.T) {
	src := makeClip(t, "clip.mp4", "aac")
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, kind, err := extractM4A(ctx, ffmpegPath(t), src, filepath.Join(dir, "clip.m4a"))
	if err == nil || kind != KindCancelled {
		t.Errorf("kind = %q err = %v, want cancelled", kind, err)
	}
	noStrays(t, dir)
}

// Every error kind that reaches the user needs a message in every language.
func TestEveryErrorKindHasAMessageInEveryLanguage(t *testing.T) {
	for _, lang := range i18n.Langs {
		cat := i18n.Load(lang)
		for _, k := range MessageKinds {
			if cat["error."+string(k)] == "" {
				t.Errorf("%s catalog has no message for error kind %q", lang, k)
			}
		}
	}
}
