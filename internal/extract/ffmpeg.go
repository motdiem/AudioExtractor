package extract

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
)

var (
	audioStreamRe = regexp.MustCompile(`(?m)^\s*Stream #\d+:\d+.*: Audio: ([A-Za-z0-9_]+)`)
	anyStreamRe   = regexp.MustCompile(`(?m)^\s*Stream #\d+:\d+`)
)

// runFFmpeg runs ffmpeg with args and returns its combined output (the log).
func runFFmpeg(ctx context.Context, bin string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, bin, append([]string{"-hide_banner", "-nostdin"}, args...)...)
	hideWindow(cmd)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}

// probe reports the codec of the first audio stream ("" if none) and whether
// ffmpeg found any stream at all. `ffmpeg -i file` with no output exits with an
// error by design; we only read its stream listing.
func probe(ctx context.Context, bin, src string) (audioCodec string, anyStream bool, log string, err error) {
	log, runErr := runFFmpeg(ctx, bin, "-i", src)
	if ctx.Err() != nil {
		return "", false, log, ctx.Err()
	}
	if m := audioStreamRe.FindStringSubmatch(log); m != nil {
		audioCodec = m[1]
	}
	anyStream = anyStreamRe.MatchString(log)
	if !anyStream && runErr != nil && !bytes.Contains([]byte(log), []byte("At least one output file")) {
		return "", false, log, fmt.Errorf("ffmpeg could not read %s", filepath.Base(src))
	}
	return audioCodec, anyStream, log, nil
}

// extractM4A writes the audio of src to dest as M4A. AAC audio is stream-copied;
// anything else is encoded to AAC. It writes to a temporary file in dest's folder
// and renames it into place, so dest never holds a partial file.
func extractM4A(ctx context.Context, bin, src, dest string) (log string, kind ErrorKind, err error) {
	codec, anyStream, log, err := probe(ctx, bin, src)
	if err != nil {
		if ctx.Err() != nil {
			return log, KindCancelled, err
		}
		return log, KindFailed, err
	}
	if !anyStream {
		return log, KindFailed, fmt.Errorf("no streams found in %s", filepath.Base(src))
	}
	if codec == "" {
		return log, KindNoAudio, fmt.Errorf("no audio stream in %s", filepath.Base(src))
	}

	tmp, err := os.CreateTemp(filepath.Dir(dest), ".audioextractor-*.part")
	if err != nil {
		return log, KindFailed, err
	}
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath) // no-op after a successful rename

	codecArgs := []string{"-c:a", "copy"}
	if codec != "aac" {
		codecArgs = []string{"-c:a", "aac", "-b:a", "160k"}
	}
	args := append([]string{"-y", "-i", src, "-vn", "-sn", "-dn", "-map", "0:a:0"}, codecArgs...)
	args = append(args, "-f", "ipod", tmpPath)
	runLog, err := runFFmpeg(ctx, bin, args...)
	log += runLog
	if err != nil {
		if ctx.Err() != nil {
			return log, KindCancelled, ctx.Err()
		}
		return log, KindFailed, err
	}
	if err := os.Rename(tmpPath, dest); err != nil {
		return log, KindFailed, err
	}
	return log, "", nil
}
