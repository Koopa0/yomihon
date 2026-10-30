package wording

// The shared chrome: what every page carries around whatever it is showing.

// SkipToContent is the first thing keyboard focus reaches on every page, and it
// exists so a reader who navigates by key does not walk the whole rail before
// arriving at what they came for.
var SkipToContent = both("跳至主要內容", "Skip to content")

// ToggleNavigation names the header control that opens the rail as a drawer at
// widths where it is folded behind it; ToggleRail names the one that folds the
// column above them.
var ToggleNavigation = both("切換導覽", "Toggle navigation")

// BrandTag sits beside the name and says what this is: a place to read what is
// already written, not a place to file it.
var BrandTag = both("書庫", "Library")

// SearchNotes labels the way into search, both as the control's accessible name
// and as the dialog's. The library is what you search.
var SearchNotes = both("搜尋書庫", "Search the library")

// SearchNotesPrompt is the search invitation with the ellipsis a placeholder
// carries — header button label, dialog field, and desk field alike.
var SearchNotesPrompt = both("搜尋書庫…", "Search the library…")

// ToggleFurigana names the control for the reading aids over Japanese text.
// Both sides say the plain thing rather than the term for it: 振假名 and
// furigana name the aids to a reader who already knows what they are called,
// and to everyone else they name nothing. The control shows and hides readings
// a note already carries — it writes none, and it speaks none, which is what
// keeps it apart from the read-aloud controls.
var ToggleFurigana = both("顯示讀音", "Show readings")

// FuriganaLabel is what the control shows where the header row has room for it,
// and FuriganaLabelShort where it has room for two characters rather than four.
// The short form is the tail of the long one, so whichever of them a reader can
// see is contained in the name above, which is the name a screen reader speaks:
// a control that is read out as something other than what it visibly says is a
// control a reader cannot ask for by name.
//
// The English side carries the same word at both lengths: unlike 讀音, "Readings"
// has no shorter form worth inventing, and the narrow band already affords a
// whole word rather than a glyph.
var (
	FuriganaLabel      = both("顯示讀音", "Readings")
	FuriganaLabelShort = both("讀音", "Readings")
)

// The two words this interface uses for a switch's state, wherever one is shown
// beside the control rather than only announced.
var (
	On  = both("開", "On")
	Off = both("關", "Off")
)

// KeyboardHelp names the panel of shortcuts the ? key opens.
var KeyboardHelp = both("鍵盤快捷鍵", "Keyboard shortcuts")

// The shortcut panel's own rows: what each key does, in the fewest words that
// still distinguish it from the others.
var (
	ShortcutSearch              = both("搜尋", "Search")
	ShortcutCloseSearchOrFilter = both("關閉搜尋或篩選", "Close search or the filter")
	ShortcutJumpToFilter        = both("跳到導覽篩選（單鍵開啟時）", "Jump to the navigation filter (when single keys are on)")
	ShortcutToggleSidebar       = both("收合或展開側欄（單鍵開啟時）", "Collapse or expand the sidebar (when single keys are on)")
	ShortcutOpenHelp            = both("開啟這份說明（單鍵開啟時）", "Open this help (when single keys are on)")
)

// SearchDialogChoose names what the arrow keys do in the dialog's own footer,
// beside the two keys below: a choice among the rows, which Enter then opens.
var SearchDialogChoose = both("選擇", "Choose")

// SearchDialogEnter and SearchDialogEsc are the dialog's own footer: the two
// keys it answers to, shown rather than only announced.
var SearchDialogEnter = both("搜尋", "Search")
var SearchDialogEsc = both("關閉", "Close")

// The strip under a listing that runs past one page. It names itself, because
// a run of bare numbers at the foot of a long page says nothing about what
// following one of them does, and a reader arriving by ear meets the numbers
// with no word in front of them.
var (
	PagerTitle    = both("頁次", "Pages")
	PagerPrevious = both("上一頁", "Previous")
	PagerNext     = both("下一頁", "Next")
)

// PagerWhole names the undivided listing. It is what a reader follows before
// printing, since paper takes no click and one sheet of a divided listing is
// not the listing.
var PagerWhole = both("全部列在一頁", "All on one page")
