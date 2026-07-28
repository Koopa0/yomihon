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
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/syllabus"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/vault"
)

// newServer builds a real nav.Model from a temp vault (real-first: no fakes)
// and wires the study-path handler behind it.
func newServer(t *testing.T, root string) *httptest.Server {
	t.Helper()
	model := loadModel(t, root)
	mux := http.NewServeMux()
	syllabus.New(func() pages.Shell { return pages.Shell{Nav: model} }, slog.New(slog.DiscardHandler)).Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func loadModel(t *testing.T, root string) *nav.Model {
	t.Helper()
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Errorf("Reader.Close() error = %v", closeErr)
		}
	})
	scan, err := reader.ScanComplete(t.Context())
	if err != nil {
		t.Fatalf("ScanComplete() error = %v", err)
	}
	notes := make(map[string]*vault.Note)
	noteList := make([]*vault.Note, 0, len(scan.Files()))
	resources := make([]string, 0, len(scan.Files()))
	for _, entry := range scan.Files() {
		if !strings.HasSuffix(entry.Path(), ".md") {
			resources = append(resources, entry.Path())
			continue
		}
		data, readErr := reader.ReadFile(t.Context(), entry)
		if readErr != nil {
			t.Fatalf("ReadFile() error = %v", readErr)
		}
		note := vault.Parse(entry.Path(), data)
		notes[entry.Path()] = note
		noteList = append(noteList, note)
	}
	idx := graph.New(noteList, resources)
	contract, err := schema.LoadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("schema.LoadFile = %v", err)
	}
	model := nav.New(scan.Files(), notes, idx, contract.NavigationRoles(), contract.KnowledgeScope(), contract.ArtifactPolicy())
	return model
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
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("close response body: %v", closeErr)
		}
	}()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, string(b)
}

// writeVault lays down one study-path with, in order, a resolved lesson, an
// unresolved row, an ambiguous same-basename row, and a trailing resolved
// lesson. The ambiguity candidates remain real, browsable files; only the
// study-path row must refuse to guess between them.
func writeVault(t *testing.T, root string) {
	t.Helper()

	lessonDir := filepath.Join(root, "Writing", "lessons", "golang")
	if err := os.MkdirAll(lessonDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	lesson := "---\ntitle: Slices\ntype: lesson\ndomain: golang\nstatus: ready\n---\n\nbody\n"
	if err := os.WriteFile(filepath.Join(lessonDir, "Slices.md"), []byte(lesson), 0o600); err != nil {
		t.Fatalf("write lesson: %v", err)
	}
	after := "---\ntitle: After\ntype: lesson\ndomain: golang\nstatus: draft\n---\n\nbody\n"
	if err := os.WriteFile(filepath.Join(lessonDir, "After.md"), []byte(after), 0o600); err != nil {
		t.Fatalf("write trailing lesson: %v", err)
	}
	for _, rel := range []string{"A/Repeated.md", "B/Repeated.md"} {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir ambiguous candidate %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte("candidate\n"), 0o600); err != nil {
			t.Fatalf("write ambiguous candidate %s: %v", rel, err)
		}
	}

	mapsDir := filepath.Join(root, "Maps")
	if err := os.MkdirAll(mapsDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := "---\ntitle: Go path\ntype: study-path\ndomain: golang\n---\n\n" +
		"## data | Data | 資料\n\n### text | Text | 文字\n\n" +
		"- [[Slices]]\n- [[Ghost Lesson]]\n- [[Repeated|Ambiguous Lesson]]\n- [[After]]\n"
	if err := os.WriteFile(filepath.Join(mapsDir, "Go path.md"), []byte(path), 0o600); err != nil {
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
	main := syllabusMain(t, body)
	for _, want := range []string{
		`class="y-shell2"`, // the study-path shell rendered
		"學習路徑",             // the switcher label outside main
		"本路徑",              // the part jump-nav outside main
	} {
		if !strings.Contains(body, want) {
			t.Errorf("study-path shell missing %q; body = %q", want, body)
		}
	}
	for _, want := range []string{
		`<h1 class="y-title">Go path</h1>`,               // the path title
		`<h2 class="y-part" id="part-1">`,                // the pipe-format H2 is a real page heading
		`<span class="y-part__name">Data</span>`,         // the pipe-format H2's English label
		`<h3 class="y-module__label">`,                   // the nested module preserves the heading hierarchy
		`<span class="y-module__name">Text</span>`,       // the module heading
		`href="/notes/Writing/lessons/golang/Slices.md"`, // the resolved lesson is a link
		`href="/notes/Writing/lessons/golang/After.md"`,  // the row after the ambiguity keeps its place
		`>Ghost Lesson</span>`,                           // the unwritten row keeps its sequence position
		`data-resolution="unresolved"`,                   // the row names why it cannot link
		`>Ambiguous Lesson</span>`,
		`<span class="y-lesson y-lesson--broken" data-resolution="ambiguous" title="目標有歧義"><span class="y-navmark y-navmark--warn" aria-hidden="true">!</span>`,
		`>有歧義</span>`,
		"y-navmark--warn", // warning is visible without color alone
	} {
		if !strings.Contains(main, want) {
			t.Errorf("study-path content missing %q; main = %q", want, main)
		}
	}
	if strings.Contains(main, `href="/notes/"`) {
		t.Errorf("study-path content fabricates an empty note link; main = %q", main)
	}
	for _, candidate := range []string{`href="/notes/A/Repeated.md"`, `href="/notes/B/Repeated.md"`} {
		if strings.Contains(main, candidate) {
			t.Errorf("study-path content guesses ambiguous candidate %q; main = %q", candidate, main)
		}
	}
	order := []string{"Slices</span>", "Ghost Lesson</span>", "Ambiguous Lesson</span>", "After</span>"}
	previous := -1
	for _, marker := range order {
		at := strings.Index(main, marker)
		if at < 0 || at <= previous {
			t.Errorf("study-path row %q at %d after %d, want original order", marker, at, previous)
		}
		previous = at
	}
}

// syllabusMain returns only the study-path projection. Candidate notes may
// legitimately be linked by other page navigation; those links do not mean the
// ambiguous curriculum row guessed a target.
func syllabusMain(t *testing.T, body string) string {
	t.Helper()
	const opening = `<main id="main-content" tabindex="-1" class="y-main">`
	_, after, ok := strings.Cut(body, opening)
	if !ok {
		t.Fatalf("syllabus response missing %q; body = %q", opening, body)
	}
	main, _, ok := strings.Cut(after, "</main>")
	if !ok {
		t.Fatalf("syllabus response has no closing main element; body = %q", body)
	}
	return main
}

func TestShowReadsOneShellSnapshot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeVault(t, root)
	model := loadModel(t, root)
	calls := 0
	mux := http.NewServeMux()
	syllabus.New(func() pages.Shell {
		calls++
		return pages.Shell{Nav: model, Advanceable: 3, AdvanceableKnown: true}
	}, slog.New(slog.DiscardHandler)).Register(mux)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/syllabus/Maps/Go%20path.md", http.NoBody))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if calls != 1 {
		t.Errorf("shell snapshot reads = %d, want 1", calls)
	}
	if !strings.Contains(rr.Body.String(), `aria-label="3 篇筆記的狀態還有下一步"`) {
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
			t.Fatal("New(nil Shell) did not panic")
		}
	}()
	syllabus.New(nil, slog.New(slog.DiscardHandler))
}
