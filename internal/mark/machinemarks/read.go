package machinemarks

import (
	"cmp"
	"slices"
	"strings"
)

// Gate says whether a mark may be given to whoever asked: the privacy rules the
// reader's vault declares, which hold back a mark in a note reserved for an
// agent and one whose words spell a reserved folder. It is asked of every value
// of a mark, and a mark with one value it says no to is not given at all, nor
// is it said that one was held back.
type Gate func(*Mark) bool

// View is what a read found of every machine's marks: this machine's own, which
// are the ledger's together with the file's, so that what git took out of the
// working tree is still shown, and every other machine's, which are the files'.
// It holds the reader's words, and gives none of them out except through a
// Gate.
type View struct {
	v *view
}

// View reads the marks. It takes no lock and writes nothing, and makes no
// folder and no file: a vault nobody has marked leaves nothing behind. It reads
// the ledger first and the vault after, so a write going on at the same time is
// seen in full or not at all, and a count of missing lines is never made by the
// look alone.
func (m *Marks) View() (*View, error) {
	v, err := m.load()
	if err != nil {
		return nil, err
	}
	return &View{v: v}, nil
}

// Entries are the marks the gate lets through, deleted ones included, in the
// order they were made, then by id. gate is asked of every value of a mark; a
// nil gate is a mistake in the caller and panics, since the answer would be
// everything the reader wrote.
func (v *View) Entries(gate Gate) []Entry {
	if gate == nil {
		panic("machinemarks: Entries with no gate")
	}
	var out []Entry
	for _, r := range v.v.byMark {
		e := r.entry(func(l *kept) bool { return v.v.st.owns(l.M) })
		if slices.ContainsFunc(e.Values, func(val Value) bool { return !gate(&val.Mark) }) {
			continue
		}
		out = append(out, *e)
	}
	slices.SortFunc(out, func(a, b Entry) int {
		ma, mb := a.Mark(), b.Mark()
		return cmp.Or(ma.At.Compare(mb.At), strings.Compare(ma.ID, mb.ID))
	})
	return out
}

// Entry is one mark the gate lets through, and false when there is none or the
// gate does not.
func (v *View) Entry(id string, gate Gate) (Entry, bool) {
	if gate == nil {
		panic("machinemarks: Entry with no gate")
	}
	r, ok := v.v.byMark[id]
	if !ok {
		return Entry{}, false
	}
	e := r.entry(func(l *kept) bool { return v.v.st.owns(l.M) })
	if slices.ContainsFunc(e.Values, func(val Value) bool { return !gate(&val.Mark) }) {
		return Entry{}, false
	}
	return *e, true
}

// MachineID is this machine's id, empty until it first writes.
func (v *View) MachineID() string { return v.v.st.id }

// Missing is how many of this machine's marks the ledger holds and its file in
// the vault does not hold as the ledger does: what git took out of the working
// tree, an agent changed, or a write that failed after it was recorded.
// Restore writes them back.
func (v *View) Missing() int { return v.v.missing }

// Damaged is how many lines of other machines' files were changed after they
// were written and are not shown. Of this machine's own, DamagedOwn counts the
// same; those marks are shown from the ledger.
func (v *View) Damaged() int { return v.v.damaged }

// DamagedOwn is how many lines of this machine's own file were changed after
// they were written.
func (v *View) DamagedOwn() int { return v.v.damagedOwn }

// Markers is how many lines of the files are git's conflict markers, which are
// read past: the lines between them are read as any are.
func (v *View) Markers() int { return v.v.markers }

// Newer is how many whole lines are of a format a later version wrote. They are
// not read, and not lost: a file of this machine's keeps them.
func (v *View) Newer() int { return v.v.newer }

// Unreadable is how many other machines' files could not be read: too long, not
// a file, or refused. The marks of the rest are all here, and the list may be
// short.
func (v *View) Unreadable() int { return v.v.unreadable }

// LedgerChanged is how many lines of the ledger were changed after they were
// written, which is said and never read.
func (v *View) LedgerChanged() int { return v.v.ledgerChanged }

// NewIDUnsaid says this machine took a new id, because another was found
// writing under its old one, and the reader has not been told. NoticeNewID
// records that they were.
func (v *View) NewIDUnsaid() bool { return v.v.st.unsaid }
