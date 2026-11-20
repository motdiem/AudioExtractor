// Package desktop is the real implementation of extract.Port, on top of Wails
// native dialogs and the OS file manager.
package desktop

import (
	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/motdiem/AudioExtractor/internal/extract"
)

// Port implements extract.Port with Wails dialogs.
type Port struct {
	App    *application.App
	Window application.Window // dialogs attach to it (sheets on macOS); may be nil
}

var _ extract.Port = (*Port)(nil)

func (p *Port) ChooseInputs(req extract.PickRequest) ([]string, error) {
	d := p.App.Dialog.OpenFile().
		SetTitle(req.Title).
		CanChooseFiles(true).
		CanChooseDirectories(false)
	if p.Window != nil {
		d.AttachToWindow(p.Window)
	}
	path, err := d.PromptForSingleSelection()
	if err != nil || path == "" {
		return nil, err
	}
	return []string{path}, nil
}

func (p *Port) ChooseSaveFile(req extract.SaveRequest) (string, error) {
	d := p.App.Dialog.SaveFile().
		SetMessage(req.Title).
		SetDirectory(req.Dir).
		SetFilename(req.Name).
		CanCreateDirectories(true).
		AddFilter(req.FilterName, "*."+req.Extension)
	if p.Window != nil {
		d.AttachToWindow(p.Window)
	}
	return d.PromptForSingleSelection()
}

func (p *Port) Reveal(path string) error { return reveal(path) }
