package wording

// The health page: what the vault says about itself that its author would want
// to know. It reports and never repairs, and says so.

// HealthTitle is the page's own heading.
var HealthTitle = both("整體狀況", "Health")

// HealthAllClear lists what was checked, because a page reporting nothing is
// otherwise indistinguishable from a page that checked nothing. It also names
// what this page does not cover, and where that is checked instead.
var HealthAllClear = both(
	"這一頁檢查的項目目前都沒有問題：每個 [[…]] 連結都有目標，每篇筆記都有人連過來，沒有名字被兩個檔案同時使用，status 值都在 schema 的清單裡。frontmatter 少了必填欄位不歸這一頁管——這裡的 status 名單是從「有寫 status 的筆記」來的，沒寫的不會出現在任何一區；那些請跑 ",
	"Everything this page checks is clear: every [[…]] link has a target, every note is cited by something, no name is claimed by two files, and every status is in the schema's list. Frontmatter missing a required field is not this page's business — the statuses here come from notes that carry one, and a note with none appears in no section at all. For those, run ",
)

// HealthAllClearAfterCommand closes that sentence after the command it names.
var HealthAllClearAfterCommand = both("。", ".")

// HealthReportsOnly is the promise the whole page rests on.
var HealthReportsOnly = both(
	"這裡只陳述狀況，不會動你的檔案。要修改請在編輯器裡改那個檔案。",
	"This page reports and never touches your files. To change something, change the file in your editor.",
)

// The lede over unreadable files, which changes with whether a complete read
// has ever happened. The reader needs to know how old the rest of the page is.
var (
	BlockedLede = both(
		"這一次讀取時打不開這些檔案，頁面顯示的內容缺了它們，或停在較舊的版本。",
		"These files could not be opened on this read, so what the pages show is missing them or stopped at an older version.",
	)
	BlockedNeverComplete   = both("啟動到現在還沒有一次完整的讀取。", " No complete read has happened since start-up.")
	BlockedLastCompleteFmt = both("最後一次完整讀取是 %s。", " The last complete read was %s.")
)

// Each section names what it found and explains what the finding means, since
// the same list can call for different repairs.
var (
	BlockedTitle   = both("讀不進來的檔案", "Files that could not be read")
	UnwrittenTitle = both("連到不存在的目標", "Links to something that is not there")
	UnwrittenLede  = both(
		"這些連結的目標不存在，而且沒有任何缺口帳列出它們。目標是作者寫下的名字：可能是一篇還沒寫的筆記，也可能是一個不在書庫裡的檔案——兩者要修的東西不一樣。",
		"These links point at something that does not exist, and no gap ledger lists it. The target is the name the author wrote: it may be a note nobody has written, or a file that is not in the library at all — and those are different repairs.",
	)
	TitleOnlyTitle = both("連結寫的是筆記的標題", "Links written to a note's title")
	TitleOnlyLede  = both(
		"這些筆記都存在。連結指不到它們，是因為寫的是標題，而標題不是這個書庫解析連結用的名字——檔名和 aliases 才是。要修的是那篇筆記的 aliases，不是再寫一篇。",
		"These notes exist. The links miss them because they name a title, and a title is not what this library resolves a link by — a filename and its aliases are. The repair is that note's aliases, not another note.",
	)
	IslandsTitle = both("沒有人連過來的筆記", "Notes nothing cites")
	IslandsLede  = both(
		"沒有其他筆記連到它們。很多寫作方式本來就不互相引用——日記、逐字稿、模板、產生出來的報告——所以這裡按資料夾分開，先看形狀，再決定哪一堆值得你看。",
		"No other note links to these. Plenty of writing is not meant to be cited — journals, transcripts, templates, generated reports — so they are grouped by folder: see the shape first, then decide which group is worth your time.",
	)
	StatusOutsideEnumTitle = both("狀態值不在允許清單的筆記", "Notes whose status is outside the list")
	StatusOutsideEnumLede  = both(
		"這些筆記的 status 不在它的類型在 schema 宣告的清單裡。筆記頁會標出它們；要修正請直接編輯 frontmatter。",
		"These notes carry a status the schema does not declare for their type. The reading page marks them; the repair is a frontmatter edit.",
	)
	CollisionsTitle = both("兩個檔案共用的名字", "Names two files answer to")
	CollisionsLede  = both(
		"連到這些名字的連結不會生效，因為無法判斷指的是哪一個，而這裡不猜。",
		"A link to one of these names resolves to nothing, because which file it means cannot be decided, and nothing here guesses.",
	)
)

// The per-row details each section shows beside a note's name.
var (
	LinkedToFmt      = both("連到「%s」", "links to %q")
	TitleOnlyMeansTo = both("寫的是「%s」，指的是", "written as %q, meaning ")
	StatusAndTypeFmt = both("狀態值「%s」，類型「%s」", "status %q, type %q")
)
