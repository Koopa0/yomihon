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
// order they were made, then by id. gate is asked of every value of a mark,
// those set aside among them; a nil gate is a mistake in the caller and panics,
// since the answer would be everything the reader wrote.
func (v *View) Entries(gate Gate) []Entry {
	if gate == nil {
		panic("machinemarks: Entries with no gate")
	}
	var out []Entry
	for id := range v.v.byMark {
		if e, ok := v.entry(id, gate); ok {
			out = append(out, e)
		}
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
	return v.entry(id, gate)
}

// entry is the mark of id as Entry gives it.
func (v *View) entry(id string, gate Gate) (Entry, bool) {
	r, ok := v.v.byMark[id]
	if !ok {
		return Entry{}, false
	}
	e := r.entry(func(l *rawLine) bool { return v.v.st.owns(l.M) })
	e.Aside = v.v.aside[id]
	refused := func(val Value) bool { return !gate(&val.Mark) }
	if slices.ContainsFunc(e.Values, refused) || slices.ContainsFunc(e.Aside, refused) {
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

// Newer is how many whole lines are of a format a later version wrote. They are
// not read, and not lost: a file of this machine's keeps them.
func (v *View) Newer() int { return v.v.newer }

// Unreadable is how many other machines' files could not be read: too long, not
// a file, or refused. The marks of the rest are all here, and the list may be
// short.
func (v *View) Unreadable() int { return v.v.unreadable }

// OwnUnreadable says this machine's own file in the vault is there and could
// not be read. The marks are shown without it, from the ledger, and no write is
// made until it can be read.
func (v *View) OwnUnreadable() bool { return v.v.ownUnreadable }

// LegacySkipped is how many marks of the earlier file could not be taken in
// when it was, because they do not fit a line: they stay in the earlier file,
// which is never changed.
func (v *View) LegacySkipped() int { return len(v.v.st.skipped) }

// LedgerChanged is how many lines of the ledger were changed after they were
// written, which is said and never read.
func (v *View) LedgerChanged() int { return v.v.ledgerChanged }

// IDRetired says this machine took a new id, because another computer was found
// writing under its old one: a copy of this one's configuration. It is for the
// log; the reader is not told.
func (v *View) IDRetired() bool { return len(v.v.st.retired) > 0 }

// AwaitsTakeIn says the marks of the earlier file have not been taken in yet:
// this machine has no file of marks and no conversion in its ledger, so its next
// write takes them in. Until then they are in the earlier file only, and a page
// that listed only what a read finds would not show them.
func (v *View) AwaitsTakeIn() bool { return !v.v.st.converted && !v.v.ownFound }

// LegacyView is the view of a vault whose reader folder cannot be entered: it
// holds no mark, and says the earlier file is not taken in, so that what a page
// lists from the earlier file, which a read never changes, is still shown.
func LegacyView() *View {
	return &View{v: &view{byMark: map[string]resolved{}, own: map[string][]*rawLine{}, ids: map[string]bool{}}}
}
