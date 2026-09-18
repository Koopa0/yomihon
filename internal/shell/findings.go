package shell

import (
	"slices"
	"time"

	"github.com/koopa0/yomihon/internal/judge"
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
	// frontmatter reads and carries something the schema does not accept.
	FrontmatterUnreadable []NoteFindings
	SchemaFaults          []NoteFindings
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

// NoteFindings is one note the schema had something to say about: how many
// things it said and how heavy the heaviest of them was. What it said stays on
// that note's own page, because one file described twice in two places is how
// two accounts of it start to disagree.
type NoteFindings struct {
	Note     nav.NoteRef
	Severity judge.Severity
	Count    int
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
	unreadable, faults := schemaFaults(snap)
	return Findings{
		Unwritten:  health.Unwritten,
		TitleOnly:  health.TitleOnly,
		Islands:    health.Islands,
		Collisions: health.Collisions,
		Blocked:    fresh.Blocked,
		Skipped:    snap.Skipped(),
		// A value the type never declared, and a declared value no lifecycle
		// row with it applies to that type: the first is a word nobody can act
		// on, the second a state nothing can put a note into.
		StatusOutsideEnum: statusNotes(lifecycle, snap, func(noteType, value string) bool {
			return !lifecycle.KnownStatus(noteType, value)
		}),
		StatusUnreachable: statusNotes(lifecycle, snap, func(noteType, value string) bool {
			return lifecycle.KnownStatus(noteType, value) && !lifecycle.ReachableStatus(noteType, value)
		}),
		FrontmatterUnreadable: unreadable,
		SchemaFaults:          faults,
		InstanceScopeUnknown:  health.InstanceScopeUnknown,
		LastComplete:          fresh.LastComplete,
	}
}

// schemaFaults splits what the schema said about the whole folder into the two
// things somebody does differently about them: frontmatter that cannot be read
// at all, which has to be repaired before anything else about the note can be
// judged, and frontmatter that reads and carries something the schema does not
// accept, which has a named field to change.
//
// The split is on the rule that fired rather than on a guess about the note,
// because one of these findings is the judge's own statement that it could read
// nothing.
func schemaFaults(snap *snapshot.Generation) (unreadable, faults []NoteFindings) {
	for _, entry := range snap.Files() {
		rel := entry.Path()
		found := snap.SchemaFindings(rel)
		if len(found) == 0 {
			continue
		}
		note, ok := snap.Note(rel)
		if !ok {
			continue
		}
		row := NoteFindings{
			Note:     nav.NoteRef{RelPath: rel, Name: note.Title},
			Severity: heaviest(found),
			Count:    len(found),
		}
		if slices.ContainsFunc(found, func(f judge.Finding) bool { return f.RuleID == "schema.frontmatter" }) {
			unreadable = append(unreadable, row)
			continue
		}
		faults = append(faults, row)
	}
	return unreadable, faults
}

// heaviest is the weight of the worst thing said about one note, which is what
// somebody sorting by weight is choosing between. A lighter finding beside a
// heavier one does not make the note lighter, so the row carries the heaviest
// rather than the first or an average of them.
func heaviest(found []judge.Finding) judge.Severity {
	worst := found[0].Severity
	for i := 1; i < len(found); i++ {
		worst = max(worst, found[i].Severity)
	}
	return worst
}

// statusNotes names the notes whose status answers holds — the whole-folder
// gathering of the flag each note page already shows one at a time. Both
// gatherings walk the same holder list, so the faces they feed cannot disagree
// about which notes exist. An authority that is closed or ungoverned names
// none and the surfaces above say nothing: an unknowable finding must not pose
// as one.
func statusNotes(lifecycle status.Authority, snap *snapshot.Generation, holds func(noteType, value string) bool) []StatusNote {
	if !lifecycle.Governed() || lifecycle.Closed() {
		return nil
	}
	// A generation that does not exist holds no notes to name. The search index
	// is the only projection here that answers a question rather than a field,
	// and it is the one an absent generation cannot stand in for.
	index := snap.Search()
	if index == nil {
		return nil
	}
	holders, err := index.StatusHolders()
	if err != nil {
		return nil
	}
	var out []StatusNote
	for _, h := range holders {
		if !holds(h.Type, h.Status) {
			continue
		}
		out = append(out, StatusNote{
			Note:   nav.NoteRef{Name: nav.Label(h.RelPath), RelPath: h.RelPath},
			Type:   h.Type,
			Status: h.Status,
		})
	}
	return out
}
