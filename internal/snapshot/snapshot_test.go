package snapshot

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/search"
)

func discardLogger() *slog.Logger { return slog.New(slog.DiscardHandler) }

func snapshotSearch(tb testing.TB, idx *search.Index, query string) []search.Result {
	tb.Helper()
	results, err := idx.Search(search.Parse(query))
	if err != nil {
		tb.Fatalf("Search(%q) error: %v", query, err)
	}
	return results
}

func assertSearchArtifactPolicy(tb testing.TB, snap *Snapshot) {
	tb.Helper()
	if got := snapshotSearch(tb, snap.Search, "status:ready"); len(got) != 0 {
		tb.Errorf("metadata search returned non-instance template: %+v", got)
	}
	if got := snapshotSearch(tb, snap.Search, "Card"); len(got) != 1 || got[0].RelPath != "System/templates/Card.md" {
		tb.Errorf("plain search = %+v, want readable template", got)
	}
	counts, err := snap.Search.CountByStatus()
	if err != nil {
		tb.Fatalf("CountByStatus() error: %v", err)
	}
	if counts["ready"] != 0 || counts["draft"] != 1 {
		tb.Errorf("status counts = %v, want draft instance only", counts)
	}
}

func testCapabilities(tb testing.TB) (schema.NavigationRoles, schema.ArtifactPolicy) {
	tb.Helper()
	contract, err := schema.LoadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		tb.Fatalf("schema.LoadFile: %v", err)
	}
	return contract.NavigationRoles(), contract.ArtifactPolicy()
}

func writeNote(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

// TestNewBuildsSnapshot pins that New produces a coherent, non-nil snapshot:
// all three models built from the same vault, each usable.
func TestNewBuildsSnapshot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeNote(t, root, "Concepts/Alpha.md", "---\ntitle: Alpha\ntype: concept\n---\n\nalpha body about kafka\n")

	roles, policy := testCapabilities(t)
	store := New(root, discardLogger(), roles, policy)
	snap := store.Current()
	if snap == nil {
		t.Fatal("Current() = nil after New")
	}
	if snap.Search.Len() == 0 {
		t.Error("snapshot search index is empty")
	}
	if got := snapshotSearch(t, snap.Search, "kafka"); len(got) == 0 {
		t.Error("kafka not found in the freshly built snapshot")
	}
	if got := snap.Graph.Resolve("Alpha"); got.Kind != graph.Unique {
		t.Errorf("graph.Resolve(Alpha).Kind = %v, want Unique", got.Kind)
	}
	// The live resolver reads the same graph.
	if got := store.Resolver().Resolve("Alpha"); got.Kind != graph.Unique {
		t.Errorf("Resolver().Resolve(Alpha).Kind = %v, want Unique", got.Kind)
	}
	if got := snap.Nav.KnowledgeNotes; len(got) != 1 || got[0].Modified.IsZero() {
		t.Errorf("snapshot KnowledgeNotes = %+v, want the scanner-captured mtime published with Alpha", got)
	}
}

// TestRescanDetectsChange pins the freshness contract: a vault change (here,
// an added note) is reflected after one scan — the mechanism the scanner runs
// on its ticker, exercised directly so the test is deterministic and fast.
func TestRescanDetectsChange(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeNote(t, root, "Concepts/Alpha.md", "---\ntitle: Alpha\ntype: concept\n---\n\nalpha body\n")

	roles, policy := testCapabilities(t)
	store := New(root, discardLogger(), roles, policy)
	if got := snapshotSearch(t, store.Current().Search, "widgets"); len(got) != 0 {
		t.Fatalf("widgets matched %d before the note existed", len(got))
	}

	const betaRel = "Concepts/Beta.md"
	writeNote(t, root, betaRel, "---\ntitle: Beta\ntype: concept\n---\n\nbeta mentions widgets\n")
	betaModified := time.Date(2026, time.July, 10, 8, 45, 0, 0, time.UTC)
	betaPath := filepath.Join(root, filepath.FromSlash(betaRel))
	if err := os.Chtimes(betaPath, betaModified, betaModified); err != nil {
		t.Fatalf("set Beta mtime: %v", err)
	}
	store.rescan()

	if got := snapshotSearch(t, store.Current().Search, "widgets"); len(got) == 0 {
		t.Error("rescan did not pick up the added note")
	}
	foundBeta := false
	for _, note := range store.Current().Nav.KnowledgeNotes {
		if note.RelPath != betaRel {
			continue
		}
		foundBeta = true
		if !note.Modified.Equal(betaModified) {
			t.Errorf("rescanned Beta mtime = %v, want scanner capture %v", note.Modified, betaModified)
		}
	}
	if !foundBeta {
		t.Error("rescanned navigation has no Beta knowledge-note summary")
	}
}

func TestRescanRetainsStartupInstanceCapabilities(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeNote(t, root, "Concepts/Alpha.md", "---\ntitle: Alpha\ntype: concept\nstatus: draft\n---\nbody\n")
	writeNote(t, root, "System/templates/Card.md", "---\ntitle: Card\ntype: concept\nstatus: ready\n---\nbody\n")
	writeNote(t, root, "Maps/Path.md", "---\ntitle: Path\ntype: study-path\n---\n## Course\n- [[Alpha]]\n")
	roles, policy := testCapabilities(t)
	store := New(root, discardLogger(), roles, policy)

	first := store.Current()
	if len(first.Nav.Paths) != 1 || first.Nav.ArtifactDiagnostic != "" || first.Nav.NavigationDiagnostic != "" {
		t.Fatalf("initial navigation = %+v, want one available path", first.Nav)
	}
	if !first.ArtifactPolicy.IsNonInstance("System/templates/Card.md") {
		t.Fatal("initial snapshot artifact policy does not classify the template")
	}
	assertSearchArtifactPolicy(t, first)

	// A malformed contract written after startup triggers a filesystem rescan
	// but cannot replace the already-derived process capabilities.
	writeNote(t, root, "System/schemas/vault-schema.toml", "not = [valid toml")
	store.rescan()

	got := store.Current()
	if got == first {
		t.Fatal("rescan did not publish a new snapshot after the contract file appeared")
	}
	if len(got.Nav.Paths) != 1 || got.Nav.ArtifactDiagnostic != "" || got.Nav.NavigationDiagnostic != "" {
		t.Errorf("rescanned navigation = %+v, want startup capabilities retained", got.Nav)
	}
	if !got.ArtifactPolicy.IsNonInstance("System/templates/Card.md") {
		t.Error("rescanned snapshot artifact policy lost the startup boundary")
	}
	assertSearchArtifactPolicy(t, got)
}

func TestNewDoesNotFabricateInstanceCapabilities(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeNote(t, root, "Maps/Path.md", "---\ntitle: Path\ntype: study-path\n---\n## Course\n- [[Ghost]]\n")
	store := New(root, discardLogger(), schema.NavigationRoles{}, schema.ArtifactPolicy{})
	snap := store.Current()

	if snap.ArtifactPolicy.Available() {
		t.Fatal("Snapshot.ArtifactPolicy.Available() = true, want unavailable zero capability")
	}
	if snap.Nav.NavigationDiagnostic == "" || snap.Nav.ArtifactDiagnostic == "" {
		t.Errorf("snapshot diagnostics = navigation %q artifact %q, want both named", snap.Nav.NavigationDiagnostic, snap.Nav.ArtifactDiagnostic)
	}
	if len(snap.Nav.Paths) != 0 || len(snap.Nav.Maps) != 0 || len(snap.Nav.KnowledgeNotes) != 0 {
		t.Errorf("snapshot instance projections = paths %d maps %d notes %d, want unavailable", len(snap.Nav.Paths), len(snap.Nav.Maps), len(snap.Nav.KnowledgeNotes))
	}
	if len(snap.Nav.Folders) == 0 {
		t.Error("snapshot folder navigation is empty, want non-instance-independent browsing")
	}
}

// TestConcurrentReadDuringSwap runs many readers of the atomic pointer while a
// writer swaps it in a loop — the exact swap the scanner performs (buildSnapshot
// then ptr.Store). Under -race this passes only because the pointer is an
// atomic.Pointer; a plain field would be flagged.
func TestConcurrentReadDuringSwap(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeNote(t, root, "Concepts/Alpha.md", "---\ntitle: Alpha\ntype: concept\n---\n\nbody\n")

	roles, policy := testCapabilities(t)
	store := New(root, discardLogger(), roles, policy)
	ctx, cancel := context.WithCancel(t.Context())

	var readers sync.WaitGroup
	for range 8 {
		readers.Go(func() {
			for ctx.Err() == nil {
				snap := store.Current()
				_ = snap.Search.Len()
				_ = snap.Graph.Resolve("Alpha")
				_ = store.Resolver().Resolve("Alpha")
			}
		})
	}

	var swapper sync.WaitGroup
	swapper.Go(func() {
		for range 100 {
			store.ptr.Store(buildSnapshot(root, discardLogger(), scanMtimes(root), roles, policy))
		}
	})
	swapper.Wait()

	cancel()
	readers.Wait()
}
