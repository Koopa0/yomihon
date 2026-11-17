package wording

// OpenThoughtsTitle names a return to the reader's own unfinished material.
var OpenThoughtsTitle = both("我的標記", "My marks")

// OpenThoughtsLede says what the shelf holds where the rules declare the
// type a reader's thoughts are filed under: the questions, and the
// thoughts that still declare draft: true.
var OpenThoughtsLede = both("你標為疑問的位置，以及還沒寫完的想法。", `Your questions, and thoughts you have not finished.`)

// OpenThoughtsMarksLede describes the shelf when no usable answer role exists.
// It says what the shelf holds and nothing of its order, which the reader
// chooses.
var OpenThoughtsMarksLede = both("你標為疑問的位置。", `Your questions.`)

// OpenThoughtKind labels a row that is a note of the reader's own, where a
// question is labelled "Question".
var OpenThoughtKind = both("想法", "Thought")

// The My marks shelf can be ordered three ways, and says it is
// being asked which.
var (
	OpenOrderLabel  = both("排序", "Order")
	OpenOrderNewest = both("最新在前", "Newest first")
	OpenOrderSource = both("依筆記", "By note")
	OpenOrderOldest = both("最早在前", "Oldest first")
)

// OpenThoughtsMarksEmpty is the marks-only empty state.
var OpenThoughtsMarksEmpty = both("尚無疑問。", `No questions yet.`)
