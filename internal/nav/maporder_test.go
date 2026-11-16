package nav

import (
	"slices"
	"testing"
)

// Maps sharing a domain and a title are told apart only by where they live,
// and that tie must read in the vault's one path order, the order a folder
// lists the same files in, not by bytes that put Untitled 2 before Untitled
// and TODO before index.
func TestMapsSharingDomainAndTitleFollowReadingOrder(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeNavFixture(t, root, "Concepts/Target.md", "---\ntype: concept\n---\n# Target\n\nbody\n")
	for _, rel := range []string{"Maps/Untitled 2.md", "Maps/TODO.md", "Maps/Untitled.md", "Maps/index.md"} {
		writeNavFixture(t, root, rel, "---\ntype: moc\n---\n# Shelf\n\n## Shelf\n- [[Target]]\n")
	}
	domains := map[string]string{}
	for _, rel := range []string{"Maps/Untitled 2.md", "Maps/TODO.md", "Maps/Untitled.md", "Maps/index.md"} {
		domains[rel] = "golang"
	}
	model := capturedModelWithDomains(t, root, domains)
	var got []string
	for _, m := range model.Maps() {
		got = append(got, m.RelPath)
	}
	want := []string{"Maps/index.md", "Maps/TODO.md", "Maps/Untitled.md", "Maps/Untitled 2.md"}
	if !slices.Equal(got, want) {
		t.Errorf("caught: maps sharing domain and title = %q, want %q", got, want)
	}
}
