package wording

// Folders, files yomihon cannot render, reports, and the page for an address
// that answered with nothing.

// What a folder listing counts, and what it says when there is nothing to list.
var (
	SubfolderCountOne  = both("%d 個資料夾", "%d folder")
	SubfolderCountMany = both("%d 個資料夾", "%d folders")
	// CountJoin sits between the folders and the notes a level holds.
	CountJoin           = both(" · ", " · ")
	FolderNoteCountOne  = both("%d 篇", "%d note")
	FolderNoteCountMany = both("%d 篇", "%d notes")
	FolderEmpty         = both("這個資料夾是空的。", "This folder is empty.")
	// RootNotes names the markdown files that sit at the vault root, so the
	// folder shelf does not read them as another folder.
	RootNotes = both("最上層的筆記", "Top-level notes")
	// OtherFiles names the files a folder holds that are not notes. They stay
	// listed, and they are not counted as 篇.
	OtherFiles = both("其他檔案", "Other files")
)

// What a list says about its own order and its own length.
var (
	AllNotes      = both("全部 %d 篇", "All %d notes")
	NotesGroup    = both("筆記", "Notes")
	TopLevel      = both("最上層", "Top level")
	SortByName    = both("依名稱", "By name")
	SortByOrder   = both("依順序", "In order")
	SortByTitle   = both("依標題", "By title")
	SortByRecent  = both("依最近變更", "By latest change")
	MonthHeading  = both("%d 月", "Month %d")
	FilterNotes   = both("篩選 %s 篇", "Filter %s notes")
	ExpandFolders = both("展開資料夾", "Open the folders")
	FoldFolders   = both("收起資料夾", "Close the folders")
	ClearFilter   = both("清除篩選", "Clear the filter")
	NoMatch       = both("沒有符合「%s」的筆記", `No note matches "%s"`)
	// FilterField labels the field for a reader who cannot see its hint.
	FilterField = both("篩選這個列表", "Filter this list")
)

// A file yomihon has no reader for: it says so, and offers the bytes.
var (
	NoReaderForFile = both(
		"這裡沒有這種檔案的閱讀器；原始檔仍可下載。",
		"There is no reader here for this kind of file; the original file can still be downloaded.",
	)
	NoteOverReadingLimit = both(
		"這篇筆記的大小是 %s，超過 %s 的閱讀上限；原始檔仍可下載。",
		"This note is %s, over the %s reading limit; the original file can still be downloaded.",
	)
	// A Markdown file the rules leave out of the vault is a document,
	// not a note, even when its size is what keeps it from being read.
	DocumentOverReadingLimit = both(
		"這份文件的大小是 %s，超過 %s 的閱讀上限；原始檔仍可下載。",
		"This document is %s, over the %s reading limit; the original file can still be downloaded.",
	)
	NoteReadingLimitHealth = both("在整體狀況查看這篇筆記", "View this note on the Health page")
	FileName               = both("名稱", "Name")
	FileSize               = both("大小", "Size")
	FileType               = both("類型", "Type")
	OpenRawBytes           = both("開啟原始位元組", "Open the raw bytes")
	DownloadFile           = both("下載檔案", "Download the file")
	FileNotIndexed         = both("這個檔案的內容不會被搜尋讀取。", "This file's contents are not read by search.")
)

// A Markdown file the rules leave out of the vault still opens as a
// document. The line above it says what it is not: a note, so nothing about it
// is indexed or checked.
var (
	FileKindDocument    = both("文件", "Document")
	SkippedDocumentNote = both(
		"這份文件不在書庫的索引與檢查範圍內。",
		"This document is outside the vault's index and checks.",
	)
)

// The page for an address that named nothing, and for a file that exists and
// could not be read. They are different repairs, so they are different pages.
var (
	ParseFailedKicker = both("解析失敗", "Parse failed")
	ParseFailedTitle  = both("這篇筆記的文字無法解析", "This note's text could not be parsed")
	ParseFailedLede   = both(
		"檔案已開啟，但 yomihon 解析這篇筆記的文字時失敗了。請依解析器的訊息檢查筆記內容。",
		"The file opened, but yomihon could not parse this note's text. Check the note's content against the parser's message.",
	)
	ParseFailedNext = both(
		"在原本的編輯器檢查這篇筆記的內容。修正並存檔後，重新整理這一頁。",
		"Check this note's content in your editor. After saving a correction, reload this page.",
	)
	NotReadableKicker = both("讀不進來", "Unreadable")
	NotReadableTitle  = both("這個檔案目前讀不進來", "This file cannot be read right now")
	NotReadableLede   = both(
		"檔案存在，但這一次讀取沒有成功——可能是權限設定，或另一個程式正擋住它。排除之後，過幾秒重新整理就會看到內容。",
		"The file is there, but this read did not succeed — a permission, or another program holding it. Once that clears, reloading in a few seconds shows it.",
	)
	// NothingHere titles the page. It says what is true of the place rather
	// than what the reader did wrong: the address may be a typo, and it may be
	// a note nobody has written yet, and this page cannot tell which.
	NothingHere    = both("這裡沒有東西", "Nothing is here")
	NotFoundKicker = both("找不到", "Not found")
	NotFoundLede   = both(
		"這個網址沒有筆記或檔案。可能是網址打錯了，也可能這一篇還沒有寫。",
		"There is no note and no file at this address. The address may be a typo, or the note may be one nobody has written yet.",
	)
	AddressAsked = both("你要找的網址", "The address you asked for")
	WhatNext     = both("下一步", "What next")
	NotFoundNext = both(
		"從閱讀桌的資料夾往下找，或用上方的搜尋。",
		"Work down from the folders on the desk, or use the search above.",
	)
	LeaveThisPage = both("離開這一頁", "Leave this page")
	BackHome      = both("返回閱讀桌", "Back to the desk")
)

// What the raw-bytes routes say when they cannot answer. These reach a reader
// as a plain-text body rather than a page, so they are one line each.
// SandboxUnavailable is the refusal when the file read perfectly well and the
// isolation its bytes are served under could not be established, so the bytes
// were withheld instead.
var (
	FileNotFound       = both("找不到指定的檔案", "That file was not found")
	FileUnreadable     = both("無法讀取檔案", "That file could not be read")
	SandboxUnavailable = both("這份內容無法安全顯示，因此沒有提供", "This content cannot be shown safely, so it was not served")
)

// ReadInFull is the way from a folder's cover paragraph to the whole README.
var ReadInFull = both("讀全文", "Read in full")
