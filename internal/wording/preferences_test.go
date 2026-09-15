package wording

import (
	"strings"
	"testing"
)

// The header's text-size control cycles three values, so its accessible name is
// the only place its group can be named. Inside the settings fieldset the legend
// names the group, and a choice that repeats it costs a reader the same words
// twice. The two sets were one set once, and shortening it in place would have
// taken the header's only context away.
func TestTextSizeChoicesAndTheHeaderControlAreNamedApart(t *testing.T) {
	t.Parallel()

	for _, lang := range []Lang{ZhHant, En} {
		t.Run(string(lang), func(t *testing.T) {
			t.Parallel()
			group := PrefTextSize.In(lang)
			header := []Phrase{TextSizeMedium, TextSizeLarge, TextSizeExtraLarge}
			choices := []Phrase{PrefTextSizeMedium, PrefTextSizeLarge, PrefTextSizeExtraLarge}

			for _, h := range header {
				if !strings.Contains(h.In(lang), group) {
					t.Errorf("the header control reads %q, want it to carry %q: a control that cycles has nowhere else to name its group",
						h.In(lang), group)
				}
			}
			for _, c := range choices {
				if strings.Contains(c.In(lang), group) {
					t.Errorf("the settings choice reads %q, want it without %q: the fieldset's legend already says it",
						c.In(lang), group)
				}
			}
			// The two sets cannot collide while both checks above hold: one
			// requires the group's name, the other forbids it. An equality
			// check here would never be the one to report a fault, so it is
			// not written.
		})
	}
}

// English settings speak English. The language picker still names each language
// in itself, so a reader can find their own.
func TestEnglishSettingsSpeakEnglish(t *testing.T) {
	t.Parallel()

	for name, got := range map[string]string{
		"legend":           PrefLanguage.In(En),
		"serif":            PrefTypefaceSerif.In(En),
		"sans":             PrefTypefaceSans.In(En),
		"kai":              PrefTypefaceKai.In(En),
		"text size medium": PrefTextSizeMedium.In(En),
	} {
		for _, phrase := range []string{"明體", "黑體", "楷體", "繁體中文"} {
			if strings.Contains(got, phrase) {
				t.Errorf("the English %s reads %q, want no %q in it", name, got, phrase)
			}
		}
	}
	if got, want := PrefTypefaceKai.In(En), "Kaiti"; got != want {
		t.Errorf("PrefTypefaceKai.In(En) = %q, want %q", got, want)
	}
	if got, want := PrefLanguage.In(En), "Language"; got != want {
		t.Errorf("PrefLanguage.In(En) = %q, want %q", got, want)
	}
	// The picker keeps each language's own name, in both interfaces.
	for _, lang := range []Lang{ZhHant, En} {
		if got, want := PrefLanguageZh.In(lang), "繁體中文"; got != want {
			t.Errorf("the language picker reads %q in %s, want %q so a reader can find their own", got, lang, want)
		}
	}
}

// Settings sentences say what they mean. "chrome" is what the people who built
// the page call it; a reader adjusting reading comfort is owed the thing itself.
// The counts went because each sat beside the list it counted, and the list is
// the copy that does not go stale.
func TestSettingsSentencesCarryNoJargonAndNoCounting(t *testing.T) {
	t.Parallel()

	sentences := map[string]Phrase{
		"text size note": PrefTextSizeNote,
		"typeface note":  PrefTypefaceNote,
		"reset note":     PrefResetNote,
		"storage note":   PrefStorageNote,
		"cookie list":    PrefStorageCookies,
		"session list":   PrefStorageSession,
	}
	banned := map[Lang][]string{
		En:     {"chrome", "six", "Six", "two values", "Two per-tab", "below"},
		ZhHant: {"六個", "兩個", "下面六"},
	}
	for name, phrase := range sentences {
		for _, lang := range []Lang{ZhHant, En} {
			got := phrase.In(lang)
			for _, word := range banned[lang] {
				if strings.Contains(got, word) {
					t.Errorf("the %s reads %q in %s, want no %q in it", name, got, lang, word)
				}
			}
		}
	}
	// The reset sentence names its destination in the reader's own language.
	if got := PrefResetNote.In(En); !strings.Contains(got, "Traditional Chinese") {
		t.Errorf("the English reset note reads %q, want it to name Traditional Chinese in English", got)
	}
	// Approved wording, carried through unchanged.
	if got, want := PrefStorageNote.In(ZhHant), "沒有任何一項離開這台機器"; !strings.Contains(got, want) {
		t.Errorf("the storage note reads %q, want it to still carry %q", got, want)
	}
	// The lists are what the counts were a second copy of, so they stay.
	for _, item := range []string{"介面語言", "外觀", "字級", "字體", "振假名", "單鍵快捷鍵"} {
		if !strings.Contains(PrefStorageCookies.In(ZhHant), item) {
			t.Errorf("the cookie list no longer names %q; the naming is the part that was never wrong", item)
		}
	}
}
