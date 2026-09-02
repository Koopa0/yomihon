package lexical

import (
	"slices"
	"strings"
	"unicode"
)

// StepBack is one looser query the empty page may offer: a real search this
// index has already answered, carried with its count. A suggestion is only
// ever built from searches that found something — the page once advised a
// filter that could not match, and the reader followed it into a second empty
// page wearing the same advice.
type StepBack struct {
	Query string
	Count int
}

// StepBacks proposes ways to loosen a query that found nothing. Three
// loosenings cover how a zero actually happens here: a quoted phrase demands
// its words sit together, so the same words are offered without their
// adjacency; a term is matched as one contiguous run, so a reader who types
// 20mg misses the note that says 20-40mg — splitting where letters meet
// digits turns the run into two terms the note does carry; and several terms
// must all land in one note, so each term is also offered alone. Every
// candidate is run before it is offered, because a zero here reads as "I
// never wrote that down", and advice that leads to another zero teaches the
// reader to distrust the page that gave it.
//
// Filter fields ride along unchanged on every candidate: the reader scoped the
// search deliberately, and loosening the words is not a licence to widen the
// scope.
func (idx *Index) StepBacks(raw string) []StepBack {
	var bare, filters []string
	quoted := false
	for _, field := range quoteFields(raw) {
		// A term shaped like a filter this grammar does not know is one of the
		// words being loosened, not a scope to carry along: it never narrowed
		// anything, so keeping it would hold back the very search being widened.
		if _, _, reading := splitFilter(field.text, field.quotedFrom); reading == readAsFilter {
			filters = append(filters, respellFilter(field.text))
			continue
		}
		if field.quotedFrom >= 0 {
			// A phrase loosens by shedding its adjacency, so its words join
			// the bare terms one by one, quote characters already stripped.
			quoted = true
			bare = append(bare, strings.Fields(field.text)...)
			continue
		}
		bare = append(bare, field.text)
	}
	if len(bare) == 0 {
		return nil
	}

	var candidates []string
	if quoted && len(bare) > 1 {
		candidates = append(candidates, strings.Join(bare, " "))
	}
	if split := splitDigitLetterRuns(bare); split != "" {
		candidates = append(candidates, split)
	}
	if len(bare) > 1 {
		candidates = append(candidates, bare...)
	}

	// The joined bare terms are the query as asked only when nothing was
	// quoted; for a quoted phrase that same join is the first loosening.
	seen := map[string]bool{}
	if !quoted {
		seen[strings.Join(bare, " ")] = true
	}
	var out []StepBack
	for _, candidate := range candidates {
		if seen[candidate] {
			continue
		}
		seen[candidate] = true
		full := strings.Join(append(slices.Clip(filters), candidate), " ")
		// A candidate is only offered for its count, so no result — and no
		// snippet — is ever materialized for it, and the count is the true
		// tally rather than the bounded page's opening stretch.
		_, total, err := idx.SearchN(Parse(full), 0)
		if err != nil || total == 0 {
			continue
		}
		out = append(out, StepBack{Query: full, Count: total})
		if len(out) == 4 {
			break
		}
	}
	return out
}

// respellFilter writes a filter back the way a reader would have to type it:
// the quotes around a value carrying a space are gone by the time a field is
// split, and a suggestion offering the bare characters would read back as a
// filter on the first word plus a stray second one. Every suggestion is run
// before it is offered, so a spelling that parses differently would cost the
// reader the count as well as the query.
func respellFilter(field string) string {
	key, value, _ := strings.Cut(field, ":")
	if strings.IndexFunc(value, unicode.IsSpace) < 0 {
		return field
	}
	return key + `:"` + value + `"`
}

// splitDigitLetterRuns rewrites the bare terms with a space wherever an ASCII
// digit run meets an ASCII letter run, and reports "" when nothing changed or
// when a split would shed a fragment too short to mean anything — a
// single-letter term matches half the vault, and a suggestion that hits for
// that reason is worse than none.
func splitDigitLetterRuns(bare []string) string {
	changed := false
	parts := make([]string, 0, len(bare))
	for _, term := range bare {
		split := splitTermRuns(term)
		if split == "" {
			parts = append(parts, term)
			continue
		}
		changed = true
		parts = append(parts, split)
	}
	if !changed {
		return ""
	}
	return strings.Join(parts, " ")
}

func splitTermRuns(term string) string {
	var runs []string
	start := 0
	for i := 1; i < len(term); i++ {
		if isASCIIDigit(term[i-1]) != isASCIIDigit(term[i]) && isBoundaryByte(term[i-1]) && isBoundaryByte(term[i]) {
			runs = append(runs, term[start:i])
			start = i
		}
	}
	if start == 0 {
		return ""
	}
	runs = append(runs, term[start:])
	for _, run := range runs {
		if len(run) < 2 {
			return ""
		}
	}
	return strings.Join(runs, " ")
}

func isASCIIDigit(b byte) bool { return b >= '0' && b <= '9' }

func isBoundaryByte(b byte) bool {
	return isASCIIDigit(b) || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}
