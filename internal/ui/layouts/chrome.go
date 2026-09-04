package layouts

import (
	"net/http"

	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/wording"
)

// Chrome is the per-request shell state rendered server-side: the page title
// and the persisted theme / furigana / text-size / single-key-shortcut signals
// stamped onto the root so the correct state paints on the first byte (no
// FOUC).
// The client runtime flips these attributes and their cookies; CSS reacts (see
// assets/css/*). The server-rendered page remains usable without enhancement.
type Chrome struct {
	Title string // page title (before " — yomihon")
	Nonce string // per-response nonce for application-owned scripts
	// Theme is the reader's stored choice: "light", "dark", or "" when they
	// never made one. A choice is stamped on the root so the first paint is
	// already right; no choice leaves the root unstamped and the stylesheet's
	// system-preference block decides.
	Theme string
	// RubyEnabled is whether furigana are shown, which the reader chooses and
	// the root carries so the first paint already matches.
	RubyEnabled bool
	// TextSize scales the reading column: "m" (the default measure), "l", or
	// "xl". It tunes presentation only — the charter's bounded reading
	// preferences — and the chrome around the article keeps its own size.
	TextSize                  string
	SingleKeyShortcutsEnabled bool // plain / and [ are enabled
	// Lang is the language the interface speaks for this request, and the
	// language the document declares. The article inside it may declare its
	// own; that answers what the note is written in, which is a different
	// question from what the interface is saying.
	Lang wording.Lang
	// ReturnTo is this request's own address — path and query — carried into
	// the language form so the redirect after the switch lands the reader back
	// on the page they were reading. The server validates it as a local path
	// before following it.
	ReturnTo string
}

// The three answers the header's own controls need in words: the state a
// checkbox is in, the language the switch leads to, and the measure the
// reading column is set at. They are here rather than beside the markup
// because Go written inside a template reaches the compiler only as
// generated output, which every linter in this repository is told to skip.

// onOff spells a two-state reading preference the way the root attribute and the
// cookie behind it are written, which is the same two words for every one of
// them.
func onOff(enabled bool) string {
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
// plus the persisted theme, furigana, and single-key-shortcut cookies, so the
// root element renders the correct state on the first byte (no FOUC). Each
// cookie honors only its known values; anything else falls to the default —
// input hygiene, since a cookie is user-controllable.
//
// The theme's default is deliberately empty rather than light: a reader who
// never chose has expressed no preference here, and the stylesheet answers an
// unstamped root with the system's own preference. Both stored values are
// honored, because an explicit light choice must keep beating a dark system.
//
// It takes no shell: what the chrome is built from is the request and nothing
// else, and a snapshot projection passed alongside would say the two were
// related when they never were.
func ChromeFromRequest(r *http.Request, title string) Chrome {
	theme := ""
	if c, err := r.Cookie("yomihon_theme"); err == nil && (c.Value == "dark" || c.Value == "light") {
		theme = c.Value
	}
	rubyEnabled := true
	if c, err := r.Cookie("yomihon_ruby"); err == nil && c.Value == "off" {
		rubyEnabled = false
	}
	textSize := "m"
	if c, err := r.Cookie("yomihon_textsize"); err == nil && (c.Value == "l" || c.Value == "xl") {
		textSize = c.Value
	}
	singleKeyShortcutsEnabled := true
	if c, err := r.Cookie("yomihon_shortcuts"); err == nil && c.Value == "off" {
		singleKeyShortcutsEnabled = false
	}
	return Chrome{
		Title:                     title,
		Lang:                      origin.Language(r),
		Nonce:                     origin.Nonce(r.Context()),
		Theme:                     theme,
		RubyEnabled:               rubyEnabled,
		TextSize:                  textSize,
		SingleKeyShortcutsEnabled: singleKeyShortcutsEnabled,
		// The request's own address, so the language form can bring the reader
		// back to this page after the switch. Only an address a GET can revisit
		// qualifies: a page rendered by a POST names a target, not a place, so
		// the form falls back to Home — and a page that knows a better return,
		// as the recovery page knows its note, overrides this afterwards.
		ReturnTo: returnableAddress(r),
	}
}

// returnableAddress is the address the language form sends a reader back to: a
// GET's own path and query, or Home when the page came from a POST, whose
// address names a target rather than a place a reader can revisit.
func returnableAddress(r *http.Request) string {
	if r.Method == http.MethodGet {
		return r.URL.RequestURI()
	}
	return "/"
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
