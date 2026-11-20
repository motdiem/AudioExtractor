package main

import (
	"embed"
	"errors"
	"log"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/motdiem/AudioExtractor/internal/desktop"
	"github.com/motdiem/AudioExtractor/internal/extract"
	"github.com/motdiem/AudioExtractor/internal/ffmpeg"
	"github.com/motdiem/AudioExtractor/internal/i18n"
	"github.com/motdiem/AudioExtractor/internal/launch"
	"github.com/motdiem/AudioExtractor/internal/settings"
	"github.com/motdiem/AudioExtractor/internal/webview2"
)

// The built frontend (frontend/dist) is embedded into the binary.
//
//go:embed all:frontend/dist
var assets embed.FS

// version is set at release time with -ldflags "-X main.version=...".
var version = "dev"

// launchDebounce merges files that arrive in quick succession into one batch.
const launchDebounce = 300 * time.Millisecond

func main() {
	// Windows without the WebView2 Runtime: explain and exit instead of
	// failing to open a window. (Always true on macOS.)
	if !webview2.Ensure(i18n.Detect()) {
		return
	}
	port := &desktop.Port{}
	// A settings problem must never stop the app: without a store the format
	// choice simply lasts for the session.
	var store *settings.Store
	if path, err := settings.DefaultPath(); err == nil {
		store = settings.New(path)
	}
	svc := extract.New(extract.Config{
		Locate: locateFFmpeg,
		Port:   port,
		Emit: func(name string, data any) {
			application.Get().Event.Emit(name, data)
		},
		Lang:     i18n.Detect(),
		Settings: store,
		Version:  version,
	})

	// Files the OS opens us with (Dock icon, "Open with", command line, a
	// second launch) are gathered into batches, held until the frontend says
	// it is ready, then sent down the same path as a window drop.
	var app *application.App
	launcher := launch.New(launchDebounce, func(paths []string) {
		if svc.Busy() {
			// Simplest robust option: tell the user instead of queueing.
			msgs := svc.Messages()
			app.Dialog.Info().SetTitle(msgs["busy.title"]).SetMessage(msgs["busy.message"]).Show()
			return
		}
		go svc.Run(paths, "") // "" = the remembered format
	})

	app = application.New(application.Options{
		Name:        "AudioExtractor",
		Description: "Extract the audio from a Zoom recording",
		Services:    []application.Service{application.NewService(svc)},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		// One instance only: a second launch (say, files dropped on the exe
		// while it runs) forwards its arguments here and exits.
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "com.motdiem.audioextractor",
			OnSecondInstanceLaunch: func(data application.SecondInstanceData) {
				if len(data.Args) > 1 {
					launcher.Add(launch.Filter(data.Args[1:], nil)...)
				}
				if w := port.Window; w != nil {
					w.Restore()
					w.Focus()
				}
			},
		},
	})
	port.App = app

	// macOS delivers the files of a Dock drop or "Open with" as one event per
	// file, both at cold launch and while running.
	app.Event.OnApplicationEvent(events.Common.ApplicationOpenedWithFile, func(e *application.ApplicationEvent) {
		if f := e.Context().Filename(); f != "" {
			launcher.Add(f)
		}
	})
	app.Event.On("frontend:ready", func(*application.CustomEvent) { launcher.Ready() })

	// Windows passes the files on the command line. (On macOS the arguments
	// are not files: the open events above carry them.)
	if runtime.GOOS == "windows" {
		launcher.Add(launch.Filter(os.Args[1:], nil)...)
	}

	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "AudioExtractor",
		Width:     420,
		Height:    320,
		MinWidth:  420,
		MinHeight: 320,
		MaxWidth:  420,
		MaxHeight: 320,
		URL:       "/",
		// Files dropped on an element marked data-file-drop-target raise
		// WindowFilesDropped below.
		EnableFileDrop: true,
	})
	port.Window = win

	win.OnWindowEvent(events.Common.WindowFilesDropped, func(e *application.WindowEvent) {
		files := e.Context().DroppedFiles()
		if len(files) == 0 {
			return
		}
		// The handler must return quickly: the dialog and ffmpeg run on their own goroutine.
		go svc.Run(files, "") // "" = the remembered format
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// locateFFmpeg returns the ffmpeg bundled in the binary. In a dev build made
// without running `wails3 task ffmpeg:mac` (or `ffmpeg:windows`) first, nothing is embedded, so it
// falls back to an ffmpeg on PATH.
func locateFFmpeg() (string, error) {
	p, err := ffmpeg.Path()
	if errors.Is(err, ffmpeg.ErrNotBundled) {
		return exec.LookPath("ffmpeg")
	}
	return p, err
}
