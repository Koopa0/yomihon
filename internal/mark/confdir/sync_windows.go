package confdir

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// directory is a directory opened to be synced.
type directory struct {
	handle windows.Handle
	path   string
}

// openDirectory opens a directory to be synced. Windows flushes a handle only
// if it may write, and os opens a directory for reading alone, so the handle is
// opened here for writing, which is all a flush asks for.
func openDirectory(path string) (directory, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return directory{}, &os.PathError{Op: "open", Path: path, Err: err}
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return directory{}, &os.PathError{Op: "open", Path: path, Err: err}
	}
	return directory{handle: handle, path: path}, nil
}

func (d directory) sync() error {
	if err := windows.FlushFileBuffers(d.handle); err != nil {
		return &os.PathError{Op: "sync", Path: d.path, Err: err}
	}
	return nil
}

func (d directory) close() error {
	if err := windows.CloseHandle(d.handle); err != nil {
		return &os.PathError{Op: "close", Path: d.path, Err: err}
	}
	return nil
}

// transient says whether a rename that failed is worth trying again. Windows
// refuses a rename over a file another has open without sharing deletion, as
// access denied or a sharing violation, until the other closes it; a reader of
// the ledger, which takes no lock, holds it open for the instant it takes to
// read it. A permission that is missing refuses it the same way, and is tried
// the same few times before it is reported.
func transient(err error) bool {
	return errors.Is(err, windows.ERROR_ACCESS_DENIED) || errors.Is(err, windows.ERROR_SHARING_VIOLATION)
}
