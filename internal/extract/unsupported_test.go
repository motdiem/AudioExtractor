package extract

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const invalidData = "[in#0 @ 0x1] Error opening input: Invalid data found when processing input\nError opening input files: Invalid data found when processing input"

func TestClassifyUnopenable(t *testing.T) {
	flac := []byte("fLaC\x00\x00\x00\x22")
	riff := []byte("RIFF\x00\x00\x00\x00AVI LIST")
	cases := []struct {
		name  string
		file  string
		bytes []byte
		diag  string
		want  ErrorKind
	}{
		{"valid flac, no demuxer", "a.flac", flac, invalidData, KindUnsupportedFormat},
		{"valid avi, no demuxer, upper case", "A.AVI", riff, invalidData, KindUnsupportedFormat},
		{"unknown binary named ogg", "a.ogg", []byte{0, 0x81, 0xfe}, invalidData, KindUnsupportedFormat},
		{"text named avi", "a.avi", []byte("hello there"), invalidData, KindNotMedia},
		{"empty flac", "a.flac", nil, invalidData, KindUnreadable},
		{"flac, but ffmpeg said something else", "a.flac", flac, "", KindUnreadable},
		{"damaged mp4 stays damaged", "a.mp4", []byte("\x00\x00\x00\x20ftypisom"), invalidData, KindUnreadable},
		{"zoom raw stays damaged", "a.zoom", []byte{0, 0x81}, invalidData, KindUnreadable},
	}
	for _, c := range cases {
		if got := classifyUnopenable(c.file, sniff(c.bytes), c.diag); got != c.want {
			t.Errorf("%s: = %q, want %q", c.name, got, c.want)
		}
	}
}

// hasDemuxer reports whether the ffmpeg at bin can read the named format.
func hasDemuxer(t *testing.T, bin, name string) bool {
	t.Helper()
	out, err := exec.Command(bin, "-hide_banner", "-demuxers").CombinedOutput()
	if err != nil {
		t.Fatalf("ffmpeg -demuxers: %v\n%s", err, out)
	}
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) >= 2 && strings.Contains(f[0], "D") {
			for _, n := range strings.Split(f[1], ",") {
				if n == name {
					return true
				}
			}
		}
	}
	return false
}

// A valid FLAC and AVI, run against the ffmpeg under test: the slim bundled
// build has neither demuxer and must say "not supported", not "damaged". A
// full ffmpeg (Homebrew) reads them, and the extraction simply succeeds.
// Run with AUDIOEXTRACTOR_FFMPEG pointing at the slim build to test what ships.
func TestValidButUnsupportedFormatsSayNotSupported(t *testing.T) {
	bin := ffmpegPath(t)
	for _, c := range []struct{ file, demuxer string }{{"talk.flac", "flac"}, {"talk.avi", "avi"}} {
		t.Run(c.file, func(t *testing.T) {
			src := filepath.Join(t.TempDir(), c.file)
			args := []string{"-f", "lavfi", "-i", "sine=frequency=440:duration=2"}
			if c.demuxer == "avi" {
				args = append(args, "-f", "lavfi", "-i", "color=c=blue:s=64x64:r=10:d=2", "-c:v", "mpeg4", "-c:a", "mp3")
			}
			runFFmpegForTest(t, append(args, src)...)
			if hasDemuxer(t, bin, c.demuxer) {
				svc, _ := newService(t, &fakePort{saveTo: filepath.Join(t.TempDir(), "out.m4a")})
				if res := svc.Run([]string{src}, "m4a"); len(res.Outcomes) != 1 || res.Outcomes[0].Kind != "" {
					t.Errorf("full ffmpeg: outcomes = %+v, want a success", res.Outcomes)
				}
				return
			}
			runExpecting(t, src, t.TempDir(), KindUnsupportedFormat)
		})
	}
}
