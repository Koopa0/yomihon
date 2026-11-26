package confdir

import (
	"context"
	"errors"
	"os"
	"testing"
	"testing/synctest"
	"time"
)

// heldElsewhere is the lock called name in a directory of the test's own, held
// as another process holds it: by the system, on a file this process opened a
// second time, with no place in this process's queue. It answers the file a
// waiter of this process looks with, and the one holding the lock.
func heldElsewhere(t *testing.T, name string) (waiter, holder *os.File) {
	t.Helper()
	dir, err := Home(At(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	if err = makeDurably(dir.path); err != nil {
		t.Fatal(err)
	}
	if holder, err = openLock(dir, name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = holder.Close() }) //nolint:errcheck // the test is over
	if taken, lockErr := tryLock(holder); lockErr != nil || !taken {
		t.Fatalf("take the lock as another process would = %v, %v", taken, lockErr)
	}
	if waiter, err = openLock(dir, name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = waiter.Close() }) //nolint:errcheck // the test is over
	return waiter, holder
}

// A waiter looks at a lock another process holds often enough that it is taken
// within 50ms of coming free however long the wait was before, and gives
// up when the wait it was given runs out, not a pause later. The clock is the
// bubble's, so the times are exact. The pauses are jittered, so the lock is let
// go after a long wait several times over: a waiter that paused for longer than
// 50ms would be late for nearly every one.
func TestAWaiterLooksAtAnotherProcesssLockOftenEnoughAndNoLongerThanItWasGiven(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		waiter, holder := heldElsewhere(t, "x.lock")
		const wait = 3 * time.Second
		began := time.Now()
		if err := waitFile(t.Context(), waiter, began.Add(wait), "x.lock"); !errors.Is(err, ErrLocked) {
			t.Fatalf("waitFile of a lock held throughout = %v, want ErrLocked", err)
		}
		if waited := time.Since(began); waited != wait {
			t.Errorf("waitFile gave up after %v, want exactly the %v it was given", waited, wait)
		}

		const heldFor = 10 * time.Second
		for round := range 5 {
			go func() {
				time.Sleep(heldFor)
				if err := unlock(holder); err != nil {
					t.Errorf("let the lock go: %v", err)
				}
			}()
			began = time.Now()
			if err := waitFile(t.Context(), waiter, began.Add(2*heldFor), "x.lock"); err != nil {
				t.Fatalf("round %d: waitFile of a lock let go after %v = %v, want the lock", round, heldFor, err)
			}
			if late := time.Since(began) - heldFor; late > 50*time.Millisecond {
				t.Errorf("round %d: the lock was taken %v after it came free, want within 50ms", round, late)
			}
			// The waiter lets it go, and the holder takes it again for the next round.
			if err := unlock(waiter); err != nil {
				t.Fatal(err)
			}
			if taken, err := tryLock(holder); err != nil || !taken {
				t.Fatalf("take the lock again as another process would = %v, %v", taken, err)
			}
		}
	})
}

// A wait for a lock another process holds ends when its context is done, and
// not when the wait it was given runs out.
func TestAWaitForAnotherProcesssLockEndsWithItsContext(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		waiter, _ := heldElsewhere(t, "x.lock")
		ctx, cancel := context.WithCancel(t.Context())
		const cancelAt = 300 * time.Millisecond
		time.AfterFunc(cancelAt, cancel)
		began := time.Now()
		err := waitFile(ctx, waiter, began.Add(time.Minute), "x.lock")
		if !errors.Is(err, context.Canceled) || errors.Is(err, ErrLocked) {
			t.Errorf("waitFile with its context cancelled = %v, want the context's error and not ErrLocked", err)
		}
		if waited := time.Since(began); waited != cancelAt {
			t.Errorf("waitFile ended %v after it began, want at the cancellation, %v", waited, cancelAt)
		}
	})
}
