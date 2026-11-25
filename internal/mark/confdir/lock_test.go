package confdir_test

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
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
		err = confdir.Exclusive(dir, lockName, 0, func() error {
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
		err = confdir.Exclusive(dir, lockName, confdir.LockWait, func() error {
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
	const rounds, writers = 30, 6
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
	if err := confdir.Exclusive(dir, lockName, 0, func() error { return errors.New("entered a lock another process holds") }); !errors.Is(err, confdir.ErrLocked) {
		t.Fatalf("Exclusive while another process holds the lock = %v, want ErrLocked", err)
	}
	if err := holder.cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	begun := time.Now()
	entered := false
	if err := confdir.Exclusive(dir, lockName, confdir.LockWait, func() error { entered = true; return nil }); err != nil || !entered {
		t.Fatalf("Exclusive after the holder was killed = %v, entered %v; want the lock, at once", err, entered)
	}
	t.Logf("the lock was taken %v after its holder was killed", time.Since(begun))
}

func TestALockIsExclusiveUntilItIsReleased(t *testing.T) {
	t.Parallel()
	dir := homeIn(t)

	first, err := confdir.Acquire(dir, "list.lock", 0)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if _, refused := confdir.Acquire(dir, "list.lock", 0); !errors.Is(refused, confdir.ErrLocked) {
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
	second, err := confdir.Acquire(dir, "list.lock", 0)
	if err != nil {
		t.Fatalf("Acquire after Release: %v", err)
	}
	if err = second.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
}

func TestAWaiterGetsTheLockWhenItsHolderReleasesInTime(t *testing.T) {
	t.Parallel()
	dir := homeIn(t)
	held, err := confdir.Acquire(dir, "list.lock", 0)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = held.Release() //nolint:errcheck // a failure surfaces as the waiter timing out below
	}()
	waiter, err := confdir.Acquire(dir, "list.lock", 5*time.Second)
	if err != nil {
		t.Fatalf("a waiter was not given the lock after its holder released it: %v", err)
	}
	if err := waiter.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
}

func TestOnlyOneHolderIsInsideAtATime(t *testing.T) {
	t.Parallel()
	dir := homeIn(t)
	counter := dir.Join("counter")
	if err := os.WriteFile(counter, []byte("0"), 0o600); err != nil {
		t.Fatal(err)
	}

	const workers, rounds = 8, 10
	var wg sync.WaitGroup
	for worker := range workers {
		wg.Go(func() {
			for round := range rounds {
				lock, err := confdir.Acquire(dir, "list.lock", 30*time.Second)
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
				err := confdir.Exclusive(dir, "x.lock", 30*time.Second, func() error {
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
	if lock, err := confdir.Acquire(dir, "list.lock", 0); err == nil {
		_ = lock.Release() //nolint:errcheck // the failure below is the report
		t.Fatal("Acquire followed a link out of its directory")
	}
	if _, err := os.Lstat(outside); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Acquire made %s through a link: %v", outside, err)
	}
}

func TestALockRefusesAnythingButAFileName(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"", ".", "..", "../x", "a/b"} {
		if _, err := confdir.Acquire(homeIn(t), name, 0); err == nil {
			t.Errorf("Acquire accepted %q as a lock name", name)
		}
	}
}
