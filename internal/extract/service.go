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
	// Settings remembers the chosen format across launches. May be nil
	// (the choice then lasts for the session only).
	Settings *settings.Store
}

// Service is the extraction application service.
type Service struct {
	cfg  Config
	msgs i18n.Catalog

	busy sync.Mutex // held for the whole of a Run

	mu     sync.Mutex // guards cancel and format
	cancel context.CancelFunc
	format Format
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

// Accept resolves raw paths (picker, drop, launch arguments) to items.
// Directories are skipped for now (folder drops arrive with #8); any file is
// accepted, ffmpeg decides later whether it has audio.
func (s *Service) Accept(paths []string) ([]Item, error) {
	return s.accept(paths, s.currentFormat())
}

func (s *Service) accept(paths []string, f Format) ([]Item, error) {
	var items []Item
	for _, p := range paths {
		fi, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if fi.IsDir() {
			continue
		}
		items = append(items, Item{Path: p, SuggestedName: SuggestName(p, f)})
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
	return s.Run(paths, "")
}

// Run extracts the audio of paths into the given format ("m4a" or "mp3"; ""
// means the remembered choice). A single item opens the Save dialog;
// cancelling it returns to idle.
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

	items, err := s.accept(paths, f)
	if err != nil || len(items) == 0 {
		return s.fail(KindFailed, err, "")
	}
	if len(items) > 1 {
		return s.fail(KindBatchUnsupported, errors.New("batches not supported yet"), "")
	}
	return s.runOne(ctx, items[0], f)
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

	s.emit(StateEvent{Phase: PhaseWorking, Index: 1, Total: 1, Percent: -1})

	bin, err := s.cfg.Locate()
	if err != nil {
		return s.fail(KindFailed, err, it.Path)
	}
	var last time.Time
	onProgress := func(percent int) {
		// Throttle to ~10 events/s, but never swallow the final 100.
		if percent < 100 && time.Since(last) < progressInterval {
			return
		}
		last = time.Now()
		s.emit(StateEvent{Phase: PhaseWorking, Index: 1, Total: 1, Percent: percent})
	}
	log, kind, err := extract(ctx, bin, it.Path, dest, f, onProgress)
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
