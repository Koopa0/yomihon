package report

import (
	"net/http"

	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/ui/pages"
)

// index lists every report the vault holds: the written ones, then the daily
// briefings. Names and paths are all navigation captured, and nothing here
// opens a file — what a report says is the author's, and a summary yomihon
// wrote would be a description of it rather than the thing itself.
func (h *Handler) index(w http.ResponseWriter, r *http.Request) {
	lang := origin.Language(r)
	view := pages.NewReportIndex(h.snapshot().Shell.Nav.Reports(), lang)
	if err := pages.ListIndex(view, layouts.ChromeFromRequest(r, view.Title)).Render(r.Context(), w); err != nil {
		h.log.Log(r.Context(), origin.WriteFailureLevel(r, err), "write report index", "error", err)
	}
}
