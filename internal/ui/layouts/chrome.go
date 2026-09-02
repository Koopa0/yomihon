package layouts

import (
	"net/http"

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

// LanguageFromRequest reads which language this request asked the interface to
// speak. It sits beside the other preference reads rather than in the
// dictionary, so the dictionary stays a set of sentences with no capability of
// its own and no import at all.
func LanguageFromRequest(r *http.Request) wording.Lang {
	c, err := r.Cookie(wording.CookieName)
	if err != nil {
		return wording.ZhHant
	}
	return wording.FromCookieValue(c.Value)
}

// ChromeFromRequest builds the page chrome from the request: the page title
// plus the persisted theme, furigana, and single-key-shortcut cookies (default
// light / on / on), so the root element renders the correct state on the first
// byte (no FOUC). Each
// cookie honors only its known values; anything else falls to the default —
// input hygiene, since a cookie is user-controllable.
//
// It takes no shell: what the chrome is built from is the request and nothing
// else, and a snapshot projection passed alongside would say the two were
// related when they never were.
func ChromeFromRequest(r *http.Request, title string) Chrome {
	theme := "light"
	if c, err := r.Cookie("yomihon_theme"); err == nil && c.Value == "dark" {
		theme = "dark"
	}
	ruby := "on"
	if c, err := r.Cookie("yomihon_ruby"); err == nil && c.Value == "off" {
		ruby = "off"
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
		Lang:                      LanguageFromRequest(r),
		Nonce:                     origin.Nonce(r.Context()),
		Theme:                     theme,
		Ruby:                      ruby,
		TextSize:                  textSize,
		SingleKeyShortcutsEnabled: singleKeyShortcutsEnabled,
	}
}
