package ledger

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/mark/childtest"
	"github.com/koopa0/yomihon/internal/mark/confdir"
)

// A test that needs a second process, or one that dies, runs this test binary
// again as one. These variables say which, and over which configuration
// directory.
const (
	childModeVar    = "YOMIHON_TEST_LEDGER_MODE"
	childConfigVar  = "YOMIHON_TEST_LEDGER_CONFIG"
	childProcessVar = "YOMIHON_TEST_LEDGER_PROCESS"
)

func TestMain(m *testing.M) {
	if mode := os.Getenv(childModeVar); mode != "" {
		if err := runChild(mode, os.Getenv(childConfigVar), os.Getenv(childProcessVar)); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

const (
	// appendLineBytes is how long a line of the appending writer is: long enough
	// that a kill now and then lands inside the write of it.
	appendLineBytes = 2 << 20
	// compactLineBytes and compactMarks make a writer that compacts often: a
	// ledger of a few marks rewritten over and over, each time it passes
	// compactAtForKills.
	compactLineBytes  = 6 << 10
	compactMarks      = 3
	compactAtForKills = 64 << 10
	// togetherLines is how many lines each of two writers at once records.
	togetherLines = 150
)

// childLine is the i-th line a child writes, which the test can make again
// to compare: a mark's line whose body says i throughout.
func childLine(mode string, i int) *Event {
	mark, n := "m", appendLineBytes
	switch mode {
	case "append":
	case "compact":
		mark, n = fmt.Sprintf("m%d", i%compactMarks), compactLineBytes
	default:
		panic("ledger test: no line for mode " + mode)
	}
	// Digits need no escaping, so the line is put together as it is rather
	// than through markLine, which would take longer than the write.
	content := `{"mark":{"id":"` + mark + `","body":"` + strings.Repeat(fmt.Sprintf("%08d", i), n/8) + `"}}`
	return &Event{Kind: KindMark, Machine: "aaaaaaaa", Process: "kill", Mark: mark, Content: []byte(content)}
}

// runChild is the second process: it records lines until it has recorded the
// number it was given, or for ever if it is to be killed, saying each as it
// begins, which is once its line is made and Record is about to be called, and
// once Record has answered.
func runChild(mode, config, process string) error {
	dir, err := confdir.VaultDir(confdir.At(config), "/vaults/notes")
	if err != nil {
		return err
	}
	l := Open(dir, slog.New(slog.DiscardHandler))
	say := func(format string, args ...any) error {
		_, sayErr := fmt.Fprintf(os.Stdout, format+"\n", args...)
		return sayErr
	}
	switch mode {
	case "together":
		l.compactAt = 4 << 10
		for i := range togetherLines {
			if _, err = recordCrowded(context.Background(), l, togetherEvent(process, i)); err != nil {
				return err
			}
		}
		return nil
	case "compact":
		l.compactAt = compactAtForKills
	case "append":
		// Every line it answers for stays, so the test can find each: the test
		// compacts between kills.
		l.compactAt = 1 << 40
	}
	for i := 0; ; i++ {
		line := childLine(mode, i)
		if sayErr := say("begin %d", i); sayErr != nil {
			return sayErr
		}
		seq, recordErr := recordCrowded(context.Background(), l, line)
		if recordErr != nil {
			return recordErr
		}
		if sayErr := say("done %d %d", seq, i); sayErr != nil {
			return sayErr
		}
	}
}

// startChild runs this test binary again as a writer of mode over config.
func startChild(t *testing.T, mode, config, process string) *childtest.Child {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^$") // #nosec G204 G702 -- this test binary, run again as the writer
	cmd.Env = append(os.Environ(), childModeVar+"="+mode, childConfigVar+"="+config, childProcessVar+"="+process)
	cmd.Stderr = os.Stderr
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	return childtest.Watch(out, func() error {
		err := cmd.Process.Kill()
		_ = cmd.Wait() //nolint:errcheck // a killed process answers that it was killed
		return err
	})
}

// done is a line the child said Record answered for: its seq and which line it
// was.
type done struct {
	seq uint64
	i   int
}

// killAfter is the child killed as childtest.Child.KillAfter kills it, and
// every line it said Record answered for.
func killAfter(t *testing.T, c *childtest.Child, begins int, until func() bool, wait time.Duration) []done {
	t.Helper()
	var answered []done
	for _, words := range c.KillAfter(t, begins, until, wait) {
		seq, i, _ := strings.Cut(words, " ")
		s, err1 := strconv.ParseUint(seq, 10, 64)
		n, err2 := strconv.Atoi(i)
		if err1 != nil || err2 != nil {
			t.Fatalf("the child said done %q", words)
		}
		answered = append(answered, done{s, n})
	}
	return answered
}

// checkAnswered holds the ledger in dir to every line the child said Record
// answered for being there whole, and to no changed line, no seq twice, and
// seqs in order. It answers the ledger's events.
func checkAnswered(t *testing.T, dir confdir.Dir, mode string, answered []done) []Event {
	t.Helper()
	l, _ := open(t, dir)
	evs, changed := events(t, l)
	if changed != 0 {
		t.Fatalf("after a kill the ledger says %d lines were changed, want 0", changed)
	}
	bySeq := map[uint64]*Event{}
	for i := range evs {
		if i > 0 && evs[i].Seq <= evs[i-1].Seq {
			t.Fatalf("after a kill seq %d follows seq %d", evs[i].Seq, evs[i-1].Seq)
		}
		bySeq[evs[i].Seq] = &evs[i]
	}
	for _, d := range answered {
		e, ok := bySeq[d.seq]
		if !ok {
			if mode == "compact" && supersededIn(evs, d) {
				continue
			}
			t.Fatalf("after a kill the line Record answered seq %d for is not in the ledger", d.seq)
		}
		if want := childLine(mode, d.i); !bytes.Equal(e.Content, want.Content) || e.Mark != want.Mark {
			t.Fatalf("after a kill seq %d does not hold line %d whole", d.seq, d.i)
		}
	}
	return evs
}

// supersededIn says whether the ledger holds a newer line of the mark d is
// about, which a compaction keeps in its place.
func supersededIn(evs []Event, d done) bool {
	mark := childLine("compact", d.i).Mark
	for i := range evs {
		if evs[i].Mark == mark && evs[i].Seq > d.seq {
			return true
		}
	}
	return false
}

// TestAWriterKilledWhileItAppendsLosesNoCommittedLine kills a process that
// records long lines, again and again, at a moment nobody chose, a little
// after it has begun one. Whatever the moment, every line Record answered for
// is in the ledger whole, nothing reads as changed, and the next line, here the
// test's own, is read whole after whatever the kill left. The kills go on until
// one has landed after a line began to reach the file and before the child
// said Record answered for it: in the write, or in the sync after it, which is
// the window the commit exists for (or, rarely, in the instant between the
// answer and the saying of it, which is no window at all but cannot be told
// from one). A kill that stops the write itself part way, which
// leaves a line cut short, is counted too; macOS shows it about once in a
// hundred kills here and Windows not at all, so it is not waited for, and the
// cut line's reading is held by TestALineCutShortIsReadPastAndTheNextBeginsOnItsOwn
// on the very bytes such a kill leaves.
func TestAWriterKilledWhileItAppendsLosesNoCommittedLine(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	dir, err := confdir.VaultDir(confdir.At(config), "/vaults/notes")
	if err != nil {
		t.Fatal(err)
	}
	const minKills, maxKills = 12, 300
	cutWrites, midRecord, kills := 0, 0, 0
	var before uint64 // the highest seq in the ledger before the child began
	for kills < maxKills && (kills < minKills || midRecord == 0) {
		kills++
		// Every other kill waits for the ledger to grow, which is the child's
		// line reaching the file, and so lands after it whatever the machine's
		// speed: on a slow runner, or under the race detector, making and
		// encoding a long line can take longer than a delay chosen blind. The
		// rest are left to chance.
		var grown func() bool
		wait := time.Duration(rand.IntN(40000)) * time.Microsecond // #nosec G404 -- the moment of a kill, not a secret
		if kills%2 == 0 {
			base := sizeOf(t, dir)
			grown = func() bool { return sizeOf(t, dir) > base }
			wait = time.Duration(rand.IntN(1000)) * time.Microsecond // #nosec G404 -- the moment of a kill, not a secret
		}
		c := startChild(t, "append", config, "")
		answered := killAfter(t, c, 1, grown, wait)
		data, err := os.ReadFile(dir.Join(fileName))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		cut := len(data) > 0 && data[len(data)-1] != '\n'
		evs := checkAnswered(t, dir, "append", answered)
		last := before
		if len(answered) > 0 {
			last = answered[len(answered)-1].seq
		}
		if cut {
			cutWrites++
		}
		if cut || (len(evs) > 0 && evs[len(evs)-1].Seq > last) {
			midRecord++
		}

		l, _ := open(t, dir)
		next := record(t, l, markEvent("aaaaaaaa", "test", "after", strconv.Itoa(kills)))
		if latest, latestErr := latestIn(l, "aaaaaaaa"); latestErr != nil || latest["after"].Seq != next {
			t.Fatalf("after kill %d the test's own line is not read whole: %v, %v", kills, latest["after"], latestErr)
		}
		if err = l.Compact(t.Context()); err != nil {
			t.Fatal(err)
		}
		before = next
	}
	if midRecord == 0 {
		t.Fatalf("none of %d kills landed after a line began to reach the file and before Record answered", kills)
	}
	t.Logf("%d kills: %d after a line began to reach the file and before Record answered, %d of them cutting the line part way", kills, midRecord, cutWrites)
}

// leftoverName is how a temporary file of a compaction is named.
var leftoverName = regexp.MustCompile(`^\.ledger-\d+\.jsonl$`)

// TestAWriterKilledWhileItCompactsLosesNoLiveLine kills a process that records
// and compacts over and over. Whatever the moment, the ledger is the one before
// a compaction or the one after it: every line Record answered for is there, or
// a newer line of the same mark is, nothing reads as changed, and no seq goes
// back. A kill between a compaction's temporary file and its rename leaves that
// file, and the next process removes it on its first Record. The kills go on
// until one has left such a file, which is the window that removal exists for.
func TestAWriterKilledWhileItCompactsLosesNoLiveLine(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	dir, err := confdir.VaultDir(confdir.At(config), "/vaults/notes")
	if err != nil {
		t.Fatal(err)
	}
	const minKills, maxKills = 12, 300
	caught, kills := 0, 0
	for kills < maxKills && (kills < minKills || caught == 0) {
		kills++
		c := startChild(t, "compact", config, "")
		answered := killAfter(t, c, 1+rand.IntN(16), nil, time.Duration(rand.IntN(20000))*time.Microsecond) // #nosec G404 -- the moment of a kill, not a secret
		// A first kill that lands before the child's first Record has made the
		// vault's directory leaves nothing there at all.
		entries, err := os.ReadDir(dir.Path())
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		left := 0
		for _, entry := range entries {
			if leftoverName.MatchString(entry.Name()) {
				left++
			}
		}
		if left > 0 {
			caught++
		}
		evs := checkAnswered(t, dir, "compact", answered)

		l, _ := open(t, dir)
		seq := record(t, l, markEvent("aaaaaaaa", "test", "after", strconv.Itoa(kills)))
		if len(evs) > 0 && seq <= evs[len(evs)-1].Seq {
			t.Fatalf("after kill %d the next line took seq %d, not above the %d the ledger holds", kills, seq, evs[len(evs)-1].Seq)
		}
		if entries, err = os.ReadDir(dir.Path()); err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if leftoverName.MatchString(entry.Name()) {
				t.Fatalf("after kill %d the next process's first Record left %s", kills, entry.Name())
			}
		}
	}
	if caught == 0 {
		t.Fatalf("none of %d kills landed between a compaction's temporary file and its rename", kills)
	}
	t.Logf("%d of %d kills landed between a compaction's temporary file and its rename", caught, kills)
}

// Two processes record at once into one ledger, with compactions among them,
// and neither loses the newest line of a mark nor doubles a seq: the lock is the file's, and
// each process's cache follows what the other did.
func TestTwoProcessesRecordAtOnceAndLoseNothing(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	dir, err := confdir.VaultDir(confdir.At(config), "/vaults/notes")
	if err != nil {
		t.Fatal(err)
	}
	var cmds []*exec.Cmd
	for _, process := range []string{"one", "two"} {
		cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^$") // #nosec G204 G702 -- this test binary, run again as a writer
		cmd.Env = append(os.Environ(), childModeVar+"=together", childConfigVar+"="+config, childProcessVar+"="+process)
		cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
		if err = cmd.Start(); err != nil {
			t.Fatal(err)
		}
		cmds = append(cmds, cmd)
	}
	for _, cmd := range cmds {
		if err = cmd.Wait(); err != nil {
			t.Fatalf("a writer failed: %v", err)
		}
	}
	checkNewestThere(t, dir, []string{"one", "two"}, togetherLines)
}

// sizeOf is how long the ledger in dir is, 0 when it is not there.
func sizeOf(t *testing.T, dir confdir.Dir) int64 {
	t.Helper()
	info, err := os.Stat(dir.Join(fileName))
	if errors.Is(err, os.ErrNotExist) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	return info.Size()
}
