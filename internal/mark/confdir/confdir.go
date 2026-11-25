// Package confdir is the directory under the platform's configuration
// directory that holds the files yomihon keeps for one vault, and the only code
// that writes into it.
//
// It knows nothing of what the files hold. A reader of marks or of figure
// approvals asks it for the directory, reads a file through a cap and installs
// a new one by rename; the shape of the bytes is theirs. Keeping the writes
// here means the promise that yomihon writes only under the configuration
// directory rests on one function rather than on every package that has a file.
//
// What it works on is a Dir, which only Home and VaultDir make. A function that
// changes a file takes one and never a path, so every write lands in a directory
// one of the two named. They make it inside a ConfigDir and never from a string,
// and a ConfigDir is made by Platform, from what the platform names, or by At,
// which shipped code may not call. That the platform's answer is not inside a
// folder being read is not something this package checks.
package confdir

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/koopa0/yomihon/internal/vault"
)

// ErrNotNamed means a directory was asked for without both of the things that
// name it: where this platform keeps a program's own files, and which vault the
// files belong to. The zero ConfigDir is the first of them missing.
var ErrNotNamed = errors.New("the vault's files need a configuration directory and a vault root")

// ErrTooLarge means a file is longer than the limit it was read under. It is
// the answer for a file this program did not write, so it is read as damaged
// rather than taken in whole.
var ErrTooLarge = errors.New("the file is longer than its limit")

// ErrNotRegular means the name is something other than a file: a named pipe, a
// device, a directory. Opening one could wait for ever for a writer that never
// comes, so it is refused before it is opened, and read as damaged.
var ErrNotRegular = errors.New("the name is not a regular file")

// nameLength is how much of the vault digest names the directory. Half of the
// digest distinguishes any two folders a person has, and a directory name is
// read by people who have to find it to delete it.
const nameLength = 32

// Dir is a directory of yomihon's own under the configuration directory: the
// reader's home, or one vault's directory inside it. Home and VaultDir are the
// only ways to have one, and nothing outside this package can fill one in or
// change where it points, so the compiler keeps every change to a file inside a
// directory one of them named, and they name one only inside a ConfigDir. The
// zero Dir names nothing and is refused by everything here.
type Dir struct{ path string }

// Path is where the directory is, for a message that tells the reader which
// file to look at. It is a string and changes nothing.
func (d Dir) Path() string { return d.path }

// Join is the path of a file inside the directory, for the same use as Path.
func (d Dir) Join(name string) string { return filepath.Join(d.path, name) }

// named refuses the zero Dir, which would put a file beside whatever folder the
// process happened to start in.
func (d Dir) named() error {
	if d.path == "" {
		return fmt.Errorf("%w: no directory", ErrNotNamed)
	}
	return nil
}

// Home is the directory yomihon keeps everything of the reader's in: the
// per-vault directories VaultDir names, and the files that belong to the reader
// and not to one vault, such as the list of vaults they have opened.
//
// The zero ConfigDir is refused rather than defaulted, for the reason VaultDir
// gives.
func Home(configDir ConfigDir) (Dir, error) {
	if configDir.IsZero() {
		return Dir{}, fmt.Errorf("%w: no configuration directory", ErrNotNamed)
	}
	return Dir{path: filepath.Join(configDir.dir, "yomihon")}, nil
}

// VaultID is what names one vault: the directory Dir gives it, and the id the
// list of vaults gives it. They are the same name so that a vault added again
// after it was removed finds its marks where it left them, and so that a page
// can hold the id without holding the path.
//
// The name is derived from the vault root rather than spelled from it: a path
// is not a safe directory name, and two vaults must land in two directories.
// The root is normalized to NFC first, because a folder reached by two
// spellings of the same name is one vault and would otherwise be given two
// directories with two different sets of marks in them. The name says nothing
// about which vault it belongs to, which is what the vault field inside each
// file is for.
func VaultID(vaultRoot string) string {
	sum := sha256.Sum256([]byte(vault.NormalizeNFC(vaultRoot)))
	return hex.EncodeToString(sum[:])[:nameLength]
}

// VaultDir is where this vault's files sit inside configDir.
//
// The zero ConfigDir and an empty vault root are refused rather than
// defaulted. A path built from no configuration directory is a relative one,
// which would put the reader's files beside whatever folder the process
// happened to start in and look like it had worked.
func VaultDir(configDir ConfigDir, vaultRoot string) (Dir, error) {
	if configDir.IsZero() || vaultRoot == "" {
		return Dir{}, fmt.Errorf("%w: configuration directory %q, vault root %q", ErrNotNamed, configDir.dir, vaultRoot)
	}
	home, err := Home(configDir)
	if err != nil {
		return Dir{}, err
	}
	return Dir{path: filepath.Join(home.path, VaultID(vaultRoot))}, nil
}

// Read returns the file called name in dir, refusing one longer than limit
// bytes with ErrTooLarge. A file that is not there answers an error that
// satisfies errors.Is(err, os.ErrNotExist).
func Read(dir Dir, name string, limit int64) ([]byte, error) {
	if err := dir.named(); err != nil {
		return nil, err
	}
	if err := checkName(name); err != nil {
		return nil, err
	}
	path := dir.Join(name)
	if info, err := os.Stat(path); err != nil {
		return nil, err
	} else if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: %s", ErrNotRegular, name)
	}
	file, err := os.Open(path) // #nosec G304 -- a file of this vault's own directory under the configuration directory
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%w: %d bytes", ErrTooLarge, limit)
	}
	return data, nil
}

// Replace installs data as the file called name in dir, creating dir on the
// first write so that a reader who keeps nothing leaves nothing behind.
//
// The bytes go to a sibling in the same directory and are renamed over the
// file, so the rename stays inside one filesystem and is the atomic operation
// it looks like: a machine that stops mid-write leaves the old file or the new
// one and never half of either. Nil means the write is on stable storage: the
// sibling is synced before the rename, the directory after it, and so is the
// directory above each one this call made; then the file is read back and
// compared with data. Any step that fails is an error, and one that fails
// before the rename leaves the file holding what it held.
//
// Everything is owner-only: what a reader is partway through is theirs.
func Replace(dir Dir, name string, data []byte) error {
	if err := dir.named(); err != nil {
		return err
	}
	if err := checkName(name); err != nil {
		return err
	}
	made := absent(dir.path)
	if err := os.MkdirAll(dir.path, 0o700); err != nil {
		return fmt.Errorf("create the directory: %w", err)
	}
	// Opened before anything is installed, so that a directory that cannot be
	// synced refuses the write while the file still holds what it held.
	directories, err := openDirectories(dir.path, made)
	if err != nil {
		return fmt.Errorf("open the directory of %s to sync it: %w", name, err)
	}
	err = install(dir, name, data, directories)
	for _, d := range directories {
		err = errors.Join(err, d.close())
	}
	return err
}

// install writes data to a sibling of the file called name, syncs it, renames it
// over the file, syncs directories, and reads the file back.
func install(dir Dir, name string, data []byte, directories []directory) error {
	extension := filepath.Ext(name)
	tmp, err := os.CreateTemp(dir.path, "."+strings.TrimSuffix(name, extension)+"-*"+extension)
	if err != nil {
		return fmt.Errorf("create the temporary file for %s: %w", name, err)
	}
	temporary := tmp.Name()
	if _, err = tmp.Write(data); err != nil {
		return abandon(tmp, temporary, fmt.Errorf("write the temporary file for %s: %w", name, err))
	}
	if err = tmp.Sync(); err != nil {
		return abandon(tmp, temporary, fmt.Errorf("sync the temporary file for %s: %w", name, err))
	}
	if err = tmp.Close(); err != nil {
		return abandon(nil, temporary, fmt.Errorf("close the temporary file for %s: %w", name, err))
	}
	if err = os.Rename(temporary, dir.Join(name)); err != nil {
		return abandon(nil, temporary, fmt.Errorf("install %s: %w", name, err))
	}
	for _, d := range directories {
		if err = d.sync(); err != nil {
			return fmt.Errorf("sync the directory after installing %s: %w", name, err)
		}
	}
	held, err := Read(dir, name, int64(len(data)))
	if err != nil {
		return fmt.Errorf("read %s back: %w", name, err)
	}
	if !bytes.Equal(held, data) {
		return fmt.Errorf("%s does not read back as what was written", name)
	}
	return nil
}

// absent is path and each directory above it that is not there, nearest first:
// the ones MkdirAll is about to make.
func absent(path string) []string {
	var missing []string
	for {
		if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
			return missing
		}
		missing = append(missing, path)
		parent := filepath.Dir(path)
		if parent == path {
			return missing
		}
		path = parent
	}
}

// openDirectories opens path, which the file is renamed into, and the directory
// above each of made, which holds an entry this write is making.
func openDirectories(path string, made []string) ([]directory, error) {
	paths := []string{path}
	for _, below := range made {
		paths = append(paths, filepath.Dir(below))
	}
	opened := make([]directory, 0, len(paths))
	for _, path := range paths {
		d, err := openDirectory(path)
		if err != nil {
			for _, open := range opened {
				_ = open.close() //nolint:errcheck // the open that failed is the cause to report
			}
			return nil, err
		}
		opened = append(opened, d)
	}
	return opened, nil
}

// abandon removes a temporary file whose bytes will not be installed and
// returns the failure that stopped it. The removal is best-effort: the cause is
// what the caller has to act on, and a leftover file under a dot name is not
// something to replace that cause with.
func abandon(open *os.File, name string, cause error) error {
	if open != nil {
		_ = open.Close() //nolint:errcheck // the cause is the actionable failure
	}
	if err := os.Remove(name); err != nil && !errors.Is(err, os.ErrNotExist) {
		return errors.Join(cause, fmt.Errorf("remove the temporary file: %w", err))
	}
	return cause
}

// checkName refuses anything that is not one file name inside the directory it
// is given, so that no caller can reach outside it by the name it passes.
func checkName(name string) error {
	if name == "" || name == "." || name == ".." || name != filepath.Base(name) || strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("%q is not a file name inside the directory", name)
	}
	return nil
}

// Remove deletes the file called name in dir. A file that is not there is
// already what was asked for.
func Remove(dir Dir, name string) error {
	if err := dir.named(); err != nil {
		return err
	}
	if err := checkName(name); err != nil {
		return err
	}
	if err := os.Remove(dir.Join(name)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// SetAside renames the file called from in dir to the name to, replacing one
// that is already there. It is how a file that cannot be read as what it should
// be is kept for the reader to open, without ever being read in whole or
// deleted. A file that is not there is already set aside.
func SetAside(dir Dir, from, to string) error {
	if err := dir.named(); err != nil {
		return err
	}
	if err := checkName(from); err != nil {
		return err
	}
	if err := checkName(to); err != nil {
		return err
	}
	if err := os.Rename(dir.Join(from), dir.Join(to)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Stat is how long the file called name in dir is, in bytes, and when it last
// changed, without reading it. It is what a poller asks first, so a file that has not
// changed costs one system call and not a parse. A file that is not there
// answers an error that satisfies errors.Is(err, os.ErrNotExist).
func Stat(dir Dir, name string) (int64, time.Time, error) {
	if err := dir.named(); err != nil {
		return 0, time.Time{}, err
	}
	if err := checkName(name); err != nil {
		return 0, time.Time{}, err
	}
	info, err := os.Stat(dir.Join(name))
	if err != nil {
		return 0, time.Time{}, err
	}
	return info.Size(), info.ModTime(), nil
}
