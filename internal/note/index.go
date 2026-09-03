package note

import (
	"net/http"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/shell"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/ui/pages"
)

// maps lists every map the vault declares. Like the study-path index it refuses
// what the rail refused: a withheld declaration lists nothing and states its
// reason, so the page cannot become a second route to a projection the contract
// closed.
func (h *Handler) maps(w http.ResponseWriter, r *http.Request) {
	lang := origin.Language(r)
	model := shell.Project(h.sources.Status(), h.sources.Snapshot().Capture()).Nav
	closure := model.DeclaredClosure()
	var declared []nav.Map
	if !closure.Closed() {
		declared = model.Maps()
	}
	view := pages.NewMapIndex(declared, lang)
	view.Fault = closure.Diagnostic()
	if err := pages.ListIndex(view, layouts.ChromeFromRequest(r, view.Title)).Render(r.Context(), w); err != nil {
		h.sources.Log.Log(r.Context(), origin.WriteFailureLevel(r, err), "write map index", "error", err)
	}
}

// folders lists the vault's own directory tree. It is plain reading — file
// names and where they sit — so no declaration gates it: a vault whose contract
// broke must not show less of its own folders than one that never carried a
// contract, because mending the contract is done while reading the vault it
// governs.
func (h *Handler) folders(w http.ResponseWriter, r *http.Request) {
	lang := origin.Language(r)
	model := shell.Project(h.sources.Status(), h.sources.Snapshot().Capture()).Nav
	view := pages.NewFolderIndex(model, lang)
	if err := pages.FolderIndex(view, layouts.ChromeFromRequest(r, view.Title)).Render(r.Context(), w); err != nil {
		h.sources.Log.Log(r.Context(), origin.WriteFailureLevel(r, err), "write folder index", "error", err)
	}
}
