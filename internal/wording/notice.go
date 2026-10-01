package wording

// What the pages say when the folder is not being published as it stands. A
// notice has a title, which names the state, and a sentence, which says why and
// what to do about it. The files a notice is about are shown beside the
// sentence exactly as they were reported, so neither phrase carries a value.
var (
	NoticeNamesCollideTitle = both("變更沒有發布", "Changes are not being published")
	NoticeNamesCollide      = both(
		"有兩個檔案的名稱相同，只差在 Unicode 的正規化形式（例如 NFC 與 NFD）。yomihon 不會猜哪一個才對，所以最近新增或修改的內容都不會出現在頁面上。刪除或改名其中一個，頁面就會恢復更新。",
		"Two files have one name: they differ only in Unicode normalization form (for example NFC and NFD). yomihon will not guess which one is meant, so nothing added or changed since then is appearing on the pages. Delete or rename one of them and the pages resume updating.",
	)
)
