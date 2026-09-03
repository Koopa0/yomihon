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
	// FreshnessWriteHold is said beside the status controls once the watch has
	// learned the page no longer shows what a reload would. A press from here
	// would make a ruling over words the reader has not seen — and where what
	// moved is an excerpt pulled in from another note, the host's own identity
	// still matches, so nothing downstream would refuse the press; the hold is
	// the only guard. The sentence names the way back instead of leaving a
	// dead control unexplained.
	FreshnessWriteHold = both(
		"此頁已與檔案不同步，狀態按鈕已停用；請先重新載入。",
		"This page no longer matches the file; the status controls are disabled. Reload first.",
	)
)

// What the freshness endpoint says when it cannot answer at all. The page
// polls it with the identity of the bytes it drew and never reads these
// bodies, so they reach a person only through an address assembled by hand —
// which is exactly when answering in a language they did not choose, or in the
// router's own English, is a small piece of the same rudeness the reading
// pages were built to stop.
var (
	FreshnessNotWatchable = both("這個位置沒有可以追蹤的筆記", "There is no note to watch at that address")
	// The first substitution is a query field's own name, which is written in
	// the address and is not a word to translate.
	FreshnessIdentityFmt = both("%s 必須是 %d 位十六進位數字", "%s must be %d hex digits")
)
