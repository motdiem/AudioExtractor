// Package desktop is the real implementation of extract.Port, on top of Wails
// native dialogs and the OS file manager.
package desktop

import (
	"errors"
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/motdiem/AudioExtractor/internal/extract"
)

// Port implements extract.Port with Wails dialogs.
type Port struct {
	App    *application.App
	Window application.Window // dialogs attach to it (sheets on macOS); may be nil
}

var _ extract.Port = (*Port)(nil)

// ChooseInputs lets the user pick several files. On macOS folders can be
// picked too; the Windows dialog cannot mix files and folders, so there they
// are dropped on the window instead.
func (p *Port) ChooseInputs(req extract.PickRequest) ([]string, error) {
	d := p.App.Dialog.OpenFile().
		SetTitle(req.Title).
		CanChooseFiles(true).
		CanChooseDirectories(runtime.GOOS == "darwin")
	if p.Window != nil {
		d.AttachToWindow(p.Window)
	}
	return noCancelError(d.PromptForMultipleSelection())
}

func (p *Port) ChooseFolder(req extract.FolderRequest) (string, error) {
	d := p.App.Dialog.OpenFile().
		SetTitle(req.Title).
		SetMessage(req.Title).
		SetDirectory(req.Dir).
		CanChooseFiles(false).
		CanChooseDirectories(true).
		CanCreateDirectories(true)
	if p.Window != nil {
		d.AttachToWindow(p.Window)
	}
	return noCancelError(d.PromptForSingleSelection())
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
	return noCancelError(d.PromptForSingleSelection())
}

func (p *Port) Reveal(path string) error { return reveal(path) }

// CopyText puts text on the clipboard through Wails, which uses the native
// clipboard on each OS (more reliable than the webview's, which needs a user
// gesture and permission).
func (p *Port) CopyText(text string) error {
	if !p.App.Clipboard.SetText(text) {
		return errors.New("could not write to the clipboard")
	}
	return nil
}
