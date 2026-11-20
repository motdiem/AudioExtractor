// Package ffmpeg ships AudioExtractor's slim ffmpeg build inside the Go
// executable and makes it runnable.
//
// The binary for the current OS is embedded at compile time from
// bin/<GOOS>/ (built by `wails3 task ffmpeg:mac`, and `ffmpeg:windows` later).
// Path writes it once to the per-user cache directory, in a folder named after
// the binary's SHA-256, so a new app version never runs a stale copy and no
// admin rights are needed:
//
//	macOS:   ~/Library/Caches/AudioExtractor/ffmpeg-<sha>/ffmpeg
//	Windows: %LocalAppData%\AudioExtractor\ffmpeg-<sha>\ffmpeg.exe
package ffmpeg

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// ErrNotBundled is returned by Path when this build of the app has no ffmpeg
// embedded (the binary was not built before compiling, or the OS is not
// supported yet). Callers such as tests may fall back to ffmpeg on PATH.
var ErrNotBundled = errors.New("ffmpeg: no ffmpeg binary bundled in this build (run `wails3 task ffmpeg:mac` and rebuild)")

// appCacheDir is the folder under os.UserCacheDir() that holds our files.
const appCacheDir = "AudioExtractor"

var (
	mu     sync.Mutex
	cached string
)

// Path returns the path of a ready-to-run ffmpeg executable, unpacking the
// embedded binary on first use. It is safe for concurrent use, also across
// several running copies of the app.
func Path() (string, error) {
	mu.Lock()
	defer mu.Unlock()
	if cached != "" {
		if _, err := os.Stat(cached); err == nil {
			return cached, nil
		}
	}
	data := embeddedBinary()
	if len(data) == 0 {
		return "", ErrNotBundled
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("ffmpeg: locate user cache directory: %w", err)
	}
	p, err := installer{
		root:    filepath.Join(base, appCacheDir),
		name:    binaryName,
		data:    data,
		prepare: prepareBinary,
	}.install()
	if err != nil {
		return "", err
	}
	cached = p
	return p, nil
}

// installer writes data as an executable file under root/ffmpeg-<sha>/name.
type installer struct {
	root string
	name string
	data []byte
	// prepare runs on the freshly written temp file before it is moved into
	// place (ad-hoc code signing on macOS). It may modify the file.
	prepare func(path string) error
}

// stampSuffix names the sidecar file holding the SHA-256 of the installed
// file. The installed file can differ from the embedded bytes when prepare
// re-signs it, so we record what we actually installed.
const stampSuffix = ".sha256"

func (in installer) install() (string, error) {
	sum := sha256.Sum256(in.data)
	dir := filepath.Join(in.root, "ffmpeg-"+hex.EncodeToString(sum[:])[:12])
	final := filepath.Join(dir, in.name)
	stamp := final + stampSuffix

	if valid(final, stamp) {
		return final, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("ffmpeg: create cache directory: %w", err)
	}

	tmp, err := os.CreateTemp(dir, in.name+".*.tmp")
	if err != nil {
		return "", fmt.Errorf("ffmpeg: create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once renamed

	_, werr := tmp.Write(in.data)
	cerr := tmp.Close()
	if err := errors.Join(werr, cerr); err != nil {
		return "", fmt.Errorf("ffmpeg: write binary: %w", err)
	}
	if err := os.Chmod(tmpName, 0o755); err != nil {
		return "", fmt.Errorf("ffmpeg: chmod binary: %w", err)
	}
	if in.prepare != nil {
		if err := in.prepare(tmpName); err != nil {
			return "", err
		}
	}
	installedSum, err := fileSHA256(tmpName)
	if err != nil {
		return "", fmt.Errorf("ffmpeg: hash binary: %w", err)
	}

	if err := os.Rename(tmpName, final); err != nil {
		// Another process may have installed it meanwhile (on Windows a
		// running exe cannot be replaced). Accept its copy if it is valid.
		if valid(final, stamp) {
			return final, nil
		}
		return "", fmt.Errorf("ffmpeg: install binary: %w", err)
	}
	if err := writeAtomic(stamp, []byte(installedSum)); err != nil {
		return "", fmt.Errorf("ffmpeg: write stamp: %w", err)
	}
	return final, nil
}

// valid reports whether path exists and matches the hash recorded in stamp.
func valid(path, stamp string) bool {
	want, err := os.ReadFile(stamp)
	if err != nil {
		return false
	}
	got, err := fileSHA256(path)
	return err == nil && bytes.Equal(bytes.TrimSpace(want), []byte(got))
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func writeAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	_, werr := tmp.Write(data)
	if err := errors.Join(werr, tmp.Close()); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
