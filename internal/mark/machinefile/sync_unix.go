//go:build unix

package machinefile

import "os"

// directory is a folder opened to be synced.
type directory struct{ file *os.File }

// openDirectory opens the folder root is on to sync it, which on Unix any open
// of it can be.
func openDirectory(root *os.Root) (directory, error) {
	file, err := root.Open(".")
	return directory{file: file}, err
}

func (d directory) sync() error  { return d.file.Sync() }
func (d directory) close() error { return d.file.Close() }
