// Package wording holds what yomihon says in its own voice, in both languages
// it says it in. The vault's words are the author's and pass through untouched;
// these are the interface's own — labels, notices, the sentences an error page
// is made of.
//
// There is no catalogue file, no key, and no lookup that can miss. A sentence
// is built from two strings at once, so one written in a single language is a
// compile error rather than a blank on a page nobody visited yet. What that
// costs is that every sentence lives in this package instead of beside the
// markup that shows it; what it buys is that no sentence can be half-written.
package wording

import "net/http"

// Lang is the language the interface speaks. It is also what the document's own
// language attribute says, so the two cannot drift.
type Lang string

const (
	// ZhHant is the default. A reader who never touches the control gets the
	// interface exactly as it was before there was a choice.
	ZhHant Lang = "zh-Hant"
	En     Lang = "en"
)

// CookieName is where the choice is kept, alongside the other reading
// preferences and read the same way: one cookie, one known value, everything
// else falling to the default.
const CookieName = "yomihon_lang"

// FromCookieValue reads a stored choice. Only the one value that is not the
// default is honoured; anything else — a language yomihon does not speak, a
// truncated value, something a hand wrote — leaves the interface where it
// started rather than somewhere unpredictable. Reading the cookie itself
// belongs to whoever already reads this request's other preferences; this
// package holds no capability of its own and imports nothing.
func FromCookieValue(value string) Lang {
	if value == string(En) {
		return En
	}
	return ZhHant
}

// LanguageFromRequest reads which language this request asked the interface to
// speak, falling back to the default when the reader has chosen none or has
// sent a value this dictionary does not know.
//
// It lives here, beside the cookie's name and the values that cookie may
// carry, because three surfaces outside the page layer answer a reader in
// their language too — the static-file route, the origin guard, and the
// composition root's own refusal — and none of them can import the layout
// package: the layout imports the origin guard, so that edge only runs one
// way. Reading one cookie is the whole of what this adds; the sentences below
// stay sentences.
func LanguageFromRequest(r *http.Request) Lang {
	c, err := r.Cookie(CookieName)
	if err != nil {
		return ZhHant
	}
	return FromCookieValue(c.Value)
}

// What the language endpoint says when a hand-built request asks for
// something the rendered form never offers. The form itself carries only the
// two spoken values, so neither sentence appears in ordinary use.
var (
	LanguageFormUnreadable = both("表單無法解讀。", "The form could not be read.")
	LanguageUnknown        = both("這個介面只說 zh-Hant 與 en。", "This interface speaks only zh-Hant and en.")
)

// Tag is the language tag a document declares. It answers for the zero value
// too, which is what a chrome assembled without a language carries: the
// interface has a default, and a document that declared none at all would be
// worse than one declaring the default.
func (l Lang) Tag() string {
	if l == En {
		return string(En)
	}
	return string(ZhHant)
}

// Phrase is one thing yomihon says, in both languages. Its fields are its own:
// a phrase can only be made by naming both languages, which is the whole
// mechanism — there is nothing else to check, and nothing that checks it later.
type Phrase struct {
	zhHant string
	en     string
}

// both builds a phrase. Two parameters, neither optional: this is where the
// guarantee lives, and it is a guarantee the compiler makes rather than a rule
// a reviewer remembers.
func both(zhHant, en string) Phrase {
	return Phrase{zhHant: zhHant, en: en}
}

// In returns this phrase in lang. An unknown language reads as the default,
// for the same reason the cookie does.
func (p Phrase) In(lang Lang) string {
	if lang == En {
		return p.en
	}
	return p.zhHant
}
