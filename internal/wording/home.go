package wording

// The desk a reader arrives at, and the blocks the folder index fills.

// HomeReadmeTitle names the note the vault may keep to explain itself, which
// home shows in place of anything yomihon would have had to invent.
var HomeReadmeTitle = both("這個書庫的說明", "About this vault")

// HomeTitle is the desk's own heading. The arrival surface carries no kicker:
// the H1 is the name, and a line above it would only repeat it.
var HomeTitle = both("閱讀桌", "Reading desk")

// Each of these blocks states what it is and what it is for. The second line
// is not a subtitle: it says what the reader is looking at, for a block whose
// contents alone would not say.
var (
	HomeFaultsTitle   = both("契約能力", "Contract capabilities")
	HomeFaultsLede    = both("書庫宣告了無法履行的事", "The vault declares something it cannot honour")
	HomeFaultsBrowser = both("有些頁面因此看不到。", "Some pages cannot be shown because of it.")
	HomeDegradedTitle = both("讀不進來的檔案", "Files that could not be read")
	FolderRecentTitle = both("最近變更", "Recently changed")
	// The scoped ledes name the set the block lists — the contract's declared
	// knowledge layer — so the list does not read as the whole folder. A folder that declared no layer lists everything and
	// carries no lede: the heading already says what the list is, and naming
	// a layer there would invent a rule its owner never wrote.
	FolderRecentLedeScoped = both(
		"知識層資料夾中",
		"In the declared knowledge folders",
	)
	FolderTiedTitle = both("排不出先後的筆記", "Notes with identical timestamps")
	FolderTiedLede  = both(
		"所以下面的順序沒有意義。",
		"So the order below means nothing.",
	)
	FolderTiedLedeScoped = both(
		"知識層資料夾中",
		"In the declared knowledge folders",
	)
)

// The count of notes a folder holds. A study path's own count is UnitWords'.
// Chinese does not inflect a noun for number and English does, so each count
// is a pair. A tally that reads "1 notes" is the first thing a reader notices.
var (
	NoteCountOne  = both("%d 篇筆記", "%d note")
	NoteCountMany = both("%d 篇筆記", "%d notes")
)

// NoStructureRead marks a study-path card whose zero is a fault rather than
// an answer: the note lists things the grammar could not read as a course.
// A path that genuinely plans nothing carries no mark, so the two zeroes
// stop looking alike.
var NoStructureRead = both("未讀到課程結構", "No course structure read")

// SearchSubmit is the button on the desk's own search field.
var SearchSubmit = both("搜尋", "Search")

// DegradedNotice says how much of the page may be missing or stale, and where
// the detail is. It carries the count, so it is a pair like the other tallies.
var (
	DegradedNoticeOne = both(
		"有 %d 個檔案讀不進來，頁面內容可能不完整，或停在較舊的版本。詳細狀況見整體狀況頁。",
		"%d file could not be read, so pages may be incomplete or stopped at an older version. The health page has the detail.",
	)
	DegradedNoticeMany = both(
		"有 %d 個檔案讀不進來，頁面內容可能不完整，或停在較舊的版本。詳細狀況見整體狀況頁。",
		"%d files could not be read, so pages may be incomplete or stopped at an older version. The health page has the detail.",
	)
)

// What the home page says about an egress declaration the contract made and
// yomihon refused. The loss is not on this page — nothing here consults egress
// authority — so the words say the tool-facing results are unavailable and
// that reading, search and the local diagnostics stay up. The command names
// belong on the health page and the CLI, not on a desk a reader opened to read.
var (
	HomePrivacyTitle = both("給工具的結果目前無法使用", "Results are not currently available to tools")

	HomePrivacyLede = both(
		"契約宣告了不得離開這台機器的目錄，而 yomihon 無法採用那份宣告。閱讀、搜尋與這裡的診斷不受影響。",
		"The contract declares which directories must never leave this machine, and yomihon could not use that declaration. Reading, search and the local diagnostics are unaffected.")

	HomePrivacyBrowser = both(
		"這是契約自己的說法，原文照錄：",
		"This is what the contract itself said, reproduced as written:")
)
