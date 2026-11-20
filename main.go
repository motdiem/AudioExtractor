package main

import (
	"embed"
	"log"
	"os/exec"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/motdiem/AudioExtractor/internal/desktop"
	"github.com/motdiem/AudioExtractor/internal/extract"
	"github.com/motdiem/AudioExtractor/internal/i18n"
)

// The built frontend (frontend/dist) is embedded into the binary.
//
//go:embed all:frontend/dist
var assets embed.FS

func main() {
	port := &desktop.Port{}
	svc := extract.New(extract.Config{
		// TODO(#3): use ffmpeg.Path()
		Locate: func() (string, error) { return exec.LookPath("ffmpeg") },
		Port:   port,
		Emit: func(name string, data any) {
			application.Get().Event.Emit(name, data)
		},
		Lang: i18n.Detect(),
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
		go svc.Run(files, string(extract.FormatM4A))
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
