// Package extract is the extraction application service: the one place that
// owns the workflow "paths in -> Save dialog -> audio file out -> reveal".
// It is bound to the frontend and is the seam the tests drive.
package extract

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/motdiem/AudioExtractor/internal/i18n"
)

// Format is an output format. Only M4A exists so far (MP3 arrives with ticket #6).
type Format string

const FormatM4A Format = "m4a"

func (f Format) ext() string { return "." + string(f) }

// ErrorKind classifies a failed item. Ticket #7 grows this set and maps each
// kind to a catalog message "error.<kind>".
type ErrorKind string

const (
	KindFailed           ErrorKind = "failed"
	KindNoAudio          ErrorKind = "no-audio"
	KindBatchUnsupported ErrorKind = "batch-unsupported"
	KindCancelled        ErrorKind = "cancelled"
)

// ErrorKinds that must have a message in every catalog (cancelled is silent).
var MessageKinds = []ErrorKind{KindFailed, KindNoAudio, KindBatchUnsupported}

// Phase is the stage the window should show.
type Phase string

const (
	PhaseIdle    Phase = "idle"
	PhaseWorking Phase = "working"
	PhaseDone    Phase = "done"
	PhaseError   Phase = "error"
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
}

// Service is the extraction application service.
type Service struct {
	cfg  Config
	msgs i18n.Catalog

	busy   sync.Mutex
	cancel context.CancelFunc
}

// New returns a Service.
func New(cfg Config) *Service {
	return &Service{cfg: cfg, msgs: i18n.Load(cfg.Lang)}
}

// Lang returns the UI language code ("en" or "fr").
func (s *Service) Lang() string { return string(s.cfg.Lang) }

// Messages returns the UI catalog for the current language.
func (s *Service) Messages() map[string]string { return s.msgs }

// Accept resolves raw paths (picker, drop, launch arguments) to items.
// Directories are skipped for now (folder drops arrive with #8); any file is
// accepted, ffmpeg decides later whether it has audio.
func (s *Service) Accept(paths []string) ([]Item, error) {
	var items []Item
	for _, p := range paths {
		fi, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if fi.IsDir() {
			continue
		}
		items = append(items, Item{Path: p, SuggestedName: SuggestName(p, FormatM4A)})
	}
	return items, nil
}

// SuggestName is the default output name for src: its own name with the new
// extension. (Zoom meeting-folder naming comes with #8.)
func SuggestName(src string, f Format) string {
	base := filepath.Base(src)
	return strings.TrimSuffix(base, filepath.Ext(base)) + f.ext()
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
	return s.Run(paths, string(FormatM4A))
}

// Run extracts the audio of paths into the given format (only "m4a" for now).
// A single item opens the Save dialog; cancelling it returns to idle.
func (s *Service) Run(paths []string, format string) Result {
	if !s.busy.TryLock() {
		return Result{Cancelled: true}
	}
	defer s.busy.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	defer cancel()

	items, err := s.Accept(paths)
	if err != nil || len(items) == 0 {
		return s.fail(KindFailed, err, "")
	}
	if len(items) > 1 {
		return s.fail(KindBatchUnsupported, errors.New("batches not supported yet"), "")
	}
	return s.runOne(ctx, items[0], Format(format))
}

// Cancel stops the running extraction, if any.
func (s *Service) Cancel() {
	if s.cancel != nil {
		s.cancel()
	}
}

func (s *Service) runOne(ctx context.Context, it Item, f Format) Result {
	if f != FormatM4A {
		return s.fail(KindFailed, fmt.Errorf("unsupported format %q", f), it.Path)
	}
	dest, err := s.cfg.Port.ChooseSaveFile(SaveRequest{
		Title:      s.msgs.T("save.title"),
		Dir:        filepath.Dir(it.Path),
		Name:       it.SuggestedName,
		FilterName: s.msgs.T("save.filter"),
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

	s.emit(StateEvent{Phase: PhaseWorking, Index: 1, Total: 1, Percent: -1})

	bin, err := s.cfg.Locate()
	if err != nil {
		return s.fail(KindFailed, err, it.Path)
	}
	log, kind, err := extractM4A(ctx, bin, it.Path, dest)
	switch {
	case kind == KindCancelled:
		s.emit(StateEvent{Phase: PhaseIdle})
		return Result{Cancelled: true, Outcomes: []Outcome{{Source: it.Path, Kind: KindCancelled, Log: log}}}
	case err != nil:
		out := Outcome{Source: it.Path, Kind: kind, Message: s.msgs.T("error." + string(kind)), Log: log}
		s.emit(StateEvent{Phase: PhaseError, Kind: kind, Message: out.Message})
		return Result{Outcomes: []Outcome{out}}
	}
	s.emit(StateEvent{Phase: PhaseDone, Index: 1, Total: 1, Percent: 100, Name: filepath.Base(dest), Output: dest})
	return Result{Outcomes: []Outcome{{Source: it.Path, Output: dest, Log: log}}}
}

// Reveal shows path in the file manager.
func (s *Service) Reveal(path string) error { return s.cfg.Port.Reveal(path) }

func (s *Service) emit(e StateEvent) {
	if s.cfg.Emit != nil {
		s.cfg.Emit(EventState, e)
	}
}

func (s *Service) fail(kind ErrorKind, err error, src string) Result {
	msg := s.msgs.T("error." + string(kind))
	log := ""
	if err != nil {
		log = err.Error()
	}
	s.emit(StateEvent{Phase: PhaseError, Kind: kind, Message: msg})
	return Result{Outcomes: []Outcome{{Source: src, Kind: kind, Message: msg, Log: log}}}
}
