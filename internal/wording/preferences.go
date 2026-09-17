package wording

// The page a reader sets the reading on: one heading per choice, one sentence
// under each saying what it does and — where a reader would reasonably guess
// wider — what it leaves alone. The sentences are the point of the page. A
// list of switches with no words is a list a reader has to try one by one.

// PreferencesTitle names the page. PreferencesLink is the same word on the
// control that leads to it, kept separate because a heading and a link are
// free to part company later without either being renamed by accident.
var (
	PreferencesTitle = both("設定", "Settings")
	PreferencesLink  = both("設定", "Settings")
)

// PreferencesLede is the whole promise of the page in one line: nothing chosen
// here is written into a note, and nothing here travels.
var PreferencesLede = both(
	"這些選擇只存在這台瀏覽器裡，不進筆記、也不出這台機器。",
	"These choices live in this browser alone — never in a note, never off this machine.",
)

// The interface language, and the boundary a reader would otherwise have to
// discover: the words yomihon writes change, the words an author wrote do not.
var (
	PrefLanguage     = both("介面語言", "Language")
	PrefLanguageNote = both(
		"只換介面的字；筆記保留作者寫的語言。",
		"Switches the interface only; a note keeps the language it was written in.",
	)
)

// The two languages name themselves in themselves. A reader looking for the
// way into a language they can read needs to recognise its name, not read a
// translation of it in the language they are leaving.
var (
	PrefLanguageZh = both("繁體中文", "繁體中文")
	PrefLanguageEn = both("English", "English")
)

// The light and dark grounds, and what choosing neither means.
var (
	PrefAppearance     = both("外觀", "Appearance")
	PrefAppearanceNote = both(
		"跟隨系統時，深淺由作業系統決定。",
		"Following the system leaves light and dark to the operating system.",
	)
	PrefAppearanceSystem = both("跟隨系統", "Follow the system")
	PrefAppearanceLight  = both("亮色", "Light")
	PrefAppearanceDark   = both("暗色", "Dark")
)

// The reading column's measure. The note draws the line the control does not:
// the article grows, the header and rails around it hold still.
var (
	PrefTextSize           = both("字級", "Text size")
	PrefTextSizeMedium     = both("中", "Medium")
	PrefTextSizeLarge      = both("大", "Large")
	PrefTextSizeExtraLarge = both("特大", "Extra large")
	PrefTextSizeNote       = both(
		"只放大內文，介面不變。",
		"Enlarges the reading column only; the header and rails around it keep their size.",
	)
)

// The face the reading is set in, and the three it may be set in. Each side
// names them as its own readers know them: the Chinese page by the characters,
// the English page by the names those faces carry in English.
var (
	PrefTypeface     = both("字體", "Typeface")
	PrefTypefaceNote = both(
		"只換內文與課文練習的字體；標題、介面、程式碼不變。",
		"Changes the reading column and the lesson exercises only; headings, the interface, and code keep theirs.",
	)
	PrefTypefaceSerif = both("明體", "Serif")
	PrefTypefaceSans  = both("黑體", "Sans serif")
	PrefTypefaceKai   = both("楷體", "Kaiti")
)

// The reading aids over Japanese text. The note says the switch is inert
// everywhere else, so a reader who turns it on and sees nothing knows why.
var (
	PrefFurigana     = both("顯示讀音", "Readings")
	PrefFuriganaNote = both(
		"只影響日文上方標了讀音的頁面。",
		"Applies to the pages that carry readings above the Japanese text.",
	)
)

// Whether a bare key does anything, and which keys the answer covers. Naming
// the two that keep working is the whole value of the sentence: "off" reads as
// "no keyboard" without it.
var (
	PrefShortcuts     = both("單鍵快捷鍵", "Single-key shortcuts")
	PrefShortcutsNote = both(
		"關掉的是 / 與 [；⌘K 與 Esc 不受影響。",
		"Turns off / and [; ⌘K and Esc keep working.",
	)
)

// The two states every switch on this page is in. They are the words a reader
// picks, so they are verbs' worth of plain rather than the glyphs the header
// controls carry.
var (
	PrefOn  = both("開啟", "On")
	PrefOff = both("關閉", "Off")
)

// The one control that is not a choice between values. Its sentence has to
// name the whole of what it clears, including the consequence a reader would
// not predict: the interface comes back speaking the language it started in.
var (
	PrefReset     = both("回到預設", "Back to the defaults")
	PrefResetNote = both(
		"清掉這一頁的每一項設定（含介面語言，所以套用後介面回到繁體中文）；本次分頁的側欄狀態與篩選字不受影響。",
		"Clears every setting on this page — including the interface language, so the page comes back in Traditional Chinese; this tab's sidebar state and filter text are untouched.",
	)
)

// PrefApply is on the control that saves the page where nothing is running to
// save it as the reader picks. It is drawn on every page and hidden where the
// enhancement is live, so a browser without scripting sees the control it
// needs and a browser with it never sees a control it does not.
var PrefApply = both("套用", "Apply")

// PrefReturn leads back to whatever the reader was holding when they came
// here. It stands in for the apply control where choices are already stored as
// they are picked: without it the page would be the one place that saves
// everything and offers no way out of itself.
var PrefReturn = both("回到剛才在讀的", "Back to what you were reading")

// PrefSaveRefused is what a choice says for itself when the browser would not
// keep it. The choice is put back to what it was, because a radio left sitting
// on a value nothing stored is the page telling the reader something untrue —
// and the likeliest cause is named, since it is one a reader can act on.
var PrefSaveRefused = both(
	"這個選擇沒有存下來，已經放回原本的值。這個瀏覽器可能不讓這個位址存 cookie。",
	"That choice was not stored, and has been put back. This browser may be refusing cookies for this address.",
)

// What this browser is holding, said plainly and completely. A page that
// stores things on a reader's machine should be the page that lists them.
var (
	PrefStorageTitle = both("這個瀏覽器記著什麼", "What this browser remembers")
	PrefStorageNote  = both(
		"設定存成這個位址上的 cookie，側欄的狀態只活到分頁關閉；沒有任何一項離開這台機器。",
		"Settings are cookies on this address; the sidebar's own values die with the tab; none of it leaves this machine.",
	)
	PrefStorageCookies = both(
		"cookie 記著：介面語言、外觀、字級、字體、顯示讀音、單鍵快捷鍵",
		"Cookies: language, appearance, text size, typeface, readings, single-key shortcuts",
	)
	PrefStorageSession = both(
		"分頁內的值：側欄展開狀態、側欄篩選字",
		"Per-tab values: sidebar expansion, sidebar filter text",
	)
)

// What the page says to a request the rendered forms could not have sent. The
// forms offer one field carrying one of the values named above, so neither
// sentence appears in ordinary use.
var (
	PrefUnknownValue = both(
		"這不是這個設定認得的值，沒有任何東西被改動。",
		"That is not a value this setting knows; nothing was changed.",
	)
	PrefFormUnreadable = both("表單讀不出來。", "The form could not be read.")
)
