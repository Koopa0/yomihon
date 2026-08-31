package wording

// The syllabus: a study path read as a course rather than as a note.

// SyllabusKicker sits above the title. The Japanese half is the vault's own
// word for the thing and is not translated: it is what the notes call it.
var SyllabusKicker = both("課綱 · ", "Syllabus · ")

// The invitation shown to a reader who has not walked this path before, and
// the way into the guide it points at.
var (
	FirstTimeHere = both("第一次使用這條路徑？", "First time on this path?")
	FirstTimeLede = both(
		"先讀課程目的、每日節奏、支線分工與完成標準，再開始第一課。",
		"Read what the course is for, the daily rhythm, how the branches divide, and what counts as finished — then start the first lesson.",
	)
	ReadTheGuide = both("閱讀完整使用方式 ", "Read the full guide ")
)

// A branch's own label. Local branches carry their heading; the main line does
// not need one. Either can resume an order already open above it, and the
// reader needs to know which they are looking at.
var (
	BranchPrefix    = both("支線：", "Branch: ")
	BranchContinued = both("（接續）", " (continued)")
	MainLine        = both("主線", "Main line")
	MainContinued   = both("主線（接續）", "Main line (continued)")
	BranchTag       = both("支線", "Branch")
	ModuleTagFmt    = both("模組 %d", "Module %d")
)

// The syllabus's own rail.
var (
	StudyPathNav = both("學習路徑導覽", "Study path navigation")
	StudyPaths   = both("學習路徑", "Study paths")
	ThisPath     = both("本路徑", "This path")
)

// What the page says when the note declares no structure at all: the grammar
// that would have made one, and whose job the repair is.
var (
	NoStructureBefore = both("這條路徑沒有讀到任何課程結構。分部與課次是由標題後面的 ", "No course structure could be read from this path. Parts and lessons are declared by the ")
	NoStructureAfter  = both(
		" 標記宣告的，沒有標記的標題不會被讀成分部，底下的連結也不會被讀成課。yomihon 只陳述，不修復；請直接編輯這篇筆記。",
		" markers after a heading. A heading without one is not read as a part, and the links under it are not read as lessons. yomihon reports and never repairs; edit this note directly.",
	)
)

// How far an entry's link got. The label is the short word beside the entry;
// the title is what a reader gets on hover, and says what went wrong.
var (
	EntryUnresolved       = both("未解析", "Unresolved")
	EntryAmbiguous        = both("有歧義", "Ambiguous")
	EntryNonInstance      = both("非治理項目", "Not governed")
	EntryResolved         = both("已解析", "Resolved")
	EntryUnresolvedTitle  = both("找不到目標", "The target was not found")
	EntryAmbiguousTitle   = both("目標有歧義", "The target is ambiguous")
	EntryNonInstanceTitle = both("目標不屬於生命週期治理範圍", "The target is outside lifecycle governance")
)
