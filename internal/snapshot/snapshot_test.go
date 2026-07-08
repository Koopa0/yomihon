package snapshot

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/search"
)

func discardLogger() *slog.Logger { return slog.New(slog.DiscardHandler) }

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

	store := New(root, discardLogger())
	snap := store.Current()
	if snap == nil {
		t.Fatal("Current() = nil after New")
	}
	if snap.Search.Len() == 0 {
		t.Error("snapshot search index is empty")
	}
	if got := snap.Search.Search(search.Parse("kafka")); len(got) == 0 {
		t.Error("kafka not found in the freshly built snapshot")
	}
	if got := snap.Graph.Resolve("Alpha"); got.Kind != graph.Unique {
		t.Errorf("graph.Resolve(Alpha).Kind = %v, want Unique", got.Kind)
	}
	// The live resolver reads the same graph.
	if got := store.Resolver().Resolve("Alpha"); got.Kind != graph.Unique {
		t.Errorf("Resolver().Resolve(Alpha).Kind = %v, want Unique", got.Kind)
	}
}

// TestRescanDetectsChange pins the freshness contract: a vault change (here,
// an added note) is reflected after one scan — the mechanism the scanner runs
// on its ticker, exercised directly so the test is deterministic and fast.
func TestRescanDetectsChange(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeNote(t, root, "Concepts/Alpha.md", "---\ntitle: Alpha\ntype: concept\n---\n\nalpha body\n")

	store := New(root, discardLogger())
	if got := store.Current().Search.Search(search.Parse("widgets")); len(got) != 0 {
		t.Fatalf("widgets matched %d before the note existed", len(got))
	}

	writeNote(t, root, "Concepts/Beta.md", "---\ntitle: Beta\ntype: concept\n---\n\nbeta mentions widgets\n")
	store.rescan()

	if got := store.Current().Search.Search(search.Parse("widgets")); len(got) == 0 {
		t.Error("rescan did not pick up the added note")
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

	store := New(root, discardLogger())
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
			store.ptr.Store(buildSnapshot(root, discardLogger()))
		}
	})
	swapper.Wait()

	cancel()
	readers.Wait()
}
