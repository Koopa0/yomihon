package machinemarks

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/koopa0/yomihon/internal/mark/ledger"
	"github.com/koopa0/yomihon/internal/mark/machinefile"
	"github.com/koopa0/yomihon/internal/mark/sealedline"
)

// ErrOwnFileUnreadable means this machine's own file is there and cannot be
// read. A write replaces the whole file, so one that had not read it would
// throw away what it holds: nothing is written until it can be read.
var ErrOwnFileUnreadable = errors.New("this machine's file of marks cannot be read")

// ErrLedgerUnreadable means the ledger, the local backup, cannot be read at
// all. A line of it changed by another program is counted and the rest read;
// this is a ledger that cannot be opened.
var ErrLedgerUnreadable = errors.New("the ledger cannot be read")

// view is everything a read found, before any of it is held back from anyone.
type view struct {
	st machine
	// byMark is every mark's lines, resolved.
	byMark map[string]resolved
	// own is this machine's own lines of each mark that none of its own others
	// replaces: what its file is to hold.
	own map[string][]*kept
	// ownDisk is what this machine's file holds now, and ownFound whether it is
	// there. ownNewer are its lines of a later format, which are kept.
	ownDisk  []byte
	ownFound bool
	ownNewer [][]byte
	// topOwn is the highest rev of any line under this machine's id in the
	// vault, and readOfMe the highest rev of one of this machine's writers that
	// another machine's line says it had read. A rev below either would be
	// taken for a line already replaced, whatever the ledger says.
	topOwn, readOfMe uint64
	// clone says a line stands under this machine's id that none of its
	// processes wrote.
	clone bool
	// ids are the machine ids of the files in the vault, which a new id must not
	// be.
	ids map[string]bool
	// aside are the values of this machine's own the ledger holds as set aside,
	// by mark.
	aside map[string][]Value

	missing    int
	damaged    int
	damagedOwn int
	newer      int
	unreadable int
	// ownUnreadable says this machine's own file is there and could not be read.
	ownUnreadable bool
	// ownErr is why.
	ownErr        error
	ledgerChanged int
}

// has says a line of some machine is a line of the mark id, a deletion among
// them.
func (v *view) has(id string) bool {
	_, ok := v.byMark[id]
	return ok
}

// holdsSame says a mark that is there and not deleted is the same act as m.
func (v *view) holdsSame(m *Mark) bool {
	for _, r := range v.byMark {
		if head := &r.heads[0].Mark; !head.Deleted && SameAct(head, m) {
			return true
		}
	}
	return false
}

// live is how many marks are not deleted.
func (v *view) live() int {
	n := 0
	for _, r := range v.byMark {
		if !r.heads[0].Mark.Deleted {
			n++
		}
	}
	return n
}

// load reads the ledger, then the vault. In that order, so that a line a writer
// has put in the vault since the ledger was read is one the ledger holds too
// when it is read next, and nothing is counted missing for want of a look: a
// writer records a line before it writes it. The ledger is read once, and a
// ledger that cannot be read fails the whole read: a view made without the
// ledger's lines of this machine would lack the marks only the ledger holds,
// and a write made from it would leave them out of this machine's file.
func (m *Marks) load() (*view, error) {
	events, changed, err := m.ledger.Events()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrLedgerUnreadable, err)
	}
	v := &view{st: stateOf(events), ledgerChanged: changed, ids: map[string]bool{}, own: map[string][]*kept{}, aside: asideOf(events)}
	files, err := m.folder.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read the machine files: %w", err)
	}

	// What the ledger holds of this machine comes first, so that of two lines of
	// one writer and one rev, which only a hand makes, it is the ledger's that
	// is read.
	all := ledgerLines(events, &v.st)
	ledgerNewest := newestOf(all)

	var disk []*kept
	for i := range files {
		f := &files[i]
		v.ids[f.ID] = true
		if f.Err != nil {
			if f.ID == v.st.id {
				v.ownUnreadable, v.ownErr = true, f.Err
			} else {
				v.unreadable++
			}
			continue
		}
		disk = append(disk, m.scan(v, f)...)
	}
	all = append(all, disk...)
	diskNewest := newestOf(disk)
	for key, line := range ledgerNewest {
		if d := diskNewest[key]; d == nil || d.Rev < line.Rev || (d.Rev == line.Rev && !bytes.Equal(d.raw, line.raw)) {
			v.missing++
		}
	}

	v.byMark = merge(all)
	ownLines := map[string][]*kept{}
	for _, line := range all {
		if m.ours(&v.st, line) {
			ownLines[line.Mark.ID] = append(ownLines[line.Mark.ID], line)
		}
	}
	for id, ls := range ownLines {
		v.own[id] = resolve(ls).heads
	}
	return v, nil
}

// asideOf is the values the ledger's set-aside lines hold, by mark, in the
// order they were set aside.
func asideOf(events []ledger.Event) map[string][]Value {
	out := map[string][]Value{}
	for i := range events {
		if events[i].Kind != kindSetAside {
			continue
		}
		if l, verdict := decode(events[i].Content); verdict == readable {
			out[l.Mark.ID] = append(out[l.Mark.ID], Value{Mark: l.Mark, Writer: l.Writer(), Rev: l.Rev, Time: time.UnixMilli(l.T), Own: true})
		}
	}
	return out
}

// scan reads the lines of one machine's file, counting what is not read.
func (m *Marks) scan(v *view, f *machinefile.File) []*kept {
	own := f.ID == v.st.id
	if own {
		v.ownDisk, v.ownFound = f.Data, true
	}
	var out []*kept
	for _, raw := range sealedline.Lines(f.Data) {
		l, verdict := decode(raw)
		switch verdict {
		case marker:
			// git's conflict markers are read past, and the lines between them
			// are read as any are.
		case damaged:
			if own {
				v.damagedOwn++
			} else {
				v.damaged++
			}
		case newer:
			v.newer++
			if own {
				v.ownNewer = append(v.ownNewer, raw)
			}
		case readable:
			line := &kept{Line: l, raw: raw}
			v.note(m, line)
			out = append(out, line)
		}
	}
	return out
}

// newestOf is the newest line of each writer of each mark, by the two together.
func newestOf(ls []*kept) map[string]*kept {
	out := map[string]*kept{}
	for _, l := range ls {
		key := l.Mark.ID + "|" + l.Writer()
		if old := out[key]; old == nil || l.Rev > old.Rev {
			out[key] = l
		}
	}
	return out
}

// note takes what a line of the vault says of the counters and of who wrote
// under this machine's id.
func (v *view) note(m *Marks, l *kept) {
	if v.st.id == "" {
		return
	}
	if l.M == v.st.id {
		v.topOwn = max(v.topOwn, l.Rev)
		if !m.ours(&v.st, l) {
			v.clone = true
		}
		return
	}
	for key, n := range l.Seen {
		if strings.HasPrefix(key, v.st.id+"/") {
			v.readOfMe = max(v.readOfMe, n)
		}
	}
}

// ours says a line under one of this machine's ids was written by one of its
// processes: one the ledger knows, or this one. Under the id it has now, a line
// with no process is its own too; under an id it has left, whose file is
// another machine's, only a process this machine ran counts. A line of a
// machine that is not this one's is not.
func (m *Marks) ours(st *machine, l *kept) bool {
	if !st.owns(l.M) {
		return false
	}
	// Under an id this machine has left, the file is another machine's now, so
	// only a line of a process this machine ran is taken for its own.
	return (l.S == "" && l.M == st.id) || (l.S != "" && (l.S == m.currentSession() || st.sessions[l.M][l.S]))
}

// ledgerLines are the lines the ledger holds of this machine, the newest of
// each mark under each of its ids and every line of a conversion, read as lines
// of the vault are, all from events, the one read of the ledger load made.
func ledgerLines(events []ledger.Event, st *machine) []*kept {
	var out []*kept
	add := func(raw []byte) {
		if l, verdict := decode(raw); verdict == readable && st.owns(l.M) {
			out = append(out, &kept{Line: l, raw: bytes.Clone(raw)})
		}
	}
	for _, id := range st.ids() {
		latest := latestLines(events, id)
		for _, mark := range slices.Sorted(maps.Keys(latest)) {
			add(latest[mark].Content)
		}
	}
	for i := range events {
		if events[i].Kind == kindConverted {
			for _, raw := range sealedline.Lines(events[i].Content) {
				add(raw)
			}
		}
	}
	return out
}

// latestLines is the newest mark line of each mark that machine wrote among
// events, by the id of the mark: what this machine last wrote of each under that
// machine id. A line the ledger recorded before it carried the mark's id is
// read for it.
func latestLines(events []ledger.Event, machine string) map[string]*ledger.Event {
	latest := map[string]*ledger.Event{}
	for i := range events {
		e := &events[i]
		if e.Kind != ledger.KindMark || e.Machine != machine {
			continue
		}
		id := e.Mark
		if id == "" {
			id = markIDOf(e.Content)
		}
		if held, ok := latest[id]; id != "" && (!ok || held.Seq < e.Seq) {
			latest[id] = e
		}
	}
	return latest
}

// markIDOf is the id of the mark a line is about, or empty when it is not a
// line of marks.
func markIDOf(raw []byte) string {
	var line struct {
		Mark struct {
			ID string `json:"id"`
		} `json:"mark"`
	}
	if json.Unmarshal(raw, &line) != nil {
		return ""
	}
	return line.Mark.ID
}

// canonical is the bytes of this machine's file for the lines it holds: its own
// lines that none of its own replaces, in the order of their marks, then the
// lines of a later format it was keeping, in the order it found them.
func (v *view) canonical(own map[string][]*kept) []byte {
	var all []*kept
	for _, ls := range own {
		all = append(all, ls...)
	}
	slices.SortFunc(all, func(a, b *kept) int {
		return cmp.Or(a.Mark.At.Compare(b.Mark.At), strings.Compare(a.Mark.ID, b.Mark.ID), strings.Compare(a.Writer(), b.Writer()), cmp.Compare(a.Rev, b.Rev))
	})
	var out bytes.Buffer
	for _, l := range all {
		out.Write(l.raw)
		out.WriteByte('\n')
	}
	for _, raw := range v.ownNewer {
		out.Write(raw)
		out.WriteByte('\n')
	}
	return out.Bytes()
}

// held says the vault's file of this machine is what a write of what is held
// would make of it.
func (v *view) held() bool {
	return !v.ownFound || bytes.Equal(v.ownDisk, v.canonical(v.own))
}
