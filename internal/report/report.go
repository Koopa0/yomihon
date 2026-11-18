// Package report serves the shelf of reports: the Markdown files written
// directly in System/reports/. A report opens on the reading page like any
// other note, so this package lists them and serves nothing else.
package report

import (
	"log/slog"
	"net/http"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/snapshot"
)

// Request is what one reports page is drawn from: the generation and the shell
// its sidebar is projected from, taken together so they describe one folder.
type Request struct {
	Generation *snapshot.Generation
	Shell      nav.Shell
}

// Handler serves the report shelf from the current reading generation.
type Handler struct {
	current func() Request
	log     *slog.Logger
}

// New wires the reports feature. Every dependency must be non-nil: a nil is a
// wiring bug that must fail here, not on the first request.
func New(provider func() Request, log *slog.Logger) *Handler {
	if provider == nil {
		panic("report: New requires a non-nil Request provider")
	}
	if log == nil {
		panic("report: New requires a non-nil Log")
	}
	return &Handler{current: provider, log: log}
}

// Register mounts the report shelf.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /reports", h.index)
}
