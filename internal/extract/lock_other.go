//go:build !windows

package extract

import (
	"errors"
	"syscall"
)

// isLockError reports whether err means another program is holding the file.
func isLockError(err error) bool {
	return errors.Is(err, syscall.ETXTBSY) || errors.Is(err, syscall.EBUSY)
}

func isDiskFull(error) bool { return false }
