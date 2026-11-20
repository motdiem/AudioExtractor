package desktop

import "strings"

// noCancelError turns a dialog the user cancelled into an empty result with
// no error, which is how the extract service reads "cancelled" (back to the
// drop zone). On macOS Wails already does this; on Windows it returns the
// common-file-dialog package's ErrorCancelled, which lives in an internal
// Wails package, so only its message can be matched.
func noCancelError[T any](v T, err error) (T, error) {
	if isCancelled(err) {
		var zero T
		return zero, nil
	}
	return v, err
}

func isCancelled(err error) bool {
	return err != nil && strings.Contains(err.Error(), "cancelled by user")
}
