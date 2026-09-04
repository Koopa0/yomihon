package wording

// The shared chrome: what every page carries around whatever it is showing.

// SkipToContent is the first thing keyboard focus reaches on every page, and it
// exists so a reader who navigates by key does not walk the whole rail before
// arriving at what they came for.
var SkipToContent = both("跳至主要內容", "Skip to content")

// ToggleNavigation names the control that opens the rail at widths where it is
// collapsed behind it.
var ToggleNavigation = both("切換導覽", "Toggle navigation")

// BrandTag sits beside the name and says what this is: a place to read what is
// already written, not a place to file it.
var BrandTag = both("書庫", "Library")

// SearchNotes labels the way into search, both as the control's accessible name
// and as the dialog's.
var SearchNotes = both("搜尋筆記", "Search notes")

// SearchNotesPrompt is the same invitation with the ellipsis a placeholder
// carries, so the control reads as a field waiting rather than a label.
var SearchNotesPrompt = both("搜尋筆記…", "Search notes…")

// SearchLibraryPrompt is the dialog's own field: it searches the whole library,
// which is a wider promise than the button that opened it.
var SearchLibraryPrompt = both("搜尋書庫…", "Search the library…")

// ToggleFurigana names the control for the reading aids over Japanese text.
var ToggleFurigana = both("切換振假名", "Toggle furigana")

// FuriganaMark is the control's glyph. It is the same in both languages: the
// character is the conventional mark for the thing itself, and a reader of an
// interface that renders Japanese will meet it in the text as well.
var FuriganaMark = both("振", "振")

// The two words this interface uses for a switch's state, wherever one is shown
// beside the control rather than only announced.
var (
	On  = both("開", "On")
	Off = both("關", "Off")
)

// The chrome's link to the health page carries no name of its own: it is
// wording.HealthTitle, the heading that page prints, so the word a reader
// follows and the word they arrive at cannot drift apart. Only the hover title
// is written here, because it says what the link does rather than what the page
// is called.

// HealthLinkTitle says what that page counts, so the label does not have to.
var HealthLinkTitle = both("整體狀況：連結、孤島、名字衝突", "Health: links, islands, name collisions")

// TextSizeMark is the text-size control's glyph. The English side is a letter
// rather than the character, because the control's whole job is to be legible
// at a glance to whoever is reading the interface.
var TextSizeMark = both("字", "A")

// TextSizeCycle says which way the control moves, since it steps rather than
// toggles and a reader pressing it once cannot tell that from the result.
var TextSizeCycle = both("字級：中 → 大 → 特大", "Text size: medium → large → extra large")

// The text-size control's accessible name at each step. The name carries the
// state because a control that cycles three values has nowhere else to put it.
var (
	TextSizeMedium     = both("字級：中", "Text size: medium")
	TextSizeLarge      = both("字級：大", "Text size: large")
	TextSizeExtraLarge = both("字級：特大", "Text size: extra large")
)

// KeyboardHelp names the panel of shortcuts and the control that opens it.
var KeyboardHelp = both("鍵盤快捷鍵", "Keyboard shortcuts")

// KeyboardHelpControl is that control's accessible name, which says it opens an
// explanation rather than performing a shortcut.
var KeyboardHelpControl = both("鍵盤快捷鍵說明", "Keyboard shortcut help")

// The shortcut panel's own rows: what each key does, in the fewest words that
// still distinguish it from the others.
var (
	ShortcutSearch              = both("搜尋", "Search")
	ShortcutCloseSearchOrFilter = both("關閉搜尋或篩選", "Close search or the filter")
	ShortcutJumpToFilter        = both("跳到導覽篩選（單鍵開啟時）", "Jump to the navigation filter (when single keys are on)")
	ShortcutToggleSidebar       = both("收合或展開側欄（單鍵開啟時）", "Collapse or expand the sidebar (when single keys are on)")
)

// ShortcutSidebarNarrowOnly is the rest of what the sidebar key does, which is
// nothing at most reading widths. There is only something to fold away where
// the window is narrow enough that the sidebar has become a drawer; on a wide
// one the sidebar is simply present, the key is inert, and the row above
// promised otherwise — leaving a reader pressing a documented key at a desk and
// reading the silence as a broken preference. The sentence describes the
// condition the way the reader can see it rather than by a pixel count, whose
// one home is the stylesheet.
var ShortcutSidebarNarrowOnly = both(
	"寬到側欄一直看得見的視窗上，這個鍵沒有作用。",
	"On a window wide enough to keep the sidebar in view, this key does nothing.",
)

// SingleKeyShortcuts names the preference that decides whether a bare key does
// anything, and the two states it reports beside the checkbox.
var SingleKeyShortcuts = both("單鍵快捷鍵", "Single-key shortcuts")
var CurrentlyOn = both("目前開啟", "Currently on")
var CurrentlyOff = both("目前關閉", "Currently off")

// SingleKeyShortcutsNote says what turning them off costs and what it does not,
// because the two keys that keep working are the two a reader would most miss.
var SingleKeyShortcutsNote = both(
	"關掉之後，/ 和 [ 都只會照瀏覽器原本的方式輸入；⌘K 與 Esc 不受影響。",
	"With these off, / and [ type the way the browser types them; ⌘K and Esc are unaffected.",
)

// SingleKeyShortcutsTakeover is the other half of that sentence: what leaving
// them on costs. The panel named the switch and what each key reaches, and
// said nothing about the page holding the key shut against whatever else the
// reader's browser does with it — so a lone slash that used to open a quick
// in-page find stopped doing so, silently, and the one control that would
// give it back read as being about this page's own keys. Which browsers bind
// a bare slash is the browser's business and not a fact yomihon can assert,
// so the sentence offers the common case as an example and claims only what
// is true everywhere: while the switch is on, these two keys stop here.
var SingleKeyShortcutsTakeover = both(
	"開啟時這兩個鍵由本頁接手，瀏覽器收不到；有些瀏覽器用單獨的 / 開啟頁內快速尋找，那時它不會出現。",
	"While they are on, this page takes both keys and the browser never sees them; some browsers open a quick in-page find on a lone /, and it will not appear.",
)

// SearchDialogEnter and SearchDialogEsc are the dialog's own footer: the two
// keys it answers to, shown rather than only announced.
var SearchDialogEnter = both("搜尋", "Search")
var SearchDialogEsc = both("關閉", "Close")

// ThemeToggle names the control that switches between the light and dark
// grounds. It is a label rather than a sentence: what it does is visible the
// moment it is pressed, so it says what it is and stops.
var ThemeToggle = both("切換主題", "Switch theme")

// LanguageToggle names the control this package exists for. The label is not
// translated into the language being left — a reader looking for the way back
// to their own language wants to recognise it, not to read it.
var LanguageToggle = both("切換語言：中文 / English", "Switch language: 中文 / English")
