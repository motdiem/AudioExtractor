//go:build !windows

package extract

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// probeRan only trusts a normal ffmpeg exit. The errors are real ones, from
// a shell standing in for ffmpeg.
func TestProbeRanTrustsOnlyANormalExit(t *testing.T) {
	run := func(script string) error { return exec.Command("/bin/sh", "-c", script).Run() }
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"exit 1: no output file, input read", run("exit 1"), true},
		{"exit 183: invalid data", run("exit 183"), true},
		{"success", nil, true},
		{"killed by SIGKILL", run("kill -KILL $$"), false},
		{"crashed with SIGSEGV", run("kill -SEGV $$"), false},
		{"did not start", exec.Command("/nonexistent/ffmpeg").Run(), false},
	}
	for _, c := range cases {
		if got := probeRan(c.err); got != c.want {
			t.Errorf("%s (%v): probeRan = %v, want %v", c.name, c.err, got, c.want)
		}
	}
}

// An ffmpeg that crashes while probing is a failure of ours, not a damaged
// file, even when its last words look like a verdict on the file.
func TestFFmpegCrashIsNotBlamedOnTheFile(t *testing.T) {
	dir := t.TempDir()
	fake := filepath.Join(dir, "ffmpeg")
	script := "#!/bin/sh\necho 'Invalid data found when processing input' >&2\nkill -SEGV $$\n"
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "clip.mp4")
	os.WriteFile(src, []byte("\x00\x00\x00\x20ftypisom"), 0o644)
	out := filepath.Join(t.TempDir(), "out.m4a")

	log, kind, err := extract(context.Background(), fake, src, out, FormatM4A, nil)
	if kind != KindFailed || err == nil {
		t.Errorf("kind = %q, err = %v, want %q", kind, err, KindFailed)
	}
	if !strings.Contains(log, "Invalid data") {
		t.Errorf("the log should still be kept for Copy details: %q", log)
	}
	noStrays(t, filepath.Dir(out))
}
