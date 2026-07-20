package search

import (
	"slices"
	"strings"

	"github.com/koopa0/yomihon/internal/vault"
)

// Filter is one structured constraint: a fixed key and its literal value.
type Filter struct {
	Key   string
	Value string
}

// Query is a parsed search query: folded bare tokens and structured filters,
// each in input order. BareText preserves the original bare tokens, joined by
// one ASCII space, for consumers whose semantics depend on the user's text
// rather than lexical folding. A repeated filter key is an AND at the match
// layer, not last-wins.
type Query struct {
	tokens   []string
	filters  []Filter
	bareText string
}

// Tokens returns the folded bare terms in input order.
func (q Query) Tokens() []string {
	return slices.Clone(q.tokens)
}

// Filters returns the structured constraints in input order.
func (q Query) Filters() []Filter {
	return slices.Clone(q.filters)
}

// BareText returns the original bare terms joined by one ASCII space.
func (q Query) BareText() string {
	return q.bareText
}

// RequiresMetadata reports whether evaluating the query needs frontmatter.
func (q Query) RequiresMetadata() bool {
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

// classifyFilterKey is the shared grammar and capability classification for
// structured filters. Every recognized key receives a capability kind here.
func classifyFilterKey(key string) (filterKind, bool) {
	switch key {
	case "type", "status", "domain", "topic", "slug":
		return filterMetadata, true
	case "folder":
		return filterPath, true
	default:
		return filterUnknown, false
	}
}

// isFilterKey reports whether key is exactly one of the six lowercase filter
// keys. The check is on the raw, pre-fold text: "Type:" or "TOPIC:" have
// a key outside this set, so the whole token is a bare token instead.
func isFilterKey(key string) bool {
	_, ok := classifyFilterKey(key)
	return ok
}

// Parse turns a raw query string into a Query under four rules:
//
//   - classify before folding: a token is a filter only if its pre-fold key
//     is exactly one of the six lowercase keys; otherwise it is a bare token,
//     folded afterward.
//   - split on the first colon ("slug:a:b" → key "slug", value "a:b").
//   - filter values are NFC only (not case-folded); bare tokens are fold()ed.
//   - a "folder:" value drops one trailing slash.
//
// Whitespace tokenizes; a whitespace-only or empty query yields an empty Query
// (nil slices), which the match layer treats as "return nothing".
func Parse(q string) Query {
	var out Query
	var bare []string
	for raw := range strings.FieldsSeq(q) {
		if key, value, ok := splitFilter(raw); ok {
			out.filters = append(out.filters, Filter{Key: key, Value: value})
			continue
		}
		out.tokens = append(out.tokens, fold(raw))
		bare = append(bare, raw)
	}
	out.bareText = strings.Join(bare, " ")
	return out
}

// splitFilter classifies one raw token: on the first colon it splits
// key/value, and returns a filter only when the key is one of the six. The
// value is NFC-normalized, and a "folder:" value drops one trailing slash.
// A non-filter token returns ok=false so the caller folds it as a bare
// token.
func splitFilter(raw string) (key, value string, ok bool) {
	key, rest, found := strings.Cut(raw, ":")
	if !found {
		return "", "", false
	}
	if !isFilterKey(key) {
		return "", "", false
	}
	value = vault.NormalizeNFC(rest)
	if key == "folder" {
		value = strings.TrimSuffix(value, "/")
	}
	return key, value, true
}
