package graph

import (
	"strconv"
	"strings"
	"unicode"
)

// GitHubAnchors lists the id GitHub gives each heading of a document, in
// order, for the names of its headings as displayed. A link written for GitHub
// or an editor that follows it carries these ids, which differ from SectionID
// wherever a heading has punctuation: GitHub deletes it where SectionID
// separates the words around it. A heading name an earlier one already took
// gets "-1", then "-2", where a page here counts on from "-2".
//
// The result is folded the way a fragment is, so it compares with one.
func GitHubAnchors(names []string) []string {
	taken := make(map[string]int, len(names))
	out := make([]string, len(names))
	for i, name := range names {
		base := githubSlug(name)
		slug := base
		for {
			if _, used := taken[slug]; !used {
				break
			}
			taken[base]++
			slug = base + "-" + strconv.Itoa(taken[base])
		}
		taken[slug] = 0
		out[i] = FoldFragment(slug)
	}
	return out
}

// githubSlug lowercases a heading name, drops everything but letters, marks,
// numbers, connector punctuation and hyphens, and turns each space into a
// hyphen, so two spaces make two hyphens and a name that was all punctuation
// leaves an empty slug.
func githubSlug(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		switch {
		case r == ' ':
			b.WriteByte('-')
		case r == '-' || unicode.IsLetter(r) || unicode.IsMark(r) || unicode.IsNumber(r) || unicode.Is(unicode.Pc, r):
			b.WriteRune(r)
		}
	}
	return b.String()
}
