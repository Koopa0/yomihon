package wording

// The reading page: the article's own furniture and the aids beside it.

// The concept sheet a lesson's wikilinks open.
var (
	ConceptNote  = both("概念筆記", "Concept note")
	CloseControl = both("關閉", "Close")
)

// The two ways out of a note to the file behind it.
var (
	RawFile        = both("原始檔", "Raw file")
	OpenInObsidian = both("在 Obsidian 開啟", "Open in Obsidian")
)

// Draft marks a note whose author declared draft: true: on its own page, and
// on the rows of the sidebar and the course that reach it. A finished note is
// the ordinary state and carries no mark.
var Draft = both("草稿", "Draft")

// NoteLanguage labels the head's language row: the BCP 47 tag a note declares
// for its own authored content, distinct from PrefLanguage, which names the
// interface's own language preference.
var NoteLanguage = both("語言", "Language")

// The label on the metarow's date. The two are two different claims: the
// first is the author's own declared update, the second the file's recorded
// change time, shown when the note declares no readable update date. They are
// kept apart because a fresh checkout stamps every file with one moment, and
// calling that moment the author's update would put words in their mouth.
var (
	UpdatedOn     = both("更新於", "Updated")
	FileChangedOn = both("檔案變更於", "File changed")
)

// NoteEmptyBody states what the article would otherwise leave blank after the
// renderer has consumed metadata and any duplicate opening title.
var NoteEmptyBody = both("這篇筆記還沒有內容。", "This note has no content yet.")

// NoteStale says the words below are the last ones that could be read, which a
// reader comparing them against what they just wrote would otherwise read as a
// lost edit.
var NoteStale = both(
	"這個檔案這一次讀不進來，下面是上一次讀到的內容，可能不是檔案現在的樣子。排除之後，過幾秒重新整理就會讀到新的。",
	"This file could not be read this time. What follows is the last version that could be, and may not be what the file says now. Once that clears, reloading in a few seconds picks up the newer one.",
)

// NoteParseStale distinguishes content a parser rejected from a failed file read.
var NoteParseStale = both(
	"檔案已開啟，但目前的筆記內容無法解析。下面保留上一次讀到的內容。請依解析器的訊息檢查筆記內容，修正並存檔後重新整理。",
	"The file opened, but its current note content could not be parsed. What follows is the last readable version. Check the note's content against the parser's message, save a correction, and reload.",
)

// The right rail and the blocks in it. CitedBy names body wikilinks only;
// BasedOn is the author's based_on declaration, a different claim, so the two
// labels stay visibly different rather than one number that mixes them.
var (
	ReadingAids = both("閱讀輔助", "Reading aids")
	OnThisPage  = both("本頁內容", "On this page")
	NoteHealth  = both("筆記狀況", "Note health")
	CitedBy     = both("正文連到這篇", "Cited in the text")
	CitedByNav  = both("正文連到這篇的筆記", "Notes that cite this one in the text")
	CitedByNone = both("目前沒有其他筆記在正文連到這篇。", "No other note cites this one in the text.")
	BasedOn     = both("聲明的來源", "Declared sources")
	BasedOnNav  = both("這篇聲明的來源", "Sources this note declared")
)

// FrontmatterNotYAML is the summary for a frontmatter block that is not valid
// YAML.
var FrontmatterNotYAML = both("frontmatter 不是有效的 YAML。", "The frontmatter is not valid YAML.")

// FrontmatterUnsupportedYAML names the authored forms that can make the YAML
// decoder fail at runtime, without quoting its Go implementation details.
var FrontmatterUnsupportedYAML = both(
	"frontmatter 使用了 yomihon 讀不進來的 YAML 寫法：合併鍵（<<），或把串列、對應表當作鍵。請直接編輯 frontmatter，改用一般的文字鍵。",
	"The frontmatter uses a YAML form yomihon cannot read: a merge key (<<), or a list or mapping used as a key. Edit the frontmatter directly to use ordinary text keys.",
)

// FrontmatterNeverCloses is said above the article of a note whose first line
// opens a frontmatter fence, whose next line reads as a field, and which has no
// closing fence after them. Every reader here takes such a note for body text
// with no frontmatter at all, so the page has to say what it cannot show: the
// keys below were not read as keys, and with them went the type, the status,
// the slug and the domain.
var FrontmatterNeverCloses = both(
	"這篇的第一行是 ---，下一行也像欄位，但後面一直沒有結尾的 ---，所以整段 frontmatter 都沒有被讀進來，type、status、slug、domain 都不算。yomihon 只陳述，不修復；請直接編輯檔案，補上結尾的 ---。",
	"This note opens a frontmatter block on line 1, and the closing --- never comes, so none of its fields were read: not type, status, slug or domain. yomihon reports and never repairs; edit the file and add the closing ---.",
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
// the other note's words and passes through untouched; these two are the
// interface's. The first answers an address with no note behind it, so the card
// says why it is empty instead of failing to appear — a card that never opens
// reads as a broken hover. The second closes an excerpt that stops short of the
// end. Neither names a way onward: the note's own name sits above them both and
// is the link out, so what the card could not show, the line naming it still
// reaches.
var (
	PreviewNoNote = both("這個網址沒有可以預覽的筆記。", "There is no note to preview at that address.")
	PreviewMore   = both("預覽到此為止，筆記後面還有。", "The preview stops here; the note goes on.")
)
