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
	// unknownKeys are the words written before a colon that this grammar does
	// not accept, in input order and without repeats. They are kept apart from
	// the tokens they also became: the term is searched for as text, and the
	// page separately says the constraint the reader appeared to write was not
	// one this program knows.
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

// filterKeys is the grammar: every key a filter may be written with, and the
// capability each one asks of an entry.
//
// It is a table rather than a switch because the page that repairs a mistyped
// filter has to offer the reader this exact set, and a list kept beside it for
// that purpose is a list that drifts. The drift is not hypothetical — the
// ledger describing this very repair names four of these keys.
var filterKeys = map[string]filterKind{
	"type":   filterMetadata,
	"status": filterMetadata,
	"domain": filterMetadata,
	"topic":  filterMetadata,
	"slug":   filterMetadata,
	"folder": filterPath,
}

// classifyFilterKey is the shared grammar and capability classification for
// structured filters. Every recognized key receives a capability kind here.
func classifyFilterKey(key string) (filterKind, bool) {
	kind, ok := filterKeys[key]
	return kind, ok
}

// FilterKeys names every filter this grammar accepts, in a fixed order.
//
// The order is alphabetical because it has to be some order and every other
// one would be a second thing to maintain: a list curated by how often a key
// is used is a claim about readers that nothing here can check, and it would
// go stale silently. What matters is that the set is the parser's own.
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

// Parse turns a raw query string into a Query under six rules:
//
//   - classify before folding: a token is a filter only if its pre-fold key
//     is exactly one of the six lowercase keys; otherwise it is a bare token,
//     folded afterward.
//   - split on the first colon ("slug:a:b" → key "slug", value "a:b").
//   - filter values are NFC only (not case-folded); bare tokens are fold()ed.
//   - a "folder:" value drops one trailing slash.
//   - a span in matched quotes — ASCII double quotes or the full-width 「」
//     and 『』 pairs — is one bare token with the quotes stripped, whitespace
//     and all. Matching keeps the token contiguous, so the one token is what
//     makes the quoted words match only where they sit together; a run of
//     whitespace between them matches any run of whitespace in the note, the
//     line break of a wrapped sentence included. The indexed text separates
//     one block from the next with a single break too, so a phrase can also
//     join the last words of a heading to the first words of the paragraph
//     under it — the price of answering the wrapped sentence, which is the
//     shape most of this vault's prose has. A quote with no partner is
//     dropped where it stands.
//   - quoting a key asks for those characters as text, so "type:lesson" in
//     quotes is a phrase and not a filter; quoting only the value leaves the
//     filter standing, which is how a value carrying a space — a topic of two
//     words — is asked for at all.
//
// Whitespace tokenizes outside quotes; a whitespace-only or empty query
// yields an empty Query (nil slices), which the match layer treats as
// "return nothing".
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

// queryField is one whitespace-delimited unit of a raw query. quotedFrom is
// the offset in text where quoting first began, or -1 when nothing in the
// field was quoted.
//
// Where the quoting began is what decides how the field reads. Quoting a key —
// or the colon after it — asks for those characters as text, so the field is
// text. Quoting only the value leaves the key spelled plainly and still asks
// for a filter, which is how a value with a space in it is written at all:
// spelled without the quotes it splits into two fields, and quoted whole it
// stops being a filter.
type queryField struct {
	text       string
	quotedFrom int
}

// quotePairs maps each opening quote character to its closing partner. ASCII
// double quotes close themselves; the full-width corner brackets of Chinese
// and Japanese prose close with their distinct partners.
var quotePairs = map[rune]rune{'"': '"', '「': '」', '『': '』'}

// quoteFields splits q into fields the way strings.FieldsSeq does, except
// that a span enclosed in a matched quote pair keeps its whitespace inside one
// field and the quote characters themselves are stripped. A field that is
// empty or whitespace-only is not emitted: an empty token would match
// everything.
//
// A quote does that only where it is grouping something: at the start of a
// field, or straight after one of the recognised filter keys and its colon.
// Everywhere else it is a character in the text like any other, and so is a
// closer with no opener and an opener with no closer.
//
// The rule exists because the alternative silently answered the wrong
// question. Stripping a quote wherever it stood spliced together the two spans
// it separated: `cause="lease epoch mismatch"` was searched for as
// `cause=lease epoch mismatch`, which is not what the note holds, so a reader
// pasting a line out of their own vault was told it was not there — and the
// page echoed their query back with the quotes it had already discarded. This
// vault's prose is mostly Chinese, where 「」 sit flush against the words they
// quote with no space anywhere near them, so every quoted phrase pasted
// verbatim hit that splice.
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
// is opening a group rather than standing in the text. Nothing before it means
// the field starts here; a recognised filter key and its colon before it is
// the one other place a phrase is written, and it is why the check is on the
// key rather than simply on "some punctuation".
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

// groupCloser returns the byte offset in rest of the closer that ends a
// group — the first one standing at a field boundary, meaning end of query or
// followed by whitespace — or -1 when the group is never closed there. A
// closer with a word pressed against it is closing nothing; it is the
// punctuation inside somebody's sentence.
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

// splitFilter classifies one raw token: on the first colon it splits
// key/value, and returns a filter only when the key is one of the six and was
// written outside quotes — quotedFrom is where quoting began in raw, or -1
// when it did not. The value is NFC-normalized, and a "folder:" value drops
// one trailing slash. A non-filter token returns ok=false so the caller folds
// it as a bare token.
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
		// Shaped like a filter and not one. The term is still searched for as
		// text, which is all this grammar can honestly do with it, but the
		// reader is told — a term that looks like a constraint and quietly is
		// not returns the same page a search for nothing returns.
		return key, "", readAsUnknownFilter
	}
	value = vault.NormalizeNFC(rest)
	if key == "folder" {
		value = strings.TrimSuffix(value, "/")
	}
	return key, value, readAsFilter
}

// filterReading is what one query field turned out to be. The three outcomes
// are distinguished because they call for three different things: a filter is
// applied, a term is searched for, and a term shaped like a filter is searched
// for and said out loud.
type filterReading uint8

const (
	readAsText filterReading = iota
	readAsUnknownFilter
	readAsFilter
)
