package confdir

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ErrLocked means the lock is held by another and did not come free in the time
// allowed.
var ErrLocked = errors.New("the lock is held by another")

const (
	// LockWait is how long a change waits for another process to finish its own.
	LockWait = 5 * time.Second
	// firstPause and lastPause bound how long a waiter sleeps between two looks
	// at a lock another process holds. A change holds it while its file and
	// directory reach stable storage, often a millisecond or two, so the first
	// pause is that short; each pause after is twice the one before, up to
	// lastPause, so that a waiter kept out by a long change, a compaction say,
	// does not wake a thousand times a second, and a lock that comes free is
	// never left unseen for longer than lastPause.
	firstPause = time.Millisecond
	lastPause  = 50 * time.Millisecond
)

// Lock is an exclusive claim on a file in a directory, which the operating
// system holds on an open handle of that file: flock on Unix and LockFileEx on
// Windows. Two claims exclude each other whether two processes make them or two
// goroutines of one, and the system lets a claim go when the process holding it
// ends, however it ends, so no lock is left behind for a waiter to judge.
//
// The claims of one process wait in a queue of their own, one for each lock
// file, and are let through it in the order they were made; only the one at its
// head asks the system, and only another process's claims are waited for by
// looking again and again. The system's lock is not fair, and a poll is not
// either: a claim that has just let the lock go and asks again would take it
// before a waiter woke to look.
//
// The file is made by the first claim and never removed. A waiter that opened
// it before a removal would lock a file no longer at that name, while the next
// claimant made another and locked that.
type Lock struct {
	dir  Dir
	name string
	file *os.File
	// turn is this process's queue for the lock, whose one place the Lock holds
	// until it is released.
	turn chan struct{}
}

// turns are this process's queues, one for each lock file it has asked for, of
// which a process has a few: each holds one place, which a claim takes before
// it asks the system and gives back when it is released.
var turns = struct {
	mu sync.Mutex
	of map[string]chan struct{}
}{of: map[string]chan struct{}{}}

// turnOf is this process's queue for the lock file at path.
func turnOf(path string) chan struct{} {
	turns.mu.Lock()
	defer turns.mu.Unlock()
	turn, ok := turns.of[path]
	if !ok {
		turn = make(chan struct{}, 1)
		turns.of[path] = turn
	}
	return turn
}

// Acquire takes the lock called name in dir, waiting up to wait for whoever
// holds it, in this process or another, and for no longer than ctx allows: a ctx
// that is done ends the wait with its error. A lock that is free is taken
// whatever ctx says, as with a wait of zero, so a change that is part way
// through is not stopped between two of its steps by a cancellation that asked
// it to wait no longer.
func Acquire(ctx context.Context, dir Dir, name string, wait time.Duration) (*Lock, error) {
	if err := dir.named(); err != nil {
		return nil, err
	}
	if err := checkName(name); err != nil {
		return nil, err
	}
	if err := makeDurably(dir.path); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(wait)
	turn := turnOf(dir.Join(name))
	if err := waitTurn(ctx, turn, deadline, name); err != nil {
		return nil, err
	}
	file, err := openLock(dir, name)
	if err != nil {
		<-turn
		return nil, fmt.Errorf("open the lock %s: %w", name, err)
	}
	if err = waitFile(ctx, file, deadline, name); err != nil {
		<-turn
		return nil, errors.Join(err, file.Close())
	}
	return &Lock{dir: dir, name: name, file: file, turn: turn}, nil
}

// waitTurn takes the one place of this process's queue for the lock, behind
// every claim of this process that asked before it, until deadline or until ctx
// is done. A place that is free is taken at once.
func waitTurn(ctx context.Context, turn chan struct{}, deadline time.Time, name string) error {
	select {
	case turn <- struct{}{}:
		return nil
	default:
	}
	left := time.Until(deadline)
	if left <= 0 {
		return fmt.Errorf("%w: %s", ErrLocked, name)
	}
	timer := time.NewTimer(left)
	defer timer.Stop()
	select {
	case turn <- struct{}{}:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("wait for the lock %s: %w", name, ctx.Err())
	case <-timer.C:
		return fmt.Errorf("%w: %s", ErrLocked, name)
	}
}

// waitFile takes the system's lock on file, looking again after each pause
// while another process holds it, until deadline or until ctx is done.
func waitFile(ctx context.Context, file *os.File, deadline time.Time, name string) error {
	for pause := firstPause; ; pause = min(2*pause, lastPause) {
		taken, err := tryLock(file)
		switch {
		case err != nil:
			return fmt.Errorf("take the lock %s: %w", name, err)
		case taken:
			return nil
		}
		left := time.Until(deadline)
		if left <= 0 {
			return fmt.Errorf("%w: %s", ErrLocked, name)
		}
		if err = sleep(ctx, min(jittered(pause), left)); err != nil {
			return fmt.Errorf("wait for the lock %s: %w", name, err)
		}
	}
}

// jittered is a pause between half of pause and all of it, chosen at random, so
// that waiters that began together, or that a coarse system timer wakes on the
// same tick, do not keep looking at the lock in step.
func jittered(pause time.Duration) time.Duration {
	half := pause / 2
	return half + rand.N(pause-half+1) // #nosec G404 -- spreads the waiters' looks; nothing depends on it being unpredictable
}

// sleep waits for d, or until ctx is done, which it answers with ctx's error.
func sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// Release gives the lock back and closes its file, which stays for the next
// claim, and then lets the next claim of this process through its queue.
func (l *Lock) Release() error {
	err := errors.Join(unlock(l.file), l.file.Close())
	if l.turn != nil {
		<-l.turn
		l.turn = nil
	}
	if err != nil {
		return fmt.Errorf("release the lock %s: %w", l.dir.Join(l.name), err)
	}
	return nil
}

// Exclusive runs change holding the lock called name in dir, so that a file
// read inside change and replaced there is replaced from what the file holds
// now. The command line and the running server are two processes over the same
// directory, and without this the later of two changes writes over the earlier.
// It waits for the lock as Acquire does.
func Exclusive(ctx context.Context, dir Dir, name string, wait time.Duration, change func() error) (err error) {
	lock, err := Acquire(ctx, dir, name, wait)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, lock.Release()) }()
	return change()
}

// makeDurably makes path and the directories above it that are not there, and
// syncs the directory above each one it made, so that a file a later call puts
// in path, and syncs only path for, is not lost with a directory whose own entry
// never reached stable storage. Replace and Append do the same for the
// directories they make; this is for the lock, which makes them first.
func makeDurably(path string) error {
	made := absent(path)
	if err := os.MkdirAll(path, 0o700); err != nil {
		return fmt.Errorf("create the directory: %w", err)
	}
	if len(made) == 0 {
		return nil
	}
	directories, err := openDirectories(filepath.Dir(made[0]), made[1:])
	if err != nil {
		return fmt.Errorf("open the directory above one this made to sync it: %w", err)
	}
	for _, d := range directories {
		err = errors.Join(err, d.sync(), d.close())
	}
	if err != nil {
		return fmt.Errorf("sync the directory above one this made: %w", err)
	}
	return nil
}
