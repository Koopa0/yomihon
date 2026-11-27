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
// The ledger reads three things of a line it did not make: its kind, the
// machine and process that wrote it, and, on a mark's line, the id of the mark
// it is about, which the caller gives it beside the content. The content
// itself it keeps as bytes and never reads.
//
// The ledger only grows, by one line a Record, until it is compacted: once it
// is past 8 MiB and twice what a compaction would leave of it, the next Record
// rewrites it whole, through confdir's Replace, keeping every line but the
// mark lines a newer line of the same mark by the same machine supersedes, and
// the changed lines it moves to ledger.damaged.jsonl. A compaction that cannot
// run is said in the log and is not the failure of the Record that triggered
// it.
//
// Nothing caps what the ledger keeps: the lines of the files a write replaced
// are kept whole for ever. Past 1 GiB it is not read, and every Record fails.
//
// It is called a ledger and not a journal because the journal is the vault's
// notes by date. Its writes go through confdir, under a lock of its own,
// ledger.lock, so two processes on one vault, or two goroutines of one, append
// and compact one at a time; a caller that holds the lock of what it writes
// into the vault takes that one first. A change waits for the lock for
// confdir.LockWait. A read takes no lock and waits for no writer: a line being
// appended is a line cut short to it, and a compaction installs the ledger
// whole. internal/mark/machinemarks calls it.
package ledger

import (
	"bytes"
	"cmp"
	"context"
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
	"github.com/koopa0/yomihon/internal/mark/sealedline"
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
	// maxBytes is the most of a ledger, or of the damaged lines moved out of
	// one, that is read. Nothing here caps what the ledger keeps, so this is a bound on
	// memory and not a size the ledger is expected to reach.
	maxBytes = 1 << 30
	// maxLineBytes is the most one line may hold: a machine file of marks as a
	// write replaced it, which is at most 16 MiB and takes at most 4/3 of that
	// however much of it JSON would escape (see encode), with room for the rest
	// of the line.
	maxLineBytes = 32 << 20
)

// The kinds of line the ledger itself reads. A line of another kind is kept as
// it is written.
const (
	// KindMark is a line of this machine's file of marks, as a write of it put
	// it there. Content is the line, Mark the id of the mark it is about,
	// Machine the machine id it was written under, and Process the process.
	KindMark = "mark"
	// KindProcess stands for a process whose every mark line a compaction
	// dropped, so that what this machine's processes were is never forgotten:
	// ID is the machine id and Process the process.
	KindProcess = "session"
)

// errTooLarge means a line would be longer than maxLineBytes.
var errTooLarge = errors.New("the line would be longer than its limit")

// Event is one line of the ledger. Record gives it its Seq and its Time; the
// rest is the caller's, and the ledger reads none of it but Kind, Machine,
// Process and Mark.
type Event struct {
	// Seq is the line's place: every line has its own, and they only go up.
	Seq uint64
	// Time is when it was recorded, to the millisecond, for a person to read.
	Time time.Time
	// Kind says what the line is.
	Kind string
	// ID is a machine id, on the lines that are about one.
	ID string
	// Machine is the machine id a mark's line was written under.
	Machine string
	// Process is the process that wrote the line.
	Process string
	// Mark is the id of the mark a mark's line is about. A mark's line recorded
	// before lines carried it has none, and a compaction keeps it.
	Mark string
	// Content is the bytes the line keeps, kept exactly. The caller does not
	// change the slice it is handed.
	Content []byte

	raw []byte // the line as it is in the file
}

// Ledger is the ledger of one vault. It is safe for use by several goroutines,
// and by several processes over one directory.
type Ledger struct {
	dir       confdir.Dir
	log       *slog.Logger
	compactAt int64

	// mu guards the fields below it, which are what this process holds of the
	// ledger, and nothing else: no file is looked at, read, written or locked
	// while it is held, so a reader never waits behind a writer's wait for the
	// lock, its append, its sync or its compaction. Writers are put one at a
	// time by the ledger's lock, which lets this process's through in the order
	// they asked and excludes another process's as well.
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
	// events are its whole lines, in the order of the file. A state is only
	// ever grown at its end, by the holder of the ledger's lock and under mu, so
	// the events a copy of it holds are never written again and a reader may
	// copy them out after letting mu go.
	events []Event
	// changed are its lines that do not fit their checksum.
	changed [][]byte
	// cuts are its lines a crash cut short, each with how many whole lines stand
	// before it, so a compaction keeps them where they were.
	cuts []cutLine
	// moved is how many damaged lines earlier compactions moved out and the
	// ledger no longer holds, and damagedFile the file they are in as it was
	// when read.
	moved       int
	damagedFile look
	// next is the seq the next line takes.
	next uint64
	// open is a file whose last line lost its newline: the next line begins with
	// one.
	open bool
}

// look is a file as a look at it found it: its info, or none when it was not
// there.
type look struct{ info fs.FileInfo }

// same says two looks found the same file at the same length and time, or both
// found none.
func (a look) same(b look) bool {
	if a.info == nil || b.info == nil {
		return a.info == nil && b.info == nil
	}
	return sameFile(a.info, b.info)
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
	return &Ledger{dir: dir, log: log, compactAt: compactAt}
}

// Record appends e as the next line and syncs it, and answers the seq it took.
// The line has that seq and the time now; e's own Seq and Time are not read,
// and a copy of Content is kept. Nil means the line is on stable storage: it is
// the commit. An error means the line may be in the
// ledger whole, in part, or not at all, so a caller that records before it
// writes the vault does not write the vault. ctx bounds the wait for the
// ledger's lock, as confdir.Acquire says; once the lock is held the line is
// appended whatever ctx says.
func (l *Ledger) Record(ctx context.Context, e *Event) (uint64, error) {
	if err := e.check(); err != nil {
		return 0, err
	}
	var seq uint64
	err := l.exclusive(ctx, func() error {
		st, err := l.load()
		if err != nil {
			return err
		}
		held := *e
		// The time as the line keeps it, to the millisecond, so that what is
		// held from this Record and what a later read finds are the same.
		held.Seq, held.Time = st.next, time.UnixMilli(time.Now().UnixMilli())
		line := encode(&held)
		if len(line)+2 > maxLineBytes {
			return fmt.Errorf("%w: %d bytes", errTooLarge, len(line))
		}
		data := slices.Concat(line, []byte{'\n'})
		if st.open {
			data = slices.Concat([]byte{'\n'}, data)
		}
		if err = confdir.Append(l.dir, fileName, data); err != nil {
			l.forget()
			return fmt.Errorf("record a line of the ledger: %w", err)
		}
		seq = held.Seq
		held.Content, held.raw = bytes.Clone(e.Content), line
		l.extend(&st, &held, int64(len(data)))
		l.compactIfLong()
		return nil
	})
	return seq, err
}

// Events is every whole line of the ledger, in the order of their seqs, and how
// many lines were changed after they were written: those in the ledger now and
// those compactions moved to ledger.damaged.jsonl. It takes no lock and waits
// for no writer, of this process or another: a line being appended as it reads
// is a line cut short to it, and is not there yet, and a compaction installs
// the ledger by rename, so it is read whole before or after. The Content of each is shared
// with what the ledger holds, and is not changed by the caller.
func (l *Ledger) Events() (events []Event, changed int, err error) {
	st, err := l.load()
	if err != nil {
		return nil, 0, err
	}
	events = slices.Clone(st.events)
	slices.SortStableFunc(events, func(a, b Event) int { return cmp.Compare(a.Seq, b.Seq) })
	return events, len(st.changed) + st.moved, nil
}

// Compact rewrites the ledger whole now, as a Record does once the ledger is
// long; see compacted for what it keeps. ctx bounds the wait for the lock.
func (l *Ledger) Compact(ctx context.Context) error {
	return l.exclusive(ctx, l.compact)
}

// Carry brings the ledger kept for an earlier path of this vault, in from, into
// this one, when this one has no line of its own, whole or changed. The ledger
// is copied byte for byte, so this machine goes on under the machine id and the
// seqs it had there, and the damaged lines moved out there are moved out here
// beside any this one holds, none twice. A ledger that has lines of its own is left as
// it is, and the earlier one is not merged into it: false says nothing was
// carried. from is only read, so the same call can be made again, and one that
// stopped part way is finished by it. ctx bounds the wait for the lock.
func (l *Ledger) Carry(ctx context.Context, from confdir.Dir) (bool, error) {
	// Read first, and the lock taken only when there is something to carry, so
	// that a vault with no earlier ledger is left without even a lock file.
	theirs, err := confdir.Read(from, fileName, maxBytes)
	if errors.Is(err, fs.ErrNotExist) || (err == nil && len(theirs) == 0) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read the earlier ledger: %w", err)
	}
	damaged, err := confdir.Read(from, damagedName, maxBytes)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, fmt.Errorf("read the damaged lines of the earlier ledger: %w", err)
	}
	carried := false
	err = l.exclusive(ctx, func() error {
		st, loadErr := l.load()
		if loadErr != nil {
			return loadErr
		}
		if len(st.events) > 0 || len(st.changed) > 0 {
			return nil
		}
		if moveErr := l.moveDamaged(sealedline.Lines(damaged)); moveErr != nil {
			return moveErr
		}
		if replaceErr := confdir.Replace(l.dir, fileName, theirs); replaceErr != nil {
			return fmt.Errorf("carry the earlier ledger: %w", replaceErr)
		}
		l.forget()
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
	for _, field := range [...]string{e.Kind, e.ID, e.Machine, e.Process, e.Mark} {
		if !utf8.ValidString(field) {
			return fmt.Errorf("record a line of the ledger: %q is not UTF-8", field)
		}
	}
	return nil
}

// exclusive runs change holding the ledger's lock, waiting for it as ctx and
// confdir.LockWait allow. The first time this process holds it, it removes the
// temporary files a compaction killed part way left: every compaction is made
// under this lock, so none of them is one still running.
func (l *Ledger) exclusive(ctx context.Context, change func() error) error {
	return confdir.Exclusive(ctx, l.dir, lockName, confdir.LockWait, func() error {
		l.mu.Lock()
		cleared := l.cleared
		l.mu.Unlock()
		if !cleared {
			if err := confdir.RemoveLeftovers(l.dir, fileName); err != nil {
				l.log.Warn("remove what a stopped compaction of the ledger left", "error", err)
			}
			l.mu.Lock()
			l.cleared = true
			l.mu.Unlock()
		}
		return change()
	})
}

// load is the ledger as it is: what was read last, while the ledger and the
// file of its damaged lines are each the same one at the same length and
// time, and both read again otherwise. Only the look at what is held and the
// keeping of what was read are made under l.mu; the files are looked at and
// read outside it.
func (l *Ledger) load() (state, error) {
	info, err := confdir.Stat(l.dir, fileName)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		info = nil
	case err != nil:
		return state{}, fmt.Errorf("look at the ledger: %w", err)
	}
	damagedFile, err := l.lookDamaged()
	if err != nil {
		return state{}, err
	}
	if held, ok := l.heldFor(info, damagedFile); ok {
		return held, nil
	}
	st := state{absent: info == nil, next: 1}
	if info != nil {
		if readErr := l.readLedger(&st, info); readErr != nil {
			return state{}, readErr
		}
	}
	if damagedErr := l.readDamaged(&st, damagedFile); damagedErr != nil {
		return state{}, damagedErr
	}
	l.hold(&st)
	return st, nil
}

// readLedger fills st from the ledger, which a look found as info.
func (l *Ledger) readLedger(st *state, info fs.FileInfo) error {
	data, err := confdir.Read(l.dir, fileName, maxBytes)
	if err != nil {
		return fmt.Errorf("read the ledger: %w", err)
	}
	st.parse(data)
	// What was read is reused only while the file is what was read: one that
	// changed or was replaced between the look and the end of the read is read
	// again next time.
	after, err := confdir.Stat(l.dir, fileName)
	if err == nil && sameFile(info, after) && after.Size() == int64(len(data)) {
		st.info = after
	}
	return nil
}

// readDamaged counts in st the damaged lines moved out of the ledger, from
// their file, which a look found as damagedFile.
func (l *Ledger) readDamaged(st *state, damagedFile look) error {
	damaged, err := confdir.Read(l.dir, damagedName, maxBytes)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("read the damaged lines of the ledger: %w", err)
	}
	// A damaged line moved out that the ledger still holds, because a
	// compaction was stopped between moving it and installing the ledger, is one
	// line.
	inLedger := map[string]bool{}
	for _, raw := range st.changed {
		inLedger[string(raw)] = true
	}
	// A last line with no newline is one a compaction is still appending, or
	// one a crash cut short as it appended: either way the line it copies is in
	// the ledger still, and counted there, and the next moveDamaged ends it on a
	// line of its own before it adds anything.
	for _, raw := range sealedline.Lines(ended(damaged)) {
		if !inLedger[string(raw)] {
			st.moved++
		}
		st.next = after(st.next, seqOf(raw))
	}
	// The damaged lines are counted in what is held, and a compaction that
	// stopped between moving them and installing the ledger, in this process or
	// another, leaves the ledger as it was: what was read is reused only while
	// that file too is what was read.
	if damagedAfter, lookErr := l.lookDamaged(); lookErr != nil || !damagedAfter.same(damagedFile) {
		st.info = nil
	}
	st.damagedFile = damagedFile
	return nil
}

// ended is data up to the end of its last newline: its lines that are whole.
func ended(data []byte) []byte {
	return data[:bytes.LastIndexByte(data, '\n')+1]
}

// lookDamaged looks at the file of the damaged lines.
func (l *Ledger) lookDamaged() (look, error) {
	info, err := confdir.Stat(l.dir, damagedName)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return look{}, nil
	case err != nil:
		return look{}, fmt.Errorf("look at the damaged lines of the ledger: %w", err)
	}
	return look{info: info}, nil
}

// heldFor is what this process holds of the ledger, when it was read from the
// ledger info describes and the file of its damaged lines damagedFile
// describes; a ledger that is not there is always read again.
func (l *Ledger) heldFor(info fs.FileInfo, damagedFile look) (state, bool) {
	if info == nil {
		return state{}, false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.read.info == nil || !sameFile(l.read.info, info) || !l.read.damagedFile.same(damagedFile) {
		return state{}, false
	}
	return l.read, true
}

// hold keeps st as what this process holds of the ledger.
func (l *Ledger) hold(st *state) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.read = *st
}

// forget drops what this process holds of the ledger, which is read again next
// time.
func (l *Ledger) forget() {
	l.hold(&state{})
}

// parse fills st from the bytes of the ledger.
func (st *state) parse(data []byte) {
	for _, raw := range sealedline.Lines(data) {
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
	if st.info == nil && !st.absent {
		l.forget()
		return
	}
	after, err := confdir.Stat(l.dir, fileName)
	if err != nil {
		l.forget()
		return
	}
	var before int64
	if st.info != nil {
		if !os.SameFile(st.info, after) {
			l.forget()
			return
		}
		before = st.info.Size()
	}
	if after.Size() != before+appended {
		l.forget()
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
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
	l.mu.Lock()
	failedAt := l.failedAt
	l.mu.Unlock()
	info, err := confdir.Stat(l.dir, fileName)
	if err != nil || info.Size() <= l.compactAt || info.Size() < 2*failedAt {
		return
	}
	st, err := l.load()
	if err != nil || info.Size() < 2*int64(len(compacted(&st))) {
		return
	}
	if err = l.compact(); err != nil {
		l.mu.Lock()
		l.failedAt = info.Size()
		l.mu.Unlock()
		l.log.Warn("compact the ledger", "bytes", info.Size(), "error", err)
	}
}

// compact moves the lines that were changed to ledger.damaged.jsonl and
// installs what compacted keeps. The caller holds the lock. What this process
// holds is let go only once the new ledger is installed: until then a reader of
// this process is answered from it and does not open the ledger the rename
// replaces, which on Windows would refuse the rename. A look tells the new file
// from the old by its identity, but on Windows it reads that identity by the
// name, so there letting go is what says the old one is gone; Carry does the
// same.
func (l *Ledger) compact() error {
	st, err := l.load()
	if err != nil {
		return err
	}
	if moveErr := l.moveDamaged(st.changed); moveErr != nil {
		return moveErr
	}
	data := compacted(&st)
	if err = confdir.Replace(l.dir, fileName, data); err != nil {
		return fmt.Errorf("compact the ledger: %w", err)
	}
	l.forget()
	return nil
}

// compacted is the ledger rewritten from its lines, in their order, each kept
// byte for byte, with what a compaction drops left out: a mark line that a
// later line of the same mark by the same machine supersedes. Lines of a kind
// the ledger does not read, lines of the files a write replaced, a mark's line
// that names no mark, and the newest line of every mark by every machine are
// kept, and so the line with the highest seq is. A process whose every line is
// dropped is kept as one process line, under the seq of the first of them. A
// line cut short is kept where it was: it is almost always a crash's, but a
// hand's cannot be told from one, and nothing that cannot be read is dropped.
// The changed lines are not here; compact moves them out.
func compacted(st *state) []byte {
	events, cuts := st.events, st.cuts
	newest := map[[2]string]uint64{}
	for i := range events {
		if e := &events[i]; e.Kind == KindMark && e.Mark != "" {
			key := [2]string{e.Machine, e.Mark}
			newest[key] = max(newest[key], e.Seq)
		}
	}
	dropped := func(e *Event) bool {
		return e.Kind == KindMark && e.Mark != "" && newest[[2]string{e.Machine, e.Mark}] != e.Seq
	}
	processes := map[[2]string]bool{}
	for i := range events {
		if e := &events[i]; !dropped(e) && e.Process != "" {
			processes[[2]string{cmp.Or(e.Machine, e.ID), e.Process}] = true
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
			process := [2]string{e.Machine, e.Process}
			if e.Process == "" || processes[process] {
				continue
			}
			processes[process] = true
			line = encode(&Event{Seq: e.Seq, Time: e.Time, Kind: KindProcess, ID: e.Machine, Process: e.Process})
		}
		out.Write(line)
		out.WriteByte('\n')
	}
	writeCuts(len(events))
	return out.Bytes()
}

// moveDamaged appends to ledger.damaged.jsonl each of changed it does not hold
// already, so a compaction killed after it moved lines there and before it
// installed the ledger moves none twice. The caller holds the lock.
func (l *Ledger) moveDamaged(changed [][]byte) error {
	if len(changed) == 0 {
		return nil
	}
	held, err := confdir.Read(l.dir, damagedName, maxBytes)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("read the damaged lines of the ledger: %w", err)
	}
	there := map[string]bool{}
	for _, raw := range sealedline.Lines(held) {
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
		return fmt.Errorf("move the changed lines of the ledger out: %w", err)
	}
	return nil
}
