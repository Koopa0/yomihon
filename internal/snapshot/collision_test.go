package snapshot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/vault"
)

// The pair every test below collides: one name as a Linux keyboard writes it,
// and the same name as a macOS-to-Linux copy or a git checkout leaves it. They
// print as one glyph, which is why the refusal has to name them both.
const (
	collidingComposed   = "Notes/\u304c.md"
	collidingDecomposed = "Notes/\u304b\u3099.md"

	collidingBody = "---\ntitle: Ga\ntype: concept\n---\nga\n"
	freshBody     = "---\ntitle: Fresh\ntype: concept\n---\nfresh\n"
)

// refusedScanMessage is the line the loop logs when a scan fails, which is where
// the files have to be named for the person watching the terminal.
const refusedScanMessage = "vault scan unavailable; retaining previous snapshot"

// loggedLine is what the loop said in one call: its message, and each of its
// attributes as the text a person reading the log would see.
type loggedLine struct {
	message string
	attrs   map[string]string
}

// recordingHandler keeps every line it is handed, so a test reads what the loop
// said rather than what a formatter made of it.
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
	line := loggedLine{message: record.Message, attrs: make(map[string]string)}
	record.Attrs(func(attr slog.Attr) bool {
		line.attrs[attr.Key] = attr.Value.String()
		return true
	})
	h.mu.Lock()
	defer h.mu.Unlock()
	*h.lines = append(*h.lines, line)
	return nil
}

func (h recordingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h recordingHandler) WithGroup(string) slog.Handler      { return h }

// refusedScanErrors are the error values of every refused-scan line so far.
func refusedScanErrors(lines []loggedLine) []string {
	var out []string
	for i := range lines {
		if lines[i].message == refusedScanMessage {
			out = append(out, lines[i].attrs["error"])
		}
	}
	return out
}

// collisionFolder is a folder holding one note and one half of the pair, read
// into a store whose log a test can inspect.
func collisionFolder(t *testing.T) (store *Store, root string, logged func() []loggedLine) {
	t.Helper()
	root = t.TempDir()
	writeNote(t, root, "Notes/Existing.md", "---\ntitle: Existing\ntype: concept\n---\nexisting\n")
	writeNote(t, root, collidingComposed, collidingBody)
	contract := testContract(t, root)
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open: %v", err)
	}
	t.Cleanup(func() { closeReader(t, reader) })
	log, logged := newRecordingLogger()
	store, err = New(t.Context(), reader, log, contract, contract.Governance())
	if err != nil {
		t.Fatalf("snapshot.New: %v", err)
	}
	return store, root, logged
}

// addTheTwin writes the second spelling beside the first, and skips the test
// where the filesystem folds the two into one file and there is no pair.
func addTheTwin(t *testing.T, root string) {
	t.Helper()
	writeNote(t, root, collidingDecomposed, collidingBody)
	entries, err := os.ReadDir(filepath.Join(root, "Notes"))
	if err != nil {
		t.Fatalf("ReadDir(Notes): %v", err)
	}
	var listed []string
	for _, entry := range entries {
		listed = append(listed, entry.Name())
	}
	if !slices.Contains(listed, path.Base(collidingComposed)) || !slices.Contains(listed, path.Base(collidingDecomposed)) {
		t.Skip("this filesystem folds the two spellings into one file, so there is no pair to refuse")
	}
}

func removeTheTwin(t *testing.T, root string) {
	t.Helper()
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(collidingDecomposed))); err != nil {
		t.Fatalf("remove the decomposed twin: %v", err)
	}
}

// TestACollidingPairIsNamedAndFreezesPublicationUntilOneOfThemIsGone is the
// whole story of the fault, start to finish. Two names with one canonical path
// make the scan refuse the folder, which is right; what was wrong was that a
// note written beside them answered 404 for good while the log said only that
// the scan was unavailable. The refusal has to name both files in the line the
// loop logs and in the account the pages read, and taking one of the two away
// has to put publication back with nothing restarted.
func TestACollidingPairIsNamedAndFreezesPublicationUntilOneOfThemIsGone(t *testing.T) {
	t.Parallel()

	store, root, logged := collisionFolder(t)
	before := store.Current()
	if pair := before.Freshness().Collision; pair != nil {
		t.Fatalf("a folder with no pair in it already names a collision: %q", pair)
	}

	addTheTwin(t, root)
	writeNote(t, root, "Notes/Fresh.md", freshBody)
	store.rescan(t.Context())

	if store.Current() != before {
		t.Error("a scan that was refused published a generation")
	}
	if _, ok := store.Current().Note("Notes/Fresh.md"); ok {
		t.Error("a note written beside the pair is served, so nothing was refused")
	}
	want := []string{collidingDecomposed, collidingComposed}
	if diff := cmp.Diff(want, store.Current().Freshness().Collision); diff != "" {
		t.Errorf("the pages' account of the refusal (-want +got):\n%s", diff)
	}
	refusals := refusedScanErrors(logged())
	if len(refusals) != 1 {
		t.Fatalf("the loop logged %d refused scans, want 1: %q", len(refusals), refusals)
	}
	for _, raw := range []string{collidingDecomposed, collidingComposed} {
		if !strings.Contains(refusals[0], vault.Spelled(raw)) {
			t.Errorf("the log line %q does not name %q", refusals[0], raw)
		}
	}

	removeTheTwin(t, root)
	store.rescan(t.Context())

	if _, ok := store.Current().Note("Notes/Fresh.md"); !ok {
		t.Error("publication did not resume once one of the pair was gone")
	}
	if pair := store.Current().Freshness().Collision; pair != nil {
		t.Errorf("the collision outlived the pair: %q", pair)
	}
}

// TestAFolderThatReturnsToWhatWasPublishedLeavesNoCollision holds the one case
// the test above cannot reach: the pair appears and goes again before anything
// else changes, so the next scan is level with what was published and the loop
// stops before it builds. A collision cleared only where a build is begun would
// stand over a folder that is whole. It needs a filesystem that keeps both
// spellings apart, so it skips on one that folds them; the test at the end of
// this file holds the same clear on every filesystem.
func TestAFolderThatReturnsToWhatWasPublishedLeavesNoCollision(t *testing.T) {
	t.Parallel()

	store, root, _ := collisionFolder(t)
	before := store.Current()

	addTheTwin(t, root)
	store.rescan(t.Context())
	if store.Current().Freshness().Collision == nil {
		t.Fatalf("the pair left no collision, so nothing below is under test")
	}

	removeTheTwin(t, root)
	store.rescan(t.Context())

	if store.Current() != before {
		t.Fatal("a folder level with what was published was rebuilt, so the loop never took the short way out")
	}
	if pair := store.Current().Freshness().Collision; pair != nil {
		t.Errorf("the collision stands over a folder with no pair in it: %q", pair)
	}
}

// TestStartingOnACollidingPairNamesBothFiles is the same refusal at the other
// place it is met. A server started on such a folder exits, and what it prints
// as it goes is all anyone has to repair the folder with.
func TestStartingOnACollidingPairNamesBothFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeNote(t, root, collidingComposed, collidingBody)
	addTheTwin(t, root)
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open: %v", err)
	}
	t.Cleanup(func() { closeReader(t, reader) })
	contract := testContract(t, root)

	_, err = New(t.Context(), reader, discardLogger(), contract, contract.Governance())
	if !errors.Is(err, vault.ErrCanonicalCollision) {
		t.Fatalf("New on a colliding pair = %v, want ErrCanonicalCollision", err)
	}
	for _, raw := range []string{collidingDecomposed, collidingComposed} {
		if !strings.Contains(err.Error(), vault.Spelled(raw)) {
			t.Errorf("the startup error %q does not name %q", err, raw)
		}
	}
}

// refusingSource answers a scan with the error it is told to, and otherwise
// leaves the folder to the reader beneath it.
type refusingSource struct {
	Source

	refusal error
}

func (s *refusingSource) ScanAvailable(ctx context.Context) (vault.Scan, error) {
	if s.refusal != nil {
		return vault.Scan{}, s.refusal
	}
	return s.Source.ScanAvailable(ctx)
}

// refusableFolder is a folder holding one note, read into a store through a
// source a test can make refuse its scan. A fake source rather than a folder,
// because no real folder is refused one way and then another by itself, and
// because a folder that holds both spellings of a name cannot be made on every
// filesystem.
func refusableFolder(t *testing.T) (store *Store, source *refusingSource) {
	t.Helper()
	root := t.TempDir()
	writeNote(t, root, "Notes/Existing.md", "---\ntitle: Existing\ntype: concept\n---\nexisting\n")
	contract := testContract(t, root)
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open: %v", err)
	}
	t.Cleanup(func() { closeReader(t, reader) })
	source = &refusingSource{Source: reader}
	store, err = New(t.Context(), source, discardLogger(), contract, contract.Governance())
	if err != nil {
		t.Fatalf("snapshot.New: %v", err)
	}
	return store, source
}

// refusedForTheCollision is the refusal the reader gives for the pair, wrapped
// the way the scan wraps what it meets.
func refusedForTheCollision() error {
	pair := &vault.CollisionError{Paths: [2]string{collidingDecomposed, collidingComposed}}
	return fmt.Errorf("list pinned vault: %w", pair)
}

// TestAScanRefusedForAnotherReasonTakesTheCollisionDown holds the branch of the
// loop where the cause of the refusal changes. A pair is reported, and then the
// folder cannot be scanned for some other reason: what stands against it is no
// longer known to be that pair, and a collision naming two files the scan did
// not get as far as would be a claim nobody just checked.
func TestAScanRefusedForAnotherReasonTakesTheCollisionDown(t *testing.T) {
	t.Parallel()

	store, source := refusableFolder(t)
	source.refusal = refusedForTheCollision()
	store.rescan(t.Context())
	if store.Current().Freshness().Collision == nil {
		t.Fatal("the pair left no collision, so nothing below is under test")
	}

	source.refusal = errors.New("list pinned vault: permission denied")
	store.rescan(t.Context())
	if pair := store.Current().Freshness().Collision; pair != nil {
		t.Errorf("a refusal that is not a collision left the collision standing: %q", pair)
	}
}

// TestAScanThatCompletesOverAnUnchangedFolderTakesTheCollisionDown holds the
// clear on every filesystem. The refusal is faked and the folder beneath it is
// never changed, so the scan that follows completes level with what was
// published and the loop stops before it builds. That is the only path on which
// nothing but the clear itself can take the collision down: a rebuild would
// store a fresh account of its own and hide a clear that was never made.
func TestAScanThatCompletesOverAnUnchangedFolderTakesTheCollisionDown(t *testing.T) {
	t.Parallel()

	store, source := refusableFolder(t)
	before := store.Current()

	source.refusal = refusedForTheCollision()
	store.rescan(t.Context())
	want := []string{collidingDecomposed, collidingComposed}
	if diff := cmp.Diff(want, store.Current().Freshness().Collision); diff != "" {
		t.Fatalf("the refusal did not leave the pair, so nothing below is under test (-want +got):\n%s", diff)
	}

	source.refusal = nil
	store.rescan(t.Context())

	if store.Current() != before {
		t.Fatal("a folder level with what was published was rebuilt, so the loop never took the short way out")
	}
	if pair := store.Current().Freshness().Collision; pair != nil {
		t.Errorf("a scan that completed left the collision standing: %q", pair)
	}
}
