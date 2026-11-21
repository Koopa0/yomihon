package wording

import "fmt"

// What the pages say when the folder is not being published as it stands. A
// notice has a title, which names the state, and a sentence, which says why and
// what to do about it. The files a notice is about are shown beside the
// sentence exactly as they were reported, so neither phrase carries a value.
//
// The title says the pages have stopped updating rather than that changes are
// not published: 發布 and "published" are words a note's own frontmatter may
// use for itself, and a title that used them could be read as a statement about
// one. The sentence says "names" rather than "files", because the pair can as
// well be two folders.
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
	NoticeNamesCollideTitle = both("頁面已停止更新", "Pages have stopped updating")
	NoticeNamesCollide      = both(
		"這個資料夾裡有兩個名稱視為同一個，只差在 Unicode 的正規化形式（例如 NFC 與 NFD）。yomihon 不會猜哪一個才對，所以第二個名稱出現之後新增或修改的內容，都不會出現在頁面上。刪除或改名其中一個；等到資料夾裡不再有這樣的一對名稱，頁面就會恢復更新。",
		"Two names in this folder count as one: they differ only in Unicode normalization form (for example NFC and NFD). yomihon will not guess which one is meant, so nothing added or changed since the second name appeared is reaching the pages. Delete or rename one of them; the pages resume updating once no such pair remains.",
	)
)

// NoticeRulesVersionTitle heads the notice for a rules file written for a
// version of the schema this one does not read.
var NoticeRulesVersionTitle = both("規則檔的版本不對", "The rules file is for another version")

// RulesVersionSummary says what differs: the version the file declares, and the
// one this version reads. An empty declared means the file has none.
func RulesVersionSummary(lang Lang, declared, supported string) string {
	if declared == "" {
		return fmt.Sprintf(both(
			"規則檔沒有 schema_version，這一版讀 \"%s\"。",
			"The rules file has no schema_version; this version reads \"%s\".",
		).In(lang), supported)
	}
	return fmt.Sprintf(both(
		"規則檔寫的是 schema_version = \"%s\"，這一版讀 \"%s\"。",
		"The rules file says schema_version = \"%s\"; this version reads \"%s\".",
	).In(lang), declared, supported)
}

// RulesVersionDetail says which line to change, for a file whose only fault is
// that line. A line of zero means none was found, so the line is to be added.
func RulesVersionDetail(lang Lang, line int, supported string) string {
	if line == 0 {
		return fmt.Sprintf(both(
			"加上 schema_version = \"%s\" 這一行。",
			"Add the line schema_version = \"%s\".",
		).In(lang), supported)
	}
	return fmt.Sprintf(both(
		"把第 %d 行改成 schema_version = \"%s\"。",
		"Change line %d to schema_version = \"%s\".",
	).In(lang), line, supported)
}

// rulesOlderFormat is what a rules file written in an older format is told in
// place of a line to change: its keys differ, so the version line alone does not
// make it readable.
var rulesOlderFormat = both(
	"這份書庫規則使用舊格式。請依升級說明遷移規則檔與筆記欄位。",
	"This vault's rules use an older format. Follow the upgrade guide to migrate the rules file and the note fields.",
)

// rulesUpgradeGuideWhere says where that upgrade guide is: the file in the
// repository, which a reader of an installed binary has no copy of, and the
// address it can be opened at. yomihon only prints it; it makes no request.
var rulesUpgradeGuideWhere = both(
	"升級說明在 yomihon repo 的 docs/upgrading.md，網址是 https://github.com/koopa0/yomihon/blob/main/docs/upgrading.md",
	"The upgrade guide is docs/upgrading.md in the yomihon repository, at https://github.com/koopa0/yomihon/blob/main/docs/upgrading.md",
)

// RulesOlderRepair is what a rules file in an older format is told: to migrate
// it, and where the upgrade guide is. The two come apart because the command
// line puts each on a line of its own and a page runs them together.
func RulesOlderRepair(lang Lang) (migrate, where string) {
	return rulesOlderFormat.In(lang), rulesUpgradeGuideWhere.In(lang)
}

// RulesVersionOlder is the summary for a rules file written in an older
// format: the versions that differ, what to do, and where the upgrade guide is.
func RulesVersionOlder(lang Lang, declared, supported string) string {
	gap := ""
	if lang == En {
		gap = " "
	}
	migrate, where := RulesOlderRepair(lang)
	return RulesVersionSummary(lang, declared, supported) + gap + migrate + gap + where
}

// RootDetail labels the selected path and startup name in the reader's language.
// Both values already carry their filesystem spelling, including any escapes.
func RootDetail(lang Lang, selected, opened string) string {
	return fmt.Sprintf(both(
		"選取路徑：%s；啟動時開啟的名稱：%s",
		"Selected path: %s; Startup opened name: %s",
	).In(lang), selected, opened)
}
