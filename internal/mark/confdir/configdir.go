package confdir

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrNotAbsolute means the platform named a configuration directory that is not
// an absolute path. Such a path is read against whichever folder the process
// started in, and yomihon started with no folder named serves the one it
// started in, so the reader's files would land in the folder being read.
var ErrNotAbsolute = errors.New("the configuration directory is not an absolute path")

// ConfigDir is the platform's configuration directory: where this platform
// keeps a program's own files for the person running it, and the directory
// every Dir is made inside.
//
// Platform makes the one the program uses, and At makes one for a test; nothing
// else can, inside this package or out of it. So a vault's root, which is a
// string, cannot be handed to Home or VaultDir where the configuration
// directory is meant. The zero ConfigDir names none, which is what a command
// holds when the environment names none, and Home and VaultDir refuse it.
type ConfigDir struct{ dir string }

// Platform is the configuration directory the platform names for the person
// running yomihon: os.UserConfigDir, which reads HOME, XDG_CONFIG_HOME on
// Linux, and %AppData% on Windows. It is the one place the program asks, and the
// environment test in cmd/yomihon permits that read in this file alone.
//
// It reads the environment and not the disk. A directory that is not there yet
// is named as it is, since the first write makes it; so is one that cannot be
// read, or that is reached through a symbolic link, and what stops a write in
// it is what that write reports. A path that is not absolute is refused with
// ErrNotAbsolute, and an environment that names none answers the error
// os.UserConfigDir gives.
func Platform() (ConfigDir, error) {
	path, err := os.UserConfigDir()
	if err != nil {
		return ConfigDir{}, err
	}
	if err := absolute(path); err != nil {
		return ConfigDir{}, err
	}
	return ConfigDir{dir: path}, nil
}

// At is a configuration directory at path, for a test that keeps the reader's
// files in a directory of its own. Shipped code may not call it: the write test
// in internal/archlock refuses it in every package the program is built from,
// since a ConfigDir made from any path is what would let a vault's root stand
// for the configuration directory.
//
// A path that is not absolute is a mistake in the test that wrote it, so At
// panics on one rather than give a test a directory relative to wherever it
// runs.
func At(path string) ConfigDir {
	if err := absolute(path); err != nil {
		panic("confdir: At: " + err.Error())
	}
	return ConfigDir{dir: path}
}

// IsZero reports whether c names no configuration directory.
func (c ConfigDir) IsZero() bool { return c.dir == "" }

// absolute refuses a path that is not absolute, the empty one included.
func absolute(path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("%w: %q", ErrNotAbsolute, path)
	}
	return nil
}
