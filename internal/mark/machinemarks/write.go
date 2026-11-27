package machinemarks

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"log/slog"

	"github.com/koopa0/yomihon/internal/mark/confdir"
	"github.com/koopa0/yomihon/internal/mark/ledger"
	"github.com/koopa0/yomihon/internal/mark/machinefile"
	"github.com/koopa0/yomihon/internal/mark/sealedline"
)

// lockName is the lock a write of this machine's marks is made under, in the
// vault's directory under the configuration directory beside the ledger. The
// ledger takes its own lock inside it, so the order is always this one first.
const lockName = "machine-file.lock"

// The bounds a machine file is held to, so that every machine can read what any
// machine wrote.
const (
	// MaxLineBytes is the most one line may hold.
	MaxLineBytes = 64 << 10
	// MaxLines is the most lines this machine's file may hold, deleted marks
	// among them: a deleted mark's line stays, so that a copy of the file from
	// before the deletion does not bring the mark back.
	MaxLines = 20000
	// MaxLive is the most marks, of every machine, that are not deleted.
	MaxLive = 5000
	// maxEditBytes bounds the id of an editing session.
	maxEditBytes = 64
)

// The refusals of a change. Each leaves the vault and the ledger as they were.
var (
	// ErrNoBase means a mark that exists was changed with no word of what the
	// page had read of it.
	ErrNoBase = errors.New("a change of a mark must say which lines of it the page had read")
	// ErrStale means this machine wrote the mark since the page read it, so the
	// change was made on a version that is no longer the latest. The error is a
	// *StaleError and holds what is read now.
	ErrStale = errors.New("this machine wrote the mark since the page read it")
	// ErrSuperseded means an earlier send of an editing session arrived after a
	// later one that is written already. Nothing is written, and the page has
	// nothing to say: the newer words are in.
	ErrSuperseded = errors.New("a later send of the same edit is written already")
	// ErrGone means the mark is not there to change: it was never written, or
	// every value of it is a deletion.
	ErrGone = errors.New("the mark is not there")
	// ErrInvalidMark means the mark is not a shape the file holds: a highlight
	// is a passage and has its quote, and a mark has an id, a kind and a note.
	ErrInvalidMark = errors.New("the mark is not a shape the machine file can hold")
	// ErrLimit means a bound of the files was reached: a line, the lines of this
	// machine's file, or the marks that are not deleted.
	ErrLimit = errors.New("a limit of the machine files was reached")
	// ErrNotInVault means the change is recorded in the ledger, whole, and was
	// not written to the vault. The marks it shows are the ledger's until
	// Restore, or the next write, puts them there.
	ErrNotInVault = errors.New("the change is kept in the ledger and is not in the vault")
	// ErrLegacy means the earlier file of marks could not be read, so that it
	// could not be taken in, and nothing is written until it can.
	ErrLegacy = errors.New("the earlier file of marks cannot be taken in")
)

// StaleError is ErrStale with what the mark's lines say now, which the page
// reads from and may write again on top of, if the reader chooses to.
type StaleError struct {
	Seen Seen
}

func (*StaleError) Error() string { return ErrStale.Error() }

// Is says a StaleError is ErrStale.
func (*StaleError) Is(target error) bool { return target == ErrStale }

// Options are what Open takes beyond the places.
type Options struct {
	// Legacy gives the marks of the earlier file, which the first write of a
	// machine with no file of marks takes in. Nil takes in nothing.
	Legacy func() ([]Mark, error)
	// Now is the time, for tests. Nil is the clock.
	Now func() time.Time
	// LockWait is how long a change waits for the vault's lock, and the ledger
	// for its own. Zero is confdir.LockWait, which is what every caller but a
	// test that crowds the lock on purpose uses: such a test asks whether
	// anything is lost, and not how fast the lock comes free.
	LockWait time.Duration
}

// Marks is this machine's hand on the marks of one vault: every read and every
// change of them. It is safe for use by several goroutines and by several
// processes over one vault; a change takes the vault's lock for its whole. A
// change takes a context, which bounds its waits for the vault's lock and the
// ledger's as confdir.Acquire says; a change of this process already under way
// is waited for whatever the context says. A read waits for no change.
type Marks struct {
	folder *machinefile.Folder
	dir    confdir.Dir
	ledger *ledger.Ledger
	opts   Options
	// lockWait is how long a change waits for the vault's lock.
	lockWait time.Duration
	// maxLive and maxLines are MaxLive and MaxLines; a test sets them lower.
	maxLive, maxLines int

	// mu orders the changes of this process, and holds issued.
	mu sync.Mutex
	// issued is the highest rev this process has handed out. No restore of the
	// ledger takes it back.
	issued uint64
	// session is this process's code, made by its first write.
	session atomic.Pointer[string]
}

// Open is the marks of the vault whose reader folder is folder, kept in dir,
// the vault's directory under the configuration directory, which is where its
// ledger is. It reads and writes nothing.
func Open(folder *machinefile.Folder, dir confdir.Dir, log *slog.Logger, opts Options) *Marks {
	if folder == nil {
		panic("machinemarks: Open with no folder")
	}
	wait := opts.LockWait
	if wait <= 0 {
		wait = confdir.LockWait
	}
	return &Marks{folder: folder, dir: dir, ledger: ledger.OpenWaiting(dir, log, wait), opts: opts, lockWait: wait, maxLive: MaxLive, maxLines: MaxLines}
}

func (m *Marks) now() time.Time {
	if m.opts.Now != nil {
		return m.opts.Now()
	}
	return time.Now()
}

func (m *Marks) currentSession() string {
	if s := m.session.Load(); s != nil {
		return *s
	}
	return ""
}

// Timing is what a change took, for the write log. It holds no path and no
// words.
type Timing struct {
	// LockWait is how long the vault's lock was waited for.
	LockWait time.Duration
	// Ledger is the lines recorded, a compaction among them if one ran.
	Ledger time.Duration
	// Write, Install and ReadBack are the file's own: see machinefile.
	Write, Install, ReadBack time.Duration
	Total                    time.Duration
	// Bytes is the length of the file written.
	Bytes int
}

// Result is what a change did.
type Result struct {
	Timing

	// Rev is the line written, and 0 for a change that wrote none.
	Rev uint64
	// Seen is what the page has read of the mark once the change is made: what
	// the line written says it read, and the line itself. It is the base of the
	// next change of the same page, and it does not hold a line the page was
	// never shown, since a change made on top of it would cover that line.
	Seen Seen
}

// proposal is what a change asks to write.
type proposal struct {
	mark Mark
	// base is what the page had read of the mark, and edit its session.
	base Seen
	edit *Edit
	// restore says to write no line, only what is held.
	restore bool
	// fresh is the mark asked for is a new one, or a deleted one made again:
	// the marks that are not deleted grow by one.
	grows bool
	// noWrite says the change finds nothing to write: the answer is base, and
	// the vault and the ledger are left as they are.
	noWrite bool
}

// Put writes mark as the whole new value of the mark of its ID: a new mark, or
// a change of one the page had read, which base says how far. A change of state
// is a Put of the same ID with another Kind. edit names the page's editing
// session, nil when it has none. A mark that exists and the gate withholds is
// ErrGone, which is what Delete and Choose answer for a mark that is not there;
// Put of an id that is not there is a new mark, so for it the answer does tell
// the two apart, and only a caller who already holds the id of a withheld mark
// can ask. The gate is not optional and a nil one panics.
func (m *Marks) Put(ctx context.Context, mark *Mark, base Seen, edit *Edit, gate Gate) (Result, error) {
	needGate(gate)
	if err := validate(mark, edit); err != nil {
		return Result{}, err
	}
	return m.change(ctx, func(v *view) (*proposal, error) {
		if withheld(v, mark.ID, gate) {
			return nil, ErrGone
		}
		return &proposal{mark: *mark, base: base, edit: edit, grows: !live(v, mark.ID)}, nil
	})
}

// Delete writes the mark as deleted, from the value that counts now. It is
// ErrGone if there is no such mark or it is deleted already, which is what a
// page that had not heard of another machine's deletion is told. A mark the
// gate withholds is ErrGone too.
func (m *Marks) Delete(ctx context.Context, id string, base Seen, edit *Edit, gate Gate) (Result, error) {
	needGate(gate)
	if err := validateEdit(edit); err != nil {
		return Result{}, err
	}
	return m.change(ctx, func(v *view) (*proposal, error) {
		r, ok := v.byMark[id]
		if !ok || r.coveredHead(base).Mark.Deleted || withheld(v, id, gate) {
			return nil, ErrGone
		}
		// What is deleted is the value the page read, which the base says: a
		// value another computer wrote that the page never read is not the page's
		// to delete, and is still a value of the mark after.
		mark := r.coveredHead(base).Mark
		mark.Deleted = true
		return &proposal{mark: mark, base: base, edit: edit}, nil
	})
}

// Pick names one value of a mark: the line of one writer at one rev.
type Pick struct {
	Writer string
	Rev    uint64
}

// Choose makes the value pick names the mark's value, written whole on top of
// what the page had read. Only a value the mark's conflict holds now can be
// chosen; one that was replaced since, or that another page already chose, is
// ErrStale and nothing is written. A line that appeared after the page read
// stays beside the chosen one. A mark the gate withholds is ErrGone.
func (m *Marks) Choose(ctx context.Context, id string, pick Pick, base Seen, gate Gate) (Result, error) {
	needGate(gate)
	return m.change(ctx, func(v *view) (*proposal, error) {
		r, ok := v.byMark[id]
		if !ok || withheld(v, id, gate) {
			return nil, ErrGone
		}
		for _, l := range r.heads {
			if l.Writer() == pick.Writer && l.Rev == pick.Rev {
				if err := validate(&l.Mark, nil); err != nil {
					return nil, err
				}
				return &proposal{mark: l.Mark, base: base}, nil
			}
		}
		return nil, &StaleError{Seen: r.entry(func(*kept) bool { return false }).Seen}
	})
}

// needGate refuses a write that names no gate: it would answer for a mark the
// reader's rules hold back.
func needGate(gate Gate) {
	if gate == nil {
		panic("machinemarks: a write with no gate")
	}
}

// withheld says the gate refuses a value of the mark. A write to one is
// answered as a write to a mark that is not there, so that nothing of it, not
// that it exists, nor its writers and revs, is told by the answer.
func withheld(v *view, id string, gate Gate) bool {
	r, ok := v.byMark[id]
	return ok && slices.ContainsFunc(r.heads, func(l *kept) bool { return !gate(&l.Mark) })
}

// Restore writes this machine's file as the ledger and the file together say it
// is, which puts back what git took out of the working tree. It writes nothing
// when the file is that already.
func (m *Marks) Restore(ctx context.Context) (Result, error) {
	return m.change(ctx, func(v *view) (*proposal, error) {
		return &proposal{restore: true}, nil
	})
}

// NoticeNewID records that the reader was told this machine took a new id, so
// that it is said once.
func (m *Marks) NoticeNewID(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Looked at before the lock is taken, so that a machine with nothing to say
	// leaves not even a lock file behind; the look is made again under it.
	if st, err := m.machine(); err != nil || !st.unsaid {
		return err
	}
	return confdir.Exclusive(ctx, m.dir, lockName, m.lockWait, func() error {
		st, err := m.machine()
		if err != nil || !st.unsaid {
			return err
		}
		_, err = m.ledger.Record(ctx, &ledger.Event{Kind: kindSaid, ID: st.id})
		return err
	})
}

// machine is what the ledger says of this machine now.
func (m *Marks) machine() (machine, error) {
	events, _, err := m.ledger.Events()
	if err != nil {
		return machine{}, err
	}
	return stateOf(events), nil
}

// live says the mark is one that is not deleted.
func live(v *view, id string) bool {
	r, ok := v.byMark[id]
	return ok && !r.heads[0].Mark.Deleted
}

// validate refuses what the file would not hold, and what it could not read
// back as it was given.
func validate(m *Mark, edit *Edit) error {
	if m == nil {
		return fmt.Errorf("%w: no mark", ErrInvalidMark)
	}
	if err := validateEdit(edit); err != nil {
		return err
	}
	if err := m.shape(); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidMark, err)
	}
	return nil
}

func validateEdit(e *Edit) error {
	if e != nil && (e.ID == "" || len(e.ID) > maxEditBytes || !utf8.ValidString(e.ID)) {
		return fmt.Errorf("%w: the edit has no usable id", ErrInvalidMark)
	}
	return nil
}

// change is every write: it takes the vault's lock, brings this machine's id, a
// conversion and a clone's retirement about, lets plan say what to write, and
// writes it.
func (m *Marks) change(ctx context.Context, plan func(*view) (*proposal, error)) (Result, error) {
	return m.run(ctx, false, plan)
}

// run is change, and says whether the earlier file of marks is to be taken in
// again whether or not it was, which is what an adoption asks for.
func (m *Marks) run(ctx context.Context, takeIn bool, plan func(*view) (*proposal, error)) (Result, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	start := time.Now()
	var res Result
	err := confdir.Exclusive(ctx, m.dir, lockName, m.lockWait, func() error {
		res.LockWait = time.Since(start)
		return m.locked(ctx, takeIn, plan, &res)
	})
	res.Total = time.Since(start)
	if errors.Is(err, ErrNotInVault) {
		// The change is in the ledger, which is the commit point: the caller is
		// told what the change is, with the failure, and decides what to say.
		return res, err
	}
	if err != nil {
		return Result{Timing: res.Timing}, err
	}
	return res, nil
}

// locked is change inside the lock.
func (m *Marks) locked(ctx context.Context, takeIn bool, plan func(*view) (*proposal, error), res *Result) error {
	ledgerTime := time.Duration(0)
	record := func(e *ledger.Event) error {
		began := time.Now()
		_, err := m.ledger.Record(ctx, e)
		ledgerTime += time.Since(began)
		return err
	}
	defer func() { res.Ledger = ledgerTime }()
	if m.session.Load() == nil {
		s := randomID()
		m.session.Store(&s)
	}
	v, err := m.load()
	if err != nil {
		return err
	}
	if v.ownUnreadable {
		// A write replaces the whole file, so one that had not read it would
		// throw away what it holds: nothing is written until it can be read.
		return fmt.Errorf("%w: %w", ErrOwnFileUnreadable, v.ownErr)
	}
	if v, err = m.settle(v, takeIn, record); err != nil {
		return err
	}
	p, err := plan(v)
	if err != nil {
		return err
	}
	if p.noWrite {
		res.Seen = maps.Clone(p.base)
		if res.Seen == nil {
			res.Seen = Seen{}
		}
		return nil
	}
	return m.commit(v, p, record, res)
}

// settle brings about what a write needs before the write: an id for this
// machine, the retirement of an id another machine has a copy of, and the
// taking in of the earlier file. It answers the view as it is after.
func (m *Marks) settle(v *view, takeIn bool, record func(*ledger.Event) error) (*view, error) {
	// apply records events and reads again what they changed.
	apply := func(events ...*ledger.Event) error {
		for _, e := range events {
			if err := record(e); err != nil {
				return err
			}
		}
		var err error
		v, err = m.load()
		return err
	}
	if v.st.id == "" {
		if err := apply(&ledger.Event{Kind: kindHeader, ID: v.newID()}); err != nil {
			return nil, err
		}
	}
	if v.clone {
		// A line stands under this machine's id that none of its processes
		// wrote: another machine has the same id, a copy of this one's
		// configuration. This one leaves the id, and its old lines stay its own.
		err := apply(&ledger.Event{Kind: kindRetire, ID: v.st.id}, &ledger.Event{Kind: kindHeader, ID: v.newID()})
		if err != nil {
			return nil, err
		}
	}
	if m.opts.Legacy != nil && (takeIn || (!v.st.converted && !v.ownFound)) {
		if err := m.convert(v, takeIn, record); err != nil {
			return nil, err
		}
		if err := apply(); err != nil {
			return nil, err
		}
	}
	return v, nil
}

// convert takes in the marks of the earlier file, once: as one line of the
// ledger holding every line, so that a conversion is one commit and one sync
// however many marks there are. The earlier file is only read.
//
// takeIn is a conversion made after the first, which takes in only the marks of
// the earlier file that no line of any machine and no earlier conversion has: a
// mark that was taken in and then deleted has its deletion in the vault and is
// not brought back, and one that was set aside as not fitting a line is not set
// aside again.
func (m *Marks) convert(v *view, takeIn bool, record func(*ledger.Event) error) error {
	legacy, err := m.opts.Legacy()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrLegacy, err)
	}
	var batch []byte
	t := m.now().UnixMilli()
	for i := range legacy {
		mark := legacy[i]
		if _, there := v.byMark[mark.ID]; there || (takeIn && v.st.skipped[mark.ID]) {
			continue
		}
		raw, why := m.legacyLine(v, &mark, t)
		if why != nil {
			// It stays in the earlier file, which is never changed, and does
			// not keep every later write from going through.
			if err := skip(record, v, &mark, why); err != nil {
				return err
			}
			continue
		}
		batch = append(append(batch, raw...), '\n')
	}
	if takeIn && len(batch) == 0 {
		return nil
	}
	return record(&ledger.Event{Kind: kindConverted, Writer: v.st.id, Session: m.currentSession(), Content: batch})
}

// legacyLine is the line a mark of the earlier file becomes, or why it cannot.
func (m *Marks) legacyLine(v *view, mark *Mark, t int64) ([]byte, error) {
	if err := validate(mark, nil); err != nil {
		return nil, err
	}
	line := Line{Fmt: Format, M: v.st.id, Rev: m.nextRev(v), T: t, Mark: *mark, S: m.currentSession()}
	raw, err := line.encode()
	if err != nil {
		return nil, err
	}
	if len(raw) > MaxLineBytes {
		return nil, fmt.Errorf("a line of %d bytes", len(raw))
	}
	return raw, nil
}

// skip records that a mark of the earlier file was not taken in.
func skip(record func(*ledger.Event) error, v *view, mark *Mark, why error) error {
	content := []byte(mark.ID + " " + why.Error())
	if !isID(mark.ID) {
		content = []byte(strings.Repeat("0", 32) + " " + why.Error())
	}
	err := record(&ledger.Event{Kind: kindSkipped, ID: v.st.id, Content: content})
	if err != nil {
		return fmt.Errorf("record a mark of the earlier file that was not taken in: %w", err)
	}
	return nil
}

// nextRev is the rev of the next line this process writes: above the ledger's
// next seq, above every rev under this machine's id in the vault and every rev
// of this machine's that another machine says it had read, and above any this
// process has issued. The last two are the ones a ledger put back to an older
// copy, or lines an agent's replace took out of the file, cannot take back.
func (m *Marks) nextRev(v *view) uint64 {
	rev := max(v.st.next, v.topOwn+1, v.readOfMe+1, m.issued+1)
	m.issued = rev
	return rev
}

// newID is an id for this machine that no file of the vault carries.
func (v *view) newID() string {
	for {
		if id := randomID(); !v.ids[id] && !v.st.owns(id) {
			return id
		}
	}
}

func randomID() string {
	var b [4]byte
	_, _ = rand.Read(b[:]) // crypto/rand.Read never returns an error
	return hex.EncodeToString(b[:])
}

// commit writes the proposal: the line to the ledger, which is the commit, and
// then the file to the vault.
func (m *Marks) commit(v *view, p *proposal, record func(*ledger.Event) error, res *Result) error {
	own := make(map[string][]*kept, len(v.own)+1)
	maps.Copy(own, v.own)
	var line *kept
	if !p.restore {
		var err error
		if line, err = m.prepare(v, p); err != nil {
			return err
		}
		own[p.mark.ID] = append([]*kept{line}, slices.DeleteFunc(slices.Clone(v.own[p.mark.ID]), func(ol *kept) bool {
			return replaces(&line.Line, &ol.Line)
		})...)
		res.Rev = line.Rev
		// What the page has read, and no more: what the line says it read, and
		// the line itself. A line that came in while the page was open is not
		// here, so a send made on top of this answer does not cover it.
		res.Seen = maps.Clone(line.Seen)
		if res.Seen == nil {
			res.Seen = Seen{}
		}
		res.Seen[line.Writer()] = line.Rev
	} else if v.held() && v.missing == 0 {
		res.Seen = Seen{}
		return nil
	}
	data := v.canonical(own)
	if len(data) > machinefile.MaxFileBytes {
		return fmt.Errorf("%w: this machine's file would be %d bytes", ErrLimit, len(data))
	}
	if n := len(sealedline.Lines(data)); n > m.maxLines {
		return fmt.Errorf("%w: this machine's file would hold %d lines", ErrLimit, n)
	}
	if p.grows && v.live() >= m.maxLive {
		return fmt.Errorf("%w: %d marks are kept", ErrLimit, v.live())
	}
	if err := m.keep(v, line, setAside(v, p, line), record); err != nil {
		return err
	}
	measured, err := m.folder.Replace(v.st.id, data)
	res.Write, res.Install, res.ReadBack, res.Bytes = measured.Write, measured.Install, measured.ReadBack, measured.Bytes
	if err != nil {
		if line == nil {
			return err
		}
		return fmt.Errorf("%w: %w", ErrNotInVault, err)
	}
	return nil
}

// setAside is the lines of this machine's own that line replaces and that held a
// live value which neither line nor the value the page built on is: the case of a
// mark that two computers changed, of which the reader is shown the newest, when
// a page that read it changes that one. By the rule that a writer's newer line
// always replaces its older, the older would be in no file and no ledger.
func setAside(v *view, p *proposal, line *kept) []*kept {
	if line == nil {
		return nil
	}
	id := p.mark.ID
	r, shown := v.byMark[id]
	if !shown {
		return nil
	}
	built := r.coveredHead(p.base).Mark
	var out []*kept
	for _, ol := range v.own[id] {
		if !replaces(&line.Line, &ol.Line) || ol.Mark.Deleted {
			continue
		}
		if !sameValue(&ol.Mark, &line.Mark) && !sameValue(&ol.Mark, &built) {
			out = append(out, ol)
		}
	}
	return out
}

// sameValue says two marks of one id say the same: the words, the resolution,
// the deletion and the place.
func sameValue(a, b *Mark) bool {
	return a.Body == b.Body && a.Resolved.Equal(b.Resolved) && a.Deleted == b.Deleted &&
		a.Kind == b.Kind && a.Path == b.Path && a.Anchor == b.Anchor && a.Quote == b.Quote &&
		a.Before == b.Before && a.After == b.After
}

// keep records in the ledger what is to be kept before the file is replaced:
// the file as it is when it is not what this machine last wrote, then the line.
func (m *Marks) keep(v *view, line *kept, aside []*kept, record func(*ledger.Event) error) error {
	if !v.held() && len(v.ownDisk) > 0 {
		// What is in the vault is not what this machine last wrote. It is about
		// to be replaced, so it is kept first, whole.
		err := record(&ledger.Event{
			Kind: kindReplaced, ID: v.st.id, Session: m.currentSession(), Content: v.ownDisk,
		})
		if err != nil {
			return fmt.Errorf("keep the file about to be replaced: %w", err)
		}
	}
	// Each line the new one replaces that held a value nobody was shown is kept
	// whole before the line that replaces it, which is the commit point: a kill
	// between the two leaves the value kept twice at worst, never not at all.
	for _, ol := range aside {
		err := record(&ledger.Event{Kind: kindSetAside, Writer: ol.Writer(), Session: m.currentSession(), Content: ol.raw})
		if err != nil {
			return fmt.Errorf("set a value aside: %w", err)
		}
	}
	if line != nil {
		err := record(&ledger.Event{Kind: ledger.KindMark, Writer: v.st.id, Session: m.currentSession(), Content: line.raw})
		if err != nil {
			return fmt.Errorf("record the line: %w", err)
		}
	}
	return nil
}

// prepare checks the proposal against what is held and makes its line, and
// takes its rev. Nothing is written.
func (m *Marks) prepare(v *view, p *proposal) (*kept, error) {
	id := p.mark.ID
	r, shown := v.byMark[id]
	if shown {
		if err := checkBase(v, p); err != nil {
			return nil, err
		}
	}
	line := Line{Fmt: Format, M: v.st.id, Rev: m.nextRev(v), T: m.now().UnixMilli(), Mark: p.mark, Edit: p.edit, S: m.currentSession()}
	self := line.Writer()
	for w, n := range p.base {
		// A line says it had read no more of another's than there is, and no
		// more than the page had: a claim of more would cover a line written
		// later, which nobody had read.
		if newest, ok := r.newest[w]; ok && w != self && min(n, newest.Rev) > 0 {
			if line.Seen == nil {
				line.Seen = Seen{}
			}
			line.Seen[w] = min(n, newest.Rev)
		}
	}
	foldSeen(&line, v.own[id])
	raw, err := line.encode()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidMark, err)
	}
	if len(raw) > MaxLineBytes {
		return nil, fmt.Errorf("%w: a line of %d bytes", ErrLimit, len(raw))
	}
	return &kept{Line: line, raw: raw}, nil
}

// foldSeen adds to a new line what the lines of this machine that it replaces
// had read. Those lines are not kept, in the file or in the ledger, so a line
// of another writer that only they covered would be a value of the mark again
// when git put an old file back; the new line says it had read them too.
func foldSeen(line *Line, replaced []*kept) {
	self := line.Writer()
	for _, ol := range replaced {
		if !replaces(line, &ol.Line) {
			continue
		}
		for w, n := range ol.Seen {
			if w == self || n == 0 {
				continue
			}
			if line.Seen == nil {
				line.Seen = Seen{}
			}
			line.Seen[w] = max(line.Seen[w], n)
		}
	}
}

// checkBase refuses a change of a mark that is held when the page it came from
// had not read what this machine has written of the mark since.
func checkBase(v *view, p *proposal) error {
	id := p.mark.ID
	if p.base == nil {
		return ErrNoBase
	}
	superseded := false
	for _, ol := range v.own[id] {
		if ol.Rev <= p.base[ol.Writer()] {
			continue
		}
		sameEdit := p.edit != nil && ol.Edit != nil && ol.Edit.ID == p.edit.ID
		switch {
		case sameEdit && ol.Edit.Seq < p.edit.Seq:
			// An earlier send of this very edit: the last words a page sends
			// as it is left go through.
		case sameEdit:
			superseded = true
		default:
			// Another page, or an old one, wrote the mark since: that is told
			// before a late send is.
			return &StaleError{Seen: staleSeen(v, p)}
		}
	}
	if superseded {
		return ErrSuperseded
	}
	return nil
}

// staleSeen is what a page that is told it is stale may write on top of: what it
// had read, and this machine's own lines of the mark that it had not. A line of
// another computer that arrived after the page was drawn is not in it, since the
// page was never shown it: writing on top of this answer replaces the words of
// the other tab, which the reader chooses to, and leaves the other computer's
// beside them as a conflict.
func staleSeen(v *view, p *proposal) Seen {
	seen := maps.Clone(p.base)
	if seen == nil {
		seen = Seen{}
	}
	for _, ol := range v.own[p.mark.ID] {
		seen[ol.Writer()] = max(seen[ol.Writer()], ol.Rev)
	}
	return seen
}

// replaces says line o replaces line l of the same mark.
func replaces(o, l *Line) bool {
	if o.Writer() == l.Writer() {
		return o.Rev > l.Rev
	}
	return o.Seen[l.Writer()] >= l.Rev
}

// entrySeen is the newest rev of each writer of the mark.
func entrySeen(v *view, id string) Seen {
	seen := Seen{}
	for w, l := range v.byMark[id].newest {
		seen[w] = l.Rev
	}
	return seen
}
