package report

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/nav"
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

// fixtureModel is a nav model listing two reports: a briefing (.html, served by
// this feature) and a plain report (.md, an ordinary note served at /notes/,
// never here). The allowlist the handler resolves against is exactly this.
func fixtureModel() *nav.Model {
	return &nav.Model{Reports: []nav.Report{
		{Name: "weekly-note.md", RelPath: "System/reports/weekly-note.md"},
		{Name: briefingName, RelPath: "System/reports/daily-briefing/" + briefingName, Briefing: true},
	}}
}

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

func newHandler(t *testing.T, root string, model *nav.Model) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	NewHandler(Deps{
		Root: root,
		Nav:  func() *nav.Model { return model },
		Log:  slog.New(slog.DiscardHandler),
	}).Register(mux)
	return mux
}

func get(t *testing.T, h http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, http.NoBody))
	return rr
}

// TestResolveReportIsTheOnlyGate pins the security model: the requested name is
// only ever looked up against the allowlist, never joined onto a path. A name
// that is not an enumerated briefing — a .md report, an unknown file, or a
// traversal-shaped string ("../../Diary/x") — never resolves, so no file
// outside the allowlist can be reached.
func TestResolveReportIsTheOnlyGate(t *testing.T) {
	t.Parallel()
	model := fixtureModel()

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
// iframe is exactly sandbox="allow-scripts" and never grants
// allow-same-origin, and its src is this report's verbatim /raw endpoint.
func TestShowRendersSandboxedIframe(t *testing.T) {
	t.Parallel()
	h := newHandler(t, vaultWithBriefing(t), fixtureModel())

	rr := get(t, h, "/reports/"+briefingName)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `sandbox="allow-scripts"`) {
		t.Errorf(`iframe must be sandbox="allow-scripts"; got:\n%s`, body)
	}
	if strings.Contains(body, "allow-same-origin") {
		t.Errorf("iframe must NEVER grant allow-same-origin; got:\n%s", body)
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
	h := newHandler(t, vaultWithBriefing(t), fixtureModel())

	rr := get(t, h, "/reports/"+briefingName+"/raw")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q, want text/html; charset=utf-8", ct)
	}
	if ns := rr.Header().Get("X-Content-Type-Options"); ns != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", ns)
	}
	if cc := rr.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store (never cache the mutable latest briefing)", cc)
	}
	if got := rr.Body.String(); got != briefingFixture {
		t.Errorf("raw body is not byte-identical to the source file:\nwant %q\ngot  %q", briefingFixture, got)
	}
}

// TestRawIsSelfSandboxing pins the resource-level containment: /raw carries a
// CSP sandbox so a script-bearing briefing lands in an opaque origin however it
// is loaded — not only when yomihon's shell iframe supplies the attribute. Absent
// this, a cross-origin frame or a top-level "open in new tab" would run the
// briefing's scripts same-origin to the whole vault-reading surface.
func TestRawIsSelfSandboxing(t *testing.T) {
	t.Parallel()
	h := newHandler(t, vaultWithBriefing(t), fixtureModel())

	rr := get(t, h, "/reports/"+briefingName+"/raw")
	csp := rr.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "sandbox allow-scripts") {
		t.Errorf("/raw must carry a CSP sandbox allow-scripts; got %q", csp)
	}
	if strings.Contains(csp, "allow-same-origin") {
		t.Errorf("/raw CSP sandbox must NEVER grant allow-same-origin; got %q", csp)
	}
}

// TestNotFoundCases: everything outside the briefing allowlist is a silent 404,
// on both the shell and the raw endpoint — an unknown name and a .md report
// (which belongs to /notes/, not here).
func TestNotFoundCases(t *testing.T) {
	t.Parallel()
	h := newHandler(t, vaultWithBriefing(t), fixtureModel())

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
	h := newHandler(t, vaultWithBriefing(t), fixtureModel())

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
	// A malformed allowlist that (wrongly) marks the Diary note as a briefing.
	model := &nav.Model{Reports: []nav.Report{
		{Name: "secret.md", RelPath: "Diary/secret.md", Briefing: true},
	}}
	h := newHandler(t, root, model)

	if rr := get(t, h, "/reports/secret.md/raw"); rr.Code != http.StatusNotFound {
		t.Errorf("a Briefing outside System/reports/ must not be served; got %d", rr.Code)
	}
}

// TestRawFailsOpenWhenFileVanishes: the allowlist still lists a briefing (a ~2s
// snapshot) but its file is gone. That is a not-found, fail-open — a
// 404, never a 500 that would leak a filesystem path.
func TestRawFailsOpenWhenFileVanishes(t *testing.T) {
	t.Parallel()
	h := newHandler(t, t.TempDir(), fixtureModel()) // empty root: the briefing file does not exist

	if rr := get(t, h, "/reports/"+briefingName+"/raw"); rr.Code != http.StatusNotFound {
		t.Errorf("vanished report must fail open to 404, got %d", rr.Code)
	}
}
