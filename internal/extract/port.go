package extract

// Port is everything the service needs from the desktop shell: native dialogs
// and "show in the file manager". The real implementation (internal/desktop)
// uses Wails; tests use a fake. Every chooser returns an empty result when the
// user cancels, which is not an error.
type Port interface {
	// ChooseInputs opens the native file picker and returns the chosen paths.
	ChooseInputs(req PickRequest) ([]string, error)
	// ChooseFolder opens the native folder dialog (batches) and returns the
	// chosen folder.
	ChooseFolder(req FolderRequest) (string, error)
	// ChooseSaveFile opens the native Save dialog and returns the chosen path.
	// The OS handles the "replace existing file?" confirmation.
	ChooseSaveFile(req SaveRequest) (string, error)
	// Reveal shows path selected in Finder / Explorer.
	Reveal(path string) error
	// CopyText puts text on the system clipboard.
	CopyText(text string) error
}

// FolderRequest describes the folder dialog: outputs of a batch go into the
// folder chosen.
type FolderRequest struct {
	Title string
	Dir   string // opens here (the first source's folder)
}

// PickRequest describes the file picker.
type PickRequest struct {
	Title string
}

// SaveRequest describes the Save dialog: where it opens and what it pre-fills.
type SaveRequest struct {
	Title      string
	Dir        string // opens here (the source file's folder)
	Name       string // pre-filled file name, e.g. "call.m4a"
	FilterName string // e.g. "M4A audio"
	Extension  string // e.g. "m4a" (no dot)
}
