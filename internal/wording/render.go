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

// TitleOnlyTargetFmt is said where a citation names a note's declared title.
// The note exists; the name it was written under is not one a link finds,
// which is what the vault's own reader does with it too. Saying there is no
// such note would be false about a file the reader can open.
var TitleOnlyTargetFmt = both(
	"「%s」是〈%s〉的 title，而 title 不是連結找得到的名字；在那篇加一個 alias 就能讓這個連結成立",
	"%q is the title of %q, and a title is not a name a link finds; an alias on that note makes this link work")

// TitleOnlySeveralFmt is the same for a title more than one note declares.
// Every holder is named: picking one would be a guess, and the reader is the
// one who knows which they meant.
var TitleOnlySeveralFmt = both(
	"「%s」是 %d 篇筆記共同的 title，而 title 不是連結找得到的名字；旁邊的筆記狀況列出了是哪幾篇",
	"%q is the title of %d notes, and a title is not a name a link finds; the note's conditions beside the article name them")

// TitleTruncatedAtHashFmt states a coincidence and the remedy, and is true
// whichever way the note got here. An unquoted value loses everything from a
// whitespace-and-hash onward, and a title written short in quotes lands in the
// same place; the parsed frontmatter cannot tell them apart, so this reports
// what is observable and leaves the author to recognise their own case.
var TitleTruncatedAtHashFmt = both(
	"這篇的 title「%s」恰好是檔名在空白加 # 處截斷的結果；若 title 原本要包含 #，用引號寫就能保留。",
	"This note's title %q is exactly what its filename becomes when cut at a space followed by #; if the title was meant to carry the #, quoting it keeps it.")

// AmbiguousTargetFmt is said where a name places more than one file. It is not
// the sentence a title-only citation gets: that name reaches a note under a
// name links do not follow, and an alias fixes it, while this name is followed
// perfectly well and arrives at several files at once. The repair differs with
// the cause — one note gains an alias, or one of these files is renamed — so
// the two are never told in the same words.
//
// The files are named because there is no guessing between them, and a reader
// deciding which they meant needs to see what the choices are.
var AmbiguousTargetFmt = both(
	"「%s」指向不只一個檔案：%s。yomihon 不替你猜是哪一個",
	"%q points at more than one file: %s. yomihon does not guess which")
