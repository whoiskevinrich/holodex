//go:build windows

package rename

import (
	"golang.org/x/sys/windows"
)

// noReplace is MoveFileEx without MOVEFILE_REPLACE_EXISTING: Windows refuses an existing target.
func noReplace(old, new string) error {
	from, err := windows.UTF16PtrFromString(old)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(new)
	if err != nil {
		return err
	}
	err = windows.MoveFileEx(from, to, 0)
	if err == windows.ERROR_ALREADY_EXISTS || err == windows.ERROR_FILE_EXISTS {
		return ErrExists
	}
	return err
}
