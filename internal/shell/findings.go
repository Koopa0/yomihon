package shell

import (
	"time"

	"github.com/koopa0/yomihon/internal/lexical"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/status"
)

// Findings is everything the whole-folder health page reports, gathered from
// one captured generation and one captured lifecycle view. It carries no words
// of its own: which language a row is read in, and what each row is called, is
// settled where the page is built.
//
// It is gathered here rather than beside the page because two surfaces read it.
// The page lists the rows; the rail's foot says how many there are. A number
// worked out a second time beside the rail is a number that disagrees with the
// page the day either one changes.
type Findings struct {
	// Unwritten are citations to names no file carries and no ledger declared.
	Unwritten []snapshot.HealthLink
	// TitleOnly are citations naming a note's title, which is never a name this
	// vault resolves by.
	TitleOnly []snapshot.HealthTitleLink
	// Islands are notes nothing cites in the body, grouped by folder.
	Islands []snapshot.HealthIslandGroup
	// Collisions are names more than one file answers to.
	Collisions []snapshot.HealthCollision
	// Blocked are the sources the latest build could not read.
	Blocked []snapshot.BlockedSource
	// Skipped are the paths the generation saw and did not index.
	Skipped []snapshot.Skipped
	// StatusOutsideEnum are the notes carrying a status outside their type's
	// declared list; StatusUnreachable are the ones whose status is declared
	// while no lifecycle row with it applies to their type.
	StatusOutsideEnum []StatusNote
	StatusUnreachable []StatusNote
	// FrontmatterUnreadable are the notes whose frontmatter is not valid YAML,
	// so nothing they declare could be judged; SchemaFaults are the ones whose
	// frontmatter reads and carries something the schema does not accept. Both
	// are the generation's own, gathered once while the folder was read.
	FrontmatterUnreadable []snapshot.HealthNoteFindings
	SchemaFaults          []snapshot.HealthNoteFindings
	// InstanceScopeUnknown is why the citation and island lists could not be
	// worked out, empty where they could.
	InstanceScopeUnknown string
	// LastComplete is when the folder was last read whole, zero when no whole
	// read has happened since start-up.
	LastComplete time.Time
}

// StatusNote is one note carrying a status its own type never declared, named
// beside the value and that type: the value is the word somebody has to edit
// and the type is why it failed, so both travel with the note.
type StatusNote struct {
	Note   nav.NoteRef
	Type   string
	Status string
}

// Total is how many findings this folder has against it, counted the way the
// health table counts its own rows: several citations out of one note fold into
// that note's row and are tallied there, so the fold cancels and every list
// contributes the things found in it rather than the lines they are drawn on.
// A note the schema said nine things about counts nine.
func (f *Findings) Total() int {
	total := len(f.Blocked) + len(f.Skipped) +
		len(f.Unwritten) + len(f.TitleOnly) +
		len(f.StatusOutsideEnum) + len(f.StatusUnreachable) +
		len(f.Collisions)
	for _, group := range f.Islands {
		total += len(group.Notes)
	}
	for _, found := range f.FrontmatterUnreadable {
		total += found.Count
	}
	for _, found := range f.SchemaFaults {
		total += found.Count
	}
	return total
}

// GatherFindings collects what the folder has to answer for from one captured
// generation and the lifecycle view captured for the same request. It reads no
// source of its own and repairs nothing.
//
// Each list arrives in the order the reading produced it, which is the order
// the page lists findings in.
func GatherFindings(lifecycle status.Authority, snap *snapshot.Generation) Findings {
	health := snap.Health()
	fresh := snap.Freshness()
	outsideEnum, unreachable := statusFaults(lifecycle, snap)
	return Findings{
		Unwritten:             health.Unwritten,
		TitleOnly:             health.TitleOnly,
		Islands:               health.Islands,
		Collisions:            health.Collisions,
		Blocked:               fresh.Blocked,
		Skipped:               snap.Skipped(),
		StatusOutsideEnum:     outsideEnum,
		StatusUnreachable:     unreachable,
		FrontmatterUnreadable: health.FrontmatterUnreadable,
		SchemaFaults:          health.SchemaFaults,
		InstanceScopeUnknown:  health.InstanceScopeUnknown,
		LastComplete:          fresh.LastComplete,
	}
}

// statusFaults names the notes carrying a status their own type cannot be in:
// one the type never declared, and one it declared while no lifecycle row with
// it applies to that type. The first is a word nobody can act on, the second a
// state nothing can put a note into.
//
// Both answers come from one walk of the folder's lifecycle, because they are
// asked of the same notes and this runs on every page: the walk keeps nothing
// it is not reporting, so a folder of thousands of notes with nothing wrong in
// it costs a scan and no memory at all. An authority that is closed or
// ungoverned names none and the surfaces above say nothing: an unknowable
// finding must not pose as one.
func statusFaults(lifecycle status.Authority, snap *snapshot.Generation) (outsideEnum, unreachable []StatusNote) {
	if !lifecycle.Governed() || lifecycle.Closed() {
		return nil, nil
	}
	// A generation that does not exist holds no notes to name. The search index
	// is the only projection here that answers a question rather than a field,
	// and it is the one an absent generation cannot stand in for.
	index := snap.Search()
	if index == nil {
		return nil, nil
	}
	holders, err := index.EachStatusHolder()
	if err != nil {
		return nil, nil
	}
	// Thousands of notes carry a handful of distinct type-and-status pairs
	// between them, and the contract's answer about a pair is the same for
	// every note holding it. Asking once per pair rather than once per note is
	// what keeps this walk off a page's cost: the answer for one pair copies
	// the type's whole declared list to compare against.
	verdicts := map[statusPair]statusVerdict{}
	for h := range holders {
		pair := statusPair{noteType: h.Type, status: h.Status}
		verdict, asked := verdicts[pair]
		if !asked {
			verdict = statusVerdict{
				known:     lifecycle.KnownStatus(h.Type, h.Status),
				reachable: lifecycle.ReachableStatus(h.Type, h.Status),
			}
			verdicts[pair] = verdict
		}
		switch {
		case !verdict.known:
			outsideEnum = append(outsideEnum, statusNote(h))
		case !verdict.reachable:
			unreachable = append(unreachable, statusNote(h))
		}
	}
	return outsideEnum, unreachable
}

// statusPair is the type and status a note declares, which is everything the
// contract is asked about it.
type statusPair struct {
	noteType string
	status   string
}

// statusVerdict is what the contract says about one such pair: two booleans
// that decide which of the two lists a note holding that pair joins, and
// neither of which varies between notes holding it.
type statusVerdict struct {
	known     bool
	reachable bool
}

// statusNote names one holder the way every other list here names a note.
func statusNote(h lexical.StatusHolder) StatusNote {
	return StatusNote{
		Note:   nav.NoteRef{Name: nav.Label(h.RelPath), RelPath: h.RelPath},
		Type:   h.Type,
		Status: h.Status,
	}
}
