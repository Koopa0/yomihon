package syllabus_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/syllabus"
	"github.com/koopa0/yomihon/internal/ui/pages"
)

// newServer builds a real nav.Model from a temp vault (real-first: no fakes)
// and wires the study-path handler behind it.
func newServer(t *testing.T, root string) *httptest.Server {
	t.Helper()
	idx, err := graph.Build(root)
	if err != nil {
		t.Fatalf("graph.Build(%q) = %v", root, err)
	}
	model, err := nav.Build(root, idx, nil)
	if err != nil {
		t.Fatalf("nav.Build(%q) = %v", root, err)
	}
	mux := http.NewServeMux()
	syllabus.NewHandler(syllabus.Deps{
		Shell: func() pages.ShellData { return pages.ShellData{Nav: model} },
		Log:   slog.New(slog.DiscardHandler),
	}).Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
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

// writeVault lays down one study-path with a resolved lesson and an unwritten
// row, so the route proves navigation includes only the note that exists.
func writeVault(t *testing.T, root string) {
	t.Helper()

	lessonDir := filepath.Join(root, "Writing", "lessons", "golang")
	if err := os.MkdirAll(lessonDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	lesson := "---\ntitle: Slices\ntype: lesson\ndomain: golang\nstatus: ready\n---\n\nbody\n"
	if err := os.WriteFile(filepath.Join(lessonDir, "Slices.md"), []byte(lesson), 0o644); err != nil {
		t.Fatalf("write lesson: %v", err)
	}

	mapsDir := filepath.Join(root, "Maps")
	if err := os.MkdirAll(mapsDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := "---\ntitle: Go path\ntype: study-path\ndomain: golang\n---\n\n" +
		"## data | Data | 資料\n\n### text | Text | 文字\n\n- [[Slices]]\n- [[Ghost Lesson]]\n"
	if err := os.WriteFile(filepath.Join(mapsDir, "Go path.md"), []byte(path), 0o644); err != nil {
		t.Fatalf("write study-path: %v", err)
	}
}

func TestShow(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeVault(t, root)
	srv := newServer(t, root)

	code, body := get(t, srv.URL+"/syllabus/Maps/Go path.md")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	for _, want := range []string{
		`class="y-shell2"`, // the study-path shell rendered
		"Go path",          // the path title
		"Study paths",      // the switcher label
		"On this path",     // the part jump-nav
		"Data",             // the pipe-format H2's English label (a part)
		"Text",             // the module heading
		`href="/notes/Writing/lessons/golang/Slices.md"`, // the resolved lesson is a link
	} {
		if !strings.Contains(body, want) {
			t.Errorf("study-path page missing %q; body = %q", want, body)
		}
	}
	if strings.Contains(body, "Ghost Lesson") {
		t.Errorf("study-path page contains the unwritten map row; body = %q", body)
	}
}

func TestShowReadsOneShellSnapshot(t *testing.T) {
	t.Parallel()
	model := &nav.Model{Paths: []nav.Map{{Title: "P", RelPath: "Maps/P.md"}}}
	calls := 0
	mux := http.NewServeMux()
	syllabus.NewHandler(syllabus.Deps{
		Shell: func() pages.ShellData {
			calls++
			return pages.ShellData{Nav: model, Pending: 3, PendingKnown: true}
		},
		Log: slog.New(slog.DiscardHandler),
	}).Register(mux)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/syllabus/Maps/P.md", http.NoBody))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if calls != 1 {
		t.Errorf("shell snapshot reads = %d, want 1", calls)
	}
	if !strings.Contains(rr.Body.String(), `aria-label="3 to decide"`) {
		t.Errorf("response missing pending chip; body = %q", rr.Body.String())
	}
}

func TestShowNotFound(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeVault(t, root)
	srv := newServer(t, root)

	// A real note path, but not a study-path → 404 (this route serves only
	// study-paths; the note lives at /notes/...).
	code, _ := get(t, srv.URL+"/syllabus/Writing/lessons/golang/Slices.md")
	if code != http.StatusNotFound {
		t.Errorf("GET /syllabus/<a note> status = %d, want 404", code)
	}

	code, _ = get(t, srv.URL+"/syllabus/Maps/Nope.md")
	if code != http.StatusNotFound {
		t.Errorf("GET /syllabus/<missing> status = %d, want 404", code)
	}
}

// TestNewHandlerPanicsOnNilShell mirrors internal/note's nil-dependency coverage:
// a provider returning an empty shell is valid, but a nil provider is a
// wiring bug that must fail at construction, not on the first request.
func TestNewHandlerPanicsOnNilShell(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("NewHandler(nil Shell) did not panic")
		}
	}()
	syllabus.NewHandler(syllabus.Deps{Shell: nil, Log: slog.New(slog.DiscardHandler)})
}
