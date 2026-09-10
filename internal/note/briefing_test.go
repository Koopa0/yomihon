package note_test

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestShowRedirectsARegisteredBriefingToTheReportSurface is the landing half of
// the canonical-report lock. A hand-typed /notes/ address for a daily-briefing
// HTML used to dump chroma source; it must 302 to the report surface instead,
// and the redirect cannot be cached because registration follows file location
// and can stop being true. Unregistered HTML keeps the source view.
func TestShowRedirectsARegisteredBriefingToTheReportSurface(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeBriefingFixture(t, root, "System/reports/daily-briefing/browser-boundary.html", "<p>briefing</p>\n")
	writeBriefingFixture(t, root, "Notes/page.html", "<p>unregistered</p>\n")
	writeBriefingFixture(t, root, "System/reports/daily-briefing/sub/x.html", "<p>nested</p>\n")
	writeBriefingFixture(t, root, "System/reports/Week of 2026-08-31.md", "# written\n")
	srv := newServer(t, root)

	client := *srv.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

	code, header, body := getHeaders(t, &client, srv.URL+"/notes/System/reports/daily-briefing/browser-boundary.html")
	if code != http.StatusFound {
		t.Errorf("GET registered briefing = %d, want 302; body = %q", code, body)
	}
	if got := header.Get("Location"); got != "/reports/browser-boundary.html" {
		t.Errorf("Location = %q, want /reports/browser-boundary.html", got)
	}
	if got := header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}

	for _, rel := range []string{
		"Notes/page.html",
		"System/reports/daily-briefing/sub/x.html",
	} {
		code, body = get(t, &client, srv.URL+"/notes/"+rel)
		if code != http.StatusOK {
			t.Errorf("GET /notes/%s = %d, want 200", rel, code)
		}
		if !strings.Contains(body, `<pre class="chroma"`) {
			t.Errorf("GET /notes/%s is not a source view", rel)
		}
	}

	code, body = get(t, &client, srv.URL+"/notes/System/reports/Week%20of%202026-08-31.md")
	if code != http.StatusOK {
		t.Errorf("GET written report = %d, want 200", code)
	}
	if !strings.Contains(body, "written") {
		t.Errorf("written report was not served as a note; body = %q", body)
	}
}

// TestFolderPageSendsARegisteredBriefingToTheReportSurface holds the folder
// listing to the same address the report surface already uses. The page carries
// no rail, so the shelf row is the only href; a /notes/ twin here is exactly
// the drop out of report mode the desk cannot keep.
func TestFolderPageSendsARegisteredBriefingToTheReportSurface(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeBriefingFixture(t, root, "System/reports/daily-briefing/browser-boundary.html", "<p>briefing</p>\n")
	srv := newServer(t, root)

	code, body := get(t, srv.Client(), srv.URL+"/folders/System/reports/daily-briefing")
	if code != http.StatusOK {
		t.Fatalf("GET folder = %d, want 200", code)
	}
	const reportRow = `class="y-row" href="/reports/browser-boundary.html" data-index-row`
	if !strings.Contains(body, reportRow) {
		t.Errorf("folder listing does not send the briefing to the report surface; body = %q", body)
	}
	if strings.Contains(body, `href="/notes/System/reports/daily-briefing/browser-boundary.html"`) {
		t.Errorf("folder listing still offers the /notes/ twin; body = %q", body)
	}
}

func writeBriefingFixture(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatalf("mkdir %s: %v", rel, err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func getHeaders(t *testing.T, client *http.Client, urlStr string) (code int, header http.Header, body string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, urlStr, http.NoBody)
	if err != nil {
		t.Fatalf("new request %s: %v", urlStr, err)
	}
	resp, err := client.Do(req)
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
