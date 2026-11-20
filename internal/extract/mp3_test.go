package extract

import (
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/motdiem/AudioExtractor/internal/i18n"
	"github.com/motdiem/AudioExtractor/internal/settings"
)

// makeAudio generates an audio-only AAC file of the given length and channel count.
func makeAudio(t *testing.T, name string, seconds, channels int) string {
	return makeAudioWith(t, name, seconds, channels, "aac", "44100")
}

// makeAudioWith is makeAudio with an explicit codec and sample rate. Raw PCM at
// a low rate is the cheapest way to get a long input that still takes seconds
// to encode to MP3.
func makeAudioWith(t *testing.T, name string, seconds, channels int, codec, rate string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	args := []string{"-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=" + strconv.Itoa(seconds),
		"-ac", strconv.Itoa(channels), "-ar", rate, "-c:a", codec, path}
	if out, err := exec.Command(fixtureFFmpeg(t), args...).CombinedOutput(); err != nil {
		t.Fatalf("generating %s: %v\n%s", name, err, out)
	}
	return path
}

var (
	mp3Re = regexp.MustCompile(`Audio: mp3.*?, (mono|stereo).*?(\d+) kb/s`)
)

func assertMP3(t *testing.T, path, wantLayout string) {
	t.Helper()
	log := probeOut(t, path)
	m := mp3Re.FindStringSubmatch(log)
	if m == nil {
		t.Fatalf("not an mp3 audio file:\n%s", log)
	}
	if m[1] != wantLayout {
		t.Errorf("channels = %s, want %s\n%s", m[1], wantLayout, log)
	}
	if m[2] != "128" {
		t.Errorf("bitrate = %s kb/s, want 128\n%s", m[2], log)
	}
	if videoRe.MatchString(log) {
		t.Errorf("output has a video stream\n%s", log)
	}
}

func TestMP3IsEncodedAt128kAndKeepsStereo(t *testing.T) {
	src := makeAudio(t, "call.m4a", 5, 2)
	dest := filepath.Join(t.TempDir(), "call.mp3")
	port := &fakePort{saveTo: dest}
	svc, _ := newService(t, port)

	res := svc.Run([]string{src}, "mp3")

	if res.Outcomes[0].Kind != "" {
		t.Fatalf("failed: %+v", res.Outcomes[0])
	}
	assertMP3(t, dest, "stereo")
	if req := port.saveReqs[0]; req.Name != "call.mp3" || req.Extension != "mp3" {
		t.Errorf("save dialog asked for %+v, want call.mp3", req)
	}
	noStrays(t, filepath.Dir(dest), "call.mp3")
}

func TestMP3KeepsMono(t *testing.T) {
	src := makeAudio(t, "mono.m4a", 5, 1)
	dest := filepath.Join(t.TempDir(), "mono.mp3")
	svc, _ := newService(t, &fakePort{saveTo: dest})

	if res := svc.Run([]string{src}, "mp3"); res.Outcomes[0].Kind != "" {
		t.Fatalf("failed: %+v", res.Outcomes[0])
	}
	assertMP3(t, dest, "mono")
}

func TestMP3AddsMissingExtension(t *testing.T) {
	src := makeAudio(t, "a.m4a", 2, 2)
	svc, _ := newService(t, &fakePort{saveTo: filepath.Join(t.TempDir(), "typed")})
	res := svc.Run([]string{src}, "mp3")
	if got := filepath.Base(res.Outcomes[0].Output); got != "typed.mp3" {
		t.Errorf("output = %q", got)
	}
}

func TestProgressIsReportedAndEndsAt100(t *testing.T) {
	src := makeAudio(t, "long.m4a", 120, 2)
	dest := filepath.Join(t.TempDir(), "long.mp3")
	svc, rec := newService(t, &fakePort{saveTo: dest})

	res := svc.Run([]string{src}, "mp3")
	if res.Outcomes[0].Kind != "" {
		t.Fatalf("failed: %+v", res.Outcomes[0])
	}

	var percents []int
	for _, e := range rec.events {
		if e.Phase == PhaseWorking && e.Percent >= 0 {
			percents = append(percents, e.Percent)
		}
	}
	if len(percents) == 0 || percents[len(percents)-1] != 100 {
		t.Errorf("progress percents = %v, want some ending at 100", percents)
	}
	for i := 1; i < len(percents); i++ {
		if percents[i] < percents[i-1] {
			t.Errorf("progress went backwards: %v", percents)
		}
	}
	// 120 s of audio encodes in well under a second, so throttling should leave
	// only a handful of events.
	if len(percents) > 12 {
		t.Errorf("%d progress events for a short job; expected throttling", len(percents))
	}
	if first := rec.events[0]; first.Phase != PhaseWorking || first.Percent != -1 {
		t.Errorf("first event = %+v, want working with unknown percent", first)
	}
}

func TestCancelMidEncodeKillsFFmpegAndLeavesNoFile(t *testing.T) {
	src := makeAudioWith(t, "huge.wav", 1200, 2, "pcm_s16le", "16000") // 20 min: long enough that the encode is still running when we cancel
	dir := t.TempDir()
	dest := filepath.Join(dir, "huge.mp3")

	var svc *Service
	var once sync.Once
	rec := &recorder{}
	bin := ffmpegPath(t)
	svc = New(Config{
		Locate: func() (string, error) { return bin, nil },
		Port:   &fakePort{saveTo: dest},
		Lang:   i18n.EN,
		Emit: func(name string, data any) {
			rec.emit(name, data)
			if e := data.(StateEvent); e.Phase == PhaseWorking && e.Percent >= 0 {
				once.Do(func() { go svc.Cancel() }) // what the Cancel button does
			}
		},
	})

	start := time.Now()
	res := svc.Run([]string{src}, "mp3")

	if !res.Cancelled || len(res.Outcomes) != 1 || res.Outcomes[0].Kind != KindCancelled {
		t.Fatalf("result = %+v, want cancelled", res)
	}
	if time.Since(start) > 20*time.Second {
		t.Errorf("cancel took %v; ffmpeg was not killed promptly", time.Since(start))
	}
	if last := rec.events[len(rec.events)-1]; last.Phase != PhaseIdle {
		t.Errorf("last event = %+v, want idle (silent)", last)
	}
	noStrays(t, dir) // neither the output nor the .part file
}

func TestFormatIsRememberedAcrossLaunches(t *testing.T) {
	path := filepath.Join(t.TempDir(), "AudioExtractor", "settings.json")
	cfg := func() Config {
		return Config{Port: &fakePort{}, Lang: i18n.EN, Settings: settings.New(path)}
	}

	first := New(cfg())
	if first.Format() != "m4a" {
		t.Errorf("default format = %q, want m4a", first.Format())
	}
	if err := first.SetFormat("mp3"); err != nil {
		t.Fatal(err)
	}
	if err := first.SetFormat("flac"); err == nil {
		t.Error("an unknown format should be rejected")
	}

	second := New(cfg()) // a new launch
	if second.Format() != "mp3" {
		t.Errorf("format after restart = %q, want mp3", second.Format())
	}
}

func TestSavedFormatDrivesPickAndSaveDialog(t *testing.T) {
	src := makeAudio(t, "talk.m4a", 2, 2)
	dest := filepath.Join(t.TempDir(), "talk.mp3")
	port := &fakePort{picks: []string{src}, saveTo: dest}
	svc, _ := newService(t, port)
	svc.SetFormat("mp3")

	svc.Pick()

	if req := port.saveReqs[0]; req.Name != "talk.mp3" || req.Extension != "mp3" || req.FilterName != "MP3 audio" {
		t.Errorf("save dialog = %+v", req)
	}
	assertMP3(t, dest, "stereo")
}

func TestParseProgressLine(t *testing.T) {
	cases := []struct {
		line    string
		want    time.Duration
		end, ok bool
	}{
		{"out_time_us=1500000", 1500 * time.Millisecond, false, true},
		{"out_time_us=N/A", 0, false, false},
		{"out_time_us=-9223372036854775807", 0, false, false},
		{"progress=continue", 0, false, false},
		{"progress=end", 0, true, true},
		{"bitrate=128.0kbits/s", 0, false, false},
	}
	for _, c := range cases {
		got, end, ok := parseProgressLine(c.line)
		if got != c.want || end != c.end || ok != c.ok {
			t.Errorf("parseProgressLine(%q) = %v %v %v", c.line, got, end, ok)
		}
	}
}

func TestParseDuration(t *testing.T) {
	if got := parseDuration("  Duration: 01:02:03.50, start: 0.0"); got != time.Hour+2*time.Minute+3500*time.Millisecond {
		t.Errorf("got %v", got)
	}
	if got := parseDuration("Duration: N/A"); got != 0 {
		t.Errorf("got %v", got)
	}
}
