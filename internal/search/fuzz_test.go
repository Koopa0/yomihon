package search

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/google/go-cmp/cmp"
)

// FuzzParse keeps the lexical grammar deterministic and ownership-safe across
// arbitrary query text. Every whitespace token must be classified exactly once
// as either bare text or one of the six structured filters.
func FuzzParse(f *testing.F) {
	f.Add("")
	f.Add("深度 type:lesson 工作 folder:Writing/")
	f.Add("Type:lesson slug:a:b")
	f.Add("domain:が topic:日本語")
	f.Add("status: foo:bar")

	f.Fuzz(func(t *testing.T, raw string) {
		first := Parse(raw)
		second := Parse(raw)
		if diff := cmp.Diff(first, second, cmp.AllowUnexported(Query{})); diff != "" {
			t.Errorf("Parse(%q) is not deterministic (-first +second):\n%s", raw, diff)
		}
		if got, want := len(first.tokens)+len(first.filters), len(strings.Fields(raw)); got != want {
			t.Errorf("Parse(%q) classified %d tokens, want %d", raw, got, want)
		}

		metadata := false
		for _, filter := range first.filters {
			kind, ok := classifyFilterKey(filter.Key)
			if !ok {
				t.Errorf("Parse(%q) emitted unknown filter key %q", raw, filter.Key)
			}
			metadata = metadata || kind == filterMetadata
		}
		if first.RequiresMetadata() != metadata {
			t.Errorf("Parse(%q).RequiresMetadata() = %v, want %v", raw, first.RequiresMetadata(), metadata)
		}
		if utf8.ValidString(raw) && (!utf8.ValidString(first.bareText) || !allValidStrings(first.tokens) || !validFilters(first.filters)) {
			t.Errorf("Parse(valid UTF-8 %q) emitted invalid UTF-8: %#v", raw, first)
		}

		// The public accessors must return detached slices: an agent that sorts
		// or edits its copy cannot change later matching decisions.
		tokens := first.Tokens()
		if len(tokens) > 0 {
			tokens[0] = "mutated"
			if first.Tokens()[0] == "mutated" {
				t.Errorf("Parse(%q).Tokens() exposes query-owned storage", raw)
			}
		}
		filters := first.Filters()
		if len(filters) > 0 {
			filters[0].Key = "mutated"
			if first.Filters()[0].Key == "mutated" {
				t.Errorf("Parse(%q).Filters() exposes query-owned storage", raw)
			}
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

// FuzzSnippet drives the real folded-text relationship used by the index. A
// snippet must stay one line, preserve valid UTF-8 boundaries, and remain
// within its fixed byte window plus at most two ellipses.
func FuzzSnippet(f *testing.F) {
	f.Add("prefix 日本語 suffix", "日本語")
	f.Add(strings.Repeat("a", 40)+"界"+strings.Repeat("b", 160), "界")
	f.Add("界"+strings.Repeat("a", 39)+"needle", "needle")
	f.Add("line one\n\tline two\r\nline three", "two")
	f.Add("", "")

	f.Fuzz(func(t *testing.T, plain, token string) {
		if len(plain) > 256<<10 || len(token) > 16<<10 {
			t.Skip()
		}
		plainFold := fold(plain)
		tokens := []string{fold(token)}
		got := snippet(plain, plainFold, tokens)
		if got != snippet(plain, plainFold, tokens) {
			t.Errorf("snippet(%q, %q) is not deterministic", plain, token)
		}
		if utf8.ValidString(plain) && !utf8.ValidString(got) {
			t.Errorf("snippet(valid UTF-8 %q, %q) returned invalid UTF-8 %x", plain, token, got)
		}
		if strings.ContainsAny(got, "\r\n\t") {
			t.Errorf("snippet(%q, %q) = %q, want one line", plain, token, got)
		}
		if got != strings.TrimSpace(got) {
			t.Errorf("snippet(%q, %q) = %q, want no surrounding whitespace", plain, token, got)
		}
		const maxSnippetBytes = snippetBefore + snippetAfter + 2*len("…")
		if len(got) > maxSnippetBytes {
			t.Errorf("snippet(%q, %q) length = %d, want at most %d", plain, token, len(got), maxSnippetBytes)
		}
	})
}
