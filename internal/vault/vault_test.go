package vault_test

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

func TestParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		content    string
		wantTitle  string
		wantStatus string
		wantSlug   string
		wantDiag   bool
		wantInBody string
	}{
		{
			name:       "lesson slug is read as the join key",
			content:    "---\ntitle: L01\ntype: lesson\nstatus: draft\nslug: jp-minna-l01\n---\n\nbody\n",
			wantTitle:  "L01",
			wantStatus: "draft",
			wantSlug:   "jp-minna-l01",
			wantInBody: "body",
		},
		{
			name:       "frontmatter and body",
			content:    "---\ntitle: 數量詞の位置\ntype: concept\nstatus: seedling\n---\n\n# 本文\n",
			wantTitle:  "數量詞の位置",
			wantStatus: "seedling",
			wantInBody: "# 本文",
		},
		{
			name:       "no frontmatter is legal",
			content:    "# 假名 quiz\n",
			wantTitle:  "note",
			wantInBody: "# 假名 quiz",
		},
		{
			name:       "broken yaml yields diagnostic not error",
			content:    "---\ntitle: [broken\n---\nbody\n",
			wantTitle:  "note",
			wantDiag:   true,
			wantInBody: "body",
		},
		{
			name:       "wikilink value survives the split",
			content:    "---\ntitle: t\nbased_on: \"[[大家的日本語 第11課]]\"\n---\nbody\n",
			wantTitle:  "t",
			wantInBody: "body",
		},
		{
			// The concept-resolver corruption guard, at the layer where the
			// guarantee lives: a lesson's frontmatter based_on holds
			// [[wikilinks]]; because the split happens here, before any body
			// preprocessing, a later concept resolver rewriting [[...]] to a
			// trigger can never reach the YAML and empty the meta (which would
			// drop status:ready and hide the lesson). The status must survive
			// intact alongside the wikilink-valued field.
			name:       "concept resolver cannot corrupt frontmatter status",
			content:    "---\ntitle: L00 テスト課\ntype: lesson\nstatus: ready\nbased_on: \"[[大家的日本語 第1課]]\"\nslug: jp-minna-l00\n---\n\nSee [[は]] and [[です]].\n",
			wantTitle:  "L00 テスト課",
			wantStatus: schema.SealStatus,
			wantSlug:   "jp-minna-l00",
			wantInBody: "[[は]]",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			n := vault.Parse("Concepts/note.md", []byte(tt.content))
			if got := n.Title(); got != tt.wantTitle {
				t.Errorf("Title() = %q, want %q", got, tt.wantTitle)
			}
			if got := n.Status(); got != tt.wantStatus {
				t.Errorf("Status() = %q, want %q", got, tt.wantStatus)
			}
			if got := n.Slug(); got != tt.wantSlug {
				t.Errorf("Slug() = %q, want %q", got, tt.wantSlug)
			}
			if (n.FMDiagnostic != "") != tt.wantDiag {
				t.Errorf("FMDiagnostic = %q, want diagnostic: %v", n.FMDiagnostic, tt.wantDiag)
			}
			if !strings.Contains(n.Body, tt.wantInBody) {
				t.Errorf("Body does not contain %q:\n%s", tt.wantInBody, n.Body)
			}
		})
	}
}

func TestParseFrontmatterBoundary(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		content    string
		wantTitle  string
		wantStatus string
		wantBody   string
	}{
		{name: "LF dash closer", content: "---\ntitle: LF\nstatus: draft\n---\nbody\n", wantTitle: "LF", wantStatus: "draft", wantBody: "body\n"},
		{name: "CRLF dash closer", content: "---\r\ntitle: CRLF\r\nstatus: ready\r\n---\r\nbody\r\n", wantTitle: "CRLF", wantStatus: "ready", wantBody: "body\r\n"},
		{name: "YAML document closer", content: "---\ntitle: Dots\nstatus: draft\n...\nbody\n", wantTitle: "Dots", wantStatus: "draft", wantBody: "body\n"},
		{name: "closer at EOF", content: "---\ntitle: EOF\nstatus: ready\n---", wantTitle: "EOF", wantStatus: "ready", wantBody: ""},
		{name: "unterminated is body", content: "---\ntitle: Open\nstatus: draft\nbody\n", wantTitle: "note", wantStatus: "", wantBody: "---\ntitle: Open\nstatus: draft\nbody\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			n := vault.Parse("note.md", []byte(tt.content))
			if got := n.Title(); got != tt.wantTitle {
				t.Errorf("Title() = %q, want %q", got, tt.wantTitle)
			}
			if got := n.Status(); got != tt.wantStatus {
				t.Errorf("Status() = %q, want %q", got, tt.wantStatus)
			}
			if n.Body != tt.wantBody {
				t.Errorf("Body = %q, want %q", n.Body, tt.wantBody)
			}
		})
	}
}
