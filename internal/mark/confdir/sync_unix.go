//go:build unix

package confdir

import "os"

// directory is a directory opened to be synced.
type directory struct{ file *os.File }

// openDirectory opens a directory to be synced, which on Unix any open of it can
// be.
func openDirectory(path string) (directory, error) {
	file, err := os.Open(path) // #nosec G304 -- a directory of the configuration directory, opened to sync it
	return directory{file: file}, err
}

func (d directory) sync() error  { return d.file.Sync() }
func (d directory) close() error { return d.file.Close() }
