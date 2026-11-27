package ledger

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/mark/confdir"
	"github.com/koopa0/yomihon/internal/mark/sealedline"
)

// logbook is a log a test reads back.
type logbook struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *logbook) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *logbook) count(message string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.Count(b.buf.String(), "msg=\""+message+"\"")
}

// vaultDir is a vault's directory under a configuration directory of the
// test's own, not made yet, as a vault that never wrote anything has it.
func vaultDir(t *testing.T) confdir.Dir {
	t.Helper()
	dir, err := confdir.VaultDir(confdir.At(t.TempDir()), "/vaults/notes")
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func open(t *testing.T, dir confdir.Dir) (*Ledger, *logbook) {
	t.Helper()
	book := &logbook{}
	return Open(dir, slog.New(slog.NewTextHandler(book, nil))), book
}

// markLine is a line of a machine file about one mark, as the machine file's
// writer would hand it to Record.
func markLine(id, body string) []byte {
	return fmt.Appendf(nil, `{"fmt":1,"m":"1f3a9c07","rev":1,"mark":{"id":%q,"kind":"thought","body":%q}}`, id, body)
}

func markEvent(writer, session, id, body string) *Event {
	return &Event{Kind: KindMark, Writer: writer, Session: session, Content: markLine(id, body)}
}

func record(t *testing.T, l *Ledger, e *Event) uint64 {
	t.Helper()
	seq, err := l.Record(t.Context(), e)
	if err != nil {
		t.Fatalf("Record %s: %v", e.Kind, err)
	}
	return seq
}

func events(t *testing.T, l *Ledger) (held []Event, changed int) {
	t.Helper()
	evs, changed, err := l.Events()
	if err != nil {
		t.Fatal(err)
	}
	return evs, changed
}

// latestIn is LatestMarks of writer over what l reads now.
func latestIn(l *Ledger, writer string) (map[string]Event, error) {
	events, _, err := l.Events()
	if err != nil {
		return nil, err
	}
	return LatestMarks(events, writer), nil
}

func fileOf(t *testing.T, dir confdir.Dir, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(dir.Join(name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func writeFile(t *testing.T, dir confdir.Dir, name string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(dir.Path(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir.Join(name), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

// appendRaw puts bytes at the end of the ledger as something other than Record
// would: a crash part way through a line, or a hand.
func appendRaw(t *testing.T, dir confdir.Dir, data []byte) {
	t.Helper()
	f, err := os.OpenFile(dir.Join(fileName), os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Write(data); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
}

func sameEvent(a, b *Event) bool {
	return a.Seq == b.Seq && a.Time.Equal(b.Time) && a.Kind == b.Kind && a.ID == b.ID && a.Path == b.Path &&
		a.Sum == b.Sum && a.Writer == b.Writer && a.Session == b.Session && bytes.Equal(a.Content, b.Content)
}

// What is recorded is read back as it was given, by this ledger and by another
// over the same directory, which is what a second process is: the bytes a line
// keeps among them, whatever they are. Bytes that are not UTF-8 would come back
// as U+FFFD from a JSON string, so they go as base64, and only they do.
func TestARecordIsReadBackAsItWasGiven(t *testing.T) {
	t.Parallel()
	dir := vaultDir(t)
	l, _ := open(t, dir)
	given := []Event{
		{Kind: "header", ID: "1f3a9c07", Session: "7a1f03d2"},
		*markEvent("1f3a9c07", "7a1f03d2", "m1", "為什麼 VACUUM 不立刻還空間？ <b>&amp;</b>\u2028"),
		{Kind: "marks-replaced", Path: "reader-notes/.yomihon/marks/1f3a9c07.jsonl", Sum: "abc", Content: []byte{0xff, 0xfe, '\n', 0, 'x'}},
		{Kind: "marks-replaced", Content: []byte("a line\nand \"another\"\r\n\t")},
		{Kind: "said", ID: "new-id"},
	}
	for i := range given {
		given[i].Seq = record(t, l, &given[i])
		if given[i].Seq != uint64(i+1) {
			t.Fatalf("line %d took seq %d, want %d", i, given[i].Seq, i+1)
		}
	}
	data := fileOf(t, dir, fileName)
	if !bytes.Contains(data, []byte(`"b64":"//4KAHg="`)) || bytes.Count(data, []byte(`"b64"`)) != 1 {
		t.Errorf("the ledger keeps the bytes that are not UTF-8 other than as base64, or others as base64:\n%s", data)
	}
	other, _ := open(t, dir)
	for _, reader := range []*Ledger{l, other} {
		got, changed := events(t, reader)
		if changed != 0 || len(got) != len(given) {
			t.Fatalf("read %d events and %d changed lines, want %d and 0", len(got), changed, len(given))
		}
		for i := range given {
			given[i].Time = got[i].Time
			if !sameEvent(&got[i], &given[i]) {
				t.Errorf("line %d reads %+v, want %+v", i, got[i], given[i])
			}
		}
	}
}

// Every line has a seq of its own, and the next is above every seq the ledger
// holds: after a restart, after a compaction, after the newest line was changed
// by hand, and after a compaction set that changed line aside.
func TestEachLineTakesItsOwnSeqAndTheyOnlyGoUp(t *testing.T) {
	t.Parallel()
	dir := vaultDir(t)
	l, _ := open(t, dir)
	for range 3 {
		record(t, l, markEvent("a", "s", "m1", "edit"))
	}
	restarted, _ := open(t, dir)
	if seq := record(t, restarted, markEvent("a", "s2", "m1", "edit")); seq != 4 {
		t.Fatalf("after a restart the line took seq %d, want 4", seq)
	}
	if err := restarted.Compact(t.Context()); err != nil {
		t.Fatal(err)
	}
	if seq := record(t, restarted, markEvent("a", "s2", "m2", "other")); seq != 5 {
		t.Fatalf("after a compaction the line took seq %d, want 5", seq)
	}

	// The newest line, seq 5, changed by hand: it is not an event, and its seq
	// is still taken.
	data := fileOf(t, dir, fileName)
	writeFile(t, dir, fileName, bytes.Replace(data, []byte("other"), []byte("OTHER"), 1))
	if seq := record(t, restarted, markEvent("a", "s2", "m3", "third")); seq != 6 {
		t.Fatalf("after the newest line was changed the next took seq %d, want 6", seq)
	}
	// A compaction sets the changed line aside, out of the ledger, and the
	// line now newest is seq 6; the set-aside seq 5 stays below it either way.
	// Change seq 6 too, and compact: the highest seq left in the ledger is 4.
	data = fileOf(t, dir, fileName)
	writeFile(t, dir, fileName, bytes.Replace(data, []byte("third"), []byte("THIRD"), 1))
	if err := restarted.Compact(t.Context()); err != nil {
		t.Fatal(err)
	}
	fresh, _ := open(t, dir)
	if seq := record(t, fresh, markEvent("a", "s3", "m4", "fourth")); seq != 7 {
		t.Fatalf("after the newest lines were set aside the next took seq %d, want 7", seq)
	}
	got, changed := events(t, fresh)
	if changed != 2 {
		t.Errorf("the ledger says %d lines were changed, want 2", changed)
	}
	seen := map[uint64]bool{}
	for _, e := range got {
		if seen[e.Seq] {
			t.Errorf("seq %d is on two lines", e.Seq)
		}
		seen[e.Seq] = true
	}
}

// A crash part way through a line leaves its first part at the end of the
// ledger. It was never committed: it is read past, said to be no changed line,
// and the next line begins on a line of its own so that neither spoils the
// other. The next line does not take a seq the cut one had begun to say, and a
// compaction keeps the cut line where it was, since a hand that cut a line and
// kept its newline cannot be told from a crash. A whole line that lost only
// its newline is whole, and the next line still begins on its own.
func TestALineCutShortIsReadPastAndTheNextBeginsOnItsOwn(t *testing.T) {
	t.Parallel()
	dir := vaultDir(t)
	l, _ := open(t, dir)
	record(t, l, markEvent("a", "s", "m1", "one"))
	record(t, l, markEvent("a", "s", "m2", "two"))
	cutLine := encode(&Event{Seq: 3, Kind: KindMark, Writer: "a", Content: markLine("m3", "three")})
	// `{"seq":3,` is the first 9 bytes: a line cut inside them says no seq.
	for _, tt := range []struct {
		at   int
		next uint64
	}{{1, 3}, {8, 3}, {9, 4}, {len(cutLine) / 2, 4}, {len(cutLine) - 2, 4}, {len(cutLine) - 1, 4}} {
		at := tt.at
		t.Run(fmt.Sprintf("cut at %d of %d", at, len(cutLine)), func(t *testing.T) {
			t.Parallel()
			sub := vaultDir(t)
			writeFile(t, sub, fileName, fileOf(t, dir, fileName))
			appendRaw(t, sub, cutLine[:at])
			reader, _ := open(t, sub)
			if got, changed := events(t, reader); len(got) != 2 || changed != 0 {
				t.Fatalf("with a line cut short at the end: %d events, %d changed; want 2 and 0", len(got), changed)
			}
			if seq := record(t, reader, markEvent("a", "s", "m4", "four")); seq != tt.next {
				t.Fatalf("the line after one cut at %d took seq %d, want %d", at, seq, tt.next)
			}
			again, _ := open(t, sub)
			got, changed := events(t, again)
			if len(got) != 3 || changed != 0 || got[2].mark != "m4" {
				t.Fatalf("after the next line: %d events, %d changed; want the next line read whole beside the two", len(got), changed)
			}
			if !bytes.Contains(fileOf(t, sub, fileName), append(append(slices.Clone(cutLine[:at]), '\n'), got[2].raw...)) {
				t.Error("the next line does not begin on a line of its own after the cut one")
			}
			if err := again.Compact(t.Context()); err != nil {
				t.Fatal(err)
			}
			if data := fileOf(t, sub, fileName); !bytes.Contains(data, append(append(slices.Clone(got[1].raw), '\n'), cutLine[:at]...)) {
				t.Errorf("a compaction did not keep the cut line where it was:\n%s", data)
			}
			if after, changed := events(t, again); len(after) != 3 || changed != 0 {
				t.Errorf("after a compaction: %d events, %d changed; want 3 and 0", len(after), changed)
			}
		})
	}
	t.Run("a whole line without its newline", func(t *testing.T) {
		t.Parallel()
		sub := vaultDir(t)
		writeFile(t, sub, fileName, fileOf(t, dir, fileName))
		appendRaw(t, sub, cutLine)
		reader, _ := open(t, sub)
		if got, _ := events(t, reader); len(got) != 3 {
			t.Fatalf("%d events, want the line that lost only its newline among them", len(got))
		}
		if seq := record(t, reader, markEvent("a", "s", "m4", "four")); seq != 4 {
			t.Fatalf("the next line took seq %d, want 4", seq)
		}
		if got, changed := events(t, reader); len(got) != 4 || changed != 0 {
			t.Fatalf("%d events, %d changed; want 4 and 0", len(got), changed)
		}
	})
}

// A whole line that does not fit its checksum was changed after it was
// written, by an agent's search and replace across the home directory, say. It
// is counted and never read as an event. A compaction moves it out of the
// ledger into ledger.damaged.jsonl byte for byte, where it is still counted,
// and a second compaction does not set it aside twice.
func TestAChangedLineIsCountedNeverReadAndSetAside(t *testing.T) {
	t.Parallel()
	dir := vaultDir(t)
	l, _ := open(t, dir)
	record(t, l, markEvent("a", "s", "m1", "the reader's words"))
	record(t, l, markEvent("a", "s", "m2", "more of them"))
	record(t, l, markEvent("a", "s", "m3", "the last"))
	data := fileOf(t, dir, fileName)
	changedData := bytes.Replace(data, []byte("more of them"), []byte("MORE of them"), 1)
	writeFile(t, dir, fileName, changedData)
	changedLine := sealedline.Lines(changedData)[1]

	got, changed := events(t, l)
	if changed != 1 || len(got) != 2 || got[0].mark != "m1" || got[1].mark != "m3" {
		t.Fatalf("after one line was changed: %d events, %d changed; want m1 and m3 read and one changed", len(got), changed)
	}
	if latest, err := latestIn(l, "a"); err != nil || len(latest) != 2 {
		t.Fatalf("LatestMarks = %v, %v; want the two whole lines", latest, err)
	}
	for range 2 {
		if err := l.Compact(t.Context()); err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(fileOf(t, dir, fileName), changedLine) {
			t.Error("the changed line is still in the ledger after a compaction")
		}
		if aside := fileOf(t, dir, damagedName); !bytes.Equal(aside, append(slices.Clone(changedLine), '\n')) {
			t.Errorf("ledger.damaged.jsonl holds %q, want the changed line once, byte for byte", aside)
		}
		if _, changed = events(t, l); changed != 1 {
			t.Errorf("after a compaction the ledger says %d lines were changed, want 1", changed)
		}
	}
	t.Run("a compaction stopped after it set the line aside", func(t *testing.T) {
		t.Parallel()
		sub := vaultDir(t)
		writeFile(t, sub, fileName, changedData)
		writeFile(t, sub, damagedName, append(slices.Clone(changedLine), '\n'))
		reader, _ := open(t, sub)
		if _, changed := events(t, reader); changed != 1 {
			t.Errorf("a line both in the ledger and set aside is counted %d times, want once", changed)
		}
	})
	t.Run("a changed line that says the last seq there is", func(t *testing.T) {
		t.Parallel()
		sub := vaultDir(t)
		body := []byte(`{"seq":18446744073709551615,"t":0,"kind":"x"`)
		top := slices.Concat(body, []byte(`,"h":"0000000000000000"}`))
		writeFile(t, sub, fileName, append(top, '\n'))
		reader, _ := open(t, sub)
		seq := record(t, reader, &Event{Kind: "header", ID: "1f3a9c07"})
		if got, _ := events(t, reader); seq == 0 || len(got) != 1 || got[0].Seq != seq {
			t.Errorf("after a line saying the last seq, Record took seq %d and the ledger reads %d lines; want one whole line", seq, len(got))
		}
	})
	t.Run("the checksum itself changed", func(t *testing.T) {
		t.Parallel()
		sub := vaultDir(t)
		line := encode(&Event{Seq: 1, Kind: "header", ID: "1f3a9c07"})
		line[len(line)-3] ^= 1
		writeFile(t, sub, fileName, append(line, '\n'))
		reader, _ := open(t, sub)
		if got, changed := events(t, reader); len(got) != 0 || changed != 1 {
			t.Errorf("%d events, %d changed; want a line whose checksum was changed counted as changed", len(got), changed)
		}
	})
}

// Recording a line again, as a caller that got an error and tried once more
// does, adds a line with a seq of its own and changes nothing else: the newest
// is what LatestMarks answers, a compaction keeps that one, and a compaction
// of what a compaction left changes no byte.
func TestRecordingTheSameLineAgainIsIdempotent(t *testing.T) {
	t.Parallel()
	dir := vaultDir(t)
	l, _ := open(t, dir)
	first := record(t, l, markEvent("a", "s", "m1", "same"))
	second := record(t, l, markEvent("a", "s", "m1", "same"))
	record(t, l, markEvent("a", "s", "m2", "other"))
	latest, err := latestIn(l, "a")
	if err != nil || len(latest) != 2 || latest["m1"].Seq != second {
		t.Fatalf("LatestMarks = %v, %v; want m1 at seq %d, not %d", latest, err, second, first)
	}
	if err = l.Compact(t.Context()); err != nil {
		t.Fatal(err)
	}
	after := fileOf(t, dir, fileName)
	if got, _ := events(t, l); len(got) != 2 || got[0].Seq != second {
		t.Fatalf("after a compaction the ledger holds %d lines, the first at seq %d; want m1's newest and m2", len(got), got[0].Seq)
	}
	if err = l.Compact(t.Context()); err != nil {
		t.Fatal(err)
	}
	if again := fileOf(t, dir, fileName); !bytes.Equal(again, after) {
		t.Errorf("a second compaction changed the ledger:\n%s\nwant\n%s", again, after)
	}
	if again, err := latestIn(l, "a"); err != nil || !sameEvent(new(again["m1"]), new(latest["m1"])) || !sameEvent(new(again["m2"]), new(latest["m2"])) {
		t.Errorf("after compactions LatestMarks = %v, %v; want %v", again, err, latest)
	}
}

// A compaction keeps every line it promises to: every line of a kind it does
// not read, the files a write replaced, a mark line whose content it cannot
// read a mark from, the newest line of each mark by each writer, a line a
// newer version wrote with a field this one does not know, each byte for byte
// and in order; and a line for each process whose mark lines it all dropped.
// What it drops is only a mark line a newer one of the same mark by the same
// writer supersedes.
func TestCompactionKeepsEveryLiveLine(t *testing.T) {
	t.Parallel()
	dir := vaultDir(t)
	l, _ := open(t, dir)
	record(t, l, &Event{Kind: "header", ID: "aaaaaaaa", Session: "s1"})
	record(t, l, markEvent("aaaaaaaa", "s1", "m1", "a1"))
	record(t, l, markEvent("aaaaaaaa", "s1", "m2", "a1"))
	record(t, l, markEvent("aaaaaaaa", "s2", "m1", "a2")) // s2 writes only lines a later one supersedes
	record(t, l, &Event{Kind: "marks-replaced", Path: "reader-notes/.yomihon/marks/aaaaaaaa.jsonl", Sum: "f00d", Content: []byte{0xc3, 0x28, 'x'}})
	record(t, l, markEvent("bbbbbbbb", "s9", "m1", "b1")) // another id of this machine writes the same mark
	record(t, l, &Event{Kind: "retire", ID: "aaaaaaaa"})
	record(t, l, markEvent("aaaaaaaa", "s3", "m1", "a3"))
	record(t, l, &Event{Kind: KindMark, Writer: "aaaaaaaa", Session: "s1", Content: []byte("not a line of marks")})
	record(t, l, markEvent("bbbbbbbb", "s9", "m1", "b2"))
	record(t, l, markEvent("aaaaaaaa", "s3", "m2", "a3"))
	// A line of a later version: a field this one does not know, under a
	// checksum that fits.
	future, err := sealedline.Seal(struct {
		Seq   uint64         `json:"seq"`
		T     int64          `json:"t"`
		Kind  string         `json:"kind"`
		ID    string         `json:"id"`
		Later map[string]int `json:"later"`
	}{12, 1795164060000, "future", "x", map[string]int{"n": 1}})
	if err != nil {
		t.Fatal(err)
	}
	appendRaw(t, dir, append(slices.Clone(future), '\n'))
	before, changed := events(t, l)
	if changed != 0 || len(before) != 12 {
		t.Fatalf("before the compaction: %d events, %d changed; want 12 and 0", len(before), changed)
	}
	latestA, errA := latestIn(l, "aaaaaaaa")
	latestB, errB := latestIn(l, "bbbbbbbb")
	if errA != nil || errB != nil {
		t.Fatal(errA, errB)
	}

	if err := l.Compact(t.Context()); err != nil {
		t.Fatal(err)
	}
	after, changed := events(t, l)
	var wantSeqs []uint64
	for i := range before {
		if e := &before[i]; e.Kind != KindMark || e.mark == "" || e.Seq == latestOf(before, e).Seq {
			wantSeqs = append(wantSeqs, e.Seq)
		}
	}
	var keptSeqs []uint64
	for _, e := range after {
		if e.Kind != KindSession {
			keptSeqs = append(keptSeqs, e.Seq)
		}
	}
	if !slices.Equal(keptSeqs, wantSeqs) || changed != 0 {
		t.Fatalf("the compaction kept the lines at %v (changed %d), want %v", keptSeqs, changed, wantSeqs)
	}
	for _, e := range after {
		if e.Kind == KindSession {
			continue
		}
		i := slices.IndexFunc(before, func(b Event) bool { return b.Seq == e.Seq })
		if !bytes.Equal(e.raw, before[i].raw) {
			t.Errorf("seq %d was rewritten:\n%s\nwant\n%s", e.Seq, e.raw, before[i].raw)
		}
	}
	if !bytes.Contains(fileOf(t, dir, fileName), future) {
		t.Error("the line of a later version is not kept byte for byte")
	}
	var sessions []string
	for _, e := range after {
		if e.Kind == KindSession {
			sessions = append(sessions, fmt.Sprintf("%d %s/%s", e.Seq, e.ID, e.Session))
		}
	}
	if !slices.Equal(sessions, []string{"4 aaaaaaaa/s2"}) {
		t.Errorf("the compaction kept the processes %q as session lines, want only s2, at the seq of its line", sessions)
	}
	for writer, want := range map[string]map[string]Event{"aaaaaaaa": latestA, "bbbbbbbb": latestB} {
		got, err := latestIn(l, writer)
		if err != nil || len(got) != len(want) {
			t.Fatalf("after the compaction LatestMarks(%s) = %v, %v; want %v", writer, got, err, want)
		}
		for id := range want {
			if !sameEvent(new(got[id]), new(want[id])) {
				t.Errorf("after the compaction %s's newest of %s is %+v, want %+v", writer, id, got[id], want[id])
			}
		}
	}
}

func latestOf(events []Event, of *Event) *Event {
	latest := &Event{}
	for i := range events {
		if e := &events[i]; e.Kind == KindMark && e.Writer == of.Writer && e.mark == of.mark && e.Seq > latest.Seq {
			latest = e
		}
	}
	return latest
}

// A Record compacts the ledger once it is past its limit and twice what a
// compaction would leave of it. A ledger whose lines are all live is never
// rewritten, however long, by this process or the next: there is nothing to
// gain. One where each mark is written four times is rewritten only when it
// has grown to twice its live lines, and each rewrite leaves at most half.
func TestCompactionRunsPastItsLimitAndAtTwiceWhatItLeaves(t *testing.T) {
	t.Parallel()
	rewrites := func(t *testing.T, markOf func(i int) string) (n int, sizes [][2]int64) {
		t.Helper()
		dir := vaultDir(t)
		var last os.FileInfo
		for i := range 200 {
			l, _ := open(t, dir) // a process a Record, as the command line is
			l.compactAt = 4 << 10
			record(t, l, markEvent("a", "s", markOf(i), strings.Repeat("x", 200)))
			info, err := os.Stat(dir.Join(fileName))
			if err != nil {
				t.Fatal(err)
			}
			if last != nil && !os.SameFile(last, info) {
				n++
				sizes = append(sizes, [2]int64{last.Size(), info.Size()})
				// The ledger the compaction read is last and the line just
				// recorded, which is under 400 bytes.
				if last.Size()+400 <= l.compactAt {
					t.Fatalf("a compaction ran on a ledger of %d bytes and one line, under its limit", last.Size())
				}
			}
			last = info
		}
		return n, sizes
	}
	if n, _ := rewrites(t, func(i int) string { return fmt.Sprintf("m%03d", i) }); n != 0 {
		t.Errorf("a ledger of 200 live lines was rewritten %d times, want never", n)
	}
	n, sizes := rewrites(t, func(i int) string { return fmt.Sprintf("m%03d", i/4) })
	if n < 2 {
		t.Errorf("a ledger of marks written four times each was rewritten %d times, want at least twice", n)
	}
	for _, size := range sizes {
		if size[1] > size[0]/2+400 {
			t.Errorf("a rewrite of %d bytes left %d, more than half: it ran before the ledger was twice what it would leave", size[0], size[1])
		}
	}
}

// A compaction that cannot run, here because the directory takes no new file
// for the rewrite, is not the failure of the Record that triggered it: the line
// is committed. It is said once in the log, and tried again only once the
// ledger has doubled.
func TestACompactionThatCannotRunDoesNotFailTheRecord(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not refuse a new file by a directory's permission bits")
	}
	dir := vaultDir(t)
	l, book := open(t, dir)
	l.compactAt = 2 << 10
	record(t, l, markEvent("a", "s", "m0", "first"))
	if err := os.Chmod(dir.Path(), 0o500); err != nil { // #nosec G302 -- the test's own temporary directory, readable and searchable
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(dir.Path(), 0o700); err != nil { // #nosec G302 -- restores the mode the test removed so the directory can be cleaned up
			t.Errorf("restore the directory's mode: %v", err)
		}
	})
	if f, err := os.CreateTemp(dir.Path(), "probe"); err == nil {
		_ = f.Close() //nolint:errcheck // only whether it was made matters
		t.Skip("this user makes files in a directory it may not write (a privileged user), so the fixture shows nothing")
	}
	var warnedAt []int64
	for range 40 {
		record(t, l, markEvent("a", "s", "m1", strings.Repeat("z", 200)))
		if n := book.count("compact the ledger"); n > len(warnedAt) {
			info, err := os.Stat(dir.Join(fileName))
			if err != nil {
				t.Fatal(err)
			}
			warnedAt = append(warnedAt, info.Size())
		}
	}
	if len(warnedAt) < 2 || len(warnedAt) > 3 {
		t.Fatalf("40 records into a ledger that cannot be compacted warned %d times, at %v bytes; want two or three, each at twice the last", len(warnedAt), warnedAt)
	}
	for i := 1; i < len(warnedAt); i++ {
		if warnedAt[i] < 2*warnedAt[i-1]-400 {
			t.Errorf("the compaction was tried again at %d bytes, before the ledger doubled from %d", warnedAt[i], warnedAt[i-1])
		}
	}
	if got, _ := events(t, l); len(got) != 41 {
		t.Errorf("the ledger holds %d lines, want all 41", len(got))
	}
}

// A compaction killed between making its temporary file and renaming it leaves
// that file behind. The first time a process holds the ledger's lock it
// removes those files and nothing else: not the ledger, the lines set aside or
// the lock, and not the temporary files of the other files beside it, which a
// write under another lock may be in the middle of. Reading takes no lock and
// removes nothing.
func TestALeftoverOfACompactionIsRemovedAndNothingElse(t *testing.T) {
	t.Parallel()
	dir := vaultDir(t)
	l, _ := open(t, dir)
	record(t, l, markEvent("a", "s", "m1", "one"))
	keep := []string{".marks-123.json", ".progress-9.json", ".ledger.damaged-4.jsonl", "marks.json", damagedName}
	for _, name := range append([]string{".ledger-123.jsonl", ".ledger-4294967295.jsonl"}, keep...) {
		writeFile(t, dir, name, []byte("x\n"))
	}
	names := func() []string {
		entries, err := os.ReadDir(dir.Path())
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, entry := range entries {
			out = append(out, entry.Name())
		}
		return out
	}
	before := names()
	restarted, _ := open(t, dir)
	events(t, restarted)
	if _, err := latestIn(restarted, "a"); err != nil {
		t.Fatal(err)
	}
	if after := names(); !slices.Equal(after, before) {
		t.Fatalf("reading changed the directory from %q to %q", before, after)
	}
	record(t, restarted, markEvent("a", "s2", "m2", "two"))
	want := append(slices.Clone(keep), fileName, lockName)
	slices.Sort(want)
	if after := names(); !slices.Equal(after, want) {
		t.Errorf("after the first Record of a process the directory holds %q, want %q", after, want)
	}
}

// Reading a vault's ledger that is not there makes nothing: not the ledger,
// not its lock, not the directory.
func TestReadingAMissingLedgerMakesNothing(t *testing.T) {
	t.Parallel()
	dir := vaultDir(t)
	l, _ := open(t, dir)
	if got, changed := events(t, l); len(got) != 0 || changed != 0 {
		t.Fatalf("a missing ledger reads %d events, %d changed", len(got), changed)
	}
	if latest, err := latestIn(l, "a"); err != nil || len(latest) != 0 {
		t.Fatalf("LatestMarks of a missing ledger = %v, %v", latest, err)
	}
	if _, err := os.Stat(dir.Path()); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("reading made the vault's directory: %v", err)
	}
}

// What one ledger holds from its last read is not what it answers once the
// file has changed under it: another process's line, another's compaction, and
// a hand that changed a line in place to the same length.
func TestALedgerSeesWhatAnotherWriterDid(t *testing.T) {
	t.Parallel()
	dir := vaultDir(t)
	mine, _ := open(t, dir)
	theirs, _ := open(t, dir)
	record(t, mine, markEvent("a", "s", "m1", "mine"))
	record(t, mine, markEvent("a", "s", "m1", "mine again"))
	events(t, mine)

	record(t, theirs, markEvent("a", "t", "m2", "theirs"))
	if got, _ := events(t, mine); len(got) != 3 || got[2].Session != "t" {
		t.Fatalf("after another process's line this one reads %d events, want 3 with theirs last", len(got))
	}
	if err := theirs.Compact(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got, _ := events(t, mine); len(got) != 2 {
		t.Fatalf("after another process's compaction this one reads %d events, want 2", len(got))
	}
	data := fileOf(t, dir, fileName)
	f, err := os.OpenFile(dir.Join(fileName), os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	at := int64(bytes.Index(data, []byte("theirs")))
	if _, err = f.WriteAt([]byte("THEIRS"), at); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	if got, changed := events(t, mine); len(got) != 1 || changed != 1 {
		t.Fatalf("after a line was changed in place this one reads %d events and %d changed, want 1 and 1", len(got), changed)
	}
	if seq := record(t, mine, markEvent("a", "s", "m3", "next")); seq <= 3 {
		t.Errorf("the next line took seq %d, which a line in the ledger has", seq)
	}
}

// A Record that waits for the ledger's lock, here held as another process
// would hold it, keeps no reader of this process waiting: every page that reads
// the marks reads the ledger, and a write can wait for the lock for seconds.
func TestAReaderDoesNotWaitBehindARecordThatWaitsForTheLock(t *testing.T) {
	t.Parallel()
	dir := vaultDir(t)
	l, _ := open(t, dir)
	l.lockWait = time.Minute
	record(t, l, markEvent("a", "s", "m1", "first"))
	other, err := confdir.Acquire(t.Context(), dir, lockName, 0)
	if err != nil {
		t.Fatal(err)
	}
	released := false
	release := func() {
		if !released {
			released = true
			if err := other.Release(); err != nil {
				t.Errorf("Release: %v", err)
			}
		}
	}
	t.Cleanup(release)

	recorded := make(chan error, 1)
	go func() {
		_, err := l.Record(t.Context(), markEvent("a", "s", "m2", "second"))
		recorded <- err
	}()
	// Read for a while as the Record waits, each read bounded: one that waited
	// behind the Record would wait for as long as the lock is held.
	for until := time.Now().Add(200 * time.Millisecond); time.Now().Before(until); {
		read := make(chan error, 1)
		go func() {
			got, _, err := l.Events()
			if err == nil && len(got) != 1 {
				err = fmt.Errorf("read %d events while the second line waits, want 1", len(got))
			}
			read <- err
		}()
		select {
		case err := <-read:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("Events waited 5s behind a Record that waits for the ledger's lock")
		}
	}
	select {
	case err := <-recorded:
		t.Fatalf("the Record answered %v while another held the lock", err)
	default:
	}
	release()
	if err := <-recorded; err != nil {
		t.Fatalf("the Record once the lock came free: %v", err)
	}
	if got, _ := events(t, l); len(got) != 2 {
		t.Errorf("after the Record the ledger reads %d events, want 2", len(got))
	}
}

// A compaction installs the ledger while a reader, which takes no lock, has the
// file open: on Windows the rename over it is refused until the reader closes
// it, and is tried again rather than failing the compaction.
func TestACompactionInstallsTheLedgerWhileAReaderHasItOpen(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "windows" {
		t.Skip("elsewhere a rename over a file a reader has open is never refused, so there is nothing to try again")
	}
	dir := vaultDir(t)
	l, _ := open(t, dir)
	record(t, l, markEvent("a", "s", "m1", "old"))
	record(t, l, markEvent("a", "s", "m1", "new"))
	reader, err := os.Open(dir.Join(fileName))
	if err != nil {
		t.Fatal(err)
	}
	closed := make(chan error, 1)
	time.AfterFunc(50*time.Millisecond, func() { closed <- reader.Close() })
	if err := l.Compact(t.Context()); err != nil {
		t.Fatalf("Compact while a reader had the ledger open = %v, want it installed once the reader closed it", err)
	}
	if err := <-closed; err != nil {
		t.Fatal(err)
	}
	if got, _ := events(t, l); len(got) != 1 || !bytes.Contains(got[0].Content, []byte("new")) {
		t.Errorf("after the compaction the ledger reads %d events, want the newer line alone", len(got))
	}
}

// Lines another process set aside are counted though the ledger itself was left
// as it was, as a compaction stopped between setting them aside and installing
// the ledger leaves it: what one ledger holds from its last read is not what it
// answers once either file has changed under it.
func TestALedgerSeesLinesSetAsideByAnotherThatLeftTheLedgerAsItWas(t *testing.T) {
	t.Parallel()
	dir := vaultDir(t)
	l, _ := open(t, dir)
	record(t, l, markEvent("a", "s", "m1", "first"))
	if _, changed := events(t, l); changed != 0 {
		t.Fatalf("a ledger of whole lines reads %d changed, want 0", changed)
	}
	line := bytes.TrimSuffix(fileOf(t, dir, fileName), []byte("\n"))
	writeFile(t, dir, damagedName, append(bytes.Replace(line, []byte("first"), []byte("FIRST"), 1), '\n'))
	if _, changed := events(t, l); changed != 1 {
		t.Errorf("after another set a changed line aside the ledger reads %d changed, want 1", changed)
	}
}

// A reader that reads the file of the lines set aside while a compaction is
// still appending a changed line to it counts that line once: it is whole in the
// ledger still, and half written in the file beside it. What it read is not
// held as the count once the append has ended.
func TestALineSetAsideHalfWayIsCountedOnce(t *testing.T) {
	t.Parallel()
	dir := vaultDir(t)
	l, _ := open(t, dir)
	record(t, l, markEvent("a", "s", "m1", "first"))
	record(t, l, markEvent("a", "s", "m2", "second"))
	changedLedger := bytes.Replace(fileOf(t, dir, fileName), []byte("first"), []byte("FIRST"), 1)
	writeFile(t, dir, fileName, changedLedger)
	if _, changed := events(t, l); changed != 1 {
		t.Fatalf("a ledger with one changed line reads %d changed, want 1", changed)
	}
	line := sealedline.Lines(changedLedger)[0]
	writeFile(t, dir, damagedName, line[:len(line)/2])
	for read := range 2 {
		if _, changed := events(t, l); changed != 1 {
			t.Errorf("read %d while the line is half set aside: %d changed, want 1", read, changed)
		}
	}
	writeFile(t, dir, damagedName, append(bytes.Clone(line), '\n'))
	if _, changed := events(t, l); changed != 1 {
		t.Errorf("once the line is set aside whole: %d changed, want 1", changed)
	}
}

// A compaction and a carry leave what this process holds of the ledger in place
// until the new ledger is installed, so a reader of this process that reads
// meanwhile is answered from it and does not open the ledger the rename is
// replacing, which on Windows would refuse the rename. Here the install is
// refused before it begins, by a directory that cannot be written to, and what
// was held is held still.
func TestWhatIsHeldStaysUntilANewLedgerIsInstalled(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("a folder's mode does not stop a file being made in it on Windows")
	}
	for _, tt := range []struct {
		name    string
		ledger  func(t *testing.T, l *Ledger, dir confdir.Dir)
		install func(t *testing.T, l *Ledger) error
	}{
		{
			name: "a compaction",
			ledger: func(t *testing.T, l *Ledger, _ confdir.Dir) {
				t.Helper()
				record(t, l, markEvent("a", "s", "m1", "old"))
				record(t, l, markEvent("a", "s", "m1", "new"))
			},
			install: func(t *testing.T, l *Ledger) error {
				t.Helper()
				return l.Compact(t.Context())
			},
		},
		{
			name: "a carry",
			ledger: func(t *testing.T, _ *Ledger, dir confdir.Dir) {
				t.Helper()
				writeFile(t, dir, fileName, nil)
				// The lock file is made now, so that what the directory refuses
				// below is the install and not the lock.
				lock, err := confdir.Acquire(t.Context(), dir, lockName, 0)
				if err != nil {
					t.Fatal(err)
				}
				if err = lock.Release(); err != nil {
					t.Fatal(err)
				}
			},
			install: func(t *testing.T, l *Ledger) error {
				t.Helper()
				from := vaultDir(t)
				earlier, _ := open(t, from)
				record(t, earlier, markEvent("a", "s", "m1", "earlier"))
				_, err := l.Carry(t.Context(), from)
				return err
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := vaultDir(t)
			l, _ := open(t, dir)
			tt.ledger(t, l, dir)
			events(t, l)
			if err := os.Chmod(dir.Path(), 0o500); err != nil { // #nosec G302 -- the test's own temporary directory
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.Chmod(dir.Path(), 0o700); err != nil { // #nosec G302 -- restores the mode the test removed so the directory can be cleaned up
					t.Errorf("restore the directory's mode: %v", err)
				}
			})
			if err := os.WriteFile(dir.Join("probe"), nil, 0o600); err == nil {
				t.Skip("this user writes in a directory it may not write in (a privileged user), so the fixture shows nothing")
			}
			if err := tt.install(t, l); err == nil {
				t.Fatalf("%s into a directory that cannot be written to = nil, want the install refused", tt.name)
			}
			l.mu.Lock()
			held := l.read.info != nil
			l.mu.Unlock()
			if !held {
				t.Errorf("%s that could not install a new ledger dropped what this process held of the old one", tt.name)
			}
		})
	}
}

// Goroutines that share one ledger, and ledgers that share one directory as two
// processes do, each record without losing or doubling a line or a seq, while
// compactions drop the lines that newer ones supersede among them, and while
// readers of the shared ledger, which take no lock, read it over and over.
func TestRecordsAtOnceLoseNothing(t *testing.T) {
	t.Parallel()
	const writers, each = 4, 25
	dir := vaultDir(t)
	// Eight writers poll one lock while compactions run under it; on a slow
	// runner one of them can wait past confdir.LockWait without anything being
	// wrong. This test asks whether a line is lost, so it waits as long as that
	// takes.
	const crowded = time.Minute
	shared, _ := open(t, dir)
	shared.compactAt = 4 << 10
	shared.lockWait = crowded
	var wg sync.WaitGroup
	errs := make(chan error, 2*writers*each)
	var names []string
	for w := range 2 * writers {
		l, name := shared, "w"+strconv.Itoa(w)
		if w >= writers {
			l, _ = open(t, dir)
			l.compactAt = 4 << 10
			l.lockWait = crowded
		}
		names = append(names, name)
		wg.Go(func() {
			for i := range each {
				if _, err := l.Record(t.Context(), togetherEvent(name, i)); err != nil {
					errs <- err
				}
			}
		})
	}
	written := make(chan struct{})
	var readers sync.WaitGroup
	for range 2 {
		readers.Go(func() {
			for {
				select {
				case <-written:
					return
				default:
				}
				if err := readWhole(shared); err != nil {
					t.Error(err)
					return
				}
			}
		})
	}
	wg.Wait()
	close(written)
	readers.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	checkNewestThere(t, dir, names, each)
}

// readWhole reads l as a page would while others write, and says what is wrong
// with what it read: a line that is not a whole one, or a seq on two lines.
func readWhole(l *Ledger) error {
	got, _, err := l.Events()
	if err != nil {
		return err
	}
	for i := range got {
		if markOf(got[i].Content) == "" {
			return fmt.Errorf("a reader read a mark line that names no mark: %q", got[i].Content)
		}
		if i > 0 && got[i].Seq <= got[i-1].Seq {
			return fmt.Errorf("a reader read seq %d after seq %d", got[i].Seq, got[i-1].Seq)
		}
	}
	return nil
}

// togetherMarks is how many marks each writer at once writes over and over.
const togetherMarks = 5

// togetherEvent is the i-th line a writer at once records: one of its marks,
// saying i.
func togetherEvent(writer string, i int) *Event {
	return markEvent("aaaaaaaa", writer, fmt.Sprintf("%s-%d", writer, i%togetherMarks), strconv.Itoa(i))
}

// checkNewestThere reads the ledger in dir anew and holds it to no seq twice,
// no changed line, the newest line of each writer's every mark there, and
// fewer lines than were written, which says a compaction ran among them.
func checkNewestThere(t *testing.T, dir confdir.Dir, writers []string, each int) {
	t.Helper()
	l, _ := open(t, dir)
	got, changed := events(t, l)
	if changed != 0 {
		t.Fatalf("the ledger says %d lines were changed, want 0", changed)
	}
	seqs := map[uint64]bool{}
	for i := range got {
		if seqs[got[i].Seq] {
			t.Fatalf("seq %d is on two lines", got[i].Seq)
		}
		seqs[got[i].Seq] = true
	}
	if len(got) >= len(writers)*each {
		t.Errorf("the ledger holds all %d lines written, so no compaction ran among the writers", len(got))
	}
	latest, err := latestIn(l, "aaaaaaaa")
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range writers {
		for i := each - togetherMarks; i < each; i++ {
			want := togetherEvent(w, i)
			if e, ok := latest[markOf(want.Content)]; !ok || !bytes.Equal(e.Content, want.Content) {
				t.Errorf("the newest line of %s's mark %d is %q, want %q", w, i%togetherMarks, e.Content, want.Content)
			}
		}
	}
}

// The earlier path's ledger is carried into a vault that has none, byte for
// byte, with what it set aside; the earlier one is left as it was; a second
// call carries nothing more and changes nothing; and a vault that has lines of
// its own keeps them, with nothing of the earlier one merged in.
func TestCarryBringsTheEarlierLedgerOnlyIntoOneThatHasNone(t *testing.T) {
	t.Parallel()
	config := confdir.At(t.TempDir())
	from, err := confdir.VaultDir(config, "/vaults/before")
	if err != nil {
		t.Fatal(err)
	}
	earlier, _ := open(t, from)
	record(t, earlier, &Event{Kind: "header", ID: "aaaaaaaa"})
	record(t, earlier, markEvent("aaaaaaaa", "s", "m1", "kept there"))
	record(t, earlier, markEvent("aaaaaaaa", "s", "m2", "changed there"))
	data := fileOf(t, from, fileName)
	writeFile(t, from, fileName, bytes.Replace(data, []byte("changed there"), []byte("CHANGED there"), 1))
	if err = earlier.Compact(t.Context()); err != nil {
		t.Fatal(err)
	}
	record(t, earlier, markEvent("aaaaaaaa", "s", "m3", "after"))
	theirLedger, theirAside := fileOf(t, from, fileName), fileOf(t, from, damagedName)

	to, err := confdir.VaultDir(config, "/vaults/after")
	if err != nil {
		t.Fatal(err)
	}
	l, _ := open(t, to)
	for i, want := range []bool{true, false} {
		carried, carryErr := l.Carry(t.Context(), from)
		if carryErr != nil || carried != want {
			t.Fatalf("Carry %d = %v, %v; want %v", i+1, carried, carryErr, want)
		}
		if got := fileOf(t, to, fileName); !bytes.Equal(got, theirLedger) {
			t.Errorf("after Carry %d the ledger is\n%s\nwant the earlier one byte for byte\n%s", i+1, got, theirLedger)
		}
		if got := fileOf(t, to, damagedName); !bytes.Equal(got, theirAside) {
			t.Errorf("after Carry %d the lines set aside are %q, want %q", i+1, got, theirAside)
		}
		if got, changed := events(t, l); len(got) != 3 || changed != 1 {
			t.Errorf("after Carry %d the ledger reads %d events and %d changed, want 3 and 1", i+1, len(got), changed)
		}
	}
	if seq := record(t, l, markEvent("aaaaaaaa", "s2", "m1", "here")); seq != 5 {
		t.Errorf("the first line after the carry took seq %d, want 5", seq)
	}
	if !bytes.Equal(fileOf(t, from, fileName), theirLedger) || !bytes.Equal(fileOf(t, from, damagedName), theirAside) {
		t.Error("Carry changed the earlier ledger")
	}

	own, err := confdir.VaultDir(config, "/vaults/own")
	if err != nil {
		t.Fatal(err)
	}
	o, _ := open(t, own)
	record(t, o, &Event{Kind: "header", ID: "bbbbbbbb"})
	mineBefore := fileOf(t, own, fileName)
	if carried, carryErr := o.Carry(t.Context(), from); carryErr != nil || carried {
		t.Fatalf("Carry into a ledger with lines of its own = %v, %v; want false, nil", carried, carryErr)
	}
	if got := fileOf(t, own, fileName); !bytes.Equal(got, mineBefore) {
		t.Error("Carry changed a ledger with lines of its own")
	}
	if _, statErr := os.Stat(own.Join(damagedName)); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("Carry set lines aside in a ledger with lines of its own: %v", statErr)
	}

	nothing, err := confdir.VaultDir(config, "/vaults/never")
	if err != nil {
		t.Fatal(err)
	}
	if carried, err := o.Carry(t.Context(), nothing); err != nil || carried {
		t.Errorf("Carry from a path with no ledger = %v, %v; want false, nil", carried, err)
	}
}

// A carry stopped after it set the earlier lines aside and before it installed
// the ledger is finished by the same call made again, and sets nothing aside
// twice.
func TestACarryMadeAgainSetsNothingAsideTwice(t *testing.T) {
	t.Parallel()
	config := confdir.At(t.TempDir())
	from, err := confdir.VaultDir(config, "/vaults/before")
	if err != nil {
		t.Fatal(err)
	}
	to, err := confdir.VaultDir(config, "/vaults/after")
	if err != nil {
		t.Fatal(err)
	}
	earlier, _ := open(t, from)
	record(t, earlier, markEvent("aaaaaaaa", "s", "m1", "changed"))
	writeFile(t, from, fileName, bytes.Replace(fileOf(t, from, fileName), []byte("changed"), []byte("CHANGED"), 1))
	record(t, earlier, markEvent("aaaaaaaa", "s", "m2", "whole"))
	if err := earlier.Compact(t.Context()); err != nil {
		t.Fatal(err)
	}
	aside := fileOf(t, from, damagedName)
	// The state a carry killed between its two steps leaves: the lines set
	// aside here, no ledger.
	writeFile(t, to, damagedName, aside)
	l, _ := open(t, to)
	if carried, err := l.Carry(t.Context(), from); err != nil || !carried {
		t.Fatalf("Carry = %v, %v; want true, nil", carried, err)
	}
	if got := fileOf(t, to, damagedName); !bytes.Equal(got, aside) {
		t.Errorf("the lines set aside are %q, want %q once", got, aside)
	}
}

// The longest file of marks a write replaces is kept whole in one line however
// much of it JSON would escape: a file that the reader filled with code and
// quoted it twice over, every byte of it a backslash or a quote, is recorded
// and read back. A refusal here would refuse every later write for as long as
// the file stood in the vault.
func TestAFileOfMarksThatJSONEscapesWhollyFitsALine(t *testing.T) {
	t.Parallel()
	l, _ := open(t, vaultDir(t))
	content := bytes.Repeat([]byte(`\"`), 8<<20)
	given := Event{Kind: "marks-replaced", Content: content}
	record(t, l, &given)
	got, _ := events(t, l)
	if len(got) != 1 || !bytes.Equal(got[0].Content, content) {
		t.Fatalf("read %d events back, want the one recorded whole", len(got))
	}
}

// A line longer than its limit is refused, and nothing is written.
func TestALineLongerThanItsLimitIsRefused(t *testing.T) {
	t.Parallel()
	dir := vaultDir(t)
	l, _ := open(t, dir)
	if _, err := l.Record(t.Context(), &Event{Kind: "marks-replaced", Content: bytes.Repeat([]byte("x"), MaxLineBytes)}); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("Record of a line past its limit = %v, want ErrTooLarge", err)
	}
	if _, err := l.Record(t.Context(), &Event{}); err == nil {
		t.Error("Record of a line with no kind = nil, want an error")
	}
	if _, err := l.Record(t.Context(), &Event{Kind: KindMark, Writer: "a\xff"}); err == nil {
		t.Error("Record of a writer that is not UTF-8 = nil, want an error: it would be read back as another")
	}
	if _, err := os.Stat(filepath.Join(dir.Path(), fileName)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a refused line made the ledger: %v", err)
	}
}
