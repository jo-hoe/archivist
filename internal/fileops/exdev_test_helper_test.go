package fileops

import "syscall"

// exdevErr returns the cross-device link error for use in tests.
func exdevErr() error { return syscall.EXDEV }
