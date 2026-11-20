package desktop

import (
	"errors"
	"fmt"
	"testing"
)

func TestNoCancelError(t *testing.T) {
	// The Windows dialog error, as Wails returns it (possibly wrapped).
	cancelled := fmt.Errorf("dialog: %w", errors.New("cancelled by user"))
	if v, err := noCancelError("x", cancelled); v != "" || err != nil {
		t.Errorf("cancel: got (%q, %v), want (\"\", nil)", v, err)
	}
	if v, err := noCancelError([]string{"a"}, cancelled); v != nil || err != nil {
		t.Errorf("cancel (multi): got (%q, %v), want (nil, nil)", v, err)
	}
	boom := errors.New("access denied")
	if _, err := noCancelError("", boom); err != boom {
		t.Errorf("real error: got %v, want it passed through", err)
	}
	if v, err := noCancelError("C:\\a.m4a", nil); v != "C:\\a.m4a" || err != nil {
		t.Errorf("ok: got (%q, %v)", v, err)
	}
}
