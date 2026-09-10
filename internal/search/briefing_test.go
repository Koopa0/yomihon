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

// TestARegisteredBriefingSearchHitOpensTheReportSurface holds the file hit a
// reader follows from search to the report surface. The result list used to
// carry /notes/ while the rail beside it already carried /reports/, so one
// screen offered two addresses for one briefing. The y-result href is the
// list's own link; the rail's reportHref is a different class and cannot
// answer this.
func TestARegisteredBriefingSearchHitOpensTheReportSurface(t *testing.T) {
	t.Parallel()

	idx := lexical.NewIndex([]lexical.Document{
		lexical.DocumentFromFile(
			"System/reports/daily-briefing/browser-boundary.html",
			[]byte("browser-boundary briefing body"),
		),
	}, validArtifactPolicy(t))
	mux := http.NewServeMux()
	NewHandler(func() RequestSnapshot {
		return RequestSnapshot{Index: idx, Shell: nav.Shell{Nav: &nav.Model{}, Governed: true}}
	}, slog.New(slog.DiscardHandler)).Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	code, body := getBody(t, srv.Client(), srv.URL+"/search?q=browser-boundary")
	if code != 200 {
		t.Fatalf("status = %d, want 200", code)
	}
	if !strings.Contains(body, `class="y-result" href="/reports/browser-boundary.html`) {
		t.Errorf("the file hit does not open the report surface; body = %q", body)
	}
	if strings.Contains(body, `class="y-result" href="/notes/System/reports/daily-briefing/browser-boundary.html`) {
		t.Errorf("the file hit still offers the /notes/ twin; body = %q", body)
	}
}
