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
// within its fixed window of characters plus at most two ellipses.
func FuzzSnippet(f *testing.F) {
	f.Add("prefix 日本語 suffix", "日本語")
	f.Add(strings.Repeat("a", 40)+"界"+strings.Repeat("b", 160), "界")
	f.Add("界"+strings.Repeat("a", 39)+"needle", "needle")
	f.Add("line one\n\tline two\r\nline three", "two")
	f.Add("", "")
	// A full window of Japanese: 200 characters of it is 600 bytes, so this is
	// the shape a byte-counted bound reports as too long while the window is
	// exactly the size it is meant to be.
	f.Add(strings.Repeat("あ", 200)+"鍵"+strings.Repeat("い", 200), "鍵")
	// A match walled in by letters on both sides, far enough from either end of
	// the run that both boundaries have to give up on keeping the word whole.
	f.Add(strings.Repeat("a", 300)+"z"+strings.Repeat("a", 300), "z")
	// The same length in ordinary spaced words, where nothing shortens the
	// window and it fills to its limit.
	f.Add(strings.Repeat("ab ", 300)+"needle"+strings.Repeat(" cd", 300), "needle")

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
		// The window is counted in characters, not bytes. Its bounds walk
		// characters outward from the match, so a full window of Japanese is
		// three times as many bytes as a full window of English while being the
		// same window — measuring it in bytes reports the wider script as a
		// defect. Each bound may then move a further 24 outward to keep a word
		// whole, and that movement only ever crosses the ASCII letters, digits,
		// hyphens and underscores that count as word bytes, so its allowance
		// carries over to characters one for one. Two ellipses are all that is
		// added afterwards.
		//
		// The numbers are written out rather than read back from the package on
		// purpose: a limit derived from the same constants that size the window
		// would widen along with them, and could never report a window that
		// grew.
		const maxSnippetRunes = 40 + 160 + 2*24 + 2
		if n := utf8.RuneCountInString(got); n > maxSnippetRunes {
			t.Errorf("snippet(%q, %q) length = %d characters, want at most %d", plain, token, n, maxSnippetRunes)
		}
	})
}
