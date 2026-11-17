package confdir

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// ErrLocked means the lock is held by another and did not come free in the time
// allowed.
var ErrLocked = errors.New("the lock is held by another")

// lockPoll is how often a waiter looks again.
const lockPoll = 10 * time.Millisecond

// Lock is an exclusive claim on a file name in a directory. It is held by
// creating that file with nothing else able to have created it first, which
// works on every platform yomihon is built for; the advisory locks of the
// operating system do not.
type Lock struct {
	dir    string
	name   string
	holder []byte
}

// Acquire takes the lock called name in dir, waiting up to wait for whoever
// holds it. holder is written into the lock file and says who holds it; it has
// to differ between two claimants, which is how Release can tell its own lock
// from one that took its place.
//
// A process that stops while holding the lock leaves the file behind, so the
// caller says what an abandoned lock looks like: abandoned is given what the
// file holds and how long ago it was made, and a lock it calls abandoned is
// removed and taken. A nil abandoned never takes a lock from its holder.
//
// Two waiters that both find the same lock abandoned can both remove it, and
// the second removal can take the lock the first waiter just made. That needs
// a crashed holder and two waiters in the same few microseconds, and what it
// costs is a read-modify-write that loses a concurrent change, which the reader
// repeats by pressing the same control; closing it would take an operating
// system lock this does not have.
func Acquire(dir, name string, holder []byte, abandoned func(holder []byte, age time.Duration) bool, wait time.Duration) (*Lock, error) {
	if err := checkName(name); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create the directory: %w", err)
	}
	path := filepath.Join(dir, name)
	deadline := time.Now().Add(wait)
	for {
		err := create(path, holder)
		if err == nil {
			return &Lock{dir: dir, name: name, holder: holder}, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return nil, fmt.Errorf("take the lock %s: %w", name, err)
		}
		retry, err := removeIfAbandoned(path, abandoned)
		if err != nil {
			return nil, err
		}
		if retry {
			continue
		}
		if !time.Now().Before(deadline) {
			return nil, fmt.Errorf("%w: %s", ErrLocked, name)
		}
		time.Sleep(lockPoll)
	}
}

// create makes the lock file and writes the holder into it. Of two callers
// only one finds no file there, which is the whole of the exclusion; the error
// for the other satisfies fs.ErrExist.
func create(path string, holder []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) // #nosec G304 -- a lock file of the configuration directory
	if err != nil {
		return err
	}
	_, writeErr := file.Write(holder)
	closeErr := file.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return errors.Join(fmt.Errorf("write the lock: %w", err), os.Remove(path))
	}
	return nil
}

// removeIfAbandoned takes away a lock the caller judges abandoned, and says
// whether the lock should be tried for again at once: it was removed, or it was
// gone, or it changed while it was being judged. It removes the file only while
// it still holds what was judged, so a lock made in the meantime is not the one
// taken away.
func removeIfAbandoned(path string, abandoned func(holder []byte, age time.Duration) bool) (retry bool, err error) {
	if abandoned == nil {
		return false, nil
	}
	held, age, err := readLock(path)
	if err != nil {
		return errors.Is(err, fs.ErrNotExist), nil
	}
	if !abandoned(held, age) {
		return false, nil
	}
	if !stillHolds(path, held) {
		return true, nil
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, fmt.Errorf("remove the abandoned lock: %w", err)
	}
	return true, nil
}

// stillHolds says whether the lock file holds exactly held. A file that cannot
// be read does not, and is looked at again.
func stillHolds(path string, held []byte) bool {
	now, _, err := readLock(path)
	return err == nil && bytes.Equal(now, held)
}

// readLock is what a lock file holds and how long ago it was made. A file that
// has just been made and not yet written reads as empty.
func readLock(path string) (holder []byte, age time.Duration, err error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, 0, err
	}
	holder, err = os.ReadFile(path) // #nosec G304 -- a lock file of the configuration directory
	if err != nil {
		return nil, 0, err
	}
	return holder, time.Since(info.ModTime()), nil
}

// Release gives the lock back. A lock that is no longer the holder's own, taken
// away as abandoned while the holder was merely slow, is left where it is.
func (l *Lock) Release() error {
	path := filepath.Join(l.dir, l.name)
	held, _, err := readLock(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("release the lock %s: %w", l.name, err)
	}
	if !bytes.Equal(held, l.holder) {
		return nil
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("release the lock %s: %w", l.name, err)
	}
	return nil
}
