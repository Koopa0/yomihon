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
	StepBackOpen     = both("「", "\"")
	StepBackClose    = both("」", "\"")
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

// What the search page says about a word written before a colon that the
// grammar does not accept. The term was searched for as text — that is the
// honest reading and it is what ran — and the sentence exists because without
// it the page is indistinguishable from the one a search for nothing returns.
//
// The keys it goes on to offer are supplied by the parser, so the two halves
// of the sentence cannot come to disagree about what a filter is.
var (
	UnknownFilterFmt = both(
		"「%s」不是 yomihon 認得的篩選器,已當一般文字搜尋。認得的是:",
		"%q is not a filter yomihon knows, so it was searched for as ordinary text. The ones it knows are: ")

	UnknownFilterEnd = both("。", ".")
)

// ListSeparator joins the items of a short list written into a sentence. The
// ideographic comma is the one Traditional Chinese writes between list items
// and reads as nothing at all in English, where an English sentence handed one
// looks broken rather than translated.
var ListSeparator = both("、", ", ")

// FilterKeysAvailable opens the blank search page with the constraints the
// field understands. A reader who has typed nothing cannot discover them any
// other way, and this is the one moment saying so costs them no answer.
var FilterKeysAvailable = both("可用的篩選器:", "Filters you can use: ")

// SearchEmptyTitle and SearchEmptyLede open the search page before anything has
// been asked. The prefixes below them are the field's grammar, which is worth
// knowing and is not what a reader who has just arrived is looking for.
var (
	SearchEmptyTitle = both("搜尋這個書庫", "Search this library")
	SearchEmptyLede  = both(
		"打字找筆記的標題或內文；按 Enter 看全部結果。",
		"Type to find a note by its title or its text; press Enter for every result.",
	)
)

// ResultAliasLabel introduces the name a result answered to when it was not
// the title the row shows. Without the word, the name would sit beside the
// title looking like a second title. The gap after the label belongs to the
// phrase in both languages, so the space a reader sees is one an author chose
// rather than one a template happened to keep.
var ResultAliasLabel = both("別名: ", "also called ")

// What the live search says about an answer it fetched, which is counted in
// the browser and therefore said there. The placeholders are filled in by the
// page's own script: one form for a single result and one for any other
// number, because the two languages disagree about where a plural shows and
// joining fragments would put the count in the wrong place in one of them.
var (
	LiveSearchCountOne  = both("「{query}」有 1 筆結果。", "1 result for “{query}”.")
	LiveSearchCountMany = both("「{query}」有 {count} 筆結果。", "{count} results for “{query}”.")
	LiveSearchOffline   = both(
		"即時結果目前無法使用；按 Enter 執行完整搜尋。",
		"Live results are unavailable right now; press Enter for the full search.")
)

// LiveSearchStaleFmt names the query a set of results answers. It travels
// hidden with every answer and is shown only when a live refresh could not be
// made and the previous answer was left standing: rows nobody has replaced read
// as an answer to whatever has since been typed, and they would be saying so
// beside a sentence asking for the search to be run again. The query is spliced
// in by the server that answered it, so the sentence is whole in the language
// it is written in.
var LiveSearchStaleFmt = both(
	"下方結果回答的是先前的查詢「%s」。",
	"The results below answer the earlier query “%s”.")
