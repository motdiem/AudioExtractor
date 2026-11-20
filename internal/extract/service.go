// Package extract is the extraction application service: the one place that
// owns the workflow "paths in -> Save dialog -> audio file out -> reveal".
// It is bound to the frontend and is the seam the tests drive.
package extract

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/motdiem/AudioExtractor/internal/i18n"
	"github.com/motdiem/AudioExtractor/internal/settings"
)

// Format is an output format.
type Format string

const (
	FormatM4A Format = "m4a" // the default: near-instant, lossless
	FormatMP3 Format = "mp3" // 128 kbps CBR re-encode
)

// ParseFormat returns the Format named s, or false if there is none.
func ParseFormat(s string) (Format, bool) {
	switch f := Format(s); f {
	case FormatM4A, FormatMP3:
		return f, true
	}
	return "", false
}

// settingsKeyFormat is where the last chosen format is remembered.
const settingsKeyFormat = "format"

// progressInterval throttles percent events to about 10 per second.
const progressInterval = 100 * time.Millisecond

func (f Format) ext() string { return "." + string(f) }

// ErrorKind classifies a failed item. Ticket #7 grows this set and maps each
// kind to a catalog message "error.<kind>".
type ErrorKind string

const (
	KindFailed                ErrorKind = "failed" // fallback when nothing more specific is known
	KindNoAudio               ErrorKind = "no-audio"
	KindUnreadable            ErrorKind = "unreadable" // damaged, truncated or still being written
	KindNotMedia              ErrorKind = "not-media"
	KindDestinationUnwritable ErrorKind = "destination-unwritable" // permission, disk full, read-only or offline folder
	KindOutputLocked          ErrorKind = "output-locked"          // the file to replace is open elsewhere
	KindEmptyFolder           ErrorKind = "empty-folder"           // a dropped folder holds no media files
	KindCancelled             ErrorKind = "cancelled"              // silent: no message
)

// ErrorKinds that must have a message in every catalog (cancelled is silent).
var MessageKinds = []ErrorKind{
	KindFailed, KindNoAudio, KindUnreadable, KindNotMedia,
	KindDestinationUnwritable, KindOutputLocked, KindEmptyFolder,
}

// Phase is the stage the window should show.
type Phase string

const (
	PhaseIdle    Phase = "idle"
	PhaseWorking Phase = "working"
	PhaseDone    Phase = "done"
	PhaseError   Phase = "error"
	// PhaseSummary ends a batch: counts and the list of failed files.
	PhaseSummary Phase = "summary"
)

// StateEvent is the payload of the "extract:state" event. Index/Total are the
// 1-based item position (batches, #8); Percent is 0-100 or -1 when unknown (#6).
type StateEvent struct {
	Phase   Phase     `json:"phase"`
	Index   int       `json:"index"`
	Total   int       `json:"total"`
	Percent int       `json:"percent"`
	Name    string    `json:"name"`   // output file name (done)
	Output  string    `json:"output"` // output path (done)
	Kind    ErrorKind `json:"kind"`   // error
	Message string    `json:"message"`
	// Summary only (batches): how many items succeeded and failed, whether the
	// user stopped the batch early, and the failed files. Output is then the
	// first output file, which "Show in Finder" selects.
	Done     int       `json:"done"`
	Failed   int       `json:"failed"`
	Stopped  bool      `json:"stopped"`
	Failures []Failure `json:"failures"`
}

// Failure is one failed file in a batch summary.
type Failure struct {
	Name    string `json:"name"`
	Message string `json:"message"`
}

// EventState is the name of the event carrying StateEvent.
const EventState = "extract:state"

// Item is one accepted input and its suggested output.
type Item struct {
	Path          string `json:"path"`
	SuggestedName string `json:"suggestedName"`
}

// Outcome is the result for one item.
type Outcome struct {
	Source  string    `json:"source"`
	Output  string    `json:"output"` // empty unless it succeeded
	Kind    ErrorKind `json:"kind"`   // empty on success
	Message string    `json:"message"`
	Log     string    `json:"log"` // raw ffmpeg log, for "Copy details" (#7)

	cause error // the Go error behind a failure, for "Copy details"
}

// Result is what a run returns.
type Result struct {
	Cancelled bool      `json:"cancelled"` // the user dismissed a dialog
	Outcomes  []Outcome `json:"outcomes"`
}

// Config wires a Service to its collaborators.
type Config struct {
	// Locate returns the ffmpeg executable (injected: bundled build in
	// production, PATH in tests).
	Locate func() (string, error)
	Port   Port
	// Emit sends an event to the window. May be nil.
	Emit func(name string, data any)
	Lang i18n.Lang
	// Settings remembers the chosen format across launches. May be nil
	// (the choice then lasts for the session only).
	Settings *settings.Store
	// Version is the app version shown in "Copy details" (default "dev").
	Version string
}

// Service is the extraction application service.
type Service struct {
	cfg  Config
	msgs i18n.Catalog

	busy sync.Mutex // held for the whole of a Run

	mu     sync.Mutex // guards cancel and format
	cancel context.CancelFunc
	format Format
	// details is the "Copy details" text for the error on screen.
	details string
}

// New returns a Service.
func New(cfg Config) *Service {
	s := &Service{cfg: cfg, msgs: i18n.Load(cfg.Lang), format: FormatM4A}
	if cfg.Settings != nil {
		var saved string
		if cfg.Settings.Get(settingsKeyFormat, &saved) {
			if f, ok := ParseFormat(saved); ok {
				s.format = f
			}
		}
	}
	return s
}

// Format returns the chosen output format ("m4a" or "mp3").
func (s *Service) Format() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return string(s.format)
}

// SetFormat changes the output format and remembers it for the next launch.
func (s *Service) SetFormat(format string) error {
	f, ok := ParseFormat(format)
	if !ok {
		return fmt.Errorf("unknown format %q", format)
	}
	s.mu.Lock()
	s.format = f
	s.mu.Unlock()
	if s.cfg.Settings != nil {
		return s.cfg.Settings.Set(settingsKeyFormat, string(f))
	}
	return nil
}

func (s *Service) currentFormat() Format {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.format
}

// Lang returns the UI language code ("en" or "fr").
func (s *Service) Lang() string { return string(s.cfg.Lang) }

// Messages returns the UI catalog for the current language.
func (s *Service) Messages() map[string]string { return s.msgs }

// Accept resolves raw paths (picker, drop, launch arguments) to items:
// folders expand to their top-level media files, explicit files are always
// kept, duplicates are dropped (see expandInputs).
func (s *Service) Accept(paths []string) ([]Item, error) {
	items, _, err := s.accept(paths, s.currentFormat())
	return items, err
}

func (s *Service) accept(paths []string, f Format) (items []Item, emptyFolders int, err error) {
	files, emptyFolders, err := expandInputs(paths)
	if err != nil {
		return nil, 0, err
	}
	for _, p := range files {
		items = append(items, Item{Path: p, SuggestedName: SuggestName(p, f)})
	}
	return items, emptyFolders, nil
}

// Pick opens the file picker, then runs the extraction on the choice.
func (s *Service) Pick() Result {
	paths, err := s.cfg.Port.ChooseInputs(PickRequest{Title: s.msgs.T("pick.title")})
	if err != nil {
		return s.fail(KindFailed, err, "")
	}
	if len(paths) == 0 {
		return Result{Cancelled: true}
	}
	return s.Run(paths, "")
}

// Run extracts the audio of paths into the given format ("m4a" or "mp3"; ""
// means the remembered choice). Folders expand to their media files. A single
// item opens the Save dialog; several open a folder dialog and run one after
// another. Cancelling a dialog returns to idle.
func (s *Service) Run(paths []string, format string) Result {
	if !s.busy.TryLock() {
		return Result{Cancelled: true}
	}
	defer s.busy.Unlock()

	f, ok := ParseFormat(format)
	if !ok {
		f = s.currentFormat()
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.cancel = cancel
	s.mu.Unlock()
	defer cancel()

	items, empty, err := s.accept(paths, f)
	switch {
	case err != nil:
		return s.fail(KindFailed, err, "")
	case len(items) == 0 && empty > 0:
		return s.fail(KindEmptyFolder, errors.New("no media files in the folder"), "")
	case len(items) == 0:
		return s.fail(KindFailed, errors.New("nothing to extract"), "")
	case len(items) == 1:
		return s.runOne(ctx, items[0], f)
	}
	return s.runBatch(ctx, items, f)
}

// Busy reports whether a Run is in progress (a Run started now would be
// refused).
func (s *Service) Busy() bool {
	if s.busy.TryLock() {
		s.busy.Unlock()
		return false
	}
	return true
}

// Cancel stops the running extraction, if any: ffmpeg is killed, the partial
// output is removed and the window returns to idle without an error.
func (s *Service) Cancel() {
	s.mu.Lock()
	cancel := s.cancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (s *Service) runOne(ctx context.Context, it Item, f Format) Result {
	dest, err := s.cfg.Port.ChooseSaveFile(SaveRequest{
		Title:      s.msgs.T("save.title"),
		Dir:        filepath.Dir(it.Path),
		Name:       it.SuggestedName,
		FilterName: s.msgs.T("save.filter." + string(f)),
		Extension:  string(f),
	})
	if err != nil {
		return s.fail(KindFailed, err, it.Path)
	}
	if dest == "" {
		s.emit(StateEvent{Phase: PhaseIdle})
		return Result{Cancelled: true}
	}
	if !strings.EqualFold(filepath.Ext(dest), f.ext()) {
		dest += f.ext()
	}

	out := s.extractItem(ctx, it, dest, f, 1, 1)
	switch {
	case out.Kind == KindCancelled:
		s.emit(StateEvent{Phase: PhaseIdle})
		return Result{Cancelled: true, Outcomes: []Outcome{out}}
	case out.Kind != "":
		s.setDetails(out)
		s.emit(StateEvent{Phase: PhaseError, Kind: out.Kind, Message: out.Message})
		return Result{Outcomes: []Outcome{out}}
	}
	s.emit(StateEvent{Phase: PhaseDone, Index: 1, Total: 1, Percent: 100, Name: filepath.Base(dest), Output: dest})
	return Result{Outcomes: []Outcome{out}}
}

// extractItem runs ffmpeg for one item, emitting working events for the
// "file index of total" position, and returns its outcome. No terminal event
// is emitted: the caller decides what the window shows next.
func (s *Service) extractItem(ctx context.Context, it Item, dest string, f Format, index, total int) Outcome {
	s.emit(StateEvent{Phase: PhaseWorking, Index: index, Total: total, Percent: -1})

	bin, err := s.cfg.Locate()
	if err != nil {
		return s.outcomeFor(it.Path, KindFailed, err, "")
	}
	var last time.Time
	onProgress := func(percent int) {
		// Throttle to ~10 events/s, but never swallow the final 100.
		if percent < 100 && time.Since(last) < progressInterval {
			return
		}
		last = time.Now()
		s.emit(StateEvent{Phase: PhaseWorking, Index: index, Total: total, Percent: percent})
	}
	log, kind, err := extract(ctx, bin, it.Path, dest, f, onProgress)
	if kind != "" {
		return s.outcomeFor(it.Path, kind, err, log)
	}
	return Outcome{Source: it.Path, Output: dest, Log: log}
}

// outcomeFor builds the failed Outcome for kind. The cause is kept (not
// serialised) for "Copy details".
func (s *Service) outcomeFor(src string, kind ErrorKind, cause error, log string) Outcome {
	o := Outcome{Source: src, Kind: kind, Log: log, cause: cause}
	if kind != KindCancelled {
		o.Message = s.msgs.T("error." + string(kind))
	}
	return o
}

// runBatch asks for a destination folder, then extracts the items one after
// another. A failure never stops the batch. Cancel stops it: the current item
// is cleaned up by extract, finished items stay in place, and the window shows
// the summary with a "Stopped after N of M" title (or goes back to idle if
// nothing had finished yet).
func (s *Service) runBatch(ctx context.Context, items []Item, f Format) Result {
	dir, err := s.cfg.Port.ChooseFolder(FolderRequest{
		Title: s.msgs.T("folder.title"),
		Dir:   filepath.Dir(items[0].Path),
	})
	if err != nil {
		return s.fail(KindFailed, err, "")
	}
	if dir == "" {
		s.emit(StateEvent{Phase: PhaseIdle})
		return Result{Cancelled: true}
	}

	taken := map[string]bool{}
	var res Result
	var failures []Failure
	var failedOutcomes []Outcome
	done, firstOut, stopped := 0, "", false
	for i, it := range items {
		if ctx.Err() != nil {
			stopped = true
			break
		}
		dest := filepath.Join(dir, uniqueName(dir, it.SuggestedName, taken))
		out := s.extractItem(ctx, it, dest, f, i+1, len(items))
		res.Outcomes = append(res.Outcomes, out)
		switch {
		case out.Kind == KindCancelled:
			stopped = true
		case out.Kind != "":
			failedOutcomes = append(failedOutcomes, out)
			failures = append(failures, Failure{Name: filepath.Base(it.Path), Message: out.Message})
		default:
			done++
			if firstOut == "" {
				firstOut = dest
			}
		}
		if stopped {
			break
		}
	}
	s.setDetails(failedOutcomes...)
	res.Cancelled = stopped
	if stopped && done+len(failures) == 0 {
		s.emit(StateEvent{Phase: PhaseIdle})
		return res
	}
	s.emit(StateEvent{
		Phase: PhaseSummary, Index: done + len(failures), Total: len(items),
		Percent: 100, Output: firstOut, Done: done, Failed: len(failures),
		Stopped: stopped, Failures: failures,
	})
	return res
}

// Reveal shows path in the file manager.
func (s *Service) Reveal(path string) error { return s.cfg.Port.Reveal(path) }

func (s *Service) emit(e StateEvent) {
	if s.cfg.Emit != nil {
		s.cfg.Emit(EventState, e)
	}
}

func (s *Service) fail(kind ErrorKind, err error, src string) Result {
	out := s.outcomeFor(src, kind, nil, "")
	if err != nil {
		out.Log = err.Error()
	}
	s.setDetails(out)
	s.emit(StateEvent{Phase: PhaseError, Kind: kind, Message: out.Message})
	return Result{Outcomes: []Outcome{out}}
}

// setDetails remembers the text "Copy details" will put on the clipboard: one
// block per failed outcome, concatenated.
func (s *Service) setDetails(outs ...Outcome) {
	blocks := make([]string, len(outs))
	for i, o := range outs {
		blocks[i] = s.detailsText(o)
	}
	d := strings.Join(blocks, "\n==========\n\n")
	s.mu.Lock()
	s.details = d
	s.mu.Unlock()
}

// detailsText is what a person pastes into an email to us: a short header and
// the raw ffmpeg log. The header carries only the file name; the log is ffmpeg's own and may include the path.
func (s *Service) detailsText(o Outcome) string {
	version := s.cfg.Version
	if version == "" {
		version = "dev"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "AudioExtractor %s\nOS: %s/%s\n", version, runtime.GOOS, runtime.GOARCH)
	if o.Source != "" {
		fmt.Fprintf(&b, "File: %s\n", filepath.Base(o.Source))
	}
	fmt.Fprintf(&b, "Error: %s\n", o.Kind)
	if o.cause != nil {
		fmt.Fprintf(&b, "Cause: %v\n", o.cause)
	}
	b.WriteString("\n--- ffmpeg log ---\n")
	b.WriteString(strings.TrimSpace(o.Log))
	b.WriteString("\n")
	return b.String()
}

// CopyDetails puts the details of the error on screen on the clipboard.
func (s *Service) CopyDetails() error {
	s.mu.Lock()
	d := s.details
	s.mu.Unlock()
	if d == "" {
		return errors.New("no error to copy")
	}
	return s.cfg.Port.CopyText(d)
}
