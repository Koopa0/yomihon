package machinemarks

import (
	"slices"
	"strings"

	"github.com/koopa0/yomihon/internal/mark/ledger"
)

// The kinds of ledger line that are this package's. The ledger reads none of
// them, and a compaction keeps every one but a mark line a newer line of the
// same mark replaces.
const (
	// kindHeader says this machine's id is ID, from here on.
	kindHeader = "header"
	// kindRetire says the machine id ID is no longer this machine's: another
	// machine was found writing under it. Its processes stay this machine's.
	kindRetire = "retire"
	// kindSaid says the reader was told that the machine took a new id.
	kindSaid = "said"
	// kindConverted holds the lines the marks of the earlier file were
	// converted to, whole, as one commit: a conversion is one line of the
	// ledger however many marks it has, one sync and not one for each.
	kindConverted = "converted"
	// kindSkipped says one mark of the earlier file does not fit a line and was
	// not taken in: its content is the mark's id and why. It is recorded before
	// the conversion, which is the commit, so one killed between the two says
	// it again and the mark is counted once.
	kindSkipped = "convert-skipped"
	// kindReplaced holds the bytes of this machine's file that a write was about
	// to replace and that were not what this machine last wrote.
	kindReplaced = "marks-replaced"
	// kindSetAside holds one line of this machine's own that a change replaced
	// while it held a live value that neither the change nor the page it was made
	// on had: whole, as it was written, recorded before the change that replaces
	// it. The ledger does not read it as a mark's line, so it is never a value of
	// the mark, and a compaction keeps every line of a kind it does not read.
	kindSetAside = "mark-set-aside"
)

// machine is what the ledger says of this machine: who it is, and which
// processes were it.
type machine struct {
	// id is this machine's id, empty before the first write.
	id string
	// retired are ids this machine had and left.
	retired []string
	// sessions are the processes this machine was, by the id they wrote under.
	sessions map[string]map[string]bool
	// unsaid is a new id the reader has not been told of.
	unsaid bool
	// converted says the earlier file's marks were taken in.
	converted bool
	// skipped are the ids of the marks of the earlier file that were not taken
	// in because they do not fit a line.
	skipped map[string]bool
	// next is above every seq the ledger has said.
	next uint64
}

// stateOf reads what the ledger's lines say of this machine.
func stateOf(events []ledger.Event) machine {
	st := machine{sessions: map[string]map[string]bool{}, skipped: map[string]bool{}, next: 1}
	note := func(id, session string) {
		if id == "" || session == "" {
			return
		}
		if st.sessions[id] == nil {
			st.sessions[id] = map[string]bool{}
		}
		st.sessions[id][session] = true
	}
	for i := range events {
		e := &events[i]
		st.next = max(st.next, e.Seq+1)
		switch e.Kind {
		case kindHeader:
			st.id = e.ID
		case kindRetire:
			if !slices.Contains(st.retired, e.ID) {
				st.retired = append(st.retired, e.ID)
			}
			st.unsaid = true
		case kindSaid:
			st.unsaid = false
		case kindSkipped:
			if id, _, found := strings.Cut(string(e.Content), " "); found {
				st.skipped[id] = true
			}
		case kindConverted:
			st.converted = true
			note(e.Writer, e.Session)
		case ledger.KindMark:
			note(e.Writer, e.Session)
		case ledger.KindSession:
			note(e.ID, e.Session)
		}
	}
	return st
}

// ids are every id this machine has had, the current one first.
func (st *machine) ids() []string {
	if st.id == "" {
		return nil
	}
	ids := []string{st.id}
	for _, id := range st.retired {
		if id != st.id {
			ids = append(ids, id)
		}
	}
	return ids
}

// owns says id is an id this machine has had, or has now.
func (st *machine) owns(id string) bool {
	return id != "" && (id == st.id || slices.Contains(st.retired, id))
}
