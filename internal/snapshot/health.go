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
	"github.com/koopa0/yomihon/internal/wording"
)

// vaultRootLabel names the folder a root-level file lives in, which has no
// name of its own.
// It resolves to the default language because a health projection is built
// from a scan rather than from a request, and so carries no reader's choice.
var vaultRootLabel = wording.VaultRoot.In(wording.ZhHant)

// Health is what the whole folder looks like at once, rather than one note at a
// time. Every fact in it was already being computed to render single pages —
// the resolver knew which citations land nowhere, the backlink index knew which
// notes nothing reaches, the name index knew which names two files both claim.
// Read per note, none of it answers "what should I fix"; gathered, it does.
//
// It reports and never repairs, the same rule the note panel follows.
//
// Every link answer here comes from one extractor, and that extractor reads
// [[…]] citations and nothing else. An ordinary Markdown link to a file that
// is not there is therefore outside this whole picture — the reading page
// navigates it to a 404 and the command line warns about it, while nothing
// below counts it. The page says which links it read for that reason: an
// all-clear is only worth what it covers.
type Health struct {
	// InstanceScopeUnknown is why the citation and island lists could not be
	// worked out, and empty when they could. It is filled here, by the build,
	// because the fact it reports is one the build meets: the artifact policy
	// this generation was handed was declared and could not be honoured, so
	// which files are instances is unknown for the whole of it.
	InstanceScopeUnknown string

	// Unwritten are citations to names no file carries and no ledger declared.
	// A name the vault has planned is not here: it is on the page because it is
	// owed, not because it is wrong.
	Unwritten []HealthLink
	// TitleOnly are citations naming a note's title. The note exists; the link
	// fails because a title is never a name this vault resolves by, which is
	// the resolver's single most consequential rule and the one that fails
	// silently. Separating it from the unwritten is not a nicety: the two need
	// opposite repairs, and calling a written note unwritten sends the reader
	// to write it again.
	TitleOnly []HealthTitleLink
	// Islands are notes nothing cites, grouped by the folder they live in. The
	// flat list was a true answer nobody could use — a real vault produced a
	// hundred and thirty-seven rows, and sixty of them were one course's
	// transcripts, which nothing cites because nothing ever would. Grouping
	// drops no row: it lets the shape of the number be seen before the rows
	// are read, which is the difference between a finding and a wall of text.
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

// HealthIslandGroup is one folder's uncited notes.
type HealthIslandGroup struct {
	Dir   string
	Name  string
	Notes []nav.NoteRef
}

// HealthTitleLink is one citation that names a note's title, with the note it
// was reaching for.
type HealthTitleLink struct {
	From   nav.NoteRef
	Target string
	Note   nav.NoteRef
}

// HealthCollision is one name several files claim, with every claimant listed —
// the vault never picks one, so neither does this.
type HealthCollision struct {
	Name       string
	Candidates []string
}

// Empty reports whether the folder has nothing to answer for.
func (h *Health) Empty() bool {
	return len(h.Unwritten) == 0 && len(h.TitleOnly) == 0 && len(h.Islands) == 0 && len(h.Collisions) == 0
}

// newHealth gathers the whole-folder view from projections this generation
// already built. It walks the same bodies through the same extractor as every
// other link answer here, so a citation counted broken on this page is broken
// on the note's own page too.
func newHealth(notes []*vault.Note, idx *graph.Index, planned judge.Planned, back *Backlinks, policy schema.ArtifactPolicy, titles map[string][]nav.NoteRef) Health {
	var h Health
	var islands []nav.NoteRef
	titleReferenced := make(map[string]bool)
	if idx == nil {
		return h
	}
	// Which files are instances decides which citations are anyone's to answer
	// for, and a policy that was declared and could not be honoured leaves
	// that unknown. Reading it anyway answers false for every path, so a
	// template's placeholders arrive as work the reader owes — a repair nobody
	// can make. The three lists that depend on it are therefore not computed
	// at all: an answer built from an unknown scope is worse than no answer,
	// because it looks exactly like a real one.
	//
	// A vault that declared nothing is a different case and keeps its answers.
	// Excluding nothing is what an undeclared exclusion means, and inventing a
	// rule its owner never wrote would be the larger error.
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
		// A template's citations name its own slots, not notes anyone owes. The
		// contract already says which folders hold artifacts rather than
		// instances; reading every note the same way put a template's
		// placeholders on this page as work to do, which is a repair nobody
		// can make.
		if policy.IsNonInstance(n.RelPath) {
			continue
		}
		from := nav.NoteRef{Name: nav.Label(n.RelPath), RelPath: n.RelPath}
		seen := make(map[string]bool)
		for _, target := range judge.LinkTargets(n.Body) {
			res := idx.Resolve(target)
			switch {
			case res.Kind != graph.Unresolved || planned.Has(target) || seen[target]:
				// Resolved, owed, or already reported for this note.
			// One holder only: naming a note out of several would be a
			// guess, and this page does not make those. The whole-vault index
			// keeps every holder, so the choice of what to do about a shared
			// title is made here rather than settled for everyone upstream.
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
		// A note reached only by its title is cited by nobody as far as the
		// resolver is concerned, because a title is not a name a link follows.
		// Calling it uncited would be true of the graph and false of the
		// vault: someone wrote its name down. The set is the one this same
		// pass already collected, not a third definition of what a citation is.
		if back.Citing(n.RelPath) == 0 && !titleReferenced[n.RelPath] {
			islands = append(islands, from)
		}
	}
	h.Collisions = collisions(idx)
	h.Islands = groupByFolder(islands)
	sortHealth(&h)
	return h
}

// collisions lists every name several files claim, asked of the resolution
// index rather than of the citations anyone happened to write. Two files can
// share a name for years before the first link to it is typed, and that link
// failing is the moment this page exists to prevent; reading the answer off
// the links instead found the pair only after the damage.
//
// Which of the index's collisions count is the resolver's own answer, shared
// with the judge that gates on the same folder: one repair belongs on this page
// once, and the two faces must not report different totals for it.
func collisions(idx *graph.Index) []HealthCollision {
	byName := idx.DistinctCollisions()
	out := make([]HealthCollision, 0, len(byName))
	for name, members := range byName {
		out = append(out, HealthCollision{Name: name, Candidates: members})
	}
	return out
}

// groupByFolder collects uncited notes under the folder each lives in, largest
// group first — the shape a reader needs to see before deciding which of them
// is worth their attention. Nothing is dropped or capped: every note that had
// a row still has one, inside its folder.
func groupByFolder(notes []nav.NoteRef) []HealthIslandGroup {
	byDir := make(map[string][]nav.NoteRef)
	for _, n := range notes {
		dir, _ := pathpkg.Split(n.RelPath)
		byDir[strings.TrimSuffix(dir, "/")] = append(byDir[strings.TrimSuffix(dir, "/")], n)
	}
	groups := make([]HealthIslandGroup, 0, len(byDir))
	for dir, members := range byDir {
		slices.SortFunc(members, func(a, b nav.NoteRef) int { return vault.ComparePaths(a.RelPath, b.RelPath) })
		name := dir
		if name == "" {
			name = vaultRootLabel
		}
		groups = append(groups, HealthIslandGroup{Dir: dir, Name: name, Notes: members})
	}
	slices.SortFunc(groups, func(a, b HealthIslandGroup) int {
		return cmp.Or(cmp.Compare(len(b.Notes), len(a.Notes)), vault.ComparePaths(a.Dir, b.Dir))
	})
	return groups
}

// sortHealth puts every list in one stable order so two runs over an unchanged
// folder produce the same page, and a reader working down the list finds it
// where they left it.
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
