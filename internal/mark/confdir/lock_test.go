package confdir_test

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/koopa0/yomihon/internal/mark/confdir"
)

// A test that needs another process runs this test binary again in a role,
// named by these variables: a holder takes the lock and keeps it until it is
// killed, and a writer adds one line to a file under the lock each time it is
// let go.
const (
	roleVar   = "YOMIHON_TEST_LOCK_ROLE"
	configVar = "YOMIHON_TEST_LOCK_CONFIG"
	writerVar = "YOMIHON_TEST_LOCK_WRITER"

	lockName  = "lines.lock"
	linesName = "lines.txt"
)

func TestMain(m *testing.M) {
	switch role := os.Getenv(roleVar); role {
	case "":
		os.Exit(m.Run())
	case "hold":
		os.Exit(hold())
	case "write":
		os.Exit(write())
	default:
		fmt.Fprintf(os.Stderr, "no role %q\n", role)
		os.Exit(2)
	}
}

// hold takes the lock, says so and keeps it: its standard input never ends, so
// the only way out is being killed.
func hold() int {
	dir, err := confdir.Home(confdir.At(os.Getenv(configVar)))
	if err == nil {
		err = confdir.Exclusive(context.Background(), dir, lockName, 0, func() error {
			if readyErr := ready(); readyErr != nil {
				return readyErr
			}
			_, _ = io.Copy(io.Discard, os.Stdin) //nolint:errcheck // what matters is that it returns only when the parent lets go
			return errors.New("the holder was let go instead of killed")
		})
	}
	fmt.Fprintln(os.Stderr, err)
	return 1
}

// write says it is ready and waits for a line on standard input; each line lets
// it add one line of its own to the file under the lock, and the end of input
// lets it go.
func write() int {
	dir, err := confdir.Home(confdir.At(os.Getenv(configVar)))
	in := bufio.NewReader(os.Stdin)
	for err == nil {
		if err = ready(); err != nil {
			break
		}
		if _, ended := in.ReadString('\n'); ended != nil {
			return 0
		}
		err = confdir.Exclusive(context.Background(), dir, lockName, confdir.LockWait, func() error {
			held, readErr := confdir.Read(dir, linesName, 1<<20)
			if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
				return readErr
			}
			return confdir.Replace(dir, linesName, append(held, "writer "+os.Getenv(writerVar)+"\n"...))
		})
	}
	fmt.Fprintln(os.Stderr, err)
	return 1
}

// ready tells the test that started this process that it may go on.
func ready() error {
	_, err := os.Stdout.WriteString("ready\n")
	return err
}

// process is this test binary running in a role.
type process struct {
	role string
	cmd  *exec.Cmd
	in   io.WriteCloser
	out  *bufio.Reader
}

// started is this test binary run again in role over config, once it has said
// it is ready.
func started(t *testing.T, config, role string, env ...string) *process {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), os.Args[0]) // #nosec G204 G702 -- this test binary, run again in a role
	cmd.Env = append(os.Environ(), append([]string{roleVar + "=" + role, configVar + "=" + config}, env...)...)
	cmd.Stderr = os.Stderr
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	p := &process{role: role, cmd: cmd, in: in, out: bufio.NewReader(out)}
	p.ready(t)
	return p
}

// ready waits for the process to say it is ready again.
func (p *process) ready(t *testing.T) {
	t.Helper()
	if line, err := p.out.ReadString('\n'); err != nil || line != "ready\n" {
		_ = p.cmd.Process.Kill() //nolint:errcheck // the failure below is the report
		t.Fatalf("the %s process said %q, %v; want ready", p.role, line, err)
	}
}

// TestWritersInSeparateProcessesLoseNothing is the command line and the running
// server writing one file at the same moment, each round just after another
// process died holding the lock. Each writer reads the file, adds a line and
// replaces it, so two inside the lock at once write one line away.
func TestWritersInSeparateProcessesLoseNothing(t *testing.T) {
	t.Parallel()
	const rounds, writers = 8, 6
	config := t.TempDir()
	var each []*process
	for w := range writers {
		each = append(each, started(t, config, "write", writerVar+"="+strconv.Itoa(w)))
	}
	for range rounds {
		holder := started(t, config, "hold")
		if err := holder.cmd.Process.Kill(); err != nil {
			t.Fatal(err)
		}
		_ = holder.cmd.Wait() //nolint:errcheck // killed, so it reports the signal
		for _, writer := range each {
			if _, err := io.WriteString(writer.in, "\n"); err != nil {
				t.Fatal(err)
			}
		}
		for _, writer := range each {
			writer.ready(t)
		}
	}
	for _, writer := range each {
		if err := writer.in.Close(); err != nil {
			t.Fatal(err)
		}
		if err := writer.cmd.Wait(); err != nil {
			t.Errorf("a writer failed: %v", err)
		}
	}
	dir, err := confdir.Home(confdir.At(config))
	if err != nil {
		t.Fatal(err)
	}
	held, err := confdir.Read(dir, linesName, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if got := bytes.Count(held, []byte("\n")); got != rounds*writers {
		t.Errorf("%d of %d writes were lost: two writers were inside the lock at once after its holder died", rounds*writers-got, rounds*writers)
	}
}

// TestAHolderThatDiesReleasesTheLock kills a process that holds the lock and
// takes the lock before the killed process is reaped. Its process id still
// answers then, as it does when the id has been given to another process, so
// only the lock itself can say that its holder is gone.
func TestAHolderThatDiesReleasesTheLock(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	dir, err := confdir.Home(confdir.At(config))
	if err != nil {
		t.Fatal(err)
	}
	holder := started(t, config, "hold")
	t.Cleanup(func() { _ = holder.cmd.Wait() }) //nolint:errcheck // killed, so it reports the signal
	if err := confdir.Exclusive(t.Context(), dir, lockName, 0, func() error { return errors.New("entered a lock another process holds") }); !errors.Is(err, confdir.ErrLocked) {
		t.Fatalf("Exclusive while another process holds the lock = %v, want ErrLocked", err)
	}
	if err := holder.cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	begun := time.Now()
	entered := false
	if err := confdir.Exclusive(t.Context(), dir, lockName, confdir.LockWait, func() error { entered = true; return nil }); err != nil || !entered {
		t.Fatalf("Exclusive after the holder was killed = %v, entered %v; want the lock, at once", err, entered)
	}
	t.Logf("the lock was taken %v after its holder was killed", time.Since(begun))
}

func TestALockIsExclusiveUntilItIsReleased(t *testing.T) {
	t.Parallel()
	dir := homeIn(t)

	first, err := confdir.Acquire(t.Context(), dir, "list.lock", 0)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if _, refused := confdir.Acquire(t.Context(), dir, "list.lock", 0); !errors.Is(refused, confdir.ErrLocked) {
		t.Fatalf("a second Acquire while the first holds it = %v, want ErrLocked", refused)
	}
	if err = first.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
	// A waiter may have the file open already; removing it would leave that
	// waiter locking a file no longer at the name the next claimant opens.
	if _, statErr := os.Stat(dir.Join("list.lock")); statErr != nil {
		t.Errorf("a released lock took its file away: %v", statErr)
	}
	second, err := confdir.Acquire(t.Context(), dir, "list.lock", 0)
	if err != nil {
		t.Fatalf("Acquire after Release: %v", err)
	}
	if err = second.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
}

// A claim of this process that waits behind another of this process's stops
// waiting when its context is cancelled, and not when the wait it was given runs
// out.
func TestACancelledContextEndsTheWaitForTheLockAtOnce(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		dir := homeIn(t)
		held, err := confdir.Acquire(t.Context(), dir, "list.lock", 0)
		if err != nil {
			t.Fatalf("Acquire: %v", err)
		}
		ctx, cancel := context.WithCancel(t.Context())
		const cancelAt = 300 * time.Millisecond
		time.AfterFunc(cancelAt, cancel)
		began := time.Now()
		lock, err := confdir.Acquire(ctx, dir, "list.lock", time.Minute)
		if err == nil {
			_ = lock.Release() //nolint:errcheck // the failure below is the report
		}
		if !errors.Is(err, context.Canceled) || errors.Is(err, confdir.ErrLocked) {
			t.Errorf("Acquire with its context cancelled = %v, want the context's error and not ErrLocked", err)
		}
		if waited := time.Since(began); waited != cancelAt {
			t.Errorf("Acquire ended %v after it began, want at the cancellation, %v", waited, cancelAt)
		}
		if err := held.Release(); err != nil {
			t.Fatalf("Release: %v", err)
		}
	})
}

// A context that is done stops a wait, never the taking of a lock that is free:
// a change part way through its steps goes on to the next.
func TestAFreeLockIsTakenWhateverTheContextSays(t *testing.T) {
	t.Parallel()
	dir := homeIn(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	lock, err := confdir.Acquire(ctx, dir, "list.lock", time.Minute)
	if err != nil {
		t.Fatalf("Acquire of a free lock with a cancelled context = %v, want the lock", err)
	}
	if err := lock.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
}

// A claim of this process waiting behind another of its own is let through the
// moment that one lets the lock go, and gives up when the wait it was given
// runs out, not a moment later. The clock is the bubble's, so the times are
// exact.
func TestAClaimIsLetThroughTheMomentTheLockIsLetGoAndWaitsNoLongerThanItWasGiven(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		dir := homeIn(t)
		held, err := confdir.Acquire(t.Context(), dir, "list.lock", 0)
		if err != nil {
			t.Fatalf("Acquire: %v", err)
		}
		const wait = 3 * time.Second
		began := time.Now()
		if _, err = confdir.Acquire(t.Context(), dir, "list.lock", wait); !errors.Is(err, confdir.ErrLocked) {
			t.Fatalf("Acquire of a lock held throughout = %v, want ErrLocked", err)
		}
		if waited := time.Since(began); waited != wait {
			t.Errorf("Acquire gave up after %v, want exactly the %v it was given", waited, wait)
		}

		const heldFor = 2 * time.Second
		go func() {
			time.Sleep(heldFor)
			if releaseErr := held.Release(); releaseErr != nil {
				t.Errorf("Release: %v", releaseErr)
			}
		}()
		began = time.Now()
		lock, err := confdir.Acquire(t.Context(), dir, "list.lock", wait)
		if err != nil {
			t.Fatalf("Acquire of a lock let go after %v = %v, want the lock", heldFor, err)
		}
		if late := time.Since(began) - heldFor; late != 0 {
			t.Errorf("the lock was taken %v after it was let go, want at once", late)
		}
		if err := lock.Release(); err != nil {
			t.Fatalf("Release: %v", err)
		}
	})
}

// The claims of one process are let through in the order they were made: one
// that has waited longest is next, and none of them is passed over by one that
// came after, however many there are.
func TestTheClaimsOfOneProcessAreLetThroughInTheOrderTheyWereMade(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		dir := homeIn(t)
		held, err := confdir.Acquire(t.Context(), dir, "list.lock", 0)
		if err != nil {
			t.Fatalf("Acquire: %v", err)
		}
		const claims = 8
		var order []int
		var mu sync.Mutex
		var all sync.WaitGroup
		for i := range claims {
			all.Go(func() {
				time.Sleep(time.Duration(i+1) * time.Millisecond)
				lock, err := confdir.Acquire(t.Context(), dir, "list.lock", time.Minute)
				if err != nil {
					t.Errorf("claim %d: Acquire: %v", i, err)
					return
				}
				mu.Lock()
				order = append(order, i)
				mu.Unlock()
				time.Sleep(time.Millisecond) // the change made under the lock
				if err := lock.Release(); err != nil {
					t.Errorf("claim %d: Release: %v", i, err)
				}
			})
		}
		time.Sleep(time.Second)
		if err := held.Release(); err != nil {
			t.Fatalf("Release: %v", err)
		}
		all.Wait()
		if want := []int{0, 1, 2, 3, 4, 5, 6, 7}; !slices.Equal(order, want) {
			t.Errorf("the claims were let through in the order %v, want %v", order, want)
		}
	})
}

// A claim whose lock file cannot be opened, here because a folder stands at its
// name, gives its place in this process's queue back: once the name is free, a
// claim that waits for nothing gets the lock.
func TestAClaimWhoseLockCannotBeOpenedGivesItsTurnBack(t *testing.T) {
	t.Parallel()
	dir := homeIn(t)
	if err := os.Mkdir(dir.Join("list.lock"), 0o700); err != nil {
		t.Fatal(err)
	}
	if lock, err := confdir.Acquire(t.Context(), dir, "list.lock", 0); err == nil {
		_ = lock.Release() //nolint:errcheck // the failure below is the report
		t.Fatal("Acquire opened a folder as its lock file")
	}
	if err := os.Remove(dir.Join("list.lock")); err != nil {
		t.Fatal(err)
	}
	lock, err := confdir.Acquire(t.Context(), dir, "list.lock", 0)
	if err != nil {
		t.Fatalf("Acquire after a claim that could not open the lock = %v, want the lock at once", err)
	}
	if err := lock.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
}

// TestExclusiveKeepsEveryChangeOfWritersOnOneFile is the lost update: each
// writer reads the file, adds a line and replaces it, and one that read before
// another replaced writes the other's line away.
func TestExclusiveKeepsEveryChangeOfWritersOnOneFile(t *testing.T) {
	t.Parallel()
	dir := homeIn(t)
	const writers, each = 4, 40
	var group sync.WaitGroup
	for w := range writers {
		group.Go(func() {
			for i := range each {
				err := confdir.Exclusive(t.Context(), dir, "x.lock", 30*time.Second, func() error {
					held, err := confdir.Read(dir, "x.txt", 1<<20)
					if err != nil && !os.IsNotExist(err) {
						return err
					}
					return confdir.Replace(dir, "x.txt", append(held, []byte(strconv.Itoa(w)+"-"+strconv.Itoa(i)+"\n")...))
				})
				if err != nil {
					t.Error(err)
					return
				}
			}
		})
	}
	group.Wait()
	held, err := confdir.Read(dir, "x.txt", 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if lines := bytes.Count(held, []byte("\n")); lines != writers*each {
		t.Errorf("the file holds %d lines, want %d", lines, writers*each)
	}
}

// A link at the lock's name is not followed: the file would be made wherever it
// points, which can be inside a vault.
func TestALockDoesNotFollowALinkOutOfItsDirectory(t *testing.T) {
	t.Parallel()
	dir := homeIn(t)
	outside := filepath.Join(t.TempDir(), "made.lock")
	if err := os.Symlink(outside, dir.Join("list.lock")); err != nil {
		t.Skipf("this platform or user cannot make a link: %v", err)
	}
	if lock, err := confdir.Acquire(t.Context(), dir, "list.lock", 0); err == nil {
		_ = lock.Release() //nolint:errcheck // the failure below is the report
		t.Fatal("Acquire followed a link out of its directory")
	}
	if _, err := os.Lstat(outside); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Acquire made %s through a link: %v", outside, err)
	}
}
