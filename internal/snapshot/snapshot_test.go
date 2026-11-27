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
	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/lexical"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

func discardLogger() *slog.Logger { return slog.New(slog.DiscardHandler) }

func snapshotSearch(tb testing.TB, idx *lexical.Index, query string) []lexical.Result {
	tb.Helper()
	answer, err := idx.Search(lexical.Parse(query), -1)
	if err != nil {
		tb.Fatalf("Search(%q) error: %v", query, err)
	}
	return answer.Results
}

func assertSearchArtifactPolicy(tb testing.TB, snap *Generation) {
	tb.Helper()
	if got := snapshotSearch(tb, snap.Search(), "draft:false type:concept"); len(got) != 0 {
		tb.Errorf("metadata search returned non-instance template: %+v", got)
	}
	if got := snapshotSearch(tb, snap.Search(), "Card"); len(got) != 1 || got[0].RelPath != "System/templates/Card.md" {
		tb.Errorf("plain search = %+v, want readable template", got)
	}
	if got := snapshotSearch(tb, snap.Search(), "draft:true"); len(got) != 1 || got[0].RelPath != "Concepts/Alpha.md" {
		tb.Errorf("draft search = %+v, want the draft instance only", got)
	}
}

// draftSearch asks the index for its drafts, the metadata projection these
// assertions read: it answers only while the bound artifact policy does.
func draftSearch(idx *lexical.Index) (lexical.Answer, error) {
	return idx.Search(lexical.Parse("draft:true"), -1)
}

func writeNote(tb testing.TB, root, rel, content string) {
	tb.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		tb.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		tb.Fatalf("write %s: %v", rel, err)
	}
}

// writeContract writes the shared contract fixture into root, passed through
// edit when edit is not nil, and loads it.
func writeContract(tb testing.TB, root string, edit func(string) string) *schema.Contract {
	tb.Helper()
	data, err := os.ReadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		tb.Fatalf("read contract fixture: %v", err)
	}
	text := string(data)
	if edit != nil {
		text = edit(text)
	}
	contractPath := filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))
	if err = os.MkdirAll(filepath.Dir(contractPath), 0o750); err != nil {
		tb.Fatalf("mkdir contract fixture: %v", err)
	}
	if err = os.WriteFile(contractPath, []byte(text), 0o600); err != nil { // #nosec G703 -- every caller supplies a testing.T.TempDir root
		tb.Fatalf("write contract fixture: %v", err)
	}
	contract, err := schema.Load(root)
	if err != nil {
		tb.Fatalf("schema.Load: %v", err)
	}
	return contract
}

// testContract writes the shared contract fixture into root unedited.
func testContract(tb testing.TB, root string) *schema.Contract {
	tb.Helper()
	return writeContract(tb, root, nil)
}

// replacing is a contract edit that swaps the fixture's one occurrence of old
// for with, and fails the test when the fixture does not hold old exactly once.
func replacing(tb testing.TB, old, with string) func(string) string {
	tb.Helper()
	return func(text string) string {
		tb.Helper()
		if n := strings.Count(text, old); n != 1 {
			tb.Fatalf("the contract fixture holds %q %d times, want once", old, n)
		}
		return strings.Replace(text, old, with, 1)
	}
}

// testContractDeclaringPrivacy writes the shared contract fixture with an
// egress declaration withholding dirs, because the fixture itself declares
// none and a policy that was never claimed cannot go stale.
func testContractDeclaringPrivacy(tb testing.TB, root string, dirs ...string) *schema.Contract {
	tb.Helper()
	quoted := make([]string, len(dirs))
	for i, dir := range dirs {
		quoted[i] = strconv.Quote(dir)
	}
	return writeContract(tb, root, func(text string) string {
		return text + "\n[privacy]\nnever_egress_dirs = [" + strings.Join(quoted, ", ") + "]\n"
	})
}

// openReader opens root for one test and closes it when the test ends.
func openReader(tb testing.TB, root string) *vault.Reader {
	tb.Helper()
	reader, err := vault.Open(root)
	if err != nil {
		tb.Fatalf("vault.Open: %v", err)
	}
	tb.Cleanup(func() {
		if err := reader.Close(); err != nil {
			tb.Errorf("Reader.Close: %v", err)
		}
	})
	return reader
}

// buildStore builds the first generation of source's folder under contract.
func buildStore(tb testing.TB, source Source, log *slog.Logger, contract *schema.Contract) *Store {
	tb.Helper()
	store, err := New(tb.Context(), source, log, contract, contract.Governance())
	if err != nil {
		tb.Fatalf("New: %v", err)
	}
	return store
}

// newTestStore is a store over root, and the reader it reads through.
func newTestStore(tb testing.TB, root string, contract *schema.Contract) (*Store, *vault.Reader) {
	tb.Helper()
	reader := openReader(tb, root)
	return buildStore(tb, reader, discardLogger(), contract), reader
}

// recordingSource counts the scans and reads a store asks of the folder, and
// fails a path's next fail[path] reads.
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

// newRecordingSource opens root through a recordingSource.
func newRecordingSource(tb testing.TB, root string) *recordingSource {
	tb.Helper()
	return &recordingSource{Source: openReader(tb, root), reads: make(map[string]int), fail: make(map[string]int)}
}

// newRecordingStore is a store over root whose reads a test can count and fail.
func newRecordingStore(tb testing.TB, root string, contract *schema.Contract) (*Store, *recordingSource) {
	tb.Helper()
	source := newRecordingSource(tb, root)
	return buildStore(tb, source, discardLogger(), contract), source
}

// loggedLine is one record a store logged: its level, message, and each
// attribute as the text a person reading the log would see.
type loggedLine struct {
	Level   slog.Level
	Message string
	Attrs   map[string]string
}

// recordingHandler keeps every record it is handed, so a test reads what the
// store said rather than what a formatter made of it.
type recordingHandler struct {
	mu    *sync.Mutex
	lines *[]loggedLine
}

func newRecordingLogger() (log *slog.Logger, logged func() []loggedLine) {
	handler := recordingHandler{mu: &sync.Mutex{}, lines: new([]loggedLine)}
	return slog.New(handler), func() []loggedLine {
		handler.mu.Lock()
		defer handler.mu.Unlock()
		return slices.Clone(*handler.lines)
	}
}

func (recordingHandler) Enabled(context.Context, slog.Level) bool { return true }

//nolint:gocritic // hugeParam: slog.Handler fixes this signature
func (h recordingHandler) Handle(_ context.Context, record slog.Record) error {
	line := loggedLine{Level: record.Level, Message: record.Message, Attrs: make(map[string]string)}
	record.Attrs(func(attr slog.Attr) bool {
		line.Attrs[attr.Key] = attr.Value.String()
		return true
	})
	h.mu.Lock()
	defer h.mu.Unlock()
	*h.lines = append(*h.lines, line)
	return nil
}

func (h recordingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h recordingHandler) WithGroup(string) slog.Handler      { return h }

// linesWith are the logged lines carrying message.
func linesWith(lines []loggedLine, message string) []loggedLine {
	var out []loggedLine
	for _, line := range lines {
		if line.Message == message {
			out = append(out, line)
		}
	}
	return out
}

// TestAccessorsHandTheCallerItsOwnCopy holds the accessors that copy what a
// generation holds: one reader's writes must not reach the next reader's page.
func TestAccessorsHandTheCallerItsOwnCopy(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const faulty, unread = "Concepts/Bad.md", "Concepts/Unread.md"
	writeNote(t, root, faulty, "---\ntype: concept\ndraft: maybe\ncreated: 2026-06-01\n---\n\nbody\n")
	writeNote(t, root, unread, "---\ntype: concept\n---\nbody\n")
	contract := testContract(t, root)
	source := newRecordingSource(t, root)
	source.fail[unread] = 1
	gen := buildStore(t, source, discardLogger(), contract).Current()

	blocked := gen.Freshness().Blocked
	if len(blocked) != 1 {
		t.Fatalf("Freshness().Blocked = %+v, want the one unread note", blocked)
	}
	blocked[0].Path = "changed"
	if got := gen.Freshness().Blocked[0].Path; got != unread {
		t.Errorf("Freshness().Blocked[0].Path after a caller's write = %q, want %q", got, unread)
	}

	findings := gen.SchemaFindings(faulty)
	if len(findings) == 0 {
		t.Fatal("the fixture note draws no schema findings, so this test would prove nothing")
	}
	findings[0] = judge.Finding{RuleID: "changed"}
	if got := gen.SchemaFindings(faulty)[0].RuleID; got == "changed" {
		t.Error("SchemaFindings() handed out the generation's own slice")
	}
}

func TestCaptureBindsArtifactAuthorityAcrossOneRequest(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const concept = "Concepts/go/Concept.md"
	writeNote(t, root, concept, "---\ntype: concept\ndraft: true\n---\nconcept body\n")
	store, _ := newTestStore(t, root, testContract(t, root))
	gen := store.Current()
	requestA := gen.Capture()

	contractPath := filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))
	if err := os.WriteFile(contractPath, []byte("changed\n"), 0o600); err != nil {
		t.Fatalf("changing contract source: %v", err)
	}
	requestB := gen.Capture()

	if !requestA.ArtifactPolicy().Available() {
		t.Error("request-captured ArtifactPolicy changed while the response was in flight")
	}
	if _, err := draftSearch(requestA.Search()); err != nil {
		t.Errorf("request-captured draft search error = %v, want stable authority", err)
	}
	if results := snapshotSearch(t, requestA.Search(), "Concept"); len(results) != 1 ||
		results[0].RelPath != concept || !results[0].Draft {
		t.Errorf("request-captured Search(Concept) = %+v, want stable metadata badge", results)
	}
	if requestB.ArtifactPolicy().Available() {
		t.Error("next ArtifactPolicy capture remained available after its source changed")
	}
	if _, err := draftSearch(requestB.Search()); !errors.Is(err, lexical.ErrMetadataUnavailable) {
		t.Errorf("next draft search error = %v, want ErrMetadataUnavailable", err)
	}
	if results := snapshotSearch(t, requestB.Search(), "Concept"); len(results) != 1 ||
		results[0].RelPath != concept || results[0].Draft {
		t.Errorf("next Search(Concept) = %+v, want lexical result without metadata badge", results)
	}
}

func TestNewScansOnceAndReadsOnlyGenerationInputs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeNote(t, root, "Concepts/Alpha.md", "---\ntype: concept\n---\nalpha\n")
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
	writeNote(t, root, "Notes/todo.txt", "a small text file\n")
	contract := testContract(t, root)
	store, source := newRecordingStore(t, root, contract)

	if source.scans != 1 {
		t.Errorf("initial scans = %d, want 1", source.scans)
	}
	for _, path := range []string{
		"Concepts/Alpha.md",
		"System/slots/L01.yaml",
	} {
		if source.reads[path] != 1 {
			t.Errorf("reads[%q] = %d, want 1", path, source.reads[path])
		}
	}
	// The contract arrives already loaded, and a small text file is neither a
	// note nor a sidecar nor a figure, so no projection has a use for its bytes.
	for _, path := range []string{schema.ContractRelPath, "Notes/todo.txt"} {
		if got := source.reads[path]; got != 0 {
			t.Errorf("non-generation reads[%q] = %d, want 0", path, got)
		}
	}
	// A picture is settled by its name before any byte is touched: nothing on
	// the page will show its characters, so reading them would buy nothing.
	if got := source.reads["Diagrams/example.png"]; got != 0 {
		t.Errorf("non-generation reads[%q] = %d, want 0", "Diagrams/example.png", got)
	}
	// folders.skipped_files leaves README.md out of the note map, so a generation
	// does not open it to index or project it.
	if got := source.reads["README.md"]; got != 0 {
		t.Errorf("skipped-basename reads[%q] = %d, want 0", "README.md", got)
	}
	if _, ok := store.Current().Note("README.md"); ok {
		t.Error("README.md was captured as a note; folders.skipped_files left it in the note map")
	}
	if !store.Current().SkipsNote("README.md") {
		t.Error("README.md was not recorded as a declared skip")
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
	writeNote(t, root, path, "---\ntype: concept\n---\nalpha\n")
	contract := testContract(t, root)
	source := newRecordingSource(t, root)
	source.fail[path] = 1
	store := buildStore(t, source, discardLogger(), contract)
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
	writeNote(t, root, relPath, "---\ntype: concept\n---\nold body\n")
	store, source := newRecordingStore(t, root, nil)
	// The loop's clock is pinned so the test can stand at the next tick: after
	// a failed rebuild the next expensive attempt waits one scan interval, and
	// real time does not pass between two direct rescan calls.
	clock := time.Now()
	store.now = func() time.Time { return clock }
	complete := store.Current()

	writeNote(t, root, relPath, "---\ntype: concept\n---\nreplacement body with a different size\n")
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
	writeNote(t, root, "Concepts/Alpha.md", "---\ntype: concept\n---\n\nalpha body about kafka\n")

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
	if got := snap.Graph().Resolve("Alpha"); got.Kind != graph.KindUnique {
		t.Errorf("graph.Resolve(Alpha).Kind = %v, want Unique", got.Kind)
	}
	if got := snap.Navigation().KnowledgeNotes(); len(got) != 1 || got[0].Modified.IsZero() {
		t.Errorf("snapshot KnowledgeNotes = %+v, want the scanner-captured mtime published with Alpha", got)
	}
}

func TestResolutionAndTransclusionsBindToOneGeneration(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeNote(t, root, "Host.md", "Host embeds ![[Target]].\n")
	writeNote(t, root, "Target.md", "old generation body\n")

	store, _ := newTestStore(t, root, nil)
	oldGeneration := store.Current()
	host, ok := oldGeneration.Note("Host.md")
	if !ok {
		t.Fatal("Host.md is absent from the initial generation")
	}
	if got := oldGeneration.Render("Host.md", host.Body, wording.ZhHant).HTML; !strings.Contains(got, "old generation body") {
		t.Fatalf("initial render = %q, want captured target body", got)
	}

	writeNote(t, root, "Target.md", "replacement generation body with a different size\n")
	store.rescan(t.Context())
	newGeneration := store.Current()
	if newGeneration == oldGeneration {
		t.Fatal("changed target did not publish a new generation")
	}
	if got := newGeneration.Render("Host.md", host.Body, wording.ZhHant).HTML; !strings.Contains(got, "replacement generation body") {
		t.Errorf("new generation render = %q, want replacement target body", got)
	}
	if got := oldGeneration.Render("Host.md", host.Body, wording.ZhHant).HTML; !strings.Contains(got, "old generation body") || strings.Contains(got, "replacement generation body") {
		t.Errorf("old generation render changed after publication: %q", got)
	}
}

// TestRescanSwapsOnlyOnFilesystemChange pins the filesystem half of the
// freshness contract without putting real I/O inside a synctest bubble.
func TestRescanSwapsOnlyOnFilesystemChange(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeNote(t, root, "Concepts/Alpha.md", "---\ntype: concept\n---\n\nalpha body\n")

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
	writeNote(t, root, betaRel, "---\ntype: concept\n---\n\nbeta mentions widgets\n")
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
	writeNote(t, root, "Concepts/Alpha.md", "---\ntype: concept\ndraft: true\n---\nbody\n")
	writeNote(t, root, "System/templates/Card.md", "---\ntype: concept\ndraft: false\n---\nbody\n")
	writeNote(t, root, "Maps/Path.md", "---\ntype: study-path\n---\n## Course {sequence=primary}\n- [[Alpha]]\n")
	contract := testContract(t, root)
	store, _ := newTestStore(t, root, contract)

	first := store.Current()
	if len(first.Navigation().Paths()) != 1 || first.Navigation().ArtifactClosure().Diagnostic() != "" || first.Navigation().NavigationClosure().Diagnostic() != "" {
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
	if len(got.Navigation().Paths()) != 0 || got.Navigation().ArtifactClosure().Diagnostic() == "" || got.Navigation().NavigationClosure().Diagnostic() != "" {
		t.Errorf("rescanned navigation = %+v, want artifact-dependent projection unavailable", got.Navigation())
	}
	if got.ArtifactPolicy().Available() || got.ArtifactPolicy().IsNonInstance("System/templates/Card.md") {
		t.Error("rescanned snapshot retained source-stale artifact authority")
	}
	if result := snapshotSearch(t, got.Search(), "Card"); len(result) != 1 || result[0].RelPath != "System/templates/Card.md" {
		t.Errorf("plain lexical search after artifact drift = %+v, want locally readable template", result)
	}
	if _, err := got.Search().Search(lexical.Parse("draft:true"), -1); err == nil {
		t.Error("metadata search succeeded under source-stale artifact policy")
	}

	// Rewrite the fixture through the helper that owns the contract path, so
	// no reader-supplied bytes reach writeNote's shared sink.
	testContract(t, root)
	store.rescan(t.Context())
	if restored := store.Current(); restored.ArtifactPolicy().Available() || len(restored.Navigation().Paths()) != 0 {
		t.Errorf("navigation after restoring the original bytes = %+v, want the digest latch to hold until restart", restored.Navigation())
	}
}

// The same gap can meet New rather than a later rescan. If that refusal is
// assigned back onto the Store, the next rebuild skips the check and instance
// projections stay dark until restart, over a file that came back. New keeps
// the startup handle; the generation that met the gap is refused; the next
// rescan asks again.
func TestNewRecoversInstanceProjectionsWhenAnUnreadableContractReturnsUnchanged(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeNote(t, root, "Concepts/Alpha.md", "---\ntype: concept\n---\nbody\n")
	writeNote(t, root, "Maps/Path.md", "---\ntype: study-path\n---\n## Course {sequence=primary}\n- [[Alpha]]\n")
	contract := testContract(t, root)

	path := filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))
	away := path + ".away"
	if err := os.Rename(path, away); err != nil {
		t.Fatalf("Rename(%q, %q) = %v", path, away, err)
	}
	store, _ := newTestStore(t, root, contract)

	startup := store.Current()
	if len(startup.Navigation().Paths()) != 0 || startup.Navigation().ArtifactClosure().Diagnostic() == "" {
		t.Errorf("navigation while New met an unreadable contract = %+v, want instance projections withheld", startup.Navigation())
	}

	if err := os.Rename(away, path); err != nil {
		t.Fatalf("Rename(%q, %q) = %v", away, path, err)
	}
	// ArtifactPolicy re-captures at read time. While the name is absent the
	// live handle and the refusal copy agree, so this read waits until the
	// identical file is back and the generation New built is still the
	// published one. A generation that classified from the live handle
	// would now report available with empty Paths.
	if startup.ArtifactPolicy().Available() {
		t.Error("the generation New built still classified from a policy that could not re-read its source")
	}
	store.rescan(t.Context())

	recovered := store.Current()
	if recovered == startup {
		t.Fatal("rescan did not publish a new snapshot after the identical contract came back")
	}
	if len(recovered.Navigation().Paths()) != 1 || recovered.Navigation().ArtifactClosure().Diagnostic() != "" {
		t.Errorf("navigation after the identical contract came back = %+v, want the path restored", recovered.Navigation())
	}
	if !recovered.ArtifactPolicy().Available() {
		t.Errorf("artifact policy stayed shut after the identical file came back: %s", recovered.ArtifactPolicy().Diagnostic())
	}
}

// A contract that cannot be read is not a contract that changed. A save-by-
// rename leaves the name absent for a few microseconds, and reading that
// instant as drift would shut instance projections for the life of the
// process over bytes nobody touched. The generation that meets the gap is
// refused; the next one asks again; only bytes that really differ latch.
func TestRescanRecoversInstanceProjectionsWhenAnUnreadableContractReturnsUnchanged(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeNote(t, root, "Concepts/Alpha.md", "---\ntype: concept\n---\nbody\n")
	writeNote(t, root, "Maps/Path.md", "---\ntype: study-path\n---\n## Course {sequence=primary}\n- [[Alpha]]\n")
	contract := testContract(t, root)
	store, _ := newTestStore(t, root, contract)

	first := store.Current()
	if len(first.Navigation().Paths()) != 1 || first.Navigation().ArtifactClosure().Diagnostic() != "" {
		t.Fatalf("initial navigation = %+v, want one available path", first.Navigation())
	}

	path := filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))
	away := path + ".away"
	if err := os.Rename(path, away); err != nil {
		t.Fatalf("Rename(%q, %q) = %v", path, away, err)
	}
	store.rescan(t.Context())

	during := store.Current()
	if during == first {
		t.Fatal("rescan did not publish a new snapshot while the contract was unreadable")
	}
	if len(during.Navigation().Paths()) != 0 || during.Navigation().ArtifactClosure().Diagnostic() == "" {
		t.Errorf("navigation while the contract was unreadable = %+v, want instance projections withheld", during.Navigation())
	}

	if err := os.Rename(away, path); err != nil {
		t.Fatalf("Rename(%q, %q) = %v", away, path, err)
	}
	// ArtifactPolicy re-captures at read time. While the name is absent the
	// live handle and the refusal copy agree, so this read waits until the
	// identical file is back and the generation that met the gap is still
	// the published one. A generation that kept the live handle would now
	// report available with empty Paths.
	if during.ArtifactPolicy().Available() {
		t.Error("the generation that met the gap still classified from a policy that could not re-read its source")
	}
	store.rescan(t.Context())

	recovered := store.Current()
	if recovered == during {
		t.Fatal("rescan did not publish a new snapshot after the identical contract came back")
	}
	if len(recovered.Navigation().Paths()) != 1 || recovered.Navigation().ArtifactClosure().Diagnostic() != "" {
		t.Errorf("navigation after the identical contract came back = %+v, want the path restored", recovered.Navigation())
	}
	if !recovered.ArtifactPolicy().Available() {
		t.Errorf("artifact policy stayed shut after the identical file came back: %s", recovered.ArtifactPolicy().Diagnostic())
	}

	// A later ordinary change must not revive a refusal that already lifted.
	writeNote(t, root, "Concepts/Beta.md", "---\ntype: concept\n---\nbody\n")
	store.rescan(t.Context())
	if got := store.Current(); len(got.Navigation().Paths()) != 1 || got.Navigation().ArtifactClosure().Diagnostic() != "" {
		t.Errorf("navigation after an unrelated rescan = %+v, want the recovered path to stay", got.Navigation())
	}
}

// The egress declaration is revalidated on the same beat as the artifact one,
// so a contract that changed after startup latches both. The oracle is the
// egress policy's own claim, because the same rewrite latches the artifact
// policy too.
func TestRescanLatchesTheEgressDeclarationWhenItsSourceChanges(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeNote(t, root, "Concepts/Alpha.md", "---\ntype: concept\n---\nbody\n")
	writeNote(t, root, "Diary/Monday.md", "---\ntype: concept\n---\nbody\n")
	contract := testContractDeclaringPrivacy(t, root, "Diary")
	store, _ := newTestStore(t, root, contract)

	first := store.Current()
	if !first.PrivacyPolicy().Available() || first.PrivacyPolicy().Diagnostic() != "" {
		t.Fatalf("startup egress policy available = %v, diagnostic = %q, want an available policy and no diagnostic",
			first.PrivacyPolicy().Available(), first.PrivacyPolicy().Diagnostic())
	}
	if first.PrivacyPolicy().EgressAllowed("Diary/Monday.md") {
		t.Fatal("startup egress policy allows the directory the contract named")
	}

	// A rescan the contract did not take part in leaves the declaration alone.
	// Without this the latch could fire on every rebuild and the assertion
	// below would pass for a reason that has nothing to do with the source.
	writeNote(t, root, "Concepts/Beta.md", "---\ntype: concept\n---\nbody\n")
	store.rescan(t.Context())
	if unchanged := store.Current(); !unchanged.PrivacyPolicy().Available() {
		t.Fatalf("egress policy after an unrelated rescan = %q, want it still available",
			unchanged.PrivacyPolicy().Diagnostic())
	}

	writeNote(t, root, "System/schemas/vault-schema.toml", "not = [valid toml")
	store.rescan(t.Context())

	got := store.Current()
	if got.PrivacyPolicy().Available() {
		t.Error("rescanned snapshot kept egress authority derived from contract bytes that are gone")
	}
	if reason := got.PrivacyPolicy().Claim().Reason(); reason != schema.ReasonContractChanged {
		t.Errorf("egress claim reason after the contract changed = %v, want ReasonContractChanged", reason)
	}
	if got.PrivacyPolicy().EgressAllowed("Concepts/Alpha.md") {
		t.Error("a latched egress policy still answers yes; permission must be positive authority")
	}
}

// TestNewReadsAFolderWithNoRulesFileByTheBuiltInRoles pins what a folder with
// no rules file gets: the built-in roles and an artifact policy that holds
// nothing apart, so a note that calls itself a study path is a course and every
// readable note is an instance. Nothing is declared, so nothing is withheld and
// nothing says so.
func TestNewReadsAFolderWithNoRulesFileByTheBuiltInRoles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeNote(t, root, "Maps/Path.md", "---\ntype: study-path\n---\n## Course {sequence=primary}\n- [[Ghost]]\n")
	writeNote(t, root, "Maps/Index.md", "---\ntype: index\n---\n- [[Maps/Path]]\n")
	store, _ := newTestStore(t, root, nil)
	snap := store.Current()

	if !snap.ArtifactPolicy().Available() || snap.ArtifactPolicy().IsNonInstance("System/templates/x.md") {
		t.Fatal("Snapshot.ArtifactPolicy() must be available and hold no directory apart for a folder with no rules file")
	}
	if snap.Navigation().NavigationClosure().Diagnostic() != "" || snap.Navigation().ArtifactClosure().Diagnostic() != "" {
		t.Errorf("snapshot diagnostics = navigation %q artifact %q, want both silent for a folder that claimed nothing", snap.Navigation().NavigationClosure().Diagnostic(), snap.Navigation().ArtifactClosure().Diagnostic())
	}
	if snap.Navigation().ArtifactClosure().Closed() || snap.Navigation().NavigationClosure().Closed() {
		t.Error("projections closed for a folder that never claimed governance")
	}
	if got := len(snap.Navigation().Paths()); got != 1 {
		t.Errorf("snapshot courses = %d, want 1: a note typed study-path is a course with no rules file", got)
	}
	if got := len(snap.Navigation().Maps()); got != 1 {
		t.Errorf("snapshot indexes = %d, want 1: a note typed index is an index with no rules file", got)
	}
	if len(snap.Navigation().KnowledgeNotes()) != 2 {
		t.Errorf("snapshot KnowledgeNotes = %d, want 2", len(snap.Navigation().KnowledgeNotes()))
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
	writeNote(t, root, "Maps/Path.md", "---\ntype: study-path\n---\n## Course {sequence=primary}\n- [[Ghost]]\n")
	store, err := New(
		t.Context(),
		openReader(t, root),
		discardLogger(),
		nil,
		schema.Unreadable(errors.New("toml: line 42: expected a key separator")),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	snap := store.Current()

	if !snap.Navigation().ArtifactClosure().Closed() {
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

// TestReadersRunBesideRescans reads the published generation from several
// goroutines while rescans replace it, so the race detector sees every handoff
// from the reconciliation loop to a reader.
func TestReadersRunBesideRescans(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const rel = "Concepts/Alpha.md"
	body := func(i int) string { return strings.Repeat("edit ", i+1) + "\n" }
	writeNote(t, root, rel, body(0))
	store, _ := newTestStore(t, root, nil)

	ctx, cancel := context.WithCancel(t.Context())
	var readers sync.WaitGroup
	for range 4 {
		readers.Go(func() {
			for ctx.Err() == nil {
				gen := store.Current()
				_, _ = gen.Note(rel)
				_ = gen.Search().Len()
				_ = gen.Freshness()
			}
		})
	}
	// Each edit changes the note's size, so each rescan publishes.
	const edits = 5
	for i := 1; i <= edits; i++ {
		writeNote(t, root, rel, body(i))
		store.rescan(t.Context())
	}
	cancel()
	readers.Wait()

	if note, ok := store.Current().Note(rel); !ok || note.Body != body(edits) {
		t.Errorf("Note(%q) after %d rescans = %+v, present %t; want the last edit published", rel, edits, note, ok)
	}
}

// TestBuildGenerationIndexesNotesOnly pins that the text corpus holds notes and
// nothing else: a file a reader can open as characters is not found by search,
// whatever it holds and wherever it sits.
func TestBuildGenerationIndexesNotesOnly(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeNote(t, root, "Concepts/Note.md", "---\ntype: concept\n---\n\nfindable body\n")
	writeNote(t, root, "Notes/todo.txt", "findable body\n")
	writeNote(t, root, "Notes/page.html", "<p>findable body</p>\n")
	writeNote(t, root, "Diagrams/drawing.svg", `<svg xmlns="http://www.w3.org/2000/svg"><text>findable body</text></svg>`)
	// A lesson sidecar is read for its practice panel and a PDF gets a viewer;
	// neither is shown as text, so neither is searched.
	writeNote(t, root, "System/slots/L01.yaml", "sentences:\n  - text: findable body\n")
	writeNote(t, root, "paper.pdf", "%PDF-1.4\n1 0 obj\n<< /Title (findable body) >>\nendobj\n%%EOF\n")

	contract := testContract(t, root)
	store, _ := newTestStore(t, root, contract)

	got := make([]string, 0, 1)
	for _, result := range snapshotSearch(t, store.Current().Search(), "findable body") {
		got = append(got, result.RelPath)
	}
	if diff := cmp.Diff([]string{"Concepts/Note.md"}, got); diff != "" {
		t.Errorf("indexed entries mismatch (-want +got):\n%s", diff)
	}
	if n := store.Current().Search().Len(); n != 1 {
		t.Errorf("the index holds %d entries, want the one note", n)
	}
}

func searchPaths(t *testing.T, store *Store, query string) []string {
	t.Helper()
	results := snapshotSearch(t, store.Current().Search(), query)
	out := make([]string, len(results))
	for i := range results {
		out[i] = results[i].RelPath
	}
	return out
}

func writeNeedlePair(t *testing.T, root string) {
	t.Helper()
	writeNote(t, root, "System/early.md", "---\ntype: system\n---\nneedle in a template\n")
	writeNote(t, root, "Writing/late.md", "---\ntype: writing\n---\nneedle in a note\n")
	writeNote(t, root, "AAA.md", "---\ntype: guide\n---\nneedle at the root\n")
}

// TestSearchRanksAKnowledgeLayerNoteAboveASystemFile plumbs OutsideKnowledge
// from the contract's KnowledgeScope. Writing/ is in the layer; System/ and a
// vault-root file are not. The System path and the root file both sort first,
// so path order cannot pass this. Nothing leaves the index.
func TestSearchRanksAKnowledgeLayerNoteAboveASystemFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeNeedlePair(t, root)
	store, _ := newTestStore(t, root, testContract(t, root))

	got := searchPaths(t, store, "needle")
	want := []string{"Writing/late.md", "AAA.md", "System/early.md"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Search(needle) order mismatch (-want +got):\n%s", diff)
	}
}

// TestSearchKeepsReadingOrderWhenNoKnowledgeLayerIsDeclared is the undeclared
// polarity through the same three files: Includes reports everything, so the
// flag stays off and the vault's reading order stands.
func TestSearchKeepsReadingOrderWhenNoKnowledgeLayerIsDeclared(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeNeedlePair(t, root)
	contract := writeContract(t, root, replacing(t, `checked = ["Concepts", "Sources", "Maps", "Writing", "Synthesis", "Inbox"]`, `checked = []`))
	if contract.KnowledgeScope().Available() {
		t.Fatal("the undeclared fixture still claims a knowledge layer")
	}
	store, _ := newTestStore(t, root, contract)

	got := searchPaths(t, store, "needle")
	want := []string{"AAA.md", "System/early.md", "Writing/late.md"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Search(needle) order mismatch (-want +got):\n%s", diff)
	}
}

// TestBuildGenerationStillResolvesWikilinksToFiles guards the hole the widening most
// plausibly opens. Every vault file has to reach the link resolver whether or
// not its bytes are read, so a note pointing at a picture keeps resolving.
func TestBuildGenerationStillResolvesWikilinksToFiles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeNote(t, root, "Concepts/Note.md", "---\ntype: concept\n---\n\nsee [[drawing.svg]]\n")
	writeNote(t, root, "Diagrams/drawing.svg", `<svg xmlns="http://www.w3.org/2000/svg"></svg>`)
	contract := testContract(t, root)
	store, _ := newTestStore(t, root, contract)

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

// TestOneUnreadableFigureDoesNotFreezeTheFolder covers the difference between
// a file the reading surface is made of and a figure, which only lends its page
// its measurements. A note that cannot be read holds the generation back until
// it can be; a figure that is permanently unreadable — a mode-000 export,
// something another process holds — would hold the folder forever, and every
// note written after it would be a 404 in a folder that is otherwise fine.
func TestOneUnreadableFigureDoesNotFreezeTheFolder(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	const figure = "Diagrams/figure.html"
	writeNote(t, root, "Concepts/Alpha.md", "---\ntype: concept\n---\nalpha\n")
	writeNote(t, root, figure, "<p>figure</p>\n")
	source := newRecordingSource(t, root)
	source.fail[figure] = 1 << 30
	store := buildStore(t, source, discardLogger(), testContract(t, root))
	if source.reads[figure] == 0 {
		t.Fatal("the build never read the figure, so nothing below is under test")
	}
	if store.retry || !store.Current().Freshness().Complete {
		t.Fatal("an unreadable figure held the generation back; every later change would stop being published")
	}

	const added = "Concepts/Beta.md"
	writeNote(t, root, added, "---\ntype: concept\n---\nbeta\n")
	store.rescan(t.Context())
	if _, ok := store.Current().Note(added); !ok {
		t.Error("a note written after the unreadable figure never reached a published generation")
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
	writeNote(t, root, failing, "---\ntype: concept\n---\nalpha\n")
	writeNote(t, root, "Concepts/Base.md", "---\ntype: concept\n---\nbase\n")
	contract := testContract(t, root)
	store, source := newRecordingStore(t, root, contract)
	complete := store.Current()
	base := time.Now()
	clock := base
	store.now = func() time.Time { return clock }

	// The note is edited — its size changes, so the next rescan rebuilds — and
	// from now on every read of it fails.
	writeNote(t, root, failing, "---\ntype: concept\n---\nalpha, now a different size\n")
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

// TestAnUnreadableNoteHoldsTheFolderBackUntilItDegrades pins both sides of
// the degrade threshold. A read that fails once and works on the next attempt
// must never publish half a folder, so the last whole generation is held. Held
// without a bound, every note written after one file stopped being readable is
// a 404, so after degradeAfter incomplete attempts in a row the folder
// publishes what it could read, carries the last copy of what it could not,
// and says it is not whole until a read succeeds again.
func TestAnUnreadableNoteHoldsTheFolderBackUntilItDegrades(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	const (
		carried = "Concepts/Carried.md"
		never   = "Concepts/Never.md"
		added   = "Concepts/Added.md"
	)
	writeNote(t, root, carried, "---\ntype: concept\n---\nthe words read before the file shut\n")
	store, source := newRecordingStore(t, root, testContract(t, root))
	whole := store.Current()
	base := time.Now()
	clock := base
	store.now = func() time.Time { return clock }

	// The read note is edited to a different size, so the next rescan builds;
	// a note arrives that is never readable, and one that reads fine.
	writeNote(t, root, carried, "---\ntype: concept\n---\nwords written after the file shut\n")
	writeNote(t, root, never, "---\ntype: concept\n---\nnever read\n")
	writeNote(t, root, added, "---\ntype: concept\n---\nadded\n")
	source.fail[carried] = 1 << 30
	source.fail[never] = 1 << 30

	// One rescan per scan interval, as the ticker drives it. The backoff puts
	// the attempts on ticks 0, 1 and 3, so ticks 0 to 2 carry two of them.
	rescanAt := func(tick int) {
		clock = base.Add(time.Duration(tick) * scanInterval)
		store.rescan(t.Context())
	}
	for tick := range 3 {
		rescanAt(tick)
	}
	if store.Current() != whole {
		t.Fatal("a failure that has not yet outlasted the threshold published half a folder")
	}

	rescanAt(3)
	degraded := store.Current()
	if degraded == whole {
		t.Fatal("the folder never degraded: every change after the failure stayed invisible")
	}
	if _, ok := degraded.Note(added); !ok {
		t.Error("a note written while two files were unreadable never reached a published generation")
	}
	fresh := degraded.Freshness()
	if fresh.Complete {
		t.Error("a generation published without two of its sources calls itself complete")
	}
	if !fresh.LastComplete.Equal(whole.Freshness().BuiltAt) {
		t.Errorf("LastComplete = %v, want the last whole read at %v", fresh.LastComplete, whole.Freshness().BuiltAt)
	}
	blocked := make([]string, 0, len(fresh.Blocked))
	for _, source := range fresh.Blocked {
		blocked = append(blocked, source.Path)
	}
	slices.Sort(blocked)
	if diff := cmp.Diff([]string{carried, never}, blocked); diff != "" {
		t.Errorf("blocked sources mismatch (-want +got):\n%s", diff)
	}

	// A note read once keeps that copy, marked stale, and answers searches for
	// the words its page shows.
	kept, ok := degraded.Note(carried)
	if !ok || !kept.Stale || !strings.Contains(kept.Body, "the words read before the file shut") {
		t.Errorf("carried note = %+v, present %t; want the last copy read, marked stale", kept, ok)
	}
	if found := snapshotSearch(t, degraded.Search(), "the words read before the file shut"); len(found) != 1 || found[0].RelPath != carried {
		t.Errorf("searching the carried copy's own words = %+v, want %s", found, carried)
	}
	// A note never read has no copy, but stays a file, so its page can say it
	// could not be read rather than that nothing is there.
	if unread, ok := degraded.Note(never); ok {
		t.Errorf("a file never read was published with a body: %+v", unread)
	}
	if _, ok := degraded.Entry(never); !ok {
		t.Error("the generation does not hold the unread file's identity")
	}
	// The carried copy replaces the resolver stub rather than joining it.
	if collisions := degraded.Health().Collisions; len(collisions) != 0 {
		t.Errorf("degraded generation reports name collisions %+v; the carried copy was registered twice", collisions)
	}

	// Recovery is the ordinary path: the files read again and every degraded
	// fact clears.
	source.fail[carried] = 0
	source.fail[never] = 0
	rescanAt(20)
	recovered := store.Current()
	if fresh := recovered.Freshness(); !fresh.Complete || len(fresh.Blocked) != 0 {
		t.Errorf("recovered freshness = %+v, want whole with nothing blocked", fresh)
	}
	if note, ok := recovered.Note(carried); !ok || note.Stale || !strings.Contains(note.Body, "words written after the file shut") {
		t.Errorf("recovered note = %+v, present %t; want the edited body read afresh", note, ok)
	}
	for _, rel := range []string{never, added} {
		if _, ok := recovered.Note(rel); !ok {
			t.Errorf("Note(%q) missing from the recovered generation", rel)
		}
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
	writeNote(t, root, unreadable, "---\ntype: concept\n---\nalpha\n")
	contract := testContract(t, root)
	store, source := newRecordingStore(t, root, contract)
	whole := store.Current()
	base := time.Now()
	clock := base
	store.now = func() time.Time { return clock }

	writeNote(t, root, unreadable, "---\ntype: concept\n---\nalpha, edited and now unreadable\n")
	source.fail[unreadable] = 1 << 30

	// A note arrives before every attempt, so the file domain differs each
	// time and the backoff restarts on each one.
	for tick := range degradeAfter {
		writeNote(t, root, "Concepts/Beta-"+strconv.Itoa(tick)+".md",
			"---\ntype: concept\n---\nbeta\n")
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
	writeNote(t, root, blocked, "---\ntype: concept\n---\nalpha\n")
	contract := testContract(t, root)
	source := newRecordingSource(t, root)
	source.fail[blocked] = 1
	store := buildStore(t, source, discardLogger(), contract)

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
	writeNote(t, root, blocked, "---\ntype: concept\n---\nalpha, edited to a different size\n")
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
	writeNote(t, root, blocked, "---\ntype: concept\n---\nalpha\n")
	contract := testContract(t, root)
	source := newRecordingSource(t, root)
	source.fail[blocked] = 1
	store := buildStore(t, source, discardLogger(), contract)

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
// reconciliation cycle: the first tick a reconcilePeriod after the last
// rebuild skips the short-circuit, so the changed bytes are republished within
// the period.
func TestMetadataInvisibleEditIsEventuallyRepublished(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	const relPath = "Concepts/Alpha.md"
	const oldBody = "---\ntype: concept\n---\nMARKER_OLD_XYZ\n"
	const newBody = "---\ntype: concept\n---\nMARKER_NEW_ABC\n"
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

	// The store's clock starts where New left lastRebuild, so each tick can
	// advance wall-clock time the same way the scanner does.
	clock := store.now()
	store.now = func() time.Time { return clock }

	// One ordinary rescan keeps the fast path honest: metadata is unchanged,
	// so nothing is re-read and the generation stands.
	store.rescan(t.Context())
	if store.Current() != first {
		t.Fatal("a metadata-invisible edit was picked up by the fast path; this test no longer covers the slow cycle")
	}

	// Driving the loop through one reconciliation period republishes the
	// changed bytes even though no metadata ever moved.
	for range int(reconcilePeriod / scanInterval) {
		clock = clock.Add(scanInterval)
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
// while rebuilds are failing, the reconciliation clock is ignored and never
// forces the expensive path. The backoff owns the rebuild cadence there, and
// every retry attempt is already a full re-read, so a forced rebuild would
// only bypass the bound the backoff exists to hold.
func TestReconciliationDefersToFailureBackoff(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	const failing = "Concepts/Alpha.md"
	writeNote(t, root, failing, "---\ntype: concept\n---\nalpha\n")
	contract := testContract(t, root)
	store, source := newRecordingStore(t, root, contract)
	// The clock never advances, so the backoff delay never expires.
	clock := time.Now()
	store.now = func() time.Time { return clock }

	writeNote(t, root, failing, "---\ntype: concept\n---\nalpha, now a different size\n")
	source.fail[failing] = 1 << 30
	store.rescan(t.Context())
	if !store.retry {
		t.Fatal("the failed rebuild did not latch a retry")
	}

	before := source.reads[failing]
	for range int(reconcilePeriod/scanInterval) + 10 {
		store.rescan(t.Context())
	}
	if got := source.reads[failing] - before; got != 0 {
		t.Errorf("reconciliation forced %d rebuild attempts past the failure backoff, want 0", got)
	}
}

// TestAnOverCapNoteIsASkipNotARead pins that a note past the source bound is
// a published skip, under the ceiling every other file has: the generation
// never reads it, holds nothing of it, and does not hold the folder back for
// it. Its name stays, so a citation lands on it and the health page lists it.
func TestAnOverCapNoteIsASkipNotARead(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	const needle = "rarespelunker"
	writeNote(t, root, "small.md", "---\ntype: concept\n---\n"+needle+" sits here; see [[huge]].\n")
	huge := "---\ntype: concept\n---\n" + needle + " sits here too.\n" +
		strings.Repeat("padding padding padding\n", render.MaxSourceBytes/24+1)
	writeNote(t, root, "huge.md", huge)
	store, source := newRecordingStore(t, root, testContract(t, root))
	if store.retry {
		t.Fatal("an over-cap note held the generation back; one oversized file would wedge the folder")
	}
	if got := source.reads["huge.md"]; got != 0 {
		t.Fatalf("the over-cap note was read %d times, want 0", got)
	}

	gen := store.Current()
	if note, ok := gen.Note("huge.md"); ok {
		t.Fatalf("the over-cap note was retained in the generation; body length = %d", len(note.Body))
	}
	if _, isFile := gen.Entry("huge.md"); !isFile {
		t.Fatal("the over-cap note vanished from the scan; the page would look like a missing file")
	}
	want := []Skipped{{Path: "huge.md", Reason: overSourceBound, Size: int64(len(huge))}}
	if diff := cmp.Diff(want, gen.Skipped()); diff != "" {
		t.Errorf("Skipped() mismatch (-want +got):\n%s", diff)
	}
	if got := searchPaths(t, store, needle); !slices.Equal(got, []string{"small.md"}) {
		t.Errorf("Search(%q) = %v, want the note under the bound alone", needle, got)
	}
	if resolved := gen.Graph().Resolve("huge"); resolved.Kind != graph.KindUnique || resolved.RelPath != "huge.md" {
		t.Errorf("Resolve(huge) = %+v, want unique huge.md", resolved)
	}
	small, ok := gen.Note("small.md")
	if !ok {
		t.Fatal("the note under the bound is absent from the generation")
	}
	if html := gen.Render("small.md", small.Body, wording.ZhHant).HTML; !strings.Contains(html, "/notes/huge.md") {
		t.Errorf("a wikilink to the over-cap note no longer resolves; html = %q", html)
	}

	writeNote(t, root, "later.md", "---\ntype: concept\n---\nlater\n")
	store.rescan(t.Context())
	if _, ok := store.Current().Note("later.md"); !ok {
		t.Error("a note written after the over-cap skip never reached a published generation")
	}
}

// One Store drives one reconciliation loop, and eight of its fields — the
// previous scan, the retry flag, the backoff schedule and the two incomplete
// counts — are that loop's alone. Run is exported, so nothing but this stops a
// caller starting a second loop over them; the race detector finds it at
// lastRebuild, and the answer a page reads afterwards is whatever the two
// loops left behind.
func TestASecondReconciliationLoopIsRefused(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeNote(t, root, "Concepts/Alpha.md", "---\ntype: concept\n---\nbody\n")
	contract := testContract(t, root)
	store, _ := newTestStore(t, root, contract)

	// A cancelled context makes Run claim the loop and return at once, so the
	// second call is reached without either loop having to be timed.
	stopped, cancel := context.WithCancel(t.Context())
	cancel()
	store.Run(stopped)

	recovered := func() (value any) {
		defer func() { value = recover() }()
		store.Run(stopped)
		t.Error("a second Run returned instead of stopping, leaving two loops free to advance the same fields")
		return nil
	}()

	if recovered == nil {
		t.Fatal("Store.Run can be called twice without saying so")
	}
}

// TestMissingFileTellsAnAbsentPathFromAnUnseenOne holds the distinction the
// reading page's picture mark rests on. The scan does not walk a path with a
// hidden segment, so that path is absent from every generation — and answering
// "missing" there would have a note's own page report this reader's boundary as
// a file somebody lost. The rule is the one the judging side already applies to
// a dead path reference, and it lives here because only this side knows where
// the scan looks.
func TestMissingFileTellsAnAbsentPathFromAnUnseenOne(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeNote(t, root, "Concepts/Alpha.md", "---\ntype: concept\n---\nbody\n")
	writeNote(t, root, "Concepts/there.png", "not really a picture\n")
	// Written composed, the way a vault holds a name.
	writeNote(t, root, "Concepts/\u00e9.png", "not really a picture\n")
	contract := testContract(t, root)
	store, _ := newTestStore(t, root, contract)
	view := store.Current()

	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "a file the scan observed", path: "Concepts/there.png"},
		{name: "a file nothing there answers to", path: "Concepts/gone.png", want: true},
		{name: "under a hidden directory, which the scan never walks", path: ".attachments/gone.png"},
		{name: "a hidden segment deeper in", path: "Concepts/.drafts/gone.png"},
		// A vault holds its names composed and a note's own text can carry
		// either spelling, so the decomposed form of a file that is there must
		// answer the same as the composed one — the route that serves the bytes
		// composes before it looks, and a page calling this missing would be
		// reporting its own reading of the name as somebody's lost file.
		{name: "the decomposed spelling of a file the scan observed", path: "Concepts/e\u0301.png"},
		{name: "the decomposed spelling of a file nothing answers to", path: "Concepts/gone\u0301.png", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := view.MissingFile(tt.path); got != tt.want {
				t.Errorf("MissingFile(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

// TestIsDirectoryTellsAFolderFromAFileAndFromNothing holds the answer a link to
// a folder is routed by: a directory the scan walked is one, a regular file is
// not, and a path nothing answers to is not.
func TestIsDirectoryTellsAFolderFromAFileAndFromNothing(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeNote(t, root, "Concepts/Alpha.md", "---\ntype: concept\n---\nbody\n")
	writeNote(t, root, "Concepts/lab/code/main.go", "package main\n")
	writeNote(t, root, "Concepts/\u00e9/x.txt", "x\n")
	contract := testContract(t, root)
	store, _ := newTestStore(t, root, contract)
	view := store.Current()

	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "a folder holding a file", path: "Concepts/lab/code", want: true},
		{name: "a folder holding only a folder", path: "Concepts/lab", want: true},
		{name: "a regular file", path: "Concepts/lab/code/main.go"},
		{name: "a note", path: "Concepts/Alpha.md"},
		{name: "nothing there", path: "Concepts/gone"},
		{name: "the decomposed spelling of a folder", path: "Concepts/e\u0301", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := view.IsDirectory(tt.path); got != tt.want {
				t.Errorf("IsDirectory(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestAnswerRoleIsFixedAtStartAcrossRescans(t *testing.T) {
	root := t.TempDir()
	contract := writeContract(t, root, func(text string) string {
		text = replacing(t, "[types.thought]\nrole = \"thought\"\n", "[types.thought]\n")(text)
		return replacing(t, "[types.writing]\n", "[types.writing]\nrole = \"thought\"\n")(text)
	})
	store, _ := newTestStore(t, root, contract)
	if got := store.Current().NavigationRoles().AnswerType(); got != "writing" {
		t.Fatalf("answer role at start = %q, want %q", got, "writing")
	}
	// The declaration is withdrawn on disk and the vault changes, so the next
	// scan rebuilds. Like the course and map roles, the role holds until the
	// next start.
	testContract(t, root)
	writeNote(t, root, "Later.md", "# Later\n\nlater\n")
	store.rescan(t.Context())
	if _, ok := store.Current().Note("Later.md"); !ok {
		t.Fatal("the rescan did not publish a new generation, so it proves nothing")
	}
	if got := store.Current().NavigationRoles().AnswerType(); got != "writing" {
		t.Errorf("answer role after a rescan = %q, want it fixed at %q until the next start", got, "writing")
	}
}
