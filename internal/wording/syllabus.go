package wording

// The syllabus: a study path read as a course rather than as a note.

// SyllabusKicker sits above the title. The Japanese half is the vault's own
// word for the thing and is not translated: it is what the notes call it.
var SyllabusKicker = both("課綱 · ", "Syllabus · ")

// The invitation shown to a reader who has not walked this path before, and
// the way into the note it points at. The lede says what this page is — the
// structural projection — and where the rest of the author's text lives. It
// claims nothing about what that text contains: the page has not read it,
// and the words it used to promise appeared in no note at all.
var (
	FirstTimeHere = both("第一次使用這條路徑？", "First time on this path?")
	FirstTimeLede = both(
		"這一頁只列出宣告的課程結構；結構之外作者還寫了什麼，都在筆記本文的頁面上。",
		"This page lists only the declared course structure; whatever else the author wrote is on the note's own page.",
	)
	ReadTheGuide = both("閱讀筆記本文 ", "Read the note itself ")
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

// What the page says over an empty course, split by which repair is due.
// The first pair teaches the grammar to a note that wrote no marker: which
// markers declare structure, and that a heading without one is not read as a
// part — the marker forms themselves are spelled between the two halves,
// from the grammar's own vocabulary. The second pair is for a note that did
// write markers the grammar could not use: telling that author no marker
// exists would be false, so it states what a marker may say instead — the
// same closed set of values, between the same joiners. Whose job the repair
// is stays the same sentence in both.
var (
	NoCourseIntro = both(
		"這條路徑沒有讀到任何課程結構。分部與課次是由標題或開啟清單那一行行尾的 ",
		"No course structure could be read from this path. Parts and lessons are declared by a ",
	)
	NoCourseNoMarkerTail = both(
		" 標記宣告的；沒有標記的標題不會被讀成分部，底下的連結也不會被讀成課。yomihon 只陳述，不修復；請直接編輯這篇筆記。",
		" marker at the end of a heading, or of the row that opens a list. A heading without one is not read as a part, and the links under it are not read as lessons. yomihon reports and never repairs; edit this note directly.",
	)
	NoCourseMarkerFaultIntro = both(
		"這條路徑寫了 sequence 標記，但沒有一個分部能讀成課程結構。標記的值只能是 ",
		"This path writes sequence markers, but none of its branches could be read as course structure. A marker takes exactly one of ",
	)
	NoCourseMarkerFaultTail = both(
		" 三者之一，寫在標題或開啟清單那一行的行尾。yomihon 只陳述，不修復；請直接編輯這篇筆記。",
		", at the end of a heading or of the row that opens a list. yomihon reports and never repairs; edit this note directly.",
	)
	// The joiners between the three spelled values. The two languages use
	// different marks and different words, so they are phrases, not markup.
	ValueJoin     = both("、", ", ")
	ValueJoinLast = both(" 或 ", " or ")
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

// The parentheses the offscreen explanation of a link sits inside, and the
// ones a syllabus uses. They are marks rather than words, and the two
// languages do not use the same ones.
var (
	ParenOpen  = both("（", " (")
	ParenClose = both("）", ")")
)

// AssetNotFound, ReportNotFound and PathNotFound are what the three
// address-shaped routes say when they answer with nothing. Each names what was
// looked for, because the address alone does not say which route refused it.
var (
	AssetNotFound  = both("找不到指定的資產", "That asset was not found")
	ReportNotFound = both("找不到指定的報告", "That report was not found")
	PathNotFound   = both("找不到指定的學習路徑", "That study path was not found")
)
