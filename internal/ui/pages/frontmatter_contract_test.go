package pages

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/wording"
)

func TestNoFrontmatterStatusFaceFollowsContract(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		legal bool
		want  map[wording.Lang]string
	}{
		{name: "legal", legal: true, want: map[wording.Lang]string{
			wording.ZhHant: "沒有 frontmatter（合法）。",
			wording.En:     "No frontmatter (which is legal).",
		}},
		{name: "required", legal: false, want: map[wording.Lang]string{
			wording.ZhHant: "沒有 frontmatter（契約要求必須有）。",
			wording.En:     "No frontmatter (the contract requires one).",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			contract := frontmatterStatusContract(t, tt.legal)
			required := contract.RequiresFrontmatter()
			if required == tt.legal {
				t.Fatalf("RequiresFrontmatter() = %v for legal=%v", required, tt.legal)
			}
			for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
				var buf bytes.Buffer
				view := NoteView{
					Governed:            true,
					Title:               "T",
					RelPath:             "plain.md",
					NoFrontmatter:       true,
					FrontmatterRequired: required,
				}
				if err := Note(view, layouts.Chrome{Lang: lang}).Render(t.Context(), &buf); err != nil {
					t.Fatalf("render %s: %v", lang, err)
				}
				html := buf.String()
				if got := strings.Count(html, tt.want[lang]); got != 2 {
					t.Errorf("%s/%s wording count = %d, want 2; page = %q", tt.name, lang, got, html)
				}
			}
		})
	}
}

func frontmatterStatusContract(t *testing.T, legal bool) *schema.Contract {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("read contract fixture: %v", err)
	}
	const from = "no_frontmatter_is_legal = true"
	want := from
	if !legal {
		want = "no_frontmatter_is_legal = false"
	}
	rewritten := strings.Replace(string(data), from, want, 1)
	if !strings.Contains(rewritten, want) {
		t.Fatalf("frontmatter declaration %q not present", want)
	}
	path := filepath.Join(t.TempDir(), "vault-schema.toml")
	if err := os.WriteFile(path, []byte(rewritten), 0o600); err != nil {
		t.Fatalf("write contract fixture: %v", err)
	}
	contract, err := schema.LoadFile(path)
	if err != nil {
		t.Fatalf("load contract: %v", err)
	}
	return contract
}
