//go:build !windows

package extract

import "syscall"

// A synthetic "file is busy" error: the closest thing to a sharing violation
// that exists on this OS. Tests build it instead of provoking a real lock.
var syntheticLockErrno error = syscall.EBUSY
