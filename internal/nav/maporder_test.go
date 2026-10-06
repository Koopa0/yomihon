package nav

import (
	"slices"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

// Maps sharing a domain and a title are told apart only by where they live,
// and that tie must read in the vault's one path order, the order a folder
// lists the same files in, not by bytes that put Untitled 2 before Untitled
// and TODO before index.
func TestMapsSharingDomainAndTitleFollowReadingOrder(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeNavFixture(t, root, "Concepts/Target.md", "---\ntitle: Target\ntype: concept\nstatus: growing\n---\nbody\n")
	for _, rel := range []string{"Maps/Untitled 2.md", "Maps/TODO.md", "Maps/Untitled.md", "Maps/index.md"} {
		writeNavFixture(t, root, rel, "---\ntitle: Shelf\ntype: moc\ndomain: golang\n---\n## Shelf\n- [[Target]]\n")
	}
	roles, policy := testCapabilities(t)
	model := capturedModel(t, root, roles, schema.KnowledgeScope{}, policy, nil)
	var got []string
	for _, m := range model.Maps() {
		got = append(got, m.RelPath)
	}
	want := []string{"Maps/index.md", "Maps/TODO.md", "Maps/Untitled.md", "Maps/Untitled 2.md"}
	if !slices.Equal(got, want) {
		t.Errorf("caught: maps sharing domain and title = %q, want %q", got, want)
	}
}
