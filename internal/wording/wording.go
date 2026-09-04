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
// started rather than somewhere unpredictable.
func FromCookieValue(value string) Lang {
	if value == string(En) {
		return En
	}
	return ZhHant
}

// Known reads a value that must be one of the two, rejecting anything else. It
// is the other half of FromCookieValue: a stored preference is normalised
// because a reader who never chose should still get a page, while a request
// that names a language is answered or refused, because silently substituting
// one would hand back a receipt for a change nothing made. A value it does not
// know is refused with the default beside the false, so a caller that ignores
// the second answer still holds a language this interface speaks.
func Known(value string) (Lang, bool) {
	switch Lang(value) {
	case ZhHant:
		return ZhHant, true
	case En:
		return En, true
	}
	return ZhHant, false
}

// Other is the language this one is not. The interface speaks two, so the
// complement is a fact about the type rather than something each surface works
// out for itself — and a surface that worked it out would be a second place to
// change on the day there is a third.
func (l Lang) Other() Lang {
	if l == En {
		return ZhHant
	}
	return En
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
