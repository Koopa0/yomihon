package wording

// FreshnessNewVersion is what the reading page says once the file has moved on
// from the words below it and reloading would show the newer ones.
var FreshnessNewVersion = both("此筆記已有新版本。", "This note has a newer version.")

// FreshnessReload names the control beside that sentence. It is a verb: what
// the sentence reports and what the control does are two different things, and
// a label repeating the report would leave the action unnamed.
var FreshnessReload = both("重新載入", "Reload")

// The rest of what the freshness watch says. Like the notice above, these are
// rendered onto the page and read off it by the script: the words are the
// server's, and a second copy inside a JavaScript file would be a second place
// to translate them.
var (
	FreshnessPreparing = both("此筆記已有新版本，頁面資料準備中…", "This note has a newer version; the page is catching up…")
	FreshnessGone      = both(
		"此筆記已經不在原本的位置了，可能被搬到別處，也可能已刪除。",
		"This note is no longer where it was. It may have moved, or it may have been deleted.",
	)
	FreshnessSearchTitle         = both("搜尋這個標題", "Search for this title")
	FreshnessHoldPreparingTitle  = both("頁面資料準備中…", "The page is catching up…")
	FreshnessHoldPreparingDetail = both(
		"這篇筆記在磁碟上已經變更，閱讀頁還沒讀到那一版；準備好之後這個連結會自己恢復。",
		"This note changed on disk and the reading page has not caught up. This link restores itself once it has.",
	)
	FreshnessHoldGoneTitle = both("筆記已不在原處", "The note has moved")
)
