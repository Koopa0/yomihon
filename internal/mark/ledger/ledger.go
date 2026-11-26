// Package ledger is this machine's record of what it wrote for one vault: one
// append-only file, ledger.jsonl, in the vault's directory under the
// configuration directory, outside the vault, where git cannot take it back.
//
// A line is a commit. The code that writes a mark into the vault records the
// line first and writes the vault only once Record has answered, so a write to
// the vault that fails, or a process killed after the line, still leaves the
// line to say what was meant. Each line ends in a checksum of the bytes before
// it, and reading tells three kinds of line apart:
//
//   - a whole line, as it was written, which is an Event;
//   - a line a crash cut short, which was never committed: it is read past,
//     the next line is written on a line of its own, and no later line takes
//     the seq it began to say;
//   - a line that is all there and does not fit its checksum, which something
//     changed after it was written. It is counted and never read as an event,
//     and a compaction moves it, byte for byte, to ledger.damaged.jsonl, where
//     it stays counted. Nothing changed is dropped.
//
// The ledger only grows, by one line a Record, until it is compacted: once it
// is past 8 MiB and twice what a compaction would leave of it, the next Record
// rewrites it whole, through confdir's Replace, keeping every line but the
// mark lines a newer line of the same mark by the same writer supersedes, and
// the changed lines it sets aside. A compaction that cannot run is said in the
// log and is not the failure of the Record that triggered it.
//
// Nothing caps what the ledger keeps: the lines of the files a write replaced
// are kept whole for ever. Past 1 GiB it is not read, and every Record fails.
//
// It is called a ledger and not a journal because the journal is the vault's
// notes by date. Its writes go through confdir, under a lock of its own,
// ledger.lock, so two processes on one vault, or two goroutines of one, append
// and compact one at a time; a caller that holds the lock of what it writes
// into the vault takes that one first. The marks of internal/mark call it.
package ledger

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"slices"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/koopa0/yomihon/internal/mark/confdir"
)

// The files of a vault's ledger, beside its other files under the
// configuration directory.
const (
	fileName    = "ledger.jsonl"
	damagedName = "ledger.damaged.jsonl"
	lockName    = "ledger.lock"
)

const (
	// compactAt is how long the ledger grows before a Record compacts it.
	compactAt = 8 << 20
	// maxBytes is the most of a ledger, or of the lines set aside from one, that
	// is read. Nothing here caps what the ledger keeps, so this is a bound on
	// memory and not a size the ledger is expected to reach.
	maxBytes = 1 << 30
	// MaxLineBytes is the most one line may hold: a machine file of marks as a
	// write replaced it, with room for base64.
	MaxLineBytes = 32 << 20
)

// The kinds of line the ledger itself reads. A line of another kind is kept as
// it is written.
const (
	// KindMark is a line of this machine's file of marks, as a write of it put
	// it there. Content is the line; Writer is the machine id it was written
	// under, and Session the process.
	KindMark = "mark"
	// KindSession stands for a process whose every mark line a compaction
	// dropped, so that what this machine's processes were is never forgotten:
	// ID is the machine id and Session the process.
	KindSession = "session"
)

// ErrTooLarge means a line would be longer than MaxLineBytes.
var ErrTooLarge = errors.New("the line would be longer than its limit")

// Event is one line of the ledger. Record gives it its Seq and its Time; the
// rest is the caller's, and the ledger reads none of it but Kind, Writer,
// Session and, on a mark's line, the id of the mark Content is about.
type Event struct {
	// Seq is the line's place: every line has its own, and they only go up.
	Seq uint64
	// Time is when it was recorded, to the millisecond, for a person to read.
	Time time.Time
	// Kind says what the line is.
	Kind string
	// ID is a machine id, on the lines that are about one.
	ID string
	// Path is this machine's file of marks, on the lines that are about it.
	Path string
	// Sum is the hash of what Content holds, on the lines that keep a file.
	Sum string
	// Writer is the machine id a mark's line was written under.
	Writer string
	// Session is the process that wrote the line.
	Session string
	// Content is the bytes the line keeps, kept exactly. The caller does not
	// change the slice it is handed.
	Content []byte

	raw  []byte // the line as it is in the file
	mark string // the mark a mark's line is about
}

// Ledger is the ledger of one vault. It is safe for use by several goroutines,
// and by several processes over one directory.
type Ledger struct {
	dir       confdir.Dir
	log       *slog.Logger
	compactAt int64
	// lockWait is how long a change waits for another holder of the ledger's
	// lock: confdir.LockWait, and longer only in a test that crowds the lock on
	// purpose and asks whether anything was lost rather than how fast.
	lockWait time.Duration

	// mu orders this process's goroutines, and keeps read the file as last read
	// while one of them appends or compacts under the lock of the file.
	mu   sync.Mutex
	read state
	// failedAt is how long the ledger was when a compaction this process tried
	// could not run: the next waits until it is twice that.
	failedAt int64
	// cleared says this process has taken the lock and removed what a killed
	// compaction left.
	cleared bool
}

// state is the ledger as last read.
type state struct {
	// info is the file as it was when read, and nil when what is held is not to
	// be reused: never read, or not read whole.
	info fs.FileInfo
	// absent is a ledger that was not there when read.
	absent bool
	// events are its whole lines, in the order of the file.
	events []Event
	// changed are its lines that do not fit their checksum.
	changed [][]byte
	// cuts are its lines a crash cut short, each with how many whole lines stand
	// before it, so a compaction keeps them where they were.
	cuts []cutLine
	// aside is how many lines earlier compactions set aside and the ledger no
	// longer holds.
	aside int
	// next is the seq the next line takes.
	next uint64
	// open is a file whose last line lost its newline: the next line begins with
	// one.
	open bool
}

// cutLine is a line a crash cut short, and how many whole lines stand before it.
type cutLine struct {
	after int
	raw   []byte
}

// Open is the ledger of the vault whose directory is dir. It reads and writes
// nothing until it is used. log is where a compaction that cannot run is said.
func Open(dir confdir.Dir, log *slog.Logger) *Ledger {
	if log == nil {
		panic("ledger: Open with no log")
	}
	return &Ledger{dir: dir, log: log, compactAt: compactAt, lockWait: confdir.LockWait}
}

// Record appends e as the next line and syncs it, and answers the seq it took.
// It sets e's Seq and Time, and keeps a copy of Content. Nil means the line is on stable storage: it is the
// commit. An error means the line may be in the ledger whole, in part, or not
// at all, so a caller that records before it writes the vault does not write
// the vault.
func (l *Ledger) Record(e *Event) (uint64, error) {
	if err := e.check(); err != nil {
		return 0, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	var seq uint64
	err := l.exclusive(func() error {
		st, err := l.load()
		if err != nil {
			return err
		}
		// The time as the line keeps it, to the millisecond, so that what is
		// held from this Record and what a later read finds are the same.
		e.Seq, e.Time = st.next, time.UnixMilli(time.Now().UnixMilli())
		line := encode(e)
		if len(line)+2 > MaxLineBytes {
			return fmt.Errorf("%w: %d bytes", ErrTooLarge, len(line))
		}
		data := slices.Concat(line, []byte{'\n'})
		if st.open {
			data = slices.Concat([]byte{'\n'}, data)
		}
		if err = confdir.Append(l.dir, fileName, data); err != nil {
			l.read = state{}
			return fmt.Errorf("record a line of the ledger: %w", err)
		}
		seq = e.Seq
		held := *e
		held.Content, held.raw = bytes.Clone(e.Content), line
		if held.Kind == KindMark {
			held.mark = markOf(held.Content)
		}
		l.extend(&st, &held, int64(len(data)))
		l.compactIfLong()
		return nil
	})
	return seq, err
}

// Events is every whole line of the ledger, in the order of their seqs, and how
// many lines were changed after they were written: those in the ledger now and
// those compactions set aside. It takes no lock: a line being appended as it
// reads is a line cut short to it, and is not there yet. The Content of each
// is shared with what the ledger holds, and is not changed by the caller.
func (l *Ledger) Events() (events []Event, changed int, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	st, err := l.load()
	if err != nil {
		return nil, 0, err
	}
	events = slices.Clone(st.events)
	slices.SortStableFunc(events, func(a, b Event) int { return cmp.Compare(a.Seq, b.Seq) })
	return events, len(st.changed) + st.aside, nil
}

// LatestMarks is the newest line of each mark that writer wrote, by the id of
// the mark: what this machine last wrote of each under that machine id. A mark
// line whose content names no mark is not among them.
func (l *Ledger) LatestMarks(writer string) (map[string]Event, error) {
	events, _, err := l.Events()
	if err != nil {
		return nil, err
	}
	latest := map[string]Event{}
	for i := range events {
		e := &events[i]
		if e.Kind != KindMark || e.Writer != writer || e.mark == "" {
			continue
		}
		if held, ok := latest[e.mark]; !ok || held.Seq < e.Seq {
			latest[e.mark] = *e
		}
	}
	return latest, nil
}

// Compact rewrites the ledger whole now, as a Record does once the ledger is
// long; see compacted for what it keeps.
func (l *Ledger) Compact() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.exclusive(l.compact)
}

// Carry brings the ledger kept for an earlier path of this vault, in from, into
// this one, when this one has no line of its own, whole or changed. The ledger
// is copied byte for byte, so this machine goes on under the machine id and the
// seqs it had there, and the lines set aside there are set aside here beside
// any this one holds, none twice. A ledger that has lines of its own is left as
// it is, and the earlier one is not merged into it: false says nothing was
// carried. from is only read, so the same call can be made again, and one that
// stopped part way is finished by it.
func (l *Ledger) Carry(from confdir.Dir) (bool, error) {
	// Read first, and the lock taken only when there is something to carry, so
	// that a vault with no earlier ledger is left without even a lock file.
	theirs, err := confdir.Read(from, fileName, maxBytes)
	if errors.Is(err, fs.ErrNotExist) || (err == nil && len(theirs) == 0) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read the earlier ledger: %w", err)
	}
	aside, err := confdir.Read(from, damagedName, maxBytes)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, fmt.Errorf("read what the earlier ledger set aside: %w", err)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	carried := false
	err = l.exclusive(func() error {
		st, loadErr := l.load()
		if loadErr != nil {
			return loadErr
		}
		if len(st.events) > 0 || len(st.changed) > 0 {
			return nil
		}
		if asideErr := l.setAside(lines(aside)); asideErr != nil {
			return asideErr
		}
		l.read = state{}
		if replaceErr := confdir.Replace(l.dir, fileName, theirs); replaceErr != nil {
			return fmt.Errorf("carry the earlier ledger: %w", replaceErr)
		}
		carried = true
		return nil
	})
	return carried, err
}

// check refuses an event the ledger would not read back as it was given: one
// with no kind, or with a field other than Content that is not UTF-8, which
// JSON would keep as U+FFFD.
func (e *Event) check() error {
	if e.Kind == "" {
		return errors.New("record a line of the ledger: no kind")
	}
	for _, field := range [...]string{e.Kind, e.ID, e.Path, e.Sum, e.Writer, e.Session} {
		if !utf8.ValidString(field) {
			return fmt.Errorf("record a line of the ledger: %q is not UTF-8", field)
		}
	}
	return nil
}

// exclusive runs change holding the ledger's lock. The first time this process
// holds it, it removes the temporary files a compaction killed part way left:
// every compaction is made under this lock, so none of them is one still
// running.
func (l *Ledger) exclusive(change func() error) error {
	return confdir.Exclusive(l.dir, lockName, l.lockWait, func() error {
		if !l.cleared {
			if err := confdir.RemoveLeftovers(l.dir, fileName); err != nil {
				l.log.Warn("remove what a stopped compaction of the ledger left", "err", err)
			}
			l.cleared = true
		}
		return change()
	})
}

// load is the ledger as it is: what was read last, while the file is the same
// one at the same length and time, and the file read again otherwise.
func (l *Ledger) load() (state, error) {
	info, err := os.Stat(l.dir.Join(fileName)) // #nosec G703 -- a file of this vault's own directory under the configuration directory
	switch {
	case errors.Is(err, fs.ErrNotExist):
		info = nil
	case err != nil:
		return state{}, fmt.Errorf("look at the ledger: %w", err)
	case l.read.info != nil && sameFile(l.read.info, info):
		return l.read, nil
	}
	st := state{absent: info == nil, next: 1}
	if info != nil {
		data, readErr := confdir.Read(l.dir, fileName, maxBytes)
		if readErr != nil {
			return state{}, fmt.Errorf("read the ledger: %w", readErr)
		}
		st.parse(data)
		// What was read is reused only while the file is what was read: one that
		// changed between the look and the read is read again next time.
		after, statErr := os.Stat(l.dir.Join(fileName)) // #nosec G703 -- a file of this vault's own directory under the configuration directory
		if statErr == nil && after.Size() == int64(len(data)) {
			st.info = after
		}
	}
	aside, err := confdir.Read(l.dir, damagedName, maxBytes)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return state{}, fmt.Errorf("read what the ledger set aside: %w", err)
	}
	// A line set aside that the ledger still holds, because a compaction was
	// stopped between setting it aside and installing the ledger, is one line.
	inLedger := map[string]bool{}
	for _, raw := range st.changed {
		inLedger[string(raw)] = true
	}
	for _, raw := range lines(aside) {
		if !inLedger[string(raw)] {
			st.aside++
		}
		st.next = after(st.next, seqOf(raw))
	}
	l.read = st
	return st, nil
}

// parse fills st from the bytes of the ledger.
func (st *state) parse(data []byte) {
	for _, raw := range lines(data) {
		e, v := decode(raw)
		switch v {
		case whole:
			st.events = append(st.events, e)
			st.next = after(st.next, e.Seq)
		case changed:
			st.changed = append(st.changed, raw)
			st.next = after(st.next, seqOf(raw))
		case cut:
			// Never committed, so its seq could be taken again; it is not, so that
			// a line a hand cut short while it kept its newline, which cannot be
			// told from one a crash cut, is never given a second line's seq.
			st.cuts = append(st.cuts, cutLine{after: len(st.events), raw: raw})
			st.next = after(st.next, seqOf(raw))
		}
	}
	st.open = len(data) > 0 && data[len(data)-1] != '\n'
}

// extend makes what is held, st as it was before this process appended
// appended bytes holding e, into the file as it is after, when the file is
// what that append alone made of it; otherwise it is read again next time.
func (l *Ledger) extend(st *state, e *Event, appended int64) {
	l.read = state{}
	if st.info == nil && !st.absent {
		return
	}
	after, err := os.Stat(l.dir.Join(fileName)) // #nosec G703 -- a file of this vault's own directory under the configuration directory
	if err != nil {
		return
	}
	var before int64
	if st.info != nil {
		if !os.SameFile(st.info, after) {
			return
		}
		before = st.info.Size()
	}
	if after.Size() != before+appended {
		return
	}
	st.events = append(st.events, *e)
	st.info, st.absent, st.open, st.next = after, false, false, e.Seq+1
	l.read = *st
}

func sameFile(a, b fs.FileInfo) bool {
	return a.Size() == b.Size() && a.ModTime().Equal(b.ModTime()) && os.SameFile(a, b)
}

// compactIfLong compacts the ledger once it is past compactAt and twice what a
// compaction would leave of it, which is read from the ledger itself, so that a
// ledger whose lines are all kept is not rewritten by every process that
// records into it. A compaction that cannot run is said, and not tried again
// by this process until the ledger has doubled: the line that triggered it is
// committed, and its failure is not that line's.
func (l *Ledger) compactIfLong() {
	info, err := os.Stat(l.dir.Join(fileName)) // #nosec G703 -- a file of this vault's own directory under the configuration directory
	if err != nil || info.Size() <= l.compactAt || info.Size() < 2*l.failedAt {
		return
	}
	st, err := l.load()
	if err != nil || info.Size() < 2*int64(len(compacted(&st))) {
		return
	}
	if err = l.compact(); err != nil {
		l.failedAt = info.Size()
		l.log.Warn("compact the ledger", "bytes", info.Size(), "err", err)
	}
}

// compact sets aside the lines that were changed and installs what compacted
// keeps. The caller holds the lock.
func (l *Ledger) compact() error {
	st, err := l.load()
	if err != nil {
		return err
	}
	if asideErr := l.setAside(st.changed); asideErr != nil {
		return asideErr
	}
	data := compacted(&st)
	l.read = state{}
	if err = confdir.Replace(l.dir, fileName, data); err != nil {
		return fmt.Errorf("compact the ledger: %w", err)
	}
	return nil
}

// compacted is the ledger rewritten from its lines, in their order, each kept
// byte for byte, with what a compaction drops left out: a mark line that a
// later line of the same mark by the same writer supersedes. Lines of a kind
// the ledger does not read, lines of the files a write replaced, and the newest
// line of every mark by every writer are kept, and so the line with the highest
// seq is. A process whose every line is dropped is kept as one session line,
// under the seq of the first of them. A line cut short is kept where it was: it
// is almost always a crash's, but a hand's cannot be told from one, and nothing
// that cannot be read is dropped. The changed lines are not here; compact sets
// them aside.
func compacted(st *state) []byte {
	events, cuts := st.events, st.cuts
	newest := map[[2]string]uint64{}
	for i := range events {
		if e := &events[i]; e.Kind == KindMark && e.mark != "" {
			key := [2]string{e.Writer, e.mark}
			newest[key] = max(newest[key], e.Seq)
		}
	}
	dropped := func(e *Event) bool {
		return e.Kind == KindMark && e.mark != "" && newest[[2]string{e.Writer, e.mark}] != e.Seq
	}
	sessions := map[[2]string]bool{}
	for i := range events {
		if e := &events[i]; !dropped(e) && e.Session != "" {
			sessions[[2]string{cmp.Or(e.Writer, e.ID), e.Session}] = true
		}
	}
	var out bytes.Buffer
	writeCuts := func(at int) {
		for len(cuts) > 0 && cuts[0].after == at {
			out.Write(cuts[0].raw)
			out.WriteByte('\n')
			cuts = cuts[1:]
		}
	}
	for i := range events {
		writeCuts(i)
		e := &events[i]
		line := e.raw
		if dropped(e) {
			session := [2]string{e.Writer, e.Session}
			if e.Session == "" || sessions[session] {
				continue
			}
			sessions[session] = true
			line = encode(&Event{Seq: e.Seq, Time: e.Time, Kind: KindSession, ID: e.Writer, Session: e.Session})
		}
		out.Write(line)
		out.WriteByte('\n')
	}
	writeCuts(len(events))
	return out.Bytes()
}

// setAside appends to ledger.damaged.jsonl each of changed it does not hold
// already, so a compaction killed after it set lines aside and before it
// installed the ledger sets none aside twice. The caller holds the lock.
func (l *Ledger) setAside(changed [][]byte) error {
	if len(changed) == 0 {
		return nil
	}
	held, err := confdir.Read(l.dir, damagedName, maxBytes)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("read what the ledger set aside: %w", err)
	}
	there := map[string]bool{}
	for _, raw := range lines(held) {
		there[string(raw)] = true
	}
	var out bytes.Buffer
	if len(held) > 0 && held[len(held)-1] != '\n' {
		out.WriteByte('\n')
	}
	added := false
	for _, raw := range changed {
		if there[string(raw)] {
			continue
		}
		there[string(raw)] = true
		out.Write(raw)
		out.WriteByte('\n')
		added = true
	}
	if !added {
		return nil
	}
	if err = confdir.Append(l.dir, damagedName, out.Bytes()); err != nil {
		return fmt.Errorf("set aside the changed lines of the ledger: %w", err)
	}
	return nil
}
