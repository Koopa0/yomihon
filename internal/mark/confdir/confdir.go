// Package confdir is the directory under the platform's configuration
// directory that holds the files yomihon keeps for one vault, and the only code
// that writes into it.
//
// It knows nothing of what the files hold. A reader of marks or of figure
// approvals asks it for the directory, reads a file through a cap and installs
// a new one by rename; the shape of the bytes is theirs. Keeping the writes
// here means the promise that yomihon writes only under the configuration
// directory rests on one function rather than on every package that has a file.
package confdir

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/koopa0/yomihon/internal/vault"
)

// ErrNotNamed means a directory was asked for without both of the things that
// name it: where this platform keeps a program's own files, and which vault the
// files belong to.
var ErrNotNamed = errors.New("the vault's files need a configuration directory and a vault root")

// ErrTooLarge means a file is longer than the limit it was read under. It is
// the answer for a file this program did not write, so it is read as damaged
// rather than taken in whole.
var ErrTooLarge = errors.New("the file is longer than its limit")

// nameLength is how much of the vault digest names the directory. Half of the
// digest distinguishes any two folders a person has, and a directory name is
// read by people who have to find it to delete it.
const nameLength = 32

// Dir is where this vault's files sit inside configDir.
//
// The name is derived from the vault root rather than spelled from it: a path
// is not a safe directory name, and two vaults must land in two directories.
// The root is normalized to NFC first, because a folder reached by two
// spellings of the same name is one vault and would otherwise be given two
// directories with two different sets of marks in them. The name says nothing
// about which vault it belongs to, which is what the vault field inside each
// file is for.
//
// Either argument empty is refused rather than defaulted. A path built from an
// empty configuration directory is a relative one, which would put the reader's
// files beside whatever folder the process happened to start in and look like
// it had worked.
func Dir(configDir, vaultRoot string) (string, error) {
	if configDir == "" || vaultRoot == "" {
		return "", fmt.Errorf("%w: configuration directory %q, vault root %q", ErrNotNamed, configDir, vaultRoot)
	}
	sum := sha256.Sum256([]byte(vault.NormalizeNFC(vaultRoot)))
	return filepath.Join(configDir, "yomihon", hex.EncodeToString(sum[:])[:nameLength]), nil
}

// Read returns the file called name in dir, refusing one longer than limit
// bytes with ErrTooLarge. A file that is not there answers an error that
// satisfies errors.Is(err, os.ErrNotExist).
func Read(dir, name string, limit int64) ([]byte, error) {
	file, err := os.Open(filepath.Join(dir, name)) // #nosec G304 -- a file of this vault's own directory under the configuration directory
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
// one and never half of either. Nothing is synchronized to the platter. What a
// power cut loses is one write, which the reader repeats by pressing the same
// control.
//
// Everything is owner-only: what a reader is partway through is theirs.
func Replace(dir, name string, data []byte) error {
	if name == "" || name == "." || name == ".." || name != filepath.Base(name) || strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("%q is not a file name inside the directory", name)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create the directory: %w", err)
	}
	extension := filepath.Ext(name)
	tmp, err := os.CreateTemp(dir, "."+strings.TrimSuffix(name, extension)+"-*"+extension)
	if err != nil {
		return fmt.Errorf("create the temporary file for %s: %w", name, err)
	}
	temporary := tmp.Name()
	if _, err = tmp.Write(data); err != nil {
		return abandon(tmp, temporary, fmt.Errorf("write the temporary file for %s: %w", name, err))
	}
	if err = tmp.Close(); err != nil {
		return abandon(nil, temporary, fmt.Errorf("close the temporary file for %s: %w", name, err))
	}
	if err = os.Rename(temporary, filepath.Join(dir, name)); err != nil {
		return abandon(nil, temporary, fmt.Errorf("install %s: %w", name, err))
	}
	return nil
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
