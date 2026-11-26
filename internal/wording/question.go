package wording

// QuestionControl names the reader's reversible location mark.
var QuestionControl = both("標為疑問", "Mark as question")

// QuestionKind labels a row of the My marks page that is a question, the noun
// where QuestionControl is the verb.
var QuestionKind = both("疑問", "Question")

// QuestionClearControl makes an already kept mark visible and removable.
var QuestionClearControl = both("移除標記", "Remove mark")

// QuestionSaved confirms that this location was kept.
var QuestionSaved = both("已標為疑問。", "Marked as a question.")

// QuestionCleared confirms that the location mark was removed.
var QuestionCleared = both("已移除這則疑問。", "The question was removed.")

// QuestionNotStored reports a failed change without suggesting reading failed.
var QuestionNotStored = both(
	"標記沒有更新。閱讀不受影響。",
	"Your marks were not updated. Reading is unaffected.",
)

// QuestionUnavailable keeps a storage failure distinct from having no marks.
var QuestionUnavailable = both(
	"目前無法讀取標記，原有檔案未更動。",
	"Marks cannot be read right now. The existing file was left unchanged.",
)

// QuestionRefused reports a malformed location without echoing its fields.
var QuestionRefused = both("這個位置無法標為疑問。", "That location could not be marked as a question.")

// NoteNotFound says a note is not there to open: a stored mark whose note is no
// longer readable, or a request that names one the vault does not let a reader
// open.
var NoteNotFound = both("找不到這篇筆記", "Note not found")

// QuestionPlaceNotFound names a stored place absent from its current note.
var QuestionPlaceNotFound = both("找不到這則疑問的位置", "Question location not found")
