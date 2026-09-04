package lexical

import (
	"maps"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/koopa0/yomihon/internal/vault"
)

// Filter is one structured constraint: a fixed key and its literal value.
type Filter struct {
	Key   string
	Value string
}

// Query is a parsed search query: folded bare tokens and structured filters,
// each in input order. A repeated filter key is an AND at the match layer, not
// last-wins.
type Query struct {
	tokens  []string
	filters []Filter
	// unknownKeys are the words written before a colon that this grammar does not
	// accept, in input order and without repeats. The term is still searched for
	// as text; this is what lets a page say the constraint was not one it knows.
	unknownKeys []string
}

// Tokens returns the folded bare terms in input order.
func (q *Query) Tokens() []string {
	return slices.Clone(q.tokens)
}

// Filters returns the structured constraints in input order.
func (q *Query) Filters() []Filter {
	return slices.Clone(q.filters)
}

// UnknownFilterKeys returns the keys of terms shaped like filters that this
// grammar does not accept, in input order. An empty result is the ordinary
// case: nothing in the query looked like a constraint that was not one.
func (q *Query) UnknownFilterKeys() []string {
	return slices.Clone(q.unknownKeys)
}

// RequiresMetadata reports whether evaluating the query needs frontmatter.
func (q *Query) RequiresMetadata() bool {
	for _, f := range q.filters {
		kind, ok := classifyFilterKey(f.Key)
		if ok && kind == filterMetadata {
			return true
		}
	}
	return false
}

type filterKind uint8

const (
	filterUnknown filterKind = iota
	filterMetadata
	filterPath
)

// StatusFilterKey is the key a status filter is written with. It is exported
// because a page builds that query for a reader — the lifecycle squares link to
// one — and a page spelling it itself would keep working until this table
// renamed the key, at which point the reader would follow a link into the
// search's own "I do not recognise that filter" answer.
const StatusFilterKey = "status"

// filterKeys is the grammar: every key a filter may be written with, and the
// capability each asks of an entry. It is a table rather than a switch because
// the page that repairs a mistyped filter has to offer this exact set.
var filterKeys = map[string]filterKind{
	"type":          filterMetadata,
	StatusFilterKey: filterMetadata,
	"domain":        filterMetadata,
	"topic":         filterMetadata,
	"slug":          filterMetadata,
	"folder":        filterPath,
}

// classifyFilterKey is the shared grammar and capability classification for
// structured filters. Every recognized key receives a capability kind here.
func classifyFilterKey(key string) (filterKind, bool) {
	kind, ok := filterKeys[key]
	return kind, ok
}

// FilterKeys names every filter this grammar accepts, alphabetically. What
// matters is that the set is the parser's own.
func FilterKeys() []string {
	keys := slices.Collect(maps.Keys(filterKeys))
	slices.Sort(keys)
	return keys
}

// isFilterKey reports whether key is exactly one of the six lowercase filter
// keys. The check is on the raw, pre-fold text: "Type:" or "TOPIC:" have
// a key outside this set, so the whole token is a bare token instead.
func isFilterKey(key string) bool {
	_, ok := classifyFilterKey(key)
	return ok
}

// Parse turns a raw query string into a Query. A token is a filter only if its
// pre-fold key is exactly one of the six lowercase keys, split on the first
// colon; every other token is a folded bare token. A filter value is NFC only,
// and a "folder:" value drops a trailing slash. A span in matched quotes — ASCII
// or the full-width pairs — is one bare token, whitespace and all, so its words
// match only where they sit together, and a run of whitespace in it matches any
// run in the note; an unpartnered quote is dropped. Quoting a key asks for those
// characters as text, while quoting only the value leaves a filter standing.
func Parse(q string) *Query {
	var out Query
	for _, field := range quoteFields(q) {
		key, value, reading := splitFilter(field.text, field.quotedFrom)
		if reading == readAsFilter {
			out.filters = append(out.filters, Filter{Key: key, Value: value})
			continue
		}
		if reading == readAsUnknownFilter && !slices.Contains(out.unknownKeys, key) {
			out.unknownKeys = append(out.unknownKeys, key)
		}
		out.tokens = append(out.tokens, fold(field.text))
	}
	return &out
}

// queryField is one whitespace-delimited unit of a raw query. quotedFrom is where
// quoting first began, or -1, and it decides how the field reads: quoting the key
// or its colon makes the field text, quoting only the value leaves a filter.
type queryField struct {
	text       string
	quotedFrom int
}

// quotePairs maps each opening quote character to its closing partner. ASCII
// double quotes close themselves; the full-width corner brackets of Chinese
// and Japanese prose close with their distinct partners.
var quotePairs = map[rune]rune{'"': '"', '「': '」', '『': '』'}

// quoteFields splits q into fields the way strings.FieldsSeq does, except that a
// span in a matched quote pair keeps its whitespace inside one field and the
// quotes themselves are stripped; an empty field is not emitted. A quote groups
// only at the start of a field or straight after a recognised filter key and its
// colon, because stripping one wherever it stood spliced the spans it separated.
func quoteFields(q string) []queryField {
	var fields []queryField
	var b strings.Builder
	quotedFrom := -1
	flush := func() {
		if strings.TrimSpace(b.String()) != "" {
			fields = append(fields, queryField{text: b.String(), quotedFrom: quotedFrom})
		}
		b.Reset()
		quotedFrom = -1
	}
	for i := 0; i < len(q); {
		r, size := utf8.DecodeRuneInString(q[i:])
		if closer, ok := quotePairs[r]; ok && quoteMayGroup(b.String()) {
			if j := groupCloser(q[i+size:], closer); j >= 0 {
				if quotedFrom < 0 {
					quotedFrom = b.Len()
				}
				b.WriteString(q[i+size : i+size+j])
				i += size + j + utf8.RuneLen(closer)
				continue
			}
		}
		if unicode.IsSpace(r) {
			flush()
			i += size
			continue
		}
		// The original bytes pass through untouched — folding is the token's
		// business, not the splitter's — so a byte that is not valid UTF-8
		// survives here exactly as strings.FieldsSeq would keep it.
		b.WriteString(q[i : i+size])
		i += size
	}
	flush()
	return fields
}

// quoteMayGroup reports whether a quote appearing after the field text so far
// opens a group rather than standing in the text. Nothing before it means the
// field starts here; a recognised filter key and its colon is the one other place.
func quoteMayGroup(fieldSoFar string) bool {
	if fieldSoFar == "" {
		return true
	}
	key, rest, found := strings.Cut(fieldSoFar, ":")
	if !found || rest != "" {
		return false
	}
	return isFilterKey(key)
}

// groupCloser returns the byte offset in rest of the closer that ends a group —
// the first at a field boundary, meaning end of query or followed by whitespace —
// or -1. A closer with a word pressed against it is punctuation in a sentence.
func groupCloser(rest string, closer rune) int {
	for at := 0; at < len(rest); {
		j := strings.IndexRune(rest[at:], closer)
		if j < 0 {
			return -1
		}
		j += at
		after := j + utf8.RuneLen(closer)
		if after == len(rest) {
			return j
		}
		next, _ := utf8.DecodeRuneInString(rest[after:])
		if unicode.IsSpace(next) {
			return j
		}
		at = after
	}
	return -1
}

// splitFilter classifies one raw token: it splits key/value on the first colon
// and returns a filter only when the key is one of the six and was written outside
// quotes, quotedFrom being where quoting began or -1. The value is NFC-normalized;
// a non-filter token returns ok=false so the caller folds it as a bare token.
func splitFilter(raw string, quotedFrom int) (key, value string, reading filterReading) {
	key, rest, found := strings.Cut(raw, ":")
	if !found {
		return "", "", readAsText
	}
	if quotedFrom >= 0 && quotedFrom <= len(key) {
		// The key, or the colon standing after it, came out of quotes: the
		// reader asked for those characters, not for a constraint.
		return "", "", readAsText
	}
	if !isFilterKey(key) {
		// Shaped like a filter and not one: still searched for as text, but said
		// out loud, since a constraint that quietly is not one returns nothing.
		return key, "", readAsUnknownFilter
	}
	value = vault.NormalizeNFC(rest)
	if key == "folder" {
		value = strings.TrimSuffix(value, "/")
	}
	return key, value, readAsFilter
}

// filterReading is what one query field turned out to be: a filter is applied, a
// term is searched for, and a term shaped like a filter is searched for and said
// out loud.
type filterReading uint8

const (
	readAsText filterReading = iota
	readAsUnknownFilter
	readAsFilter
)
