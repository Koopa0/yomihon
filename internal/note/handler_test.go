package note_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/kurodo/internal/graph"
	"github.com/koopa0/kurodo/internal/nav"
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
	navModel, err := nav.Build(root, idx)
	if err != nil {
		t.Fatalf("nav.Build(%q) = %v", root, err)
	}
	h := note.NewHandler(note.Deps{
		Root:       root,
		Renderer:   render.New(root, idx),
		Status:     svc,
		Nav:        func() *nav.Model { return navModel },
		Counts:     func() map[string]int { return nil },
		Provenance: func(context.Context, string) (string, error) { return "", nil },
		Log:        slog.New(slog.DiscardHandler),
	})
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
		"ui-status--draft", // the note's status, rendered as its badge
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

// TestShowTTSGatedToLessons is the landmine guard for the TTS lesson gate: a
// lesson note's ruby paragraphs get a speak button, but a non-lesson note that
// contains the identical ruby does NOT — render.HTML is generic and the gate
// lives in the handler's type branch, so TTS must never leak into other note
// types. The ruby markup itself must survive in both (only the wrapper is gated).
func TestShowTTSGatedToLessons(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const body = "<ruby>今日<rt>きょう</rt></ruby>は晴れ。\n"

	lessonDir := filepath.Join(root, "Writing", "lessons", "japanese")
	if err := os.MkdirAll(lessonDir, 0o750); err != nil {
		t.Fatalf("mkdir lesson: %v", err)
	}
	if err := os.WriteFile(filepath.Join(lessonDir, "L00.md"),
		[]byte("---\ntitle: L00\ntype: lesson\nstatus: draft\n---\n\n"+body), 0o644); err != nil {
		t.Fatalf("write lesson: %v", err)
	}

	srcDir := filepath.Join(root, "Sources")
	if err := os.MkdirAll(srcDir, 0o750); err != nil {
		t.Fatalf("mkdir source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "S00.md"),
		[]byte("---\ntitle: S00\ntype: source-note\nstatus: draft\n---\n\n"+body), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	srv := newServer(t, root)

	_, lessonBody := get(t, srv.URL+"/notes/Writing/lessons/japanese/L00.md")
	if !strings.Contains(lessonBody, `data-tts="今日は晴れ。"`) {
		t.Errorf("lesson page missing the TTS button with reading-stripped data-tts; body = %q", lessonBody)
	}

	_, sourceBody := get(t, srv.URL+"/notes/Sources/S00.md")
	if strings.Contains(sourceBody, "data-tts") || strings.Contains(sourceBody, "k-tts") {
		t.Errorf("non-lesson (source-note) page leaked a TTS button — the lesson gate failed; body = %q", sourceBody)
	}
	if !strings.Contains(sourceBody, "<ruby>今日<rt>きょう</rt></ruby>") {
		t.Errorf("non-lesson page lost its ruby (render must stay generic); body = %q", sourceBody)
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

// TestHome pins the home route's contract. The navigation sidebar (spec §2)
// already ships on every /notes page (see TestShowIncludesSidebar); the
// Vault-Index home page (spec §2's four blocks) is deliberately deferred, so
// until it lands GET / is a placeholder that redirects (302) to README.md.
// A regression that changed the target, changed the status code, dropped the
// route, or "re-implemented" home as something else is caught here.
func TestHome(t *testing.T) {
	t.Parallel()
	srv := newServer(t, t.TempDir())

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/", http.NoBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	// Assert on the 302 itself — do not let the client follow it.
	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		t.Errorf("GET / status = %d, want %d", resp.StatusCode, http.StatusFound)
	}
	if got := resp.Header.Get("Location"); got != "/notes/README.md" {
		t.Errorf("GET / Location = %q, want %q", got, "/notes/README.md")
	}
}

// TestShowNoFrontmatter exercises handler.go's NoFrontmatter branch with a
// loaded (non-nil) contract, so WriteClosed is false and note.templ's
// statusPanel cannot fall into the "Contract unavailable" fail-closed case first —
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
	if !strings.Contains(body, "No frontmatter") {
		t.Errorf("page missing the no-frontmatter notice; body = %q", body)
	}
	if strings.Contains(body, "Contract unavailable") || strings.Contains(body, "fail-closed") {
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
	if strings.Contains(body, "Contract unavailable") || strings.Contains(body, "fail-closed") || strings.Contains(body, "No frontmatter") {
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
	note.NewHandler(note.Deps{
		Root:       root,
		Renderer:   render.New(root, idx),
		Status:     nil, // the nil under test
		Nav:        func() *nav.Model { return &nav.Model{} },
		Counts:     func() map[string]int { return nil },
		Provenance: func(context.Context, string) (string, error) { return "", nil },
		Log:        slog.New(slog.DiscardHandler),
	})
}

// TestNewHandlerPanicsOnNilNavigation mirrors the StatusPolicy check: a
// provider returning an empty *nav.Model is valid (an empty sidebar renders,
// §0.1), but a nil provider is a wiring bug that must fail at construction, not
// three calls deep inside the first reading request. Under D25 the sidebar is
// read live per request through this closure (never a captured startup value),
// so the guard is on the closure being present, not on the model it returns.
func TestNewHandlerPanicsOnNilNavigation(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("NewHandler(nil navigation provider) did not panic")
		}
	}()
	root := t.TempDir()
	idx, err := graph.Build(root)
	if err != nil {
		t.Fatalf("graph.Build(%q) = %v", root, err)
	}
	note.NewHandler(note.Deps{
		Root:       root,
		Renderer:   render.New(root, idx),
		Status:     status.NewService(root, nil),
		Nav:        nil, // the nil under test
		Counts:     func() map[string]int { return nil },
		Provenance: func(context.Context, string) (string, error) { return "", nil },
		Log:        slog.New(slog.DiscardHandler),
	})
}

// TestShowIncludesSidebar is the navigation-face regression: the reading
// page must still render AND now carry the plain sidebar — a known
// lifecycle folder, a study-path title, and a resolvable lesson link with
// its status badge. It builds a tiny vault with one syllabus and one lesson
// so the assertions are hand-derived, not tautological.
func TestShowIncludesSidebar(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	lessonDir := filepath.Join(root, "Writing", "lessons", "golang")
	if err := os.MkdirAll(lessonDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	lesson := "---\ntitle: Slices\ntype: lesson\ndomain: golang\nstatus: draft\n---\n\nbody\n"
	if err := os.WriteFile(filepath.Join(lessonDir, "Slices.md"), []byte(lesson), 0o644); err != nil {
		t.Fatalf("write lesson: %v", err)
	}

	mapsDir := filepath.Join(root, "Maps")
	if err := os.MkdirAll(mapsDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	syllabus := "---\ntitle: Go path\ntype: study-path\ndomain: golang\n---\n\n" +
		"## data | Data | 資料\n\n### text | Text | 文字\n\n- [[Slices]]\n"
	if err := os.WriteFile(filepath.Join(mapsDir, "Go path.md"), []byte(syllabus), 0o644); err != nil {
		t.Fatalf("write syllabus: %v", err)
	}

	srv := newServer(t, root)

	code, body := get(t, srv.URL+"/notes/Maps/Go path.md")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	for _, want := range []string{
		`class="k-rail-left"`, // the nav rail rendered at all
		"Writing",             // a lifecycle folder in the collapsed Folders tree
		"Go path",             // the study-path title
		"Data",                // the pipe-format H2's English label
		`href="/notes/Writing/lessons/golang/Slices.md"`, // the resolved lesson link
		"draft", // the lesson's status badge
	} {
		if !strings.Contains(body, want) {
			t.Errorf("reading page sidebar missing %q; body = %q", want, body)
		}
	}
}
