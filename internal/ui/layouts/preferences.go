package layouts

import (
	"net/http"
	"slices"
)

// preference is one reading choice the browser hands back on every request:
// the cookie it is kept under, the values that cookie may carry, and the
// answer when it carries none of them.
//
// A fallback is not always one of the values. A theme or a typeface nobody
// chose answers with the empty string, which says no choice was made and
// leaves the stylesheet's own answer standing — so anything listing what is on
// offer has to treat the fallback as a separate question from the values.
type preference struct {
	cookie   string
	values   []string
	fallback string
}

// The reading choices, one row each, named so a caller can ask for the one it
// means rather than for whichever sits at some position. A cookie is bytes a
// reader can write, so each row names the values it honours and nothing else
// reaches the page.
var (
	themeChoice     = preference{cookie: "yomihon_theme", values: []string{"light", "dark"}, fallback: ""}
	rubyChoice      = preference{cookie: "yomihon_ruby", values: []string{"on", "off"}, fallback: "on"}
	textSizeChoice  = preference{cookie: "yomihon_textsize", values: []string{"m", "l", "xl"}, fallback: "m"}
	shortcutsChoice = preference{cookie: "yomihon_shortcuts", values: []string{"on", "off"}, fallback: "on"}
	fontChoice      = preference{cookie: "yomihon_font", values: []string{"serif", "sans", "kai"}, fallback: ""}
)

// preferences is every reading choice in one place. The interface language is
// not among them: each row here resolves to a short value the root carries for
// the stylesheet to answer, while the language decides which words the server
// writes at all, so it is read beside the dictionary those words come from.
var preferences = [...]preference{themeChoice, rubyChoice, textSizeChoice, shortcutsChoice, fontChoice}

// read answers one choice for this request: the stored value when the cookie
// carries one the row honours, and the row's fallback otherwise. An unknown
// value is not something to report — a cookie is user-controlled input, and a
// value nobody offered reads as a choice never made.
func read(r *http.Request, p preference) string {
	c, err := r.Cookie(p.cookie)
	if err != nil || !slices.Contains(p.values, c.Value) {
		return p.fallback
	}
	return c.Value
}

// Preference is one reading choice seen from outside this package: the name of
// the cookie holding it, the values that cookie may carry, and the answer when
// it carries none of them. The fallback may be a value nobody may store — the
// empty string, meaning the reader has chosen nothing here.
type Preference struct {
	Cookie   string
	Values   []string
	Fallback string
}

// Preferences lists the reading choices the browser stores, in the order they
// are declared here. Every call builds its own slices, so writing through the
// returned Values changes nothing and the next call answers the same as this
// one.
func Preferences() []Preference {
	out := make([]Preference, 0, len(preferences))
	for _, p := range preferences {
		out = append(out, Preference{
			Cookie:   p.cookie,
			Values:   slices.Clone(p.values),
			Fallback: p.fallback,
		})
	}
	return out
}
