package wording

import (
	"strings"
	"testing"
)

// Inside the settings fieldset the legend names the text-size group, and a
// choice that repeats it costs a reader the same words twice.
func TestTextSizeChoicesDoNotRepeatTheirLegend(t *testing.T) {
	t.Parallel()

	for _, lang := range []Lang{ZhHant, En} {
		t.Run(string(lang), func(t *testing.T) {
			t.Parallel()
			group := PrefTextSize.In(lang)
			for _, c := range []Phrase{PrefTextSizeMedium, PrefTextSizeLarge, PrefTextSizeExtraLarge} {
				if strings.Contains(c.In(lang), group) {
					t.Errorf("the settings choice reads %q, want it without %q: the fieldset's legend already says it",
						c.In(lang), group)
				}
			}
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
		"local list":     PrefStorageLocal,
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
	if got := PrefResetNote.In(En); !strings.Contains(got, "browser") {
		t.Errorf("the English reset note reads %q, want it to say the language follows the browser", got)
	}
	// Approved wording, carried through unchanged.
	if got, want := PrefStorageNote.In(ZhHant), "沒有任何一項離開這台機器"; !strings.Contains(got, want) {
		t.Errorf("the storage note reads %q, want it to still carry %q", got, want)
	}
	// The lists are what the counts were a second copy of, so they stay.
	for _, item := range []string{"介面語言", "外觀", "字級", "字體", "顯示讀音", "單鍵快捷鍵", "側欄", "小節旁的按鈕"} {
		if !strings.Contains(PrefStorageCookies.In(ZhHant), item) {
			t.Errorf("the cookie list no longer names %q; the naming is the part that was never wrong", item)
		}
	}
}

// The row for the buttons beside each section is worded by the owner, not by
// whoever builds the page: the three strings below were approved as written,
// and the cookie list names the row by the same words as its heading.
func TestSectionButtonsRowUsesTheApprovedWords(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name   string
		phrase Phrase
		zh, en string
	}{
		{"heading", PrefSectionButtons, "小節旁的按鈕", "Buttons beside sections"},
		{"always show", PrefSectionButtonsAlways, "一直顯示", "Always show"},
		{"show on hover", PrefSectionButtonsHover, "滑鼠停留時顯示", "Show on hover"},
	} {
		if got := tt.phrase.In(ZhHant); got != tt.zh {
			t.Errorf("the %s reads %q in Chinese, want the approved %q", tt.name, got, tt.zh)
		}
		if got := tt.phrase.In(En); got != tt.en {
			t.Errorf("the %s reads %q in English, want the approved %q", tt.name, got, tt.en)
		}
	}
	if got, want := PrefStorageCookies.In(ZhHant), "、"+PrefSectionButtons.In(ZhHant)+"、"; !strings.Contains(got, want) {
		t.Errorf("the Chinese cookie list reads %q, want it to carry %q", got, want)
	}
	if got, want := PrefStorageCookies.In(En), ", buttons beside sections, "; !strings.Contains(got, want) {
		t.Errorf("the English cookie list reads %q, want it to carry %q", got, want)
	}
}

// The row that returns a reader to where they stopped tells them where the place
// is kept, in the words the owner asked for: in this browser, for this address.
// A browser keeps one store for each address, so "this machine" would promise a
// place that 127.0.0.1 and localhost do not share, and a private window or
// cleared site data does not keep.
func TestThePositionRowSaysWhereThePlaceIsKept(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		lang     Lang
		wants    []string
		refuseds []string
	}{
		{ZhHant, []string{"這個瀏覽器", "這個位址"}, []string{"電腦", "機器"}},
		{En, []string{"this browser", "this address"}, []string{"machine", "computer"}},
	} {
		got := PrefPositionNote.In(tt.lang)
		for _, want := range tt.wants {
			if !strings.Contains(got, want) {
				t.Errorf("the position row reads %q in %s, want it to say %q", got, tt.lang, want)
			}
		}
		for _, refused := range tt.refuseds {
			if strings.Contains(got, refused) {
				t.Errorf("the position row reads %q in %s, want no %q: the place is in the browser's storage, not on the machine", got, tt.lang, refused)
			}
		}
	}
	// The owner approved these words on 2026-11-22: the heading, and a note that
	// says what the row does before where it is kept.
	if got, want := PrefPosition.In(ZhHant), "自動記閱讀位置"; got != want {
		t.Errorf("the position row's heading reads %q, want the approved %q", got, want)
	}
	if got, want := PrefPosition.In(En), "Reading position"; got != want {
		t.Errorf("the position row's English heading reads %q, want the approved %q", got, want)
	}
	if got, want := PrefPositionNote.In(ZhHant), "每篇重新打開時，回到上次讀到的地方；不影響書籤。只存在這個瀏覽器、這個位址上；關閉時一併清除。"; got != want {
		t.Errorf("the position row's note reads %q, want the approved %q", got, want)
	}
	if got := PrefPositionNote.In(En); !strings.Contains(got, "bookmarks are not affected") {
		t.Errorf("the English position note reads %q, want it to say bookmarks are not affected, as the Chinese does", got)
	}
	// The browser's own store is listed beside the cookies, naming both things in it.
	for _, item := range []string{"每篇的閱讀位置", "最近的搜尋"} {
		if !strings.Contains(PrefStorageLocal.In(ZhHant), item) {
			t.Errorf("the list of what stays in this browser reads %q, want it to name %q", PrefStorageLocal.In(ZhHant), item)
		}
	}
	// The cookie list names the row by the same words as its heading.
	if got, want := PrefStorageCookies.In(ZhHant), "、"+PrefPosition.In(ZhHant); !strings.HasSuffix(got, want) {
		t.Errorf("the Chinese cookie list reads %q, want it to end with %q", got, want)
	}
	if got, want := PrefStorageCookies.In(En), ", "+strings.ToLower(PrefPosition.In(En)[:1])+PrefPosition.In(En)[1:]; !strings.HasSuffix(got, want) {
		t.Errorf("the English cookie list reads %q, want it to end with %q", got, want)
	}
}
