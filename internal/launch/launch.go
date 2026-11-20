// Package launch gathers the files the OS hands to the app (Dock icon drops,
// "Open with", command-line arguments, a second launch) into batches for the
// extraction service.
//
// It has three small, independent pieces: Filter cleans a command line,
// Coalescer merges paths that arrive close together, and Launcher holds the
// batches back until the window is ready.
package launch

import (
	"os"
	"strings"
	"sync"
	"time"
)

// Filter returns the paths among args (the command line without the program
// name): flags (anything starting with "-") and paths that do not exist are
// dropped. exists defaults to an os.Stat check.
func Filter(args []string, exists func(string) bool) []string {
	if exists == nil {
		exists = func(p string) bool { _, err := os.Stat(p); return err == nil }
	}
	var out []string
	for _, a := range args {
		a = strings.TrimSpace(a)
		if a == "" || strings.HasPrefix(a, "-") || !exists(a) {
			continue
		}
		out = append(out, a)
	}
	return out
}

// Coalescer merges Add calls made within delay of each other into a single
// flush. macOS, for one, delivers one open event per file.
type Coalescer struct {
	delay time.Duration
	flush func([]string)

	mu      sync.Mutex
	pending []string
	timer   *time.Timer
}

// NewCoalescer returns a Coalescer calling flush (on its own goroutine) once
// no Add has happened for delay.
func NewCoalescer(delay time.Duration, flush func([]string)) *Coalescer {
	return &Coalescer{delay: delay, flush: flush}
}

// Add queues paths and restarts the quiet period.
func (c *Coalescer) Add(paths ...string) {
	if len(paths) == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pending = append(c.pending, paths...)
	if c.timer != nil {
		c.timer.Stop()
	}
	c.timer = time.AfterFunc(c.delay, c.fire)
}

func (c *Coalescer) fire() {
	c.mu.Lock()
	batch := c.pending
	c.pending = nil
	c.mu.Unlock()
	if len(batch) > 0 {
		c.flush(batch)
	}
}

// Launcher coalesces incoming paths and hands each batch to deliver, but only
// once Ready has been called: a cold launch can receive its files before the
// frontend is listening. Batches arriving earlier are merged and delivered
// together on Ready.
//
// Ready is exported so Wails exposes it to the frontend, which calls it when
// its event listeners are in place.
type Launcher struct {
	deliver func([]string)
	co      *Coalescer

	mu    sync.Mutex
	ready bool
	held  []string
}

// New returns a Launcher. deliver is called on its own goroutine.
func New(delay time.Duration, deliver func([]string)) *Launcher {
	l := &Launcher{deliver: deliver}
	l.co = NewCoalescer(delay, l.batch)
	return l
}

// Add receives paths from any source.
func (l *Launcher) Add(paths ...string) { l.co.Add(paths...) }

func (l *Launcher) batch(paths []string) {
	l.mu.Lock()
	if !l.ready {
		l.held = append(l.held, paths...)
		l.mu.Unlock()
		return
	}
	l.mu.Unlock()
	l.deliver(paths)
}

// Ready tells the launcher the window can take files; held paths go out now.
func (l *Launcher) Ready() {
	l.mu.Lock()
	l.ready = true
	held := l.held
	l.held = nil
	l.mu.Unlock()
	if len(held) > 0 {
		l.deliver(held)
	}
}
