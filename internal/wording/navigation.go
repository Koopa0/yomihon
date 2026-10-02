package wording

// HealthNavigationTitle names a navigation build failure independently of note readability.
var HealthNavigationTitle = both("導覽建構失敗", "Navigation could not be built")

// HealthNavigationLede explains the scope of an omitted navigation tree.
var HealthNavigationLede = both(
	"這些筆記的導覽無法建構，因此未列入導覽。筆記仍可閱讀與搜尋，其他筆記的導覽仍可使用。",
	"Navigation could not be built for these notes, so their navigation entries are omitted. The notes remain readable and searchable; other notes’ navigation remains available.",
)
