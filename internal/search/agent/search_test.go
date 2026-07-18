package agent

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/google/go-cmp/cmp"
)

func TestParseSearchArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want searchArgs
	}{
		{
			name: "flag between query words",
			args: []string{"Japanese", "--semantic", "grammar"},
			want: searchArgs{limit: 20, semantic: true, query: "Japanese grammar"},
		},
		{
			name: "all flags and end marker",
			args: []string{"--json", "one", "--limit=7", "--root", "/vault", "--semantic", "--", "-two"},
			want: searchArgs{root: "/vault", limit: 7, semantic: true, json: true, query: "one -two"},
		},
		{
			name: "idempotent booleans and explicit empty query",
			args: []string{"--json", "--semantic", "--json", "--semantic", ""},
			want: searchArgs{limit: 20, semantic: true, json: true, query: ""},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSearchArgs(tt.args)
			if err != nil {
				t.Fatalf("parseSearchArgs() error: %v", err)
			}
			if diff := cmp.Diff(tt.want, got, cmp.AllowUnexported(searchArgs{})); diff != "" {
				t.Errorf("parseSearchArgs() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestParseSearchArgsErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing query", args: []string{"--semantic"}, want: "query is required"},
		{name: "unknown flag", args: []string{"--unknown=YOMIHON_EMBED_KEY", "query"}, want: "unknown flag"},
		{name: "duplicate root", args: []string{"--root", "/a", "query", "--root=/b"}, want: "flag --root specified more than once"},
		{name: "limit below range", args: []string{"--limit", "0", "query"}, want: "--limit must be an integer from 1 to 1000"},
		{name: "query control", args: []string{"line\nbreak"}, want: "query contains control characters"},
		{name: "query too long", args: []string{strings.Repeat("x", 4097)}, want: "query exceeds 4096 bytes"},
		{name: "root control", args: []string{"--root", "bad\x7fpath", "query"}, want: "root contains control characters"},
		{name: "root too long", args: []string{"--root", strings.Repeat("x", 4097), "query"}, want: "root exceeds 4096 bytes"},
		{name: "duplicate limit", args: []string{"--limit=1", "--limit", "2", "query"}, want: "flag --limit specified more than once"},
		{name: "limit not integer", args: []string{"--limit=YOMIHON_EMBED_KEY", "query"}, want: "--limit must be an integer from 1 to 1000"},
		{name: "limit above range", args: []string{"--limit=1001", "query"}, want: "--limit must be an integer from 1 to 1000"},
		{name: "missing limit", args: []string{"query", "--limit"}, want: "flag --limit needs a value"},
		{name: "empty root", args: []string{"--root=", "query"}, want: "flag --root needs a non-empty value"},
		{name: "semantic value killed", args: []string{"--semantic=best-effort", "query"}, want: "flag --semantic takes no value"},
		{name: "json value rejected", args: []string{"--json=true", "query"}, want: "flag --json takes no value"},
		{name: "query C1 control", args: []string{"bad\u0085query"}, want: "query contains control characters"},
		{name: "invalid UTF-8 query", args: []string{string([]byte{0xff})}, want: "arguments must be valid UTF-8"},
		{name: "invalid UTF-8 root", args: []string{"--root", string([]byte{0xff}), "query"}, want: "arguments must be valid UTF-8"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseSearchArgs(tt.args)
			if err == nil || err.Error() != tt.want {
				t.Errorf("parseSearchArgs() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func FuzzParseSearchArgs(f *testing.F) {
	f.Add("--semantic", "query")
	f.Add("--", "--limit")
	f.Add("--root", "/vault")
	f.Fuzz(func(t *testing.T, a, b string) {
		args := []string{a, b}
		first, firstErr := parseSearchArgs(args)
		second, secondErr := parseSearchArgs(args)
		if diff := cmp.Diff(first, second, cmp.AllowUnexported(searchArgs{})); diff != "" {
			t.Errorf("parseSearchArgs(%q) is not deterministic (-first +second):\n%s", args, diff)
		}
		if errorText(firstErr) != errorText(secondErr) {
			t.Errorf("parseSearchArgs(%q) errors = %q then %q", args, errorText(firstErr), errorText(secondErr))
		}
		if firstErr != nil {
			return
		}
		if first.limit < 1 || first.limit > 1000 {
			t.Errorf("parseSearchArgs(%q) limit = %d, want within [1,1000]", args, first.limit)
		}
		if len(first.query) > maxCLIInputBytes || containsCLIControl(first.query) || !utf8.ValidString(first.query) {
			t.Errorf("parseSearchArgs(%q) accepted invalid query %q", args, first.query)
		}
		if first.root != "" {
			if err := validateCLIRoot(first.root); err != nil {
				t.Errorf("parseSearchArgs(%q) accepted invalid root %q: %v", args, first.root, err)
			}
		}
	})
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
