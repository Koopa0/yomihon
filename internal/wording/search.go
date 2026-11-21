package wording

import (
	"fmt"
	"strings"
)

// The search face: the page, its live fragment, and what it says when it
// found nothing.

// SearchTitle is the page's own heading. Its field is named by the same pair
// of words the desk field and the palette carry — one thing, said once.
var SearchTitle = both("搜尋", "Search")

// SearchTitleFor is what the tab of a search page says before the vault's name:
// the word and the query, which is what tells a reader's searches apart. A
// query is one line of the words that were typed, and a blank one is the page's
// own title.
func SearchTitleFor(lang Lang, query string) string {
	words := strings.Join(strings.Fields(query), " ")
	if words == "" {
		return SearchTitle.In(lang)
	}
	return SearchTitle.In(lang) + both("：", ": ").In(lang) + words
}

// What the results say about their own limits.
var (
	MetadataSearchUnavailable = both("依欄位篩選目前無法使用。", "Filtering by field is unavailable right now.")
	GovernanceUnavailable     = both("狀態與類型資料目前無法使用。", "Status and type data are unavailable right now.")
)

// The empty result, and the way back out of it.
var (
	StepBackLabel          = both("改用較短的詞：", "Try a shorter query: ")
	StepBackOpen           = both("「", "\"")
	StepBackClose          = both("」", "\"")
	StepBackCountFmt       = both("（%d 筆）", " (%d)")
	FolderSuggestionsLabel = both("folder 篩選器要填從書庫根目錄起算的路徑。你是指：", "folder: takes a path from the vault root. Did you mean: ")
)

// The terms a query held in an operator's shape but searched for as words.
// Each term carries its own quotes, so two terms are not read as one term
// that happens to contain a comma, and the sentence agrees with how many
// terms it names.
var (
	literalOperatorTermFmt = both("「%s」", "%q")
	literalOperatorFmt     = both("%s不是搜尋運算子，已當一般文字搜尋。", "%s is not a search operator; it was searched for as ordinary text.")
	literalOperatorsFmt    = both("%s都不是搜尋運算子，已當一般文字搜尋。", "%s are not search operators; they were searched for as ordinary text.")
)

// LiteralOperatorSentence explains, in lang, that terms were searched for as
// ordinary words although they look like operators.
func LiteralOperatorSentence(lang Lang, terms []string) string {
	quoted := make([]string, len(terms))
	for i, term := range terms {
		quoted[i] = fmt.Sprintf(literalOperatorTermFmt.In(lang), term)
	}
	sentence := literalOperatorFmt
	if len(terms) > 1 {
		sentence = literalOperatorsFmt
	}
	return fmt.Sprintf(sentence.In(lang), strings.Join(quoted, ListSeparator.In(lang)))
}

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
		"「%s」不是 yomihon 認得的篩選器，已當一般文字搜尋。認得的是：",
		"%q is not a filter yomihon knows, so it was searched for as ordinary text. The ones it knows are: ")

	UnknownFilterEnd = both("。", ".")
)

// ListSeparator joins the items of a short list written into a sentence. The
// ideographic comma is the one Traditional Chinese writes between list items
// and reads as nothing at all in English, where an English sentence handed one
// looks broken rather than translated.
var ListSeparator = both("、", ", ")

// ResultAliasLabel introduces the name a result answered to when it was not
// the title the row shows. Without the word, the name would sit beside the
// title looking like a second title. The gap after the label belongs to the
// phrase in both languages, so the space a reader sees is one an author chose
// rather than one a template happened to keep.
var ResultAliasLabel = both("又稱 ", "also called ")

// ResultTagLabel names the frontmatter tag that explains a tag-only result.
var ResultTagLabel = both("標籤： ", "tag: ")

// SearchHitUnlocated is the line a result carries when the index found a
// match the opened page cannot locate — a phrase that spans two blocks, and
// nothing in the first of them a text directive can name. Finding something
// and then silently failing to show it is the one outcome this sentence
// exists to remove.
var SearchHitUnlocated = both(
	"這頁找不到這段相符的文字。",
	"The match could not be located on this page.",
)

// What the live search says about an answer it fetched, which is counted in
// the browser and therefore said there. The placeholders are filled in by the
// page's own script: one form for a single result and one for any other
// number, because the two languages disagree about where a plural shows and
// joining fragments would put the count in the wrong place in one of them.
var (
	LiveSearchCountOne  = both("「{query}」有 1 筆結果。", "1 result for \"{query}\".")
	LiveSearchCountMany = both("「{query}」有 {count} 筆結果。", "{count} results for \"{query}\".")
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
	"The results below answer the earlier query \"%s\".")

// The words the kind row shows for a note type that the rules give a role. Any
// other type is the vault's own word and is shown as written.
var (
	SearchKindUnit    = both("單元", "Unit")
	SearchKindCourse  = both("課程", "Course")
	SearchKindIndex   = both("索引", "Index")
	SearchKindNote    = both("筆記", "Note")
	SearchKindDrill   = both("句型練習", "Sentence practice")
	SearchKindThought = both("想法", "Thought")
)

// The results page's own controls: what the order is by, what the list is
// narrowed to, and how it goes on.
var (
	SearchSortRelevance = both("依相關", "By relevance")
	SearchSortRecent    = both("依最近變更", "By last change")
	SearchSortLabel     = both("排序", "Order")
	SearchKindLabel     = both("種類", "Kind")
	SearchLocationLabel = both("位置", "Location")
	SearchAll           = both("全部", "All")
	SearchTopLevel      = both("最上層", "Top level")
	SearchShowMoreFmt   = both("再顯示 %d 篇", "Show %d more")
	SearchShownOfFmt    = both("%s / %s", "%s of %s")
	SearchNoneFmt       = both("沒有符合「%s」的筆記", "No note matches \"%s\"")
	SearchDidYouMeanFmt = both("要找的是「%s」嗎？", "Did you mean \"%s\"?")
	SearchBrowseFolders = both("瀏覽資料夾", "Browse folders")
	SearchSeeAllFmt     = both("看全部 %s 篇", "See all %s")
	SearchRecent        = both("最近搜尋", "Recent searches")
	SearchClear         = both("清除", "Clear")
	SearchClearField    = both("清除輸入", "Clear the field")
	SearchCancel        = both("取消", "Cancel")
)

// SearchSearchFor is the row that offers the nearest word in place of a query
// nothing matched, written as the search it runs.
var SearchSearchFor = both("搜尋「%s」", "Search for \"%s\"")

// The brackets a results page puts round the query it is titled by.
var (
	SearchQueryOpen  = both("「", "\"")
	SearchQueryClose = both("」", "\"")
)
