package wording

// QuestionControl names the reader's reversible location mark.
var QuestionControl = both("還不確定", "Not sure yet")

// QuestionClearControl makes an already kept mark visible and removable.
var QuestionClearControl = both("移除標記", "Remove mark")

// QuestionSaved confirms that this location was kept.
var QuestionSaved = both("已標記這個位置。", "This location is marked.")

// QuestionCleared confirms that the location mark was removed.
var QuestionCleared = both("已移除這個位置的標記。", "This location's mark was removed.")

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
var QuestionRefused = both("這個位置無法標記。", "That location could not be marked.")

// QuestionPlaceOnly states the deliberately small persistence promise.
var QuestionPlaceOnly = both(
	"只記住這台裝置上的位置與時間，不保存選出的句子；內容變動後，位置可能失效。",
	"Only the location and time are kept on this device, not the chosen sentence. The location may stop working if the content changes.",
)

// QuestionNoteNotFound names a stored mark whose note is no longer readable.
var QuestionNoteNotFound = both("找不到這篇筆記", "Note not found")

// QuestionPlaceNotFound names a stored place absent from its current note.
var QuestionPlaceNotFound = both("找不到這個位置", "Location not found")
