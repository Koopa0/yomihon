package lexical

import (
	"strings"
	"unicode"
)

// NearWord is the word in the vault's titles that a mistyped query is one
// edit from: a letter added, left out, changed, or two neighbours swapped. It
// answers a query that is a single word of letters, and is empty when no title
// word is that close. Of several, the one more titles use wins, so the offer
// is the spelling the vault itself favours.
func (idx *Index) NearWord(query string) string {
	typed := []rune(strings.ToLower(strings.TrimSpace(query)))
	if len(typed) < 3 || !allLetters(typed) {
		return ""
	}
	uses := map[string]int{}
	for _, e := range idx.entries {
		if e.isFile {
			continue
		}
		for _, w := range titleWords(e.Title) {
			uses[w]++
		}
	}
	if uses[string(typed)] > 0 {
		return ""
	}
	best, bestUses := "", 0
	for w, n := range uses {
		if !oneEditApart(typed, []rune(w)) {
			continue
		}
		if n > bestUses || n == bestUses && w < best {
			best, bestUses = w, n
		}
	}
	return best
}

func allLetters(r []rune) bool {
	for _, c := range r {
		if c > unicode.MaxASCII || !unicode.IsLetter(c) {
			return false
		}
	}
	return true
}

// titleWords are the runs of ASCII letters in a title, lower-cased. Other
// scripts are not cut into words here, so they are never offered.
func titleWords(title string) []string {
	return strings.FieldsFunc(strings.ToLower(title), func(c rune) bool {
		return c > unicode.MaxASCII || !unicode.IsLetter(c)
	})
}

// oneEditApart reports whether a and b differ by exactly one insertion,
// deletion, substitution or swap of two neighbours.
func oneEditApart(a, b []rune) bool {
	if len(a) > len(b) {
		a, b = b, a
	}
	if len(b)-len(a) > 1 {
		return false
	}
	i := 0
	for i < len(a) && a[i] == b[i] {
		i++
	}
	if i == len(a) && len(a) == len(b) {
		return false
	}
	switch {
	case len(a) == len(b):
		if slicesEqual(a[i+1:], b[i+1:]) {
			return true
		}
		return i+1 < len(a) && a[i] == b[i+1] && a[i+1] == b[i] && slicesEqual(a[i+2:], b[i+2:])
	default:
		return slicesEqual(a[i:], b[i+1:])
	}
}

func slicesEqual(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
