package note

import (
	"net/http"

	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/wording"
)

// langFormMaxBytes bounds the POST /lang body: two short form fields never
// need more than this.
const langFormMaxBytes = 4096

// langCookieMaxAgeSeconds keeps the stored choice for a year — the same span
// the client-side preference writes use, so a choice made with script and one
// made without expire together.
const langCookieMaxAgeSeconds = 31536000

// language applies the interface-language choice: store the cookie every page
// build reads, then send the reader back to the page the form came from. It is
// a plain form target rather than a scripted exchange because the words on a
// page are the server's — no script can retranslate a rendered document, so
// the honest mechanism is to store the choice and ask for the page again, and
// that mechanism works exactly as well with scripting disabled. This is the
// interface's own preference, not a vault write: no note file is touched.
func (h *Handler) language(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, langFormMaxBytes)
	if err := r.ParseForm(); err != nil {
		http.Error(w, wording.LanguageFormUnreadable.In(origin.Language(r)), http.StatusBadRequest)
		return
	}
	choice, spoken := wording.Known(r.PostFormValue("lang"))
	if !spoken {
		// An unknown value is refused outright rather than silently
		// normalised: the reader asked for something, and answering with a
		// redirect would hand back a receipt for a change nothing made. The
		// rendered form only ever offers the two spoken values, so this
		// answer is for hand-built requests.
		http.Error(w, wording.LanguageUnknown.In(origin.Language(r)), http.StatusUnprocessableEntity)
		return
	}
	// #nosec G124 -- deliberately neither Secure nor HttpOnly: the server is
	// loopback HTTP by design, and the client-side cache-restore check must be
	// able to read the value to compare it against the revived document.
	http.SetCookie(w, &http.Cookie{
		Name:     wording.CookieName,
		Value:    string(choice),
		Path:     "/",
		MaxAge:   langCookieMaxAgeSeconds,
		SameSite: http.SameSiteLaxMode,
	})
	// #nosec G710 -- LocalNext admits only a same-site absolute path; every
	// other shape of the client-controlled field falls back to Home.
	http.Redirect(w, r, origin.LocalNext(r.PostFormValue("next")), http.StatusSeeOther)
}
