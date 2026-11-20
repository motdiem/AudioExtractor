package extract

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// folderExts are the extensions a folder expansion keeps. Files the user names
// explicitly are always attempted, whatever their extension.
var folderExts = map[string]bool{
	".mp4": true, ".mov": true, ".m4v": true, ".mkv": true, ".webm": true,
	".m4a": true, ".mp3": true, ".wav": true, ".aac": true,
}

// expandInputs turns raw paths (picker, drop, launch arguments) into the file
// list to process, in order:
//   - a file is kept whatever its extension, so that error classification can
//     explain a file that is not media;
//   - a folder is replaced by the media files at its top level only, sorted by
//     name case-insensitively; hidden files (leading dot) and files with other
//     extensions are ignored;
//   - duplicates (by cleaned path) are dropped, keeping the first.
//
// emptyFolders counts the folders that contributed nothing.
func expandInputs(paths []string) (files []string, emptyFolders int, err error) {
	seen := map[string]bool{}
	add := func(p string) {
		if k := filepath.Clean(p); !seen[k] {
			seen[k] = true
			files = append(files, k)
		}
	}
	for _, p := range paths {
		fi, err := os.Stat(p)
		if err != nil {
			return nil, 0, err
		}
		if !fi.IsDir() {
			add(p)
			continue
		}
		entries, err := os.ReadDir(p)
		if err != nil {
			return nil, 0, err
		}
		var names []string
		for _, e := range entries {
			name := e.Name()
			if strings.HasPrefix(name, ".") || !folderExts[strings.ToLower(filepath.Ext(name))] {
				continue
			}
			if st, err := os.Stat(filepath.Join(p, name)); err != nil || st.IsDir() {
				continue // dangling link or a folder named like a file
			}
			names = append(names, name)
		}
		sort.Slice(names, func(i, j int) bool {
			a, b := strings.ToLower(names[i]), strings.ToLower(names[j])
			if a != b {
				return a < b
			}
			return names[i] < names[j]
		})
		if len(names) == 0 {
			emptyFolders++
		}
		for _, n := range names {
			add(filepath.Join(p, n))
		}
	}
	return files, emptyFolders, nil
}

// Zoom's generic recording names, matched case-insensitively against the
// whole file name.
var zoomGenericRes = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^video\d+\.mp4$`),
	regexp.MustCompile(`(?i)^audio\d+\.m4a$`),
	regexp.MustCompile(`(?i)^zoom_\d+\.mp4$`),
	regexp.MustCompile(`(?i)^GMT\d{8}-\d{6}_Recording.*\.mp4$`),
}

// zoomFolderRe matches a Zoom meeting folder: "2026-11-20 10.00.00 Weekly sync".
var zoomFolderRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \d{2}\.\d{2}\.\d{2}`)

func isZoomGeneric(name string) bool {
	for _, re := range zoomGenericRes {
		if re.MatchString(name) {
			return true
		}
	}
	return false
}

// isZoomFolder reports whether dir looks like a Zoom meeting folder: its name
// starts with a "YYYY-MM-DD HH.MM.SS" timestamp, or it holds another Zoom
// generic file, or Zoom's "audio_only.m4a", next to the one being named.
func isZoomFolder(dir, self string) bool {
	if zoomFolderRe.MatchString(filepath.Base(dir)) {
		return true
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		n := e.Name()
		if n != self && (isZoomGeneric(n) || strings.EqualFold(n, "audio_only.m4a")) {
			return true
		}
	}
	return false
}

// invalidNameChars are not allowed in Windows file names (control characters
// are removed separately). We strip them on every OS so a name made on a Mac
// is still valid when the file moves to Windows.
const invalidNameChars = `<>:"/\|?*`

// cleanFolderName makes a folder name usable as a file name.
func cleanFolderName(s string) string {
	s = strings.Map(func(r rune) rune {
		if r < 0x20 || strings.ContainsRune(invalidNameChars, r) {
			return -1
		}
		return r
	}, s)
	return strings.TrimRight(strings.TrimSpace(s), ". ")
}

// SuggestName is the default output name for src: its own base name with the
// new extension, except that a Zoom generic name (video123.mp4, ...) inside a
// Zoom meeting folder takes the folder's name.
func SuggestName(src string, f Format) string {
	base := filepath.Base(src)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	if dir := filepath.Dir(src); isZoomGeneric(base) && isZoomFolder(dir, base) {
		if n := cleanFolderName(filepath.Base(dir)); n != "" {
			stem = n
		}
	}
	return stem + f.ext()
}

// uniqueName returns name, or "stem (2).ext", "stem (3).ext"... if a file of
// that name exists in dir or was already given out in this batch (taken holds
// the lower-cased names). The chosen name is added to taken.
func uniqueName(dir, name string, taken map[string]bool) string {
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	candidate := name
	for n := 2; ; n++ {
		_, err := os.Lstat(filepath.Join(dir, candidate))
		if !taken[strings.ToLower(candidate)] && os.IsNotExist(err) {
			break
		}
		candidate = stem + " (" + strconv.Itoa(n) + ")" + ext
	}
	taken[strings.ToLower(candidate)] = true
	return candidate
}
