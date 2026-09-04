package syllabus

import (
	"net/http"

	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/ui/pages"
)

// index lists every study path the vault declares. It is the page the rail used
// to stand in for, so it reads the projection the rail read and refuses in the
// same case: a declaration yomihon could not honour lists nothing and says why,
// rather than becoming a second route to a projection that was withheld.
func (h *Handler) index(w http.ResponseWriter, r *http.Request) {
	lang := origin.Language(r)
	model := h.shell().Nav
	view := pages.NewPathIndex(model.Paths(), model.DeclaredClosure(), lang)
	if err := pages.ListIndex(view, layouts.ChromeFromRequest(r, view.Shelf.Title)).Render(r.Context(), w); err != nil {
		h.log.Log(r.Context(), origin.WriteFailureLevel(r, err), "write study-path index", "error", err)
	}
}
