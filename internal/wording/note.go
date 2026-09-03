package wording

// The reading page: the article's own furniture, the aids beside it, and the
// two faces of the status control.

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

// The label on the metarow's date. The two are two different claims: the
// first is the author's own declared update, the second the file's recorded
// change time, shown when the note declares no readable update date. They are
// kept apart because a fresh checkout stamps every file with one moment, and
// calling that moment the author's update would put words in their mouth.
var (
	UpdatedOn     = both("更新於", "Updated")
	FileChangedOn = both("檔案變更於", "File changed")
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

// The two-step confirmation a no-return target carries: what it costs, and
// the press that accepts it. The cost it names is the reader's own footing —
// no offered transition leads from there back to the status the note carries
// now — not whether the destination offers anything onward.
// Both name the target, so both are formats. Each is split around the value
// it names, because the value is marked up where it appears and a format
// string cannot carry an element.
var (
	NoReturnTargetBefore = both("設為 ", "After ")
	NoReturnTargetAfter  = both(" 之後，這裡不再有回到目前狀態的路。", ", this offers no way back to the current status.")
	ConfirmSetBefore     = both("確認設為 ", "Confirm ")
	ConfirmSetAfter      = both("", "")
)

// The name the two-step control answers to. It shows the same word the quiet
// one-press control shows — the status it leads to — so a reader who cannot
// see the disclosure mark has nothing to tell a reversible press from one with
// no way back, and the shape cue a collapsed disclosure gives is a cue about
// shape rather than about stakes. The word on screen stays inside the name, so
// what is heard still contains what is shown. Split around the value for the
// same reason as the pair above.
var (
	NoReturnSummaryBefore = both("設為 ", "Set ")
	NoReturnSummaryAfter  = both("，這一步不能回頭，需要再確認一次。", " — this step has no way back and asks to be confirmed.")
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

// The panel's label and sentence for a citation that named a note's title, and
// for a title that is exactly its filename cut where YAML starts a comment.
var (
	DiagLinkTitleOnly = both("連結寫到 title", "Link written to a title")
	DiagTitleOnlyNote = both(
		"這個名字是某篇筆記的 title。title 不是連結找得到的名字，那篇筆記加一個 alias 就能讓連結成立；下面列出是哪幾篇。",
		"This name is a note's title. A title is not a name a link finds; an alias on that note makes the link work. The notes are named below.")
	DiagTitleCut     = both("title 在 # 處截斷", "Title cut at a hash")
	DiagTitleCutNote = both(
		"這篇的 title 恰好是檔名在空白加 # 處截斷的結果。未加引號的值到那裡就被 YAML 當成註解；若 title 原本要包含 #，用引號寫就能保留。",
		"This note's title is exactly what its filename becomes when cut at a space followed by #. An unquoted value ends there, where YAML starts a comment; if the title was meant to carry the #, quoting it keeps it.")
)

// What the hover preview card says in its own voice. The excerpt inside it is
// the other note's words and passes through untouched; these three are the
// interface's. The first answers an address with no note behind it, so the card
// says why it is empty instead of failing to appear — a card that never opens
// reads as a broken hover. The second closes an excerpt that stops short of the
// end. The third names the way onward and is the text of the one control the
// card offers, so a reader met by either report is met by the same way out of
// it; what the card could not show, the note itself still can.
var (
	PreviewNoNote   = both("這個位置沒有可以預覽的筆記。", "There is no note to preview at that address.")
	PreviewMore     = both("預覽到此為止，筆記後面還有。", "The preview stops here; the note goes on.")
	PreviewOpenNote = both("開啟整篇筆記", "Open the whole note")
)
