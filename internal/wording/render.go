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

// EmbedSourceFrom opens an excerpt's provenance line; the source note's name
// follows it as a link. An excerpt is another note's words, and the one thing
// it cannot say for itself is whose.
var EmbedSourceFrom = both("出自 ", "From ")

// ReadAloud names the control beside a paragraph of Japanese.
var ReadAloud = both("朗讀這段日文", "Read this Japanese aloud")

// The read-aloud bar, which only exists once speech is available, so the page
// carries its words rather than the browser building them. A sentence written
// into a script is written in one language for everyone, and a reader who has
// asked for the other one meets it in the middle of a page that is otherwise
// theirs.
//
// ReadAloudRateFmt takes the rate as the reader sees it on the button beside
// it, so the announcement and the control agree.
var (
	ReadAloudStop        = both("停止", "Stop")
	ReadAloudStopThis    = both("停止朗讀", "Stop reading aloud")
	ReadAloudControls    = both("日文朗讀控制", "Japanese read-aloud controls")
	ReadAloudSpeed       = both("朗讀速度", "Reading speed")
	ReadAloudRateFmt     = both("速度 {rate}×", "Speed {rate}×")
	ReadAloudStopped     = both("已停止", "Stopped")
	ReadAloudPlaying     = both("播放中", "Playing")
	ReadAloudFinished    = both("播放完成", "Finished")
	ReadAloudUnavailable = both("目前無法播放日語語音", "Japanese speech is unavailable right now")
)

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

// EmbedRepeatedHeadingFmt is said above an excerpt whose fragment named a
// section the source note carries more than once. It states the count and the
// name, because the reader's next move is to open that note and see which of
// them they were looking at — and the author's next move is to rename one.
//
// It does not offer to show the others. An embed addresses one section, and
// which one an address means is the author's to settle in their own file.
var EmbedRepeatedHeadingFmt = both(
	"這篇筆記裡有 %d 個叫「%s」的小節,以下顯示第一個。",
	"This note has %d sections called %q; the first one follows.")

// UnwrittenFileFmt is a name with a file's extension and nothing behind it.
// The author was reaching for a picture or a document to show, and saying no
// note answers to the name sends them looking for the wrong thing to write.
var UnwrittenFileFmt = both("還沒有「%s」這個檔案", "There is no file called %q yet")

// EmbedNotExpanded is said once above an excerpt that contains a citation its
// author wrote as an embed. The words are shown as a link instead, which still
// leads where they pointed; what the reader would otherwise have no way to
// know is that an excerpt was asked for and a link is what arrived.
var EmbedNotExpanded = both(
	"這段摘錄裡有一個嵌入，因為嵌入只展開一層，改以連結呈現。",
	"This excerpt contains an embed of its own; embeds expand one level, so it is shown as a link.")
