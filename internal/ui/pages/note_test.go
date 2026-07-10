package pages

import (
	"bytes"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/ui/layouts"
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
			view:     NoteView{Title: "T", RelPath: "a.md", Status: "draft", SealTarget: schema.SealStatus, Transitions: []string{schema.SealStatus}},
			wantAids: false,
			wantPresent: []string{
				"y-shell--rail-empty",
				"y-sealbar",
				"data-seal",
			},
		},
		{
			name:     "no aids, closed contract: the seal bar carries the fail-closed notice",
			view:     NoteView{Title: "T", RelPath: "a.md", Status: "draft", WriteClosed: true},
			wantAids: false,
			wantPresent: []string{
				"y-shell--rail-empty",
				"y-sealbar",
				"the write face is closed (fail-closed)",
			},
		},
		{
			name:     "no aids, no frontmatter: the seal bar says so",
			view:     NoteView{Title: "T", RelPath: "a.md", NoFrontmatter: true},
			wantAids: false,
			wantPresent: []string{
				"y-shell--rail-empty",
				"y-sealbar",
				"No frontmatter (valid).",
			},
		},
		{
			name:     "headings keep the rail and add the inline disclosure",
			view:     NoteView{Title: "T", RelPath: "a.md", Status: "draft", SealTarget: schema.SealStatus, Transitions: []string{schema.SealStatus}, TOC: []render.TOCEntry{{Level: 2, Text: "H", ID: "h"}}},
			wantAids: true,
			wantPresent: []string{
				"y-statuspanel",
				"y-inlineaids",
				"y-toc-inline",
				"On this page",
			},
			wantAbsent: []string{"y-shell--rail-empty"},
		},
		{
			name:     "broken frontmatter: the diagnostics face owns the page, no seal bar",
			view:     NoteView{Title: "T", RelPath: "a.md", Diagnostic: "unterminated string"},
			wantAids: true,
			wantPresent: []string{
				"Diagnostics",
				"y-inlineaids",
			},
			wantAbsent: []string{"y-shell--rail-empty", "y-sealbar", "y-statuspanel"},
		},
		{
			name:     "render diagnostics alone keep the rail",
			view:     NoteView{Title: "T", RelPath: "a.md", Status: "draft", SealTarget: schema.SealStatus, Transitions: []string{schema.SealStatus}, RenderDiagnostics: []render.Diagnostic{{Kind: render.DiagWikilinkBroken, Target: "X", Message: "broken"}}},
			wantAids: true,
			wantPresent: []string{
				"Diagnostics",
				"y-inlineaids",
				"y-statuspanel",
			},
			wantAbsent: []string{"y-shell--rail-empty"},
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

func TestSealFormUsesViewTarget(t *testing.T) {
	t.Parallel()

	v := NoteView{
		Title:       "T",
		RelPath:     "a.md",
		Status:      "draft",
		SealTarget:  "candidate",
		Transitions: []string{"candidate"},
	}
	var buf bytes.Buffer
	if err := Note(v, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, "data-seal") {
		t.Fatalf("rendered page has no seal form: %q", html)
	}
	if want := `name="to" value="candidate"`; !strings.Contains(html, want) {
		t.Errorf("seal form is missing %q", want)
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

			v := NoteView{Title: "T", RelPath: "a.md", Status: schema.SealStatus, SealTarget: schema.SealStatus, Sealed: true, JustSealed: tt.justSealed}
			var buf bytes.Buffer
			if err := Note(v, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render: %v", err)
			}
			if got := strings.Contains(buf.String(), "y-toast"); got != tt.justSealed {
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
		{name: "open contract", view: NoteView{Status: "draft", SealTarget: schema.SealStatus, Transitions: []string{schema.SealStatus}}, wantSealBar: true},
		{name: "closed contract", view: NoteView{Status: "draft", WriteClosed: true}, wantSealBar: true},
		{name: "no frontmatter", view: NoteView{NoFrontmatter: true}, wantSealBar: true},
		{name: "sealed note", view: NoteView{Status: schema.SealStatus, SealTarget: schema.SealStatus, Sealed: true}, wantSealBar: true},
		{name: "frontmatter diagnostic", view: NoteView{Diagnostic: "bad yaml"}, wantSealBar: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			if err := Note(tt.view, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render: %v", err)
			}
			if got := strings.Contains(buf.String(), "y-sealbar"); got != tt.wantSealBar {
				t.Errorf("seal bar rendered = %v, want %v", got, tt.wantSealBar)
			}
		})
	}
}
