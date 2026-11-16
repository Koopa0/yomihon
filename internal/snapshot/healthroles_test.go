package snapshot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/schema"
)

func TestHealthExcludesEveryDeclaredNavigationEntryFromIslands(t *testing.T) {
	t.Parallel()
	for _, unlisted := range []bool{false, true} {
		name := "complete course"
		if unlisted {
			name = "unlisted lesson stays an island"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			contract := testContract(t, root)
			roles := contract.Capabilities(contract.Governance()).Navigation
			if diff := cmp.Diff([]string{"study-path"}, roles.PathTypes()); diff != "" {
				t.Fatalf("path fixture roles (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff([]string{"moc", "source-map", "topic-map"}, roles.MapTypes()); diff != "" {
				t.Fatalf("map fixture roles (-want +got):\n%s", diff)
			}
			for _, typ := range append(roles.PathTypes(), roles.MapTypes()...) {
				writeNote(t, root, "Writing/"+typ+".md", "---\ntype: "+typ+"\ncreated: 2026-06-01\n---\n\n# Entry "+typ+"\n\n## Unit\n- [[Listed]]\n")
			}
			writeNote(t, root, "Writing/Listed.md", "---\nslug: listed\ntype: lesson\ncreated: 2026-06-01\n---\n\nLesson text.\n")
			want := []HealthIslandGroup{}
			if unlisted {
				writeNote(t, root, "Maps/Unlisted.md", "---\nslug: unlisted\ntype: lesson\ncreated: 2026-06-01\n---\n\nLesson text.\n")
				want = []HealthIslandGroup{{Dir: "Maps", Notes: []nav.NoteRef{{Name: "Unlisted", RelPath: "Maps/Unlisted.md"}}}}
			}
			store, _ := newTestStore(t, root, contract)
			if diff := cmp.Diff(want, store.Current().Health().Islands); diff != "" {
				t.Errorf("caught: declared entry island rows (-want +got):\n%s", diff)
			}
		})
	}
}

func TestHealthEntryExemptionUsesTheContractAndKeepsCitationFaults(t *testing.T) {
	t.Parallel()
	const defaultRoles = "[types.study-path]\nrole = \"course\"\n\n[types.moc]\nrole = \"map\"\n\n[types.source-map]\nrole = \"map\"\n\n[types.topic-map]\nrole = \"map\"\n"
	const lessonRole = "[types.lesson]\nrole = \"lesson\"\n"
	for _, tt := range []struct {
		name, roles, lessonRole, mapType string
		islands                          []nav.NoteRef
	}{
		{name: "default roles", roles: defaultRoles, islands: []nav.NoteRef{{Name: "Ordinary", RelPath: "Maps/Ordinary.md"}}},
		{name: "type case is not guessed", roles: defaultRoles, mapType: "MOC", islands: []nav.NoteRef{{Name: "Map", RelPath: "Maps/Map.md"}, {Name: "Ordinary", RelPath: "Maps/Ordinary.md"}}},
		{name: "reassigned roles", roles: "[types.study-path]\n\n[types.moc]\n\n[types.source-map]\nrole = \"course\"\n\n[types.topic-map]\n", lessonRole: "[types.lesson]\nrole = \"map\"\n", islands: []nav.NoteRef{{Name: "Map", RelPath: "Maps/Map.md"}, {Name: "Path", RelPath: "Maps/Path.md"}}},
		{name: "empty roles", roles: "[types.study-path]\n\n[types.moc]\n\n[types.source-map]\n\n[types.topic-map]\n", islands: []nav.NoteRef{{Name: "Map", RelPath: "Maps/Map.md"}, {Name: "Ordinary", RelPath: "Maps/Ordinary.md"}, {Name: "Path", RelPath: "Maps/Path.md"}}},
		{name: "a second thought type rejects the roles", roles: "[types.study-path]\nrole = \"thought\"\n\n[types.moc]\nrole = \"map\"\n\n[types.source-map]\nrole = \"map\"\n\n[types.topic-map]\nrole = \"map\"\n", islands: []nav.NoteRef{{Name: "Map", RelPath: "Maps/Map.md"}, {Name: "Ordinary", RelPath: "Maps/Ordinary.md"}, {Name: "Path", RelPath: "Maps/Path.md"}}},
		{name: "rejected roles", roles: "[types.study-path]\nrole = \"not-a-role\"\n\n[types.moc]\nrole = \"map\"\n\n[types.source-map]\nrole = \"map\"\n\n[types.topic-map]\nrole = \"map\"\n", islands: []nav.NoteRef{{Name: "Map", RelPath: "Maps/Map.md"}, {Name: "Ordinary", RelPath: "Maps/Ordinary.md"}, {Name: "Path", RelPath: "Maps/Path.md"}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			mapType := tt.mapType
			if mapType == "" {
				mapType = "moc"
			}
			_ = testContract(t, root)
			file := filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))
			data, err := os.ReadFile(file) // #nosec G304 -- file is the contract written under this test's TempDir
			if err != nil {
				t.Fatal(err)
			}
			text := string(data)
			if strings.Count(text, defaultRoles) != 1 || strings.Count(text, lessonRole) != 1 {
				t.Fatal("role fixture declarations are not unique")
			}
			text = strings.Replace(text, defaultRoles, tt.roles, 1)
			if tt.lessonRole != "" {
				text = strings.Replace(text, lessonRole, tt.lessonRole, 1)
			}
			if writeErr := os.WriteFile(file, []byte(text), 0o600); writeErr != nil { // #nosec G703 -- file is the contract written under this test's TempDir
				t.Fatal(writeErr)
			}
			contract, err := schema.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			for _, note := range []struct{ path, typ, slug, body string }{
				{path: "Maps/Path.md", typ: "study-path", body: "- [[Listed]]\n"},
				{path: "Maps/Map.md", typ: mapType, body: "- [[Listed]]\n- [[Missing]]\n"},
				{path: "Maps/Ordinary.md", typ: "lesson", slug: "slug: ordinary\n", body: "Ordinary text.\n"},
				{path: "Writing/Listed.md", typ: "lesson", slug: "slug: listed\n", body: "Listed text.\n"},
			} {
				writeNote(t, root, note.path, "---\ntype: \""+note.typ+"\"\ncreated: 2026-06-01\n"+note.slug+"---\n\n"+note.body)
			}
			store, _ := newTestStore(t, root, contract)
			health := store.Current().Health()
			want := []HealthIslandGroup{{Dir: "Maps", Notes: tt.islands}}
			if diff := cmp.Diff(want, health.Islands); diff != "" {
				t.Errorf("caught: contract-selected islands (-want +got):\n%s", diff)
			}
			links := []HealthLink{{From: nav.NoteRef{Name: "Map", RelPath: "Maps/Map.md"}, Target: "Missing"}}
			if diff := cmp.Diff(links, health.Unwritten); diff != "" {
				t.Errorf("caught: entry citations lost (-want +got):\n%s", diff)
			}
		})
	}
}
