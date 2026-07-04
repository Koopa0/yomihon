package render_test

import (
	"strings"
	"testing"

	"github.com/koopa0/kurodo/internal/render"
)

// TestPlainText is the table-driven acceptance for the plain-text
// extraction rules: what a note body contributes to the search index. Each row
// asserts substrings that must be present and, where a rule is exclusionary,
// substrings that must be absent.
func TestPlainText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		body    string
		present []string
		absent  []string
	}{
		{
			name:    "heading and body text included",
			body:    "# Title Here\n\nSome body text.\n",
			present: []string{"Title Here", "Some body text"},
		},
		{
			name:    "wikilink target and display both included",
			body:    "See [[Target Note|the alias]] for more.\n",
			present: []string{"Target Note", "the alias"},
		},
		{
			name:    "code fence contents included",
			body:    "```go\nfunc Foo() int { return 42 }\n```\n",
			present: []string{"func Foo", "return 42"},
		},
		{
			name:    "ruby base and rt included, tags excluded",
			body:    "<ruby>今日<rt>きょう</rt></ruby>は晴れ\n",
			present: []string{"今日", "きょう", "は晴れ"},
			absent:  []string{"<ruby>", "<rt>", "</rt>", "</ruby>"},
		},
		{
			name:    "html tags themselves excluded",
			body:    "first line<br>second line\n",
			present: []string{"first line", "second line"},
			absent:  []string{"<br>"},
		},
		{
			name:    "callout marker excluded, title and body kept",
			body:    "> [!note] Important Heading\n> the detail text\n",
			present: []string{"Important Heading", "the detail text"},
			absent:  []string{"[!note]", "!note"},
		},
		{
			name:    "table cells included",
			body:    "| Command | Description |\n|---|---|\n| status | flip it |\n",
			present: []string{"Command", "Description", "status", "flip it"},
		},
		{
			name:    "task text included",
			body:    "- [ ] buy milk\n- [x] write tests\n",
			present: []string{"buy milk", "write tests"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := render.PlainText(tt.body)
			for _, p := range tt.present {
				if !strings.Contains(got, p) {
					t.Errorf("PlainText(%q) = %q, missing %q", tt.body, got, p)
				}
			}
			for _, a := range tt.absent {
				if strings.Contains(got, a) {
					t.Errorf("PlainText(%q) = %q, must not contain %q", tt.body, got, a)
				}
			}
		})
	}
}
