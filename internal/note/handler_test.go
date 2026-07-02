package note_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/kurodo/internal/graph"
	"github.com/koopa0/kurodo/internal/note"
	"github.com/koopa0/kurodo/internal/render"
	"github.com/koopa0/kurodo/internal/schema"
	"github.com/koopa0/kurodo/internal/status"
)

// newServer wires the reading page against a real (not faked — testing.md's
// "real first") status.Service, with a nil contract (fail-closed). Good
// enough for tests whose point is that the page renders regardless of
// whether the write face is available (§0.1) — NOT for exercising
// handler.go's NoFrontmatter/Transitions branch selection, since a
// fail-closed Service makes WriteClosed true and note.templ's statusPanel
// switches on WriteClosed first, before either of those ever matters. Use
// newServerWithContract for anything that needs to distinguish them.
func newServer(t *testing.T, root string) *httptest.Server {
	t.Helper()
	return newServerWithContract(t, root, nil)
}

// newServerWithContract is newServer with an explicit contract, so tests
// can put the write face in its non-fail-closed state (WriteClosed ==
// false) and actually observe which of NoFrontmatter / Transitions /
// "no legal transitions" handler.go's show() selected.
func newServerWithContract(t *testing.T, root string, contract *schema.Schema) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	svc := status.NewService(root, contract)
	idx, err := graph.Build(root)
	if err != nil {
		t.Fatalf("graph.Build(%q) = %v", root, err)
	}
	h := note.NewHandler(root, render.New(root, idx), svc, slog.New(slog.DiscardHandler))
	h.Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// loadContract is a loader fixture, not a second schema (wall 3): it reuses
// schema.LoadFile as-is against a lesson-only slice of the real contract
// shape (testdata/contract.toml), mirroring internal/status/status_test.go.
func loadContract(t *testing.T) *schema.Schema {
	t.Helper()
	s, err := schema.LoadFile(filepath.Join("testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("LoadFile(testdata/contract.toml) = %v", err)
	}
	return s
}

func get(t *testing.T, url string) (code int, body string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, http.NoBody)
	if err != nil {
		t.Fatalf("new request %s: %v", url, err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, string(b)
}

func TestShow(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	lesson := "---\ntitle: L00 テスト課\ntype: lesson\nstatus: draft\n---\n\n<ruby>今日<rt>きょう</rt></ruby>は<ruby>晴<rt>は</rt></ruby>れ。\n"
	dir := filepath.Join(root, "Writing", "lessons", "japanese")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "L00 テスト課.md"), []byte(lesson), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	srv := newServer(t, root)

	code, body := get(t, srv.URL+"/notes/Writing/lessons/japanese/L00 テスト課.md")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	for _, want := range []string{
		"L00 テスト課",
		"status: draft",
		"<ruby>今日<rt>きょう</rt></ruby>",
		// The status service is wired but fail-closed (nil contract):
		// the page must still render, with the write face's own notice
		// instead of any transition form (§0.1 asymmetric fault
		// tolerance — a missing contract never breaks reading).
		"fail-closed",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page missing %q", want)
		}
	}
}

func TestShowNotFound(t *testing.T) {
	t.Parallel()
	srv := newServer(t, t.TempDir())

	code, _ := get(t, srv.URL+"/notes/nope.md")
	if code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", code)
	}
}

// TestShowNoFrontmatter exercises handler.go's NoFrontmatter branch with a
// loaded (non-nil) contract, so WriteClosed is false and note.templ's
// statusPanel cannot fall into the "契約不可用" fail-closed case first —
// the only way to actually observe that show() set view.NoFrontmatter
// instead of leaving it false.
func TestShowNoFrontmatter(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, "Drills")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// No frontmatter block at all — legal per the contract's
	// no_frontmatter_is_legal (e.g. drills).
	if err := os.WriteFile(filepath.Join(dir, "d1.md"), []byte("just a drill body\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	srv := newServerWithContract(t, root, loadContract(t))

	code, body := get(t, srv.URL+"/notes/Drills/d1.md")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if !strings.Contains(body, "無 frontmatter") {
		t.Errorf("page missing the no-frontmatter notice; body = %q", body)
	}
	if strings.Contains(body, "契約不可用") || strings.Contains(body, "fail-closed") {
		t.Errorf("page shows the fail-closed notice even though the contract loaded; body = %q", body)
	}
}

// TestShowTransitions exercises handler.go's default branch (view.Transitions
// = h.statusSvc.Transitions(n.Type(), n.Status())) with a loaded contract.
// Getting the argument order backwards (Transitions(current, noteType)) or
// swapping the switch's case order would silently render the wrong panel —
// this test is the only one in the repo that would catch either.
func TestShowTransitions(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	lesson := "---\ntitle: L01\ntype: lesson\ndomain: japanese\nstatus: draft\ncreated: 2026-06-01\nupdated: 2026-06-01\n---\n\nbody\n"
	dir := filepath.Join(root, "Writing", "lessons", "japanese")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "L01.md"), []byte(lesson), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	srv := newServerWithContract(t, root, loadContract(t))

	code, body := get(t, srv.URL+"/notes/Writing/lessons/japanese/L01.md")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	// draft -> [ready, archived] for actor "koopa" per
	// testdata/contract.toml's lifecycle table (cross-checked by hand,
	// mirroring internal/status/status_test.go's TestTransitions).
	for _, want := range []string{`value="ready"`, `value="archived"`} {
		if !strings.Contains(body, want) {
			t.Errorf("page missing transition key %s; body = %q", want, body)
		}
	}
	if strings.Contains(body, "契約不可用") || strings.Contains(body, "fail-closed") || strings.Contains(body, "無 frontmatter") {
		t.Errorf("page shows the wrong status-panel branch; body = %q", body)
	}
}

// TestNewHandlerPanicsOnNilStatusPolicy mirrors
// internal/status/handler_test.go's coverage of status.NewHandler's own
// nil-dependency panic: a fail-closed *status.Service is a valid
// StatusPolicy (Closed() reports true), but a literal nil is not — a
// future caller passing one must fail at wiring time, not three calls deep
// inside the first GET /notes/... request.
func TestNewHandlerPanicsOnNilStatusPolicy(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("NewHandler(nil StatusPolicy) did not panic")
		}
	}()
	root := t.TempDir()
	idx, err := graph.Build(root)
	if err != nil {
		t.Fatalf("graph.Build(%q) = %v", root, err)
	}
	note.NewHandler(root, render.New(root, idx), nil, slog.New(slog.DiscardHandler))
}
