package wording

// The navigation rail: what it calls the places a reader can go, and what it
// says when a projection it would have shown is unavailable.

// LibraryNavigation names the rail itself for whoever reaches it by key.
var LibraryNavigation = both("書庫導覽", "Library navigation")

// VaultRoot is what the rail calls the folder everything else sits under, since
// that folder has no name of its own to show.
var VaultRoot = both("書庫根目錄", "Library root")

// The filter over the rail: its placeholder, its accessible name, and what it
// says when nothing matches.
var (
	FilterNavigation      = both("篩選導覽…", "Filter navigation…")
	FilterNavigationLabel = both("篩選導覽", "Filter navigation")
	FilterNoMatch         = both("沒有相符項目；按 Esc 清除篩選。", "Nothing matches. Press Esc to clear the filter.")
)

// CurrentFolder names the block listing the note's own neighbours.
var CurrentFolder = both("目前資料夾", "Current folder")

// MoreInFolder is the link to the rest of a folder the rail could not show
// whole. It carries the count, so it is a format rather than a sentence.
var MoreInFolder = both("另外 %d 篇 →", "%d more →")

// The rail's section headings.
var (
	PathsAndMaps = both("路徑與地圖", "Paths and maps")
	Paths        = both("路徑", "Paths")
	Maps         = both("地圖", "Maps")
	Journal      = both("日誌", "Journal")
	Reports      = both("報告", "Reports")
	Folders      = both("資料夾", "Folders")
	Newest       = both("最新", "Newest")
)

// OpenMap and OpenSyllabus are the links from a rail branch to the whole thing
// it is a branch of.
var (
	OpenMap      = both("開啟地圖", "Open the map")
	OpenSyllabus = both("開啟課綱", "Open the syllabus")
)

// What the rail says when a projection it would have listed cannot be built.
var (
	PathsMapsAndArtifactsUnavailable = both(
		"路徑、地圖與治理項目投影目前無法使用。",
		"Paths, maps and governed-item projections are unavailable.",
	)
	PathsAndMapsUnavailable = both("路徑與地圖目前無法使用。", "Paths and maps are unavailable.")
	ArtifactsUnavailable    = both("治理項目投影目前無法使用。", "Governed-item projections are unavailable.")
)

// CourseOrderOf names a study path's own order, which the foot of the article
// and the rail both walk. It carries the path's title, so it is a format.
var CourseOrderOf = both("%s 課程順序", "%s course order")

// FolderAdjacency is the other order the foot can walk: the files either side
// of this one inside its own folder.
var FolderAdjacency = both("同資料夾的前後檔案", "The files either side of this one in its folder")

// The step words at the foot of the article. Which pair is used says which
// order arrived, because a course's declared order and a folder's can disagree
// completely and the words are all a sighted reader has to tell them apart.
var (
	PreviousLesson = both("上一課", "Previous lesson")
	PreviousFile   = both("上一份", "Previous file")
	NextLesson     = both("下一課", "Next lesson")
	NextFile       = both("下一份", "Next file")
)
