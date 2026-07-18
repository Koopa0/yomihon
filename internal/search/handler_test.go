package search

import (
	"bytes"
	"encoding/json"
	"html"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/ui/pages"
)

// TestSearchHandler exercises GET /search end to end: route registration, the
// live-index provider closure, query parsing, and the rendered results page.
func TestSearchHandler(t *testing.T) {
	t.Parallel()

	idx := NewIndex([]Document{
		{RelPath: "Writing/Kafka.md", Title: "Kafka Basics", NoteType: "lesson", Status: "draft", PlainText: "kafka is a distributed log"},
	}, validArtifactPolicy(t))
	mux := http.NewServeMux()
	NewHandler(func() RequestSnapshot {
		return RequestSnapshot{Index: idx, Shell: pages.Shell{Nav: &nav.Model{}}}
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

	missingPolicy := schema.ArtifactPolicy{}
	invalidPolicy := invalidArtifactPolicy(t)
	tests := []struct {
		name       string
		idx        *Index
		diagnostic string
	}{
		{name: "missing", idx: NewIndex([]Document{{RelPath: "Concepts/Note.md", Title: "Note", NoteType: "concept"}}, missingPolicy), diagnostic: missingPolicy.Diagnostic()},
		{name: "invalid", idx: NewIndex([]Document{{RelPath: "Concepts/Note.md", Title: "Note", NoteType: "concept"}}, invalidPolicy), diagnostic: invalidPolicy.Diagnostic()},
		{name: "zero value index", idx: &Index{}, diagnostic: missingPolicy.Diagnostic()},
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
	idx := NewIndex([]Document{{RelPath: "Concepts/One.md", Title: "One", PlainText: "needle"}}, validArtifactPolicy(t))
	calls := 0
	h := NewHandler(func() RequestSnapshot {
		calls++
		return RequestSnapshot{
			Index: idx,
			Shell: pages.Shell{Nav: &nav.Model{}, Advanceable: 4, AdvanceableKnown: true},
		}
	}, slog.New(slog.DiscardHandler))
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/search?q=needle", http.NoBody)
	rr := httptest.NewRecorder()
	h.search(rr, req)
	if calls != 1 {
		t.Errorf("request snapshot reads = %d, want 1", calls)
	}
	for _, want := range []string{"One", `aria-label="4 篇筆記可進入下一個合法狀態"`} {
		if !strings.Contains(rr.Body.String(), want) {
			t.Errorf("search response missing %q; body = %q", want, rr.Body.String())
		}
	}
}

func TestSearchResultsFragment(t *testing.T) {
	t.Parallel()

	idx := NewIndex([]Document{
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

func TestSearchResultsFragmentMetadataDiagnostic(t *testing.T) {
	t.Parallel()

	policy := schema.ArtifactPolicy{}
	idx := NewIndex([]Document{
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
		return RequestSnapshot{Index: NewIndex(nil, validArtifactPolicy(t)), Shell: pages.Shell{Nav: &nav.Model{}}}
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
	idx := NewIndex(nil, validArtifactPolicy(t))
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
