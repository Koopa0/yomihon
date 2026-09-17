package wording

// MarkSetControl names the control a reader presses to keep the place they
// stopped at. It is a verb about what the press does, not a noun for the thing
// it leaves behind: a reader looking for a way to stop is looking for an
// action.
var MarkSetControl = both("留下待續位置", "Leave off here")

// MarkSaved is what the page says once the place has been kept. The control
// stays where it is and this sentence appears beside it, because a control
// that renamed itself would leave nothing to press a second time.
var MarkSaved = both("已留下待續位置。", "Your place is kept.")

// MarkPerDevice is the sentence that says where the place is kept. A mark
// lives in a file on this machine, so one left on a laptop is not on a phone
// and nothing syncs. It is said in words rather than left for a reader to
// discover on the second machine.
var MarkPerDevice = both(
	"待續位置只留在這台裝置上，不會同步。",
	"Your place is kept on this device only, and nothing syncs.",
)

// MarkFormUnreadable is what the mark route answers a submission it cannot
// parse at all.
var MarkFormUnreadable = both("表單無法解讀。", "The form could not be read.")

// MarkRefused is what it answers a submission whose shape it will not store.
// The reader is not told which field, because they wrote none of them: the
// page did, and a reader cannot act on the difference.
var MarkRefused = both("這個待續位置無法保留。", "That place could not be kept.")

// MarkNotStored is what it answers when the shape was fine and the file could
// not be written. Reading is unaffected either way, which the sentence says so
// the reader does not go looking for what else broke.
var MarkNotStored = both(
	"待續位置沒有寫入。閱讀不受影響。",
	"Your place was not written. Reading is unaffected.",
)

// ContinueReading titles the one row on the desk that returns a reader to
// where they left off.
var ContinueReading = both("繼續閱讀", "Continue reading")

// MarkNoteChanged is what that row says when the note has been edited since
// the place was kept. The row still goes there — it is the reader's own best
// pointer, and nothing else knows better where they were — and this sentence
// is why the sentence they stopped at may have moved.
var MarkNoteChanged = both(
	"這篇筆記在你留下位置之後改過了。",
	"This note has changed since you left off.",
)

// MarkNoteGone is what it says instead when the note is no longer in the
// vault. Nothing is cleared: the reader did not ask for that, and the next
// place they keep replaces this one.
var MarkNoteGone = both(
	"你留下位置的那篇筆記已經不在了。",
	"The note you left off in is no longer here.",
)
