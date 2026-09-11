package wording

// The desk and the four mode index pages a reader enters it through.

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
		"一個主題底下有什麼。",
		"What's under one subject.",
	)
	DeskReportsLede = both(
		"每日簡報與稽核。",
		"Daily briefings and audits.",
	)
	DeskFoldersLede = both(
		"照檔案存放的位置瀏覽。",
		"Browse by where the files are stored.",
	)
)

// What an empty path or map listing says, and the one step a first-time reader
// can take next. The state sentence and the step are separate phrases so a
// check can name each through wording.*.In without literals.
var (
	PathIndexEmpty = both(
		"這個書庫裡還沒有學習路徑。",
		"This vault has no study paths yet.",
	)
	MapIndexEmpty = both(
		"這個書庫裡還沒有地圖。",
		"This vault has no maps yet.",
	)
	IndexDeclaredEmptyNext = both(
		"在 yomihon 正在讀的資料夾裡新增一個 .md 檔。",
		"Add a .md file to the folder yomihon is reading.",
	)
	// What a folder with no contract is told instead. It names what is missing
	// without speaking as if the folder had declared an empty set.
	IndexUngoverned = both(
		"這個資料夾還沒有契約。",
		"This folder has no contract yet.",
	)
	IndexUngovernedNext = both(
		"請新增 System/schemas/vault-schema.toml。",
		"Add System/schemas/vault-schema.toml.",
	)
	ReportIndexLede = both(
		"每日簡報與寫下的報告。",
		"Daily briefings and written reports.",
	)
	ReportIndexEmpty = both(
		"這個書庫裡沒有報告。",
		"There are no reports in this vault.",
	)
	FolderIndexLede = both(
		"依檔案的存放位置瀏覽。",
		"Browse files by where they are stored.",
	)
	FolderIndexEmpty = both(
		"這裡沒有列出檔案。",
		"No files are listed here.",
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
