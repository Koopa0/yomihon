package nav

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

func writeNavFixture(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatalf("mkdir %s: %v", rel, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func capturedModel(
	t *testing.T,
	root string,
	roles schema.NavigationRoles,
	policy schema.ArtifactPolicy,
	resolver Resolver,
) *Model {
	t.Helper()
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Errorf("Reader.Close() error = %v", closeErr)
		}
	})
	scan, err := reader.ScanComplete(t.Context())
	if err != nil {
		t.Fatalf("ScanComplete() error = %v", err)
	}

	notes := make(map[string]*vault.Note)
	noteList := make([]*vault.Note, 0, len(scan.Files()))
	resources := make([]string, 0, len(scan.Files()))
	for _, entry := range scan.Files() {
		if !strings.HasSuffix(entry.Path(), ".md") {
			resources = append(resources, entry.Path())
			continue
		}
		data, readErr := reader.ReadFile(t.Context(), entry)
		if readErr != nil {
			t.Fatalf("ReadFile() error = %v", readErr)
		}
		note := vault.Parse(entry.Path(), data)
		notes[entry.Path()] = note
		noteList = append(noteList, note)
	}
	if resolver == nil {
		resolver = graph.New(noteList, resources)
	}
	return New(scan.Files(), notes, resolver, roles, policy)
}

func testCapabilities(t *testing.T) (schema.NavigationRoles, schema.ArtifactPolicy) {
	t.Helper()
	contract, err := schema.LoadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("schema.LoadFile: %v", err)
	}
	return contract.NavigationRoles(), contract.ArtifactPolicy()
}

func testArtifactPolicy(t *testing.T) schema.ArtifactPolicy {
	t.Helper()
	_, policy := testCapabilities(t)
	return policy
}

func TestMapEntryCountsIncludesWarningsButMatchesResolvedOnly(t *testing.T) {
	t.Parallel()

	m := Map{Branches: []Branch{
		{
			Entries: []Entry{
				{Status: "ready", Kind: EntryResolved},
				{Status: "ready", Kind: EntryUnresolved},
			},
			Subbranches: []Branch{{Entries: []Entry{
				{Status: "draft", Kind: EntryResolved},
				{Status: "ready", Kind: EntryResolved},
			}}},
		},
		{Entries: []Entry{{Status: "ready", Kind: EntryNonInstance}}},
	}}

	matching, total := m.EntryCounts("ready")
	if matching != 2 || total != 5 {
		t.Fatalf("EntryCounts(ready) = (%d, %d), want (2, 5)", matching, total)
	}
}

func TestBranchEntryCountsIncludesDescendants(t *testing.T) {
	t.Parallel()

	branch := Branch{
		Entries: []Entry{{Status: "draft", Kind: EntryResolved}},
		Subbranches: []Branch{{Entries: []Entry{
			{Status: "ready", Kind: EntryResolved},
			{Kind: EntryAmbiguous},
		}}},
	}

	matching, total := branch.EntryCounts("ready")
	if matching != 1 || total != 3 {
		t.Fatalf("EntryCounts(ready) = (%d, %d), want (1, 3)", matching, total)
	}
}

func loadCapabilityContract(t *testing.T, navigation, artifacts string) *schema.Contract {
	t.Helper()
	contractPath := filepath.Join(t.TempDir(), "vault-schema.toml")
	text := `schema_version = "1"

[enums]
type = ["concept", "moc", "study-path"]

[enums.status]
note = ["draft"]

[fields]
required = ["title", "type"]
known = ["title", "type", "based_on"]

[rules]
concept_requires_provenance = ["based_on"]

[scan]
knowledge_dirs = ["Concepts", "Maps"]
` + navigation + artifacts + `
[[lifecycle]]
status = "draft"
applies_to = ["*"]
from = []
owner = ["koopa"]
`
	if err := os.WriteFile(contractPath, []byte(text), 0o600); err != nil {
		t.Fatalf("write capability contract: %v", err)
	}
	contract, err := schema.LoadFile(contractPath)
	if err != nil {
		t.Fatalf("schema.LoadFile: %v", err)
	}
	return contract
}

func TestNewBuildsFromCapturedProjectionAfterSourceDisappears(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	targetPath := "Concepts/go/Target.md"
	mapPath := "Maps/Course.md"
	targetBytes := []byte("---\ntitle: Target\ntype: concept\ndomain: golang\nstatus: growing\n---\nbody\n")
	mapBytes := []byte("---\ntitle: Course\ntype: study-path\ndomain: golang\n---\n## Shelf\n- [[Target]]\n")
	writeNavFixture(t, root, targetPath, string(targetBytes))
	writeNavFixture(t, root, "Concepts/go/Unreadable.md", "not captured\n")
	writeNavFixture(t, root, mapPath, string(mapBytes))
	writeNavFixture(t, root, "System/reports/audit.md", "report\n")

	reader, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open: %v", err)
	}
	scan, err := reader.ScanComplete(t.Context())
	if err != nil {
		t.Fatalf("ScanComplete: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("Reader.Close: %v", err)
	}
	if err := os.RemoveAll(root); err != nil {
		t.Fatalf("remove captured vault: %v", err)
	}

	roles, policy := testCapabilities(t)
	model := New(
		scan.Files(),
		map[string]*vault.Note{
			targetPath: vault.Parse(targetPath, targetBytes),
			mapPath:    vault.Parse(mapPath, mapBytes),
		},
		resolver(t, targetPath, mapPath),
		roles,
		policy,
	)

	modified := make(map[string]time.Time)
	for _, entry := range scan.Files() {
		modified[entry.Path()] = entry.ModTime()
	}
	wantNotes := []NoteSummary{
		{
			Title: "Target", RelPath: targetPath, Type: "concept", Status: "growing",
			Modified: modified[targetPath],
		},
		{
			Title: "Course", RelPath: mapPath, Type: "study-path",
			Modified: modified[mapPath],
		},
	}
	if diff := cmp.Diff(wantNotes, model.KnowledgeNotes()); diff != "" {
		t.Errorf("New KnowledgeNotes mismatch (-want +got):\n%s", diff)
	}
	wantPaths := []Map{{
		Title: "Course", RelPath: mapPath, Domain: "golang", Type: "study-path",
		Branches: []Branch{{
			Heading: "Shelf",
			Level:   2,
			Entries: []Entry{{
				Text: "Target", Target: "Target", RelPath: targetPath, Status: "growing", Kind: EntryResolved,
			}},
		}},
	}}
	if diff := cmp.Diff(wantPaths, model.Paths()); diff != "" {
		t.Errorf("New Paths mismatch (-want +got):\n%s", diff)
	}
	if got := model.Reports(); len(got) != 1 || got[0].RelPath != "System/reports/audit.md" {
		t.Errorf("New Reports = %+v, want captured report", got)
	}
	dir, siblings := model.Siblings("Concepts/go/Unreadable.md")
	if dir != "Concepts/go" || len(siblings) != 2 {
		t.Errorf("New Siblings(unreadable note) = (%q, %+v), want captured folder membership", dir, siblings)
	}
}

func TestNewUsesEntryModTime(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	relPath := "Concepts/go/Channels.md"
	noteBytes := []byte("---\ntitle: Channels\ntype: concept\nstatus: growing\n---\nbody\n")
	writeNavFixture(t, root, relPath, string(noteBytes))
	fullPath := filepath.Join(root, filepath.FromSlash(relPath))
	captured := time.Date(2026, time.July, 9, 14, 30, 0, 0, time.UTC)
	if err := os.Chtimes(fullPath, captured, captured); err != nil {
		t.Fatalf("Chtimes captured: %v", err)
	}

	reader, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Errorf("Reader.Close: %v", closeErr)
		}
	})
	scan, err := reader.ScanComplete(t.Context())
	if err != nil {
		t.Fatalf("ScanComplete: %v", err)
	}
	changed := captured.Add(24 * time.Hour)
	if err := os.Chtimes(fullPath, changed, changed); err != nil {
		t.Fatalf("Chtimes changed: %v", err)
	}

	roles, policy := testCapabilities(t)
	model := New(
		scan.Files(),
		map[string]*vault.Note{relPath: vault.Parse(relPath, noteBytes)},
		resolver(t, relPath),
		roles,
		policy,
	)
	want := []NoteSummary{{
		Title: "Channels", RelPath: relPath, Type: "concept", Status: "growing", Modified: captured,
	}}
	if diff := cmp.Diff(want, model.KnowledgeNotes()); diff != "" {
		t.Errorf("New KnowledgeNotes mismatch (-want +got):\n%s", diff)
	}
}

func TestNewBuildsMapTypesAndReversePlacements(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeNavFixture(t, root, "Concepts/go/Target.md", "---\ntitle: Target\ntype: concept\ndomain: golang\nstatus: growing\n---\nbody\n")
	writeNavFixture(t, root, "A/Dup.md", "body\n")
	writeNavFixture(t, root, "B/Dup.md", "body\n")

	fixtures := []struct {
		rel, title, noteType, domain string
	}{
		{rel: "Maps/Course.md", title: "Course", noteType: "study-path", domain: "golang"},
		{rel: "Maps/A-zeta.md", title: "Zeta", noteType: "moc", domain: "golang"},
		{rel: "Maps/Beta.md", title: "Beta", noteType: "topic-map", domain: "japanese"},
		{rel: "Maps/Z-alpha.md", title: "Alpha", noteType: "source-map", domain: "golang"},
	}
	for _, f := range fixtures {
		writeNavFixture(t, root, f.rel, fmt.Sprintf("---\ntitle: %s\ntype: %s\ndomain: %s\n---\n## Shelf\n- [[Target]]\n- [[Ghost]]\n- [[Dup]]\n", f.title, f.noteType, f.domain))
	}

	roles, policy := testCapabilities(t)
	model := capturedModel(t, root, roles, policy, nil)

	resolvedBranches := []Branch{{
		Heading: "Shelf",
		Level:   2,
		Entries: []Entry{{Text: "Target", Target: "Target", RelPath: "Concepts/go/Target.md", Status: "growing", Kind: EntryResolved}},
	}}
	wantPaths := []Map{{
		Title: "Course", RelPath: "Maps/Course.md", Domain: "golang", Type: "study-path",
		Branches: []Branch{{
			Heading: "Shelf",
			Level:   2,
			Entries: []Entry{
				{Text: "Target", Target: "Target", RelPath: "Concepts/go/Target.md", Status: "growing", Kind: EntryResolved},
				{Text: "Ghost", Target: "Ghost", Kind: EntryUnresolved},
				{Text: "Dup", Target: "Dup", Kind: EntryAmbiguous, Candidates: []string{"A/Dup.md", "B/Dup.md"}},
			},
		}},
	}}
	if diff := cmp.Diff(wantPaths, model.Paths()); diff != "" {
		t.Errorf("New Paths mismatch (-want +got):\n%s", diff)
	}
	wantMaps := []Map{
		{Title: "Alpha", RelPath: "Maps/Z-alpha.md", Domain: "golang", Type: "source-map", Branches: resolvedBranches},
		{Title: "Zeta", RelPath: "Maps/A-zeta.md", Domain: "golang", Type: "moc", Branches: resolvedBranches},
		{Title: "Beta", RelPath: "Maps/Beta.md", Domain: "japanese", Type: "topic-map", Branches: resolvedBranches},
	}
	if diff := cmp.Diff(wantMaps, model.Maps()); diff != "" {
		t.Errorf("New Maps mismatch (-want +got):\n%s", diff)
	}

	wantPlacements := []Placement{
		{MapRelPath: "Maps/Course.md", Headings: []string{"Shelf"}},
		{MapRelPath: "Maps/Z-alpha.md", Headings: []string{"Shelf"}},
		{MapRelPath: "Maps/A-zeta.md", Headings: []string{"Shelf"}},
		{MapRelPath: "Maps/Beta.md", Headings: []string{"Shelf"}},
	}
	if diff := cmp.Diff(wantPlacements, model.Placements("Concepts/go/Target.md")); diff != "" {
		t.Errorf("Placements(Target) mismatch (-want +got):\n%s", diff)
	}
	if got := model.Placements("Ghost"); len(got) != 0 {
		t.Errorf("Placements(Ghost) = %v, want empty", got)
	}
	if got := model.Placements(""); len(got) != 0 {
		t.Errorf("Placements(empty path) = %v, want no unresolved or ambiguous rows indexed", got)
	}
}

func TestNewRetainsNonInstanceStudyPathRowsAsWarnings(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeNavFixture(t, root, "Concepts/Instance.md", "---\ntitle: Instance\ntype: concept\nstatus: draft\n---\nbody\n")
	writeNavFixture(t, root, "System/templates/Template target.md", "---\ntitle: Template target\ntype: concept\nstatus: ready\n---\nbody\n")
	writeNavFixture(t, root, "System/templates/Template map.md", "---\ntitle: Template map\ntype: moc\n---\n## Shelf\n- [[Instance]]\n")
	writeNavFixture(t, root, "Maps/Course.md", "---\ntitle: Course\ntype: study-path\n---\n## Shelf\n- [[Template target]]\n- [[Instance]]\n")

	roles, policy := testCapabilities(t)
	model := capturedModel(t, root, roles, policy, nil)

	if len(model.Maps()) != 0 {
		t.Errorf("New Maps = %+v, want non-instance map document absent", model.Maps())
	}
	if len(model.Paths()) != 1 || len(model.Paths()[0].Branches) != 1 {
		t.Fatalf("New Paths = %+v, want one course branch", model.Paths())
	}
	wantEntries := []Entry{
		{Text: "Template target", Target: "Template target", Kind: EntryNonInstance},
		{Text: "Instance", Target: "Instance", RelPath: "Concepts/Instance.md", Status: "draft", Kind: EntryResolved},
	}
	if diff := cmp.Diff(wantEntries, model.Paths()[0].Branches[0].Entries); diff != "" {
		t.Errorf("New path entries mismatch (-want +got):\n%s", diff)
	}
	for _, note := range model.KnowledgeNotes() {
		if note.RelPath == "System/templates/Template target.md" || note.RelPath == "System/templates/Template map.md" {
			t.Errorf("New KnowledgeNotes contains non-instance %q", note.RelPath)
		}
	}
	if got := model.Placements("System/templates/Template target.md"); len(got) != 0 {
		t.Errorf("Placements(non-instance target) = %+v, want empty", got)
	}
	if got := model.Placements("Concepts/Instance.md"); len(got) != 1 {
		t.Errorf("Placements(instance target) = %+v, want one", got)
	}
	dir, siblings := model.Siblings("System/templates/Template target.md")
	if dir != "System/templates" || len(siblings) != 2 {
		t.Errorf("Siblings(non-instance target) = (%q, %+v), want unchanged folder presence", dir, siblings)
	}
}

func TestNewOmitsNonInstanceTargetsFromGeneralMaps(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeNavFixture(t, root, "Concepts/Instance.md", "---\ntitle: Instance\ntype: concept\nstatus: draft\n---\nbody\n")
	writeNavFixture(t, root, "System/templates/Template target.md", "---\ntitle: Template target\ntype: concept\nstatus: ready\n---\nbody\n")
	writeNavFixture(t, root, "Maps/General.md", "---\ntitle: General\ntype: moc\n---\n## Shelf\n- [[Template target]]\n- [[Instance]]\n")

	roles, policy := testCapabilities(t)
	model := capturedModel(t, root, roles, policy, nil)

	if len(model.Maps()) != 1 || len(model.Maps()[0].Branches) != 1 {
		t.Fatalf("New Maps = %+v, want one general-map branch", model.Maps())
	}
	wantEntries := []Entry{{Text: "Instance", Target: "Instance", RelPath: "Concepts/Instance.md", Status: "draft", Kind: EntryResolved}}
	if diff := cmp.Diff(wantEntries, model.Maps()[0].Branches[0].Entries); diff != "" {
		t.Errorf("New general-map entries mismatch (-want +got):\n%s", diff)
	}
	if got := model.Placements("System/templates/Template target.md"); len(got) != 0 {
		t.Errorf("Placements(non-instance general-map target) = %+v, want empty", got)
	}
}

func TestNewReportsIndependentCapabilityDiagnostics(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeNavFixture(t, root, "Concepts/Target.md", "---\ntitle: Target\ntype: concept\nstatus: draft\n---\nbody\n")
	writeNavFixture(t, root, "Maps/Map.md", "---\ntitle: Map\ntype: moc\n---\n## Shelf\n- [[Target]]\n")
	validRoles, validPolicy := testCapabilities(t)
	invalidNavigation := loadCapabilityContract(t, `
[navigation]
path_types = ["missing-type"]
map_types = ["moc"]
`, `
[artifacts]
non_instance_dirs = ["System/templates"]
`)
	incompleteNavigation := loadCapabilityContract(t, `
[navigation]
path_types = ["study-path"]
`, `
[artifacts]
non_instance_dirs = ["System/templates"]
`)
	invalidArtifact := loadCapabilityContract(t, `
[navigation]
path_types = ["study-path"]
map_types = ["moc"]
`, `
[artifacts]
non_instance_dirs = ["../templates"]
`)
	incompleteArtifact := loadCapabilityContract(t, `
[navigation]
path_types = ["study-path"]
map_types = ["moc"]
`, `
[artifacts]
`)

	tests := []struct {
		name               string
		roles              schema.NavigationRoles
		policy             schema.ArtifactPolicy
		wantNavigation     string
		wantArtifact       string
		wantKnowledgeNotes int
	}{
		{
			name:               "missing navigation",
			roles:              schema.NavigationRoles{},
			policy:             validPolicy,
			wantNavigation:     "contract declares no navigation roles",
			wantKnowledgeNotes: 2,
		},
		{
			name:               "invalid navigation",
			roles:              invalidNavigation.NavigationRoles(),
			policy:             validPolicy,
			wantNavigation:     `missing-type`,
			wantKnowledgeNotes: 2,
		},
		{
			name:               "incomplete navigation",
			roles:              incompleteNavigation.NavigationRoles(),
			policy:             validPolicy,
			wantNavigation:     `missing required key "map_types"`,
			wantKnowledgeNotes: 2,
		},
		{
			name:         "missing artifact policy",
			roles:        validRoles,
			policy:       schema.ArtifactPolicy{},
			wantArtifact: "contract declares no artifact policy",
		},
		{
			name:         "invalid artifact policy",
			roles:        validRoles,
			policy:       invalidArtifact.ArtifactPolicy(),
			wantArtifact: `../templates`,
		},
		{
			name:         "incomplete artifact policy",
			roles:        validRoles,
			policy:       incompleteArtifact.ArtifactPolicy(),
			wantArtifact: `missing required key "non_instance_dirs"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			model := capturedModel(t, root, tt.roles, tt.policy, nil)
			if tt.wantNavigation == "" && model.NavigationDiagnostic() != "" {
				t.Errorf("NavigationDiagnostic = %q, want exactly empty while artifact policy is unavailable", model.NavigationDiagnostic())
			} else if tt.wantNavigation != "" && !strings.Contains(model.NavigationDiagnostic(), tt.wantNavigation) {
				t.Errorf("NavigationDiagnostic = %q, want substring %q", model.NavigationDiagnostic(), tt.wantNavigation)
			}
			if tt.wantArtifact == "" && model.ArtifactDiagnostic() != "" {
				t.Errorf("ArtifactDiagnostic = %q, want exactly empty while navigation roles are unavailable", model.ArtifactDiagnostic())
			} else if tt.wantArtifact != "" && !strings.Contains(model.ArtifactDiagnostic(), tt.wantArtifact) {
				t.Errorf("ArtifactDiagnostic = %q, want substring %q", model.ArtifactDiagnostic(), tt.wantArtifact)
			}
			if len(model.Paths()) != 0 || len(model.Maps()) != 0 {
				t.Errorf("degraded navigation = %d paths, %d maps; want unavailable", len(model.Paths()), len(model.Maps()))
			}
			if got := len(model.KnowledgeNotes()); got != tt.wantKnowledgeNotes {
				t.Errorf("degraded KnowledgeNotes = %d, want %d", got, tt.wantKnowledgeNotes)
			}
			if len(model.Folders()) == 0 {
				t.Error("degraded Folders is empty, want browsing preserved")
			}
		})
	}
}

func TestNewKeepsZeroEntryMap(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeNavFixture(t, root, "Maps/Empty.md", "---\ntitle: Empty map\ntype: moc\n---\n## Shelf\n- [[Unwritten]]\n")
	roles, policy := testCapabilities(t)
	model := capturedModel(t, root, roles, policy, nil)
	want := []Map{{Title: "Empty map", RelPath: "Maps/Empty.md", Type: "moc"}}
	if diff := cmp.Diff(want, model.Maps()); diff != "" {
		t.Errorf("New zero-entry Maps mismatch (-want +got):\n%s", diff)
	}
}

func TestNewBuildsJournalFromCapturedMtimes(t *testing.T) {
	t.Parallel()

	t.Run("newest five without frontmatter", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		mtimes := make(map[string]time.Time)
		for day := 1; day <= 7; day++ {
			rel := fmt.Sprintf("Diary/2026-07-%02d.md", day)
			writeNavFixture(t, root, rel, fmt.Sprintf("# Day %d\n", day))
			mtimes[rel] = time.Date(2026, time.July, day, 8, 0, 0, 0, time.UTC)
			full := filepath.Join(root, filepath.FromSlash(rel))
			if err := os.Chtimes(full, mtimes[rel], mtimes[rel]); err != nil {
				t.Fatalf("Chtimes() error = %v", err)
			}
		}
		roles, policy := testCapabilities(t)
		model := capturedModel(t, root, roles, policy, nil)
		want := []JournalEntry{
			{Title: "2026-07-07", RelPath: "Diary/2026-07-07.md", Modified: mtimes["Diary/2026-07-07.md"]},
			{Title: "2026-07-06", RelPath: "Diary/2026-07-06.md", Modified: mtimes["Diary/2026-07-06.md"]},
			{Title: "2026-07-05", RelPath: "Diary/2026-07-05.md", Modified: mtimes["Diary/2026-07-05.md"]},
			{Title: "2026-07-04", RelPath: "Diary/2026-07-04.md", Modified: mtimes["Diary/2026-07-04.md"]},
			{Title: "2026-07-03", RelPath: "Diary/2026-07-03.md", Modified: mtimes["Diary/2026-07-03.md"]},
		}
		if diff := cmp.Diff(want, model.Journal()); diff != "" {
			t.Errorf("New Journal mismatch (-want +got):\n%s", diff)
		}
	})

	for _, name := range []string{"absent", "empty"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writeNavFixture(t, root, "Notes/ordinary.md", "ordinary note\n")
			if name == "empty" {
				if err := os.Mkdir(filepath.Join(root, "Diary"), 0o750); err != nil {
					t.Fatalf("mkdir Diary: %v", err)
				}
			}
			roles, policy := testCapabilities(t)
			model := capturedModel(t, root, roles, policy, nil)
			if len(model.Journal()) != 0 {
				t.Errorf("New Journal = %v, want empty", model.Journal())
			}
		})
	}
}

// TestNewCarriesScannerMtimes proves Home's freshness data comes from the
// scanner-owned capture handed to New, not from another stat inside nav or a
// request handler.
func TestNewCarriesScannerMtimes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	rel := "Concepts/go/Channels.md"
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := "---\ntitle: Channels\ntype: concept\nstatus: growing\n---\n\nbody\n"
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		t.Fatalf("write note: %v", err)
	}

	captured := time.Date(2026, time.July, 9, 14, 30, 0, 0, time.UTC)
	if err := os.Chtimes(full, captured, captured); err != nil {
		t.Fatalf("Chtimes() error = %v", err)
	}
	roles, policy := testCapabilities(t)
	model := capturedModel(t, root, roles, policy, resolver(t, rel))
	want := []NoteSummary{{
		Title:    "Channels",
		RelPath:  rel,
		Type:     "concept",
		Status:   "growing",
		Modified: captured,
	}}
	if diff := cmp.Diff(want, model.KnowledgeNotes()); diff != "" {
		t.Errorf("New KnowledgeNotes mismatch (-want +got):\n%s", diff)
	}
}

// resolver builds a real *graph.Index (testing.md's "real first") from
// in-memory note paths — no disk — so parseBranches resolves entry
// wikilinks through the exact normalization and ambiguity rules production
// uses. Same technique as internal/render's tests.
func resolver(t *testing.T, paths ...string) *graph.Index {
	t.Helper()
	notes := make([]graph.NoteInput, 0, len(paths))
	for _, p := range paths {
		notes = append(notes, graph.NoteInput{Path: p})
	}
	return graph.BuildFromNotes(notes, nil)
}

// TestParseBranchesGoShape covers the pipe-format Go map shape: H2/H3
// headings "slug | English | Chinese" (the English column becomes the
// label) and "- [[Entry]]" bullets, with prose lines (範圍/★) between
// headings that must NOT become entries, and a trailing part with no
// entries that must be pruned away.
func TestParseBranchesGoShape(t *testing.T) {
	t.Parallel()

	idx := resolver(t, "L/Entry A.md", "L/Entry B.md", "L/Entry C.md")
	statusByPath := map[string]string{
		"L/Entry A.md": "draft",
		"L/Entry B.md": schema.SealStatus,
		// Entry C intentionally absent -> empty status badge.
	}

	body := "> intro blockquote citing [[Entry A]] — a quote, not a bullet\n" +
		"\n" +
		"## data-and-hardware | Data and the Hardware | 資料與硬體本質\n" +
		"\n" +
		"範圍:this is prose, not a list item, and it names [[Entry B]].\n" +
		"★ 待補模組:bits-bytes-words (prose only).\n" +
		"\n" +
		"### text-as-bytes | Text as Bytes | 文字即 bytes\n" +
		"\n" +
		"- [[Entry A]]\n" +
		"- [[Entry B]]\n" +
		"\n" +
		"### alignment | Alignment | 對齊\n" +
		"\n" +
		"- [[Entry C]]\n" +
		"\n" +
		"## empty-part | Empty Part | 空帶\n" +
		"\n" +
		"範圍:prose only, no entries anywhere under this part.\n"

	want := []Branch{
		{
			Heading: "Data and the Hardware",
			Level:   2,
			Subbranches: []Branch{
				{
					Heading: "Text as Bytes",
					Level:   3,
					Entries: []Entry{
						{Text: "Entry A", Target: "Entry A", RelPath: "L/Entry A.md", Status: "draft"},
						{Text: "Entry B", Target: "Entry B", RelPath: "L/Entry B.md", Status: schema.SealStatus},
					},
				},
				{
					Heading: "Alignment",
					Level:   3,
					Entries: []Entry{
						{Text: "Entry C", Target: "Entry C", RelPath: "L/Entry C.md"},
					},
				},
			},
		},
	}

	got := parseBranches(body, idx, statusByPath, testArtifactPolicy(t), false)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("parseBranches (Go shape) mismatch (-want +got):\n%s", diff)
	}
}

// TestParseBranchesMinnaShape covers the 大家 shape: the warm-up branch holds
// direct P entries and the course-sequence branch holds a nested L entry tree;
// the daily-loop (ordered list), learning levels (a table), and gaps (task
// checkboxes) branches carry no entry bullets and must prune away — even the
// gap task item that contains a [[wikilink]], and even the loop's ordered item
// that contains one. A "待建" bullet with no wikilink is not an entry. The H1
// title is ignored.
func TestParseBranchesMinnaShape(t *testing.T) {
	t.Parallel()

	idx := resolver(t, "jp/P01 Kana.md", "jp/L01 Intro.md", "jp/L02 Next.md", "jp/L03 Verbs.md")
	statusByPath := map[string]string{
		"jp/P01 Kana.md":  "draft",
		"jp/L01 Intro.md": "draft",
		"jp/L02 Next.md":  "draft",
		"jp/L03 Verbs.md": schema.SealStatus,
	}

	body := "# Doc Title (an H1, ignored)\n" +
		"\n" +
		"> [!info] callout prose that mentions [[Loop Link]]\n" +
		"\n" +
		"## Kana warm-up (order = lines)\n" +
		"\n" +
		"- **P01** kana · [[P01 Kana]]\n" +
		"\n" +
		"## Daily loop\n" +
		"\n" +
		"1. **step** in an ORDERED item, links [[Loop Link]] — not an entry.\n" +
		"2. another step.\n" +
		"\n" +
		"## Learning levels (a table)\n" +
		"\n" +
		"| level | 課 |\n" +
		"| --- | --- |\n" +
		"| decode | L1-3 |\n" +
		"\n" +
		"## Course sequence (order = lines)\n" +
		"\n" +
		"### Decode\n" +
		"\n" +
		"- **L1** intro · grammar · app:x · [[L01 Intro]]\n" +
		"- **L2** next · [[L02 Next]]\n" +
		"\n" +
		"### Verbs\n" +
		"\n" +
		"- **L3** verbs · [[L03 Verbs]]\n" +
		"- **L4** more · 待建\n" +
		"\n" +
		"## Gaps\n" +
		"\n" +
		"- [x] done (see [[Some Guide]])\n" +
		"- [ ] todo (spec [[Another Guide]])\n"

	want := []Branch{
		{
			Heading: "Kana warm-up (order = lines)",
			Level:   2,
			Entries: []Entry{
				{Text: "P01 Kana", Target: "P01 Kana", RelPath: "jp/P01 Kana.md", Status: "draft"},
			},
		},
		{
			Heading: "Course sequence (order = lines)",
			Level:   2,
			Subbranches: []Branch{
				{
					Heading: "Decode",
					Level:   3,
					Entries: []Entry{
						{Text: "L01 Intro", Target: "L01 Intro", RelPath: "jp/L01 Intro.md", Status: "draft"},
						{Text: "L02 Next", Target: "L02 Next", RelPath: "jp/L02 Next.md", Status: "draft"},
					},
				},
				{
					Heading: "Verbs",
					Level:   3,
					Entries: []Entry{
						{Text: "L03 Verbs", Target: "L03 Verbs", RelPath: "jp/L03 Verbs.md", Status: schema.SealStatus},
					},
				},
			},
		},
	}

	got := parseBranches(body, idx, statusByPath, testArtifactPolicy(t), false)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("parseBranches (大家 shape) mismatch (-want +got):\n%s", diff)
	}
}

// TestParseBranchesFaultTolerance proves unresolved and ambiguous rows stay out
// of navigation while a uniquely resolved neighbor remains, and malformed
// heading/list lines are ignored without panicking.
func TestParseBranchesFaultTolerance(t *testing.T) {
	t.Parallel()

	// "Dup" resolves to two files -> Ambiguous with sorted candidates.
	idx := resolver(t, "A/Dup.md", "B/Dup.md", "ok/Real.md")

	body := "## part | Part | 部\n" +
		"\n" +
		"###notaspace this is not a heading\n" +
		"#### \n" + // empty-label deeper heading, no entries -> pruned
		"- \n" + // bare bullet, no wikilink -> not an entry
		"- [ ] a task with a [[Real]] link -> excluded\n" +
		"- no wikilink here at all\n" +
		"\n" +
		"### mod | Module | 模組\n" +
		"\n" +
		"- [[Real]]\n" +
		"- [[Ghost]]\n" + // unresolved: stays on the map note, absent here
		"- [[Dup]]\n" // ambiguous: stays on the map note, absent here

	want := []Branch{
		{
			Heading: "Part",
			Level:   2,
			Subbranches: []Branch{
				{
					Heading: "Module",
					Level:   3,
					Entries: []Entry{{Text: "Real", Target: "Real", RelPath: "ok/Real.md"}},
				},
			},
		},
	}

	got := parseBranches(body, idx, map[string]string{}, testArtifactPolicy(t), false)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("parseBranches (fault tolerance) mismatch (-want +got):\n%s", diff)
	}
}

func TestParseBranchesPathRetainsUnresolvedEntry(t *testing.T) {
	t.Parallel()

	idx := resolver(t, "Writing/Existing.md")
	body := "## Course\n- [[Existing]]\n- [[Unwritten Lesson]]\n"
	want := []Branch{{
		Heading: "Course",
		Level:   2,
		Entries: []Entry{
			{Text: "Existing", Target: "Existing", RelPath: "Writing/Existing.md"},
			{Text: "Unwritten Lesson", Target: "Unwritten Lesson", Kind: EntryUnresolved},
		},
	}}

	got := parseBranches(body, idx, map[string]string{}, testArtifactPolicy(t), true)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("parseBranches(path) mismatch (-want +got):\n%s", diff)
	}
}

func TestParseBranchesPathRetainsAmbiguousEntryInOrder(t *testing.T) {
	t.Parallel()

	idx := resolver(t, "Writing/First.md", "A/Repeated.md", "B/Repeated.md", "Writing/Last.md")
	body := "## Course\n- [[First]]\n- [[Repeated|Unresolved choice]]\n- [[Last]]\n"
	want := []Branch{{
		Heading: "Course",
		Level:   2,
		Entries: []Entry{
			{Text: "First", Target: "First", RelPath: "Writing/First.md", Kind: EntryResolved},
			{
				Text:       "Unresolved choice",
				Target:     "Repeated",
				Kind:       EntryAmbiguous,
				Candidates: []string{"A/Repeated.md", "B/Repeated.md"},
			},
			{Text: "Last", Target: "Last", RelPath: "Writing/Last.md", Kind: EntryResolved},
		},
	}}

	got := parseBranches(body, idx, map[string]string{}, testArtifactPolicy(t), true)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("parseBranches(path ambiguity) mismatch (-want +got):\n%s", diff)
	}
}

// TestParseHeading and TestParseEntryItem lock down the two line
// classifiers the whole rule rests on.
func TestParseHeading(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		line      string
		wantText  string
		wantLevel int
		wantOK    bool
	}{
		{name: "h2 pipe", line: "## slug | English | 中文", wantText: "slug | English | 中文", wantLevel: 2, wantOK: true},
		{name: "h3 plain", line: "### 解碼期", wantText: "解碼期", wantLevel: 3, wantOK: true},
		{name: "h1 ignored", line: "# Title", wantOK: false},
		{name: "no space", line: "###notaspace", wantOK: false},
		{name: "not a heading", line: "- [[Entry]]", wantOK: false},
		{name: "h4 empty label", line: "#### ", wantText: "", wantLevel: 4, wantOK: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			text, level, ok := parseHeading(tt.line)
			if ok != tt.wantOK || text != tt.wantText || level != tt.wantLevel {
				t.Errorf("parseHeading(%q) = (%q, %d, %t), want (%q, %d, %t)",
					tt.line, text, level, ok, tt.wantText, tt.wantLevel, tt.wantOK)
			}
		})
	}
}

func TestParseEntryItem(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		line      string
		wantInner string
		wantOK    bool
	}{
		{name: "go bullet", line: "- [[Slices- Strings and Slices]]", wantInner: "Slices- Strings and Slices", wantOK: true},
		{name: "minna trailing link", line: "- **L1** intro · [[L01 〜は〜です]]", wantInner: "L01 〜は〜です", wantOK: true},
		{name: "star bullet", line: "* [[Alt]]", wantInner: "Alt", wantOK: true},
		{name: "ordered item excluded", line: "1. **step** links [[Loop Link]]", wantOK: false},
		{name: "task unchecked excluded", line: "- [ ] todo (spec [[Guide]])", wantOK: false},
		{name: "task checked excluded", line: "- [x] done, see [[Guide]]", wantOK: false},
		{name: "bullet without wikilink", line: "- **L21** 引用・意見 · 待建", wantOK: false},
		{name: "blockquote excluded", line: "> prose with [[Link]]", wantOK: false},
		{name: "bare bullet", line: "- ", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			inner, ok := parseEntryItem(tt.line)
			if ok != tt.wantOK || inner != tt.wantInner {
				t.Errorf("parseEntryItem(%q) = (%q, %t), want (%q, %t)",
					tt.line, inner, ok, tt.wantInner, tt.wantOK)
			}
		})
	}
}

// TestBuildFolderTree checks the browse tree mirrors the real directory
// structure (to whatever depth it has), strips only .md for display,
// surfaces a vault-root file as a RootNote, and orders the top level by
// lifecycle (Inbox before Concepts before Writing before Views), leaving
// deeper levels lexical.
func TestBuildFolderTree(t *testing.T) {
	t.Parallel()

	// Captured generation paths are lexical; provide the paths that way.
	paths := []string{
		"CLAUDE.md",
		"Concepts/golang/Foo.md",
		"Concepts/rust/Bar.md",
		"Inbox/note.md",
		"Views/board.base",
		"Writing/entries/golang/Entry.md",
	}

	wantFolders := []Folder{
		{Name: "Inbox", RelPath: "Inbox", Notes: []NoteRef{{Name: "note", RelPath: "Inbox/note.md"}}},
		{Name: "Concepts", RelPath: "Concepts", Subfolders: []Folder{
			{Name: "golang", RelPath: "Concepts/golang", Notes: []NoteRef{{Name: "Foo", RelPath: "Concepts/golang/Foo.md"}}},
			{Name: "rust", RelPath: "Concepts/rust", Notes: []NoteRef{{Name: "Bar", RelPath: "Concepts/rust/Bar.md"}}},
		}},
		{Name: "Writing", RelPath: "Writing", Subfolders: []Folder{
			{Name: "entries", RelPath: "Writing/entries", Subfolders: []Folder{
				{Name: "golang", RelPath: "Writing/entries/golang", Notes: []NoteRef{{Name: "Entry", RelPath: "Writing/entries/golang/Entry.md"}}},
			}},
		}},
		{Name: "Views", RelPath: "Views", Notes: []NoteRef{{Name: "board.base", RelPath: "Views/board.base"}}},
	}
	wantRoot := []NoteRef{{Name: "CLAUDE", RelPath: "CLAUDE.md"}}

	folders, rootNotes := buildFolderTree(paths)
	if diff := cmp.Diff(wantFolders, folders); diff != "" {
		t.Errorf("buildFolderTree folders mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(wantRoot, rootNotes); diff != "" {
		t.Errorf("buildFolderTree rootNotes mismatch (-want +got):\n%s", diff)
	}
}

// TestPlacements inverts the map trees into the note -> placements map the
// sidebar reads to open itself to the current note. It covers the two cases that
// make the inversion worth building: a note listed by two different study-paths
// (two placements, one per path), and a note listed by two branches of the same
// study-path (two placements, each carrying its own heading chain). An
// unresolved entry carries no path and must not appear.
func TestPlacements(t *testing.T) {
	t.Parallel()

	paths := []Map{
		{
			Title: "Go", RelPath: "Maps/go-path.md",
			Branches: []Branch{
				{Heading: "Part A", Level: 2, Subbranches: []Branch{
					{Heading: "Module 1", Level: 3, Entries: []Entry{
						{Text: "L1", Target: "L1", RelPath: "L/L1.md"},
						{Text: "Shared", Target: "Shared", RelPath: "L/Shared.md"},
					}},
				}},
				{Heading: "Part B", Level: 2, Entries: []Entry{
					{Text: "Shared", Target: "Shared", RelPath: "L/Shared.md"},
				}},
			},
		},
		{
			Title: "JP", RelPath: "Maps/jp-path.md",
			Branches: []Branch{
				{Heading: "Unit 1", Level: 2, Entries: []Entry{
					{Text: "L1", Target: "L1", RelPath: "L/L1.md"},
				}},
			},
		},
	}
	m := &Model{placementIndex: buildPlacementIndex(paths)}

	tests := []struct {
		name    string
		relPath string
		want    []Placement
	}{
		{
			name:    "listed by two study-paths",
			relPath: "L/L1.md",
			want: []Placement{
				{MapRelPath: "Maps/go-path.md", Headings: []string{"Part A", "Module 1"}},
				{MapRelPath: "Maps/jp-path.md", Headings: []string{"Unit 1"}},
			},
		},
		{
			name:    "listed by two branches of one study-path",
			relPath: "L/Shared.md",
			want: []Placement{
				{MapRelPath: "Maps/go-path.md", Headings: []string{"Part A", "Module 1"}},
				{MapRelPath: "Maps/go-path.md", Headings: []string{"Part B"}},
			},
		},
		{name: "unresolved entry never indexed", relPath: "Ghost", want: nil},
		{name: "unreferenced note", relPath: "L/Orphan.md", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if diff := cmp.Diff(tt.want, m.Placements(tt.relPath)); diff != "" {
				t.Errorf("Placements(%q) mismatch (-want +got):\n%s", tt.relPath, diff)
			}
		})
	}
}

// TestSiblings checks the "here" lookup returns a note's whole directory (itself
// included, for the caller to mark) in the folder tree's lexical order, reports
// the empty directory for a vault-root note, and yields nothing for an unknown
// directory.
func TestSiblings(t *testing.T) {
	t.Parallel()

	paths := []string{
		"Concepts/golang/Baz.md",
		"Concepts/golang/Foo.md",
		"Concepts/rust/Bar.md",
		"README.md",
	}
	m := &Model{dirNotes: buildDirNotes(paths)}

	tests := []struct {
		name      string
		relPath   string
		wantDir   string
		wantNotes []NoteRef
	}{
		{
			name:    "same-directory siblings include self",
			relPath: "Concepts/golang/Foo.md",
			wantDir: "Concepts/golang",
			wantNotes: []NoteRef{
				{Name: "Baz", RelPath: "Concepts/golang/Baz.md"},
				{Name: "Foo", RelPath: "Concepts/golang/Foo.md"},
			},
		},
		{
			name:      "vault-root note has the empty directory",
			relPath:   "README.md",
			wantDir:   "",
			wantNotes: []NoteRef{{Name: "README", RelPath: "README.md"}},
		},
		{name: "unknown directory", relPath: "Nowhere/x.md", wantDir: "Nowhere", wantNotes: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir, notes := m.Siblings(tt.relPath)
			if dir != tt.wantDir {
				t.Errorf("Siblings(%q) dir = %q, want %q", tt.relPath, dir, tt.wantDir)
			}
			if diff := cmp.Diff(tt.wantNotes, notes); diff != "" {
				t.Errorf("Siblings(%q) notes mismatch (-want +got):\n%s", tt.relPath, diff)
			}
		})
	}
}

func TestWithoutInstanceProjectionsPreservesOrdinaryBrowse(t *testing.T) {
	t.Parallel()

	original := &Model{
		navigationDiagnostic: "navigation diagnostic",
		folders:              []Folder{{Name: "Concepts", RelPath: "Concepts"}},
		rootNotes:            []NoteRef{{Name: "README", RelPath: "README.md"}},
		paths:                []Map{{Title: "Path"}},
		maps:                 []Map{{Title: "Map"}},
		journal:              []JournalEntry{{Title: "Today"}},
		reports:              []Report{{Name: "report.md"}},
		knowledgeNotes:       []NoteSummary{{Title: "A"}},
		placementIndex:       map[string][]Placement{"A.md": {{MapRelPath: "Maps/A.md"}}},
		dirNotes:             map[string][]NoteRef{"Concepts": {{Name: "A", RelPath: "Concepts/A.md"}}},
	}
	degraded := original.WithoutInstanceProjections("artifact unavailable")

	if degraded == original {
		t.Fatal("WithoutInstanceProjections() returned the mutable source model")
	}
	if len(degraded.Paths()) != 0 || len(degraded.Maps()) != 0 || len(degraded.KnowledgeNotes()) != 0 || degraded.placementIndex != nil {
		t.Errorf("instance projections remain: paths=%d maps=%d notes=%d placements=%v",
			len(degraded.Paths()), len(degraded.Maps()), len(degraded.KnowledgeNotes()), degraded.placementIndex)
	}
	if degraded.ArtifactDiagnostic() != "artifact unavailable" {
		t.Errorf("ArtifactDiagnostic = %q, want %q", degraded.ArtifactDiagnostic(), "artifact unavailable")
	}
	if diff := cmp.Diff(original.Folders(), degraded.Folders()); diff != "" {
		t.Errorf("Folders changed (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(original.RootNotes(), degraded.RootNotes()); diff != "" {
		t.Errorf("RootNotes changed (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(original.Journal(), degraded.Journal()); diff != "" {
		t.Errorf("Journal changed (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(original.Reports(), degraded.Reports()); diff != "" {
		t.Errorf("Reports changed (-want +got):\n%s", diff)
	}
	if dir, notes := degraded.Siblings("Concepts/A.md"); dir != "Concepts" || len(notes) != 1 {
		t.Errorf("Siblings() = (%q, %v), want ordinary folder browse preserved", dir, notes)
	}
	if len(original.Paths()) != 1 || len(original.Maps()) != 1 || len(original.KnowledgeNotes()) != 1 || original.placementIndex == nil {
		t.Error("WithoutInstanceProjections() mutated the source model")
	}
}

type modelProjections struct {
	folders        []Folder
	rootNotes      []NoteRef
	paths          []Map
	maps           []Map
	journal        []JournalEntry
	reports        []Report
	knowledgeNotes []NoteSummary
	placements     []Placement
	siblingDir     string
	siblings       []NoteRef
}

func immutableModelFixture() *Model {
	return &Model{
		folders: []Folder{{
			Name:    "Writing",
			RelPath: "Writing",
			Notes:   []NoteRef{{Name: "Root", RelPath: "Writing/Root.md"}},
			Subfolders: []Folder{{
				Name:    "Lessons",
				RelPath: "Writing/Lessons",
				Notes:   []NoteRef{{Name: "Lesson", RelPath: "Writing/Lessons/Lesson.md"}},
			}},
		}},
		rootNotes: []NoteRef{{Name: "README", RelPath: "README.md"}},
		paths: []Map{{
			Title:   "Path",
			RelPath: "Maps/Path.md",
			Branches: []Branch{{
				Heading: "Part",
				Entries: []Entry{{
					Text:       "Ambiguous",
					Kind:       EntryAmbiguous,
					Candidates: []string{"A/Target.md", "B/Target.md"},
				}},
				Subbranches: []Branch{{Heading: "Module", Entries: []Entry{{Text: "Lesson", RelPath: "Writing/Lessons/Lesson.md", Kind: EntryResolved}}}},
			}},
		}},
		maps:           []Map{{Title: "Map", RelPath: "Maps/Map.md", Branches: []Branch{{Heading: "Shelf"}}}},
		journal:        []JournalEntry{{Title: "Recent", RelPath: "Journal/Recent.md"}},
		reports:        []Report{{Name: "report.md", RelPath: "System/reports/report.md"}},
		knowledgeNotes: []NoteSummary{{Title: "Lesson", RelPath: "Writing/Lessons/Lesson.md"}},
		placementIndex: map[string][]Placement{
			"Writing/Lessons/Lesson.md": {{MapRelPath: "Maps/Path.md", Headings: []string{"Part", "Module"}}},
		},
		dirNotes: map[string][]NoteRef{
			"Writing/Lessons": {{Name: "Lesson", RelPath: "Writing/Lessons/Lesson.md"}},
		},
	}
}

func captureModelProjections(model *Model) modelProjections {
	dir, siblings := model.Siblings("Writing/Lessons/Lesson.md")
	return modelProjections{
		folders:        model.Folders(),
		rootNotes:      model.RootNotes(),
		paths:          model.Paths(),
		maps:           model.Maps(),
		journal:        model.Journal(),
		reports:        model.Reports(),
		knowledgeNotes: model.KnowledgeNotes(),
		placements:     model.Placements("Writing/Lessons/Lesson.md"),
		siblingDir:     dir,
		siblings:       siblings,
	}
}

func mutateModelProjections(model *Model) {
	folders := model.Folders()
	folders[0].Name = "mutated"
	folders[0].Notes[0].Name = "mutated"
	folders[0].Subfolders[0].Name = "mutated"
	folders[0].Subfolders[0].Notes[0].Name = "mutated"

	rootNotes := model.RootNotes()
	rootNotes[0].Name = "mutated"

	paths := model.Paths()
	paths[0].Title = "mutated"
	paths[0].Branches[0].Heading = "mutated"
	paths[0].Branches[0].Entries[0].Text = "mutated"
	paths[0].Branches[0].Entries[0].Candidates[0] = "mutated"
	paths[0].Branches[0].Subbranches[0].Heading = "mutated"

	maps := model.Maps()
	maps[0].Title = "mutated"
	maps[0].Branches[0].Heading = "mutated"

	journal := model.Journal()
	journal[0].Title = "mutated"
	reports := model.Reports()
	reports[0].Name = "mutated"
	knowledgeNotes := model.KnowledgeNotes()
	knowledgeNotes[0].Title = "mutated"

	placements := model.Placements("Writing/Lessons/Lesson.md")
	placements[0].MapRelPath = "mutated"
	placements[0].Headings[0] = "mutated"
	_, siblings := model.Siblings("Writing/Lessons/Lesson.md")
	siblings[0].Name = "mutated"
}

func TestModelReturnsIndependentProjections(t *testing.T) {
	t.Parallel()
	model := immutableModelFixture()
	want := captureModelProjections(immutableModelFixture())

	mutateModelProjections(model)

	if diff := cmp.Diff(want, captureModelProjections(model), cmp.AllowUnexported(modelProjections{})); diff != "" {
		t.Errorf("model changed through a returned projection (-want +got):\n%s", diff)
	}
}

func TestModelConcurrentProjectionMutationDoesNotChangePublishedData(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		model := immutableModelFixture()
		want := captureModelProjections(immutableModelFixture())
		for range 32 {
			go func() {
				for range 100 {
					mutateModelProjections(model)
				}
			}()
		}
		synctest.Wait()
		if diff := cmp.Diff(want, captureModelProjections(model), cmp.AllowUnexported(modelProjections{})); diff != "" {
			t.Errorf("concurrent callers changed model data (-want +got):\n%s", diff)
		}
	})
}

// TestBuildReports checks the .md reports (directly under System/reports/)
// come first, then the daily-briefing/ HTML briefings with latest.html
// marked, and that a daily-briefing README.md, a stray .txt, and files
// outside System/reports/ are all excluded.
func TestBuildReports(t *testing.T) {
	t.Parallel()

	paths := []string{
		"Concepts/foo.md",
		"System/reports/Run-Report.md",
		"System/reports/daily-briefing/README.md",
		"System/reports/daily-briefing/koopa0-briefing-2026-07-02.html",
		"System/reports/daily-briefing/latest.html",
		"System/reports/vault-check.md",
		"System/reports/notes.txt",
	}

	want := []Report{
		{Name: "Run-Report.md", RelPath: "System/reports/Run-Report.md"},
		{Name: "vault-check.md", RelPath: "System/reports/vault-check.md"},
		{Name: "koopa0-briefing-2026-07-02.html", RelPath: "System/reports/daily-briefing/koopa0-briefing-2026-07-02.html", Briefing: true},
		{Name: "latest.html", RelPath: "System/reports/daily-briefing/latest.html", Briefing: true, Latest: true},
	}

	got := buildReports(paths)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("buildReports mismatch (-want +got):\n%s", diff)
	}
}
