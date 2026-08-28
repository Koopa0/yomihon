package pages

import (
	"bytes"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/ui/layouts"
)

func TestDiagnosticAddressPutsBackWhatTheAuthorWrote(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		diag render.Diagnostic
		want string
	}{
		{name: "a name alone", diag: render.Diagnostic{Target: "B"}, want: "B"},
		{name: "a section address", diag: render.Diagnostic{Target: "B", Section: "Gamma"}, want: "B#Gamma"},
		{name: "a block address", diag: render.Diagnostic{Target: "B", Block: "nope"}, want: "B#^nope"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := diagnosticAddress(tt.diag); got != tt.want {
				t.Errorf("diagnosticAddress() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestFragmentSplitSpeaksOnlyOfAMissingNote holds the sentence to the one case
// it is true of. It says the missing half is the note, which is exactly wrong
// about a link whose note was found and whose section was not — and both kinds
// now carry a section name, so a check on that name alone would let it through.
func TestFragmentSplitSpeaksOnlyOfAMissingNote(t *testing.T) {
	t.Parallel()
	const wording = "缺的是筆記目標"

	for _, tt := range []struct {
		name string
		kind render.DiagnosticKind
		want bool
	}{
		{name: "a name nothing answered to", kind: render.DiagWikilinkBroken, want: true},
		{name: "a note that was found, missing the section", kind: render.DiagLinkSectionMissing, want: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			view := NoteView{RenderDiagnostics: []render.Diagnostic{
				{Kind: tt.kind, Target: "B", Section: "Gamma", Message: "m"},
			}}
			var buf bytes.Buffer
			if err := Note(view, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render: %v", err)
			}
			page := buf.String()
			if !strings.Contains(page, "B#Gamma") {
				t.Fatalf("the panel lost the address the author wrote: %q", page)
			}
			if got := strings.Contains(page, wording); got != tt.want {
				t.Errorf("panel says %q = %t, want %t", wording, got, tt.want)
			}
		})
	}
}
