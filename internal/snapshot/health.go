package snapshot

import (
	"cmp"
	pathpkg "path"
	"slices"
	"strings"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

// Health is what the whole folder looks like at once rather than one note at a
// time, gathered from facts the single pages already compute. It reports and
// never repairs. Every link answer comes from the one extractor, which reads
// [[…]] citations and nothing else, so an ordinary markdown link to a missing
// file is outside this picture and the page says which links it read.
type Health struct {
	// InstanceScopeUnknown is why the citation and island lists could not be
	// worked out, and empty when they could: the artifact policy this
	// generation was handed was declared and could not be honoured.
	InstanceScopeUnknown string

	// Unwritten are citations to names no file carries and no ledger declared.
	// A name the vault has planned is owed rather than wrong, and is not here.
	Unwritten []HealthLink
	// TitleOnly are citations naming a note's title. The note exists; the link
	// fails because a title is never a name this vault resolves by. It is kept
	// apart from the unwritten because the two need opposite repairs.
	TitleOnly []HealthTitleLink
	// Islands are notes nothing cites, grouped by the folder they live in so the
	// shape of the number is visible before the rows are read. No row is dropped.
	Islands []HealthIslandGroup
	// Collisions are names more than one file answers to, where a citation
	// resolves to none of them because the vault refuses to guess.
	Collisions []HealthCollision
}

// HealthLink is one citation with nowhere to land, and the note making it.
type HealthLink struct {
	From   nav.NoteRef
	Target string
}

// HealthIslandGroup is one folder's uncited notes. Dir is the folder, empty for
// the one at the top, which a page names in the reader's own language instead.
type HealthIslandGroup struct {
	Dir   string
	Notes []nav.NoteRef
}

// HealthTitleLink is one citation naming a note's title, with the note meant.
type HealthTitleLink struct {
	From   nav.NoteRef
	Target string
	Note   nav.NoteRef
}

// HealthCollision is one name several files claim, with every claimant listed.
type HealthCollision struct {
	Name       string
	Candidates []string
}

// Empty reports whether the folder has nothing to answer for.
func (h *Health) Empty() bool {
	return len(h.Unwritten) == 0 && len(h.TitleOnly) == 0 && len(h.Islands) == 0 && len(h.Collisions) == 0
}

// newHealth gathers the whole-folder view from projections this generation
// already built, through the same extractor as every other link answer here.
func newHealth(notes []*vault.Note, idx *graph.Index, planned judge.Planned, back *Backlinks, policy schema.ArtifactPolicy, titles map[string][]nav.NoteRef) Health {
	var h Health
	var islands []nav.NoteRef
	titleReferenced := make(map[string]bool)
	if idx == nil {
		return h
	}
	// Which files are instances decides which citations are anyone's to answer
	// for, so a policy that was declared and could not be honoured leaves the
	// three lists depending on it uncomputed: an answer built from an unknown
	// scope looks exactly like a real one. A vault that declared nothing is a
	// different case and keeps its answers.
	if !policy.Trustworthy() {
		h.InstanceScopeUnknown = policy.Diagnostic()
		h.Collisions = collisions(idx)
		sortHealth(&h)
		return h
	}
	for _, n := range notes {
		if n == nil {
			continue
		}
		// A template's citations name its own slots, not notes anyone owes.
		if policy.IsNonInstance(n.RelPath) {
			continue
		}
		from := nav.NoteRef{Name: nav.Label(n.RelPath), RelPath: n.RelPath}
		seen := make(map[string]bool)
		for _, target := range judge.LinkTargets(n.Body) {
			res := idx.Resolve(target)
			switch {
			case res.Kind != graph.KindUnresolved || planned.Has(target) || seen[target]:
				// Resolved, owed, or already reported for this note.
			// One holder only: naming a note out of several would be a guess.
			case len(titles[graph.NormalizeKey(target)]) == 1:
				seen[target] = true
				titleReferenced[titles[graph.NormalizeKey(target)][0].RelPath] = true
				h.TitleOnly = append(h.TitleOnly, HealthTitleLink{
					From: from, Target: target, Note: titles[graph.NormalizeKey(target)][0],
				})
			default:
				seen[target] = true
				h.Unwritten = append(h.Unwritten, HealthLink{From: from, Target: target})
			}
		}
		// A note reached only by its title is uncited as far as the resolver is
		// concerned, but someone did write its name down.
		if back.Citing(n.RelPath) == 0 && !titleReferenced[n.RelPath] {
			islands = append(islands, from)
		}
	}
	h.Collisions = collisions(idx)
	h.Islands = groupByFolder(islands)
	sortHealth(&h)
	return h
}

// collisions lists every name several files claim, asked of the resolution index
// rather than of the citations anyone happened to write: two files can share a
// name for years before the first link to it is typed. Which of them count is
// the resolver's own answer, shared with the judge gating the same folder.
func collisions(idx *graph.Index) []HealthCollision {
	byName := idx.DistinctCollisions()
	out := make([]HealthCollision, 0, len(byName))
	for name, members := range byName {
		out = append(out, HealthCollision{Name: name, Candidates: members})
	}
	return out
}

// groupByFolder collects uncited notes under the folder each lives in, largest
// group first. Nothing is dropped or capped.
func groupByFolder(notes []nav.NoteRef) []HealthIslandGroup {
	byDir := make(map[string][]nav.NoteRef)
	for _, n := range notes {
		dir, _ := pathpkg.Split(n.RelPath)
		byDir[strings.TrimSuffix(dir, "/")] = append(byDir[strings.TrimSuffix(dir, "/")], n)
	}
	groups := make([]HealthIslandGroup, 0, len(byDir))
	for dir, members := range byDir {
		slices.SortFunc(members, func(a, b nav.NoteRef) int { return vault.ComparePaths(a.RelPath, b.RelPath) })
		groups = append(groups, HealthIslandGroup{Dir: dir, Notes: members})
	}
	slices.SortFunc(groups, func(a, b HealthIslandGroup) int {
		return cmp.Or(cmp.Compare(len(b.Notes), len(a.Notes)), vault.ComparePaths(a.Dir, b.Dir))
	})
	return groups
}

// sortHealth puts every list in one stable order, so two runs over an unchanged
// folder produce the same page.
func sortHealth(h *Health) {
	slices.SortFunc(h.Unwritten, func(a, b HealthLink) int {
		return cmp.Or(vault.ComparePaths(a.From.RelPath, b.From.RelPath), vault.ComparePaths(a.Target, b.Target))
	})
	slices.SortFunc(h.TitleOnly, func(a, b HealthTitleLink) int {
		return cmp.Or(vault.ComparePaths(a.From.RelPath, b.From.RelPath), vault.ComparePaths(a.Target, b.Target))
	})
	slices.SortFunc(h.Collisions, func(a, b HealthCollision) int {
		return vault.ComparePaths(a.Name, b.Name)
	})
}
