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
		Nav: func() *nav.Model { return model },
		Log: slog.New(slog.DiscardHandler),
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

// writeVault lays down one study-path with a resolvable lesson (ready) and a
// broken lesson (no such note), so the assertions below are hand-derived: the
// resolved lesson must be a link, the broken one must still be listed + marked.
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
		"Ghost Lesson", // the broken lesson is STILL listed
		"unresolved",   // ...and marked, never silently dropped
	} {
		if !strings.Contains(body, want) {
			t.Errorf("study-path page missing %q; body = %q", want, body)
		}
	}
	// The broken lesson must NOT be a link to a note that does not exist.
	if strings.Contains(body, `href="/notes/Ghost Lesson`) {
		t.Errorf("broken lesson rendered as a link; body = %q", body)
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

// TestNewHandlerPanicsOnNilNav mirrors internal/note's nil-dependency coverage:
// a provider returning an empty *nav.Model is valid, but a nil provider is a
// wiring bug that must fail at construction, not on the first request.
func TestNewHandlerPanicsOnNilNav(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("NewHandler(nil Nav) did not panic")
		}
	}()
	syllabus.NewHandler(syllabus.Deps{Nav: nil, Log: slog.New(slog.DiscardHandler)})
}
