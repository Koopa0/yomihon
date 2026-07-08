package pages

import (
	"bytes"
	"strings"
	"testing"

	"github.com/koopa0/kurodo/internal/render"
	"github.com/koopa0/kurodo/internal/ui/layouts"
)

// TestWriteFaceReachableInEveryLayoutState is the write-face safety lock over
// the layout matrix. The right rail exists to hold reading aids; a note with
// none drops the rail's column, and narrow viewports hide the rail outright —
// so the fixed-bottom seal bar must carry the status face, including the
// fail-closed notice, whenever the rail is absent. The status panel is the
// one surface the vault is written through: no combination of aids and
// contract state may leave its state with nowhere to appear.
//
// Each case constructs the view directly and asserts the collapse predicate
// plus the rendered page. The closed-contract, no-aid note is the pinned
// worst case: before the seal bar carried the notices, that combination
// rendered the write face's state nowhere at all.
func TestWriteFaceReachableInEveryLayoutState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		view        NoteView
		wantAids    bool
		wantPresent []string
		wantAbsent  []string
	}{
		{
			name:     "no aids, open contract: the seal bar carries the seal form",
			view:     NoteView{Title: "T", RelPath: "a.md", Status: "draft", Transitions: []string{"ready"}},
			wantAids: false,
			wantPresent: []string{
				"k-shell--rail-empty",
				"k-sealbar",
				"data-seal",
			},
		},
		{
			name:     "no aids, closed contract: the seal bar carries the fail-closed notice",
			view:     NoteView{Title: "T", RelPath: "a.md", Status: "draft", WriteClosed: true},
			wantAids: false,
			wantPresent: []string{
				"k-shell--rail-empty",
				"k-sealbar",
				"the write face is closed (fail-closed)",
			},
		},
		{
			name:     "no aids, no frontmatter: the seal bar says so",
			view:     NoteView{Title: "T", RelPath: "a.md", NoFrontmatter: true},
			wantAids: false,
			wantPresent: []string{
				"k-shell--rail-empty",
				"k-sealbar",
				"No frontmatter (valid).",
			},
		},
		{
			name:     "headings keep the rail and add the inline disclosure",
			view:     NoteView{Title: "T", RelPath: "a.md", Status: "draft", Transitions: []string{"ready"}, TOC: []render.TOCEntry{{Level: 2, Text: "H", ID: "h"}}},
			wantAids: true,
			wantPresent: []string{
				"k-statuspanel",
				"k-inlineaids",
				"k-toc-inline",
				"On this page",
			},
			wantAbsent: []string{"k-shell--rail-empty"},
		},
		{
			name:     "broken frontmatter: the diagnostics face owns the page, no seal bar",
			view:     NoteView{Title: "T", RelPath: "a.md", Diagnostic: "unterminated string"},
			wantAids: true,
			wantPresent: []string{
				"Diagnostics",
				"k-inlineaids",
			},
			wantAbsent: []string{"k-shell--rail-empty", "k-sealbar", "k-statuspanel"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.view.hasAids(); got != tt.wantAids {
				t.Errorf("hasAids() = %v, want %v", got, tt.wantAids)
			}

			var buf bytes.Buffer
			if err := Note(tt.view, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render: %v", err)
			}
			html := buf.String()
			for _, want := range tt.wantPresent {
				if !strings.Contains(html, want) {
					t.Errorf("rendered page is missing %q", want)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(html, absent) {
					t.Errorf("rendered page unexpectedly contains %q", absent)
				}
			}
		})
	}
}

// TestSealToastRidesTheRedirectSignal locks the toast's one-shot contract:
// it renders exactly when the page carries the seal redirect's signal, so
// the confirmation is server-rendered CSS that plays with or without the
// enhancement script.
func TestSealToastRidesTheRedirectSignal(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name       string
		justSealed bool
	}{
		{name: "just sealed renders the toast", justSealed: true},
		{name: "an ordinary view does not", justSealed: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			v := NoteView{Title: "T", RelPath: "a.md", Status: "ready", JustSealed: tt.justSealed}
			var buf bytes.Buffer
			if err := Note(v, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render: %v", err)
			}
			if got := strings.Contains(buf.String(), "k-toast"); got != tt.justSealed {
				t.Errorf("toast rendered = %v, want %v", got, tt.justSealed)
			}
		})
	}
}

// TestSealBarMirrorsTheStatusPanelGuard records the seal bar's render
// condition as the invariant it is: the bar renders exactly when the status
// panel does — whenever the frontmatter parsed — because at narrow widths and
// on no-aid notes it is the only status face on the page. A frontmatter
// diagnostic suppresses both, and the diagnostics list explains why.
func TestSealBarMirrorsTheStatusPanelGuard(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		view        NoteView
		wantSealBar bool
	}{
		{name: "open contract", view: NoteView{Status: "draft", Transitions: []string{"ready"}}, wantSealBar: true},
		{name: "closed contract", view: NoteView{Status: "draft", WriteClosed: true}, wantSealBar: true},
		{name: "no frontmatter", view: NoteView{NoFrontmatter: true}, wantSealBar: true},
		{name: "sealed note", view: NoteView{Status: "ready"}, wantSealBar: true},
		{name: "frontmatter diagnostic", view: NoteView{Diagnostic: "bad yaml"}, wantSealBar: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			if err := Note(tt.view, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render: %v", err)
			}
			if got := strings.Contains(buf.String(), "k-sealbar"); got != tt.wantSealBar {
				t.Errorf("seal bar rendered = %v, want %v", got, tt.wantSealBar)
			}
		})
	}
}
