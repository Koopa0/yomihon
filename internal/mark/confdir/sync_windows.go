package confdir

import (
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
// opened here with write access.
func openDirectory(path string) (directory, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return directory{}, &os.PathError{Op: "open", Path: path, Err: err}
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE,
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
