package main

import (
	"embed"
	"errors"
	"log"
	"os/exec"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/motdiem/AudioExtractor/internal/desktop"
	"github.com/motdiem/AudioExtractor/internal/extract"
	"github.com/motdiem/AudioExtractor/internal/ffmpeg"
	"github.com/motdiem/AudioExtractor/internal/i18n"
	"github.com/motdiem/AudioExtractor/internal/settings"
)

// The built frontend (frontend/dist) is embedded into the binary.
//
//go:embed all:frontend/dist
var assets embed.FS

func main() {
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
	})

	app := application.New(application.Options{
		Name:        "AudioExtractor",
		Description: "Extract the audio from a Zoom recording",
		Services:    []application.Service{application.NewService(svc)},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})
	port.App = app

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
