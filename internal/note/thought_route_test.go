package note_test

import (
	"html"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/note"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/wording"
)

func TestThoughtRouteOffersOrdinaryDocumentsAndLessonsTheSameDoor(t *testing.T) {
	t.Parallel()
	for _, noteType := range []string{"", "lesson"} {
		t.Run(noteType, func(t *testing.T) {
			t.Parallel()
			mux, root := thoughtRouteFixture(t, true, noteType)
			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/thought/Source.md?section=chapter-one", http.NoBody))
			if recorder.Code != http.StatusOK {
				t.Fatalf("thought route status = %d, body %s", recorder.Code, recorder.Body.String())
			}
			markdown := thoughtTemplate(t, recorder.Body.String())
			const want = "---\ntype: \"writing\"\nstatus: \"draft\"\ndomain: \"japanese\"\nbased_on: \"[[Source#Chapter one]]\"\n---\n"
			if markdown != want {
				t.Errorf("offered Markdown = %q, want %q", markdown, want)
			}
			if _, err := os.Stat(filepath.Join(root, "Source-thought.md")); !os.IsNotExist(err) {
				t.Errorf("opening the door created a thought file: %v", err)
			}
			if strings.Contains(recorder.Body.String(), `<img src=x onerror=alert(1)>`) {
				t.Error("source title reached the page as executable HTML")
			}
		})
	}
}

// thoughtNamePattern is the shape of the suggested file name for the source in
// thoughtRouteHandler: the title, the word for a thought, and the minute. The
// minute itself is the clock's, so the test pins its shape and not its value.
var thoughtNamePattern = regexp.MustCompile(`^img src=x onerror=alert\(1\)的想法 \d{4}-\d{2}-\d{2} \d{4}$`)

func TestThoughtRouteOffersObsidianOnlyInAnObsidianVault(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		obsidian bool
	}{
		{name: "an Obsidian vault", obsidian: true},
		{name: "a plain folder"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mux, root := thoughtRouteFixture(t, true, "")
			if tc.obsidian {
				if err := os.Mkdir(filepath.Join(root, ".obsidian"), 0o750); err != nil {
					t.Fatal(err)
				}
			}
			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/thought/Source.md", http.NoBody))
			if recorder.Code != http.StatusOK {
				t.Fatalf("thought route status = %d, body %s", recorder.Code, recorder.Body.String())
			}
			page := recorder.Body.String()
			if !strings.Contains(page, "data-thought-copy") {
				t.Error("copying is always an action")
			}
			link := regexp.MustCompile(`href="(obsidian://new[^"]*)"`).FindStringSubmatch(page)
			if !tc.obsidian {
				if link != nil {
					t.Errorf("a vault without .obsidian was offered %q", link[1])
				}
				return
			}
			if link == nil {
				t.Fatal("an Obsidian vault was not offered the Obsidian action")
			}
			parsed, err := url.Parse(html.UnescapeString(link[1]))
			if err != nil {
				t.Fatal(err)
			}
			query := parsed.Query()
			if len(query) != 3 || query.Get("vault") != filepath.Base(root) {
				t.Errorf("new-note URI carries %v, want exactly the vault %q, a name and the content", query, filepath.Base(root))
			}
			if name := query.Get("name"); !thoughtNamePattern.MatchString(name) {
				t.Errorf("file name %q does not read as a thought on the source", name)
			}
			if !strings.Contains(query.Get("content"), `based_on: "[[Source]]"`) {
				t.Errorf("content = %q, want it to cite the source", query.Get("content"))
			}
			if strings.Index(page, "data-thought-copy") < strings.Index(page, "obsidian://new") {
				t.Error("the Obsidian action must come before the copy action")
			}
		})
	}
}

func TestThoughtRoutePageLeadsWithIntent(t *testing.T) {
	t.Parallel()
	mux, _ := thoughtRouteFixture(t, true, "")
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/thought/Source.md", http.NoBody))
	page := recorder.Body.String()
	last := -1
	for _, part := range []string{"<h1", `href="/notes/Source.md`, wording.ThoughtExternal.In(wording.ZhHant), "<pre", "y-thought__actions"} {
		at := strings.Index(page, part)
		if at < 0 || at < last {
			t.Fatalf("%q is missing or out of order (at %d, after %d); the order is title, source, sentence, template, actions", part, at, last)
		}
		last = at
	}
	if strings.Contains(page, "<textarea") {
		t.Error("the template is read-only text and is not drawn as a form field")
	}
}

func TestThoughtRouteRejectsUnavailableAuthorityAndInvalidSections(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		answer  bool
		request string
	}{
		{name: "absent role", request: "/thought/Source.md"},
		{name: "unknown section", answer: true, request: "/thought/Source.md?section=not-a-section"},
		{name: "missing file", answer: true, request: "/thought/Missing.md"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mux, _ := thoughtRouteFixture(t, tc.answer, "")
			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, tc.request, http.NoBody))
			if recorder.Code != http.StatusNotFound {
				t.Errorf("thought route status = %d, want 404", recorder.Code)
			}
			if strings.Contains(recorder.Body.String(), "data-thought-markdown") {
				t.Error("unavailable thought door still offered Markdown")
			}
		})
	}
}

func TestHasPlaceAcceptsOnlyRenderedAnchorsOfReadableNotes(t *testing.T) {
	t.Parallel()
	handler, _ := thoughtRouteHandler(t, false, "")
	for _, tc := range []struct {
		rel, anchor string
		want        bool
	}{
		{"Source.md", "", true},
		{"Source.md", "chapter-one", true},
		{"Source.md", "not-a-section", false},
		{"Source.md", "slot-pattern-1", false},
		{"Missing.md", "", false},
		{"Missing.md", "chapter-one", false},
		{"../Source.md", "", false},
	} {
		if got := handler.HasPlace(tc.rel, tc.anchor); got != tc.want {
			t.Errorf("HasPlace(%q, %q) = %v, want %v", tc.rel, tc.anchor, got, tc.want)
		}
	}
}

func thoughtRouteFixture(t *testing.T, declareAnswer bool, sourceType string) (mux http.Handler, root string) {
	t.Helper()
	handler, root := thoughtRouteHandler(t, declareAnswer, sourceType)
	routes := http.NewServeMux()
	handler.Register(routes)
	return routes, root
}

func thoughtRouteHandler(t *testing.T, declareAnswer bool, sourceType string) (handler *note.Handler, root string) {
	t.Helper()
	root = t.TempDir()
	content := "---\ntitle: '<img src=x onerror=alert(1)>'\ndomain: japanese\n"
	if sourceType != "" {
		content += "type: " + sourceType + "\n"
	}
	content += "---\n# Chapter one\nPROMPT AND EXPLANATION MUST NOT BE COPIED\n"
	if err := os.WriteFile(filepath.Join(root, "Source.md"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if declareAnswer {
		text = strings.Replace(text, "[navigation]\n", "[navigation]\nanswer_type = \"writing\"\n", 1)
	}
	contractPath := filepath.Join(t.TempDir(), "vault-schema.toml")
	if err = os.WriteFile(contractPath, []byte(text), 0o600); err != nil { // #nosec G703 -- a temporary file this test just named
		t.Fatal(err)
	}
	contract, err := schema.LoadFile(contractPath)
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.DiscardHandler)
	store, source := newSnapshotStore(t, root, log, contract, contract.Governance())
	writer := openStatusWriter(t, source, contract, contract.Governance())
	handler = note.New(&note.Sources{
		Source: source, Contract: contract, Snapshot: store.Current, Status: writer.Authority,
		ObservedStatus: writer.ObservedStatus, ConsumeReceipt: writer.ConsumeReceipt,
		Continuation: noMark, Log: log,
	})
	return handler, root
}

// thoughtTemplate returns the composed Markdown from the page's reading code
// block.
func thoughtTemplate(t *testing.T, body string) string {
	t.Helper()
	_, after, ok := strings.Cut(body, "<code data-thought-markdown>")
	if !ok {
		t.Fatal("thought page has no code block for the Markdown")
	}
	content, _, ok := strings.Cut(after, "</code></pre>")
	if !ok {
		t.Fatal("thought code block is not closed")
	}
	return html.UnescapeString(content)
}
