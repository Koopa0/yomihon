package snapshot

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
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
	if notices := before.Freshness().Notices; len(notices) != 0 {
		t.Fatalf("a folder with no pair in it already carries notices: %+v", notices)
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
	want := []Notice{{Reason: NoticeNamesCollide, Paths: []string{collidingDecomposed, collidingComposed}}}
	if diff := cmp.Diff(want, store.Current().Freshness().Notices); diff != "" {
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
	if notices := store.Current().Freshness().Notices; len(notices) != 0 {
		t.Errorf("the notice outlived the pair: %+v", notices)
	}
}

// TestAFolderThatReturnsToWhatWasPublishedLeavesNoNotice holds the one case the
// test above cannot reach: the pair appears and goes again before anything
// else changes, so the next scan is level with what was published and the loop
// stops before it builds. A notice cleared only where a build is begun would
// stand over a folder that is whole.
func TestAFolderThatReturnsToWhatWasPublishedLeavesNoNotice(t *testing.T) {
	t.Parallel()

	store, root, _ := collisionFolder(t)
	before := store.Current()

	addTheTwin(t, root)
	store.rescan(t.Context())
	if len(store.Current().Freshness().Notices) != 1 {
		t.Fatalf("the pair left no notice, so nothing below is under test")
	}

	removeTheTwin(t, root)
	store.rescan(t.Context())

	if store.Current() != before {
		t.Fatal("a folder level with what was published was rebuilt, so the loop never took the short way out")
	}
	if notices := store.Current().Freshness().Notices; len(notices) != 0 {
		t.Errorf("the notice stands over a folder with no pair in it: %+v", notices)
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

// noticeOutsideTheSet is a reason the closed set does not hold, so the two
// tests below are about the record that carries notices and not about any one
// reason: a scan that succeeds takes the collision notice down, and these have
// to show a notice put there some other way is left alone.
const noticeOutsideTheSet NoticeReason = 200

func standingNoticeFolder(t *testing.T) (store *Store, source *recordingSource, root string) {
	t.Helper()
	root = t.TempDir()
	writeNote(t, root, "Notes/Existing.md", "---\ntitle: Existing\ntype: concept\n---\nexisting\n")
	contract := testContract(t, root)
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open: %v", err)
	}
	t.Cleanup(func() { closeReader(t, reader) })
	source = &recordingSource{Source: reader, reads: make(map[string]int), fail: make(map[string]int)}
	store, err = New(t.Context(), source, discardLogger(), contract, contract.Governance())
	if err != nil {
		t.Fatalf("snapshot.New: %v", err)
	}
	store.setNotice(Notice{Reason: noticeOutsideTheSet, Paths: []string{"Notes/Existing.md"}})
	return store, source, root
}

// TestANoticeSurvivesAnIncompleteAttempt: a build that could not read a file
// replaces the record's account of what could not be read, and says nothing
// about the folder's other notices, which are about the scan and not the build.
func TestANoticeSurvivesAnIncompleteAttempt(t *testing.T) {
	t.Parallel()

	store, source, root := standingNoticeFolder(t)
	const unread = "Notes/Unread.md"
	writeNote(t, root, unread, freshBody)
	source.fail[unread] = 1 << 30
	store.rescan(t.Context())

	fresh := store.Current().Freshness()
	if len(fresh.Blocked) != 1 || fresh.FailedRetries != 1 {
		t.Fatalf("the attempt left Blocked = %+v, FailedRetries = %d, so no incomplete attempt was recorded", fresh.Blocked, fresh.FailedRetries)
	}
	if len(fresh.Notices) != 1 || fresh.Notices[0].Reason != noticeOutsideTheSet {
		t.Errorf("an incomplete attempt dropped the notices: %+v", fresh.Notices)
	}
}

// TestANoticeSurvivesACompletedBuild: a build that read everything clears the
// record of what could not be read, and a notice that is not about that stands.
func TestANoticeSurvivesACompletedBuild(t *testing.T) {
	t.Parallel()

	store, _, root := standingNoticeFolder(t)
	writeNote(t, root, "Notes/Fresh.md", freshBody)
	before := store.Current()
	store.rescan(t.Context())

	if store.Current() == before {
		t.Fatal("the folder changed and nothing was published, so no build completed")
	}
	if notices := store.Current().Freshness().Notices; len(notices) != 1 || notices[0].Reason != noticeOutsideTheSet {
		t.Errorf("a completed build dropped the notices: %+v", notices)
	}
}

// TestNoticeReasonsIsEveryReasonDeclared pins the set the surfaces word. The
// list is derived from the declarations by a sentinel rather than kept beside
// them, and this reads the declarations from the source to say it did not fall
// behind them: a reason it left out would be one no test of "every reason has
// words" ever asks about.
func TestNoticeReasonsIsEveryReasonDeclared(t *testing.T) {
	t.Parallel()

	file, err := parser.ParseFile(token.NewFileSet(), "notice.go", nil, 0)
	if err != nil {
		t.Fatalf("parse notice.go: %v", err)
	}
	var declared []string
	ast.Inspect(file, func(n ast.Node) bool {
		spec, ok := n.(*ast.ValueSpec)
		if !ok {
			return true
		}
		if typed, isType := spec.Type.(*ast.Ident); isType && typed.Name == "NoticeReason" {
			for _, name := range spec.Names {
				if name.IsExported() {
					declared = append(declared, name.Name)
				}
			}
		}
		return true
	})
	if len(declared) == 0 {
		t.Fatal("no NoticeReason is declared in notice.go, so this read the wrong file")
	}
	reasons := NoticeReasons()
	if len(reasons) != len(declared) {
		t.Fatalf("NoticeReasons() lists %d reasons and notice.go declares %d: %v", len(reasons), len(declared), declared)
	}
	for i, reason := range reasons {
		if want := NoticeReason(i + 1); reason != want {
			t.Errorf("NoticeReasons()[%d] = %d, want %d", i, reason, want)
		}
	}
}

// TestEveryNoticeReasonSaysItsOwnName: a log line or a failure that prints a
// reason prints its name, and every reason the set lists has one of its own.
// A reason added and left unnamed would print as a number, which nobody reading
// a log has the constant block open beside.
func TestEveryNoticeReasonSaysItsOwnName(t *testing.T) {
	t.Parallel()

	named := make(map[string]NoticeReason)
	for _, reason := range NoticeReasons() {
		name := reason.String()
		if name == "" || strings.HasPrefix(name, "notice_") {
			t.Errorf("reason %d is named %q, which is no name of its own", reason, name)
		}
		if other, taken := named[name]; taken {
			t.Errorf("reasons %d and %d are both named %q", other, reason, name)
		}
		named[name] = reason
	}
	if len(named) == 0 {
		t.Fatal("the set lists no reason, so nothing was named")
	}
	if got := NoticeReason(0).String(); !strings.HasPrefix(got, "notice_") {
		t.Errorf("the zero reason is named %q, want it to read as no reason", got)
	}
}
