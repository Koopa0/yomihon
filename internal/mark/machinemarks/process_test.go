package machinemarks

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/mark/confdir"
	"github.com/koopa0/yomihon/internal/mark/sealedline"
	"github.com/koopa0/yomihon/internal/vault"
)

// A test that needs a second process, or one that dies, runs this test binary
// again as one. These variables say which, over which vault and configuration
// directory.
const (
	childModeVar   = "YOMIHON_TEST_MACHINEMARKS_MODE"
	childRootVar   = "YOMIHON_TEST_MACHINEMARKS_ROOT"
	childConfigVar = "YOMIHON_TEST_MACHINEMARKS_CONFIG"
	childWhoVar    = "YOMIHON_TEST_MACHINEMARKS_WHO"
)

func TestMain(m *testing.M) {
	if mode := os.Getenv(childModeVar); mode != "" {
		if err := runChild(mode, os.Getenv(childRootVar), os.Getenv(childConfigVar), os.Getenv(childWhoVar)); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

const (
	// togetherMarks is how many marks of its own each of two writers makes, and
	// every fifth of its writes is an edit of the one mark they share.
	togetherMarks = 60
	// killMarks and killBody make a file long enough that a kill lands inside
	// the write of it: each of killMarks marks holds killBody bytes of words.
	killMarks = 120
	killBody  = 8 << 10
)

// open is a Marks over a vault and a configuration directory, as a process of
// its own opens one.
func openMarks(root, conf string) (*Marks, func(), error) {
	r, err := vault.Open(root)
	if err != nil {
		return nil, nil, err
	}
	dir, err := confdir.VaultDir(confdir.At(conf), root)
	if err != nil {
		return nil, nil, errors.Join(err, r.Close())
	}
	m, err := Open(r, dir, slog.New(slog.DiscardHandler), nil)
	if err != nil {
		return nil, nil, errors.Join(err, r.Close())
	}
	return m, func() { _ = m.Close(); _ = r.Close() }, nil //nolint:errcheck // the process is ending
}

// killBodyOf is what the i-th write of a killed writer says, which the test can
// make again: a token that goes up with i, and padding.
func killBodyOf(i int) string {
	return fmt.Sprintf("%09d", i) + strings.Repeat("x", killBody-9)
}

func killMark(i int) Mark {
	m := thought(100+i%killMarks, killBodyOf(i))
	m.At = time.Date(2026, 1, 1, 0, 0, i%killMarks, 0, time.UTC)
	return m
}

// runChild is the second process.
func runChild(mode, root, conf, who string) error {
	m, closeAll, err := openMarks(root, conf)
	if err != nil {
		return err
	}
	defer closeAll()
	out := bufio.NewWriter(os.Stdout)
	say := func(format string, args ...any) error {
		if _, writeErr := fmt.Fprintf(out, format+"\n", args...); writeErr != nil {
			return writeErr
		}
		return out.Flush()
	}
	switch mode {
	case "together":
		return together(m, who)
	case "kill":
		start, convErr := strconv.Atoi(who)
		if convErr != nil {
			return convErr
		}
		for i := start; ; i++ {
			// The window of the kill is between "begin" and "done": a Put is
			// asked for, and has not answered.
			if err := say("begin %d", i); err != nil {
				return err
			}
			mark := killMark(i)
			var base Seen
			view, viewErr := m.View()
			if viewErr != nil {
				return viewErr
			}
			if e, ok := view.Entry(mark.ID, open); ok {
				base = e.Seen
			}
			if _, putErr := crowded(func() (Result, error) { return m.Put(context.Background(), &mark, base, nil) }); putErr != nil {
				return putErr
			}
			if err := say("done %d", i); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("no mode %q", mode)
	}
}

// together writes marks of its own, and edits one it shares with the other
// writer, retrying when the page it read from was stale.
func together(m *Marks, who string) error {
	base := 1000
	if who == "two" {
		base = 2000
	}
	for i := range togetherMarks {
		mark := thought(base+i, who+" "+strconv.Itoa(i))
		if _, err := crowded(func() (Result, error) { return m.Put(context.Background(), &mark, nil, nil) }); err != nil {
			return err
		}
		if i%5 != 0 {
			continue
		}
		for attempt := 0; ; attempt++ {
			shared := thought(1, who+" edit "+strconv.Itoa(i))
			var seen Seen
			view, err := m.View()
			if err != nil {
				return err
			}
			if e, ok := view.Entry(shared.ID, open); ok {
				seen = e.Seen
			}
			_, err = crowded(func() (Result, error) { return m.Put(context.Background(), &shared, seen, nil) })
			if err == nil {
				break
			}
			if !errors.Is(err, ErrStale) && !errors.Is(err, ErrNoBase) || attempt > 50 {
				return err
			}
		}
	}
	return nil
}

type child struct {
	cmd   *exec.Cmd
	lines chan string
}

func startChild(t *testing.T, mode, root, conf, who string) *child {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^$") // #nosec G204 G702 -- this test binary, run again as the writer
	cmd.Env = append(os.Environ(), childModeVar+"="+mode, childRootVar+"="+root, childConfigVar+"="+conf, childWhoVar+"="+who)
	cmd.Stderr = os.Stderr
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	c := &child{cmd: cmd, lines: make(chan string, 64)}
	go func() {
		defer close(c.lines)
		scanner := bufio.NewScanner(out)
		for scanner.Scan() {
			c.lines <- scanner.Text()
		}
		_, _ = io.Copy(io.Discard, out) //nolint:errcheck // what is left after a kill says nothing
	}()
	return c
}

// Two processes of one machine, as the server and a command are, write marks of
// their own and edit one they share, again and again, with compactions of the
// ledger among them: not one write is lost, no one is told to be another
// machine, and the shared mark has one value.
func TestTwoProcessesWriteAtOnceAndLoseNothing(t *testing.T) {
	t.Parallel()
	a := newClone(t, "A")
	a.say(1, "the shared mark")
	var cmds []*exec.Cmd
	for _, who := range []string{"one", "two"} {
		cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^$") // #nosec G204 G702 -- this test binary, run again as a writer
		cmd.Env = append(os.Environ(), childModeVar+"=together", childRootVar+"="+a.root, childConfigVar+"="+a.conf, childWhoVar+"="+who)
		cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		cmds = append(cmds, cmd)
	}
	for _, cmd := range cmds {
		if err := cmd.Wait(); err != nil {
			t.Fatalf("a writer failed: %v", err)
		}
	}
	a.restart()
	v := a.view()
	if v.Damaged() != 0 || v.DamagedOwn() != 0 || v.Missing() != 0 {
		t.Fatalf("%d damaged, %d own damaged, %d missing", v.Damaged(), v.DamagedOwn(), v.Missing())
	}
	if bytes.Contains(a.ledgerBytes(), []byte(`"kind":"retire"`)) {
		t.Fatal("a process of this machine was taken for another machine")
	}
	for i := range togetherMarks {
		for who, base := range map[string]int{"one": 1000, "two": 2000} {
			if got, want := a.words(base+i), who+" "+strconv.Itoa(i); got != want {
				t.Fatalf("mark %d reads %q, want %q", base+i, got, want)
			}
		}
	}
	if e := a.entry(mid(1)); len(e.Values) > 1 {
		t.Fatalf("the shared mark is a conflict of %d values after writers that each read the last", len(e.Values))
	}
	if got := len(v.Entries(open)); got != 1+2*togetherMarks {
		t.Fatalf("%d marks, want %d", got, 1+2*togetherMarks)
	}
	if n := len(sealedline.Lines(readFile(t, filepath.Join(a.marksDir(), a.id()+".jsonl")))); n != 1+2*togetherMarks {
		t.Fatalf("the file holds %d lines, want %d: one a mark", n, 1+2*togetherMarks)
	}
}

// Goroutines of one process, and several Marks of one vault, write at once under
// the race detector.
func TestGoroutinesWriteAtOnce(t *testing.T) {
	t.Parallel()
	a := newClone(t, "A")
	a.say(1, "the shared mark")
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for g := range 4 {
		wg.Go(func() {
			m := a.m
			if g%2 == 1 {
				other, closeAll, err := openMarks(a.root, a.conf)
				if err != nil {
					errs <- err
					return
				}
				defer closeAll()
				m = other
			}
			for i := range 15 {
				mark := thought(10+g*100+i, "g")
				if _, err := crowded(func() (Result, error) { return m.Put(t.Context(), &mark, nil, nil) }); err != nil {
					errs <- err
					return
				}
				if _, err := m.View(); err != nil {
					errs <- err
					return
				}
			}
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	if got := len(a.view().Entries(open)); got != 1+4*15 {
		t.Fatalf("%d marks, want %d", got, 1+4*15)
	}
	if v := a.view(); v.Missing() != 0 || v.DamagedOwn() != 0 {
		t.Fatalf("%d missing, %d damaged", v.Missing(), v.DamagedOwn())
	}
}

// answered is what the child said a Put answered for.
func (c *child) killAfter(t *testing.T, begins int, until func() bool, wait time.Duration) []int {
	t.Helper()
	var answered []int
	read := func(line string) {
		if rest, ok := strings.CutPrefix(line, "done "); ok {
			n, err := strconv.Atoi(rest)
			if err != nil {
				t.Fatalf("the child said %q", line)
			}
			answered = append(answered, n)
		}
	}
	for begun := 0; begun < begins; {
		line, ok := <-c.lines
		if !ok {
			t.Fatal("the child stopped before it was killed")
		}
		if strings.HasPrefix(line, "begin ") {
			begun++
		}
		read(line)
	}
	for deadline := time.Now().Add(time.Minute); until != nil && !until(); {
		if time.Now().After(deadline) {
			t.Fatal("what the kill waits for did not happen in a minute")
		}
		select {
		case line, ok := <-c.lines:
			if !ok {
				t.Fatal("the child stopped before it was killed")
			}
			read(line)
		case <-time.After(50 * time.Microsecond):
		}
	}
	time.Sleep(wait)
	if err := c.cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = c.cmd.Wait() //nolint:errcheck // a killed process answers that it was killed
	for line := range c.lines {
		read(line)
	}
	return answered
}

// A writer killed at any moment of a write loses nothing it answered for. It is
// killed again and again, some of the time between the line it records and the
// file it installs: what the vault holds is then the file before, whole, the
// view reads the line from the ledger and counts it missing, and the next write
// by another process puts it back. The kills go on until one has landed in that
// window, at least twelve, up to three hundred.
func TestAWriterKilledAtAnyMomentLosesNothingItAnsweredFor(t *testing.T) {
	t.Parallel()
	a := newClone(t, "A")
	for i := range killMarks { // the file the child rewrites is made long, once
		a.put(killMark(i), nil)
	}
	const minKills, maxKills = 12, 300
	inWindow, kills := 0, 0
	highest := map[string]int{} // the highest token a mark was answered for
	for kills < maxKills && (kills < minKills || inWindow == 0) {
		kills++
		// Every other kill waits for the ledger to grow, which is the child's
		// line being recorded, so that it lands after it whatever the machine's
		// speed; the rest are left to chance.
		wait := time.Duration(rand.IntN(60000)) * time.Microsecond // #nosec G404 -- the moment of a kill, not a secret
		var grown func() bool
		if kills%2 == 0 {
			size := int64(len(a.ledgerBytes()))
			grown = func() bool { return int64(len(a.ledgerBytes())) > size }
			wait = time.Duration(rand.IntN(3000)) * time.Microsecond // #nosec G404 -- the moment of a kill, not a secret
		}
		// Each child begins at a token of its own, above every earlier one, and
		// a multiple of the number of marks so that token i is of mark i mod it.
		c := startChild(t, "kill", a.root, a.conf, strconv.Itoa(kills*killMarks*1000))
		answered := c.killAfter(t, 1+rand.IntN(3), grown, wait) // #nosec G404 -- the moment of a kill, not a secret
		for _, i := range answered {
			highest[mid(100+i%killMarks)] = max(highest[mid(100+i%killMarks)], i)
		}
		a.restart()
		v := a.view()
		if v.DamagedOwn() != 0 || v.LedgerChanged() != 0 {
			t.Fatalf("after kill %d: %d own lines damaged, %d ledger lines changed", kills, v.DamagedOwn(), v.LedgerChanged())
		}
		if v.Missing() > 0 {
			inWindow++
		}
		for id, want := range highest {
			e, ok := v.Entry(id, open)
			if !ok {
				t.Fatalf("after kill %d: mark %s that was answered for is not read", kills, id)
			}
			got, err := strconv.Atoi(e.Mark().Body[:9])
			if err != nil || got < want {
				t.Fatalf("after kill %d: mark %s reads token %d, below %d it was answered for", kills, id, got, want)
			}
		}
		// Another process writes: what was recorded and not installed is put
		// back, and the file is whole.
		a.say(5000+kills, "after the kill")
		after := a.view()
		if after.Missing() != 0 || after.DamagedOwn() != 0 {
			t.Fatalf("after kill %d and a write: %d missing, %d damaged", kills, after.Missing(), after.DamagedOwn())
		}
		if got, want := len(sealedline.Lines(readFile(t, filepath.Join(a.marksDir(), a.id()+".jsonl")))), len(after.Entries(open)); got != want {
			t.Fatalf("after kill %d and a write the file holds %d lines for %d marks", kills, got, want)
		}
	}
	if inWindow == 0 {
		t.Fatalf("none of %d kills landed between a line recorded in the ledger and the file installed", kills)
	}
	t.Logf("%d kills: %d landed between the line recorded and the file installed (counted missing, put back by the next write)", kills, inWindow)
}

// A writer killed during the first writes of a vault, while it makes its id,
// the folders and the first file, leaves a vault the next process writes to, and
// the id it made is the id that goes on.
func TestAWriterKilledDuringItsFirstWritesLeavesAVaultThatWrites(t *testing.T) {
	t.Parallel()
	// Kills go on until one has landed after the id was made and before the
	// file was: the first write's own window.
	const minKills, maxKills = 25, 150
	caught, kills := 0, 0
	for kills < maxKills && (kills < minKills || caught == 0) {
		kill := kills
		kills++
		a := newClone(t, "A")
		c := startChild(t, "kill", a.root, a.conf, strconv.Itoa(kill*killMarks*1000))
		answered := c.killAfter(t, 1, nil, time.Duration(rand.IntN(12000))*time.Microsecond) // #nosec G404 -- the moment of a kill, not a secret
		_, noFile := os.Stat(a.marksDir())
		if noFile != nil && strings.Contains(string(a.ledgerBytes()), `"kind":"header"`) {
			caught++
		}
		a.restart()
		before := a.view().MachineID()
		a.say(1, "after the kill")
		if after := a.view(); after.MachineID() == "" || (before != "" && after.MachineID() != before) {
			t.Fatalf("kill %d: the id was %q and is %q", kill, before, after.MachineID())
		}
		for _, i := range answered {
			if got := a.words(100 + i%killMarks); got == "(none)" {
				t.Fatalf("kill %d: mark %d was answered for and is not read", kill, i)
			}
		}
		if v := a.view(); v.DamagedOwn() != 0 || v.Missing() != 0 {
			t.Fatalf("kill %d: %d damaged, %d missing after a write", kill, v.DamagedOwn(), v.Missing())
		}
		if n := strings.Count(string(a.ledgerBytes()), `"kind":"header"`); n != 1 {
			t.Fatalf("kill %d: the ledger holds %d headers", kill, n)
		}
	}
	if caught == 0 {
		t.Fatalf("none of %d kills landed after the id was made and before the folders were", kills)
	}
	t.Logf("%d of %d kills landed after the id was made and before the folders were", caught, kills)
}
