package extract

import "syscall"

// ERROR_SHARING_VIOLATION, what Windows returns when another program has the
// file open without sharing.
var syntheticLockErrno error = syscall.Errno(32)
