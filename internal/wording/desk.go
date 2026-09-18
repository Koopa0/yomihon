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

// What an empty path or map listing says. Those two shelves are filled by a
// declaration and by nothing else: a markdown file added to the folder is read,
// listed and searchable without ever reaching either of them, so the sentence
// names the declaration that does reach them and the edit that makes one. The
// type word is the contract's, interpolated here rather than written, because a
// word spelled in this package is a second copy of the vault's vocabulary and
// goes on saying what that vault has since renamed.
var (
	NoDeclaredTypeEmptyFmt = both(
		"還沒有筆記帶著 type: %s。在一篇筆記的 frontmatter 裡寫上這一行，它就會出現在這裡。",
		"No note declares type: %s yet. Give a note that type in its frontmatter and it appears here.",
	)
	// The same sentence where the contract declares more than one type for the
	// shelf, which is one list and any of its words.
	NoDeclaredTypesEmptyFmt = both(
		"還沒有筆記帶著 type: %s。在一篇筆記的 frontmatter 裡寫上其中一個，它就會出現在這裡。",
		"No note declares type: %s yet. Give a note one of those types in its frontmatter and it appears here.",
	)
	// What an empty folder listing says, and the one step its reader can take
	// next: a file added to the folder is on that shelf, which is the promise
	// only this desk can keep. The state sentence and the step are separate
	// phrases so a check can name each through wording.*.In without literals.
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
	// What a folder is told when the contract is there and this process never
	// read it, which is what a reader sees who added one while yomihon was
	// already reading the folder. The contract is read once, at start.
	IndexContractUnloaded = both(
		"這個資料夾有契約，但 yomihon 是在啟動時讀契約的，當時還沒有這一份。",
		"This folder has a contract, but yomihon reads the contract when it starts, and this one was not there then.",
	)
	IndexContractUnloadedNext = both(
		"請重新啟動 yomihon。",
		"Restart yomihon.",
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

// ReportUndated is what leads a report that carries no day, where the others
// lead with one. A shelf read by date needs an answer in that column from every
// row: a blank there reads as a day nobody has looked up yet, and this says
// instead that the report never wrote one.
var ReportUndated = both("沒有寫日期", "No date")
