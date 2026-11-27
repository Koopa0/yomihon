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
	// NavigationFaults are readable notes whose navigation trees could not be built.
	NavigationFaults []nav.CoreFault

	// InstanceScopeUnknown is why the citation and island lists could not be
	// worked out, and empty when they could: the artifact policy this
	// generation was handed was declared and could not be honoured.
	InstanceScopeUnknown string

	// Unwritten are citations to names no file carries and nothing tracks as
	// owed. A name the vault has planned, or a link under a gap heading, is
	// owed rather than wrong, and is not here.
	Unwritten []HealthLink
	// TitleOnly are citations naming a note's title, one row for each note
	// carrying it. The note exists; the link fails because a title is never a
	// name this vault resolves by. It is kept apart from the unwritten because
	// the two need opposite repairs.
	TitleOnly []HealthTitleLink
	// Islands are notes nothing cites in the body, grouped by the folder they
	// live in so the shape of the number is visible before the rows are read. A
	// based_on declaration is not a citation here. Contract-declared paths and
	// maps are entry points and need no citation to stay off this list.
	Islands []HealthIslandGroup
	// Collisions are names more than one file answers to, where a citation
	// resolves to none of them because the vault refuses to guess.
	Collisions []HealthCollision

	// FrontmatterUnreadable are the notes whose frontmatter is not valid YAML or
	// opens a fence nothing closes, so nothing they declare could be judged at
	// all; SchemaFaults are the ones whose frontmatter reads and carries
	// something the schema does not accept. They are gathered while the folder
	// is read, with everything else here, because the rail's foot states how
	// many findings stand against the folder and a walk over every note is not
	// a thing to do on every page.
	FrontmatterUnreadable []HealthNoteFindings
	SchemaFaults          []HealthNoteFindings

	// UnmappedConcepts are the notes of the rules file's concept type that no
	// index reaches, counted the way the coverage command counts a concept as
	// mounted. They are how the folder's indexes cover its concepts, said for
	// the author to see, and not findings: a concept waiting for its index is
	// not a fault, and the page holds no weight for it. Empty where the rules
	// file declares no concept type.
	UnmappedConcepts []nav.NoteRef
	// EmptyBranches are the headings of the folder's maps that no note stands
	// under, which a map's own tree leaves out. They are said here for the same
	// reason, and are no findings either.
	EmptyBranches []HealthEmptyBranch
}

// HealthEmptyBranch is one heading of a map with no note beneath it.
type HealthEmptyBranch struct {
	Map     nav.NoteRef
	Heading string
	Level   int
}

// HealthNoteFindings is one note the schema had something to say about: how
// many things it said and how heavy the heaviest of them was. What it said
// stays on that note's own page, because one file described twice in two places
// is how two accounts of it start to disagree. What travels is the number and
// the weight, which the note's own page never states and which are the only way
// a table can tell a note that drew one complaint from one that drew nine.
type HealthNoteFindings struct {
	Note     nav.NoteRef
	Severity judge.Severity
	Count    int
}

// schemaFaultRows splits what the schema said about the whole folder into the
// two things somebody does differently about them: frontmatter that cannot be
// read at all, which has to be repaired before anything else about the note can
// be judged, and frontmatter that reads and carries something the schema does
// not accept, which has a named field to change.
//
// The split is on the rule that fired rather than on a guess about the note,
// because one of these findings is the judge's own statement that it could read
// nothing. Both lists arrive in the order the folder was read, which is the
// order every other list here is in.
func schemaFaultRows(
	notes []*vault.Note,
	findings map[string][]judge.Finding,
	readings map[string]Reading,
) (unreadable, faults []HealthNoteFindings) {
	for _, n := range notes {
		if n == nil {
			continue
		}
		found := findings[n.RelPath]
		if len(found) == 0 {
			continue
		}
		reading, ok := readings[n.RelPath]
		if !ok {
			continue
		}
		row := HealthNoteFindings{
			Note:     nav.NoteRef{RelPath: n.RelPath, Name: reading.Title},
			Severity: heaviestFinding(found),
			Count:    len(found),
		}
		if slices.ContainsFunc(found, func(f judge.Finding) bool { return f.RuleID == judge.RuleSchemaFrontmatter }) {
			unreadable = append(unreadable, row)
			continue
		}
		faults = append(faults, row)
	}
	return unreadable, faults
}

// heaviestFinding is the weight of the worst thing said about one note, which
// is what somebody sorting by weight is choosing between. A lighter finding
// beside a heavier one does not make the note lighter, so the row carries the
// heaviest rather than the first or an average of them.
func heaviestFinding(found []judge.Finding) judge.Severity {
	worst := found[0].Severity
	for i := 1; i < len(found); i++ {
		worst = max(worst, found[i].Severity)
	}
	return worst
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
	return len(h.NavigationFaults) == 0 && len(h.Unwritten) == 0 && len(h.TitleOnly) == 0 && len(h.Islands) == 0 && len(h.Collisions) == 0
}

// newHealth gathers the whole-folder view from projections this generation
// already built. cited is what each note cites, by path, as it was read when
// the note was, and every citation is weighed by judge.Standing, the verdict
// the check's link rules give it, so the page and the command list the same
// links. Three differences are the page's own: the page reads every note,
// where the check leaves out what the rules file withholds; a template's
// citations are its slots; and a course's rows and an embed naming a file are
// listed here as unwritten where the check reports them under the course and
// image rules.
func newHealth(notes []*vault.Note, cited map[string][]judge.Citation, idx *graph.Index, planned judge.Planned, back *Backlinks, policy schema.ArtifactPolicy, titles map[string][]nav.NoteRef, roles schema.NavigationRoles, coverage *mapCoverage) Health {
	var h Health
	h.EmptyBranches = emptyBranchRows(coverage.maps)
	var uncited []nav.NoteRef
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
		h.Collisions = collisions(idx, cited)
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
		from := nav.NoteRef{Name: graph.TitleOf(n), RelPath: n.RelPath}
		h.addCitations(from, cited[n.RelPath], func(c judge.Citation) ([]nav.NoteRef, judge.LinkStanding) {
			holders := titles[graph.NormalizeKey(c.Target)]
			return holders, judge.Standing(c, idx, planned, len(holders) > 0)
		})
		if back.Citing(n.RelPath) == 0 && !roles.IsPathType(n.Type()) && !roles.IsMapType(n.Type()) {
			uncited = append(uncited, from)
		}
	}
	// A note reached only by its title is uncited as far as the resolver is
	// concerned, but someone did write its name down. Which names were written
	// is settled only once every note has been read, so this asks after the
	// pass rather than inside it: asked inside, a note was spared only when the
	// note naming it sorted ahead of it, and renaming either file changed the
	// answer for a folder nobody had edited.
	titleReferenced := make(map[string]bool, len(h.TitleOnly))
	for _, link := range h.TitleOnly {
		titleReferenced[link.Note.RelPath] = true
	}
	islands := slices.DeleteFunc(uncited, func(n nav.NoteRef) bool { return titleReferenced[n.RelPath] })
	h.Collisions = collisions(idx, cited)
	h.Islands = groupByFolder(islands)
	h.UnmappedConcepts = unmappedConcepts(notes, cited, idx, policy, roles, coverage)
	sortHealth(&h)
	return h
}

// addCitations files one note's citations that land nowhere: once per name,
// under the reason weigh gives, and a title once for every note it belongs to,
// since picking one of several would be a guess.
func (h *Health) addCitations(from nav.NoteRef, cited []judge.Citation, weigh func(judge.Citation) ([]nav.NoteRef, judge.LinkStanding)) {
	seen := make(map[string]bool)
	for _, c := range cited {
		if seen[c.Target] {
			continue
		}
		holders, standing := weigh(c)
		switch standing {
		case judge.LinkResolves, judge.LinkOwed:
			continue
		case judge.LinkUnwritten:
			h.Unwritten = append(h.Unwritten, HealthLink{From: from, Target: c.Target})
		case judge.LinkTitleOnly:
			for _, holder := range holders {
				h.TitleOnly = append(h.TitleOnly, HealthTitleLink{From: from, Target: c.Target, Note: holder})
			}
		}
		seen[c.Target] = true
	}
}

// mapCoverage is what the two coverage groups are worked out from: the
// folder's maps, the type its concepts are filed under (empty where the
// contract declares none), and the layer the concepts are read in.
type mapCoverage struct {
	maps        []nav.Map
	conceptType string
	knowledge   schema.KnowledgeScope
}

// emptyBranchRows lists every heading of every map that no note stands under,
// map by map in the order the maps are listed.
func emptyBranchRows(maps []nav.Map) []HealthEmptyBranch {
	var rows []HealthEmptyBranch
	for i := range maps {
		for _, branch := range maps[i].Empty {
			rows = append(rows, HealthEmptyBranch{
				Map:     nav.NoteRef{Name: maps[i].Title, RelPath: maps[i].RelPath},
				Heading: branch.Heading,
				Level:   branch.Level,
			})
		}
	}
	return rows
}

// unmappedConcepts lists the concept notes no index reaches, in path order. An
// index reaches a note the way the coverage command counts it, through
// judge.Reaches over its body links and its frontmatter references; a concept
// reached only by other notes is still waiting to be placed on one. Notes
// outside the knowledge layer and declared non-instances are no concepts of
// this vault, and no index either.
func unmappedConcepts(notes []*vault.Note, cited map[string][]judge.Citation, idx *graph.Index, policy schema.ArtifactPolicy, roles schema.NavigationRoles, coverage *mapCoverage) []nav.NoteRef {
	if coverage.conceptType == "" {
		return nil
	}
	inLayer := func(n *vault.Note) bool {
		return n != nil && !policy.IsNonInstance(n.RelPath) && coverage.knowledge.Includes(n.RelPath)
	}
	mapped := make(map[string]bool)
	for _, n := range notes {
		if !inLayer(n) || !roles.IsMapType(n.Type()) {
			continue
		}
		names := slices.Concat(n.ListOrText(schema.FieldBasedOn), n.ListOrText("related"))
		for _, c := range cited[n.RelPath] {
			names = append(names, c.Target)
		}
		for _, path := range judge.Reaches(idx, names) {
			mapped[path] = true
		}
	}
	var out []nav.NoteRef
	for _, n := range notes {
		if !inLayer(n) || schema.NormalizeWord(n.Type()) != coverage.conceptType || mapped[n.RelPath] {
			continue
		}
		out = append(out, nav.NoteRef{Name: graph.TitleOf(n), RelPath: n.RelPath})
	}
	slices.SortFunc(out, func(a, b nav.NoteRef) int { return vault.ComparePaths(a.RelPath, b.RelPath) })
	return out
}

// collisions lists every name several files claim and some note cites. It asks
// the resolution index which names collide and the citations which of them
// matter, with the predicate the judge's name-collision rule applies, so the
// page and the gate count the same names: two files sharing a name nobody
// writes a link against break nothing, and a course vault repeats main.go in
// every lab. Which of them count is the resolver's own answer.
func collisions(idx *graph.Index, cited map[string][]judge.Citation) []HealthCollision {
	targets := func(yield func(string) bool) {
		for _, citations := range cited {
			for _, c := range citations {
				if !yield(c.Target) {
					return
				}
			}
		}
	}
	byName := graph.WithoutRestatedNames(graph.LinkedNames(idx.Collisions(), targets))
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
		return cmp.Or(vault.ComparePaths(a.From.RelPath, b.From.RelPath), vault.ComparePaths(a.Target, b.Target),
			vault.ComparePaths(a.Note.RelPath, b.Note.RelPath))
	})
	slices.SortFunc(h.Collisions, func(a, b HealthCollision) int {
		return vault.ComparePaths(a.Name, b.Name)
	})
}
