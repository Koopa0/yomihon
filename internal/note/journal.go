package note

import (
	"net/http"
	"time"

	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/shell"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/ui/pages"
)

// journal shows the journal one month at a time. The rail offers the newest few
// entries and nothing else; a journal is read by date, and a list of dates is
// the wrong shape for finding the week you are thinking of.
//
// The month comes from the address, and the month a reader is in now is the
// default. An address naming something that is not a month — a thirteenth one,
// a whole day, a word — is answered with that same default rather than with a
// redirect: there is nothing here to lose by guessing wrong, and a reader who
// mistyped one is already looking at the calendar they can step from.
//
// This is where the clock is read, and the only place: the page is built from
// the month rather than from the moment, so everything that records or measures
// it names a month outright and says the same thing tomorrow.
func (h *Handler) journal(w http.ResponseWriter, r *http.Request) {
	lang := origin.Language(r)
	authority := h.sources.Status()
	// The entries, their days and the language each was written in all come
	// from this one reading of the published pointer. Taken twice, a rebuild
	// between them puts one entry's title in another generation's month.
	snap := h.sources.Snapshot().Capture()
	pageShell := shell.Project(authority, snap)
	model := pageShell.Nav
	month, named := pages.ParseMonth(r.URL.Query().Get(pages.JournalMonthParam))
	if !named {
		month = pages.MonthOf(time.Now())
	}
	view := pages.NewJournalIndex(model.Journal(), month, model.JournalClosure(), lang, articleLanguageLookup(snap))
	if err := pages.JournalIndex(view, layouts.ChromeFromRequest(r, view.Title)).Render(r.Context(), w); err != nil {
		h.sources.Log.Log(r.Context(), origin.WriteFailureLevel(r, err), "write journal index", "month", month.String(), "error", err)
	}
}
