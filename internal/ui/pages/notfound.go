package pages

import (
	"context"
	"net/http"

	"github.com/koopa0/yomihon/internal/ui/layouts"
)

// WriteNotFound answers one request with the shared not-found page: the content
// type, the 404 status, and the page itself, in that order. It exists because
// three faces refuse a name they do not hold — a note, a report, a study path —
// and each had written those three steps out. Three copies of a response are
// three chances for one of them to answer a missing name with a different
// status or no content type at all, and nothing about the pages would look
// wrong while it happened.
//
// What each face keeps is what actually differs: the title over the page, since
// a study path names its own refusal, and the sentence it logs when the write
// fails, since that names the route the reader was on.
func WriteNotFound(
	ctx context.Context,
	w http.ResponseWriter,
	//nolint:gocritic // hugeParam: the component this hands them to takes both
	// by value, and a pointer here would let a caller keep changing what the
	// renderer is already writing out.
	view NotFoundView,
	//nolint:gocritic // hugeParam: see the view above.
	chrome layouts.Chrome,
) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	return NotFound(view, chrome).Render(ctx, w)
}
