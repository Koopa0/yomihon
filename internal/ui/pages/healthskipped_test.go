package pages

import (
	"bytes"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestHealthNamesANoteOverTheSourceBound holds the size-skip row on /health.
// The generation already drops that note from the index; without this row the
// only trace is a server log, and a reader who searches for a sentence they
// wrote has no way to learn why the listed file is absent.
func TestHealthNamesANoteOverTheSourceBound(t *testing.T) {
	t.Parallel()

	const (
		path = "Notes/Big oversized note.md"
		size = int64(1_395_074)
	)
	view := HealthView{Skipped: []HealthSkippedSource{{
		Path:   path,
		Reason: "over the source bound",
		Size:   size,
	}}}

	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		t.Run(string(lang), func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			if err := Health(view, layouts.Chrome{Lang: lang}).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render: %v", err)
			}
			html := buf.String()
			if !strings.Contains(html, path) {
				t.Errorf("the skipped row does not name the path; html = %q", html)
			}
			if want := humanSize(size, lang); !strings.Contains(html, want) {
				t.Errorf("the skipped row does not name the size %q; html = %q", want, html)
			}
			if !strings.Contains(html, "over the source bound") {
				t.Error("the skipped row does not say the note is over the source bound")
			}
		})
	}
}
