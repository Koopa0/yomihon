package judge

import (
	"fmt"
	"slices"
	"strings"

	"github.com/koopa0/yomihon/internal/lexical"
)

// The exists oracle answers "does a note for this name already exist?" for a
// dedup check before writing. It is deliberately wider than the resolver,
// matching filename, title, alias and English title, and each hit reports which
// field matched. A false "no" would make a caller write a duplicate, so it
// over-recalls. Fold-equal names — a fullwidth colon beside its ASCII twin —
// are reported as near matches, distinct from an exact hit, and do not flip
// the exit code. The shape is part of the frozen output.

// existsMatch is one note that exposes the queried name, and the field it
// matched on.
type existsMatch struct {
	Path  string `json:"path"`
	Field string `json:"field"`
	Value string `json:"value"`
}

// existsReport is the result of an existence query.
type existsReport struct {
	Query   string        `json:"query"`
	Matches []existsMatch `json:"matches"`
	// NearMatches are notes that answer under the search index's fold
	// (fullwidth ASCII narrowed, then lowercase) but not under the resolver
	// key. They are omitted when the second pass finds nothing, so an
	// ordinary answer's bytes stay unchanged — the same omitempty contract
	// Withheld already ships under.
	NearMatches []existsMatch `json:"near_matches,omitempty"`
	// Withheld says a note the contract keeps out of agent-facing output
	// answers to the name. It carries nothing else, because the caller needs
	// only two warnings: do not create a second note under the name, and do not
	// assume a link to it resolves cleanly. The field is omitted when nothing
	// was withheld, so an ordinary answer's bytes are unchanged.
	Withheld bool `json:"withheld,omitempty"`
}

// found reports whether any note answers to the queried name, including one
// this command may not describe. A withheld note is a note: answering "absent"
// for one would have a caller gating on the exit code put a second,
// describable note under a private note's own name. A near match is not an
// answer: flipping found() for one would change the exit code a write-if-absent
// gate already depends on.
func (r existsReport) found() bool {
	return len(r.Matches) > 0 || r.Withheld
}

// existsLookup looks query up across every note's filename, title, aliases, and
// English title. The first pass keys both sides the way the resolver keys
// names. A second pass runs the same fields through lexical.Fold, and any hit
// that was not already exact is a near match. A note that exposes the name on
// more than one field yields one match per field. Matches are ordered by path,
// then field. title_en is matched only when the contract declares it for that
// note's type — fields.known, or a per-type list such as fields.lesson_only on
// a lesson. A field check would call unknown is not a name this oracle may
// report.
func existsLookup(notes []note, query string, authority scanAuthority) existsReport {
	key := normalizeKey(query)
	foldKey := lexical.Fold(query)
	matches := []existsMatch{}
	var near []existsMatch
	withheld := false
	for i := range notes {
		n := &notes[i]
		known := knownFrontmatter(authority, n.noteType)
		exact := noteMatches(n, key, known)
		if !authority.egressAllowed(n.path) {
			// A contract-private note never describes itself here: no path, no
			// field, no value. That it answers to the name is still reported,
			// because the alternative tells the caller the name is free. A
			// fold-only hit is not an answer and must not flip withheld: that
			// would change the exit code the same way flipping found() would.
			if len(exact) > 0 {
				withheld = true
			}
			continue
		}
		matches = append(matches, exact...)
		near = append(near, withoutExact(noteMatchesFolded(n, foldKey, known), exact)...)
	}
	sortExistsMatches(matches)
	sortExistsMatches(near)
	return existsReport{Query: query, Matches: matches, NearMatches: near, Withheld: withheld}
}

// noteMatches returns every field of n that exposes the normalized key: its
// filename stem, full filename, title, each alias, and English title when
// the contract declares title_en for this note's type.
func noteMatches(n *note, key string, known []string) []existsMatch {
	return collectFieldMatches(n, key, known, normalizeKey)
}

// noteMatchesFolded is the second pass: the same fields, keyed through the
// search index's fold rather than the resolver key.
func noteMatchesFolded(n *note, key string, known []string) []existsMatch {
	return collectFieldMatches(n, key, known, lexical.Fold)
}

func collectFieldMatches(n *note, key string, known []string, fold func(string) string) []existsMatch {
	var matches []existsMatch
	stem := filenameStem(n.path)
	if fold(stem) == key {
		matches = append(matches, existsMatch{Path: n.path, Field: "filename", Value: stem})
	}
	// A full filename also matches, so a caller that built a candidate filename
	// (with its extension) never gets a false "not found".
	full := filename(n.path)
	if full != stem && fold(full) == key {
		matches = append(matches, existsMatch{Path: n.path, Field: "filename", Value: full})
	}
	if n.title != "" && fold(n.title) == key {
		matches = append(matches, existsMatch{Path: n.path, Field: "title", Value: n.title})
	}
	for _, alias := range n.aliases {
		if fold(alias) == key {
			matches = append(matches, existsMatch{Path: n.path, Field: "alias", Value: alias})
		}
	}
	if n.titleEn != "" && slices.Contains(known, "title_en") && fold(n.titleEn) == key {
		matches = append(matches, existsMatch{Path: n.path, Field: "title_en", Value: n.titleEn})
	}
	return matches
}

func withoutExact(folded, exact []existsMatch) []existsMatch {
	if len(folded) == 0 || len(exact) == 0 {
		return folded
	}
	seen := make(map[existsMatch]struct{}, len(exact))
	for _, m := range exact {
		seen[m] = struct{}{}
	}
	var out []existsMatch
	for _, m := range folded {
		if _, ok := seen[m]; !ok {
			out = append(out, m)
		}
	}
	return out
}

func sortExistsMatches(matches []existsMatch) {
	slices.SortStableFunc(matches, func(a, b existsMatch) int {
		if c := strings.Compare(a.Path, b.Path); c != 0 {
			return c
		}
		return strings.Compare(a.Field, b.Field)
	})
}

// knownFrontmatter is the contract's declared frontmatter set for one note
// type: the shared fields.known list, plus the per-type list only when that
// type is the one the list applies to (today fields.lesson_only on a lesson).
// A nil contract declares no field, so title_en cannot match.
func knownFrontmatter(authority scanAuthority, noteType string) []string {
	if authority.contract == nil {
		return nil
	}
	fields := authority.contract.Definition().Fields
	lessonType, _ := authority.contract.LessonType()
	if noteType == lessonType {
		return slices.Concat(fields.Known, fields.LessonOnly)
	}
	return fields.Known
}

// filename is the last path segment of a vault-relative, forward-slash path.
func filename(path string) string {
	if i := strings.LastIndexByte(path, '/'); i >= 0 {
		return path[i+1:]
	}
	return path
}

// filenameStem is a note's filename without its ".md" extension; a non-md
// resource keeps its full name, since only the markdown extension is a note's.
func filenameStem(path string) string {
	return strings.TrimSuffix(filename(path), ".md")
}

// notesAmong counts the notes the matches name, not the matches. One note
// answers to a name once per way it carries it — a filename and a title, a
// title and an alias — so counting rows told a reader two notes hold a name and
// then printed one path twice. The reader of this line is deciding whether to
// write a note under that name, and the count is the part of the answer they
// act on.
func notesAmong(matches []existsMatch) int {
	seen := make(map[string]struct{}, len(matches))
	for _, m := range matches {
		seen[m.Path] = struct{}{}
	}
	return len(seen)
}

// renderExists renders an existence query for a terminal reader. The query is
// echoed inside literal quotes and never escaped, so its own bytes read back
// unchanged.
func renderExists(r existsReport) string {
	var s strings.Builder
	if !r.found() {
		fmt.Fprintf(&s, "\"%s\" does not exist\n", r.Query)
	}
	if len(r.Matches) > 0 {
		fmt.Fprintf(&s, "\"%s\" exists in %d note(s):\n", r.Query, notesAmong(r.Matches))
		for _, m := range r.Matches {
			fmt.Fprintf(&s, "  %s (matched %s)\n", m.Path, m.Field)
		}
	}
	if len(r.NearMatches) > 0 {
		fmt.Fprintf(&s, "\"%s\" is near %d note(s):\n", r.Query, notesAmong(r.NearMatches))
		for _, m := range r.NearMatches {
			fmt.Fprintf(&s, "  %s (matched %s)\n", m.Path, m.Field)
		}
	}
	if r.Withheld {
		fmt.Fprintf(&s, "\"%s\" also answers to a note in a directory this vault withholds; it cannot be named here\n", r.Query)
	}
	return s.String()
}
