package search

import (
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/lexical"
	"github.com/koopa0/yomihon/internal/nav"
)

// The count and the marked terms are both things a reader looked for and did
// not find: four asked how many results there were, five asked why a given
// result was in the list. Both are asserted on the served page rather than on
// the functions that produce them, because a correct function nothing calls
// reaches nobody — the first version of the marking was wired through a field
// the page never read, and every unit test still passed.
func TestServedResultsCarryTheCountAndTheMarks(t *testing.T) {
	t.Parallel()

	idx := lexical.NewIndex([]lexical.Document{
		{RelPath: "notes/Kafka.md", Title: "Kafka Basics", PlainText: "kafka is a distributed log"},
		{RelPath: "notes/Streams.md", Title: "Streams", PlainText: "kafka streams build on the log"},
	}, validArtifactPolicy(t))
	mux := http.NewServeMux()
	NewHandler(func() RequestSnapshot {
		return RequestSnapshot{Index: idx, Shell: nav.Shell{Nav: &nav.Model{}, Governed: true}}
	}, slog.New(slog.DiscardHandler)).Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	code, body := getBody(t, srv.Client(), srv.URL+"/search?q=kafka")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}

	// Rendered by the server, so it is there for a reader with no scripting at
	// all — where it used to live, in the live region, it was there for neither.
	if !strings.Contains(body, "共 2 筆") {
		t.Errorf("results page does not say how many were found; body = %q", body)
	}
	if n := strings.Count(body, "<mark>kafka</mark>"); n == 0 {
		t.Errorf("results page marks none of the searched term; body = %q", body)
	}
	// The mark must not have eaten or duplicated the note's own words.
	if !strings.Contains(body, "is a distributed log") {
		t.Errorf("results page lost snippet text around the mark; body = %q", body)
	}

	// The fragment the live search replaces the list with has to agree with the
	// page, or turning JavaScript on quietly removes both.
	fragCode, frag := getBody(t, srv.Client(), srv.URL+"/search/results?q=kafka")
	if fragCode != http.StatusOK {
		t.Fatalf("fragment status = %d, want 200", fragCode)
	}
	for _, want := range []string{"共 2 筆", "<mark>kafka</mark>"} {
		if !strings.Contains(frag, want) {
			t.Errorf("live-search fragment missing %q; body = %q", want, frag)
		}
	}
}

// The loosened offers appear exactly where the reader hits the wall — the
// empty answer — and nowhere else. A page with results carrying "step back"
// advice would be the page second-guessing an answer it just gave.
func TestServedStepBacksAppearOnlyOnTheEmptyAnswer(t *testing.T) {
	t.Parallel()

	idx := lexical.NewIndex([]lexical.Document{
		{RelPath: "臨床/利尿劑調整.md", Title: "利尿劑", PlainText: "Furosemide 起始 20-40mg"},
	}, validArtifactPolicy(t))
	mux := http.NewServeMux()
	NewHandler(func() RequestSnapshot {
		return RequestSnapshot{Index: idx, Shell: nav.Shell{Nav: &nav.Model{}, Governed: true}}
	}, slog.New(slog.DiscardHandler)).Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	code, body := getBody(t, srv.Client(), srv.URL+"/search?q=20mg")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if !strings.Contains(body, "退一步找") || !strings.Contains(body, "20+mg") {
		t.Errorf("empty answer does not offer the loosened search; body = %q", body)
	}

	// This query has results AND would generate a loosened candidate — the
	// only shape that can tell "the gate held" apart from "there was nothing
	// to offer anyway".
	code, body = getBody(t, srv.Client(), srv.URL+"/search?q=20-40mg")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if !strings.Contains(body, "共 1 筆") {
		t.Fatalf("the with-results probe found nothing, so it can prove nothing; body = %q", body)
	}
	if strings.Contains(body, "退一步找") {
		t.Errorf("an answer with results still carries step-back advice; body = %q", body)
	}
}

// Both search faces answer one query, and reading it is the one expensive thing
// they do: the parse allocates, and it happened twice per request because the
// function that ran the query threw its parse away and the function that built
// the view needed one back. Nothing said so — two correct answers, arrived at
// twice.
//
// What holds it is where the parse is written, so that is what this reads. A
// benchmark would measure it and a test that called both would not notice; a
// second parse anywhere but the query itself is the defect returning.
func TestOneQueryIsReadOnce(t *testing.T) {
	t.Parallel()

	// The whole package, not one file: a second read moved into a new file
	// beside this one is the same defect, and reading only the file it lives in
	// today would be a check that holds until somebody splits a function out.
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read the package directory: %v", err)
	}
	parsesIn := map[string]int{}
	sources := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || filepath.Ext(name) != ".go" || strings.HasSuffix(name, "_test.go") {
			continue
		}
		sources++
		file, parseErr := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", name, parseErr)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				selector, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || selector.Sel.Name != "Parse" {
					return true
				}
				if pkg, ok := selector.X.(*ast.Ident); ok && pkg.Name == "lexical" {
					parsesIn[fn.Name.Name]++
				}
				return true
			})
		}
	}
	if sources == 0 {
		t.Fatal("no source file was read, so this check would pass over any package")
	}

	// Two places read a query, and the second is not on the path a reader
	// waits on: a fault being logged parses the text again to say what shape it
	// was, which costs nothing on a request that already failed.
	mayRead := map[string]string{
		"query":      "answering the reader",
		"queryFacts": "describing a query in a log line, on the failure path",
	}
	total := 0
	for name, count := range parsesIn {
		total += count
		if _, allowed := mayRead[name]; !allowed {
			t.Errorf("%s reads the query itself; answering one takes one read, in query", name)
			continue
		}
		if count > 1 {
			t.Errorf("%s reads the query %d times, want once", name, count)
		}
	}
	for name, why := range mayRead {
		if parsesIn[name] == 0 {
			t.Errorf("%s no longer reads the query, so the allowance for it (%s) is watching a name that moved", name, why)
		}
	}
	if total == 0 {
		t.Fatal("no call to lexical.Parse was found in this package, so this test is watching code that moved")
	}
}
