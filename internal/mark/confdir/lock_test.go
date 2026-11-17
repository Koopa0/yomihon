package confdir_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/mark/confdir"
)

func TestALockIsExclusiveUntilItIsReleased(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	first, err := confdir.Acquire(dir, "list.lock", []byte("first"), nil, 0)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if _, refused := confdir.Acquire(dir, "list.lock", []byte("second"), nil, 0); !errors.Is(refused, confdir.ErrLocked) {
		t.Fatalf("a second Acquire while the first holds it = %v, want ErrLocked", refused)
	}
	if err = first.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
	second, err := confdir.Acquire(dir, "list.lock", []byte("second"), nil, 0)
	if err != nil {
		t.Fatalf("Acquire after Release: %v", err)
	}
	if err = second.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "list.lock")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a released lock left its file behind: %v", err)
	}
}

func TestAWaiterGetsTheLockWhenItsHolderReleasesInTime(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	held, err := confdir.Acquire(dir, "list.lock", []byte("holder"), nil, 0)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = held.Release() //nolint:errcheck // a failure surfaces as the waiter timing out below
	}()
	waiter, err := confdir.Acquire(dir, "list.lock", []byte("waiter"), nil, 5*time.Second)
	if err != nil {
		t.Fatalf("a waiter was not given the lock after its holder released it: %v", err)
	}
	if err := waiter.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
}

func TestOnlyOneHolderIsInsideAtATime(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	counter := filepath.Join(dir, "counter")
	if err := os.WriteFile(counter, []byte("0"), 0o600); err != nil {
		t.Fatal(err)
	}

	const workers, rounds = 8, 10
	var wg sync.WaitGroup
	for worker := range workers {
		wg.Go(func() {
			for round := range rounds {
				holder := []byte(strconv.Itoa(worker) + "-" + strconv.Itoa(round))
				lock, err := confdir.Acquire(dir, "list.lock", holder, nil, 30*time.Second)
				if err != nil {
					t.Errorf("worker %d round %d: Acquire: %v", worker, round, err)
					return
				}
				data, err := os.ReadFile(counter) // #nosec G304 -- a file of this test's own temporary directory
				if err != nil {
					t.Errorf("read the counter: %v", err)
				}
				n, _ := strconv.Atoi(string(data)) //nolint:errcheck // a damaged counter reads as zero and fails the total below
				if err := os.WriteFile(counter, []byte(strconv.Itoa(n+1)), 0o600); err != nil {
					t.Errorf("write the counter: %v", err)
				}
				if err := lock.Release(); err != nil {
					t.Errorf("Release: %v", err)
				}
			}
		})
	}
	wg.Wait()

	data, err := os.ReadFile(counter) // #nosec G304 -- a file of this test's own temporary directory
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), strconv.Itoa(workers*rounds); got != want {
		t.Fatalf("the counter reads %s after %d read-modify-writes under the lock, want %s: two holders were inside at once", got, workers*rounds, want)
	}
}

func TestALockCalledAbandonedIsTakenAndOneNotCalledSoIsNot(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "list.lock")
	leftBehind := []byte("a process that stopped")
	if err := os.WriteFile(path, leftBehind, 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}

	var sawHolder []byte
	var sawAge time.Duration
	// A holder that is still there is not taken from, however old the file.
	if _, err := confdir.Acquire(dir, "list.lock", []byte("new"), func([]byte, time.Duration) bool { return false }, 0); !errors.Is(err, confdir.ErrLocked) {
		t.Fatalf("a lock its predicate did not call abandoned was taken: %v", err)
	}
	lock, err := confdir.Acquire(dir, "list.lock", []byte("new"), func(holder []byte, age time.Duration) bool {
		sawHolder, sawAge = holder, age
		return true
	}, 0)
	if err != nil {
		t.Fatalf("a lock called abandoned was not taken: %v", err)
	}
	if !bytes.Equal(sawHolder, leftBehind) {
		t.Errorf("the predicate was given %q, want what the file held, %q", sawHolder, leftBehind)
	}
	if sawAge < 59*time.Minute {
		t.Errorf("the predicate was given an age of %v for a file an hour old", sawAge)
	}
	if err := lock.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
}

func TestReleaseLeavesAnotherHoldersLockWhereItIs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "list.lock")
	slow, err := confdir.Acquire(dir, "list.lock", []byte("slow"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	// Judged abandoned while its holder was only slow, and taken by someone else.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("another"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := slow.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "another" { // #nosec G304 -- a file of this test's own temporary directory
		t.Fatalf("Release removed a lock that was no longer its own: %q, %v", data, err)
	}
}

func TestALockRefusesAnythingButAFileName(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"", ".", "..", "../x", "a/b"} {
		if _, err := confdir.Acquire(t.TempDir(), name, nil, nil, 0); err == nil {
			t.Errorf("Acquire accepted %q as a lock name", name)
		}
	}
}
