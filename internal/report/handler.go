package report

import (
	"bytes"
	"net/http"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/wording"
)

// briefingSandbox is the vault-file sandbox plus data: fonts, images and
// media, which a briefing carries inline.
const briefingSandbox = "sandbox; default-src 'none'; base-uri 'none'; connect-src 'none'; " +
	"font-src data:; form-action 'none'; frame-ancestors 'self'; frame-src 'none'; " +
	"img-src data:; media-src data:; object-src 'none'; " +
	"script-src 'none'; script-src-attr 'none'; " +
	"style-src 'unsafe-inline'; worker-src 'none'"

// Register mounts the report index, the report shell page, and the raw
// endpoint its iframe points at.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /reports", h.index)
	mux.HandleFunc("GET /reports/{name}", h.show)
	mux.HandleFunc("GET /reports/{name}/raw", h.raw)
}

// show renders the report shell: sidebar, title, and a sandboxed iframe whose
// src is this report's raw endpoint. An unenumerated name is a 404 page.
func (h *Handler) show(w http.ResponseWriter, r *http.Request) {
	lang := origin.Language(r)
	request := h.snapshot()
	snap := request.Generation
	shell := request.Shell
	rep, ok := resolveReport(snap.Navigation(), r.PathValue("name"))
	if !ok {
		h.showNotFound(w, r, lang, shell)
		return
	}

	// The frame refuses scripts, so the shell reads the bytes to say so rather
	// than leaving the reader a silent hole.
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
		h.log.Log(r.Context(), origin.WriteFailureLevel(r, err), "write report page", "name", rep.Name, "error", err)
	}
}

// raw serves the briefing's bytes unchanged, read fresh from disk each
// request. A file that vanished between the snapshot and this request is a 404
// rather than a server failure.
func (h *Handler) raw(w http.ResponseWriter, r *http.Request) {
	lang := origin.Language(r)
	snap := h.snapshot().Generation
	rep, ok := resolveReport(snap.Navigation(), r.PathValue("name"))
	if !ok {
		http.Error(w, wording.ReportNotFound.In(lang), http.StatusNotFound)
		return
	}

	b, err := readReport(r.Context(), h.source, snap, rep.RelPath)
	if err != nil {
		h.log.Warn("read report", "name", rep.Name, "path", rep.RelPath, "error", err)
		http.Error(w, wording.ReportNotFound.In(lang), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// The mutable latest.html is re-read from disk each request, so a cached
	// copy in the browser would serve an older briefing than the one on disk.
	w.Header().Set("Cache-Control", "no-store")
	// The sandbox is set on the resource itself, not only through the shell's
	// iframe attribute, so containment holds however a briefing is loaded.
	if !origin.SetContentSecurityPolicy(r.Context(), w, briefingSandbox) {
		h.log.Error("withhold a report whose sandbox could not be established",
			"name", rep.Name, "path", rep.RelPath)
		http.Error(w, wording.SandboxUnavailable.In(lang), http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(b) //nolint:errcheck // response is committed and Handler has no later recovery channel
}

// showNotFound answers an unenumerated report name with the reading shell, so
// a mistyped name still carries the folder tree, the search and a way home.
func (h *Handler) showNotFound(w http.ResponseWriter, r *http.Request, lang wording.Lang, shell nav.Shell) {
	view := pages.NotFoundView{Asked: r.URL.Path, Sidebar: pages.NewSidebar(shell.Nav, "")}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	if err := pages.NotFound(view, layouts.ChromeFromRequest(r, wording.NotFoundKicker.In(lang))).Render(r.Context(), w); err != nil {
		h.log.Log(r.Context(), origin.WriteFailureLevel(r, err), "write not-found page", "path", r.URL.Path, "error", err)
	}
}
