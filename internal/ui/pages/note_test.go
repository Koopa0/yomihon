package pages

import (
	"bytes"
	"strings"
	"testing"

	"github.com/koopa0/kurodo/internal/render"
	"github.com/koopa0/kurodo/internal/ui/layouts"
)

// TestRailNeverCollapsesOverAPresentBlock is the write-face safety lock. The
// reading page's right rail may surrender its grid column only when its three
// blocks — the table of contents, the status panel, and the diagnostics list —
// are all absent. The status panel is the one surface the vault is written
// through, so no layout state may hide it.
//
// Each case constructs the view directly and asserts both the collapse
// predicate and the rendered page: the collapse modifier class must be absent
// and the block that keeps the rail alive must be present. A defect that let a
// no-heading note collapse its rail (dropping the status panel) turns the first
// case red.
func TestRailNeverCollapsesOverAPresentBlock(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		view     NoteView
		survives string // text from the block that must keep the rail
	}{
		{
			name:     "no-heading clean note keeps the status panel",
			view:     NoteView{Title: "T", RelPath: "a.md", Status: "draft", Transitions: []string{"ready"}},
			survives: "k-statuspanel",
		},
		{
			name:     "headings keep the table of contents",
			view:     NoteView{Title: "T", RelPath: "a.md", Diagnostic: "bad yaml", TOC: []render.TOCEntry{{Level: 2, Text: "H", ID: "h"}}},
			survives: "On this page",
		},
		{
			name:     "broken frontmatter keeps the diagnostics list",
			view:     NoteView{Title: "T", RelPath: "a.md", Diagnostic: "unterminated string"},
			survives: "Diagnostics",
		},
		{
			name:     "render diagnostics keep the diagnostics list",
			view:     NoteView{Title: "T", RelPath: "a.md", RenderDiagnostics: []render.Diagnostic{{Kind: render.DiagWikilinkBroken, Target: "X", Message: "broken"}}},
			survives: "Diagnostics",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.view.railEmpty() {
				t.Errorf("railEmpty() = true, want false: a present block must keep the rail")
			}

			var buf bytes.Buffer
			if err := Note(tt.view, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render: %v", err)
			}
			html := buf.String()
			if strings.Contains(html, "k-shell--rail-empty") {
				t.Errorf("the collapse modifier is present, but %q should keep the rail open", tt.survives)
			}
			if !strings.Contains(html, tt.survives) {
				t.Errorf("rendered page is missing the rail block that must survive: %q", tt.survives)
			}
		})
	}
}

// TestRailStatusAndDiagnosticsAreComplementary records why the collapse is
// defensive and currently unreachable: the status panel shows exactly when the
// frontmatter diagnostic is empty, and the diagnostics list shows exactly when
// it is not (or a render diagnostic exists). For every note one of the two is
// therefore present, so the rail is never actually empty. If a future change let
// both fall away at once — stranding a blank column that might still carry the
// write face — this test fails first.
func TestRailStatusAndDiagnosticsAreComplementary(t *testing.T) {
	t.Parallel()

	diags := []string{"", "bad yaml"}
	renders := [][]render.Diagnostic{nil, {{Kind: render.DiagWikilinkBroken, Target: "X", Message: "m"}}}
	for _, d := range diags {
		for _, rd := range renders {
			v := NoteView{Diagnostic: d, RenderDiagnostics: rd}
			if v.railEmpty() {
				t.Errorf("railEmpty() = true for Diagnostic=%q, renderDiagnostics=%d; one rail block is always present", d, len(rd))
			}
		}
	}
}
