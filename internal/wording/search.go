package wording

// The search face: the page, its live fragment, and what it says when it
// found nothing.

// SearchTitle is the page's own heading, and SearchNotesField labels its field.
var (
	SearchTitle      = both("搜尋", "Search")
	SearchNotesField = both("搜尋筆記", "Search notes")
)

// The counts over a result list. The second form is for a search that returned
// everything it found, which is a different claim from one that was cut short.
var (
	ResultCountShownFmt = both("共 %d 筆，顯示前 %d 筆", "%d results; showing the first %d")
	ResultCountOne      = both("共 %d 筆", "%d result")
	ResultCountMany     = both("共 %d 筆", "%d results")
)

// What the results say about their own limits.
var (
	MetadataSearchUnavailable = both("中介資料搜尋目前無法使用。", "Metadata search is unavailable.")
	GovernanceUnavailable     = both("vault schema 的治理資料無法使用。", "The vault schema's governance data is unavailable.")
	ResultKindFile            = both("檔案", "File")
)

// The empty result, and the way back out of it.
var (
	NoResultsFmt     = both("找不到「%s」的結果。", "Nothing matches %q.")
	StepBackLabel    = both("退一步找：", "Try a shorter query: ")
	StepBackOpen     = both("「", "")
	StepBackClose    = both("」", "")
	StepBackCountFmt = both("（%d 筆）", " (%d)")
)

// What search covers, said where a reader has just been told it found nothing.
// The second form is for a library with no lifecycle to filter by, where the
// status hint would name a facility that is not there.
var (
	SearchScopeHintWithStatus = both(
		"搜尋涵蓋筆記，以及任何能以文字呈現的檔案；圖片、PDF 與其他二進位檔案只在導覽中列出，內容不會被讀取。請嘗試較少或不同的詞，或用 ",
		"Search covers notes and any file that can be shown as text. Images, PDFs and other binary files are listed in the navigation but never read. Try fewer or different words, or narrow by lifecycle with ",
	)
	SearchScopeHintAfterCommand = both(" 依生命週期縮小範圍。", ".")
	SearchScopeHint             = both(
		"搜尋涵蓋筆記，以及任何能以文字呈現的檔案；圖片、PDF 與其他二進位檔案只在導覽中列出，內容不會被讀取。請嘗試較少或不同的詞。",
		"Search covers notes and any file that can be shown as text. Images, PDFs and other binary files are listed in the navigation but never read. Try fewer or different words.",
	)
)

// What the search face says when the query itself is refused, and when the
// index behind it cannot answer. The two refusals name what is wrong with the
// query rather than restating that it failed.
var (
	SearchUnavailable   = both("搜尋目前暫時無法使用。", "Search is unavailable right now.")
	QueryTooLong        = both("搜尋字串過長", "The query is too long")
	QueryHasControlByte = both("搜尋字串含有控制字元", "The query contains a control character")
)
