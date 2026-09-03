// Package syllabus serves the study-path page: one study-path tree with a
// switcher across every study-path in the vault. It reads paths the navigation
// model already parsed, and parses no note itself.
package syllabus

import (
	"log/slog"
	"net/http"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

// Handler serves the study-path page.
type Handler struct {
	shell func() nav.Shell
	log   *slog.Logger
}

// New wires the syllabus feature. Every dependency must be non-nil: a nil
// is a wiring bug that must fail here, not on the first request.
func New(shell func() nav.Shell, log *slog.Logger) *Handler {
	if shell == nil {
		panic("syllabus: New requires a non-nil Shell provider")
	}
	if log == nil {
		panic("syllabus: New requires a non-nil Log")
	}
	return &Handler{shell: shell, log: log}
}

// Register mounts the feature's route.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /syllabus/{path...}", h.show)
}

// show renders the study-path whose vault path matches the request. An unknown
// path, and an empty model before the first scan, both answer the not-found
// page rather than a line of text.
func (h *Handler) show(w http.ResponseWriter, r *http.Request) {
	// A vault holds its names composed and a request can carry either spelling
	// of the same letter, so the name is composed before it is looked up.
	rel := vault.NormalizeNFC(r.PathValue("path"))

	shell := h.shell()
	current := shell.Nav.Path(rel)
	if current == nil {
		lang := origin.Language(r)
		view := pages.NotFoundView{Asked: r.URL.Path, Sidebar: pages.NewSidebar(shell.Nav, "")}
		// The title names which route refused; the page below it is shared.
		chrome := layouts.ChromeFromRequest(r, wording.PathNotFound.In(lang))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		if err := pages.NotFound(view, chrome).Render(r.Context(), w); err != nil {
			h.log.Log(r.Context(), origin.WriteFailureLevel(r, err), "write study-path not-found page", "path", rel, "error", err)
		}
		return
	}

	view := pages.BuildPathView(current, shell.Nav.Paths())
	if err := pages.Syllabus(view, layouts.ChromeFromRequest(r, current.Title)).Render(r.Context(), w); err != nil {
		h.log.Log(r.Context(), origin.WriteFailureLevel(r, err), "write syllabus page", "path", rel, "error", err)
	}
}
