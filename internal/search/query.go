package search

import (
	"strings"

	"github.com/koopa0/kurodo/internal/graph"
)

// Filter is one structured constraint: a fixed key and its literal value.
type Filter struct {
	Key   string
	Value string
}

// Query is a parsed search query: folded bare tokens and structured filters,
// each in input order. A repeated filter key is an AND at the match layer,
// not last-wins.
type Query struct {
	Tokens  []string
	Filters []Filter
}

// isFilterKey reports whether key is exactly one of the six lowercase filter
// keys. The check is on the raw, pre-fold text: "Type:" or "TOPIC:" have
// a key outside this set, so the whole token is a bare token instead.
func isFilterKey(key string) bool {
	switch key {
	case "type", "status", "domain", "slug", "topic", "folder":
		return true
	default:
		return false
	}
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
	for raw := range strings.FieldsSeq(q) {
		if key, value, ok := splitFilter(raw); ok {
			out.Filters = append(out.Filters, Filter{Key: key, Value: value})
			continue
		}
		out.Tokens = append(out.Tokens, fold(raw))
	}
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
	value = graph.NormalizeNFC(rest)
	if key == "folder" {
		value = strings.TrimSuffix(value, "/")
	}
	return key, value, true
}
