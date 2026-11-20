package extract

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	audioStreamRe = regexp.MustCompile(`(?m)^\s*Stream #\d+:\d+.*: Audio: ([A-Za-z0-9_]+)`)
	anyStreamRe   = regexp.MustCompile(`(?m)^\s*Stream #\d+:\d+`)
	durationRe    = regexp.MustCompile(`Duration: (\d+):(\d+):(\d+(?:\.\d+)?)`)
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

// probeInfo is what `ffmpeg -i file` tells us about an input.
type probeInfo struct {
	audioCodec string        // "" if there is no audio stream
	anyStream  bool          // ffmpeg found at least one stream
	duration   time.Duration // 0 when unknown
	log        string
}

// probe inspects src. `ffmpeg -i file` with no output exits with an error by
// design; we only read its stream listing.
func probe(ctx context.Context, bin, src string) (probeInfo, error) {
	log, runErr := runFFmpeg(ctx, bin, "-i", src)
	info := probeInfo{log: log}
	if ctx.Err() != nil {
		return info, ctx.Err()
	}
	if m := audioStreamRe.FindStringSubmatch(log); m != nil {
		info.audioCodec = m[1]
	}
	info.anyStream = anyStreamRe.MatchString(log)
	info.duration = parseDuration(log)
	if !info.anyStream && runErr != nil && !strings.Contains(log, "At least one output file") {
		return info, fmt.Errorf("ffmpeg could not read %s", filepath.Base(src))
	}
	return info, nil
}

// parseDuration reads "Duration: 00:01:02.50" from ffmpeg's log.
func parseDuration(log string) time.Duration {
	m := durationRe.FindStringSubmatch(log)
	if m == nil {
		return 0
	}
	h, _ := strconv.Atoi(m[1])
	mins, _ := strconv.Atoi(m[2])
	sec, _ := strconv.ParseFloat(m[3], 64)
	return time.Duration((float64(h*3600+mins*60) + sec) * float64(time.Second))
}

// runWithProgress runs ffmpeg with `-progress pipe:1` and calls onProgress with
// each out_time it reports (and with end=true on the final "progress=end").
// It returns the stderr log, which is what "Copy details" will show.
func runWithProgress(ctx context.Context, bin string, onProgress func(outTime time.Duration, end bool), args ...string) (string, error) {
	full := append([]string{"-hide_banner", "-nostdin", "-nostats", "-progress", "pipe:1"}, args...)
	cmd := exec.CommandContext(ctx, bin, full...)
	hideWindow(cmd)
	var logBuf bytes.Buffer
	cmd.Stderr = &logBuf
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}
	// Read until ffmpeg closes stdout (it exited or was killed), then Wait.
	sc := bufio.NewScanner(stdout)
	for sc.Scan() {
		if t, end, ok := parseProgressLine(sc.Text()); ok {
			onProgress(t, end)
		}
	}
	err = cmd.Wait()
	return logBuf.String(), err
}

// parseProgressLine understands the two lines we care about in ffmpeg's
// key=value progress stream: out_time_us=<microseconds> and progress=end.
// out_time_us is "N/A" or negative before the first frame; those are ignored.
func parseProgressLine(line string) (outTime time.Duration, end, ok bool) {
	switch {
	case line == "progress=end":
		return 0, true, true
	case strings.HasPrefix(line, "out_time_us="):
		us, err := strconv.ParseInt(strings.TrimPrefix(line, "out_time_us="), 10, 64)
		if err != nil || us < 0 {
			return 0, false, false
		}
		return time.Duration(us) * time.Microsecond, false, true
	}
	return 0, false, false
}

// extract writes the audio of src to dest in the given format. For M4A, AAC
// audio is stream-copied and anything else is encoded to AAC; MP3 is 128 kbps
// CBR (libmp3lame keeps mono as mono and stereo as stereo). Output goes to a
// temporary file in dest's folder and is renamed into place, so dest never
// holds a partial file, and the temp file is removed on every failure path,
// cancellation included. onProgress receives 0-100 and may be nil.
func extract(ctx context.Context, bin, src, dest string, f Format, onProgress func(percent int)) (log string, kind ErrorKind, err error) {
	info, err := probe(ctx, bin, src)
	log = info.log
	if err != nil {
		if ctx.Err() != nil {
			return log, KindCancelled, err
		}
		return log, KindFailed, err
	}
	if !info.anyStream {
		return log, KindFailed, fmt.Errorf("no streams found in %s", filepath.Base(src))
	}
	if info.audioCodec == "" {
		return log, KindNoAudio, fmt.Errorf("no audio stream in %s", filepath.Base(src))
	}

	tmp, err := os.CreateTemp(filepath.Dir(dest), ".audioextractor-*.part")
	if err != nil {
		return log, KindFailed, err
	}
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath) // no-op after a successful rename

	var codecArgs []string
	var container string
	switch f {
	case FormatMP3:
		codecArgs, container = []string{"-c:a", "libmp3lame", "-b:a", "128k"}, "mp3"
	default:
		codecArgs, container = []string{"-c:a", "copy"}, "ipod"
		if info.audioCodec != "aac" {
			codecArgs = []string{"-c:a", "aac", "-b:a", "160k"}
		}
	}
	args := append([]string{"-y", "-i", src, "-vn", "-sn", "-dn", "-map", "0:a:0"}, codecArgs...)
	args = append(args, "-f", container, tmpPath)

	report := func(outTime time.Duration, end bool) {
		if onProgress == nil {
			return
		}
		switch {
		case end:
			onProgress(100)
		case info.duration > 0:
			// Never claim 100 before ffmpeg says it is finished.
			onProgress(min(99, int(outTime*100/info.duration)))
		}
	}
	runLog, err := runWithProgress(ctx, bin, report, args...)
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
