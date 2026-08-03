package snapshot

import (
	"cmp"
	"slices"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/vault"
)

// Health is what the whole folder looks like at once, rather than one note at a
// time. Every fact in it was already being computed to render single pages —
// the resolver knew which citations land nowhere, the backlink index knew which
// notes nothing reaches, the name index knew which names two files both claim.
// Read per note, none of it answers "what should I fix"; gathered, it does.
//
// It reports and never repairs, the same rule the note panel follows.
type Health struct {
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
	// Islands are notes nothing cites. It is the weakest signal here and the
	// most useful to a person: a note written once and never reached again is
	// usually one that was read and not digested.
	Islands []nav.NoteRef
	// Collisions are names more than one file answers to, where a citation
	// resolves to none of them because the vault refuses to guess.
	Collisions []HealthCollision
}

// HealthLink is one citation with nowhere to land, and the note making it.
type HealthLink struct {
	From   nav.NoteRef
	Target string
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
func newHealth(notes []*vault.Note, idx *graph.Index, planned judge.Planned, back *Backlinks) Health {
	var h Health
	if idx == nil {
		return h
	}
	titles := titleIndex(notes)
	for _, n := range notes {
		if n == nil {
			continue
		}
		from := nav.NoteRef{Name: nav.Label(n.RelPath), RelPath: n.RelPath}
		seen := make(map[string]bool)
		for _, target := range judge.LinkTargets(n.Body) {
			res := idx.Resolve(target)
			switch {
			case res.Kind == graph.Ambiguous:
				h.Collisions = append(h.Collisions, HealthCollision{
					Name:       target,
					Candidates: slices.Clone(res.Candidates),
				})
			case res.Kind != graph.Unresolved || planned.Has(target) || seen[target]:
				// Resolved, owed, or already reported for this note.
			case titles[graph.NormalizeKey(target)].RelPath != "":
				seen[target] = true
				h.TitleOnly = append(h.TitleOnly, HealthTitleLink{
					From: from, Target: target, Note: titles[graph.NormalizeKey(target)],
				})
			default:
				seen[target] = true
				h.Unwritten = append(h.Unwritten, HealthLink{From: from, Target: target})
			}
		}
		if back.Citing(n.RelPath) == 0 {
			h.Islands = append(h.Islands, from)
		}
	}
	sortHealth(&h)
	return h
}

// sortHealth puts every list in one stable order so two runs over an unchanged
// folder produce the same page, and a reader working down the list finds it
// where they left it.
func sortHealth(h *Health) {
	slices.SortFunc(h.Unwritten, func(a, b HealthLink) int {
		return cmp.Or(cmp.Compare(a.From.RelPath, b.From.RelPath), cmp.Compare(a.Target, b.Target))
	})
	slices.SortFunc(h.TitleOnly, func(a, b HealthTitleLink) int {
		return cmp.Or(cmp.Compare(a.From.RelPath, b.From.RelPath), cmp.Compare(a.Target, b.Target))
	})
	slices.SortFunc(h.Islands, func(a, b nav.NoteRef) int {
		return cmp.Compare(a.RelPath, b.RelPath)
	})
	slices.SortFunc(h.Collisions, func(a, b HealthCollision) int {
		return cmp.Compare(a.Name, b.Name)
	})
	h.Collisions = slices.CompactFunc(h.Collisions, func(a, b HealthCollision) bool {
		return a.Name == b.Name
	})
}

// titleIndex maps a note's declared title to the note, for the one question the
// resolver deliberately cannot answer: which note was a citation reaching for
// when it named a title. A title two notes both declare is left out — naming
// one of them would be a guess, and this page does not make those.
func titleIndex(notes []*vault.Note) map[string]nav.NoteRef {
	byTitle := make(map[string]nav.NoteRef)
	ambiguous := make(map[string]bool)
	for _, n := range notes {
		if n == nil {
			continue
		}
		key := graph.NormalizeKey(n.Title())
		if key == "" {
			continue
		}
		if _, taken := byTitle[key]; taken {
			ambiguous[key] = true
			continue
		}
		byTitle[key] = nav.NoteRef{Name: nav.Label(n.RelPath), RelPath: n.RelPath}
	}
	for key := range ambiguous {
		delete(byTitle, key)
	}
	return byTitle
}
