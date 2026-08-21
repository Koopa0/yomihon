package report

import (
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/vault"
)

// briefingFixture carries a <script>, an HTML entity, and CJK content, so the
// verbatim /raw round-trip proves yomihon neither rewrites, escapes, nor
// transcodes the bytes it serves — a briefing lands in the sandboxed frame
// exactly as authored.
const briefingFixture = `<!doctype html>
<html lang="zh-Hant">
<head><meta charset="utf-8"><title>日次簡報 · test</title>
<script src="https://cdn.jsdelivr.net/npm/chart.js@4.5.0/dist/chart.umd.js"></script>
</head>
<body><h1>週報 &amp; チャート</h1><script>new Chart(document.getElementById('c'), {});</script></body>
</html>
`

const briefingName = "koopa0-briefing-2026-07-03.html"

// vaultWithBriefing writes the briefing fixture to disk under the layout the
// nav model's RelPath expects, and returns the vault root.
func vaultWithBriefing(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "System", "reports", "daily-briefing")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, briefingName), []byte(briefingFixture), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

func newHandler(t *testing.T, root string) http.Handler {
	t.Helper()
	source, view := rootedReportView(t, root)
	mux := http.NewServeMux()
	New(
		source,
		func() *snapshot.View { return view },
		func(snap *snapshot.View) pages.Shell { return pages.Shell{Nav: snap.Navigation()} },
		slog.New(slog.DiscardHandler),
	).Register(mux)
	return mux
}

func rootedReportView(t *testing.T, root string) (*vault.Reader, *snapshot.View) {
	t.Helper()
	source, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open(%q) error: %v", root, err)
	}
	t.Cleanup(func() {
		if closeErr := source.Close(); closeErr != nil {
			t.Errorf("Reader.Close() error: %v", closeErr)
		}
	})
	store, err := snapshot.New(t.Context(), source, slog.New(slog.DiscardHandler), nil, schema.Ungoverned())
	if err != nil {
		t.Fatalf("snapshot.New(%q) error: %v", root, err)
	}
	return source, store.Current()
}

func TestReportRoutesCaptureSnapshotOnce(t *testing.T) {
	t.Parallel()
	root := vaultWithBriefing(t)
	source, view := rootedReportView(t, root)
	for _, tt := range []struct {
		name, path string
		wantShell  bool
	}{
		{name: "shell", path: "/reports/" + briefingName, wantShell: true},
		{name: "raw", path: "/reports/" + briefingName + "/raw"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			calls := 0
			mux := http.NewServeMux()
			New(
				source,
				func() *snapshot.View {
					calls++
					return view
				},
				func(snap *snapshot.View) pages.Shell {
					return pages.Shell{Nav: snap.Navigation(), Governed: true}
				},
				slog.New(slog.DiscardHandler),
			).Register(mux)
			rr := get(t, mux, tt.path)
			if rr.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rr.Code)
			}
			if calls != 1 {
				t.Errorf("shell snapshot reads = %d, want 1", calls)
			}
			// The navigation model rides on the shell this route was handed,
			// so the rail's reports group witnesses that shell rather than
			// one the renderer derived for itself.
			if tt.wantShell && !strings.Contains(rr.Body.String(), `data-sidebar-group="reports"`) {
				t.Errorf("response did not render the shell it was handed; body = %q", rr.Body.String())
			}
		})
	}
}

func get(t *testing.T, h http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, http.NoBody))
	return rr
}

// TestResolveReportAllowsOnlyEnumeratedBriefings pins the request-name gate. A
// name that is not an enumerated briefing — a .md report, an unknown file, or
// a traversal-shaped string — never resolves.
func TestResolveReportAllowsOnlyEnumeratedBriefings(t *testing.T) {
	t.Parallel()
	_, view := rootedReportView(t, vaultWithBriefing(t))
	model := view.Navigation()

	cases := []struct {
		name string
		want bool
	}{
		{briefingName, true},                 // the enumerated briefing
		{"weekly-note.md", false},            // a .md report — served at /notes/, not here
		{"nonexistent.html", false},          // unknown
		{"../../Diary/2026-07-03.md", false}, // traversal-shaped: never resolves
		{"..", false},
		{"", false},
	}
	for _, tc := range cases {
		if _, ok := resolveReport(model, tc.name); ok != tc.want {
			t.Errorf("resolveReport(%q) = %v, want %v", tc.name, ok, tc.want)
		}
	}
	if _, ok := resolveReport(nil, briefingName); ok {
		t.Error("resolveReport(nil model) must not resolve")
	}
}

// TestShowRendersSandboxedIframe pins the sandbox contract: the rendered
// iframe grants no sandbox capability, and its src is this report's verbatim
// /raw endpoint.
func TestShowRendersSandboxedIframe(t *testing.T) {
	t.Parallel()
	h := newHandler(t, vaultWithBriefing(t))

	rr := get(t, h, "/reports/"+briefingName)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `sandbox=""`) {
		t.Errorf(`iframe must carry a bare sandbox attribute; got:\n%s`, body)
	}
	if strings.Contains(body, "allow-") {
		t.Errorf("iframe must not grant any sandbox capability; got:\n%s", body)
	}
	if want := `src="/reports/` + briefingName + `/raw"`; !strings.Contains(body, want) {
		t.Errorf("iframe src must be the verbatim /raw endpoint %q; got:\n%s", want, body)
	}
}

// TestRawServesVerbatim is the byte-identical round-trip: the /raw endpoint
// returns the source file's bytes unchanged (the <script>, the &amp;, and the
// CJK all intact), with an explicit text/html; charset=utf-8 and nosniff.
func TestRawServesVerbatim(t *testing.T) {
	t.Parallel()
	h := newHandler(t, vaultWithBriefing(t))

	rr := get(t, h, "/reports/"+briefingName+"/raw")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	header := rr.Result().Header
	if ct := header.Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q, want text/html; charset=utf-8", ct)
	}
	if ns := header.Get("X-Content-Type-Options"); ns != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", ns)
	}
	if cc := header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store (never cache the mutable latest briefing)", cc)
	}
	if got := rr.Body.String(); got != briefingFixture {
		t.Errorf("raw body is not byte-identical to the source file:\nwant %q\ngot  %q", briefingFixture, got)
	}
}

func TestRawStaysOnOpenedVaultWhenConfiguredPathIsReplaced(t *testing.T) {
	t.Parallel()
	root := vaultWithBriefing(t)
	h := newHandler(t, root)

	moved := root + "-moved"
	if err := os.Rename(root, moved); err != nil {
		t.Fatal(err)
	}
	replacement := filepath.Join(root, "System", "reports", "daily-briefing")
	if err := os.MkdirAll(replacement, 0o750); err != nil {
		t.Fatal(err)
	}
	const sentinel = "replacement vault report\n"
	if err := os.WriteFile(filepath.Join(replacement, briefingName), []byte(sentinel), 0o600); err != nil {
		t.Fatal(err)
	}

	rr := get(t, h, "/reports/"+briefingName+"/raw")
	if rr.Code != http.StatusOK {
		t.Fatalf("GET report after root replacement status = %d, want %d", rr.Code, http.StatusOK)
	}
	if got := rr.Body.String(); got != briefingFixture {
		t.Errorf("GET report after root replacement body = %q, want originally opened vault bytes", got)
	}
	if strings.Contains(rr.Body.String(), sentinel) {
		t.Errorf("GET report after root replacement body = %q, want no replacement-vault bytes", rr.Body.String())
	}
}

func TestRawResolvesNFCRequestToObservedNFDEntry(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, "System", "reports", "daily-briefing")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	const (
		nfdName = "koopa\u0301.html"
		nfcName = "koop\u00e1.html"
		content = "NFD filesystem spelling\n"
	)
	if err := os.WriteFile(filepath.Join(dir, nfdName), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	h := newHandler(t, root)

	rr := get(t, h, "/reports/"+url.PathEscape(nfcName)+"/raw")
	if rr.Code != http.StatusOK {
		t.Fatalf("GET NFC report name status = %d, want %d; body = %q", rr.Code, http.StatusOK, rr.Body.String())
	}
	if got := rr.Body.String(); got != content {
		t.Errorf("GET NFC report name body = %q, want %q", got, content)
	}
}

func TestRawRefreshesAtomicReportEdit(t *testing.T) {
	t.Parallel()
	root := vaultWithBriefing(t)
	h := newHandler(t, root)
	dir := filepath.Join(root, "System", "reports", "daily-briefing")
	replacement := filepath.Join(dir, "replacement.html")
	const updated = "updated report bytes\n"
	if err := os.WriteFile(replacement, []byte(updated), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, filepath.Join(dir, briefingName)); err != nil {
		t.Fatal(err)
	}

	rr := get(t, h, "/reports/"+briefingName+"/raw")
	if rr.Code != http.StatusOK {
		t.Fatalf("GET atomically edited report status = %d, want %d", rr.Code, http.StatusOK)
	}
	if got := rr.Body.String(); got != updated {
		t.Errorf("GET atomically edited report body = %q, want %q", got, updated)
	}
}

func TestReadReportRejectsFileAddedAfterSnapshot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	source, view := rootedReportView(t, root)
	dir := filepath.Join(root, "System", "reports", "daily-briefing")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, briefingName), []byte(briefingFixture), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := readReport(
		t.Context(),
		source,
		view,
		"System/reports/daily-briefing/"+briefingName,
	)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("readReport(file added after snapshot) error = %v, want %v", err, fs.ErrNotExist)
	}
	if len(got) != 0 {
		t.Errorf("readReport(file added after snapshot) = %q, want no bytes", got)
	}
}

func TestRawRejectsSymlinkOutsideReportRoot(t *testing.T) {
	t.Parallel()
	root := vaultWithBriefing(t)
	h := newHandler(t, root)
	sentinelDir := t.TempDir()
	const sentinel = "outside report root\n"
	sentinelPath := filepath.Join(sentinelDir, "sentinel.html")
	if err := os.WriteFile(sentinelPath, []byte(sentinel), 0o600); err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(root, "System", "reports", "daily-briefing", briefingName)
	if err := os.Remove(reportPath); err != nil {
		t.Fatal(err)
	}
	target, err := filepath.Rel(filepath.Dir(reportPath), sentinelPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, reportPath); err != nil {
		t.Fatal(err)
	}

	rr := get(t, h, "/reports/"+briefingName+"/raw")
	if rr.Code != http.StatusNotFound {
		t.Errorf("GET report symlink status = %d, want %d; body = %q", rr.Code, http.StatusNotFound, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), sentinel) {
		t.Errorf("GET report symlink body = %q, want no sentinel bytes", rr.Body.String())
	}
}

func TestRawRejectsFinalSymlinkWithinReportRoot(t *testing.T) {
	t.Parallel()
	root := vaultWithBriefing(t)
	h := newHandler(t, root)
	reportsDir := filepath.Join(root, "System", "reports", "daily-briefing")
	const sentinel = "symlinked report bytes\n"
	if err := os.WriteFile(filepath.Join(reportsDir, "sentinel.html"), []byte(sentinel), 0o600); err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(reportsDir, briefingName)
	if err := os.Remove(reportPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("sentinel.html", reportPath); err != nil {
		t.Fatal(err)
	}

	rr := get(t, h, "/reports/"+briefingName+"/raw")
	if rr.Code != http.StatusNotFound {
		t.Errorf("GET final report symlink status = %d, want %d; body = %q", rr.Code, http.StatusNotFound, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), sentinel) {
		t.Errorf("GET final report symlink body = %q, want no sentinel bytes", rr.Body.String())
	}
}

// TestRawSourceChangeReturnsNoReportBytes replaces an observed parent with a
// symlink after snapshot capture. The rooted refresh rejects the changed
// source before any old or replacement report bytes reach the response.
func TestRawSourceChangeReturnsNoReportBytes(t *testing.T) {
	t.Parallel()
	root := vaultWithBriefing(t)
	h := newHandler(t, root)
	outsideReports := filepath.Join(root, "OutsideReports", "daily-briefing")
	if err := os.MkdirAll(outsideReports, 0o750); err != nil {
		t.Fatal(err)
	}
	const sentinel = "relocated report capability\n"
	if err := os.WriteFile(filepath.Join(outsideReports, briefingName), []byte(sentinel), 0o600); err != nil {
		t.Fatal(err)
	}
	reportsDir := filepath.Join(root, "System", "reports")
	if err := os.Rename(reportsDir, reportsDir+"-observed"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "OutsideReports"), reportsDir); err != nil {
		t.Fatal(err)
	}

	rr := get(t, h, "/reports/"+briefingName+"/raw")
	if rr.Code != http.StatusNotFound {
		t.Errorf("GET through directory symlink status = %d, want %d; body = %q", rr.Code, http.StatusNotFound, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), sentinel) || strings.Contains(rr.Body.String(), briefingFixture) {
		t.Errorf("GET through changed directory body = %q, want no report bytes", rr.Body.String())
	}
}

func TestRawRejectsNonRegularFinalEntry(t *testing.T) {
	t.Parallel()
	root := vaultWithBriefing(t)
	h := newHandler(t, root)
	entry := filepath.Join(root, "System", "reports", "daily-briefing", briefingName)
	if err := os.Remove(entry); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(entry, 0o750); err != nil {
		t.Fatal(err)
	}

	rr := get(t, h, "/reports/"+briefingName+"/raw")
	if rr.Code != http.StatusNotFound {
		t.Errorf("GET report replaced by directory status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

// TestRawIsSelfSandboxing pins the resource-level containment: /raw carries a
// CSP sandbox so a script-bearing briefing lands in an opaque origin however it
// is loaded — not only when yomihon's shell iframe supplies the attribute. Absent
// this, a cross-origin frame or a top-level "open in new tab" would run the
// briefing's scripts same-origin to the whole vault-reading surface.
func TestRawIsSelfSandboxing(t *testing.T) {
	t.Parallel()
	h := newHandler(t, vaultWithBriefing(t))

	rr := get(t, h, "/reports/"+briefingName+"/raw")
	csp := rr.Result().Header.Get("Content-Security-Policy")
	const want = "sandbox; default-src 'none'; base-uri 'none'; connect-src 'none'; " +
		"font-src data:; form-action 'none'; frame-ancestors 'self'; frame-src 'none'; " +
		"img-src data:; media-src data:; object-src 'none'; " +
		"script-src 'none'; script-src-attr 'none'; " +
		"style-src 'unsafe-inline'; worker-src 'none'"
	if csp != want {
		t.Errorf("/raw CSP = %q, want the byte-exact scriptless, zero-automatic-egress policy %q", csp, want)
	}
}

// TestNotFoundCases verifies that everything outside the briefing allowlist is
// a 404 on both the shell and raw endpoints.
func TestNotFoundCases(t *testing.T) {
	t.Parallel()
	h := newHandler(t, vaultWithBriefing(t))

	for _, target := range []string{
		"/reports/nope.html",
		"/reports/nope.html/raw",
		"/reports/weekly-note.md",
		"/reports/weekly-note.md/raw",
	} {
		if rr := get(t, h, target); rr.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", target, rr.Code)
		}
	}
}

// TestTraversalNeverServesAFile drives a traversal-shaped request through the
// full HTTP path. However the router cleans or decodes the path, the allowlist
// is the only gate, so the response is never a 200 serving out-of-allowlist
// content — the path-escape rejection this face must uphold.
func TestTraversalNeverServesAFile(t *testing.T) {
	t.Parallel()
	h := newHandler(t, vaultWithBriefing(t))

	for _, target := range []string{
		"/reports/%2e%2e%2f%2e%2e%2fsecret.md",
		"/reports/..%2f..%2fsecret.md/raw",
	} {
		if rr := get(t, h, target); rr.Code == http.StatusOK {
			t.Errorf("GET %s served a 200 (must never escape the allowlist):\n%s", target, rr.Body.String())
		}
	}
}

// TestRawConfinesToSystemReports is defense-in-depth: even if a scanner bug ever
// listed a Briefing whose path is outside System/reports/ (here a Diary note, a
// hard never-egress), /raw refuses to serve it — the allowlist is not the only
// thing between a request and an arbitrary in-vault file.
func TestRawConfinesToSystemReports(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, "Diary")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "secret.md"), []byte("private\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	source, view := rootedReportView(t, root)
	got, err := readReport(t.Context(), source, view, "Diary/secret.md")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("readReport(Diary) error = %v, want %v", err, fs.ErrNotExist)
	}
	if len(got) != 0 {
		t.Errorf("readReport(Diary) = %q, want no bytes", got)
	}
}

// TestRawReturnsNotFoundWhenFileVanishes verifies that a vanished allowlisted
// briefing returns 404 without committing report bytes or success headers.
func TestRawReturnsNotFoundWhenFileVanishes(t *testing.T) {
	t.Parallel()
	root := vaultWithBriefing(t)
	h := newHandler(t, root)
	if err := os.Remove(filepath.Join(root, "System", "reports", "daily-briefing", briefingName)); err != nil {
		t.Fatal(err)
	}

	rr := get(t, h, "/reports/"+briefingName+"/raw")
	if rr.Code != http.StatusNotFound {
		t.Errorf("vanished report status = %d, want %d", rr.Code, http.StatusNotFound)
	}
	if strings.Contains(rr.Body.String(), briefingFixture) {
		t.Errorf("vanished report body = %q, want no report bytes", rr.Body.String())
	}
	header := rr.Result().Header
	if got := header.Get("Content-Security-Policy"); got != "" {
		t.Errorf("vanished report Content-Security-Policy = %q, want no success header", got)
	}
	if got := header.Get("Cache-Control"); got != "" {
		t.Errorf("vanished report Cache-Control = %q, want no success header", got)
	}
	if got := header.Get("Content-Type"); got == "text/html; charset=utf-8" {
		t.Errorf("vanished report Content-Type = %q, want no success content type", got)
	}
}

// TestReportSaysWhenPartOfItCannotDraw covers a hole with nothing beside it.
// The frame refuses scripts, which is right: a briefing that pulls a charting
// library from a CDN would be reaching off this machine, and that is the one
// thing this program does not do. But a reader who wrote a briefing with three
// charts and opens it here sees three blank spaces and no reason for them, and
// reads that as a broken report rather than as the boundary holding.
func TestReportSaysWhenPartOfItCannotDraw(t *testing.T) {
	t.Parallel()

	const notice = "程式不會在這裡執行"

	t.Run("a briefing that draws with a script says so", func(t *testing.T) {
		t.Parallel()
		rr := get(t, newHandler(t, vaultWithBriefing(t)), "/reports/"+briefingName)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}
		if !strings.Contains(rr.Body.String(), notice) {
			t.Errorf("the page shows a frame that cannot draw the document and says nothing; body = %q", rr.Body.String())
		}
	})

	t.Run("a briefing that draws nothing stays quiet", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		dir := filepath.Join(root, "System", "reports", "daily-briefing")
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
		plain := "<!doctype html>\n<html lang=\"zh-Hant\"><body><h1>週報</h1><p>只有文字。</p></body></html>\n"
		if err := os.WriteFile(filepath.Join(dir, briefingName), []byte(plain), 0o600); err != nil {
			t.Fatal(err)
		}
		rr := get(t, newHandler(t, root), "/reports/"+briefingName)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}
		if strings.Contains(rr.Body.String(), notice) {
			t.Errorf("a document with nothing to draw was told its drawing would not run; body = %q", rr.Body.String())
		}
	})
}
