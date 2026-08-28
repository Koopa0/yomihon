package render_test

import (
	"testing"

	"github.com/koopa0/yomihon/internal/render"
)

// TestFragmentDiagnosticsKeepTheNameBare holds the contract Target carries for
// every reader of it. Downstream readers look a planned name up by that field,
// and a name with an address welded onto it matches nothing they hold — a
// failure that reports nothing and simply finds nothing, which is why no test
// caught it before the field was split.
func TestFragmentDiagnosticsKeepTheNameBare(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name        string
		renderer    func(*testing.T) *render.Pipeline
		body        string
		kind        render.DiagnosticKind
		wantSection string
		wantBlock   string
	}{
		{
			name: "a link naming a section the destination lacks", renderer: sectionRenderer,
			body: "[[B#Gamma]]\n", kind: render.DiagLinkSectionMissing, wantSection: "Gamma",
		},
		{
			name: "a link naming a block the destination lacks", renderer: blockRenderer,
			body: "[[B#^nope]]\n", kind: render.DiagLinkFragmentMissing, wantBlock: "nope",
		},
		{
			name: "an embed naming a section the destination lacks", renderer: sectionRenderer,
			body: "![[B#Gamma]]\n", kind: render.DiagEmbedFragmentMissing, wantSection: "Gamma",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.renderer(t).HTML("note.md", "", tt.body)
			var found int
			for _, d := range got.Diagnostics {
				if d.Kind != tt.kind {
					continue
				}
				found++
				if d.Target != "B" {
					t.Errorf("Diagnostic.Target = %q, want the bare name %q", d.Target, "B")
				}
				if d.Section != tt.wantSection {
					t.Errorf("Diagnostic.Section = %q, want %q", d.Section, tt.wantSection)
				}
				if d.Block != tt.wantBlock {
					t.Errorf("Diagnostic.Block = %q, want %q", d.Block, tt.wantBlock)
				}
			}
			if found != 1 {
				t.Fatalf("%d diagnostics of kind %q, want exactly 1", found, tt.kind)
			}
		})
	}
}
