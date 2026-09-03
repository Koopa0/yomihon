package preference

import (
	"net/http"
	"net/url"
	"slices"

	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/wording"
)

// page renders the choices with the reader's current answers already marked,
// and every form on it addressed back to wherever they came from.
func (h *Handler) page(w http.ResponseWriter, r *http.Request) {
	lang := wording.LanguageFromRequest(r)
	chrome := layouts.ChromeFromRequest(r, wording.PreferencesTitle.In(lang))
	// The shell's own answer to "where should a control send the reader back
	// to" is this request's address, which on this page is the page itself.
	// Left alone it would fold the page's address into its own return, and the
	// entry in the header would carry a copy of this page inside every later
	// copy of it. The address the reader arrived from replaces it, so the forms
	// and that entry read one value.
	chrome.ReturnTo = returnAddress(r)
	if err := pages.Preferences(view(&chrome), chrome).Render(r.Context(), w); err != nil {
		h.deps.Log.Error("write preferences page", "error", err)
	}
}

// apply stores the one choice this submission carries and asks for the page the
// reader was on again. The redirect is what makes the choice visible: the words
// and the ground are decided while a page is built, so the honest answer to a
// change is to build one.
func (h *Handler) apply(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, formMaxBytes)
	lang := wording.LanguageFromRequest(r)
	if err := r.ParseForm(); err != nil {
		http.Error(w, wording.PrefFormUnreadable.In(lang), http.StatusBadRequest)
		return
	}
	if !store(w, r.PostForm) {
		http.Error(w, wording.PrefUnknownValue.In(lang), http.StatusUnprocessableEntity)
		return
	}
	// #nosec G710 -- localNext admits only a same-site absolute path; every
	// other shape of the client-controlled field falls back to Home.
	http.Redirect(w, r, localNext(r.PostFormValue(nextField)), http.StatusSeeOther)
}

// returnAddress is where applying a choice sends the reader. It is the address
// they arrived from, validated here so what the page renders into every form is
// already a same-site path, and the page itself when they arrived carrying
// none — a reader who opened this page directly stays on it.
func returnAddress(r *http.Request) string {
	from := r.URL.Query().Get(fromParam)
	if from == "" {
		return address
	}
	return localNext(from)
}

// view is the page's own answer for this request: every choice in the order the
// page shows them, each carrying the words for its values and which of them is
// currently in force.
func view(c *layouts.Chrome) pages.PreferencesView {
	stored := honouredValues()
	fields := make([]pages.PreferenceField, 0, len(choices))
	for i := range choices {
		ch := &choices[i]
		current := ch.inForce(c)
		options := ch.options(stored[ch.cookie])
		offered := make([]pages.PreferenceOption, 0, len(options))
		for _, option := range options {
			offered = append(offered, pages.PreferenceOption{
				Value:   option,
				Label:   ch.labels[option].In(c.Lang),
				Checked: ch.chosen(option, current),
			})
		}
		fields = append(fields, pages.PreferenceField{
			Name:    ch.name,
			Legend:  ch.legend.In(c.Lang),
			Note:    ch.note.In(c.Lang),
			Options: offered,
		})
	}
	return pages.PreferencesView{Fields: fields, ReturnTo: c.ReturnTo}
}

// store applies the submission, answering false when it carries no change this
// page offers: no field it knows, more than one of them, a field carrying more
// than one answer, or a value the named choice does not honour.
//
// A false answer has written nothing at all. A refusal that had already set one
// cookie would hand the reader a receipt for a change it then declined to make,
// and leave them unable to tell which half happened.
func store(w http.ResponseWriter, form url.Values) bool {
	named := submitted(form)
	if len(named) != 1 {
		return false
	}
	if named[0] == resetField {
		if value, sole := onlyValue(form, resetField); !sole || value != resetValue {
			return false
		}
		// Everything this page can write, cleared together. The interface
		// language goes with them, which is the one consequence a reader would
		// not predict and which the sentence beside the control says out loud.
		for i := range choices {
			clearCookie(w, choices[i].cookie)
		}
		return true
	}
	ch, known := choiceNamed(named[0])
	if !known {
		return false
	}
	value, sole := onlyValue(form, ch.name)
	if !sole {
		return false
	}
	values := honouredValues()[ch.cookie]
	if !ch.honours(value, values) {
		return false
	}
	if slices.Contains(values, value) {
		writeCookie(w, ch.cookie, value)
		return true
	}
	// The option that stores nothing. Deleting the cookie is how that state is
	// held: the stylesheet answers an unstamped root with the system's own
	// preference, and any stored value would be an answer of yomihon's own.
	clearCookie(w, ch.cookie)
	return true
}

// submitted names the fields of this form that mean something here, so a
// submission carrying none of them, or more than one, is told apart from one
// carrying exactly the change a rendered form sends.
func submitted(form url.Values) []string {
	named := make([]string, 0, len(choices)+1)
	for i := range choices {
		if form.Has(choices[i].name) {
			named = append(named, choices[i].name)
		}
	}
	if form.Has(resetField) {
		named = append(named, resetField)
	}
	return named
}

// onlyValue is the single answer a field carries, and false when it carries
// none or several. A rendered form sends one value per field; a body naming one
// field twice has asked for two changes at once, which is the question the
// field count already refuses and which gets the same answer here — otherwise
// the second answer would be dropped in silence and the reader would hold a
// receipt naming only the first.
func onlyValue(form url.Values, field string) (string, bool) {
	sent := form[field]
	if len(sent) != 1 {
		return "", false
	}
	return sent[0], true
}

// choiceNamed finds the choice a form field belongs to.
func choiceNamed(name string) (*choice, bool) {
	for i := range choices {
		if choices[i].name == name {
			return &choices[i], true
		}
	}
	return nil, false
}

// writeCookie stores one choice on this site's own address for a year.
func writeCookie(w http.ResponseWriter, name, value string) {
	// #nosec G124 -- deliberately neither Secure nor HttpOnly: the server is
	// loopback HTTP by design, and the client-side cache-restore check must be
	// able to read the value to compare it against the revived document.
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   cookieMaxAgeSeconds,
		SameSite: http.SameSiteLaxMode,
	})
}

// clearCookie removes one stored choice. A negative age is how the standard
// library writes the deletion, and the path matches the one it was written
// under — a deletion addressed anywhere else leaves the value standing.
func clearCookie(w http.ResponseWriter, name string) {
	// #nosec G124 -- the same address and flags the value was written with, so
	// the browser recognises this as the same cookie and drops it.
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Path:     "/",
		MaxAge:   -1,
		SameSite: http.SameSiteLaxMode,
	})
}
