package wording

// What the reader's own marks say on a note page: the toolbar over a selection,
// the field a thought is written in, the line a change answers with, the actions
// on one mark and the names a screen reader reads. A mark is one thing in one of
// three states, so the same words serve a highlight turned into a thought and a
// question turned back.
//
// Nothing here says where a mark is kept. The reader only reads; what happened to
// a file is not the reader's business, and a reason a mark was not kept is one of
// the three the reader can act on.

// The toolbar over a selection and the field beside a mark.
var (
	// MarksToolbar names the toolbar for a screen reader.
	MarksToolbar = both("對選取的字", "For the selected text")
	// MarksHighlight and MarksAskAbout are the toolbar's two verbs; the key is
	// drawn beside them.
	MarksHighlight = both("劃線", "Highlight")
	MarksAskAbout  = both("標為疑問", "Mark as a question")
	// MarksCopy is the toolbar's third verb. The control is drawn only where it
	// works.
	MarksCopy = both("複製（含出處）", "Copy with source")

	// MarksWords names the field the reader writes in.
	MarksWords = both("我的字", "My words")
	// MarksDone and MarksCancel close the field.
	MarksDone   = both("完成", "Done")
	MarksCancel = both("取消", "Cancel")
	// MarksComposing is said when the reader presses the finish key while a
	// word is still being chosen.
	MarksComposing = both("還在選字：先按 Enter 選定，再按 ⌘↵", "Still choosing characters: press Enter to confirm, then ⌘↵")
	// MarksEditing labels the field above the words while it is open.
	MarksEditing = both("編輯中", "Editing")
	// MarksSectionNeedsWords is said when a thought on a whole section is
	// emptied: a section has no passage to fall back to.
	MarksSectionNeedsWords = both("整節的想法要有字，留著原來的", "A thought on a section needs words; the earlier words are kept")
)

// The lines a change answers with, said where the reader's eyes are.
var (
	MarksAskedUndo    = both("已標為疑問", "Marked as a question")
	MarksAlreadyAsked = both("這一段已經是疑問", "This passage is already a question")
	MarksAlreadyLit   = both("這一段已經劃過了", "This passage is already highlighted")
	MarksSelectFirst  = both("先選一段原文", "Select a passage first")
	MarksNowThought   = both("已改成想法", "Made a thought")
	MarksNowHighlight = both("已改成劃線", "Made a highlight")
	MarksCancelled    = both("已取消", "Cancelled")
	MarksDeleted      = both("已刪除", "Deleted")
	MarksUndo         = both("復原", "Undo")
	// MarksUndoneFmt says what an undo took back; the argument is one of the
	// nouns below.
	MarksUndoneFmt = both("已復原：%s", "Undone: %s")
	// MarksNothingToUndo and MarksCannotUndo answer an undo with nothing to do.
	MarksNothingToUndo = both("沒有可以復原的", "Nothing to undo")
	MarksCannotUndo    = both("這一步已經不能復原了", "This step can no longer be undone")
	// MarksNotKeptFmt is a field's own line when the words were not kept; the
	// argument is a reason below.
	MarksNotKeptFmt = both("剛寫的字沒有留下來：%s", "The words you just wrote were not kept: %s")
	MarksRetry      = both("再試一次", "Try again")
	MarksKept       = both("留下來了", "Kept")
	// MarksNotDeletedFmt and MarksNotUndoneFmt answer a delete and an undo that
	// did not happen; the argument is a reason.
	MarksNotDeletedFmt = both("沒有刪除：%s", "Not deleted: %s")
	MarksNotUndoneFmt  = both("沒有復原：%s", "Not undone: %s")
	MarksPressEscFirst = both("先按 Esc 取消選取，再刪除", "Press Esc to clear the selection, then delete")
	// MarksAmbiguous answers a selection that cannot be placed.
	MarksAmbiguous = both(
		"這一句在這一篇裡不只一處，分不出是哪一處，沒有留下。",
		"This sentence occurs more than once in this note and which one is meant cannot be told, so nothing was kept.",
	)
	MarksHiddenShow = both("文中留下的隱藏著", "Marks in the text are hidden")
	MarksShow       = both("顯示", "Show")
)

// What an undo took back, the noun that follows "Undone:".
var (
	MarksWhatHighlight   = both("劃線", "highlight")
	MarksWhatQuestion    = both("標為疑問", "mark as a question")
	MarksWhatWords       = both("想法的字", "thought")
	MarksWhatThought     = both("改成想法", "make it a thought")
	MarksWhatToHighlight = both("改成劃線", "make it a highlight")
	MarksWhatResolve     = both("標為已解決", "mark as resolved")
	MarksWhatDelete      = both("刪除", "delete")
)

// The reasons a mark was not kept, which are all the reader is told.
var (
	MarksReasonTooLong  = both("這一則太長，沒有留下。", "This mark is too long and was not kept.")
	MarksReasonLimit    = both("標記已達上限（5,000 處）", "Marks have reached the limit (5,000)")
	MarksReasonNotWrite = both("書庫現在不能寫入", "The vault cannot be written to right now")
)

// The actions on one mark.
var (
	MarksWriteThought = both("寫想法", "Write a thought")
	MarksEdit         = both("編輯", "Edit")
	MarksResolve      = both("標為已解決", "Mark as resolved")
	MarksReopen       = both("再標為疑問", "Mark as a question again")
	MarksToThought    = both("改成想法", "Make it a thought")
	MarksToHighlight  = both("改成劃線", "Make it a highlight")
	MarksDelete       = both("刪除", "Delete")
	MarksMore         = both("更多", "More")
	MarksActions      = both("這一則的動作", "Actions for this mark")
)

// How a mark is named in the margin and to a screen reader.
var (
	MarksResolvedLead = both("已解決 ·", "Resolved ·")
	MarksSectionLead  = both("整節 ·", "Whole section ·")
	MarksChanged      = both("原文已變動", "The passage has changed")
	// MarksOfThePassageFmt wraps the first words of a passage that a question
	// with no words of its own stands for.
	MarksOfThePassageFmt = both("「%s」", "\"%s\"")
	// The accessible names, which begin with the state. The argument is the
	// first words of the thought, the question or the passage.
	MarksNameHighlightFmt = both("劃線：%s", "Highlight: %s")
	MarksNameThoughtFmt   = both("想法：%s", "Thought: %s")
	MarksNameQuestionFmt  = both("疑問：%s", "Question: %s")
	MarksNameResolvedFmt  = both("已解決的疑問：%s", "Resolved question: %s")
)

// The keys the help panel adds on a note page that can keep marks. The key is
// drawn beside each; these say what it does.
var (
	KeysHighlight = both("劃線，旁邊打開欄位，寫了字就是想法。焦點在一則上：寫它的字", "Highlight, with a field beside it; what you type is a thought. On a mark: write its words")
	KeysQuestion  = both("標為疑問，不打開欄位。焦點在一則上：把它轉成疑問", "Mark as a question, without a field. On a mark: make it a question")
	KeysEnter     = both("焦點在一則上：寫想法或編輯", "On a mark: write or edit")
	KeysDone      = both("欄位裡：完成", "In a field: done")
	KeysEscape    = both("欄位裡：收起，字留著。其他：關掉最上層", "In a field: close it, the words stay. Otherwise: close the top layer")
	KeysDelete    = both("焦點在一則上：刪除（可以復原）", "On a mark: delete (can be undone)")
	KeysUndo      = both("復原上一步（欄位裡是文字的復原）", "Undo the last step (in a field, undo typing)")
	KeysResolve   = both("焦點在疑問上：標為已解決，或再標為疑問。有讀音的頁：切換讀音", "On a question: mark as resolved, or as a question again. On a page with readings: show or hide them")
	KeysNeighbour = both("上一則／下一則留下的", "Previous / next mark")
	KeysShowMarks = both("顯示或隱藏文中留下的", "Show or hide marks in the text")
)
