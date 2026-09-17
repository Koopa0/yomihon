package lexical

import (
	"maps"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/koopa0/yomihon/internal/vault"
)

// Filter is one structured constraint: a fixed key and its value, folded except
// for a folder value, which is held as the reader wrote it because matching
// folds it one directory name at a time.
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

// facetKeys are the keys an answer is divided along, in the order a reader
// meets them. It stands beside the grammar table because a key can only be
// offered as a division of an answer if the grammar accepts it as a filter,
// and because what keeps the list this short is size rather than any property
// of the vocabulary: a note declares one type, one status and one domain,
// while it may declare many topics and lives at one of hundreds of addresses,
// and a column of two hundred subjects narrows nothing. A test holds this set
// inside the grammar's own.
var facetKeys = []string{StatusFilterKey, "type", "domain"}

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
// colon; every other token is a folded bare token. A filter value is folded
// here, once, the way a token is; a "folder:" value instead drops a trailing
// slash and stays as written, because matching folds it a directory name at a
// time. A span in matched quotes — ASCII or the full-width pairs —
// is one bare token, whitespace and all, so its words match only where they sit
// together, and a run of whitespace in it matches any run in the note; an
// unpartnered quote is dropped. Quoting a key asks for those characters as
// text, while quoting only the value leaves a filter standing.
func Parse(q string) *Query {
	var out Query
	for _, field := range quoteFields(q) {
		key, value, reading := splitFilter(field.text, field.quotedFrom)
		if reading == readAsFilter {
			if key != "folder" {
				value = fold(value)
			}
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
//
// from and to bound the field in the raw query it was read out of, quotes
// included. text is not that stretch: quotes are stripped out of it and a
// grouped span holds whitespace the splitter would otherwise have cut at. A
// caller rewriting the query works from the offsets, because rejoining the
// fields it kept would spell the reader's own words back at them with the
// quoting and the spacing this splitter chose rather than the ones they typed.
type queryField struct {
	text       string
	quotedFrom int
	from, to   int
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
	from := -1
	flush := func(to int) {
		if strings.TrimSpace(b.String()) != "" {
			fields = append(fields, queryField{text: b.String(), quotedFrom: quotedFrom, from: from, to: to})
		}
		b.Reset()
		quotedFrom = -1
		from = -1
	}
	for i := 0; i < len(q); {
		if inner, next, ok := groupAt(q, i, b.String()); ok {
			if quotedFrom < 0 {
				quotedFrom = b.Len()
			}
			if from < 0 {
				from = i
			}
			b.WriteString(inner)
			i = next
			continue
		}
		r, size := utf8.DecodeRuneInString(q[i:])
		if unicode.IsSpace(r) {
			flush(i)
			i += size
			continue
		}
		if from < 0 {
			from = i
		}
		// The original bytes pass through untouched — folding is the token's
		// business, not the splitter's — so a byte that is not valid UTF-8
		// survives here exactly as strings.FieldsSeq would keep it.
		b.WriteString(q[i : i+size])
		i += size
	}
	flush(len(q))
	return fields
}

// groupAt reports the quoted span opening at i: the characters inside it, where
// scanning resumes after its closing partner, and whether there was one at all.
// A quote that opens no group — one standing mid-field, or one whose partner
// never arrives at a field boundary — is an ordinary character, and the caller
// writes it through.
func groupAt(q string, i int, fieldSoFar string) (inner string, next int, ok bool) {
	r, size := utf8.DecodeRuneInString(q[i:])
	closer, quoted := quotePairs[r]
	if !quoted || !quoteMayGroup(fieldSoFar) {
		return "", 0, false
	}
	j := groupCloser(q[i+size:], closer)
	if j < 0 {
		return "", 0, false
	}
	return q[i+size : i+size+j], i + size + j + utf8.RuneLen(closer), true
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

// WithFilter returns raw with f written on the end of it, and reports whether
// the result reads back as the same query plus exactly that one constraint.
//
// Every byte the reader typed stays where it stood, because the constraint is
// appended rather than spliced in. The report is not a formality: a value whose
// own quoting closes a group early comes back as a shorter constraint and a
// stray word, and a surface offering the link anyway would send the reader to
// an answer that is not the one the offer counted. The check runs the candidate
// back through the parser instead of reasoning about which characters are safe,
// so it cannot disagree with the grammar it is protecting.
func WithFilter(raw string, f Filter) (string, bool) {
	written := spellFilter(f)
	out := written
	if raw != "" {
		out = raw + written
		if last, _ := utf8.DecodeLastRuneInString(raw); !unicode.IsSpace(last) {
			out = raw + " " + written
		}
	}
	before, after := Parse(raw), Parse(out)
	if !slices.Equal(before.tokens, after.tokens) || !slices.Equal(before.unknownKeys, after.unknownKeys) {
		return "", false
	}
	want := append(slices.Clip(before.filters), Parse(written).filters...)
	if len(want) != len(before.filters)+1 || !slices.Equal(want, after.filters) {
		return "", false
	}
	return out, true
}

// WithoutFilter returns raw with every field that reads as f cut out of it. The
// bytes on either side are the reader's own: their spacing, their quoting and
// their capitalisation all stand, because the cut is made at the offsets the
// splitter recorded rather than by taking the query apart and writing it out
// again. Whitespace goes with the field it separated — the run in front of it,
// or the run behind it where the field opened the query — so removing a
// constraint from the middle of a query does not weld its neighbours together.
func WithoutFilter(raw string, f Filter) string {
	var out strings.Builder
	kept := 0
	for _, field := range quoteFields(raw) {
		key, value, reading := splitFilter(field.text, field.quotedFrom)
		if reading != readAsFilter || key != f.Key || !filterValuesEqual(key, value, f.Value) {
			continue
		}
		from, to := field.from, field.to
		if start := whitespaceRunEndingAt(raw, from); start < from {
			from = start
		} else {
			to += whitespaceRun(raw, to)
		}
		// Two constraints with nothing but one space between them both claim
		// that space: the first takes it as the run behind it, having none in
		// front, and the second then walks back into ground already cut. The
		// second claim yields, so the space is removed once and the cut stays
		// a forward walk.
		from = max(from, kept)
		out.WriteString(raw[kept:from])
		kept = to
	}
	if kept == 0 {
		return raw
	}
	out.WriteString(raw[kept:])
	return out.String()
}

// filterValuesEqual reports whether two values written for the same key name
// the same constraint. The comparison is the one matching makes, so a link
// removing "status:Draft" removes the "status:draft" a reader typed: both were
// asking the index the same question. A folder value is the exception the
// matcher already carries — it is compared a directory name at a time, so the
// as-written spellings are what stand here.
func filterValuesEqual(key, left, right string) bool {
	if key == "folder" {
		return vault.NormalizeNFC(left) == vault.NormalizeNFC(right)
	}
	return fold(left) == fold(right)
}

// spellFilter writes one constraint the way a reader would have to type it for
// this grammar to read it back: a value holding whitespace is quoted, because
// the bare characters would read as a filter plus a stray word.
func spellFilter(f Filter) string {
	if strings.IndexFunc(f.Value, unicode.IsSpace) < 0 {
		return f.Key + ":" + f.Value
	}
	return f.Key + `:"` + f.Value + `"`
}

// whitespaceRunEndingAt returns the offset where the run of whitespace ending
// at pos begins, or pos itself when the character in front of it is not
// whitespace.
func whitespaceRunEndingAt(s string, pos int) int {
	for pos > 0 {
		r, size := utf8.DecodeLastRuneInString(s[:pos])
		if !unicode.IsSpace(r) {
			break
		}
		pos -= size
	}
	return pos
}
