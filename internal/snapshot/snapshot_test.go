package snapshot

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/search"
	"github.com/koopa0/yomihon/internal/vault"
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

func assertSearchArtifactPolicy(tb testing.TB, snap *View) {
	tb.Helper()
	if got := snapshotSearch(tb, snap.Search(), "status:ready"); len(got) != 0 {
		tb.Errorf("metadata search returned non-instance template: %+v", got)
	}
	if got := snapshotSearch(tb, snap.Search(), "Card"); len(got) != 1 || got[0].RelPath != "System/templates/Card.md" {
		tb.Errorf("plain search = %+v, want readable template", got)
	}
	counts, err := snap.Search().CountByStatus()
	if err != nil {
		tb.Fatalf("CountByStatus() error: %v", err)
	}
	if counts["ready"] != 0 || counts["draft"] != 1 {
		tb.Errorf("status counts = %v, want draft instance only", counts)
	}
}

func testContract(tb testing.TB, root string) *schema.Contract {
	tb.Helper()
	data, err := os.ReadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		tb.Fatalf("read contract fixture: %v", err)
	}
	contractPath := filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))
	if mkdirErr := os.MkdirAll(filepath.Dir(contractPath), 0o750); mkdirErr != nil {
		tb.Fatalf("mkdir contract fixture: %v", mkdirErr)
	}
	if writeErr := os.WriteFile(contractPath, data, 0o600); writeErr != nil { // #nosec G703 -- every caller supplies a testing.T.TempDir root
		tb.Fatalf("write contract fixture: %v", writeErr)
	}
	contract, err := schema.Load(root)
	if err != nil {
		tb.Fatalf("schema.Load: %v", err)
	}
	return contract
}

func newTestStore(tb testing.TB, root string, contract *schema.Contract) (*Store, *vault.Reader) {
	tb.Helper()
	reader, err := vault.Open(root)
	if err != nil {
		tb.Fatalf("vault.Open: %v", err)
	}
	tb.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			tb.Errorf("Reader.Close: %v", closeErr)
		}
	})
	store, err := New(tb.Context(), reader, discardLogger(), contract)
	if err != nil {
		tb.Fatalf("snapshot.New: %v", err)
	}
	return store, reader
}

func closeReader(tb testing.TB, reader *vault.Reader) {
	tb.Helper()
	if err := reader.Close(); err != nil {
		tb.Errorf("Reader.Close: %v", err)
	}
}

func writeNote(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func immutableViewFixture(t *testing.T) (view *View, root string) {
	t.Helper()
	root = t.TempDir()
	writeNote(t, root, "A/Foo.md", "first body\n")
	writeNote(t, root, "B/Foo.md", "second body\n")
	writeNote(t, root, "Concepts/go/Concept.md", "---\ntitle: Concept\ntype: concept\nstatus: draft\n---\nconcept body\n")
	writeNote(t, root, "Writing/Lesson.md", "---\ntitle: Lesson\ntype: lesson\nstatus: draft\nslug: lesson-l01\n---\nlesson body\n")
	writeNote(t, root, "System/slots/L01.yaml", `lesson: L01
slug: lesson-l01
title: Lesson
patterns:
  - id: p1
    template: "{A}"
    gloss_zh: "{A}"
    slots:
      A:
        fills:
          - {jp: 私, reading: わたし, zh: 我}
`)
	contract := testContract(t, root)
	store, _ := newTestStore(t, root, contract)
	return store.Current(), root
}

func TestViewReturnsImmutableGenerationProjections(t *testing.T) {
	t.Parallel()
	view, _ := immutableViewFixture(t)

	files := view.Files()
	if len(files) == 0 {
		t.Fatal("Files() returned no captured files")
	}
	wantFirstPath := files[0].Path()
	files[0] = vault.Entry{}
	if got := view.Files()[0].Path(); got != wantFirstPath {
		t.Errorf("Files()[0].Path() after mutation = %q, want %q", got, wantFirstPath)
	}

	resolution := view.Graph().Resolve("Foo")
	if resolution.Kind != graph.Ambiguous || len(resolution.Candidates) != 2 {
		t.Fatalf("Resolve(Foo) = %+v, want two ambiguous candidates", resolution)
	}
	resolution.Candidates[0] = "mutated"
	if got := view.Graph().Resolve("Foo").Candidates[0]; got != "A/Foo.md" {
		t.Errorf("Resolve(Foo) after mutation starts with %q, want %q", got, "A/Foo.md")
	}

	results := snapshotSearch(t, view.Search(), "Foo")
	if len(results) != 2 {
		t.Fatalf("Search(Foo) returned %d results, want 2", len(results))
	}
	results[0].Title = "mutated"
	if got := snapshotSearch(t, view.Search(), "Foo")[0].Title; got != "Foo" {
		t.Errorf("Search(Foo) after mutation starts with title %q, want %q", got, "Foo")
	}
	counts, err := view.Search().CountByStatus()
	if err != nil {
		t.Fatalf("CountByStatus() error = %v", err)
	}
	counts["draft"] = 0
	counts, err = view.Search().CountByStatus()
	if err != nil {
		t.Fatalf("CountByStatus() after mutation error = %v", err)
	}
	if counts["draft"] != 2 {
		t.Errorf("CountByStatus()[draft] after mutation = %d, want 2", counts["draft"])
	}
	allowed, err := view.Search().AllowedPaths(search.Parse("folder:A"))
	if err != nil {
		t.Fatalf("AllowedPaths(folder:A) error = %v", err)
	}
	delete(allowed, "A/Foo.md")
	allowed, err = view.Search().AllowedPaths(search.Parse("folder:A"))
	if err != nil {
		t.Fatalf("AllowedPaths(folder:A) after mutation error = %v", err)
	}
	if _, ok := allowed["A/Foo.md"]; !ok {
		t.Error("AllowedPaths(folder:A) lost A/Foo.md after caller mutation")
	}

	slot, ok := view.Slots().Lookup("lesson-l01")
	if !ok {
		t.Fatal("Slots().Lookup(lesson-l01) = false, want true")
	}
	position := slot.Patterns[0].Slots["A"]
	position.Fills[0].JP = "mutated"
	slot.Patterns[0].Slots["A"] = position
	secondSlot, ok := view.Slots().Lookup("lesson-l01")
	if !ok || secondSlot.Patterns[0].Slots["A"].Fills[0].JP != "私" {
		t.Errorf("Slots().Lookup() after mutation = %+v, want original fill", secondSlot)
	}

	concept, ok := view.Concepts().Document(func(body string) string { return body }, "Concepts/go/Concept.md")
	if !ok {
		t.Fatal("Concepts().Document() = false, want true")
	}
	concept.Title = "mutated"
	secondConcept, ok := view.Concepts().Document(func(body string) string { return body }, "Concepts/go/Concept.md")
	if !ok || concept.Title != "mutated" || secondConcept.Title != "Concept" {
		t.Errorf("Concepts().Document() after mutation = %+v, want original title", secondConcept)
	}
}

func TestCaptureBindsArtifactAuthorityAcrossOneRequest(t *testing.T) {
	t.Parallel()
	view, root := immutableViewFixture(t)
	requestA := view.Capture()

	contractPath := filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))
	if err := os.WriteFile(contractPath, []byte("changed\n"), 0o600); err != nil {
		t.Fatalf("changing contract source: %v", err)
	}
	requestB := view.Capture()

	if !requestA.ArtifactPolicy().Available() {
		t.Error("request-captured ArtifactPolicy changed while the response was in flight")
	}
	if _, err := requestA.Search().CountByStatus(); err != nil {
		t.Errorf("request-captured Search.CountByStatus() error = %v, want stable authority", err)
	}
	if results := snapshotSearch(t, requestA.Search(), "Concept"); len(results) != 1 || results[0].Status != "draft" {
		t.Errorf("request-captured Search(Concept) = %+v, want stable metadata badge", results)
	}
	if requestB.ArtifactPolicy().Available() {
		t.Error("next ArtifactPolicy capture remained available after its source changed")
	}
	if _, err := requestB.Search().CountByStatus(); !errors.Is(err, search.ErrMetadataUnavailable) {
		t.Errorf("next Search.CountByStatus() error = %v, want ErrMetadataUnavailable", err)
	}
	if results := snapshotSearch(t, requestB.Search(), "Concept"); len(results) != 1 || results[0].Status != "" {
		t.Errorf("next Search(Concept) = %+v, want lexical result without metadata badge", results)
	}
}

type recordingSource struct {
	Source

	scans int
	reads map[string]int
	fail  map[string]int
}

func (s *recordingSource) ScanAvailable(ctx context.Context) (vault.Scan, error) {
	s.scans++
	return s.Source.ScanAvailable(ctx)
}

func (s *recordingSource) ReadFile(ctx context.Context, entry vault.Entry) ([]byte, error) {
	path := entry.Path()
	s.reads[path]++
	if s.fail[path] > 0 {
		s.fail[path]--
		return nil, errors.New("injected read failure")
	}
	return s.Source.ReadFile(ctx, entry)
}

func TestNewScansOnceAndReadsOnlyGenerationInputs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeNote(t, root, "Concepts/Alpha.md", "---\ntitle: Alpha\ntype: concept\n---\nalpha\n")
	writeNote(t, root, "README.md", "home\n")
	writeNote(t, root, "System/slots/L01.yaml", `lesson: L01
slug: lesson-l01
title: Lesson
patterns:
  - id: p1
    template: "{A}"
    gloss_zh: "{A}"
    slots:
      A:
        fills:
          - {jp: 私, reading: わたし, zh: 我}
`)
	writeNote(t, root, "Diagrams/example.png", "not really an image")
	contract := testContract(t, root)
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeReader(t, reader) })
	source := &recordingSource{
		Source: reader,
		reads:  make(map[string]int),
		fail:   make(map[string]int),
	}
	store, err := New(t.Context(), source, discardLogger(), contract)
	if err != nil {
		t.Fatal(err)
	}

	if source.scans != 1 {
		t.Errorf("initial scans = %d, want 1", source.scans)
	}
	for _, path := range []string{"Concepts/Alpha.md", "README.md", "System/slots/L01.yaml"} {
		if source.reads[path] != 1 {
			t.Errorf("reads[%q] = %d, want 1", path, source.reads[path])
		}
	}
	for _, path := range []string{"Diagrams/example.png", schema.ContractRelPath} {
		if source.reads[path] != 0 {
			t.Errorf("non-generation reads[%q] = %d, want 0", path, source.reads[path])
		}
	}
	if _, ok := store.Current().Note("Concepts/Alpha.md"); !ok {
		t.Error("captured Alpha note is unavailable")
	}
	if store.Current().Slots().Len() != 1 {
		t.Errorf("captured slot indexes = %d, want 1", store.Current().Slots().Len())
	}
}

func TestRescanRetriesTransientReadWithoutMetadataChange(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	const path = "Concepts/Alpha.md"
	writeNote(t, root, path, "---\ntitle: Alpha\ntype: concept\n---\nalpha\n")
	contract := testContract(t, root)
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeReader(t, reader) })
	source := &recordingSource{
		Source: reader,
		reads:  make(map[string]int),
		fail:   map[string]int{path: 1},
	}
	store, err := New(t.Context(), source, discardLogger(), contract)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := store.Current().Note(path); ok {
		t.Fatal("initial generation retained a note whose read failed")
	}
	if !store.retry {
		t.Fatal("initial generation lost its retry signal")
	}

	store.rescan(t.Context())
	if _, ok := store.Current().Note(path); !ok {
		t.Fatal("retry did not recover the unchanged note")
	}
	if source.scans != 2 || source.reads[path] != 2 {
		t.Errorf("retry work = scans %d reads %d, want 2 and 2", source.scans, source.reads[path])
	}
	if store.retry {
		t.Fatal("successful retry left the store in retry state")
	}
}

func TestRescanRetainsLastCompleteGenerationAcrossTransientRead(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	const relPath = "Concepts/Alpha.md"
	writeNote(t, root, relPath, "---\ntitle: Alpha\ntype: concept\n---\nold body\n")
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeReader(t, reader) })
	source := &recordingSource{
		Source: reader,
		reads:  make(map[string]int),
		fail:   make(map[string]int),
	}
	store, err := New(t.Context(), source, discardLogger(), nil)
	if err != nil {
		t.Fatal(err)
	}
	complete := store.Current()

	writeNote(t, root, relPath, "---\ntitle: Alpha\ntype: concept\n---\nreplacement body with a different size\n")
	source.fail[relPath] = 1
	store.rescan(t.Context())
	if got := store.Current(); got != complete {
		t.Fatal("transient read published an incomplete generation instead of retaining the last complete one")
	}
	if !store.retry {
		t.Fatal("transient read did not retain a retry signal")
	}

	store.rescan(t.Context())
	if got := store.Current(); got == complete {
		t.Fatal("successful retry did not publish the replacement generation")
	}
	note, ok := store.Current().Note(relPath)
	if !ok || !strings.Contains(note.Body, "replacement body") {
		t.Fatalf("replacement generation note = %+v, present %t", note, ok)
	}
}

// TestNewBuildsSnapshot pins that New produces a coherent, non-nil generation:
// the core projections share one captured vault and are each usable.
func TestNewBuildsSnapshot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeNote(t, root, "Concepts/Alpha.md", "---\ntitle: Alpha\ntype: concept\n---\n\nalpha body about kafka\n")

	contract := testContract(t, root)
	store, _ := newTestStore(t, root, contract)
	snap := store.Current()
	if snap == nil {
		t.Fatal("Current() = nil after New")
	}
	if snap.Search().Len() == 0 {
		t.Error("snapshot search index is empty")
	}
	if got := snapshotSearch(t, snap.Search(), "kafka"); len(got) == 0 {
		t.Error("kafka not found in the freshly built snapshot")
	}
	if got := snap.Graph().Resolve("Alpha"); got.Kind != graph.Unique {
		t.Errorf("graph.Resolve(Alpha).Kind = %v, want Unique", got.Kind)
	}
	if got := snap.Navigation().KnowledgeNotes(); len(got) != 1 || got[0].Modified.IsZero() {
		t.Errorf("snapshot KnowledgeNotes = %+v, want the scanner-captured mtime published with Alpha", got)
	}
}

func TestViewBindsResolutionAndTransclusionsToOneGeneration(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeNote(t, root, "Host.md", "Host embeds ![[Target]].\n")
	writeNote(t, root, "Target.md", "old generation body\n")

	store, _ := newTestStore(t, root, nil)
	oldView := store.Current()
	host, ok := oldView.Note("Host.md")
	if !ok {
		t.Fatal("Host.md is absent from the initial generation")
	}
	if got := oldView.Render(host.Body).HTML; !strings.Contains(got, "old generation body") {
		t.Fatalf("initial render = %q, want captured target body", got)
	}

	writeNote(t, root, "Target.md", "replacement generation body with a different size\n")
	store.rescan(t.Context())
	newView := store.Current()
	if newView == oldView {
		t.Fatal("changed target did not publish a new generation")
	}
	if got := newView.Render(host.Body).HTML; !strings.Contains(got, "replacement generation body") {
		t.Errorf("new generation render = %q, want replacement target body", got)
	}
	if got := oldView.Render(host.Body).HTML; !strings.Contains(got, "old generation body") || strings.Contains(got, "replacement generation body") {
		t.Errorf("old generation render changed after publication: %q", got)
	}
}

// TestRescanSwapsOnlyOnFilesystemChange pins the filesystem half of the
// freshness contract without putting real I/O inside a synctest bubble.
func TestRescanSwapsOnlyOnFilesystemChange(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeNote(t, root, "Concepts/Alpha.md", "---\ntitle: Alpha\ntype: concept\n---\n\nalpha body\n")

	contract := testContract(t, root)
	store, _ := newTestStore(t, root, contract)
	first := store.Current()
	store.rescan(t.Context())
	if store.Current() != first {
		t.Error("rescan swapped the snapshot when the vault was unchanged")
	}
	if got := snapshotSearch(t, store.Current().Search(), "widgets"); len(got) != 0 {
		t.Fatalf("widgets matched %d before the note existed", len(got))
	}

	const betaRel = "Concepts/Beta.md"
	writeNote(t, root, betaRel, "---\ntitle: Beta\ntype: concept\n---\n\nbeta mentions widgets\n")
	betaModified := time.Date(2026, time.July, 10, 8, 45, 0, 0, time.UTC)
	betaPath := filepath.Join(root, filepath.FromSlash(betaRel))
	if err := os.Chtimes(betaPath, betaModified, betaModified); err != nil {
		t.Fatalf("set Beta mtime: %v", err)
	}
	store.rescan(t.Context())

	if store.Current() == first {
		t.Error("rescan retained the snapshot after the vault changed")
	}
	if got := snapshotSearch(t, store.Current().Search(), "widgets"); len(got) == 0 {
		t.Error("rescan did not pick up the added note")
	}
	foundBeta := false
	for _, note := range store.Current().Navigation().KnowledgeNotes() {
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
	contract := testContract(t, root)
	store, _ := newTestStore(t, root, contract)

	first := store.Current()
	if len(first.Navigation().Paths()) != 1 || first.Navigation().ArtifactDiagnostic() != "" || first.Navigation().NavigationDiagnostic() != "" {
		t.Fatalf("initial navigation = %+v, want one available path", first.Navigation())
	}
	if !first.ArtifactPolicy().IsNonInstance("System/templates/Card.md") {
		t.Fatal("initial snapshot artifact policy does not classify the template")
	}
	assertSearchArtifactPolicy(t, first)

	// A changed contract cannot be hot-reloaded, but retaining artifact
	// eligibility from stale source bytes would be false authority. The process
	// therefore latches that capability unavailable until restart.
	writeNote(t, root, "System/schemas/vault-schema.toml", "not = [valid toml")
	store.rescan(t.Context())

	got := store.Current()
	if got == first {
		t.Fatal("rescan did not publish a new snapshot after the contract file appeared")
	}
	if len(got.Navigation().Paths()) != 0 || got.Navigation().ArtifactDiagnostic() == "" || got.Navigation().NavigationDiagnostic() != "" {
		t.Errorf("rescanned navigation = %+v, want artifact-dependent projection unavailable", got.Navigation())
	}
	if got.ArtifactPolicy().Available() || got.ArtifactPolicy().IsNonInstance("System/templates/Card.md") {
		t.Error("rescanned snapshot retained source-stale artifact authority")
	}
	if result := snapshotSearch(t, got.Search(), "Card"); len(result) != 1 || result[0].RelPath != "System/templates/Card.md" {
		t.Errorf("plain lexical search after artifact drift = %+v, want locally readable template", result)
	}
	if _, err := got.Search().Search(search.Parse("status:ready")); err == nil {
		t.Error("metadata search succeeded under source-stale artifact policy")
	}
}

func TestNewDoesNotFabricateInstanceCapabilities(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeNote(t, root, "Maps/Path.md", "---\ntitle: Path\ntype: study-path\n---\n## Course\n- [[Ghost]]\n")
	store, _ := newTestStore(t, root, nil)
	snap := store.Current()

	if snap.ArtifactPolicy().Available() {
		t.Fatal("Snapshot.ArtifactPolicy().Available() = true, want unavailable zero capability")
	}
	if snap.Navigation().NavigationDiagnostic() == "" || snap.Navigation().ArtifactDiagnostic() == "" {
		t.Errorf("snapshot diagnostics = navigation %q artifact %q, want both named", snap.Navigation().NavigationDiagnostic(), snap.Navigation().ArtifactDiagnostic())
	}
	if len(snap.Navigation().Paths()) != 0 || len(snap.Navigation().Maps()) != 0 || len(snap.Navigation().KnowledgeNotes()) != 0 {
		t.Errorf("snapshot instance projections = paths %d maps %d notes %d, want unavailable", len(snap.Navigation().Paths()), len(snap.Navigation().Maps()), len(snap.Navigation().KnowledgeNotes()))
	}
	if len(snap.Navigation().Folders()) == 0 {
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

	contract := testContract(t, root)
	store, reader := newTestStore(t, root, contract)
	ctx, cancel := context.WithCancel(t.Context())

	var readers sync.WaitGroup
	for range 8 {
		readers.Go(func() {
			for ctx.Err() == nil {
				snap := store.Current()
				_ = snap.Search().Len()
				_ = snap.Graph().Resolve("Alpha")
			}
		})
	}

	var swapper sync.WaitGroup
	swapper.Go(func() {
		for range 100 {
			scan, err := reader.ScanAvailable(t.Context())
			if err != nil {
				t.Errorf("ScanAvailable: %v", err)
				return
			}
			view, _, err := buildView(
				t.Context(),
				reader,
				scan,
				discardLogger(),
				contract.NavigationRoles(),
				contract.ArtifactPolicy(),
				contract.ArticleLanguage(),
			)
			if err != nil {
				t.Errorf("buildView: %v", err)
				return
			}
			store.ptr.Store(view)
		}
	})
	swapper.Wait()

	cancel()
	readers.Wait()
}
