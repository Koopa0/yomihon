package confdir

import (
	"os"
	"os/exec"
	"strconv"
	"sync"
	"testing"
	"time"
)

// deadPID is the id of a process that has ended.
func deadPID(t *testing.T) int {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^$") // #nosec G204 G702 -- this test binary, run again so that it ends at once
	if err := cmd.Run(); err != nil {
		t.Fatalf("run a process that ends at once: %v", err)
	}
	return cmd.ProcessState.Pid()
}

func TestProcessAliveKnowsThisProcessAndOneThatEnded(t *testing.T) {
	t.Parallel()
	if !ProcessAlive(os.Getpid()) {
		t.Error("this process was called gone")
	}
	if ProcessAlive(deadPID(t)) {
		t.Error("a process that has ended was called alive")
	}
	for _, pid := range []int{0, -1} {
		if ProcessAlive(pid) {
			t.Errorf("ProcessAlive(%d) = true", pid)
		}
	}
}

func TestALockLeftByAProcessThatIsGoneIsTakenAtOnce(t *testing.T) {
	t.Parallel()
	dir := Dir{path: t.TempDir()}
	if err := Replace(dir, "x.lock", []byte(strconv.Itoa(deadPID(t))+" 1 1")); err != nil {
		t.Fatal(err)
	}
	begun := time.Now()
	if err := Exclusive(dir, "x.lock", 5*time.Second, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if took := time.Since(begun); took > 2*time.Second {
		t.Fatalf("a lock left by a process that is gone took %v to be taken, want at once", took)
	}
}

func TestALockOlderThanAnyChangeTakesIsTakenWhoeverHoldsIt(t *testing.T) {
	t.Parallel()
	dir := Dir{path: t.TempDir()}
	path := dir.Join("x.lock")
	// This very process is alive, so only the lock's age can have it taken: the
	// case of a process id given to another process after the holder's crash.
	if err := Replace(dir, "x.lock", []byte(strconv.Itoa(os.Getpid())+" 1 1")); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * lockStaleAfter)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	if err := Exclusive(dir, "x.lock", 0, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
}

func TestALockNotYetWrittenIsNotCalledAbandoned(t *testing.T) {
	t.Parallel()
	if abandoned(nil, time.Millisecond) {
		t.Error("a lock file just made and still empty was called abandoned")
	}
	if abandoned([]byte("not a number 1"), time.Second) {
		t.Error("a holder this did not write was called abandoned before it was old")
	}
	if !abandoned([]byte("not a number 1"), 2*lockStaleAfter) {
		t.Error("a holder nothing can place was not called abandoned once it was old")
	}
}

// TestExclusiveKeepsEveryChangeOfWritersOnOneFile is the lost update: each
// writer reads the file, adds a line and replaces it, and one that read before
// another replaced writes the other's line away.
func TestExclusiveKeepsEveryChangeOfWritersOnOneFile(t *testing.T) {
	t.Parallel()
	dir := Dir{path: t.TempDir()}
	const writers, each = 4, 40
	var group sync.WaitGroup
	for w := range writers {
		group.Go(func() {
			for i := range each {
				err := Exclusive(dir, "x.lock", 30*time.Second, func() error {
					held, err := Read(dir, "x.txt", 1<<20)
					if err != nil && !os.IsNotExist(err) {
						return err
					}
					return Replace(dir, "x.txt", append(held, []byte(strconv.Itoa(w)+"-"+strconv.Itoa(i)+"\n")...))
				})
				if err != nil {
					t.Error(err)
					return
				}
			}
		})
	}
	group.Wait()
	held, err := Read(dir, "x.txt", 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	lines := 0
	for _, b := range held {
		if b == '\n' {
			lines++
		}
	}
	if lines != writers*each {
		t.Errorf("the file holds %d lines, want %d", lines, writers*each)
	}
}
