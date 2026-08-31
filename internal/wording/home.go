package wording

// The home page: the desk a reader arrives at, and the blocks on it.

// HomeReadmeTitle names the note the vault may keep to explain itself, which
// home shows in place of anything yomihon would have had to invent.
var HomeReadmeTitle = both("這個書庫的說明", "About this library")

// The page's own heading and the word above it.
var (
	HomeKicker = both("閱讀桌", "Reading desk")
	HomeTitle  = both("首頁", "Home")
)

// Each block on the desk states what it is and what it is for. The second line
// is not a subtitle: it says what the reader is looking at, for a block whose
// contents alone would not say.
var (
	HomeFaultsTitle   = both("契約能力", "Contract capabilities")
	HomeFaultsLede    = both("書庫宣告了無法履行的事", "The vault declares something it cannot honour")
	HomeFaultsBrowser = both("部分投影已關閉。", "Some projections are closed.")
	HomeDegradedTitle = both("讀取狀況", "Read health")
	HomeDegradedLede  = both("有檔案這次沒有讀進來", "Some files could not be read this time")
	HomeRecentTitle   = both("最近變更", "Recently changed")
	HomeRecentLede    = both("最近改動過的筆記", "Notes changed most recently")
	HomeTiedTitle     = both("筆記", "Notes")
	HomeTiedLede      = both(
		"這些檔案的時間戳一模一樣，排不出先後，所以這裡不是「最近」",
		"These files carry identical timestamps, so nothing here is more recent than anything else",
	)
	HomeLifecycleTitle = both("依狀態分組", "By status")
	HomeLifecycleLede  = both("書庫中每個狀態的筆記數", "How many notes hold each status the vault declares")
	HomePathsTitle     = both("學習路徑", "Study paths")
	HomePathsLede      = both("沿著書庫中的課程繼續學習", "Carry on through a course the vault lays out")
	HomeSearchTitle    = both("搜尋", "Search")
	HomeSearchLede     = both("尋找筆記或篩選書庫", "Find a note, or narrow the library down")
)

// What the desk says about a folder it is standing in for, and the empty case.
var (
	StandInEmpty     = both("這個資料夾目前沒有檔案。", "This folder holds no files.")
	StandInBefore    = both("這個資料夾有 %d 個檔案，最近一次變更是", "This folder holds %d files; the most recent change is ")
	StandInDateOpen  = both("（", " (")
	StandInDateClose = both("）。", ").")
)

// The counts beside a status chip and a study path.
var (
	NoteCount   = both("%d 篇筆記", "%d notes")
	LessonCount = both("%d 課", "%d lessons")
)

// SearchSubmit is the button on the desk's own search field.
var SearchSubmit = both("搜尋", "Search")

// NoStatusStated names the square for notes whose frontmatter carries no
// status. They still have somewhere to go, so they are counted and listed like
// any other; the words say what the file is missing, which a blank chip would
// leave the reader to guess.
var NoStatusStated = both("未標示狀態", "No status stated")
