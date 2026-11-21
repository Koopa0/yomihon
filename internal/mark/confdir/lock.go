package confdir

import (
	"errors"
	"fmt"
	"os"
	"time"
)

// ErrLocked means the lock is held by another and did not come free in the time
// allowed.
var ErrLocked = errors.New("the lock is held by another")

const (
	// LockWait is how long a change waits for another process to finish its own.
	LockWait = 5 * time.Second
	// lockPoll is how often a waiter looks again.
	lockPoll = 10 * time.Millisecond
)

// Lock is an exclusive claim on a file in a directory, which the operating
// system holds on an open handle of that file: flock on Unix and LockFileEx on
// Windows. Two claims exclude each other whether two processes make them or two
// goroutines of one, and the system lets a claim go when the process holding it
// ends, however it ends, so no lock is left behind for a waiter to judge.
//
// The file is made by the first claim and never removed. A waiter that opened
// it before a removal would lock a file no longer at that name, while the next
// claimant made another and locked that.
type Lock struct {
	dir  Dir
	name string
	file *os.File
}

// Acquire takes the lock called name in dir, waiting up to wait for whoever
// holds it.
func Acquire(dir Dir, name string, wait time.Duration) (*Lock, error) {
	if err := dir.named(); err != nil {
		return nil, err
	}
	if err := checkName(name); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir.path, 0o700); err != nil {
		return nil, fmt.Errorf("create the directory: %w", err)
	}
	file, err := openLock(dir, name)
	if err != nil {
		return nil, fmt.Errorf("open the lock %s: %w", name, err)
	}
	deadline := time.Now().Add(wait)
	for {
		taken, err := tryLock(file)
		switch {
		case err != nil:
			return nil, errors.Join(fmt.Errorf("take the lock %s: %w", name, err), file.Close())
		case taken:
			return &Lock{dir: dir, name: name, file: file}, nil
		case !time.Now().Before(deadline):
			return nil, errors.Join(fmt.Errorf("%w: %s", ErrLocked, name), file.Close())
		}
		time.Sleep(lockPoll)
	}
}

// Release gives the lock back and closes its file, which stays for the next
// claim.
func (l *Lock) Release() error {
	if err := errors.Join(unlock(l.file), l.file.Close()); err != nil {
		return fmt.Errorf("release the lock %s: %w", l.dir.Join(l.name), err)
	}
	return nil
}

// Exclusive runs change holding the lock called name in dir, so that a file
// read inside change and replaced there is replaced from what the file holds
// now. The command line and the running server are two processes over the same
// directory, and without this the later of two changes writes over the earlier.
// It waits up to wait for the lock.
func Exclusive(dir Dir, name string, wait time.Duration, change func() error) (err error) {
	lock, err := Acquire(dir, name, wait)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, lock.Release()) }()
	return change()
}
