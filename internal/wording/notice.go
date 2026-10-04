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
	NoticeRootChangedTitle = both("讀取的是啟動時的資料夾", "Reading the startup folder")
	NoticeRootChanged      = both(
		"選取的路徑已不再指向啟動時開啟的資料夾。yomihon 仍在讀取原本的資料夾；請重新啟動，開啟現在選取的資料夾。",
		"The selected path no longer points to the folder opened at startup. yomihon is still reading the original folder. Restart to open the folder now at the selected path.",
	)
	NoticeRootUnconfirmedTitle = both("資料夾身分無法確認", "Folder identity cannot be confirmed")
	NoticeRootUnconfirmed      = both(
		"目前無法確認選取的路徑是否仍指向啟動時開啟的資料夾。yomihon 仍使用原本開啟的資料夾；請恢復路徑的存取權，或重新啟動。",
		"The selected path cannot currently be confirmed to name the folder opened at startup. yomihon still uses the originally opened folder. Restore access to the path or restart.",
	)
	NoticeRootSelected      = both("選取路徑", "Selected path")
	NoticeRootOpened        = both("啟動時開啟的名稱", "Startup opened name")
	NoticeNamesCollideTitle = both("頁面已停止更新", "Pages have stopped updating")
	NoticeNamesCollide      = both(
		"這個資料夾裡有兩個名稱視為同一個，只差在 Unicode 的正規化形式（例如 NFC 與 NFD）。yomihon 不會猜哪一個才對，所以第二個名稱出現之後新增或修改的內容，都不會出現在頁面上。刪除或改名其中一個；等到資料夾裡不再有這樣的一對名稱，頁面就會恢復更新。",
		"Two names in this folder count as one: they differ only in Unicode normalization form (for example NFC and NFD). yomihon will not guess which one is meant, so nothing added or changed since the second name appeared is reaching the pages. Delete or rename one of them; the pages resume updating once no such pair remains.",
	)
)
