package wording

// What the pages say when the folder is not being published as it stands. A
// notice has a title, which names the state, and a sentence, which says why and
// what to do about it. The files a notice is about are shown beside the
// sentence exactly as they were reported, so neither phrase carries a value.
//
// The title says the pages have stopped updating rather than that changes are
// not published: 發布 and "published" are what this product already calls a
// note's lifecycle status, and a vault that declares that status could read the
// title as a statement about it. The sentence says "names" rather than "files",
// because the pair can as well be two folders.
var (
	NoticeNamesCollideTitle = both("頁面已停止更新", "Pages have stopped updating")
	NoticeNamesCollide      = both(
		"這個資料夾裡有兩個名稱視為同一個，只差在 Unicode 的正規化形式（例如 NFC 與 NFD）。yomihon 不會猜哪一個才對，所以第二個名稱出現之後新增或修改的內容，都不會出現在頁面上。刪除或改名其中一個；等到資料夾裡不再有這樣的一對名稱，頁面就會恢復更新。",
		"Two names in this folder count as one: they differ only in Unicode normalization form (for example NFC and NFD). yomihon will not guess which one is meant, so nothing added or changed since the second name appeared is reaching the pages. Delete or rename one of them; the pages resume updating once no such pair remains.",
	)
)
