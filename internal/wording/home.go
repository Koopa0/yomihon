package wording

// The desk a reader arrives at, and the blocks the folder index fills.

// HomeReadmeTitle names the note the vault may keep to explain itself, which
// home shows in place of anything yomihon would have had to invent.
var HomeReadmeTitle = both("這個書庫的說明", "About this library")

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
	// knowledge layer — because the status distribution beside it counts
	// every indexed note, and two true numbers over unstated sets read as a
	// contradiction. A folder that declared no layer lists everything and
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
	FolderLifecycleTitle = both("依狀態分組", "By status")
	// Beside a shelf narrowed to the declared layer the distribution still
	// counts every indexed note, so its sentence says that the count reaches
	// past the shelf; otherwise the heading stands alone, because a second
	// sentence that restates it would name no set the heading does not.
	FolderLifecycleLedeScoped = both(
		"書庫中每篇已索引筆記落在哪裡，含書架之外的資料夾",
		"Where each indexed note in the vault sits, including folders off the shelf",
	)
)

// The counts beside a status chip and a study path.
// Chinese does not inflect a noun for number and English does, so each count
// is a pair. A tally that reads "1 notes" is the first thing a reader notices.
var (
	NoteCountOne    = both("%d 篇筆記", "%d note")
	NoteCountMany   = both("%d 篇筆記", "%d notes")
	LessonCountOne  = both("%d 課", "%d lesson")
	LessonCountMany = both("%d 課", "%d lessons")
)

// NoStructureRead marks a study-path card whose zero is a fault rather than
// an answer: the note lists things the grammar could not read as a course.
// A path that genuinely plans nothing carries no mark, so the two zeroes
// stop looking alike.
var NoStructureRead = both("未讀到課程結構", "No course structure read")

// SearchSubmit is the button on the desk's own search field.
var SearchSubmit = both("搜尋", "Search")

// NoStatusStated names the square for notes whose frontmatter carries no
// status. They still have somewhere to go, so they are counted and listed like
// any other; the words say what the file is missing, which a blank chip would
// leave the reader to guess.
var NoStatusStated = both("未標示狀態", "No status stated")

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

// LifecycleUnreadable names the cell for notes whose frontmatter was there and
// could not be parsed. Nothing they declare could be read, so they belong to
// no status — and until the YAML is repaired, nothing else about them can be
// judged either.
var LifecycleUnreadable = both("讀不出來", "Could not be read")

// LifecycleUnstatedNote sits under the cells for notes carrying no status. One
// of those cells is not a fault: for some kinds of note, declaring no status
// is exactly right, and a number in a panel about lifecycle would otherwise
// read as something to go and fix.
var LifecycleUnstatedNote = both(
	"有些類型的筆記本來就不寫狀態。",
	"Some kinds of note do not carry a status.")

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
