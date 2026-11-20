package extract

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"unicode/utf8"
)

// This file turns evidence into an ErrorKind. The evidence, in the order the
// extraction collects it:
//
//  1. Go pre-checks: can we open the source, can we create a file next to the
//     destination? (classifyReadError, classifyWriteError)
//  2. What ffmpeg says: no audio stream, "moov atom not found", a "partial
//     file" warning, read from its diagnostic lines only. (diagnostics,
//     logSaysInputBroken, logSaysPartial, logSaysWriteFailed)
//  3. What the file looks like, to tell "a video that is damaged or still
//     being written" from "not a media file at all". (sniff, classifyBroken)
//
// Everything here is a pure function of its inputs so it can be unit-tested
// without ffmpeg or a real disk fault.

// classifyReadError maps a failure to open the source file. A file that is
// missing, unreadable or still being written is, to the user, the same
// situation: it is not usable yet.
func classifyReadError(err error) ErrorKind {
	if err == nil {
		return ""
	}
	return KindUnreadable
}

// classifyWriteError maps a failure to create or replace the output file.
// replacing is true when the destination already existed, which is when a
// lock held by another program (a player, a sync client) is the likely cause.
func classifyWriteError(err error, replacing bool) ErrorKind {
	switch {
	case err == nil:
		return ""
	case replacing && isLockError(err):
		return KindOutputLocked
	case errors.Is(err, os.ErrPermission),
		errors.Is(err, os.ErrNotExist), // folder gone: unplugged drive, offline cloud folder
		errors.Is(err, syscall.ENOSPC),
		errors.Is(err, syscall.EROFS),
		errors.Is(err, syscall.ENOTDIR),
		isDiskFull(err):
		return KindDestinationUnwritable
	}
	return KindFailed
}

// Phrases in ffmpeg's log, lower-cased. They are only looked for in
// ffmpeg's diagnostic lines (see diagnostics): the log also echoes the file
// paths and the files' metadata, and a folder called "Permission denied
// tests" must not turn into an error.
var (
	brokenInputPhrases = []string{
		"invalid data found when processing input",
		"moov atom not found",
	}
	writeFailurePhrases = []string{
		"no space left on device",
		"read-only file system",
		"permission denied",
		"disk quota exceeded",
	}
)

// partialRe matches the exact diagnostics ffmpeg prints when it runs out of
// data in a file it could open. ffmpeg exits 0 on a file cut off after its
// header (a "faststart" MP4) and happily writes the part it could read. We
// do not want to hand the user half a meeting as if it were all of it.
//
// Every one of them is a log line of its own that starts with the
// "[component @ 0x...]" prefix, which is what keeps a path or a title such as
// "Partial file reviews" from matching:
//
//	[mov,mp4,m4a,3gp,3g2,mj2 @ 0x...] stream 1, offset 0x27f4: partial file
//	[in#0/mov,mp4,m4a,3gp,3g2,mj2 @ 0x...] corrupt input packet in stream 1
//	[mov,mp4,m4a,3gp,3g2,mj2 @ 0x...] Packet corrupt (stream = 1, dts = 36864).
var partialRe = regexp.MustCompile(`(?mi)^\[[^\]\n]+\] (?:stream \d+, offset 0x[0-9a-f]+: partial file|corrupt input packet in stream \d+|packet corrupt \(stream = \d+)`)

// diagnostics keeps the lines of an ffmpeg log that are ffmpeg talking, not
// ffmpeg describing the files: the "Input #0 ... from '<path>':" and
// "Output #0" headers and everything indented under them (metadata, streams)
// are dropped, and any of paths (the source, the output) is blanked out of
// the remaining lines ("Error opening input file <path>.").
func diagnostics(log string, paths ...string) string {
	var keep []string
	for _, line := range strings.Split(log, "\n") {
		trimmed := strings.TrimRight(line, "\r")
		if trimmed == "" || trimmed[0] == ' ' || trimmed[0] == '\t' ||
			strings.HasPrefix(trimmed, "Input #") || strings.HasPrefix(trimmed, "Output #") {
			continue
		}
		for _, p := range paths {
			if p != "" {
				trimmed = strings.ReplaceAll(trimmed, p, "")
			}
		}
		keep = append(keep, trimmed)
	}
	return strings.Join(keep, "\n")
}

func logHasAny(log string, phrases []string) bool {
	l := strings.ToLower(log)
	for _, p := range phrases {
		if strings.Contains(l, p) {
			return true
		}
	}
	return false
}

// logSaysInputBroken reports whether ffmpeg rejected the input as undecodable.
// Pass the log through diagnostics first.
func logSaysInputBroken(log string) bool { return logHasAny(log, brokenInputPhrases) }

// logSaysPartial reports whether ffmpeg read the input but ran out of data.
func logSaysPartial(log string) bool { return partialRe.MatchString(log) }

// logSaysWriteFailed reports whether ffmpeg itself could not write the output
// (the disk filled up mid-extraction, for example). Pass the log through
// diagnostics first.
func logSaysWriteFailed(log string) bool { return logHasAny(log, writeFailurePhrases) }

// Looks is what sniff concluded about a file's first bytes.
type Looks int

const (
	LooksEmpty   Looks = iota // zero bytes: a download that never started
	LooksMedia                // a known audio/video signature
	LooksText                 // plain text
	LooksUnknown              // binary, but no signature we know
)

// mediaExts are extensions that mean "this was meant to be audio or video".
// ".zoom" is Zoom's raw recording, before it has been converted to MP4.
var mediaExts = map[string]bool{
	".mp4": true, ".m4a": true, ".m4v": true, ".mov": true, ".mkv": true, ".webm": true,
	".avi": true, ".mp3": true, ".wav": true, ".aac": true, ".flac": true, ".ogg": true,
	".wma": true, ".wmv": true, ".3gp": true, ".ts": true, ".zoom": true,
	".oga": true, ".asf": true, ".mts": true, ".m2ts": true, ".aiff": true, ".aif": true,
}

// sniff classifies the first bytes of a file.
func sniff(head []byte) Looks {
	if len(head) == 0 {
		return LooksEmpty
	}
	if len(head) >= 8 {
		switch string(head[4:8]) {
		case "ftyp", "moov", "mdat", "free", "wide": // MP4, MOV, M4A, 3GP boxes
			return LooksMedia
		}
	}
	switch {
	case hasPrefix(head, "RIFF"), // WAV, AVI
		hasPrefix(head, "ID3"), // MP3 with a tag
		hasPrefix(head, "OggS"),
		hasPrefix(head, "fLaC"),
		hasPrefix(head, "\x1a\x45\xdf\xa3"),                      // Matroska, WebM
		hasPrefix(head, "\x30\x26\xb2\x75"),                      // ASF: WMA, WMV
		head[0] == 0xff && len(head) > 1 && head[1]&0xe0 == 0xe0: // MPEG audio / ADTS frame sync
		return LooksMedia
	}
	if isText(head) {
		return LooksText
	}
	return LooksUnknown
}

func hasPrefix(b []byte, s string) bool { return len(b) >= len(s) && string(b[:len(s)]) == s }

// isText reports whether b is plain text: valid UTF-8 (a character cut off at
// the end of the sample is fine) with no NULs or other control characters.
func isText(b []byte) bool {
	for i := 0; i < utf8.UTFMax && len(b) > 0 && !utf8.Valid(b); i++ {
		b = b[:len(b)-1]
	}
	if len(b) == 0 || !utf8.Valid(b) {
		return false
	}
	for _, r := range string(b) {
		if r < 0x20 && r != '\n' && r != '\r' && r != '\t' && r != '\f' {
			return false
		}
	}
	return true
}

// unsupportedExts are media formats that AudioExtractor's slim ffmpeg has no
// demuxer for: a valid file of one of these kinds is "not supported", not
// "damaged". (inputs.go's folderExts is the supported set.) 3GP is not here:
// the MOV demuxer reads it, so a 3GP that fails to open really is damaged.
var unsupportedExts = map[string]bool{
	".avi": true, ".flac": true, ".ogg": true, ".oga": true, ".wma": true, ".wmv": true,
	".asf": true, ".ts": true, ".mts": true, ".m2ts": true, ".aiff": true, ".aif": true,
}

// classifyUnopenable decides for a file ffmpeg could not open at all. diag is
// the diagnostics of ffmpeg's log. A known media extension we do not support,
// rejected by ffmpeg as invalid data (which is what a missing demuxer looks
// like), is an unsupported format; the rest is classifyBroken's call. Text and
// empty files keep their own, more useful, messages.
func classifyUnopenable(name string, looks Looks, diag string) ErrorKind {
	ext := strings.ToLower(filepath.Ext(name))
	if unsupportedExts[ext] && looks != LooksText && looks != LooksEmpty && logSaysInputBroken(diag) {
		return KindUnsupportedFormat
	}
	return classifyBroken(name, looks)
}

// classifyBroken decides, for a file ffmpeg could not read, between
// "unreadable" (a media file that is damaged, truncated or still being
// written) and "not-media" (something else entirely).
func classifyBroken(name string, looks Looks) ErrorKind {
	ext := strings.ToLower(filepath.Ext(name))
	switch {
	case ext == ".zoom", looks == LooksEmpty, looks == LooksMedia:
		return KindUnreadable
	case looks == LooksText:
		return KindNotMedia
	case mediaExts[ext]: // binary garbage under a media name: most likely damaged
		return KindUnreadable
	}
	return KindNotMedia
}

// sniffFile reads the start of path and classifies it.
func sniffFile(path string) Looks {
	f, err := os.Open(path)
	if err != nil {
		return LooksUnknown
	}
	defer f.Close()
	buf := make([]byte, 4096)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return LooksUnknown
	}
	return sniff(buf[:n])
}
