package search

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestSearchHandler exercises GET /search end to end: route registration, the
// live-index provider closure, query parsing, and the rendered results page.
func TestSearchHandler(t *testing.T) {
	t.Parallel()

	idx := BuildFromDocs([]Doc{
		{RelPath: "Writing/Kafka.md", Title: "Kafka Basics", NoteType: "lesson", Status: "draft", PlainText: "kafka is a distributed log"},
	})
	mux := http.NewServeMux()
	NewHandler(func() *Index { return idx }, slog.New(slog.DiscardHandler)).Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	t.Run("matching query renders the hit", func(t *testing.T) {
		t.Parallel()
		code, body := getBody(t, srv.URL+"/search?q=kafka")
		if code != http.StatusOK {
			t.Fatalf("status = %d, want 200", code)
		}
		for _, want := range []string{"Kafka Basics", `href="/notes/Writing/Kafka.md"`, "[draft]"} {
			if !strings.Contains(body, want) {
				t.Errorf("search page missing %q; body = %q", want, body)
			}
		}
	})

	t.Run("non-matching query shows no results", func(t *testing.T) {
		t.Parallel()
		code, body := getBody(t, srv.URL+"/search?q=zzznotpresent")
		if code != http.StatusOK {
			t.Fatalf("status = %d, want 200", code)
		}
		if !strings.Contains(body, "No results") {
			t.Errorf(`search page missing "No results"; body = %q`, body)
		}
	})
}

func getBody(t *testing.T, url string) (code int, body string) {
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
