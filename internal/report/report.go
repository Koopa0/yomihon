// Package report serves the shelf of reports: the Markdown files written
// directly in System/reports/. A report opens on the reading page like any
// other note, so this package lists them and serves nothing else.
package report

import (
	"log/slog"
	"net/http"

	"github.com/koopa0/yomihon/internal/snapshot"
)

// Handler serves the report shelf from the current reading generation.
type Handler struct {
	snapshot func() *snapshot.Generation
	log      *slog.Logger
}

// New wires the reports feature. Every dependency must be non-nil: a nil is a
// wiring bug that must fail here, not on the first request.
func New(snapshotProvider func() *snapshot.Generation, log *slog.Logger) *Handler {
	if snapshotProvider == nil {
		panic("report: New requires a non-nil Snapshot provider")
	}
	if log == nil {
		panic("report: New requires a non-nil Log")
	}
	return &Handler{snapshot: snapshotProvider, log: log}
}

// Register mounts the report shelf.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /reports", h.index)
}
