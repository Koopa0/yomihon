package nav

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestCapturedNavigationPathSuffix(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for rel, body := range map[string]string{
		"Concepts/Atlas/Page.md":    "---\ntitle: Atlas page\ntype: concept\n---\nbody\n",
		"Concepts/Atlas/Second.md":  "---\ntitle: Second page\ntype: concept\n---\nbody\n",
		"Concepts/A/Shared/Twin.md": "body\n",
		"Concepts/B/Shared/Twin.md": "body\n",
		"Maps/Map.md":               "---\ntitle: Map\ntype: moc\n---\n## Shelf\n- [[Atlas/Page]]\n- [[Shared/Twin]]\n- [[Nope/Page]]\n",
		"Maps/Course.md":            "---\ntitle: Course\ntype: study-path\n---\n## Main {sequence=primary}\n- [[Atlas/Page]]\n- [[Shared/Twin]]\n- [[Nope/Page]]\n- [[Atlas/Second]]\n",
	} {
		writeNavFixture(t, root, rel, body)
	}
	contract := testContract(t)
	m := capturedModel(t, root, contract.NavigationRoles(), contract.KnowledgeScope(), contract.ArtifactPolicy(), nil)
	t.Log("invoked: actual suffix resolution contract")
	maps := m.Maps()
	if len(maps) != 1 || len(maps[0].Branches) != 1 || len(maps[0].Branches[0].Entries) != 1 {
		t.Fatalf("caught: suffix map selection changed: %+v", maps)
	}
	entry := maps[0].Branches[0].Entries[0]
	if entry.RelPath != "Concepts/Atlas/Page.md" || entry.Name != "Atlas page" {
		t.Errorf("caught: suffix map selection changed: %+v", entry)
	}
	paths := m.Paths()
	if len(paths) != 1 || len(paths[0].Groups) != 1 || len(paths[0].Groups[0].Items) != 4 {
		t.Fatalf("caught: suffix course selection changed: %+v", paths)
	}
	items := paths[0].Groups[0].Items
	rows := []*PathEntry{items[0].Entry, items[1].Entry, items[2].Entry, items[3].Entry}
	if rows[0].Kind != EntryResolved || rows[0].RelPath != "Concepts/Atlas/Page.md" || rows[1].Kind != EntryAmbiguous || rows[2].Kind != EntryUnresolved || rows[3].RelPath != "Concepts/Atlas/Second.md" {
		t.Errorf("caught: suffix course selection changed: %+v", rows)
	}
	if diff := cmp.Diff([]string{"Concepts/A/Shared/Twin.md", "Concepts/B/Shared/Twin.md"}, rows[1].Candidates); diff != "" {
		t.Errorf("caught: suffix course candidates changed: %s", diff)
	}
	want := []Neighbors{{PathTitle: "Course", PathRelPath: "Maps/Course.md", Unit: UnitItem, Next: NoteRef{Name: "Second page", RelPath: "Concepts/Atlas/Second.md"}}}
	if diff := cmp.Diff(want, m.PathNeighbors("Concepts/Atlas/Page.md")); diff != "" {
		t.Errorf("caught: suffix course walk changed: %s", diff)
	}
}
