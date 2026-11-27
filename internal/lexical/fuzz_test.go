package lexical

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/google/go-cmp/cmp"
)

// FuzzParse keeps the lexical grammar deterministic and ownership-safe across
// arbitrary query text. Every whitespace token must be classified exactly once
// as either bare text or one of the structured filters.
func FuzzParse(f *testing.F) {
	f.Add("")
	f.Add("深度 type:lesson 工作 folder:Writing/")
	f.Add("Type:lesson slug:a:b")
	f.Add("domain:\u304b\u3099 topic:\u65e5\u672c\u8a9e")
	f.Add("status: foo:bar")
	f.Add(`"semantic retrieval" type:lesson`)
	f.Add("「深度 工作」 『読本』")
	f.Add(`don"t ""  読本」 "unclosed`)
	f.Add("goroutine -channel OR -channel")
	f.Add(`"OR" 「-flag」 topic:-flag`)

	f.Fuzz(func(t *testing.T, raw string) {
		first := Parse(raw)
		second := Parse(raw)
		if diff := cmp.Diff(first, second, cmp.AllowUnexported(Query{})); diff != "" {
			t.Errorf("Parse(%q) is not deterministic (-first +second):\n%s", raw, diff)
		}
		// Quote characters restructure fields — a matched pair glues
		// whitespace into one token, an unmatched one is dropped — so the
		// one-to-one count against strings.Fields holds only for quote-free
		// input.
		if !strings.ContainsAny(raw, "\"「」『』") {
			if got, want := len(first.tokens)+len(first.filters), len(strings.Fields(raw)); got != want {
				t.Errorf("Parse(%q) classified %d tokens, want %d", raw, got, want)
			}
		}

		metadata := false
		for _, filter := range first.filters {
			kind, ok := classifyFilterKey(filter.Key)
			if !ok {
				t.Errorf("Parse(%q) emitted unknown filter key %q", raw, filter.Key)
			}
			metadata = metadata || kind == filterMetadata
		}
		if first.requiresMetadata() != metadata {
			t.Errorf("Parse(%q).requiresMetadata() = %v, want %v", raw, first.requiresMetadata(), metadata)
		}
		if utf8.ValidString(raw) && (!allValidStrings(first.tokens) || !validFilters(first.filters)) {
			t.Errorf("Parse(valid UTF-8 %q) emitted invalid UTF-8: %#v", raw, first)
		}
	})
}

func allValidStrings(values []string) bool {
	for _, value := range values {
		if !utf8.ValidString(value) {
			return false
		}
	}
	return true
}

func validFilters(filters []Filter) bool {
	for _, filter := range filters {
		if !utf8.ValidString(filter.Key) || !utf8.ValidString(filter.Value) {
			return false
		}
	}
	return true
}
