package wording

// OpenThoughtsTitle names a return to the reader's own unfinished material.
var OpenThoughtsTitle = both("還沒想通的", "What you left open")

// OpenThoughtsCount formats the combined number of notes and marked locations.
var OpenThoughtsCount = both("%d 筆", "%d items")

// OpenThoughtsCountOne is the singular combined item count.
var OpenThoughtsCountOne = both("%d 筆", "%d item")

// OpenThoughtsLede says what the shelf holds where the contract declares the
// type a reader's thoughts are filed under: the marked locations, and the
// thoughts that still declare draft: true.
var OpenThoughtsLede = both("你標記為「還不確定」的位置，以及還沒寫完的想法。", `Locations you marked "Not sure yet", and thoughts you have not finished.`)

// OpenThoughtsMarksLede describes the shelf when no usable answer role exists.
var OpenThoughtsMarksLede = both("你標記為「還不確定」的位置，依時間由新到舊。", `Locations you marked as "Not sure yet", newest first.`)

// OpenThoughtsMarksEmpty is the marks-only empty state.
var OpenThoughtsMarksEmpty = both("尚無「還不確定」的位置標記。", `No locations marked "Not sure yet".`)
