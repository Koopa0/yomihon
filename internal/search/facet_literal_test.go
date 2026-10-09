package search

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/lexical"
	"github.com/koopa0/yomihon/internal/nav"
)

func TestFacetDoesNotLinkToAnotherDeclaredValue(t *testing.T) {
	t.Parallel()
	idx := lexical.NewIndex([]lexical.Document{
		{RelPath: "Notes/Literal.md", Title: "Literal", Domain: `"east"`, PlainText: "needle"},
		{RelPath: "Notes/Plain.md", Title: "Plain", Domain: "east", PlainText: "needle"},
	}, validArtifactPolicy(t))
	mux := http.NewServeMux()
	NewHandler(func() RequestSnapshot {
		return RequestSnapshot{Index: idx, Shell: nav.Shell{Nav: &nav.Model{}, Governed: true}}
	}, slog.New(slog.DiscardHandler)).Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	for _, route := range []string{"/search?q=needle", "/search/results?q=needle&facets=1"} {
		code, body := getBody(t, srv.Client(), srv.URL+route)
		if code != http.StatusOK {
			t.Fatalf("%s: status = %d, want 200", route, code)
		}
		if !strings.Contains(body, `&#34;east&#34;</span>`) {
			t.Errorf("%s: literal domain value disappeared from its facet", route)
		}
		if strings.Contains(body, `href="/search?q=needle+domain%3A%22east%22"`) {
			t.Errorf("%s: literal domain facet offers a query for the plain domain value", route)
		}
		if !strings.Contains(body, `href="/search?q=needle+domain%3Aeast"`) {
			t.Errorf("%s: ordinary domain facet lost its valid query", route)
		}
	}
}
