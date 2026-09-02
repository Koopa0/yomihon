package snapshot

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/search"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
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
	store, err := New(tb.Context(), reader, discardLogger(), contract, contract.Governance())
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

	concept, ok := view.Concepts().Document(func(_, body string) string { return body }, "Concepts/go/Concept.md")
	if !ok {
		t.Fatal("Concepts().Document() = false, want true")
	}
	concept.Title = "mutated"
	secondConcept, ok := view.Concepts().Document(func(_, body string) string { return body }, "Concepts/go/Concept.md")
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
	// Notes are answered before files, and the contract is a vault file whose
	// characters a reader can open, so it joins the text corpus and can share a
	// query with this note. The claim under test is the note's badge, so the
	// note is named rather than assumed to be alone.
	if results := snapshotSearch(t, requestA.Search(), "Concept"); len(results) == 0 ||
		results[0].RelPath != "Concepts/go/Concept.md" || results[0].Status != "draft" {
		t.Errorf("request-captured Search(Concept) = %+v, want stable metadata badge", results)
	}
	if requestB.ArtifactPolicy().Available() {
		t.Error("next ArtifactPolicy capture remained available after its source changed")
	}
	if _, err := requestB.Search().CountByStatus(); !errors.Is(err, search.ErrMetadataUnavailable) {
		t.Errorf("next Search.CountByStatus() error = %v, want ErrMetadataUnavailable", err)
	}
	if results := snapshotSearch(t, requestB.Search(), "Concept"); len(results) == 0 ||
		results[0].RelPath != "Concepts/go/Concept.md" || results[0].Status != "" {
		t.Errorf("next Search(Concept) = %+v, want lexical result without metadata badge", results)
	}
	// A page combines two samples of the same authority: the policy it asks for
	// classification, and the index it asks for counts. Capture binds both to
	// one authority, which is why a reader can trust them together. Were they
	// ever to disagree, a page could show a lifecycle tally beside a policy that
	// had already stopped classifying — so the agreement is asserted here rather
	// than left to be inferred from the two halves above.
	for _, tt := range []struct {
		name string
		view *View
	}{
		{name: "captured before the source changed", view: requestA},
		{name: "captured after the source changed", view: requestB},
	} {
		_, countErr := tt.view.Search().CountByStatus()
		if got, want := tt.view.ArtifactPolicy().Available(), countErr == nil; got != want {
			t.Errorf("%s: ArtifactPolicy().Available() = %t but the bound index answers counts = %t; "+
				"one request would combine two disagreeing authorities", tt.name, got, want)
		}
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
	store, err := New(t.Context(), source, discardLogger(), contract, contract.Governance())
	if err != nil {
		t.Fatal(err)
	}

	if source.scans != 1 {
		t.Errorf("initial scans = %d, want 1", source.scans)
	}
	// The contract is on this list because a generation now reads it twice over
	// in two different roles — the schema loader takes it as authority, and the
	// text index takes it as one more vault file a reader can open — and each
	// of those roles gets its own read of its own. What this asserts is that the
	// generation reads it once, like everything else it reads.
	for _, path := range []string{
		"Concepts/Alpha.md",
		"README.md",
		"System/slots/L01.yaml",
		schema.ContractRelPath,
	} {
		if source.reads[path] != 1 {
			t.Errorf("reads[%q] = %d, want 1", path, source.reads[path])
		}
	}
	// A picture is settled by its name before any byte is touched: nothing on
	// the page will show its characters, so reading them would buy nothing.
	if got := source.reads["Diagrams/example.png"]; got != 0 {
		t.Errorf("non-generation reads[%q] = %d, want 0", "Diagrams/example.png", got)
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
	store, err := New(t.Context(), source, discardLogger(), contract, contract.Governance())
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
	store, err := New(t.Context(), source, discardLogger(), nil, schema.Ungoverned())
	if err != nil {
		t.Fatal(err)
	}
	// The loop's clock is pinned so the test can stand at the next tick: after
	// a failed rebuild the next expensive attempt waits one scan interval, and
	// real time does not pass between two direct rescan calls.
	clock := time.Now()
	store.now = func() time.Time { return clock }
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

	clock = clock.Add(scanInterval)
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
	if got := oldView.Render("Host.md", host.Body, wording.ZhHant).HTML; !strings.Contains(got, "old generation body") {
		t.Fatalf("initial render = %q, want captured target body", got)
	}

	writeNote(t, root, "Target.md", "replacement generation body with a different size\n")
	store.rescan(t.Context())
	newView := store.Current()
	if newView == oldView {
		t.Fatal("changed target did not publish a new generation")
	}
	if got := newView.Render("Host.md", host.Body, wording.ZhHant).HTML; !strings.Contains(got, "replacement generation body") {
		t.Errorf("new generation render = %q, want replacement target body", got)
	}
	if got := oldView.Render("Host.md", host.Body, wording.ZhHant).HTML; !strings.Contains(got, "old generation body") || strings.Contains(got, "replacement generation body") {
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
	writeNote(t, root, "Maps/Path.md", "---\ntitle: Path\ntype: study-path\n---\n## Course {sequence=primary}\n- [[Alpha]]\n")
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

// TestNewDoesNotFabricateInstanceCapabilities pins what a folder with no
// contract gets: no held artifact authority, so no note is classified as a
// governed instance and no type is a study path — but the projections are open
// and silent, because nothing was ever declared and empty is the true answer
// over an empty declared set.
func TestNewDoesNotFabricateInstanceCapabilities(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeNote(t, root, "Maps/Path.md", "---\ntitle: Path\ntype: study-path\n---\n## Course {sequence=primary}\n- [[Ghost]]\n")
	store, _ := newTestStore(t, root, nil)
	snap := store.Current()

	if snap.ArtifactPolicy().Available() {
		t.Fatal("Snapshot.ArtifactPolicy().Available() = true, want no held declaration")
	}
	if snap.Navigation().NavigationDiagnostic() != "" || snap.Navigation().ArtifactDiagnostic() != "" {
		t.Errorf("snapshot diagnostics = navigation %q artifact %q, want both silent for a folder that claimed nothing", snap.Navigation().NavigationDiagnostic(), snap.Navigation().ArtifactDiagnostic())
	}
	if snap.Navigation().InstanceProjectionsClosed() {
		t.Error("instance projections closed for a folder that never claimed governance")
	}
	if len(snap.Navigation().Paths()) != 0 || len(snap.Navigation().Maps()) != 0 {
		t.Errorf("snapshot navigation = paths %d maps %d, want none: no type was declared as either", len(snap.Navigation().Paths()), len(snap.Navigation().Maps()))
	}
	// Nothing excluded this note, so it is an ordinary readable one.
	if len(snap.Navigation().KnowledgeNotes()) != 1 {
		t.Errorf("snapshot KnowledgeNotes = %d, want 1", len(snap.Navigation().KnowledgeNotes()))
	}
	if len(snap.Navigation().Folders()) == 0 {
		t.Error("snapshot folder navigation is empty, want non-instance-independent browsing")
	}
}

// TestNewClosesEveryProjectionForAnUnreadableContract is the companion: the
// folder claimed governance and then could not deliver it, so the
// contract-derived projections that stay open above must close here — with a
// diagnostic, so the fault is visible. The recent-notes summary is plain
// reading and survives, over everything readable: a folder whose contract
// broke must not show less than one that never had a contract.
func TestNewClosesEveryProjectionForAnUnreadableContract(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeNote(t, root, "Maps/Path.md", "---\ntitle: Path\ntype: study-path\n---\n## Course {sequence=primary}\n- [[Ghost]]\n")
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Errorf("Reader.Close: %v", closeErr)
		}
	})
	store, err := New(
		t.Context(),
		reader,
		discardLogger(),
		nil,
		schema.Unreadable(errors.New("toml: line 42: expected a key separator")),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	snap := store.Current()

	if !snap.Navigation().InstanceProjectionsClosed() {
		t.Error("instance projections stayed open under a contract that could not be read")
	}
	if got := len(snap.Navigation().KnowledgeNotes()); got != 1 {
		t.Errorf("KnowledgeNotes = %d, want 1: plain reading survives an unreadable contract", got)
	}
	if snap.Navigation().KnowledgeScoped() {
		t.Error("KnowledgeScoped() = true under an unreadable contract; the layer is that contract's own claim")
	}
	if len(snap.Navigation().Paths()) != 0 {
		t.Error("study paths were projected from a contract that could not be read")
	}
	if len(snap.Navigation().Folders()) == 0 {
		t.Error("ordinary folder browsing closed with the contract")
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
				nil,
				scan,
				discardLogger(),
				contract.NavigationRoles(),
				contract.KnowledgeScope(),
				contract.ArtifactPolicy(),
				contract.ArticleLanguage(),
				contract,
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

// TestBuildViewIndexesTextFilesAndSkipsTheRest pins which vault files reach the
// text corpus and, just as much, which do not. The rule is the file page's own:
// if yomihon shows it to you as characters, you can find it. Each exclusion is
// decided by a different thing — a picture by its name, a blob by its bytes, an
// oversized file by its size — so each is listed separately, and a fix that
// widens one must not quietly widen the others.
func TestBuildViewIndexesTextFilesAndSkipsTheRest(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeNote(t, root, "Concepts/Note.md", "---\ntitle: Note\ntype: concept\n---\n\nfindable body\n")
	writeNote(t, root, "Notes/todo.txt", "findable body\n")
	writeNote(t, root, "Notes/page.html", "<p>findable body</p>\n")
	// Its bytes are characters and its page is a picture: a hit inside one would
	// open a page where the matched word is nowhere on the screen.
	writeNote(t, root, "Diagrams/drawing.svg", `<svg xmlns="http://www.w3.org/2000/svg"><text>findable body</text></svg>`)
	writeNote(t, root, "Notes/blob.bin", "findable\x00body")
	writeNote(t, root, "Notes/huge.txt", strings.Repeat("findable body ", (render.MaxSourceBytes/14)+1))

	contract := testContract(t, root)
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeReader(t, reader) })
	store, err := New(t.Context(), reader, discardLogger(), contract, contract.Governance())
	if err != nil {
		t.Fatal(err)
	}

	got := make([]string, 0, 3)
	for _, result := range snapshotSearch(t, store.Current().Search(), "findable body") {
		got = append(got, result.RelPath)
	}
	want := []string{"Concepts/Note.md", "Notes/page.html", "Notes/todo.txt"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("indexed entries mismatch (-want +got):\n%s", diff)
	}
}

// TestBuildViewStillResolvesWikilinksToFiles guards the hole the widening most
// plausibly opens. Every vault file has to reach the link resolver whether or
// not its bytes are read, so a note pointing at a picture keeps resolving.
func TestBuildViewStillResolvesWikilinksToFiles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeNote(t, root, "Concepts/Note.md", "---\ntitle: Note\ntype: concept\n---\n\nsee [[drawing.svg]]\n")
	writeNote(t, root, "Diagrams/drawing.svg", `<svg xmlns="http://www.w3.org/2000/svg"></svg>`)
	contract := testContract(t, root)
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeReader(t, reader) })
	store, err := New(t.Context(), reader, discardLogger(), contract, contract.Governance())
	if err != nil {
		t.Fatal(err)
	}

	note, ok := store.Current().Note("Concepts/Note.md")
	if !ok {
		t.Fatal("the linking note is absent from the generation")
	}
	result := store.Current().Render("Concepts/Note.md", note.Body, wording.ZhHant)
	if !strings.Contains(result.HTML, "/notes/Diagrams/drawing.svg") {
		t.Errorf("a wikilink to a file no longer resolves; html = %q", result.HTML)
	}
	if len(result.Diagnostics) != 0 {
		t.Errorf("resolving a link to a file produced diagnostics: %+v", result.Diagnostics)
	}
}

// TestOneUnreadableOrdinaryFileDoesNotFreezeTheFolder covers the difference
// between a file the reading surface is made of and a file that only lends the
// index its words.
//
// A note that cannot be read leaves the surface incomplete, so the generation is
// held back until it can be. Retry also means "ignore that nothing changed", so
// a file that is permanently unreadable — a mode-000 export, a socket, something
// another process holds — would hold the whole folder at the generation before
// it, rebuilding every couple of seconds and never publishing again. Every note
// written after it would be a 404 in a folder that is otherwise fine.
func TestOneUnreadableOrdinaryFileDoesNotFreezeTheFolder(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeNote(t, root, "Concepts/Alpha.md", "---\ntitle: Alpha\ntype: concept\n---\nalpha\n")
	writeNote(t, root, "notes.txt", "plain text\n")
	contract := testContract(t, root)
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeReader(t, reader) })
	// A read that never succeeds, on a file that is not a note.
	source := &recordingSource{
		Source: reader,
		reads:  make(map[string]int),
		fail:   map[string]int{"notes.txt": 1 << 30},
	}
	store, err := New(t.Context(), source, discardLogger(), contract, contract.Governance())
	if err != nil {
		t.Fatal(err)
	}
	if store.retry {
		t.Fatal("an unreadable ordinary file held the generation back; every later change would stop being published")
	}

	const added = "Concepts/Beta.md"
	writeNote(t, root, added, "---\ntitle: Beta\ntype: concept\n---\nbeta\n")
	store.rescan(t.Context())
	if _, ok := store.Current().Note(added); !ok {
		t.Error("a note written after the unreadable file never reached a published generation")
	}
}

// TestPermanentReadFailureBoundsRebuildWork pins the cost of a read that never
// succeeds. Every rescan tick keeps its cheap metadata scan, but the expensive
// full rebuild — re-reading every file and rebuilding every projection — backs
// off exponentially while the vault's metadata holds still, and any
// metadata-visible change retries immediately. Without the bound, one
// permanently unreadable note re-reads the whole folder every two seconds,
// forever, with stderr as the only evidence.
//
// The schedule is the same whether or not an attempt publishes: past the
// degrade threshold the folder publishes what each of these same attempts
// could read, and that neither adds an attempt nor moves one.
func TestPermanentReadFailureBoundsRebuildWork(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	const failing = "Concepts/Alpha.md"
	writeNote(t, root, failing, "---\ntitle: Alpha\ntype: concept\n---\nalpha\n")
	writeNote(t, root, "Concepts/Base.md", "---\ntitle: Base\ntype: concept\n---\nbase\n")
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
	store, err := New(t.Context(), source, discardLogger(), contract, contract.Governance())
	if err != nil {
		t.Fatal(err)
	}
	complete := store.Current()
	base := time.Now()
	clock := base
	store.now = func() time.Time { return clock }

	// The note is edited — its size changes, so the next rescan rebuilds — and
	// from now on every read of it fails.
	writeNote(t, root, failing, "---\ntitle: Alpha\ntype: concept\n---\nalpha, now a different size\n")
	source.fail[failing] = 1 << 30

	// One rescan per scan interval, as the ticker would drive it. The cheap
	// metadata scan runs on every tick; the expensive rebuild follows the
	// doubling schedule, capped at one minute.
	var attemptTicks []int
	last := source.reads[failing]
	for tick := range 62 {
		clock = base.Add(time.Duration(tick) * scanInterval)
		store.rescan(t.Context())
		if source.reads[failing] > last {
			attemptTicks = append(attemptTicks, tick)
			last = source.reads[failing]
		}
	}
	// Ticks are two seconds apart: attempts at 0s, 2s, 6s, 14s, 30s, 62s and
	// then the capped interval, 122s. Unbounded retry attempts on all 62.
	if diff := cmp.Diff([]int{0, 1, 3, 7, 15, 31, 61}, attemptTicks); diff != "" {
		t.Errorf("rebuild attempt ticks mismatch (-want +got):\n%s", diff)
	}
	if store.Current() == complete {
		t.Error("the folder never left its last whole generation, so every change after the failure stayed invisible")
	}
	if fresh := store.Current().Freshness(); fresh.Complete {
		t.Errorf("the published generation calls itself whole: %+v", fresh)
	}

	// A metadata-visible change — here the fixed permission the operator just
	// made — retries immediately rather than waiting out the delay.
	if err := os.Chmod(filepath.Join(root, filepath.FromSlash(failing)), 0o400); err != nil {
		t.Fatalf("Chmod(%s): %v", failing, err)
	}
	before := source.reads[failing]
	store.rescan(t.Context())
	if got := source.reads[failing] - before; got != 1 {
		t.Errorf("rebuild attempts after a metadata change = %d, want an immediate 1", got)
	}
	// The change also restarts the schedule: the next attempt after this new
	// failure waits one interval again rather than the capped minute.
	clock = clock.Add(scanInterval)
	before = source.reads[failing]
	store.rescan(t.Context())
	if got := source.reads[failing] - before; got != 1 {
		t.Errorf("rebuild attempts one interval after the restarted schedule = %d, want 1", got)
	}
}

// TestUnreadableNoteRetainsGenerationUntilTheDegradeThreshold locks the
// behavior the comment above TestOneUnreadableOrdinaryFileDoesNotFreezeTheFolder
// describes for the opposite branch: a note the reading surface is made of.
// While one note cannot be read, no later generation publishes — the folder
// holds at the last whole one, and a note written meanwhile stays invisible —
// because a read that fails once and works on the next attempt must never
// publish half a folder. That retention is bounded: degradeAfter attempts in a
// row is where it ends, and the companion test states what happens then. The
// retry work stays on the backoff schedule throughout.
func TestUnreadableNoteRetainsGenerationUntilTheDegradeThreshold(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	const blocked = "Concepts/Alpha.md"
	writeNote(t, root, blocked, "---\ntitle: Alpha\ntype: concept\n---\nalpha\n")
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
	store, err := New(t.Context(), source, discardLogger(), contract, contract.Governance())
	if err != nil {
		t.Fatal(err)
	}
	frozen := store.Current()
	base := time.Now()
	clock := base
	store.now = func() time.Time { return clock }

	// The note is edited to a different size — so the next rescan rebuilds —
	// and from now on every read of it fails. A second note is written
	// alongside it, so what the retention costs the reader is visible.
	writeNote(t, root, blocked, "---\ntitle: Alpha\ntype: concept\n---\nalpha, edited to a different size\n")
	source.fail[blocked] = 1 << 30
	const added = "Concepts/Beta.md"
	writeNote(t, root, added, "---\ntitle: Beta\ntype: concept\n---\nbeta\n")

	// Under the backoff the attempts land on ticks 0, 1, 3, and 7, so ticks 0
	// through 2 carry the first two failures — one short of the threshold.
	var attemptTicks []int
	last := source.reads[blocked]
	rescanTicks := func(from, to int) {
		for tick := from; tick < to; tick++ {
			clock = base.Add(time.Duration(tick) * scanInterval)
			store.rescan(t.Context())
			if source.reads[blocked] > last {
				attemptTicks = append(attemptTicks, tick)
				last = source.reads[blocked]
			}
		}
	}
	rescanTicks(0, 3)
	if store.Current() != frozen {
		t.Fatal("a failure that has not yet outlasted the threshold published half a folder")
	}
	if _, ok := store.Current().Note(added); ok {
		t.Error("a note written during a failure that may still clear appeared before the folder degraded")
	}

	// The third attempt, on tick 3, is where the retention ends.
	rescanTicks(3, 10)
	if store.Current() == frozen {
		t.Fatal("the retention never ended; every change after the failure stayed invisible")
	}
	if diff := cmp.Diff([]int{0, 1, 3, 7}, attemptTicks); diff != "" {
		t.Errorf("rebuild attempt ticks mismatch (-want +got):\n%s", diff)
	}
}

// TestDegradedGenerationPublishesWhatCouldBeRead pins where retention stops.
// Holding the last whole generation is the right answer for a read that fails
// once and works on the next attempt. Held without a bound it means every note
// written after one file stopped being readable is answered with a 404, in a
// folder that is otherwise entirely fine, with nothing on the page a reader
// could act on. So after degradeAfter attempts in a row come back incomplete,
// the folder publishes what it could read, carries the last copy of what it
// could not, and reports itself as not whole for as long as that lasts.
func TestDegradedGenerationPublishesWhatCouldBeRead(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	const unreadable = "Concepts/Alpha.md"
	writeNote(t, root, unreadable, "---\ntitle: Alpha\ntype: concept\n---\nalpha as first read\n")
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
	store, err := New(t.Context(), source, discardLogger(), contract, contract.Governance())
	if err != nil {
		t.Fatal(err)
	}
	whole := store.Current()
	wholeAt := whole.Freshness().BuiltAt
	base := time.Now()
	clock := base
	store.now = func() time.Time { return clock }

	// The note is edited to a different size — so the next rescan rebuilds —
	// and from now on every read of it fails. A second note is written while
	// the folder cannot be read whole: this is the reader's own new work, and
	// it is what the old policy suppressed for as long as the failure lasted.
	writeNote(t, root, unreadable, "---\ntitle: Alpha\ntype: concept\n---\nalpha, edited and now unreadable\n")
	source.fail[unreadable] = 1 << 30
	const added = "Concepts/Beta.md"
	writeNote(t, root, added, "---\ntitle: Beta\ntype: concept\n---\nbeta\n")

	// One rescan per scan interval, as the ticker drives it. The backoff puts
	// the first three attempts on ticks 0, 1, and 3 — about six seconds.
	for tick := range 4 {
		clock = base.Add(time.Duration(tick) * scanInterval)
		store.rescan(t.Context())
	}

	degraded := store.Current()
	if degraded == whole {
		t.Fatal("a read failure that never clears held the folder at its last whole generation forever")
	}
	if _, ok := degraded.Note(added); !ok {
		t.Error("a note written while one file was unreadable never reached a published generation")
	}
	fresh := degraded.Freshness()
	if fresh.Complete {
		t.Error("a generation published without one of its sources calls itself complete")
	}
	if len(fresh.Blocked) != 1 || fresh.Blocked[0].Path != unreadable {
		t.Errorf("Blocked = %+v, want the one source that could not be read", fresh.Blocked)
	}
	if !fresh.LastComplete.Equal(wholeAt) {
		t.Errorf("LastComplete = %v, want the last whole read at %v", fresh.LastComplete, wholeAt)
	}
	carried, ok := degraded.Note(unreadable)
	if !ok {
		t.Fatal("the degraded generation dropped the note it could not re-read instead of carrying its last copy")
	}
	if !strings.Contains(carried.Body, "alpha as first read") {
		t.Errorf("carried body = %q, want the copy the last whole generation read", carried.Body)
	}
	if !carried.Stale {
		t.Error("the carried copy is not marked as one that could not be re-read")
	}
	// The carried copy replaces the resolver stub rather than joining it: two
	// registrations of one path would report the folder as having two files
	// answering to the same name.
	if collisions := degraded.Health().Collisions; len(collisions) != 0 {
		t.Errorf("degraded generation reports name collisions %+v; the carried copy was registered twice", collisions)
	}

	// Recovery is the ordinary path: the file reads again, the next attempt
	// publishes a whole generation, and every degraded fact clears.
	source.fail[unreadable] = 0
	clock = base.Add(20 * scanInterval)
	store.rescan(t.Context())
	recovered := store.Current()
	if recovered == degraded {
		t.Fatal("a folder that reads whole again did not publish a new generation")
	}
	fresh = recovered.Freshness()
	if !fresh.Complete || len(fresh.Blocked) != 0 {
		t.Errorf("recovered freshness = %+v, want whole with nothing blocked", fresh)
	}
	note, ok := recovered.Note(unreadable)
	if !ok || note.Stale || !strings.Contains(note.Body, "edited and now unreadable") {
		t.Errorf("recovered note = %+v, present %t; want the edited body read afresh", note, ok)
	}
	if _, ok := recovered.Note(added); !ok {
		t.Error("the recovered generation lost the note written while the folder was degraded")
	}
}

// TestDegradedGenerationNamesEverySourceItCouldNotRead separates the two kinds
// of unreadable file a degraded generation holds, because the reading pages
// answer for them differently.
//
// A file the folder read once keeps that copy, so its page still has words to
// show and says they are the last known ones. A file that appeared and was
// never readable has no copy to show: it stays out of the generation's notes
// while remaining one of its files, which is what lets the missing-note page
// tell the reader the file is there and could not be read this time, rather
// than that there is nothing at that address.
//
// The freshness record names both. Naming only the first sends the reader to
// clear one permission and leaves the folder degraded with no account of why.
func TestDegradedGenerationNamesEverySourceItCouldNotRead(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	const (
		carried = "Concepts/Carried.md"
		never   = "Concepts/Never.md"
	)
	writeNote(t, root, carried, "---\ntitle: Carried\ntype: concept\n---\nthe words read before the file shut\n")
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
	store, err := New(t.Context(), source, discardLogger(), contract, contract.Governance())
	if err != nil {
		t.Fatal(err)
	}
	whole := store.Current()
	base := time.Now()
	clock := base
	store.now = func() time.Time { return clock }

	// The note that was read is edited to a different size, so the next rescan
	// rebuilds; a second note arrives beside it. From here neither can be read.
	writeNote(t, root, carried, "---\ntitle: Carried\ntype: concept\n---\nwords written after the file shut\n")
	writeNote(t, root, never, "---\ntitle: Never\ntype: concept\n---\nnever read\n")
	source.fail[carried] = 1 << 30
	source.fail[never] = 1 << 30

	// One rescan per scan interval, as the ticker drives it. The backoff puts
	// the first three attempts on ticks 0, 1, and 3.
	for tick := range 4 {
		clock = base.Add(time.Duration(tick) * scanInterval)
		store.rescan(t.Context())
	}
	degraded := store.Current()
	if degraded == whole {
		t.Fatal("the folder never degraded, so nothing below is under test")
	}
	fresh := degraded.Freshness()
	if fresh.Complete {
		t.Error("a generation published without two of its sources calls itself complete")
	}
	blocked := make([]string, 0, len(fresh.Blocked))
	for _, source := range fresh.Blocked {
		blocked = append(blocked, source.Path)
	}
	slices.Sort(blocked)
	if diff := cmp.Diff([]string{carried, never}, blocked); diff != "" {
		t.Errorf("blocked sources mismatch (-want +got):\n%s", diff)
	}

	kept, ok := degraded.Note(carried)
	if !ok {
		t.Fatal("the degraded generation dropped the note it had read once instead of carrying its last copy")
	}
	if !kept.Stale || !strings.Contains(kept.Body, "the words read before the file shut") {
		t.Errorf("carried note = %+v, want the last copy read, marked as one that could not be re-read", kept)
	}
	if unread, ok := degraded.Note(never); ok {
		t.Errorf("a file the folder has never had a reading of was published with a body: %+v", unread)
	}
	if _, ok := degraded.Entry(never); !ok {
		t.Error("the generation does not hold the unread file's identity, so its page cannot tell " +
			"a reader to clear a permission apart from telling them nothing is there")
	}
}

// TestAFolderBeingWrittenInStillDegrades pins which of the two failure counts
// decides. The retry backoff restarts whenever the folder's files change,
// because a changed folder deserves an immediate attempt rather than the
// remains of the previous wait. Counting those restarts toward the degrade
// threshold as well would mean a folder somebody is writing in never reaches
// it — and that is precisely the folder where holding every new note back
// costs the most.
func TestAFolderBeingWrittenInStillDegrades(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	const unreadable = "Concepts/Alpha.md"
	writeNote(t, root, unreadable, "---\ntitle: Alpha\ntype: concept\n---\nalpha\n")
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
	store, err := New(t.Context(), source, discardLogger(), contract, contract.Governance())
	if err != nil {
		t.Fatal(err)
	}
	whole := store.Current()
	base := time.Now()
	clock := base
	store.now = func() time.Time { return clock }

	writeNote(t, root, unreadable, "---\ntitle: Alpha\ntype: concept\n---\nalpha, edited and now unreadable\n")
	source.fail[unreadable] = 1 << 30

	// A note arrives before every attempt, so the file domain differs each
	// time and the backoff restarts on each one.
	for tick := range degradeAfter {
		writeNote(t, root, "Concepts/Beta-"+strconv.Itoa(tick)+".md",
			"---\ntitle: Beta\ntype: concept\n---\nbeta\n")
		clock = base.Add(time.Duration(tick) * scanInterval)
		store.rescan(t.Context())
	}
	if store.Current() == whole {
		t.Fatal("a folder being written in never degraded, so none of the new notes were published")
	}
	last := "Concepts/Beta-" + strconv.Itoa(degradeAfter-1) + ".md"
	if _, ok := store.Current().Note(last); !ok {
		t.Errorf("the degraded generation is missing %s, the note written before the attempt that published it", last)
	}
}

// TestFreshnessReportsStartupIncompletenessAndRetainedStaleness pins the two
// degraded states the freshness record exists for. A startup view published
// with a source missing says so; and when later rebuilds fail, the retained —
// still published — generation reports the blocked sources and the running
// failure count through the same record, without its own build facts moving.
func TestFreshnessReportsStartupIncompletenessAndRetainedStaleness(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	const blocked = "Concepts/Alpha.md"
	writeNote(t, root, blocked, "---\ntitle: Alpha\ntype: concept\n---\nalpha\n")
	contract := testContract(t, root)
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeReader(t, reader) })
	source := &recordingSource{
		Source: reader,
		reads:  make(map[string]int),
		fail:   map[string]int{blocked: 1},
	}
	store, err := New(t.Context(), source, discardLogger(), contract, contract.Governance())
	if err != nil {
		t.Fatal(err)
	}

	fresh := store.Current().Freshness()
	if fresh.Complete {
		t.Error("a startup generation that lost a note reports itself complete")
	}
	if len(fresh.Blocked) != 1 || fresh.Blocked[0].Path != blocked {
		t.Errorf("startup Blocked = %+v, want the one lost note", fresh.Blocked)
	}
	if fresh.BuiltAt.IsZero() {
		t.Error("startup freshness carries no build time")
	}
	if fresh.FailedRetries != 0 {
		t.Errorf("startup FailedRetries = %d, want 0", fresh.FailedRetries)
	}

	// The read failure was transient, so the first retry publishes a complete
	// generation and the record follows it.
	store.rescan(t.Context())
	fresh = store.Current().Freshness()
	if !fresh.Complete || len(fresh.Blocked) != 0 || fresh.FailedRetries != 0 {
		t.Fatalf("recovered freshness = %+v, want complete with nothing blocked", fresh)
	}
	builtAt := fresh.BuiltAt

	// The note is edited — so the next rescan rebuilds — and now never reads.
	// The retained generation is the one being served, and it is the view
	// that must be able to say it has gone stale.
	clock := time.Now()
	store.now = func() time.Time { return clock }
	retained := store.Current()
	writeNote(t, root, blocked, "---\ntitle: Alpha\ntype: concept\n---\nalpha, edited to a different size\n")
	source.fail[blocked] = 1 << 30
	store.rescan(t.Context())
	if store.Current() != retained {
		t.Fatal("an incomplete rebuild published a generation")
	}
	fresh = retained.Freshness()
	if !fresh.Complete {
		t.Error("the retained generation stopped reporting its own complete build")
	}
	if len(fresh.Blocked) != 1 || fresh.Blocked[0].Path != blocked {
		t.Errorf("retained Blocked = %+v, want the unreadable note", fresh.Blocked)
	} else if !strings.Contains(fresh.Blocked[0].Reason, "injected read failure") {
		t.Errorf("Blocked reason = %q, want the read error", fresh.Blocked[0].Reason)
	}
	if fresh.FailedRetries != 1 {
		t.Errorf("FailedRetries after one failed rebuild = %d, want 1", fresh.FailedRetries)
	}
	if !fresh.BuiltAt.Equal(builtAt) {
		t.Errorf("failed attempts moved BuiltAt from %v to %v", builtAt, fresh.BuiltAt)
	}

	clock = clock.Add(scanInterval)
	store.rescan(t.Context())
	if got := retained.Freshness().FailedRetries; got != 2 {
		t.Errorf("FailedRetries after two failed rebuilds = %d, want 2", got)
	}
}

// TestSupersededGenerationKeepsItsOwnBuildFacts pins which way the freshness
// record is allowed to be wrong. A request captures a generation and renders
// its content; a rebuild can replace that generation while the response is
// still being written. The record read beside the content then has to describe
// the generation the reader is looking at — the one that never read a source
// stays incomplete and keeps naming it — because the alternative direction
// tells the reader that a folder they cannot see is whole and current.
func TestSupersededGenerationKeepsItsOwnBuildFacts(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	const blocked = "Concepts/Alpha.md"
	writeNote(t, root, blocked, "---\ntitle: Alpha\ntype: concept\n---\nalpha\n")
	contract := testContract(t, root)
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeReader(t, reader) })
	source := &recordingSource{
		Source: reader,
		reads:  make(map[string]int),
		fail:   map[string]int{blocked: 1},
	}
	store, err := New(t.Context(), source, discardLogger(), contract, contract.Governance())
	if err != nil {
		t.Fatal(err)
	}

	// The generation a request would be holding: published for availability at
	// startup without the note it could not read.
	superseded := store.Current()
	startup := superseded.Freshness()
	if startup.Complete || len(startup.Blocked) != 1 {
		t.Fatalf("startup freshness = %+v, want incomplete with the lost note", startup)
	}

	// The read failure was transient, so the next attempt publishes a complete
	// generation — while the earlier one is still being rendered.
	clock := store.now().Add(time.Hour)
	store.now = func() time.Time { return clock }
	store.rescan(t.Context())
	if store.Current() == superseded {
		t.Fatal("the recovering rebuild published no generation")
	}

	after := superseded.Freshness()
	if after.Complete {
		t.Error("a generation that never read a source calls itself complete once a later one did")
	}
	if !after.BuiltAt.Equal(startup.BuiltAt) {
		t.Errorf("BuiltAt moved from %v to %v: a later generation's build time was reported beside this one's content", startup.BuiltAt, after.BuiltAt)
	}
	if len(after.Blocked) != 1 || after.Blocked[0].Path != blocked {
		t.Errorf("Blocked = %+v, want the source this generation never read", after.Blocked)
	}
	if current := store.Current().Freshness(); !current.Complete || len(current.Blocked) != 0 {
		t.Errorf("published freshness = %+v, want the new generation reported complete and whole", current)
	}
}

// TestMetadataInvisibleEditIsEventuallyRepublished pins the slow half of the
// freshness contract. The fast path trusts identity and metadata, so an
// in-place edit that preserves inode, mode, size, and mtime is invisible to
// it — deliberately, and the companion scanner test states that boundary. What
// keeps that from meaning "stale forever in a quiescent folder" is the slow
// reconciliation cycle: every reconcileEvery-th tick rebuilds without the
// short-circuit, so the changed bytes are republished within minutes.
func TestMetadataInvisibleEditIsEventuallyRepublished(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	const relPath = "Concepts/Alpha.md"
	const oldBody = "---\ntitle: Alpha\ntype: concept\n---\nMARKER_OLD_XYZ\n"
	const newBody = "---\ntitle: Alpha\ntype: concept\n---\nMARKER_NEW_ABC\n"
	if len(oldBody) != len(newBody) {
		t.Fatalf("fixture bodies differ in size (%d vs %d); this would prove nothing", len(oldBody), len(newBody))
	}
	writeNote(t, root, relPath, oldBody)
	contract := testContract(t, root)
	store, _ := newTestStore(t, root, contract)
	first := store.Current()
	entry, ok := first.Entry(relPath)
	if !ok {
		t.Fatalf("Entry(%s) missing from the initial generation", relPath)
	}

	// Rewrite the bytes in place — same inode, same size — and put the mtime
	// back where the scanner observed it.
	full := filepath.Join(root, filepath.FromSlash(relPath))
	file, err := os.OpenFile(full, os.O_WRONLY, 0) // #nosec G304 -- a path inside this test's own TempDir
	if err != nil {
		t.Fatalf("OpenFile(%s): %v", relPath, err)
	}
	if _, err := file.WriteAt([]byte(newBody), 0); err != nil {
		t.Fatalf("WriteAt(%s): %v", relPath, err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close(%s): %v", relPath, err)
	}
	if err := os.Chtimes(full, entry.ModTime(), entry.ModTime()); err != nil {
		t.Fatalf("Chtimes(%s): %v", relPath, err)
	}

	// One ordinary rescan keeps the fast path honest: metadata is unchanged,
	// so nothing is re-read and the generation stands.
	store.rescan(t.Context())
	if store.Current() != first {
		t.Fatal("a metadata-invisible edit was picked up by the fast path; this test no longer covers the slow cycle")
	}

	// Driving the loop through one reconciliation period republishes the
	// changed bytes even though no metadata ever moved.
	for range reconcileEvery {
		store.rescan(t.Context())
	}
	note, ok := store.Current().Note(relPath)
	if !ok {
		t.Fatalf("Note(%s) missing after the reconciliation period", relPath)
	}
	if !strings.Contains(note.Body, "MARKER_NEW_ABC") {
		t.Errorf("note body after the reconciliation period still carries the old bytes: %q", note.Body)
	}
}

// TestReconciliationDefersToFailureBackoff pins how the two cadences meet:
// while rebuilds are failing, the reconciliation counter keeps counting but
// never forces the expensive path. The backoff owns the rebuild cadence
// there, and every retry attempt is already a full re-read, so a forced
// rebuild would only bypass the bound the backoff exists to hold.
func TestReconciliationDefersToFailureBackoff(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	const failing = "Concepts/Alpha.md"
	writeNote(t, root, failing, "---\ntitle: Alpha\ntype: concept\n---\nalpha\n")
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
	store, err := New(t.Context(), source, discardLogger(), contract, contract.Governance())
	if err != nil {
		t.Fatal(err)
	}
	// The clock never advances, so the backoff delay never expires.
	clock := time.Now()
	store.now = func() time.Time { return clock }

	writeNote(t, root, failing, "---\ntitle: Alpha\ntype: concept\n---\nalpha, now a different size\n")
	source.fail[failing] = 1 << 30
	store.rescan(t.Context())
	if !store.retry {
		t.Fatal("the failed rebuild did not latch a retry")
	}

	before := source.reads[failing]
	for range reconcileEvery + 10 {
		store.rescan(t.Context())
	}
	if got := source.reads[failing] - before; got != 0 {
		t.Errorf("reconciliation forced %d rebuild attempts past the failure backoff, want 0", got)
	}
}

// TestASidecarTooLargeToShowIsNotSearchable keeps one rule across both faces. A
// lesson sidecar is read whatever its size, because the practice panel is built
// from it — but its own page says its contents are not searched, and deciding
// again from the bytes alone made that sentence false.
func TestASidecarTooLargeToShowIsNotSearchable(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeNote(t, root, "Concepts/Alpha.md", "---\ntitle: Alpha\ntype: concept\n---\nalpha\n")
	oversize := "sentences:\n" + strings.Repeat("  - text: rarespelunker\n", (render.MaxSourceBytes/24)+64)
	writeNote(t, root, "System/slots/L01.yaml", oversize)
	contract := testContract(t, root)
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeReader(t, reader) })
	store, err := New(t.Context(), reader, discardLogger(), contract, contract.Governance())
	if err != nil {
		t.Fatal(err)
	}
	results, err := store.Current().Search().Search(search.Parse("rarespelunker"))
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	for _, r := range results {
		if strings.HasSuffix(r.RelPath, ".yaml") {
			t.Errorf("a sidecar past the size its page will show is in the index: %s", r.RelPath)
		}
	}
}

// TestAReadablePDFIsNotSearchable holds the index to the same branch set the
// file page uses. A PDF gets its own viewer rather than a source listing, so its
// bytes are never shown as characters — and a small enough PDF is often valid
// UTF-8 with no NUL, which is all a bytes-only test looks at. Indexing it hands
// back a hit whose page contains none of the words that matched.
func TestAReadablePDFIsNotSearchable(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeNote(t, root, "Concepts/Alpha.md", "---\ntitle: Alpha\ntype: concept\n---\nalpha\n")
	writeNote(t, root, "paper.pdf", "%PDF-1.4\n1 0 obj\n<< /Title (rarespelunker) >>\nendobj\n%%EOF\n")
	contract := testContract(t, root)
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeReader(t, reader) })
	store, err := New(t.Context(), reader, discardLogger(), contract, contract.Governance())
	if err != nil {
		t.Fatal(err)
	}
	results, err := store.Current().Search().Search(search.Parse("rarespelunker"))
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	for _, r := range results {
		if strings.HasSuffix(r.RelPath, ".pdf") {
			t.Errorf("a PDF is in the text index: %s — its page shows a viewer, not these words", r.RelPath)
		}
	}
}

// TestAnOversizeNoteRendersAndStaysOutOfTheIndex is the half of the bound that
// makes it honest. Every file in the folder stays readable whatever its size —
// so the note is captured and its body is there — but the index is where a note
// costs three copies of itself, and that is where the ceiling belongs. The
// reader is told on the note's own page; this is the fact that sentence is
// about.
func TestAnOversizeNoteRendersAndStaysOutOfTheIndex(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	const needle = "rarespelunker"
	writeNote(t, root, "small.md", "---\ntitle: Small\ntype: concept\n---\n"+needle+" sits here.\n")
	huge := "---\ntitle: Huge\ntype: concept\n---\n" + needle + " sits here too.\n" +
		strings.Repeat("padding padding padding\n", 60000)
	if len(huge) <= render.MaxSourceBytes {
		t.Fatalf("the oversize fixture is %d bytes, under the cap; this would prove nothing", len(huge))
	}
	writeNote(t, root, "huge.md", huge)
	contract := testContract(t, root)
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeReader(t, reader) })
	store, err := New(t.Context(), reader, discardLogger(), contract, contract.Governance())
	if err != nil {
		t.Fatal(err)
	}
	view := store.Current()

	// It is captured and readable.
	note, ok := view.Note("huge.md")
	if !ok {
		t.Fatal("the oversize note is absent from the generation; reading is never withheld")
	}
	if !strings.Contains(note.Body, "sits here too") {
		t.Error("the oversize note lost its body")
	}
	if note.Searchable {
		t.Error("the oversize note reports itself searchable, so its page would say nothing")
	}
	if small, _ := view.Note("small.md"); !small.Searchable {
		t.Error("a note under the cap reports itself unsearchable")
	}

	results, err := view.Search().Search(search.Parse(needle))
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	var paths []string
	for _, r := range results {
		paths = append(paths, r.RelPath)
	}
	if !slices.Contains(paths, "small.md") {
		t.Errorf("search lost the note under the cap; got %v", paths)
	}
	if slices.Contains(paths, "huge.md") {
		t.Errorf("the oversize note reached the index, so its page's sentence is untrue; got %v", paths)
	}
}
