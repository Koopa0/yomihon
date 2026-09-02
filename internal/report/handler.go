package report

import (
	"bytes"
	"net/http"

	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/wording"
)

// briefingSandbox is what an agent-authored briefing is served under. It is
// the vault-file sandbox with one difference: data: fonts, images and media
// are admitted, because a briefing carries its own charts and pictures inline
// and would otherwise render as a page full of holes.
const briefingSandbox = "sandbox; default-src 'none'; base-uri 'none'; connect-src 'none'; " +
	"font-src data:; form-action 'none'; frame-ancestors 'self'; frame-src 'none'; " +
	"img-src data:; media-src data:; object-src 'none'; " +
	"script-src 'none'; script-src-attr 'none'; " +
	"style-src 'unsafe-inline'; worker-src 'none'"

// Register mounts the reports face's routes: the shell page and the verbatim
// raw endpoint the shell's iframe points at. ServeMux matches the more specific
// /raw pattern first, so the two never collide.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /reports/{name}", h.show)
	mux.HandleFunc("GET /reports/{name}/raw", h.raw)
}

// show renders the report shell: header + sidebar + title + a sandboxed iframe
// whose src is this report's /raw endpoint. An unknown name (or one that is not
// an enumerated briefing) is a localized 404 — the same fail-quiet stance the
// reading page takes for a missing note.
func (h *Handler) show(w http.ResponseWriter, r *http.Request) {
	snap := h.current().Capture()
	rep, ok := resolveReport(snap.Navigation(), r.PathValue("name"))
	if !ok {
		http.Error(w, wording.ReportNotFound.In(layouts.LanguageFromRequest(r)), http.StatusNotFound)
		return
	}
	shell := h.shell(snap)

	// The frame refuses scripts, which is right — a briefing that fetches from a
	// CDN would be reaching off this machine. Reading the bytes here is what
	// lets the page say so instead of showing a hole.
	body, err := readReport(r.Context(), h.source, snap, rep.RelPath)
	if err != nil {
		h.log.Warn("read report for the shell", "name", rep.Name, "error", err)
	}
	view := pages.ReportView{
		Name:        rep.Name,
		Sidebar:     pages.NewSidebar(shell.Nav, rep.RelPath),
		NeedsScript: bytes.Contains(bytes.ToLower(body), []byte("<script")),
	}
	if err := pages.Report(view, layouts.ChromeFromRequest(r, rep.Name)).Render(r.Context(), w); err != nil {
		h.log.Error("write report page", "name", rep.Name, "error", err)
	}
}

// raw serves the briefing HTML byte-for-byte: the file the allowlist resolved,
// read fresh each request (never cached — always the latest on disk), written
// with no templ, no renderer, and no rewriting, so a briefing carrying <script>
// tags round-trips identically. The content type is set explicitly (asset.serve's
// discipline) with nosniff; the bytes are never sniffed or transcoded. A file
// that vanished or cannot be read between the ~2s snapshot and this request is a
// 404, not a 500: the report list is a snapshot, so a gone file is a not-found,
// fail-quiet response rather than a server failure.
func (h *Handler) raw(w http.ResponseWriter, r *http.Request) {
	snap := h.current().Capture()
	rep, ok := resolveReport(snap.Navigation(), r.PathValue("name"))
	if !ok {
		http.Error(w, wording.ReportNotFound.In(layouts.LanguageFromRequest(r)), http.StatusNotFound)
		return
	}

	b, err := readReport(r.Context(), h.source, snap, rep.RelPath)
	if err != nil {
		h.log.Warn("read report", "name", rep.Name, "path", rep.RelPath, "error", err)
		http.Error(w, wording.ReportNotFound.In(layouts.LanguageFromRequest(r)), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// Never cached: the file is re-read from disk each request, so the mutable
	// latest.html always serves the newest briefing — the "always the latest"
	// this route promises must hold in the browser too, not only server-side.
	w.Header().Set("Cache-Control", "no-store")
	// Enforce the sandbox on the resource itself, not only through the shell's
	// iframe attribute: a bare CSP sandbox lands this agent-authored briefing in a
	// unique opaque origin however it is loaded — inside yomihon's shell, framed
	// cross-origin, or opened top-level ("open frame in new tab"). Without it the
	// containment would depend on the embedder. The sandbox grants no capability:
	// scripts and automatic features such as meta refresh remain disabled, while
	// self-contained HTML, CSS, SVG, and data media still render. This is the
	// browser-level form that preserves the report's zero-automatic-egress contract;
	// an opaque origin alone does not prevent script-driven navigation or WebRTC.
	// frame-ancestors 'self' also
	// refuses cross-origin framing of raw vault HTML — yomihon's own shell is the
	// only legitimate embedder.
	//
	// The briefing is written by an agent and served verbatim, so the sandbox
	// is the whole of its containment: it is written out only once that policy
	// is known to reach the reader, and withheld otherwise. Under the reading
	// shell's own policy this same document would be a first-party page with
	// script access to the surface around it.
	if !origin.SetContentSecurityPolicy(r.Context(), w, briefingSandbox) {
		h.log.Error("serve report bytes", "name", rep.Name, "path", rep.RelPath,
			"error", "the response sandbox could not be established")
		http.Error(w, wording.SandboxUnavailable.In(layouts.LanguageFromRequest(r)), http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(b) //nolint:errcheck // response is committed and Handler has no later recovery channel
}
