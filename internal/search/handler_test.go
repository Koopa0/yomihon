package search

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/lexical"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestSearchHandler exercises GET /search end to end: route registration, the
// live-index provider closure, query parsing, and the rendered results page.
func TestSearchHandler(t *testing.T) {
	t.Parallel()

	idx := lexical.NewIndex([]lexical.Document{
		{RelPath: "Writing/Kafka.md", Title: "Kafka Basics", NoteType: "lesson", Status: "draft", PlainText: "kafka is a distributed log"},
	}, validArtifactPolicy(t))
	mux := http.NewServeMux()
	NewHandler(func() RequestSnapshot {
		return RequestSnapshot{Index: idx, Shell: pages.Shell{Nav: &nav.Model{}, Governed: true}}
	}, slog.New(slog.DiscardHandler)).Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	t.Run("matching query renders the hit", func(t *testing.T) {
		t.Parallel()
		code, body := getBody(t, srv.URL+"/search?q=kafka")
		if code != http.StatusOK {
			t.Fatalf("status = %d, want 200", code)
		}
		// y-rail-left is the shared sidebar: the results page renders inside
		// the same shell as every other page, never a chromeless view.
		for _, want := range []string{"Kafka Basics", `href="/notes/Writing/Kafka.md"`, "draft", "y-rail-left"} {
			if !strings.Contains(body, want) {
				t.Errorf("search page missing %q; body = %q", want, body)
			}
		}
		searchPage := searchPageHTML(t, body)
		for _, want := range []string{
			`data-live-search-endpoint="/search/results"`,
			`data-live-search-form`,
			`data-live-search-input`,
			`data-live-search-status`,
			`role="status"`,
			`aria-live="polite"`,
			`aria-atomic="true"`,
		} {
			if !strings.Contains(searchPage, want) {
				t.Errorf("search page region missing %q; region = %q", want, searchPage)
			}
		}
	})

	t.Run("non-matching query shows no results", func(t *testing.T) {
		t.Parallel()
		code, body := getBody(t, srv.URL+"/search?q=zzznotpresent")
		if code != http.StatusOK {
			t.Fatalf("status = %d, want 200", code)
		}
		if !strings.Contains(body, "找不到") {
			t.Errorf(`search page missing the Traditional Chinese no-results message; body = %q`, body)
		}
		if strings.Contains(body, "data-search-diagnostic") {
			t.Errorf("ordinary no-results page rendered a capability diagnostic; body = %q", body)
		}
	})
}

func TestSearchHandlerMetadataDiagnostic(t *testing.T) {
	t.Parallel()

	undeclaredPolicy := undeclaredArtifactPolicy(t)
	invalidPolicy := invalidArtifactPolicy(t)
	incompletePolicy := incompleteArtifactPolicy(t)
	tests := []struct {
		name       string
		idx        *lexical.Index
		diagnostic string
	}{
		{name: "undeclared", idx: lexical.NewIndex([]lexical.Document{{RelPath: "Concepts/Note.md", Title: "Note", NoteType: "concept"}}, undeclaredPolicy), diagnostic: undeclaredPolicy.Diagnostic()},
		{name: "invalid", idx: lexical.NewIndex([]lexical.Document{{RelPath: "Concepts/Note.md", Title: "Note", NoteType: "concept"}}, invalidPolicy), diagnostic: invalidPolicy.Diagnostic()},
		{name: "incomplete", idx: lexical.NewIndex([]lexical.Document{{RelPath: "Concepts/Note.md", Title: "Note", NoteType: "concept"}}, incompletePolicy), diagnostic: incompletePolicy.Diagnostic()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := NewHandler(func() RequestSnapshot {
				return RequestSnapshot{Index: tt.idx, Shell: pages.Shell{Nav: &nav.Model{}}}
			}, slog.New(slog.DiscardHandler))
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/search?q=type:concept", http.NoBody)
			rr := httptest.NewRecorder()
			h.search(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rr.Code)
			}
			body := html.UnescapeString(rr.Body.String())
			for _, want := range []string{"data-search-diagnostic", tt.diagnostic, "y-rail-left"} {
				if !strings.Contains(body, want) {
					t.Errorf("search diagnostic page missing %q; body = %q", want, body)
				}
			}
			if strings.Contains(body, "找不到「") {
				t.Errorf("search capability diagnostic rendered ordinary no-results copy; body = %q", body)
			}
		})
	}
}

func TestSearchHandlerReadsOneRequestSnapshot(t *testing.T) {
	t.Parallel()
	idx := lexical.NewIndex([]lexical.Document{{RelPath: "Concepts/One.md", Title: "One", Status: "draft", PlainText: "needle"}}, validArtifactPolicy(t))
	calls := 0
	h := NewHandler(func() RequestSnapshot {
		calls++
		return RequestSnapshot{
			Index: idx,
			Shell: pages.Shell{Nav: &nav.Model{}, Governed: true},
		}
	}, slog.New(slog.DiscardHandler))
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/search?q=needle", http.NoBody)
	rr := httptest.NewRecorder()
	h.search(rr, req)
	if calls != 1 {
		t.Errorf("request snapshot reads = %d, want 1", calls)
	}
	// The governed flag rides on the shell this handler was handed and gates
	// the result row's status, so that word witnesses the shell rather than
	// one the renderer derived for itself.
	for _, want := range []string{"One", "draft"} {
		if !strings.Contains(rr.Body.String(), want) {
			t.Errorf("search response missing %q; body = %q", want, rr.Body.String())
		}
	}
}

func TestSearchResultsFragment(t *testing.T) {
	t.Parallel()

	idx := lexical.NewIndex([]lexical.Document{
		{RelPath: "Writing/Kafka.md", Title: "Kafka Basics", Status: "draft", PlainText: "kafka is a distributed log"},
	}, validArtifactPolicy(t))
	mux := http.NewServeMux()
	NewHandler(func() RequestSnapshot {
		return RequestSnapshot{Index: idx, Shell: pages.Shell{Nav: &nav.Model{}}}
	}, slog.New(slog.DiscardHandler)).Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	code, header, body := getBodyWithHeaders(t, srv.URL+"/search/results?q=kafka")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	for name, want := range map[string]string{
		"Content-Type":           "text/html; charset=utf-8",
		"Cache-Control":          "no-store",
		"X-Content-Type-Options": "nosniff",
	} {
		if got := header.Get(name); got != want {
			t.Errorf("search results %s = %q, want %q", name, got, want)
		}
	}
	for _, want := range []string{`data-live-search-results`, `data-result-count="1"`, "Kafka Basics"} {
		if !strings.Contains(body, want) {
			t.Errorf("search results fragment missing %q; body = %q", want, body)
		}
	}
	for _, absent := range []string{"<!DOCTYPE html>", "y-rail-left"} {
		if strings.Contains(body, absent) {
			t.Errorf("search results fragment unexpectedly contains %q; body = %q", absent, body)
		}
	}
}

// TestSearchHitStatusChipFollowsGovernance pins C7's separation. The index
// happily matches and returns a note's raw frontmatter status for a folder no
// contract governs — that is a text field like any other. Rendering it as a
// lifecycle chip is a different claim, and it is withheld until something has
// declared the vocabulary the chip would be naming.
func TestSearchHitStatusChipFollowsGovernance(t *testing.T) {
	t.Parallel()

	idx := lexical.NewIndex([]lexical.Document{
		{RelPath: "Writing/Kafka.md", Title: "Kafka Basics", NoteType: "lesson", Status: "draft", PlainText: "kafka log"},
	}, schema.ArtifactPolicy{})

	tests := []struct {
		name     string
		governed bool
		wantChip bool
	}{
		{name: "governed vault names the status", governed: true, wantChip: true},
		{name: "ungoverned folder does not", governed: false, wantChip: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := NewHandler(func() RequestSnapshot {
				return RequestSnapshot{Index: idx, Shell: pages.Shell{Nav: &nav.Model{}, Governed: tt.governed}}
			}, slog.New(slog.DiscardHandler))
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/search/results?q=kafka", http.NoBody)
			rr := httptest.NewRecorder()
			h.results(rr, req)

			body := rr.Body.String()
			if !strings.Contains(body, "Kafka Basics") {
				t.Fatalf("the hit itself is missing; body = %q", body)
			}
			// y-result__sep is the separator the results fragment writes only
			// when it has a status to place after the path.
			if got := strings.Contains(body, `y-result__sep`) && strings.Contains(body, "draft"); got != tt.wantChip {
				t.Errorf("status chip present = %t, want %t; body = %q", got, tt.wantChip, body)
			}
		})
	}
}

// TestSearchResponseIsBounded pins the response cap. A term matching most of a
// large vault used to ship every hit — a multi-megabyte fragment rebuilt on
// each pause in typing — so the list holds only the opening two hundred while
// the count line and the fragment's count marker keep the true tally.
func TestSearchResponseIsBounded(t *testing.T) {
	t.Parallel()

	docs := make([]lexical.Document, 0, 207)
	for i := range 207 {
		docs = append(docs, lexical.Document{
			RelPath:   fmt.Sprintf("Notes/n%03d.md", i),
			Title:     fmt.Sprintf("Note %03d", i),
			PlainText: "bounded needle body",
		})
	}
	idx := lexical.NewIndex(docs, validArtifactPolicy(t))
	h := NewHandler(func() RequestSnapshot {
		return RequestSnapshot{Index: idx, Shell: pages.Shell{Nav: &nav.Model{}, Governed: true}}
	}, slog.New(slog.DiscardHandler))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/search/results?q=needle", http.NoBody)
	rr := httptest.NewRecorder()
	h.results(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if got := strings.Count(body, "<li>"); got != 200 {
		t.Errorf("rendered result rows = %d, want 200", got)
	}
	if !strings.Contains(body, "共 207 筆，顯示前 200 筆") {
		t.Errorf("the capped list does not say both numbers; body opens %q", body[:min(len(body), 400)])
	}
	if !strings.Contains(body, `data-result-count="207"`) {
		t.Errorf("the count marker does not carry the true tally; body opens %q", body[:min(len(body), 400)])
	}

	req = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/search?q=needle", http.NoBody)
	rr = httptest.NewRecorder()
	h.search(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("full page status = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "共 207 筆，顯示前 200 筆") {
		t.Error("the full page does not say both numbers")
	}
}

// TestSearchResultsSurviveFoldLengtheningNote pins the whole request path for
// a note whose snippet lowercases to a longer byte string: one Ⱥ (or one stray
// invalid byte) ahead of the matched word used to push the fold-space match
// offset past the snippet and panic the request that should have surfaced that
// very note.
func TestSearchResultsSurviveFoldLengtheningNote(t *testing.T) {
	t.Parallel()

	idx := lexical.NewIndex([]lexical.Document{
		{RelPath: "Notes/stroke.md", Title: "Stroke", PlainText: "Ⱥ body qfindq"},
	}, validArtifactPolicy(t))
	h := NewHandler(func() RequestSnapshot {
		return RequestSnapshot{Index: idx, Shell: pages.Shell{Nav: &nav.Model{}, Governed: true}}
	}, slog.New(slog.DiscardHandler))
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/search/results?q=qfindq", http.NoBody)
	rr := httptest.NewRecorder()
	h.results(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "<mark>qfindq</mark>") {
		t.Errorf("the matched word is not the marked text; body = %q", rr.Body.String())
	}
}

func TestSearchResultsFragmentMetadataDiagnostic(t *testing.T) {
	t.Parallel()

	policy := undeclaredArtifactPolicy(t)
	idx := lexical.NewIndex([]lexical.Document{
		{RelPath: "Concepts/Note.md", Title: "Note", NoteType: "concept"},
	}, policy)
	h := NewHandler(func() RequestSnapshot {
		return RequestSnapshot{Index: idx, Shell: pages.Shell{Nav: &nav.Model{}}}
	}, slog.New(slog.DiscardHandler))
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/search/results?q=type:concept", http.NoBody)
	rr := httptest.NewRecorder()
	h.results(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("results() status = %d, want %d", rr.Code, http.StatusOK)
	}
	body := html.UnescapeString(rr.Body.String())
	for _, want := range []string{"data-live-search-results", "data-search-diagnostic", policy.Diagnostic()} {
		if !strings.Contains(body, want) {
			t.Errorf("results() diagnostic fragment missing %q; body = %q", want, body)
		}
	}
	if strings.Contains(body, "找不到「") {
		t.Errorf("results() capability diagnostic rendered ordinary no-results copy; body = %q", body)
	}
}

func TestSearchHandlerRejectsInvalidQuery(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	NewHandler(func() RequestSnapshot {
		return RequestSnapshot{Index: lexical.NewIndex(nil, validArtifactPolicy(t)), Shell: pages.Shell{Nav: &nav.Model{}}}
	}, slog.New(slog.DiscardHandler)).Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	tests := []struct {
		name string
		path string
	}{
		{name: "full page rejects a control character", path: "/search?q=%00"},
		{name: "fragment rejects a control character", path: "/search/results?q=%00"},
		{name: "full page rejects an oversized query", path: "/search?q=" + strings.Repeat("a", maxQueryBytes+1)},
		{name: "fragment rejects an oversized query", path: "/search/results?q=" + strings.Repeat("a", maxQueryBytes+1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			code, _ := getBody(t, srv.URL+tt.path)
			if code != http.StatusBadRequest {
				t.Errorf("GET %s status = %d, want %d", tt.path, code, http.StatusBadRequest)
			}
		})
	}
}

func TestSearchHandlerLogsExcludeRawQuery(t *testing.T) {
	t.Parallel()

	const sentinel = "OWNER_THOUGHT_CANARY type:TYPE_PRIVATE status:STATUS_PRIVATE domain:DOMAIN_PRIVATE topic:TOPIC_PRIVATE slug:SLUG_PRIVATE folder:FOLDER_PRIVATE"
	idx := lexical.NewIndex(nil, validArtifactPolicy(t))
	tests := []struct {
		name   string
		writer func() http.ResponseWriter
		invoke func(*Handler, http.ResponseWriter, *http.Request)
	}{
		{name: "full page render error", invoke: (*Handler).search},
		{name: "results fragment render error", invoke: (*Handler).results},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var recordedLogs bytes.Buffer
			h := NewHandler(func() RequestSnapshot {
				return RequestSnapshot{Index: idx, Shell: pages.Shell{Nav: &nav.Model{}}}
			}, slog.New(slog.NewJSONHandler(&recordedLogs, nil)))
			writer := http.ResponseWriter(&failingResponseWriter{header: make(http.Header)})
			if tt.writer != nil {
				writer = tt.writer()
			}
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/search?q="+url.QueryEscape(sentinel), http.NoBody)
			tt.invoke(h, writer, req)

			if recordedLogs.Len() == 0 {
				t.Fatal("search handler recorded no error log")
			}
			for record := range strings.SplitSeq(strings.TrimSpace(recordedLogs.String()), "\n") {
				for _, privateValue := range []string{
					"OWNER_THOUGHT_CANARY",
					"TYPE_PRIVATE",
					"STATUS_PRIVATE",
					"DOMAIN_PRIVATE",
					"TOPIC_PRIVATE",
					"SLUG_PRIVATE",
					"FOLDER_PRIVATE",
				} {
					if strings.Contains(record, privateValue) {
						t.Errorf("search handler log contains query-derived value %q", privateValue)
					}
				}
				var gotAttrs map[string]any
				if err := json.Unmarshal([]byte(record), &gotAttrs); err != nil {
					t.Fatalf("decoding recorded search log: %v", err)
				}
				delete(gotAttrs, "time")
				delete(gotAttrs, "level")
				delete(gotAttrs, "msg")
				wantAttrs := map[string]any{
					"error_type":  "*errors.errorString",
					"filter_keys": []any{"type", "status", "domain", "topic", "slug", "folder"},
					"query_bytes": float64(len(sentinel)),
				}
				if diff := cmp.Diff(wantAttrs, gotAttrs); diff != "" {
					t.Errorf("search handler log attributes mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}

type failingResponseWriter struct {
	header http.Header
}

func (w *failingResponseWriter) Header() http.Header {
	return w.header
}

func (*failingResponseWriter) Write([]byte) (int, error) {
	return 0, io.ErrClosedPipe
}

func (*failingResponseWriter) WriteHeader(int) {}

func getBody(t *testing.T, urlStr string) (code int, body string) {
	t.Helper()
	code, _, body = getBodyWithHeaders(t, urlStr)
	return code, body
}

func getBodyWithHeaders(t *testing.T, urlStr string) (code int, header http.Header, body string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, urlStr, http.NoBody)
	if err != nil {
		t.Fatalf("new request %s: %v", urlStr, err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", urlStr, err)
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
	return resp.StatusCode, resp.Header.Clone(), string(b)
}

func searchPageHTML(t *testing.T, pageHTML string) string {
	t.Helper()

	start := strings.Index(pageHTML, `<div class="y-searchpage"`)
	if start < 0 {
		t.Fatalf("search response has no search page region; html = %q", pageHTML)
	}
	end := strings.Index(pageHTML[start:], "</main>")
	if end < 0 {
		t.Fatalf("search page region has no closing main; html = %q", pageHTML)
	}
	return pageHTML[start : start+end]
}

// TestEmptyResultsNameTheCorpusAndFilesAreLabelled covers the two places the
// widened corpus has to be visible to a reader. A query that finds nothing has
// to say what was searched, because the alternative is what a trial reader
// actually did: conclude a file they had just read was gone and type it again.
// A hit that is a file has to say so, because a file has no lifecycle and never
// will, and a bare row beside notes reads as a note whose status went missing.
//
// The hint is asserted absent when there are hits, or it becomes permanent
// furniture that says nothing at the moment it would mean something.
func TestEmptyResultsNameTheCorpusAndFilesAreLabelled(t *testing.T) {
	t.Parallel()
	idx := lexical.NewIndex([]lexical.Document{
		{RelPath: "Concepts/One.md", Title: "One", PlainText: "needle"},
		lexical.DocumentFromFile("Notes/todo.txt", []byte("needle in a file")),
	}, validArtifactPolicy(t))
	h := NewHandler(func() RequestSnapshot {
		return RequestSnapshot{Index: idx, Shell: pages.Shell{Nav: &nav.Model{}, Governed: true}}
	}, slog.New(slog.DiscardHandler))

	const corpusHint = "圖片、PDF 與其他二進位檔案只在導覽中列出"

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/search?q=haystack", http.NoBody)
	rr := httptest.NewRecorder()
	h.search(rr, req)
	if !strings.Contains(rr.Body.String(), corpusHint) {
		t.Errorf("a query that found nothing does not say what was searched; body = %q", rr.Body.String())
	}

	req = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/search?q=needle", http.NoBody)
	rr = httptest.NewRecorder()
	h.search(rr, req)
	body := rr.Body.String()
	if strings.Contains(body, corpusHint) {
		t.Error("the corpus hint appears beside results, where it says nothing")
	}
	if got := strings.Count(body, `<span class="y-result__kind">檔案</span>`); got != 1 {
		t.Errorf("file rows labelled = %d, want exactly the one file hit; body = %q", got, body)
	}
	note := strings.Index(body, "Concepts/One.md")
	file := strings.Index(body, "Notes/todo.txt")
	if note < 0 || file < 0 || note > file {
		t.Errorf("the note hit is not answered before the file hit; note at %d, file at %d", note, file)
	}
}

// TestAnUnreadableContractSpeaksTheReadersLanguage holds the one vault-level
// fault to the same rule as every other sentence on the page. It is settled at
// startup, before anyone has asked for anything, so the language it is said in
// cannot be decided there — and the label beside it on this very page is
// already chosen per reader, which is how a reader came to see one of them in
// each language.
func TestAnUnreadableContractSpeaksTheReadersLanguage(t *testing.T) {
	t.Parallel()

	const loaderFault = "toml: line 42: expected a key separator"
	policy := (*schema.Contract)(nil).
		Capabilities(schema.Unreadable(errors.New(loaderFault))).Artifacts
	idx := lexical.NewIndex([]lexical.Document{{RelPath: "Concepts/Note.md", Title: "Note", NoteType: "concept"}}, policy)

	tests := []struct {
		name   string
		lang   string
		want   string
		absent string
	}{
		{
			name:   "the default reader reads the default language",
			want:   wording.ContractUnreadablePrefix.In(wording.ZhHant) + loaderFault,
			absent: wording.ContractUnreadablePrefix.In(wording.En),
		},
		{
			name:   "a reader who chose english reads english",
			lang:   string(wording.En),
			want:   wording.ContractUnreadablePrefix.In(wording.En) + loaderFault,
			absent: wording.ContractUnreadablePrefix.In(wording.ZhHant),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := NewHandler(func() RequestSnapshot {
				return RequestSnapshot{Index: idx, Shell: pages.Shell{Nav: &nav.Model{}}}
			}, slog.New(slog.DiscardHandler))
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/search?q=type:concept", http.NoBody)
			if tt.lang != "" {
				req.Header.Set("Cookie", wording.CookieName+"="+tt.lang)
			}
			rr := httptest.NewRecorder()
			h.search(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rr.Code)
			}
			body := html.UnescapeString(rr.Body.String())
			if !strings.Contains(body, tt.want) {
				t.Errorf("the page does not say %q; body = %q", tt.want, body)
			}
			if strings.Contains(body, tt.absent) {
				t.Errorf("the page says %q, which this reader did not ask for", tt.absent)
			}
		})
	}
}
