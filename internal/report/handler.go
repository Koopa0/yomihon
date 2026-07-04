package report

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/koopa0/kurodo/internal/ui/pages"
)

// Register mounts the reports face's routes: the shell page and the verbatim
// raw endpoint the shell's iframe points at. ServeMux matches the more specific
// /raw pattern first, so the two never collide.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /reports/{name}", h.show)
	mux.HandleFunc("GET /reports/{name}/raw", h.raw)
}

// show renders the report shell: header + sidebar + title + a sandboxed iframe
// whose src is this report's /raw endpoint. An unknown name (or one that is not
// an enumerated briefing) is a silent 404 — the same fail-quiet stance the
// reading page takes for a missing note.
func (h *Handler) show(w http.ResponseWriter, r *http.Request) {
	model := h.deps.Nav()
	rep, ok := resolveReport(model, r.PathValue("name"))
	if !ok {
		http.NotFound(w, r)
		return
	}

	view := pages.ReportView{Name: rep.Name, Nav: model}
	if err := pages.Report(view, pages.ChromeFromRequest(r, rep.Name)).Render(r.Context(), w); err != nil {
		h.deps.Log.Error("write report page", "name", rep.Name, "error", err)
	}
}

// raw serves the briefing HTML byte-for-byte: the file the allowlist resolved,
// read fresh each request (never cached — always the latest on disk), written
// with no templ, no renderer, and no rewriting, so a briefing carrying <script>
// tags round-trips identically. The content type is set explicitly (asset.serve's
// discipline) with nosniff; the bytes are never sniffed or transcoded. A file
// that vanished or cannot be read between the ~2s snapshot and this request is a
// 404, not a 500: the report list is a snapshot, so a gone file is a not-found,
// fail-open — degrade, never break.
func (h *Handler) raw(w http.ResponseWriter, r *http.Request) {
	rep, ok := resolveReport(h.deps.Nav(), r.PathValue("name"))
	if !ok {
		http.NotFound(w, r)
		return
	}

	// rep.RelPath is scanner-produced, so the allowlist above already confines it
	// to System/reports/daily-briefing/. Re-assert two independent properties at
	// this raw-file boundary as defense-in-depth (verify the served path stays
	// within the allowed directory): IsLocal keeps it within the vault root
	// (no served path may escape the vault), and the prefix keeps it under
	// System/reports/ — so even a future scanner bug that set Briefing on a note
	// elsewhere in the vault could never be served here, in particular never a
	// Diary/ file (private; unconditionally excluded from every egress path).
	// reportsRoot mirrors nav.buildReports' own scan root.
	const reportsRoot = "System/reports/"
	if !filepath.IsLocal(rep.RelPath) || !strings.HasPrefix(rep.RelPath, reportsRoot) {
		http.NotFound(w, r)
		return
	}
	b, err := os.ReadFile(filepath.Join(h.deps.Root, filepath.FromSlash(rep.RelPath)))
	if err != nil {
		h.deps.Log.Warn("read report", "name", rep.Name, "path", rep.RelPath, "error", err)
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// Never cached: the file is re-read from disk each request, so the mutable
	// latest.html always serves the newest briefing — the "always the latest"
	// this route promises must hold in the browser too, not only server-side.
	w.Header().Set("Cache-Control", "no-store")
	// Enforce the sandbox on the resource itself, not only through the shell's
	// iframe attribute: a CSP sandbox lands this script-bearing briefing in a
	// unique opaque origin however it is loaded — inside kurodo's shell, framed
	// cross-origin, or opened top-level ("open frame in new tab"). Without it the
	// containment would depend on the embedder, so a hostile briefing (a
	// prompt-injected brief, or a compromised CDN script it loads) opened outside
	// the shell would run same-origin to the whole vault-reading surface and
	// could read and exfiltrate it — including Diary/, a hard never-egress.
	// allow-scripts, never allow-same-origin, mirrors the shell iframe exactly,
	// so the briefing still renders; frame-ancestors 'self' also
	// refuses cross-origin framing of raw vault HTML — kurodo's own shell is the
	// only legitimate embedder.
	w.Header().Set("Content-Security-Policy", "sandbox allow-scripts; frame-ancestors 'self'")
	_, _ = w.Write(b)
}
