// Package snapshot owns vault freshness. It holds one Snapshot —
// {Graph, Nav, Search}, the three derived models — behind an atomic.Pointer,
// and runs a single scanner that, about every 2 seconds, stat-walks the vault
// and, on any mtime or file-set change, rebuilds all three and swaps the
// pointer once. Handlers read the pointer once per request, so an edited note
// is reflected within one scan cycle (worst case ≤3s: a ~2s cadence plus a
// ~100 ms rebuild) and the three models are never torn against one another
// (they are published together, atomically).
//
// This replaces the earlier arrangement where graph and nav were built once at
// startup and never refreshed, leaving them stale until a restart. Change
// detection is by mtime alone — no content hash: a full rebuild is ~100 ms at
// this scale, and hashing would force reading every file on every scan
// (reconsider past ~10k files).
//
// The scanner is fault-tolerant by the same asymmetry as the rest of yomihon
// (reading is fail-open): a failed build logs and publishes an empty/partial
// snapshot, and reading never depends on the build succeeding.
package snapshot

import (
	"context"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/search"
	"github.com/koopa0/yomihon/internal/vault"
)

// scanInterval is the reconciliation cadence: a full mtime stat over ~420
// files is millisecond-scale, so running it every 2 seconds is cheap and keeps
// an edited note's staleness bounded (≤3s worst case) with margin.
const scanInterval = 2 * time.Second

// Snapshot is one view of the vault's three derived models, built by three
// independent walks (simpler than a shared pass, and cheap at this scale) and
// published together via one atomic pointer swap. Because the walks read the filesystem at three
// moments, a note edited mid-rebuild can leave the models momentarily
// inconsistent about that note — but rescan captures the mtime set BEFORE the
// rebuild (see rescan), so the edit is not recorded in s.prev and the next scan
// detects it and rebuilds: the skew self-heals within one scan cycle. Any field
// may be an empty (never nil) model when its build failed — reading tolerates
// that (fail-open).
type Snapshot struct {
	Graph  *graph.Index
	Nav    *nav.Model
	Search *search.Index
}

// Store holds the current Snapshot behind an atomic.Pointer and drives the
// scanner. Current and Resolver are safe for concurrent use; prev is touched
// only by the single scanner goroutine (and set once by New before that
// goroutine starts), so it needs no lock.
type Store struct {
	ptr  atomic.Pointer[Snapshot]
	root string
	log  *slog.Logger
	prev map[string]time.Time
}

// New builds the initial Snapshot synchronously (so Current() is non-nil before
// the first request) and records the initial mtime set the scanner compares
// against. Start the scanner with Run.
func New(root string, log *slog.Logger) *Store {
	s := &Store{root: root, log: log}
	mtimes := scanMtimes(root)
	s.prev = mtimes
	snap := buildSnapshot(root, log, mtimes)
	s.ptr.Store(snap)
	log.Info("vault snapshot built",
		"notes_indexed", snap.Search.Len(),
		"paths", len(snap.Nav.Paths),
		"maps", len(snap.Nav.Maps),
		"journal", len(snap.Nav.Journal),
		"reports", len(snap.Nav.Reports))
	return s
}

// Current returns the live Snapshot. Never nil after New.
func (s *Store) Current() *Snapshot {
	return s.ptr.Load()
}

// Run drives the scanner until ctx is cancelled: every scanInterval it
// stat-walks the vault and, on any change, rebuilds and swaps. It is
// cancellable for graceful shutdown — a cancelled ctx returns promptly.
func (s *Store) Run(ctx context.Context) {
	ticker := time.NewTicker(scanInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.rescan()
		}
	}
}

// Resolver returns a wikilink resolver bound to this store's live graph. It is
// useful to consumers that intentionally want independent live resolution; a
// request renderer must instead bind the graph from its already-captured
// Snapshot so resolution cannot cross snapshot generations.
func (s *Store) Resolver() *Resolver {
	return &Resolver{store: s}
}

// Resolver resolves wikilink targets against a Store's current graph.
type Resolver struct {
	store *Store
}

// Resolve looks name up in the store's current graph. A snapshot whose graph
// failed to build is still an empty graph (never nil), against which every name
// is Unresolved — the fail-open reading behavior.
func (r *Resolver) Resolve(name string) graph.Resolution {
	return r.store.Current().Graph.Resolve(name)
}

// rescan compares the current mtime set to the previous one and, on any add,
// removal, or mtime change, rebuilds the Snapshot and swaps the pointer once.
func (s *Store) rescan() {
	now := scanMtimes(s.root)
	if mtimesEqual(s.prev, now) {
		return
	}
	snap := buildSnapshot(s.root, s.log, now)
	s.ptr.Store(snap)
	s.prev = now
	s.log.Info("vault snapshot rebuilt",
		"notes_indexed", snap.Search.Len(),
		"paths", len(snap.Nav.Paths),
		"maps", len(snap.Nav.Maps),
		"journal", len(snap.Nav.Journal),
		"reports", len(snap.Nav.Reports))
}

// buildSnapshot rebuilds all three models from the vault in dependency order
// (graph, then nav against that graph, then search). Each build failure is
// tolerated independently: it logs and substitutes an empty model, so a single
// failing model never takes the others — or reading — down.
func buildSnapshot(root string, log *slog.Logger, mtimes map[string]time.Time) *Snapshot {
	idx, err := graph.Build(root)
	if err != nil {
		log.Warn("vault graph unavailable; wikilinks will render as unresolved", "error", err)
		idx = graph.BuildFromNotes(nil, nil)
	}

	navModel, err := nav.Build(root, idx, mtimes)
	if err != nil {
		log.Warn("vault navigation unavailable; serving an empty sidebar", "error", err)
		navModel = &nav.Model{}
	}

	searchIdx, err := search.Build(root)
	if err != nil {
		log.Warn("vault search index unavailable; search will return nothing", "error", err)
		searchIdx = search.BuildFromDocs(nil)
	}

	return &Snapshot{Graph: idx, Nav: navModel, Search: searchIdx}
}

// scanMtimes returns the current {rel_path → mtime} set, reusing vault.List so
// the scan covers exactly the files the builders will see (same dot-skip and
// NFC normalization). A file that vanishes mid-scan is skipped; the next scan
// catches its removal. A failure to list the vault at all yields an empty set —
// treated as "no files", which triggers a rebuild that also degrades gracefully.
func scanMtimes(root string) map[string]time.Time {
	paths, err := vault.List(root)
	if err != nil {
		return map[string]time.Time{}
	}
	m := make(map[string]time.Time, len(paths))
	for _, p := range paths {
		info, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(p))) // #nosec G703 -- p is a vault.List entry under the operator's own root
		if statErr != nil {
			continue // best-effort: a file that vanished between the walk and the stat
		}
		m[p] = info.ModTime()
	}
	return m
}

// mtimesEqual reports whether two {path → mtime} sets are identical — any added
// or removed path, or any changed mtime, makes them unequal and forces a
// rebuild. time.Time is compared with Equal (not ==) to ignore the monotonic
// clock reading.
func mtimesEqual(a, b map[string]time.Time) bool {
	return maps.EqualFunc(a, b, func(x, y time.Time) bool { return x.Equal(y) })
}
