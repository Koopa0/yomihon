package wording

// The desk and the four mode index pages a reader enters it through.

// The line under the desk's title, and the sentence marking off the part of
// the page that is about the vault's health rather than about reading.
var (
	HomeDeskLede = both(
		"一個書庫。從你要讀的那一種東西進去。",
		"One library. Come in through the kind of thing you want to read.",
	)
	HomeSeamNote = both(
		"這條線以下是這個書庫的說明，以及有事時才出現的診斷與契約狀況。",
		"Below this line: what this library says about itself, and any diagnostics or contract faults there are to report.",
	)
)

// ShelfAll ends a narrowed shelf, where the rest of it is one click away. It
// states no number: the shelf's own count is already in its head, and a second
// figure describing the same listing is a figure that can disagree with it.
var ShelfAll = both("全部 →", "All of it →")

// Each mode block on the desk says what it holds, in one line, before its first
// few items. The heading is the mode's own name, which the rail already had.
var (
	DeskPathsLede = both(
		"照順序讀的課程與書。",
		"Courses and books, read in the order they lay out.",
	)
	DeskMapsLede = both(
		"一個主題的枝與葉，從上往下看。",
		"One subject's branches and leaves, seen from above.",
	)
	DeskReportsLede = both(
		"每日簡報與稽核，照它們被寫下的樣子。",
		"Daily briefings and audits, as they were written.",
	)
	DeskFoldersLede = both(
		"照檔案的位置瀏覽；狀態與最近修改在這裡。",
		"Browse by where the files are; status and recent changes are here.",
	)
)

// The mode index pages state the same thing at more length, and say what an
// empty mode means: a vault that declared none of something is not a vault
// missing a feature.
var (
	PathIndexLede = both(
		"書庫宣告的每一條路徑，以及各自的課數。",
		"Every path the vault declares, and how many lessons each lays out.",
	)
	PathIndexEmpty = both(
		"這個書庫沒有宣告任何路徑。",
		"This vault declares no paths.",
	)
	MapIndexLede = both(
		"書庫宣告的每一張地圖，以及各自的枝數。",
		"Every map the vault declares, and how many branches each holds.",
	)
	MapIndexEmpty = both(
		"這個書庫沒有宣告任何地圖。",
		"This vault declares no maps.",
	)
	ReportIndexLede = both(
		"寫在書庫裡的報告與每日簡報，照原樣顯示。",
		"The reports and daily briefings written into the vault, shown as they were written.",
	)
	ReportIndexEmpty = both(
		"這個書庫裡沒有報告。",
		"There are no reports in this vault.",
	)
	FolderIndexLede = both(
		"照檔案實際存放的位置瀏覽整個書庫。",
		"Browse the whole vault by where its files actually sit.",
	)
	FolderIndexEmpty = both(
		"這個書庫裡沒有檔案。",
		"There are no files in this vault.",
	)
)

// What each mode counts. Chinese uses a different measure word for each kind of
// thing and English inflects the noun, so every count is a pair.
var (
	PathCountOne    = both("%d 條", "%d path")
	PathCountMany   = both("%d 條", "%d paths")
	MapCountOne     = both("%d 張", "%d map")
	MapCountMany    = both("%d 張", "%d maps")
	ReportCountOne  = both("%d 份", "%d report")
	ReportCountMany = both("%d 份", "%d reports")
	BranchCountOne  = both("%d 枝", "%d branch")
	BranchCountMany = both("%d 枝", "%d branches")
)

// A report row says which of the two kinds it is, because the two are read
// differently: a briefing is drawn by a program and shown here as bytes inside
// an isolated frame, a written report is a note like any other.
var (
	DailyBriefing = both("每日簡報", "Daily briefing")
	WrittenReport = both("書庫筆記", "Vault note")
)
