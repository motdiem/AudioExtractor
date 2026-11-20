// Package notices makes the third-party licence texts reachable from the app.
//
// The release ships them in three places: inside the macOS app bundle, next to
// the Windows exe in the zip, and embedded in the binary itself so that a lone
// exe copied elsewhere still carries them. This package serves the last one:
// the "Licences" link on the idle screen writes the text to a temporary file
// and opens it in the default viewer.
package notices

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Name is the file name of the notice, in the repo root and in the archives.
const Name = "THIRD_PARTY_NOTICES.txt"

// licencesDir is where the full licence texts live, relative to the root of
// the embedded file system.
const licencesDir = "third_party/licenses"

// Text returns the notice followed by every licence text under
// third_party/licenses, each under a header line.
func Text(fsys fs.FS) (string, error) {
	notice, err := fs.ReadFile(fsys, Name)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.Write(notice)

	var files []string
	err = fs.WalkDir(fsys, licencesDir, func(path string, d fs.DirEntry, err error) error {
		// README.md documents the folder for maintainers; it is not a licence.
		if err == nil && !d.IsDir() && path != licencesDir+"/README.md" {
			files = append(files, path)
		}
		return err
	})
	if err != nil {
		return "", err
	}
	sort.Strings(files)
	for _, f := range files {
		body, err := fs.ReadFile(fsys, f)
		if err != nil {
			return "", err
		}
		b.WriteString("\n\n================================================================================\n")
		b.WriteString(strings.TrimPrefix(f, licencesDir+"/") + "\n")
		b.WriteString("================================================================================\n\n")
		b.Write(body)
	}
	b.WriteString("\n")
	return b.String(), nil
}

// Service is bound to the frontend; the "Licences" link calls Open.
type Service struct {
	// FS holds Name and third_party/licenses.
	FS fs.FS
	// OpenFile opens a file in the default viewer.
	OpenFile func(path string) error
	// Dir is where the text is written; empty means the OS temp folder.
	Dir string
}

// Open writes the full text to a temporary file and opens it. The file name is
// fixed, so repeated clicks overwrite one file instead of piling up.
func (s *Service) Open() error {
	text, err := Text(s.FS)
	if err != nil {
		return err
	}
	dir := s.Dir
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "AudioExtractor")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, Name)
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return err
	}
	return s.OpenFile(path)
}
