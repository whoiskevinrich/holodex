//go:build !linux && !windows

package rename

import "syscall"

// noReplace has no portable primitive here, so every rename takes the checked fallback.
func noReplace(old, new string) error { return syscall.ENOSYS }
