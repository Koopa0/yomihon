package wording

// OpenThoughtsTitle names a return to the reader's own unfinished material.
var OpenThoughtsTitle = both("還沒想通的", "What you left open")

// OpenThoughtsLede says what the shelf holds where the contract declares the
// type a reader's thoughts are filed under: the marked locations, and the
// thoughts that still declare draft: true.
var OpenThoughtsLede = both("你標記為「還不確定」的位置，以及還沒寫完的想法。", `Locations you marked "Not sure yet", and thoughts you have not finished.`)

// OpenThoughtsMarksLede describes the shelf when no usable answer role exists.
// It says what the shelf holds and nothing of its order, which the reader
// chooses.
var OpenThoughtsMarksLede = both("你標記為「還不確定」的位置。", `Locations you marked "Not sure yet".`)

// OpenThoughtKind labels a row that is a note of the reader's own, where a
// marked location is labelled "Not sure yet".
var OpenThoughtKind = both("想法", "Thought")

// The shelf of what was left open can be ordered three ways, and says it is
// being asked which.
var (
	OpenOrderLabel  = both("排序", "Order")
	OpenOrderNewest = both("最新在前", "Newest first")
	OpenOrderSource = both("依來源", "By source")
	OpenOrderOldest = both("最早在前", "Oldest first")
)

// OpenThoughtsMarksEmpty is the marks-only empty state.
var OpenThoughtsMarksEmpty = both("尚無「還不確定」的位置標記。", `No locations marked "Not sure yet".`)
