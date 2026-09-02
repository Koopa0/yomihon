package wording

// Folders, files yomihon cannot render, reports, and the page for an address
// that answered with nothing.

// What a folder listing counts, and what it says when there is nothing to list.
var (
	SubfolderCountOne   = both("%d 個資料夾、", "%d folder, ")
	SubfolderCountMany  = both("%d 個資料夾、", "%d folders, ")
	FolderNoteCountOne  = both("%d 篇", "%d note")
	FolderNoteCountMany = both("%d 篇", "%d notes")
	Subfolders          = both("子資料夾", "Subfolders")
	FilesInFolder       = both("這個資料夾裡的檔案", "Files in this folder")
	FolderEmpty         = both("這個資料夾是空的。", "This folder is empty.")
)

// A file yomihon has no reader for: it says so, and offers the bytes.
var (
	NoReaderForFile = both(
		"這裡沒有可呈現此檔案的閱讀器；仍可取得未修改的原始位元組。",
		"There is no reader here for this file. Its bytes are still available, unchanged.",
	)
	FileName       = both("名稱", "Name")
	FileSize       = both("大小", "Size")
	FileType       = both("類型", "Type")
	OpenRawBytes   = both("開啟原始位元組", "Open the raw bytes")
	FileNotIndexed = both("這個檔案的內容不會被搜尋讀取。", "This file's contents are not read by search.")
)

// ReportAsIs says what a report loses by being shown here rather than run.
var ReportAsIs = both(
	"這份文件以原樣顯示。它有一部分要靠程式繪製，而程式不會在這裡執行——那些部分不會出現。",
	"This document is shown as it was written. Parts of it are drawn by code, and code does not run here, so those parts are absent.",
)

// The page for an address that named nothing, and for a file that exists and
// could not be read. They are different repairs, so they are different pages.
var (
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
		"這個位置沒有筆記或檔案。可能是網址打錯了，也可能這一篇還沒有寫。",
		"There is no note and no file at this address. The address may be a typo, or the note may be one nobody has written yet.",
	)
	AddressAsked = both("你要找的位置", "The address you asked for")
	WhatNext     = both("下一步", "What next")
	NotFoundNext = both(
		"從左邊的資料夾往下找，或用上方的搜尋找筆記裡的字。",
		"Work down the folders on the left, or use the search above to look inside notes.",
	)
	LeaveThisPage = both("離開這一頁", "Leave this page")
	BackHome      = both("返回首頁", "Back to home")
)

// The syllabus's own units, in the same pairs and for the same reason.
var (
	PartCountOne    = both("%d 部", "%d part")
	PartCountMany   = both("%d 部", "%d parts")
	ModuleCountOne  = both("%d 模組", "%d module")
	ModuleCountMany = both("%d 模組", "%d modules")
)

// What the raw-bytes routes say when they cannot answer. These reach a reader
// as a plain-text body rather than a page, so they are one line each.
// SandboxUnavailable is the refusal both of those routes share: the file read
// perfectly well and the isolation its bytes are served under could not be
// established, so the bytes were withheld instead.
var (
	FileNotFound       = both("找不到指定的檔案", "That file was not found")
	FileUnreadable     = both("無法讀取檔案", "That file could not be read")
	SandboxUnavailable = both("無法為此內容建立隔離，因此沒有提供", "This content's isolation could not be established, so it was not served")
)
