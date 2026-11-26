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
	HomeFaultsTitle   = both("規則檔能力", "Rules capabilities")
	HomeFaultsLede    = both("書庫宣告了無法履行的事", "The vault declares something it cannot honour")
	HomeFaultsBrowser = both("有些頁面因此看不到。", "Some pages cannot be shown because of it.")
	HomeDegradedTitle = both("讀不進來的檔案", "Files that could not be read")
)

// NoStructureRead marks a study-path card whose zero is a fault rather than
// an answer: the note lists things the grammar could not read as a course.
// A path that genuinely plans nothing carries no mark, so the two zeroes
// stop looking alike.
var NoStructureRead = both("未讀到課程結構", "No course structure read")

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
		"規則檔宣告了不得離開這台機器的目錄，而 yomihon 無法採用那份宣告。閱讀、搜尋與這裡的發現不受影響。",
		"The rules file declares which directories must never leave this machine, and yomihon could not use that declaration. Reading, search and the local findings are unaffected.")

	HomePrivacyBrowser = both(
		"這是規則檔自己的說法，原文照錄：",
		"This is what the rules file itself said, reproduced as written:")
)

// The plate the desk returns a reader to their bookmark from: the label it is
// headed by, the section the bookmark was added under, and how long ago that
// was. The age is counted in minutes within the hour and in hours within the
// same day; the day before is yesterday, and anything older is the day itself.
var (
	BookmarkLabel = both("書籤", "Bookmark")
	StoppedAtFmt  = both("停在 %s", "Stopped at %s")
	MinutesAgoFmt = both("%d 分鐘前", "%d min ago")
	HoursAgoFmt   = both("%d 小時前", "%d h ago")
	Yesterday     = both("昨天", "Yesterday")
)

// RecentlyChanged heads the desk's band of the notes that changed last on disk.
// A note's change is 變更, the word the order "by latest change" uses too.
var RecentlyChanged = both("最近變更", "Recently changed")
