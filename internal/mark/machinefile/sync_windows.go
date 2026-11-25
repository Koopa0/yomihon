package machinefile

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// directory is a folder opened to be synced.
type directory struct{ handle windows.Handle }

// openDirectory opens the folder root is on to sync it. Windows flushes a
// handle only if it may write, and os.Root opens a folder to read alone, so the
// folder is opened again for writing by the path Windows gives the handle root
// opened, and the new handle is held to being that same folder. Nothing is
// written through it: the only call made with it is the flush.
func openDirectory(root *os.Root) (directory, error) {
	opened, err := root.Open(".")
	if err != nil {
		return directory{}, err
	}
	d, err := reopenToFlush(windows.Handle(opened.Fd()))
	if closeErr := opened.Close(); closeErr != nil {
		if err == nil {
			err = errors.Join(closeErr, d.close())
		}
		return directory{}, err
	}
	return d, err
}

// reopenToFlush opens the folder that read is open on again, for writing.
func reopenToFlush(read windows.Handle) (directory, error) {
	var path [windows.MAX_LONG_PATH]uint16
	n, err := windows.GetFinalPathNameByHandle(read, &path[0], windows.MAX_LONG_PATH, 0)
	if err != nil {
		return directory{}, fmt.Errorf("find the folder's path: %w", err)
	}
	if n >= windows.MAX_LONG_PATH {
		return directory{}, fmt.Errorf("find the folder's path: %w", windows.ERROR_FILENAME_EXCED_RANGE)
	}
	handle, err := windows.CreateFile(&path[0], windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return directory{}, fmt.Errorf("open %s to flush it: %w", windows.UTF16ToString(path[:]), err)
	}
	d := directory{handle: handle}
	var was, is windows.ByHandleFileInformation
	if err := errors.Join(windows.GetFileInformationByHandle(read, &was), windows.GetFileInformationByHandle(handle, &is)); err != nil {
		return directory{}, errors.Join(err, d.close())
	}
	if was.VolumeSerialNumber != is.VolumeSerialNumber || was.FileIndexHigh != is.FileIndexHigh || was.FileIndexLow != is.FileIndexLow {
		return directory{}, errors.Join(errors.New("the folder was moved while it was opened to sync"), d.close())
	}
	return d, nil
}

func (d directory) sync() error {
	if err := windows.FlushFileBuffers(d.handle); err != nil {
		return fmt.Errorf("flush the folder: %w", err)
	}
	return nil
}

func (d directory) close() error {
	if err := windows.CloseHandle(d.handle); err != nil {
		return fmt.Errorf("close the folder opened to flush it: %w", err)
	}
	return nil
}
