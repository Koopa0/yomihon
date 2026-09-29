package wording

// OpenThoughtsTitle names a return to the reader's own unfinished material.
var OpenThoughtsTitle = both("還沒說清楚的", "What you left open")

// OpenThoughtsCount formats the combined number of notes and marked locations.
var OpenThoughtsCount = both("%d 筆", "%d items")

// OpenThoughtsCountOne is the singular combined item count.
var OpenThoughtsCountOne = both("%d 筆", "%d item")

// OpenThoughtsLedeFmt names the literal role and initial stages behind this view.
var OpenThoughtsLedeFmt = both("類型為 %s、處於以下任一初始狀態的筆記：%s；加上你的位置標記，依時間由新到舊。", "Notes of type %s in any of these initial statuses: %s; together with your marked locations, newest first.")

// OpenThoughtsEmptyFmt states which declarations fill the empty shelf.
var OpenThoughtsEmptyFmt = both("尚無標記位置，也沒有類型為 %s、處於以下任一初始狀態的筆記：%s。", "No marked locations or notes of type %s in any of these initial statuses: %s.")

// OpenThoughtsMarksLede describes the shelf when no usable answer role exists.
var OpenThoughtsMarksLede = both("你標記為「還不確定」的位置，依時間由新到舊。", `Locations you marked as "Not sure yet", newest first.`)

// OpenThoughtsMarksEmpty is the marks-only empty state.
var OpenThoughtsMarksEmpty = both("尚無「還不確定」的位置標記。", `No locations marked "Not sure yet".`)

// OpenThoughtsReadFailed does not equate a failed live status read with completion.
var OpenThoughtsReadFailed = both("部分筆記的目前狀態無法讀取，這份清單可能不完整。", "Some notes' current statuses could not be read; this list may be incomplete.")
