package extract

import (
	"errors"
	"syscall"
)

// Windows error numbers (winerror.h) that syscall does not name.
const (
	errorAccessDenied     syscall.Errno = 5  // replacing a file another program holds open
	errorSharingViolation syscall.Errno = 32 // opened without sharing by another program
	errorLockViolation    syscall.Errno = 33
	errorHandleDiskFull   syscall.Errno = 39
	errorDiskFull         syscall.Errno = 112
)

// isLockError reports whether err means another program is holding the file.
// Only call it for an error from replacing a file that exists: there,
// ACCESS_DENIED is how MoveFileEx reports a file that is open elsewhere.
func isLockError(err error) bool {
	for _, e := range []syscall.Errno{errorSharingViolation, errorLockViolation, errorAccessDenied} {
		if errors.Is(err, e) {
			return true
		}
	}
	return false
}

func isDiskFull(err error) bool {
	return errors.Is(err, errorDiskFull) || errors.Is(err, errorHandleDiskFull)
}
