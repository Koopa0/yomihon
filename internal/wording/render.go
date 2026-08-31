package wording

// What the renderer says inside an article about a link it could not honour.
// These sit in the body rather than in the chrome, so they carry the reason at
// the place the reader's cursor already is.

// UnwrittenTargetFmt is a name with nothing behind it. It says the name is
// unwritten rather than that the link is broken: the note may simply not be
// written yet, which is an ordinary state in a vault.
var UnwrittenTargetFmt = both("還沒有「%s」這篇筆記", "There is no note called %q yet")

// UnwrittenHeadingFmt is added when the author also wrote a fragment. A file
// whose own name carries the mark can never be linked whole, and without this
// the reader is told a shorter name than the one they typed failed.
var UnwrittenHeadingFmt = both("；「#」之後的「%s」被讀成章節名稱", "; what follows \"#\" was read as the section %q")

// What a link that placed its note but not its fragment says.
var (
	BlockNotFound      = both("找不到這個區塊，連結已改為指向整篇筆記", "That block was not found; the link now points at the whole note")
	SectionNotFoundFmt = both("找不到「%s」這個小節，連結會落在筆記最上方", "No section called %q was found; the link lands at the top of the note")
)

// EmbedUnreadable is an embed whose note exists and whose bytes could not be
// read this time — a different fault from a name with nothing behind it.
var EmbedUnreadable = both("這篇筆記存在，但這次讀取時拿不到它的內容", "This note exists, but its contents could not be read this time")

// What an embed says when it could not find the fragment it was given and
// showed the whole note instead. It is split around the fragment, which is
// marked up where it appears.
var (
	EmbedWidenedBefore = both("找不到 ", "Could not find ")
	EmbedWidenedAfter  = both("，以下顯示整篇筆記。", "; the whole note follows.")
)

// ReadAloud names the control beside a paragraph of Japanese.
var ReadAloud = both("朗讀這段日文", "Read this Japanese aloud")
