//go:build realvault

package nav

import (
	"os"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/sequence"
	"github.com/koopa0/yomihon/internal/vault"
)

// TestNewRealVault checks anonymous structural invariants against an
// explicitly selected private vault. Synthetic fixtures remain the contract
// for exact path identities, headings, ordering, and entry semantics.
func TestNewRealVault(t *testing.T) {
	t.Parallel()
	defer redactRealVaultPanic(t)

	root := requireRealVaultRoot(t)
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatal("open configured real vault failed")
	}
	t.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Error("close configured real vault failed")
		}
	})
	contract, err := schema.LoadReader(t.Context(), reader)
	if err != nil {
		t.Fatal("load configured real-vault contract failed")
	}
	roles := contract.NavigationRoles()
	policy := contract.ArtifactPolicy()

	scan, err := reader.ScanComplete(t.Context())
	if err != nil {
		t.Fatal("scan configured real vault failed")
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
			t.Fatal("read configured real-vault note failed")
		}
		note := vault.Parse(entry.Path(), data)
		notes[entry.Path()] = note
		noteList = append(noteList, note)
	}
	model := New(scan.Files(), notes, graph.New(noteList, resources), roles, contract.KnowledgeScope(), policy)
	if roles.Available() == (model.NavigationClosure().Diagnostic() != "") {
		t.Error("real-vault navigation diagnostic disagrees with capability availability")
	}
	if policy.Available() == (model.ArtifactClosure().Diagnostic() != "") {
		t.Error("real-vault artifact diagnostic disagrees with capability availability")
	}

	pathEntries := validateRealVaultPaths(t, model.Paths())
	mapEntries := validateRealVaultMaps(t, "map", model.Maps())
	folderCount := validateRealVaultFolders(t, model.Folders())
	validateRealVaultReports(t, model.Reports())

	if !roles.Available() && len(model.Paths())+len(model.Maps()) != 0 {
		t.Error("unavailable navigation roles produced map projections")
	}
	if !policy.Available() &&
		(len(model.Paths())+len(model.Maps()) != 0 || len(model.KnowledgeNotes()) != 0) {
		t.Error("unavailable artifact policy produced instance projections")
	}

	t.Logf(
		"real-vault aggregate: navigation_available=%t artifact_available=%t paths=%d maps=%d entries=%d folders=%d reports=%d knowledge_notes=%d",
		roles.Available(),
		policy.Available(),
		len(model.Paths()),
		len(model.Maps()),
		pathEntries+mapEntries,
		folderCount,
		len(model.Reports()),
		len(model.KnowledgeNotes()),
	)
}

func redactRealVaultPanic(t *testing.T) {
	t.Helper()

	if recover() != nil {
		t.Fatal("real-vault navigation verification panicked")
	}
}

func validateRealVaultMaps(t *testing.T, category string, maps []Map) int {
	t.Helper()

	entryCount := 0
	for mapOrdinal, model := range maps {
		if model.RelPath == "" {
			t.Errorf("%s[%d] has no source identity", category, mapOrdinal)
		}
		entries := flattenRealVaultEntries(model.Branches)
		entryCount += len(entries)
		for entryOrdinal, entry := range entries {
			switch entry.Kind {
			case EntryResolved:
				if entry.RelPath == "" {
					t.Errorf("%s[%d] entry[%d] resolved without a path", category, mapOrdinal, entryOrdinal)
				}
			case EntryUnresolved, EntryAmbiguous, EntryNonInstance:
				if entry.RelPath != "" || entry.Status != "" {
					t.Errorf("%s[%d] entry[%d] warning fabricates resolved metadata", category, mapOrdinal, entryOrdinal)
				}
			default:
				t.Errorf("%s[%d] entry[%d] has an unknown kind", category, mapOrdinal, entryOrdinal)
			}
		}
	}
	return entryCount
}

func flattenRealVaultEntries(branches []Branch) []Entry {
	var entries []Entry
	for _, branch := range branches {
		entries = append(entries, branch.Entries...)
		entries = append(entries, flattenRealVaultEntries(branch.Subbranches)...)
	}
	return entries
}

func validateRealVaultFolders(t *testing.T, folders []Folder) int {
	t.Helper()

	count := 0
	var walk func([]Folder)
	walk = func(current []Folder) {
		for ordinal, folder := range current {
			count++
			if folder.Name == "" || folder.RelPath == "" {
				t.Errorf("folder[%d] has an incomplete identity", ordinal)
			}
			for noteOrdinal, note := range folder.Notes {
				if note.Name == "" || note.RelPath == "" {
					t.Errorf("folder[%d] note[%d] has an incomplete identity", ordinal, noteOrdinal)
				}
			}
			walk(folder.Subfolders)
		}
	}
	walk(folders)
	return count
}

func validateRealVaultReports(t *testing.T, reports []Report) {
	t.Helper()

	for ordinal, report := range reports {
		if report.Name == "" || report.RelPath == "" {
			t.Errorf("report[%d] has an incomplete identity", ordinal)
		}
	}
}

func requireRealVaultRoot(t *testing.T) string {
	t.Helper()

	root := os.Getenv("YOMIHON_ROOT")
	if root == "" {
		t.Skip("YOMIHON_ROOT is required for real-vault verification")
	}
	if _, err := os.Stat(root); err != nil { // #nosec G703 -- an explicit test-only opt-in selects the private root
		t.Fatal("configured real vault is unavailable")
	}
	return root
}

// validateRealVaultPaths holds the same invariant against the live vault's
// study paths: a lesson that resolves has a path, and a lesson that does not
// never fabricates one. A row the grammar refused is not a lesson at all, so it
// carries no resolution outcome to check.
func validateRealVaultPaths(t *testing.T, paths []Path) int {
	t.Helper()

	entryCount := 0
	for pathOrdinal := range paths {
		p := &paths[pathOrdinal]
		if p.RelPath == "" {
			t.Errorf("path[%d] has no source identity", pathOrdinal)
		}
		entries := flattenRealVaultPathEntries(p.Groups)
		entryCount += len(entries)
		for entryOrdinal, entry := range entries {
			if entry.State != sequence.EntryAccepted {
				if entry.RelPath != "" || entry.Status != "" {
					t.Errorf("path[%d] entry[%d] was refused yet carries a resolution", pathOrdinal, entryOrdinal)
				}
				continue
			}
			switch entry.Kind {
			case EntryResolved:
				if entry.RelPath == "" {
					t.Errorf("path[%d] entry[%d] resolved without a path", pathOrdinal, entryOrdinal)
				}
			case EntryUnresolved, EntryAmbiguous, EntryNonInstance:
				if entry.RelPath != "" || entry.Status != "" {
					t.Errorf("path[%d] entry[%d] warning fabricates resolved metadata", pathOrdinal, entryOrdinal)
				}
			default:
				t.Errorf("path[%d] entry[%d] has an unknown kind", pathOrdinal, entryOrdinal)
			}
		}
	}
	return entryCount
}

func flattenRealVaultPathEntries(groups []*PathGroup) []*PathEntry {
	var entries []*PathEntry
	for _, g := range groups {
		for _, item := range g.Items {
			switch {
			case item.Entry != nil:
				entries = append(entries, item.Entry)
			case item.Group != nil:
				entries = append(entries, flattenRealVaultPathEntries([]*PathGroup{item.Group})...)
			}
		}
	}
	return entries
}
