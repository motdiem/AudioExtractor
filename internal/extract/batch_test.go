package extract

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/motdiem/AudioExtractor/internal/i18n"
)

// touch creates an empty file (and its folder) and returns its path.
func touch(t *testing.T, elems ...string) string {
	t.Helper()
	p := filepath.Join(elems...)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// copyClip makes a real clip named name inside dir.
func copyClip(t *testing.T, dir, name, audio string) string {
	t.Helper()
	src := makeClip(t, name, audio)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, name)
	if err := os.Rename(src, dst); err != nil {
		t.Fatal(err)
	}
	return dst
}

func names(items []Item) []string {
	var out []string
	for _, it := range items {
		out = append(out, filepath.Base(it.Path))
	}
	return out
}

func TestFolderExpandsToTopLevelMediaOnly(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"b.mp4", "A.MOV", "c.m4a", "notes.txt", ".hidden.mp4", "._junk.mp4", "d.pdf"} {
		touch(t, dir, n)
	}
	touch(t, dir, "sub", "deep.mp4") // not top level
	svc := New(Config{Port: &fakePort{}, Lang: i18n.EN})

	items, err := svc.Accept([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"A.MOV", "b.mp4", "c.m4a"}
	if got := names(items); !reflect.DeepEqual(got, want) {
		t.Errorf("items = %v, want %v", got, want)
	}
}

func TestExplicitFilesAreKeptWhateverTheirExtensionAndDeduped(t *testing.T) {
	dir := t.TempDir()
	txt := touch(t, dir, "notes.txt")
	mp4 := touch(t, dir, "a.mp4")
	svc := New(Config{Port: &fakePort{}, Lang: i18n.EN})

	// The folder also contains a.mp4, which is dropped explicitly first.
	items, err := svc.Accept([]string{txt, mp4, dir, mp4})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := names(items), []string{"notes.txt", "a.mp4"}; !reflect.DeepEqual(got, want) {
		t.Errorf("items = %v, want %v", got, want)
	}
}

func TestEmptyFolderGivesAClearMessage(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "readme.txt")
	port := &fakePort{}
	svc, rec := newService(t, port)

	res := svc.Run([]string{dir}, "m4a")

	if len(res.Outcomes) != 1 || res.Outcomes[0].Kind != KindEmptyFolder {
		t.Fatalf("result = %+v", res)
	}
	if res.Outcomes[0].Message != i18n.Load(i18n.EN).T("error.empty-folder") || res.Outcomes[0].Message == "" {
		t.Errorf("message = %q", res.Outcomes[0].Message)
	}
	if last := rec.events[len(rec.events)-1]; last.Phase != PhaseError || last.Kind != KindEmptyFolder {
		t.Errorf("last event = %+v", last)
	}
	if len(port.saveReqs)+len(port.folderReqs) != 0 {
		t.Error("no dialog expected")
	}
}

func TestFolderWithOneMP4BehavesLikeASingleFile(t *testing.T) {
	dir := t.TempDir()
	copyClip(t, dir, "talk.mp4", "aac")
	touch(t, dir, "notes.txt")
	out := filepath.Join(t.TempDir(), "talk.m4a")
	port := &fakePort{saveTo: out}
	svc, _ := newService(t, port)

	res := svc.Run([]string{dir}, "m4a")

	if len(port.saveReqs) != 1 || len(port.folderReqs) != 0 {
		t.Fatalf("expected the Save dialog only: save=%d folder=%d", len(port.saveReqs), len(port.folderReqs))
	}
	if res.Outcomes[0].Output != out {
		t.Errorf("result = %+v", res)
	}
}

func TestZoomNaming(t *testing.T) {
	root := t.TempDir()
	zoom := filepath.Join(root, "2026-11-20 10.00.00 Weekly sync")
	plain := filepath.Join(root, "Downloads")
	cases := []struct {
		path, want string
	}{
		{filepath.Join(zoom, "video1234567890.mp4"), "2026-11-20 10.00.00 Weekly sync.m4a"},
		{filepath.Join(zoom, "VIDEO12.MP4"), "2026-11-20 10.00.00 Weekly sync.m4a"},
		{filepath.Join(zoom, "audio1234567890.m4a"), "2026-11-20 10.00.00 Weekly sync.m4a"},
		{filepath.Join(zoom, "zoom_0.mp4"), "2026-11-20 10.00.00 Weekly sync.m4a"},
		{filepath.Join(zoom, "GMT20261120-100000_Recording_1920x1080.mp4"), "2026-11-20 10.00.00 Weekly sync.m4a"},
		{filepath.Join(zoom, "gmt20261120-100000_recording.mp4"), "2026-11-20 10.00.00 Weekly sync.m4a"},
		// Not generic names: keep their own.
		{filepath.Join(zoom, "Team offsite.mp4"), "Team offsite.m4a"},
		{filepath.Join(zoom, "video.mp4"), "video.m4a"},
		{filepath.Join(zoom, "video12.mov"), "video12.m4a"},
		{filepath.Join(zoom, "audio_only.m4a"), "audio_only.m4a"},
		// Generic name, but not in a Zoom folder.
		{filepath.Join(plain, "video1234567890.mp4"), "video1234567890.m4a"},
	}
	for _, c := range cases {
		touch(t, c.path)
		if got := SuggestName(c.path, FormatM4A); got != c.want {
			t.Errorf("SuggestName(%s) = %q, want %q", c.path, got, c.want)
		}
	}

	// A folder without a timestamp still counts when it holds another Zoom file.
	other := filepath.Join(root, "Sync notes")
	a := touch(t, other, "video111.mp4")
	touch(t, other, "audio_only.m4a")
	if got := SuggestName(a, FormatMP3); got != "Sync notes.mp3" {
		t.Errorf("sibling rule: %q", got)
	}

	// Characters invalid on Windows are stripped from folder-derived names.
	bad := filepath.Join(root, `2026-11-20 10.00.00 Q3: plan?*`)
	v := touch(t, bad, "video1.mp4")
	if got := SuggestName(v, FormatM4A); got != "2026-11-20 10.00.00 Q3 plan.m4a" {
		t.Errorf("sanitised: %q", got)
	}
}

func TestZoomNamingInTheSingleFileSaveDialog(t *testing.T) {
	zoom := filepath.Join(t.TempDir(), "2026-11-20 10.00.00 Weekly sync")
	src := copyClip(t, zoom, "video1234567890.mp4", "aac")
	port := &fakePort{saveTo: filepath.Join(t.TempDir(), "x.m4a")}
	svc, _ := newService(t, port)

	svc.Run([]string{src}, "m4a")

	if got := port.saveReqs[0].Name; got != "2026-11-20 10.00.00 Weekly sync.m4a" {
		t.Errorf("Save dialog name = %q", got)
	}
}

func TestBatchUsesFolderDialogAndZoomNamesAndSuffixesCollisions(t *testing.T) {
	root := t.TempDir()
	zoomA := filepath.Join(root, "2026-11-20 10.00.00 Weekly sync")
	zoomB := filepath.Join(root, "2026-11-21 09.00.00 Weekly sync") // different folder, same name once suffix is stripped? no: distinct names
	v := copyClip(t, zoomA, "video1.mp4", "aac")
	a := copyClip(t, zoomA, "audio1.m4a", "aac") // same Zoom folder: same suggested name as v
	b := copyClip(t, zoomB, "video2.mp4", "aac")
	own := copyClip(t, root, "Own name.mp4", "aac")
	outDir := t.TempDir()
	touch(t, outDir, "Own name.m4a") // existing file: must not be overwritten
	port := &fakePort{folderTo: outDir}
	svc, rec := newService(t, port)

	res := svc.Run([]string{v, a, b, own}, "m4a")

	if len(port.saveReqs) != 0 || len(port.folderReqs) != 1 {
		t.Fatalf("dialogs: save=%d folder=%d", len(port.saveReqs), len(port.folderReqs))
	}
	if got := port.folderReqs[0].Dir; got != zoomA {
		t.Errorf("folder dialog opened in %q, want the first source's folder %q", got, zoomA)
	}
	var got []string
	for _, o := range res.Outcomes {
		if o.Kind != "" {
			t.Fatalf("unexpected failure: %+v", o)
		}
		got = append(got, filepath.Base(o.Output))
	}
	want := []string{
		"2026-11-20 10.00.00 Weekly sync.m4a",
		"2026-11-20 10.00.00 Weekly sync (2).m4a",
		"2026-11-21 09.00.00 Weekly sync.m4a",
		"Own name (2).m4a",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("outputs = %v, want %v", got, want)
	}
	if fi, _ := os.Stat(filepath.Join(outDir, "Own name.m4a")); fi == nil || fi.Size() != 0 {
		t.Error("the pre-existing file was overwritten")
	}
	for _, o := range res.Outcomes {
		assertM4A(t, o.Output, "aac")
	}
	last := rec.events[len(rec.events)-1]
	if last.Phase != PhaseSummary || last.Done != 4 || last.Failed != 0 || last.Total != 4 {
		t.Errorf("last event = %+v", last)
	}
}

func TestUniqueNameCountsUpAndIgnoresCase(t *testing.T) {
	dir := t.TempDir()
	taken := map[string]bool{}
	var got []string
	for i := 0; i < 3; i++ {
		got = append(got, uniqueName(dir, "x.m4a", taken))
	}
	got = append(got, uniqueName(dir, "X.M4A", taken))
	if want := []string{"x.m4a", "x (2).m4a", "x (3).m4a", "X (4).M4A"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestMixedBatchContinuesPastAFailureAndSummarises(t *testing.T) {
	dir := t.TempDir()
	good1 := copyClip(t, dir, "a.mp4", "aac")
	silent := copyClip(t, dir, "b silent.mp4", "")
	notes := touch(t, dir, "c.txt")
	if err := os.WriteFile(notes, []byte("just some text\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	good2 := copyClip(t, dir, "d.mp4", "aac")
	outDir := t.TempDir()
	port := &fakePort{folderTo: outDir}
	svc, rec := newService(t, port)

	res := svc.Run([]string{good1, silent, notes, good2}, "m4a")

	kinds := []ErrorKind{}
	for _, o := range res.Outcomes {
		kinds = append(kinds, o.Kind)
	}
	if want := []ErrorKind{"", KindNoAudio, KindNotMedia, ""}; !reflect.DeepEqual(kinds, want) {
		t.Fatalf("kinds = %v, want %v", kinds, want)
	}
	noStrays(t, outDir, "a.m4a", "d.m4a")

	last := rec.events[len(rec.events)-1]
	if last.Phase != PhaseSummary || last.Done != 2 || last.Failed != 2 || last.Total != 4 || last.Stopped {
		t.Fatalf("summary = %+v", last)
	}
	if last.Output != filepath.Join(outDir, "a.m4a") {
		t.Errorf("summary output = %q", last.Output)
	}
	cat := i18n.Load(i18n.EN)
	wantF := []Failure{{"b silent.mp4", cat.T("error.no-audio")}, {"c.txt", cat.T("error.not-media")}}
	if !reflect.DeepEqual(last.Failures, wantF) {
		t.Errorf("failures = %+v, want %+v", last.Failures, wantF)
	}

	// "file N of M" progress.
	var idx []int
	for _, e := range rec.events {
		if e.Phase == PhaseWorking && e.Percent == -1 {
			idx = append(idx, e.Index)
			if e.Total != 4 {
				t.Errorf("total = %d", e.Total)
			}
		}
	}
	if !reflect.DeepEqual(idx, []int{1, 2, 3, 4}) {
		t.Errorf("working indexes = %v", idx)
	}

	// Copy details covers every failed item, one block each.
	if err := svc.CopyDetails(); err != nil || len(port.copied) != 1 {
		t.Fatalf("copy: %v %v", err, port.copied)
	}
	d := port.copied[0]
	if strings.Count(d, "AudioExtractor ") != 2 || !strings.Contains(d, "File: b silent.mp4") ||
		!strings.Contains(d, "File: c.txt") || strings.Contains(d, "File: a.mp4") {
		t.Errorf("details:\n%s", d)
	}
}

func TestCancelMidBatchKeepsFinishedOutputsAndLeavesNoPartialFile(t *testing.T) {
	dir := t.TempDir()
	var srcs []string
	for _, n := range []string{"a.mp4", "b.mp4", "c.mp4"} {
		srcs = append(srcs, copyClip(t, dir, n, "aac"))
	}
	outDir := t.TempDir()
	rec := &recorder{}
	var svc *Service
	var once sync.Once
	bin := ffmpegPath(t)
	svc = New(Config{
		Locate: func() (string, error) { return bin, nil },
		Port:   &fakePort{folderTo: outDir},
		Lang:   i18n.EN,
		Emit: func(name string, data any) {
			rec.emit(name, data)
			// The moment item 2 starts, press Cancel.
			if e := data.(StateEvent); e.Phase == PhaseWorking && e.Index == 2 {
				once.Do(func() { svc.Cancel() })
			}
		},
	})

	res := svc.Run(srcs, "m4a")

	if !res.Cancelled {
		t.Fatalf("result = %+v, want cancelled", res)
	}
	if len(res.Outcomes) < 1 || res.Outcomes[0].Kind != "" {
		t.Fatalf("outcomes = %+v", res.Outcomes)
	}
	entries, _ := os.ReadDir(outDir)
	var files []string
	for _, e := range entries {
		files = append(files, e.Name())
	}
	if files[0] != "a.m4a" || strings.HasPrefix(strings.Join(files, "|"), ".") || strings.Contains(strings.Join(files, "|"), "c.m4a") {
		t.Errorf("output folder = %v: want a.m4a kept, no partial file, c never started", files)
	}
	for _, f := range files {
		if strings.HasSuffix(f, ".part") {
			t.Errorf("partial file left: %s", f)
		}
	}
	last := rec.events[len(rec.events)-1]
	if last.Phase != PhaseSummary || !last.Stopped || last.Done < 1 || last.Total != 3 {
		t.Errorf("last event = %+v", last)
	}
}

func TestCancellingTheFolderDialogReturnsToIdle(t *testing.T) {
	dir := t.TempDir()
	a, b := copyClip(t, dir, "a.mp4", "aac"), copyClip(t, dir, "b.mp4", "aac")
	svc, rec := newService(t, &fakePort{})

	res := svc.Run([]string{a, b}, "m4a")

	if !res.Cancelled || len(res.Outcomes) != 0 {
		t.Errorf("result = %+v", res)
	}
	if p := rec.phases(); len(p) != 1 || p[0] != PhaseIdle {
		t.Errorf("phases = %v", p)
	}
}
