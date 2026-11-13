package snapshot

import (
	"cmp"
	"slices"
	"strings"
	"time"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

// NotesOfType returns captured readable notes of one declared type, including
// notes outside the knowledge layer but never a declared non-instance, such as
// a template. It grants no role or lifecycle meaning; the caller supplies the
// type obtained from its contract authority.
func (g *Generation) NotesOfType(noteType string) []Reading {
	if g == nil || noteType == "" {
		return nil
	}
	var notes []Reading
	policy := g.ArtifactPolicy()
	for rel := range g.notes {
		reading := g.notes[rel]
		if schema.NormalizeWord(reading.Type) == noteType && !policy.IsNonInstance(reading.RelPath) {
			notes = append(notes, reading)
		}
	}
	slices.SortFunc(notes, func(a, b Reading) int { return vault.ComparePaths(a.RelPath, b.RelPath) })
	return notes
}

// BasedOnDeclarations returns the source words the note authored, including
// any section or block fragments. The detached list grants no link target.
func (g *Generation) BasedOnDeclarations(rel string) []string {
	if g == nil {
		return nil
	}
	return slices.Clone(basedOnValues(g.parsed[rel]))
}

// DeclaredTitle is the title rel's own frontmatter declares, empty when it
// declares none. A reading's Title falls back to the file name, which is a
// path rather than a title.
func (g *Generation) DeclaredTitle(rel string) string {
	if g == nil || g.parsed[rel] == nil {
		return ""
	}
	title, _ := g.parsed[rel].Text("title")
	return title
}

// NotText reports whether rel's frontmatter writes key with a value that is
// not text — a list, a mapping, a number or a date — so that nothing can be
// read from it as that field. An absent or empty key wrote nothing to misread
// and answers false.
func (g *Generation) NotText(rel, key string) bool {
	if g == nil || g.parsed[rel] == nil {
		return false
	}
	value, present := g.parsed[rel].Frontmatter[key]
	if !present || value == nil {
		return false
	}
	_, text := value.(string)
	return !text
}

// NotBoolean reports whether rel's frontmatter writes key with a value that is
// not a boolean — text, a number, a list — so that whether the flag is set
// cannot be read from it. An absent or empty key wrote nothing to misread and
// answers false.
func (g *Generation) NotBoolean(rel, key string) bool {
	if g == nil || g.parsed[rel] == nil {
		return false
	}
	value, present := g.parsed[rel].Frontmatter[key]
	if !present || value == nil {
		return false
	}
	_, boolean := value.(bool)
	return !boolean
}

// OpenMark is one location the reader marked "Not sure yet", as the selection
// of what the reader left open needs it: where, and when it was marked. The
// marks themselves are kept by the reading face; this is the copy it hands in.
type OpenMark struct {
	RelPath string
	Anchor  string
	At      time.Time
}

// OpenItem is one thing the reader has left open. Exactly one of Note and Mark
// is set: a note of the contract's answer type that declares draft: true, or a
// marked location. At is what orders it — the mark's time, or the note's
// own declared update date, zero when it declares none.
type OpenItem struct {
	Note *Reading
	Mark *OpenMark
	At   time.Time
}

// Path is the vault path the item names.
func (o *OpenItem) Path() string {
	if o.Mark != nil {
		return o.Mark.RelPath
	}
	return o.Note.RelPath
}

// OpenItems is the one selection of what the reader has left open, shared by
// every face that lists it: the notes of this generation's answer type that
// declare draft: true, interleaved with every mark
// handed in, newest first. A note with no declared date sorts after every dated
// item; ties fall to path order, then anchor. A mark is kept even when its note
// or its location is gone, because the stored identity is the reader's own record.
//
// It grants nothing about who may see an item: a caller writing for a program
// filters by its own authority before it resolves any item further.
func (g *Generation) OpenItems(marks []OpenMark) []OpenItem {
	var items []OpenItem
	role := g.NavigationRoles().AnswerType()
	notes := g.NotesOfType(role)
	for i := range notes {
		if notes[i].Draft {
			items = append(items, OpenItem{Note: &notes[i], At: notes[i].Updated})
		}
	}
	for i := range marks {
		items = append(items, OpenItem{Mark: &marks[i], At: marks[i].At})
	}
	slices.SortStableFunc(items, func(a, b OpenItem) int {
		return cmp.Or(b.At.Compare(a.At), vault.ComparePaths(a.Path(), b.Path()), strings.Compare(a.anchor(), b.anchor()))
	})
	return items
}

func (o *OpenItem) anchor() string {
	if o.Mark != nil {
		return o.Mark.Anchor
	}
	return ""
}

// NameWithin reports whether every note or file the wikilink target name
// could reach in this generation is a path allowed accepts: the one it
// resolves to, every candidate when it is ambiguous, and the name itself,
// read as a path with and without the note extension, when it resolves to
// nothing. A name a reader cannot tell apart from forbidden ground is not
// within, so a caller can withhold words that would describe that ground.
func (g *Generation) NameWithin(target string, allowed func(string) bool) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		return true
	}
	var resolved graph.Resolution
	if g != nil {
		resolved = g.Graph().Resolve(target)
	}
	switch resolved.Kind {
	case graph.KindUnique:
		return allowed(resolved.RelPath)
	case graph.KindAmbiguous:
		return !slices.ContainsFunc(resolved.Candidates, func(candidate string) bool { return !allowed(candidate) })
	default:
		name := vault.NormalizeNFC(target)
		return allowed(name) && allowed(name+".md")
	}
}

// BasedOnWithin reports whether every source rel declares in based_on names
// only ground allowed accepts, as NameWithin reads a name. A declaration that
// carries no name at all names no ground.
func (g *Generation) BasedOnWithin(rel string, allowed func(string) bool) bool {
	if g == nil {
		return true
	}
	for _, value := range basedOnValues(g.parsed[rel]) {
		if target, ok := basedOnTarget(strings.TrimSpace(value)); ok && !g.NameWithin(target, allowed) {
			return false
		}
	}
	return true
}
