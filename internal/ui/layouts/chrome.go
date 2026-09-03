package layouts

import (
	"net/http"
	"net/url"

	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/wording"
)

// The three answers the header's own controls need in words: the state a
// checkbox is in, the language the switch leads to, and the measure the
// reading column is set at. They are here rather than beside the markup
// because Go written inside a template reaches the compiler only as
// generated output, which every linter in this repository is told to skip.

func singleKeyShortcutsState(enabled bool) string {
	if enabled {
		return "on"
	}
	return "off"
}

// languageMark labels the language control with the language it switches to
// rather than the one in force. A control that names the current state reads as
// a status line, and a reader who cannot read the interface they are looking at
// needs to recognise the way out of it.
func languageMark(lang wording.Lang) string {
	if lang == wording.En {
		return "中"
	}
	return "EN"
}

// textSizeLabel is the text-size control's accessible name, which carries the
// size the reader is at rather than the action the press performs. Its two
// neighbours in the header are two-state and say where they stand through
// aria-pressed; this one cycles through three, which no boolean carries, so
// the name is where its state has to live. The script keeps it current on
// every press, and a reader arriving on a reload finds the same answer.
func textSizeLabel(size string, lang wording.Lang) string {
	switch size {
	case "l":
		return wording.TextSizeLarge.In(lang)
	case "xl":
		return wording.TextSizeExtraLarge.In(lang)
	default:
		return wording.TextSizeMedium.In(lang)
	}
}

// ChromeFromRequest builds the page chrome from the request: the page title
// plus the reading choices the browser carries, so the root element renders
// the correct state on the first byte (no FOUC). Every one of those choices is
// read through the same table, which names the values each cookie may carry;
// anything else falls to that row's answer — input hygiene, since a cookie is
// user-controllable.
//
// The theme's and the typeface's answers are deliberately empty rather than a
// named default: a reader who never chose has expressed no preference there,
// and the stylesheet answers an unstamped root with its own base value — the
// system's own preference, in the theme's case. Both stored themes are
// honored, because an explicit light choice must keep beating a dark system.
//
// It takes no shell: what the chrome is built from is the request and nothing
// else, and a snapshot projection passed alongside would say the two were
// related when they never were.
func ChromeFromRequest(r *http.Request, title string) Chrome {
	return Chrome{
		Title:                     title,
		Lang:                      wording.LanguageFromRequest(r),
		Nonce:                     origin.Nonce(r.Context()),
		Theme:                     read(r, themeChoice),
		Ruby:                      read(r, rubyChoice),
		TextSize:                  read(r, textSizeChoice),
		Font:                      read(r, fontChoice),
		SingleKeyShortcutsEnabled: read(r, shortcutsChoice) == "on",
		// The request's own address, so a control the server answers can bring
		// the reader back to this page afterwards. Only an address a GET can
		// revisit qualifies: a page rendered by a POST names a target, not a
		// place, so it falls back to Home — and a page that knows a better
		// return, as the recovery page knows its note, overrides this
		// afterwards.
		ReturnTo: ReturnableAddress(r),
	}
}

// ReturnableAddress is the address a control the server answers sends a reader
// back to: a GET's own path and query, or Home when the page came from a POST,
// whose address names a target rather than a place a reader can revisit.
func ReturnableAddress(r *http.Request) string {
	if r.Method == http.MethodGet {
		return r.URL.RequestURI()
	}
	return "/"
}

// preferencesHref builds the address of the page a reader sets their reading
// choices on, carrying where they are so that page can send them back there. A
// reader already on it carries nothing: an address pointing at its own page
// needs no return, and one carrying its own would nest another copy of itself
// on every visit. Only the encoding happens here — whether the address names
// somewhere this site can return to is answered where it is followed.
func preferencesHref(returnTo string) string {
	if returnTo == "" || returnTo == "/preferences" {
		return "/preferences"
	}
	return "/preferences?from=" + url.QueryEscape(returnTo)
}

// otherLanguage is the language the form asks for: the one the interface is
// not speaking. It is the same answer languageMark draws, held as a value the
// server can store rather than a glyph a reader can recognise.
func otherLanguage(lang wording.Lang) wording.Lang {
	if lang == wording.En {
		return wording.ZhHant
	}
	return wording.En
}
