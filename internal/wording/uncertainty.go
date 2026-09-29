package wording

// UncertaintyControl names the reader's reversible location mark.
var UncertaintyControl = both("還不確定", "Not sure yet")

// UncertaintyClearControl makes an already kept mark visible and removable.
var UncertaintyClearControl = both("移除標記", "Remove mark")

// UncertaintySaved confirms that this location was kept.
var UncertaintySaved = both("已標記這個位置。", "This location is marked.")

// UncertaintyCleared confirms that the location mark was removed.
var UncertaintyCleared = both("已移除這個位置的標記。", "This location's mark was removed.")

// UncertaintyNotStored reports a failed change without suggesting reading failed.
var UncertaintyNotStored = both(
	"標記沒有更新。閱讀不受影響。",
	"Your marks were not updated. Reading is unaffected.",
)

// UncertaintyUnavailable keeps a storage failure distinct from having no marks.
var UncertaintyUnavailable = both(
	"目前無法讀取標記，原有檔案未更動。",
	"Marks cannot be read right now. The existing file was left unchanged.",
)

// UncertaintyRefused reports a malformed location without echoing its fields.
var UncertaintyRefused = both("這個位置無法標記。", "That location could not be marked.")

// UncertaintyPlaceOnly states the deliberately small persistence promise.
var UncertaintyPlaceOnly = both(
	"只記住這台裝置上的位置與時間，不保存選出的句子；內容變動後，位置可能失效。",
	"Only the location and time are kept on this device, not the chosen sentence. The location may stop working if the content changes.",
)
