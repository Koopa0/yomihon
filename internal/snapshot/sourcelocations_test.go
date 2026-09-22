package snapshot

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

func TestDeclaredSourcesPreserveAuthoredPlaces(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeNote(t, root, "Source.md", "---\ntitle: Source\n---\n# Source\n\n## Methods\n\nmethod ^Quote-1\n\n## Limitations\n\nlimits\n\n```md\n## Fiction\n```\n")
	writeNote(t, root, "Other.md", "## Observation\n\nobservation\n")
	writeNote(t, root, "A/twin.md", "a\n")
	writeNote(t, root, "B/twin.md", "b\n")
	writeNote(t, root, "Claim.md", `---
based_on:
 - "[[Source#Limitations|Limit first]]"
 - "[[Other#observation]]"
 - "[[Source#methods|Method evidence]]"
 - "[[Source#Methods|Ignored repeat]]"
 - "[[Source#^Quote-1]]"
 - "[[Source^quote-1|Ignored block repeat]]"
 - "[[Source|Ignored file alias]]"
 - "[[Source#Missing|Missing evidence]]"
 - "[[Source#^absent]]"
 - "[[Source#Fiction]]"
 - "[[Source#Source]]"
 - "[[twin]]"
 - "[[nowhere]]"
 - "[[nowhere]]"
 - ""
---
No body citation.
`)
	store, _ := newTestStore(t, root, testContract(t, root))
	got, diagnostics := store.Current().DeclaredSources("Claim.md", wording.En)
	if len(diagnostics) != 3 {
		t.Fatalf("missing-location diagnostics = %+v, want three", diagnostics)
	}
	wantKinds := []render.DiagnosticKind{render.DiagLinkSectionMissing, render.DiagLinkFragmentMissing, render.DiagLinkSectionMissing}
	for i, diagnostic := range diagnostics {
		if diagnostic.Kind != wantKinds[i] {
			t.Errorf("diagnostic[%d] = %s, want %s", i, diagnostic.Kind, wantKinds[i])
		}
	}
	// Diagnostic messages are checked at the production HTTP boundary; this
	// oracle pins location identity, ordering, labels and fallback addresses.
	for i := range got {
		for j := range got[i].Locations {
			got[i].Locations[j].Diagnostic = nil
			got[i].Locations[j].Reason = ""
		}
	}
	want := []DeclaredSource{
		{Name: "Source", RelPath: "Source.md", WholeFile: true, Locations: []render.SourceLocation{
			{Label: "Limit first", Fragment: "limitations"},
			{Label: "Method evidence", Fragment: "methods"},
			{Label: "^Quote-1", Fragment: "^quote-1"},
			{Label: "Missing evidence"}, {Label: "^absent"}, {Label: "Fiction"},
			{Label: "Source", Fragment: "source"},
		}},
		{Name: "Other", RelPath: "Other.md", Locations: []render.SourceLocation{{Label: "Observation", Fragment: "observation"}}},
		{Name: "[[twin]]"}, {Name: "[[nowhere]]"},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("declared source locations (-want +got):\n%s", diff)
	}
	if refs := store.Current().CitedBy("Source.md"); len(refs) != 0 {
		t.Errorf("declarations became body citations: %+v", refs)
	}
}

func TestDeclaredLocationsUseTheirCapturedGeneration(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeNote(t, root, "Source.md", "## Methods\n\nold\n")
	writeNote(t, root, "Single.md", "---\nbased_on: \"[[Source#methods]]\"\n---\nclaim\n")
	store, _ := newTestStore(t, root, testContract(t, root))
	old := store.Current()
	writeNote(t, root, "Source.md", "## Replaced\n\nnew\n")
	got, diags := old.DeclaredSources("Single.md", wording.En)
	want := []DeclaredSource{{Name: "Source", RelPath: "Source.md", Locations: []render.SourceLocation{{Label: "Methods", Fragment: "methods"}}}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("captured locations (-want +got):\n%s", diff)
	}
	if len(diags) != 0 {
		t.Errorf("live disk leaked into captured diagnostics: %+v", diags)
	}
}
