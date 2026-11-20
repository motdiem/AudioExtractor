package extract

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func isHidden(t *testing.T, path string) bool {
	t.Helper()
	p, _ := windows.UTF16PtrFromString(path)
	attrs, err := windows.GetFileAttributes(p)
	if err != nil {
		t.Fatal(err)
	}
	return attrs&windows.FILE_ATTRIBUTE_HIDDEN != 0
}

func TestTempFileIsHiddenThenShown(t *testing.T) {
	p := filepath.Join(t.TempDir(), ".audioextractor-1.part")
	os.WriteFile(p, nil, 0o644)
	if err := setHidden(p, true); err != nil || !isHidden(t, p) {
		t.Fatalf("hide: err = %v, hidden = %v", err, isHidden(t, p))
	}
	if err := setHidden(p, false); err != nil || isHidden(t, p) {
		t.Fatalf("unhide: err = %v, hidden = %v", err, isHidden(t, p))
	}
}

// The output must never come out hidden: extract clears the attribute before
// the rename (MoveFileEx keeps attributes).
func TestOutputIsNotHidden(t *testing.T) {
	src := makeClip(t, "clip.mp4", "aac")
	dest := filepath.Join(t.TempDir(), "out.m4a")
	svc, _ := newService(t, &fakePort{saveTo: dest})
	if res := svc.Run([]string{src}, "m4a"); res.Outcomes[0].Kind != "" {
		t.Fatalf("outcome = %+v", res.Outcomes[0])
	}
	if isHidden(t, dest) {
		t.Error("the output is hidden")
	}
}
