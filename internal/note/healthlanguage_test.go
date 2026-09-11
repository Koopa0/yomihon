package note

import (
	"testing"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/snapshot"
)

// TestHealthCarriesDeclaredArticleLanguage locks the carry path from a
// generation's resolved note language onto every health link name.
func TestHealthCarriesDeclaredArticleLanguage(t *testing.T) {
	t.Parallel()
	ref := nav.NoteRef{Name: "L01", RelPath: "Writing/lessons/japanese/L01.md"}
	articleLang := func(relPath string) string {
		if relPath == "Writing/lessons/japanese/L01.md" {
			return "ja"
		}
		return ""
	}
	out := healthLinks([]snapshot.HealthLink{{From: ref, Target: "Ghost"}}, articleLang)
	if len(out) != 1 {
		t.Fatalf("healthLinks() = %d links, want 1", len(out))
	}
	if out[0].From.Language != "ja" {
		t.Errorf("healthLinks() language = %q, want ja", out[0].From.Language)
	}
}
