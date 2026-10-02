package nav

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

func TestNavigationFaultContainsOnlyTheFailedTree(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"path", "map"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			for _, value := range []any{"walk failed", errors.New("walk failed"), 42, nil} {
				t.Run(fmt.Sprintf("%T", value), func(t *testing.T) {
					t.Parallel()
					walkers := coreWalkers{path: buildPath, mapping: parseMap}
					if kind == "path" {
						walkers.path = func(n *vault.Note, idx *graph.Index, facts map[string]noteFacts, policy schema.ArtifactPolicy) Path {
							tree := buildPath(n, idx, facts, policy)
							if n.RelPath == "Maps/B-path.md" {
								panic(value)
							}
							return tree
						}
					} else {
						walkers.mapping = func(n *vault.Note, idx *graph.Index, facts map[string]noteFacts, policy schema.ArtifactPolicy) Map {
							tree := parseMap(n, idx, facts, policy)
							if n.RelPath == "Maps/B-map.md" {
								panic(value)
							}
							return tree
						}
					}
					m := faultModel(t, walkers, "available")
					failed := "Maps/B-" + kind + ".md"
					faults := m.CoreFaults()
					want := NoteRef{Name: "B-" + kind, RelPath: failed, Language: "ja"}
					if len(faults) != 1 || faults[0].Note != want || !strings.HasPrefix(faults[0].Reason, "navigation build failed: ") {
						t.Fatalf("navigation faults = %#v, want one named fault for %#v", faults, want)
					}
					if value != nil && faults[0].Reason != fmt.Sprintf("navigation build failed: %v", value) {
						t.Errorf("reason = %q, want original panic value", faults[0].Reason)
					}
					paths, maps := []string{}, []string{}
					for _, p := range m.Paths() {
						paths = append(paths, p.RelPath)
					}
					for _, p := range m.Maps() {
						maps = append(maps, p.RelPath)
					}
					wantPaths := []string{"Maps/A-path.md", "Maps/B-path.md", "Maps/C-path.md"}
					wantMaps := []string{"Maps/A-map.md", "Maps/B-map.md", "Maps/C-map.md"}
					if kind == "path" {
						wantPaths = []string{"Maps/A-path.md", "Maps/C-path.md"}
					} else {
						wantMaps = []string{"Maps/A-map.md", "Maps/C-map.md"}
					}
					if diff := cmp.Diff(wantPaths, paths); diff != "" {
						t.Errorf("surviving paths (-want +got):\n%s", diff)
					}
					if diff := cmp.Diff(wantMaps, maps); diff != "" {
						t.Errorf("surviving maps (-want +got):\n%s", diff)
					}
					placements := m.Placements("Concepts/Target.md")
					placed := []string{}
					for _, p := range placements {
						placed = append(placed, p.MapRelPath)
					}
					expected := append(slices.Clone(wantPaths), wantMaps...)
					if diff := cmp.Diff(expected, placed); diff != "" {
						t.Errorf("partial or missing placements (-want +got):\n%s", diff)
					}
					if !slices.ContainsFunc(m.dirNotes["Maps"], func(n NoteRef) bool { return n.RelPath == failed }) {
						t.Error("failed readable note disappeared from folder navigation")
					}
					faults[0].Note.Name = "changed"
					faults[0].Reason = "changed"
					if got := m.CoreFaults()[0]; got.Note != want || got.Reason == "changed" {
						t.Error("caller changed captured fault")
					}
				})
			}
		})
	}
}

func TestNavigationFaultOrderAndUnavailableControls(t *testing.T) {
	t.Parallel()
	walkers := coreWalkers{
		path:    func(*vault.Note, *graph.Index, map[string]noteFacts, schema.ArtifactPolicy) Path { panic("path") },
		mapping: func(*vault.Note, *graph.Index, map[string]noteFacts, schema.ArtifactPolicy) Map { panic("map") },
	}
	m := faultModel(t, walkers, "available")
	got := []string{}
	for _, f := range m.CoreFaults() {
		got = append(got, f.Note.RelPath)
	}
	want := []string{"Maps/A-map.md", "Maps/A-path.md", "Maps/B-map.md", "Maps/B-path.md", "Maps/C-map.md", "Maps/C-path.md"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("fault order (-want +got):\n%s", diff)
	}
	if faults := faultModel(t, walkers, "absent").CoreFaults(); len(faults) != 0 {
		t.Errorf("absent classification produced faults: %v", faults)
	}
	for _, state := range []string{"closed", "non-instance"} {
		if faults := faultModel(t, walkers, state).CoreFaults(); len(faults) != 0 {
			t.Errorf("%s classification produced faults: %v", state, faults)
		}
	}
	empty := coreWalkers{
		path:    func(*vault.Note, *graph.Index, map[string]noteFacts, schema.ArtifactPolicy) Path { return Path{} },
		mapping: func(*vault.Note, *graph.Index, map[string]noteFacts, schema.ArtifactPolicy) Map { return Map{} },
	}
	if faults := faultModel(t, empty, "available").CoreFaults(); len(faults) != 0 {
		t.Errorf("legitimate empty trees produced faults: %v", faults)
	}
	if faults := faultModel(t, coreWalkers{path: buildPath, mapping: parseMap}, "available").CoreFaults(); len(faults) != 0 {
		t.Errorf("successful trees produced faults: %v", faults)
	}
	var absent *Model
	if absent.CoreFaults() != nil {
		t.Error("absent model has faults")
	}
}

func faultModel(t *testing.T, walkers coreWalkers, state string) *Model {
	t.Helper()
	files := []capturedFile{{path: "Concepts/Target.md", note: vault.Parse("Concepts/Target.md", []byte("---\ntitle: Target\ntype: concept\n---\nbody\n"))}}
	for _, name := range []string{"A", "B", "C"} {
		for _, kind := range []string{"map", "path"} {
			typ, heading := "moc", "## Shelf"
			if kind == "path" {
				typ, heading = "study-path", "## Shelf {sequence=primary}"
			}
			rel := "Maps/" + name + "-" + kind + ".md"
			n := vault.Parse(rel, []byte(fmt.Sprintf("---\ntitle: %s-%s\ntype: %s\nlang: ja\n---\n%s\n- [[Target]]\n", name, kind, typ, heading)))
			files = append(files, capturedFile{path: rel, note: n})
		}
	}
	notes := []*vault.Note{}
	for _, f := range files {
		notes = append(notes, f.note)
	}
	contract := testContract(t)
	roles := contract.NavigationRoles()
	if state == "absent" {
		roles = schema.NavigationRoles{}
	}
	policy := contract.ArtifactPolicy()
	if state == "closed" {
		roles = loadCapabilityContract(t, `[navigation]
path_types = ["missing-type"]
map_types = ["moc"]
`, `[artifacts]
non_instance_dirs = ["System/templates"]
`).NavigationRoles()
	}
	if state == "non-instance" {
		policy = loadCapabilityContract(t, `[navigation]
path_types = ["study-path"]
map_types = ["moc"]
`, `[artifacts]
non_instance_dirs = ["Maps"]
`).ArtifactPolicy()
	}
	return newModel(files, graph.New(notes, nil), roles, contract.KnowledgeScope(), policy, contract.JournalDir(), contract.ArticleLanguage(), contract.AuthoredDate(), contract.Settlement(), walkers)
}
