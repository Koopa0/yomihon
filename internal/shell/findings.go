package shell

import (
	"time"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/snapshot"
)

// Findings is everything the whole-folder health page reports, gathered from
// one captured generation. It carries no words of its own: which language a row
// is read in, and what each row is called, is settled where the page is built.
//
// It is gathered here rather than beside the page so that what the folder has
// to answer for is read from one captured generation, in one place.
type Findings struct {
	// NavigationFaults are readable notes with omitted navigation trees.
	NavigationFaults []nav.CoreFault

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
	// FrontmatterUnreadable are the notes whose frontmatter is not valid YAML or
	// opens a fence nothing closes, so nothing they declare could be judged;
	// SchemaFaults are the ones whose frontmatter reads and carries something
	// the schema does not accept. Both are the generation's own, gathered once
	// while the folder was read.
	FrontmatterUnreadable []snapshot.HealthNoteFindings
	SchemaFaults          []snapshot.HealthNoteFindings
	// UnmappedConcepts are the concepts no map links to, and EmptyBranches the
	// map headings no note stands under. They say how the folder's maps cover
	// it; neither is a finding.
	UnmappedConcepts []nav.NoteRef
	EmptyBranches    []snapshot.HealthEmptyBranch
	// InstanceScopeUnknown is why the citation and island lists could not be
	// worked out, empty where they could.
	InstanceScopeUnknown string
	// LastComplete is when the folder was last read whole, zero when no whole
	// read has happened since start-up.
	LastComplete time.Time
	// Collision is the two files whose names fold to one path, as the
	// filesystem spells them, or nil. It says whether the findings are current
	// rather than being one, so it is not a list of findings; the health page
	// draws it above its tables rather than as a row in one.
	Collision []string
	// Root reports selected-path identity trouble without adding a finding row.
	Root *snapshot.RootNotice
}

// GatherFindings collects what the folder has to answer for from one captured
// generation. It reads no source of its own and repairs nothing.
//
// Each list arrives in the order the reading produced it, which is the order
// the page lists findings in.
func GatherFindings(snap *snapshot.Generation) Findings {
	health := snap.Health()
	fresh := snap.Freshness()
	return Findings{
		NavigationFaults:      health.NavigationFaults,
		Unwritten:             health.Unwritten,
		TitleOnly:             health.TitleOnly,
		Islands:               health.Islands,
		Collisions:            health.Collisions,
		Blocked:               fresh.Blocked,
		Skipped:               snap.Skipped(),
		FrontmatterUnreadable: health.FrontmatterUnreadable,
		SchemaFaults:          health.SchemaFaults,
		UnmappedConcepts:      health.UnmappedConcepts,
		EmptyBranches:         health.EmptyBranches,
		InstanceScopeUnknown:  health.InstanceScopeUnknown,
		LastComplete:          fresh.LastComplete,
		Collision:             fresh.Collision,
		Root:                  fresh.Root,
	}
}
