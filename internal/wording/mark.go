package wording

// BookmarkSetControl names the control a reader presses to keep the place they
// stopped at. It is a verb about what the press does, not a noun for the thing
// it leaves behind: a reader looking for a way to stop is looking for an
// action.
var BookmarkSetControl = both("加書籤", "Add bookmark")

// BookmarkSaved is what the page says once the place has been kept. The control
// stays where it is and this sentence appears beside it, because a control
// that renamed itself would leave nothing to press a second time.
var BookmarkSaved = both("已加書籤。", "Bookmark added.")

// BookmarkPerDevice is the sentence that says where the place is kept. A mark
// lives in a file on this machine, so one left on a laptop is not on a phone
// and nothing syncs. It is said in words rather than left for a reader to
// discover on the second machine.
var BookmarkPerDevice = both(
	"書籤只留在這台裝置上，不會同步。",
	"Your bookmark stays on this device only, and nothing syncs.",
)

// MarkFormUnreadable is what the mark route answers a submission it cannot
// parse at all.
var MarkFormUnreadable = both("表單無法解讀。", "The form could not be read.")

// BookmarkRefused is what it answers a submission whose shape it will not store.
// The reader is not told which field, because they wrote none of them: the
// page did, and a reader cannot act on the difference.
var BookmarkRefused = both("這個書籤無法保留。", "That bookmark could not be kept.")

// BookmarkNotStored is what it answers when the shape was fine and the file could
// not be written. Reading is unaffected either way, which the sentence says so
// the reader does not go looking for what else broke.
var BookmarkNotStored = both(
	"書籤沒有寫入。閱讀不受影響。",
	"Your bookmark was not written. Reading is unaffected.",
)

// MarkNotKept is what a route answers a highlight, a question or a thought it
// will not keep, or will not put back. It says no more than the bookmark's does:
// the page wrote every field, and a reader cannot act on which.
var MarkNotKept = both("這則標記無法保留。", "That mark could not be kept.")

// MarkNotFound is what a route answers a mark that is not kept: one that was
// removed, or an id that never named one.
var MarkNotFound = both("找不到這則標記。", "That mark is not here.")

// RequestUnreadable is what a route answers a request it cannot read as the
// one thing it takes.
var RequestUnreadable = both("請求無法解讀。", "The request could not be read.")

// BookmarkNotFound is what a route answers a request to clear a bookmark when
// the course or the vault has none.
var BookmarkNotFound = both("這裡沒有書籤。", "There is no bookmark here.")

// BookmarkMovedElsewhere is what a route answers a request to put a bookmark
// back when another page has changed it since: the later change is kept and
// the undo is not applied.
var BookmarkMovedElsewhere = both("書籤已在別處改過，沒有復原。", "The bookmark was changed elsewhere, so it was not put back.")

// FinishedRefused is what a route answers when it will not say a note is
// finished: a course that does not walk it, or more finished notes than the
// file holds. The page wrote every field, and a reader cannot act on which.
var FinishedRefused = both("這篇筆記無法標成讀完。", "That note could not be marked finished.")

// FinishedUndoRefused is what a route answers when it will not say a note is
// not finished: a course that does not walk it.
var FinishedUndoRefused = both("這篇筆記無法取消讀完。", "That note could not be marked unfinished.")

// FinishedNotStored is what it answers when the shape was fine and the file
// could not be written. Reading is unaffected, and the sentence says so.
var FinishedNotStored = both(
	"讀完沒有記下來。閱讀不受影響。",
	"Finished was not saved. Reading is unaffected.",
)

// BookmarkKeptHere is said, to a reader who cannot see the bookmark drawn beside
// it, of the one unit or note the bookmark was added in. It is a noun phrase that
// follows the title it belongs to in a link, so it names the place and claims
// nothing about how much of the course lies behind it.
var BookmarkKeptHere = both("你加書籤的地方", "Where your bookmark is")

// ContinueReading titles the one row on the desk that returns a reader to
// where they left off.
var ContinueReading = both("繼續閱讀", "Continue reading")

// BookmarkNoteChanged is what that row says when the note has been edited since
// the place was kept. The row still goes there — it is the reader's own best
// pointer, and nothing else knows better where they were — and this sentence
// is why the sentence they stopped at may have moved.
var BookmarkNoteChanged = both(
	"這篇筆記在你加書籤之後改過了。",
	"This note has changed since you added your bookmark.",
)

// BookmarkNoteGone is what it says instead when the note is no longer in the
// vault. Nothing is cleared: the reader did not ask for that, and the next
// place they keep replaces this one.
var BookmarkNoteGone = both(
	"你加書籤的那篇筆記已經不在了。",
	"The note you bookmarked is no longer here.",
)
