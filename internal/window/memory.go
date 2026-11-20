package window

import (
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/motdiem/AudioExtractor/internal/settings"
)

// settingsKey is the window's entry in the per-user settings file.
const settingsKey = "window"

// saveDelay groups the stream of move events of one drag into a single write.
const saveDelay = 500 * time.Millisecond

// Restore sets the window's initial position to the saved one, if any. Call
// it on the options before creating the window. store may be nil.
//
// The saved position is applied as is: the screens are only known once the
// app runs, so Track checks it against them then and moves the window if it
// would open off-screen.
func Restore(opts *application.WebviewWindowOptions, store *settings.Store) {
	var p Point
	if store == nil || !store.Get(settingsKey, &p) {
		return // first launch: let Wails centre it
	}
	opts.InitialPosition = application.WindowXY
	opts.X, opts.Y = p.X, p.Y
}

// Track keeps the saved position up to date (after a move, and when the
// window closes) and, once the window is up, pulls it back onto a screen if
// the saved position is no longer visible. store may be nil.
func Track(app *application.App, win *application.WebviewWindow, store *settings.Store) {
	if store == nil {
		return
	}
	var (
		mu    sync.Mutex
		last  *Point
		timer *time.Timer
	)
	save := func() {
		mu.Lock()
		p := last
		mu.Unlock()
		if p != nil {
			_ = store.Set(settingsKey, *p) // a convenience: never worth an error dialog
		}
	}

	// The first runtime-ready event comes after the native window exists and
	// the screens are known, which is the earliest the clamp can run.
	var once sync.Once
	win.OnWindowEvent(events.Common.WindowRuntimeReady, func(*application.WindowEvent) {
		once.Do(func() {
			x, y := win.Position()
			w, h := win.Size()
			var areas []Rect
			for _, s := range app.Screen.GetAll() {
				a := s.WorkArea
				areas = append(areas, Rect{a.X, a.Y, a.Width, a.Height})
			}
			if p := Place(Point{x, y}, w, h, areas); p != (Point{x, y}) {
				win.SetPosition(p.X, p.Y)
			}
		})
	})

	win.OnWindowEvent(events.Common.WindowDidMove, func(*application.WindowEvent) {
		x, y := win.Position()
		mu.Lock()
		last = &Point{x, y}
		if timer != nil {
			timer.Stop()
		}
		timer = time.AfterFunc(saveDelay, save)
		mu.Unlock()
	})

	// Closing the window or quitting (Cmd+Q) ends the app, so flush a move
	// still waiting for its timer. A hook (unlike a listener) runs before the
	// window goes away. The position comes from the last move: the window may
	// already be on its way out and unable to answer.
	flush := func() {
		mu.Lock()
		if timer != nil {
			timer.Stop()
		}
		mu.Unlock()
		save()
	}
	win.RegisterHook(events.Common.WindowClosing, func(*application.WindowEvent) { flush() })
	app.OnShutdown(flush)
}
