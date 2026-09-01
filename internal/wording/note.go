package wording

// The reading page: the article's own furniture, the aids beside it, and the
// two faces of the status control.

// The labels on the diagnostic kinds the renderer can report. Each names the
// fault in the fewest words that still distinguish it from its neighbours.
var (
	DiagLinkNoTarget      = both("連結沒有目標", "Link with no target")
	DiagLinkManyTargets   = both("連結有多個目標", "Link with several targets")
	DiagUnknownCallout    = both("沒見過的提示框類型", "Unrecognised callout type")
	DiagRiskyFence        = both("程式碼區塊裡的筆記語法", "Note syntax inside a code block")
	DiagEmbedFragmentGone = both("找不到嵌入指定的段落", "The embedded section was not found")
	DiagLinkBlockGone     = both("找不到連結指定的區塊", "The linked block was not found")
	DiagLinkSectionGone   = both("找不到連結指定的小節", "The linked section was not found")
	DiagCommentUnclosed   = both("沒有配對的註解記號", "Unpaired comment marker")
)

// The concept sheet a lesson's wikilinks open.
var (
	GrammarNote  = both("文法筆記", "Grammar note")
	CloseControl = both("關閉", "Close")
)

// The two ways out of a note to the file behind it.
var (
	RawFile        = both("原始檔", "Raw file")
	OpenInObsidian = both("在 Obsidian 開啟", "Open in Obsidian")
)

// NoteTooLargeToIndex says a note is readable and unsearchable, which is a
// promise the folder would otherwise appear to break.
var NoteTooLargeToIndex = both(
	"這個筆記太大,內容不會被搜尋讀取;閱讀不受影響。",
	"This note is too large to index, so search will not find words in it. Reading is unaffected.",
)

// NoteStale says the words below are the last ones that could be read, which a
// reader comparing them against what they just wrote would otherwise read as a
// lost edit.
var NoteStale = both(
	"這個檔案這一次讀不進來，下面是上一次讀到的內容，可能不是檔案現在的樣子。排除之後，過幾秒重新整理就會讀到新的。",
	"This file could not be read this time. What follows is the last version that could be, and may not be what the file says now. Once that clears, reloading in a few seconds picks up the newer one.",
)

// The rail's links to the lessons either side of this one inside a course. They
// carry their own separator because the name follows immediately.
var (
	RailPreviousLesson = both("上一課：", "Previous lesson: ")
	RailNextLesson     = both("下一課：", "Next lesson: ")
)

// The right rail and the blocks in it.
var (
	ReadingAids = both("閱讀輔助", "Reading aids")
	OnThisPage  = both("本頁內容", "On this page")
	NoteHealth  = both("筆記狀況", "Note health")
	CitedBy     = both("連到這篇", "Cited by")
	CitedByNav  = both("連到這篇的筆記", "Notes that cite this one")
	CitedByNone = both("目前沒有其他筆記連到這篇。", "No other note cites this one.")
)

// The status face: its label, and what it says in each state that offers no
// control.
var (
	StatusLabel          = both("狀態", "Status")
	WriteFaceUnavailable = both("生命週期寫入目前無法使用。", "Lifecycle writes are unavailable.")
	NoFrontmatter        = both("沒有 frontmatter（合法）。", "No frontmatter (which is legal).")
	NoLegalTransitions   = both("目前沒有合法的狀態轉換。", "No status transition is legal from here.")
	FrontmatterNotYAML   = both("frontmatter 不是有效的 YAML。", "The frontmatter is not valid YAML.")
	ReportsOnlyNote      = both("只陳述狀態，不自動修復。", "Reported, never repaired.")
)

// The way out of a state the interface offers nothing onward from. The second
// form names the editor link the page already carries.
var (
	EditFrontmatterToRecover = both(
		"要恢復請直接編輯 frontmatter。",
		"To recover, edit the frontmatter directly.",
	)
	EditFrontmatterToRecoverWithLink = both(
		"要恢復請直接編輯 frontmatter；「在 Obsidian 開啟」就在標題下方。",
		"To recover, edit the frontmatter directly; \"Open in Obsidian\" is under the title.",
	)
)

// TransitionEffect states what pressing a status control does, before it is
// pressed and beside the controls themselves.
var TransitionEffect = both(
	"這個按鈕只會改寫這篇筆記 frontmatter 的 status 欄位。",
	"This button rewrites one field of this note's frontmatter: status.",
)

// What the status face says about a value it cannot rule on. Both end the same
// way, because the repair is the same one and it is a hand edit.
var (
	StatusOutsideList = both(
		"不在 schema 允許清單中。yomihon 只陳述，不修復；請直接編輯 frontmatter。",
		"is not in the schema's declared list. yomihon reports and never repairs; edit the frontmatter directly.",
	)
	// The four causes are a complete division of what reaches this sentence,
	// not a list of the shapes anyone happened to meet: it is said when the
	// status reads as no non-empty text, and a YAML value fails that by being
	// absent, by being empty or null, by not being a single value, or by being
	// a single value that is not text. A fifth cause would have to fall
	// outside all four.
	StatusUnreadable = both(
		"frontmatter 裡讀不出 status 值（缺少、是空的、不是單一值，或不是文字）。yomihon 只陳述，不修復；",
		"No status value could be read from the frontmatter — it is missing, empty, not a single value, or not text. yomihon reports and never repairs; ",
	)
	StatusValuePrefix = both("狀態值 ", "The status ")
)

// The two-step confirmation a terminal target carries: what it costs, and the
// press that accepts it. Both name the target, so both are formats.
// Each is split around the value it names, because the value is marked up
// where it appears and a format string cannot carry an element.
var (
	TerminalTargetBefore = both("設為 ", "After ")
	TerminalTargetAfter  = both(" 之後，這裡不會再提供任何狀態轉換。", ", this offers no further transition.")
	ConfirmSetBefore     = both("確認設為 ", "Confirm ")
	ConfirmSetAfter      = both("", "")
)

// How a link that named a section was read, for a link whose base name resolved
// to nothing. Both carry the two halves, so both are formats.
// The first is split around the two marked-up halves it names; the second
// carries the note's name as a format, since nothing there is marked up.
var (
	LinkSplitBefore          = both("連結被讀成兩段：筆記目標 ", "The link was read in two parts: the note ")
	LinkSplitBetween         = both("、章節 ", ", and the section ")
	LinkSplitAfter           = both("。", ".")
	LinkMissingHalfIsTheNote = both(
		"缺的是筆記目標「%s」，不是章節；「#」是章節的分隔符號，所以檔名裡真的帶「#」的檔案，wikilink 指不到。",
		"What is missing is the note %q, not the section. \"#\" separates a section, so a file whose own name contains one cannot be reached by a wikilink at all.",
	)
)

// What the page says once, on arrival, about the change the reader just made.
// It is split around the two values, which are marked up where they appear.
var (
	FlipReceiptBefore  = both("狀態已從 ", "The status changed from ")
	FlipReceiptBetween = both(" 改為 ", " to ")
	FlipReceiptAfter   = both("。", ".")
)
