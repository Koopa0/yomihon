package note

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/nav"
)

func TestHealthNavigationFaultLanguages(t *testing.T) {
	t.Parallel()
	found := []nav.CoreFault{
		{Note: nav.NoteRef{Name: "Declared", RelPath: "Maps/Declared.md", Language: "ja"}, Reason: "navigation build failed: declared"},
		{Note: nav.NoteRef{Name: "Lookup", RelPath: "Maps/Lookup.md"}, Reason: "navigation build failed: lookup"},
	}
	want := []nav.CoreFault{
		{Note: nav.NoteRef{Name: "Declared", RelPath: "Maps/Declared.md", Language: "ja"}, Reason: "navigation build failed: declared"},
		{Note: nav.NoteRef{Name: "Lookup", RelPath: "Maps/Lookup.md", Language: "en"}, Reason: "navigation build failed: lookup"},
	}
	got := healthNavigationFaults(found, func(string) string { return "en" })
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("authored languages (-want +got):\n%s", diff)
	}
	got[0].Reason = "changed"
	if found[0].Reason != "navigation build failed: declared" {
		t.Error("Health mapping changed its captured findings")
	}
	if diff := cmp.Diff(found, healthNavigationFaults(found, nil)); diff != "" {
		t.Errorf("nil language lookup changed facts (-want +got):\n%s", diff)
	}
}
