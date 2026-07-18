package judge

import (
	"fmt"
	"slices"
	"strings"
)

// The exists oracle answers "does a note for this name already exist?" for a
// dedup check before writing. It is deliberately wider than the resolver: a
// writer searches by filename, title, alias, or English title, so all four are
// matched, even though the resolver never resolves by a title. Each hit reports
// which field matched, so a caller can tell a real duplicate from a
// merely-similar name. The error cost is the opposite of the resolver's — a
// false "no" makes an agent write a duplicate — so this over-recalls. The shape
// is part of the frozen output.

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
}

// found reports whether any note exposes the queried name.
func (r existsReport) found() bool {
	return len(r.Matches) > 0
}

// existsLookup looks query up across every note's filename, title, aliases, and
// English title, normalizing both sides the same way the resolver keys names.
// A note that exposes the name on more than one field yields one match per
// field. Matches are ordered by path, then field.
func existsLookup(notes []note, query string, authority scanAuthority) existsReport {
	key := normalizeKey(query)
	matches := []existsMatch{}
	for i := range notes {
		n := &notes[i]
		if !authority.egressAllowed(n.path) {
			// A contract-private note never surfaces through the existence
			// oracle: a dedup query must not learn its name, path, or alias.
			continue
		}
		matches = append(matches, noteMatches(n, key)...)
	}
	slices.SortStableFunc(matches, func(a, b existsMatch) int {
		if c := strings.Compare(a.Path, b.Path); c != 0 {
			return c
		}
		return strings.Compare(a.Field, b.Field)
	})
	return existsReport{Query: query, Matches: matches}
}

// noteMatches returns every field of n that exposes the normalized key: its
// filename stem, full filename, title, each alias, and English title.
func noteMatches(n *note, key string) []existsMatch {
	var matches []existsMatch
	stem := filenameStem(n.path)
	if normalizeKey(stem) == key {
		matches = append(matches, existsMatch{Path: n.path, Field: "filename", Value: stem})
	}
	// A full filename also matches, so a caller that built a candidate filename
	// (with its extension) never gets a false "not found".
	full := filename(n.path)
	if full != stem && normalizeKey(full) == key {
		matches = append(matches, existsMatch{Path: n.path, Field: "filename", Value: full})
	}
	if n.title != "" && normalizeKey(n.title) == key {
		matches = append(matches, existsMatch{Path: n.path, Field: "title", Value: n.title})
	}
	for _, alias := range n.aliases {
		if normalizeKey(alias) == key {
			matches = append(matches, existsMatch{Path: n.path, Field: "alias", Value: alias})
		}
	}
	if n.titleEn != "" && normalizeKey(n.titleEn) == key {
		matches = append(matches, existsMatch{Path: n.path, Field: "title_en", Value: n.titleEn})
	}
	return matches
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

// renderExists renders an existence query for a terminal reader. The query is
// echoed inside literal quotes and never escaped, so its own bytes read back
// unchanged.
func renderExists(r existsReport) string {
	if !r.found() {
		return "\"" + r.Query + "\" does not exist\n"
	}
	var s strings.Builder
	fmt.Fprintf(&s, "\"%s\" exists in %d note(s):\n", r.Query, len(r.Matches))
	for _, m := range r.Matches {
		fmt.Fprintf(&s, "  %s (matched %s)\n", m.Path, m.Field)
	}
	return s.String()
}
