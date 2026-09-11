// Package preference serves the page a reader sets the reading on, and applies
// what they choose there.
//
// Nothing chosen here is written into the vault. Each choice lands in one
// cookie on this machine's own address, which the page shell reads back on the
// next request so the first painted byte already carries it. What values a
// cookie may hold, and what a cookie holding none of them means, is answered
// by the table that shell keeps: this package adds the words for those values,
// the form that submits one of them, and the answer to a request no rendered
// form could have sent.
//
// Every choice is applied by an ordinary form post — one field, one value, a
// stored cookie and a redirect back to where the reader was. That path is the
// whole mechanism rather than a fallback: the interface language cannot be
// changed by any script, because the words on a rendered page are the server's,
// and a page that works without scripting for the hardest of these choices may
// as well work without it for all of them.
package preference

import (
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/wording"
)

const (
	// formMaxBytes bounds a submitted body. One enumerated value and one
	// same-site path never need more than this, and one field per form is what
	// keeps the bound this small.
	formMaxBytes = 4096

	// cookieMaxAgeSeconds keeps a stored choice for a year — the same span the
	// client-side preference writes use, so a choice made with script and one
	// made without expire together.
	cookieMaxAgeSeconds = 31536000

	// address is where the page answers and where every form on it posts.
	address = "/preferences"

	// nextField carries the address the reader came from, so applying a choice
	// returns them to what they were reading instead of leaving them here.
	nextField = "next"

	// fromParam carries that same address into the page, so the forms it
	// renders already know where to send the reader back to.
	fromParam = "from"

	// resetField is the last form's only field, and resetValue the only value
	// it may carry. A control that clears everything is worth refusing on a
	// typo rather than acting on one.
	resetField = "reset"
	resetValue = "1"

	// systemTheme is the ground option that stores nothing. Light and dark are
	// the two values a cookie may carry; "whatever the operating system says"
	// is the state the cookie's absence holds, and no stored value could hold
	// it without the stylesheet having to ask the question twice. Submitting
	// it therefore deletes the cookie.
	systemTheme = "system"

	// unchosenTypeface is the face the reading column is set in while no face
	// is stamped on the root. It is a storable value like the other two, and it
	// is named here so a reader who has chosen nothing still sees which face
	// they are looking at rather than a heading with no answer under it.
	unchosenTypeface = "serif"

	// The two words a switch on this page is submitted with. The shell's table
	// owns them; they are spelled here only to turn the answer it hands back
	// about the single-key shortcuts — which arrives as a boolean — into the
	// word the form sends.
	stateOn  = "on"
	stateOff = "off"
)

// Dependencies is what this face needs from the composition root. It is a
// short list because the page reads nothing from the vault: what it shows is
// the request's own cookies and the words for them.
type Dependencies struct {
	Log *slog.Logger
}

// Handler answers for the reading choices this browser stores. It holds none
// of them: a choice lives in the request that carries it and in the cookie
// that request goes back with.
type Handler struct {
	deps Dependencies
}

// New wires the preferences face. Every reference must be non-nil: a wiring
// bug fails here rather than on the first request.
func New(d *Dependencies) *Handler {
	if d == nil {
		panic("preference: New requires a non-nil Dependencies")
	}
	if d.Log == nil {
		panic("preference: New requires a non-nil Log")
	}
	return &Handler{deps: *d}
}

// Register mounts the page and the form it posts to. Both patterns name their
// method, so a request arriving with any other one is answered by the router
// with the methods that are mounted rather than by a branch written here.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET "+address, h.page)
	mux.HandleFunc("POST "+address, h.apply)
}

// choice is one row of the page and one field of the form that applies it: the
// name it is submitted under, the cookie it lands in, the sentences that
// introduce it, and the word each of its values reads as.
//
// The values themselves are not here. They come from the table the page shell
// keeps, so this file can put a word beside each of them and can neither
// lengthen nor shorten the list.
type choice struct {
	name   string
	cookie string
	legend wording.Phrase
	note   wording.Phrase
	labels map[string]wording.Phrase
	// whenUnset is the option that answers for a browser storing nothing here.
	// It is empty wherever the shell's table already falls back to a value a
	// cookie may carry, because the shell then hands back that value and the
	// page has nothing extra to say.
	//
	// Where the fallback is no value at all, this names what the reader is in
	// fact looking at. A value among the storable ones is shown as chosen and
	// stored like any other; one that is not among them — the ground's
	// "whatever the system says" — is offered as an option and deletes the
	// cookie when it is submitted, which is the only way to hold that state.
	whenUnset string
	// inForce reads this choice's current answer out of the same shell the page
	// is rendered inside, so what the page shows as chosen and what the first
	// paint carries cannot disagree.
	inForce func(*layouts.Chrome) string
}

// choices is what the page offers, in the order it offers it. The language
// comes first because it decides what every other row on the page says.
var choices = []choice{
	{
		name:   "lang",
		cookie: wording.CookieName,
		legend: wording.PrefLanguage,
		note:   wording.PrefLanguageNote,
		labels: map[string]wording.Phrase{
			string(wording.ZhHant): wording.PrefLanguageZh,
			string(wording.En):     wording.PrefLanguageEn,
		},
		inForce: func(c *layouts.Chrome) string { return string(c.Lang) },
	},
	{
		name:   "theme",
		cookie: "yomihon_theme",
		legend: wording.PrefAppearance,
		note:   wording.PrefAppearanceNote,
		labels: map[string]wording.Phrase{
			systemTheme: wording.PrefAppearanceSystem,
			"light":     wording.PrefAppearanceLight,
			"dark":      wording.PrefAppearanceDark,
		},
		whenUnset: systemTheme,
		inForce:   func(c *layouts.Chrome) string { return c.Theme },
	},
	{
		name:   "textsize",
		cookie: "yomihon_textsize",
		legend: wording.PrefTextSize,
		note:   wording.PrefTextSizeNote,
		labels: map[string]wording.Phrase{
			"m":  wording.TextSizeMedium,
			"l":  wording.TextSizeLarge,
			"xl": wording.TextSizeExtraLarge,
		},
		inForce: func(c *layouts.Chrome) string { return c.TextSize },
	},
	{
		name:   "font",
		cookie: "yomihon_font",
		legend: wording.PrefTypeface,
		note:   wording.PrefTypefaceNote,
		labels: map[string]wording.Phrase{
			unchosenTypeface: wording.PrefTypefaceSerif,
			"sans":           wording.PrefTypefaceSans,
			"kai":            wording.PrefTypefaceKai,
		},
		whenUnset: unchosenTypeface,
		inForce:   func(c *layouts.Chrome) string { return c.Font },
	},
	{
		name:   "ruby",
		cookie: "yomihon_ruby",
		legend: wording.PrefFurigana,
		note:   wording.PrefFuriganaNote,
		labels: map[string]wording.Phrase{
			stateOn:  wording.PrefOn,
			stateOff: wording.PrefOff,
		},
		inForce: func(c *layouts.Chrome) string { return c.Ruby },
	},
	{
		name:   "shortcuts",
		cookie: "yomihon_shortcuts",
		legend: wording.PrefShortcuts,
		note:   wording.PrefShortcutsNote,
		labels: map[string]wording.Phrase{
			stateOn:  wording.PrefOn,
			stateOff: wording.PrefOff,
		},
		inForce: func(c *layouts.Chrome) string {
			if c.SingleKeyShortcutsEnabled {
				return stateOn
			}
			return stateOff
		},
	},
}

// honouredValues answers, for every cookie this page writes, the values it may
// carry. Five of them come from the shell's own table, which is where a reader
// gets to say what a stored value means; the sixth is the interface language,
// whose two values belong to the dictionary the words themselves come from.
func honouredValues() map[string][]string {
	stored := layouts.Preferences()
	out := make(map[string][]string, len(stored)+1)
	for _, p := range stored {
		out[p.Cookie] = p.Values
	}
	out[wording.CookieName] = []string{string(wording.ZhHant), string(wording.En)}
	return out
}

// options lists what this choice offers, in the order the page shows them: the
// option that stores nothing first where there is one, then the values a
// cookie may carry.
func (ch *choice) options(stored []string) []string {
	if ch.whenUnset == "" || slices.Contains(stored, ch.whenUnset) {
		return stored
	}
	return append([]string{ch.whenUnset}, stored...)
}

// chosen says whether option is the one this request has settled on. A browser
// carrying nothing settles on the option that stands for having chosen
// nothing, so exactly one of a choice's options is always marked.
func (ch *choice) chosen(option, current string) bool {
	if current == "" {
		return option == ch.whenUnset
	}
	return option == current
}

// honours says whether this choice can be applied with value: one of the
// values a cookie may carry, or the option that stands for storing none.
func (ch *choice) honours(value string, stored []string) bool {
	return slices.Contains(stored, value) || (ch.whenUnset != "" && value == ch.whenUnset)
}

// localNext validates the address the form asks to return to. The field is
// client-controlled bytes, so only a same-site absolute path survives:
// anything else — an empty value, a full URL, a protocol-relative or
// backslashed address a browser would read as one — falls back to Home rather
// than carrying the reader somewhere the form never stood.
func localNext(next string) string {
	// A control byte is refused before any shape check. This side writes the
	// value into a header where a tab or a delete survives, and the WHATWG URL
	// parser on the receiving side strips such bytes before it reads the
	// shape — so "/\t/host" leaves here as a same-site path and arrives as a
	// protocol-relative address. No address a page's own form carries contains
	// one, so the fallback refuses no honest request.
	for i := range len(next) {
		if next[i] < 0x20 || next[i] == 0x7f {
			return "/"
		}
	}
	if next == "" || next[0] != '/' {
		return "/"
	}
	if strings.HasPrefix(next, "//") || strings.HasPrefix(next, `/\`) {
		return "/"
	}
	return next
}
