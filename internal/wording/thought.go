package wording

// ThoughtDoor names the link that starts a note of the reader's own, based on this note.
var ThoughtDoor = both("留下自己的想法", "Leave a thought")

// ThoughtMarkdown labels the copyable text.
var ThoughtMarkdown = both("Markdown", "Markdown")

// ThoughtExternal states who saves and completes the note.
var ThoughtExternal = both("複製到你的編輯器，再填上自己的內容與欄位。yomihon 不會建立筆記。", "Copy this into your editor and add your words and fields. Yomihon does not create the note.")

// ThoughtCopy labels the clipboard enhancement.
var ThoughtCopy = both("複製 Markdown", "Copy Markdown")

// ThoughtCopied confirms a clipboard write.
var ThoughtCopied = both("已複製。", "Copied.")

// ThoughtCopyFailed preserves the manual copying path.
var ThoughtCopyFailed = both("請選取上方文字，手動複製。", "Select the text above and copy it manually.")

// ThoughtObsidian names the optional external editor action.
var ThoughtObsidian = both("在 Obsidian 建立筆記", "Create in Obsidian")

// ThoughtSectionFmt names, in the contents list, the link that starts a note of
// the reader's own based on one section; the argument is that heading's text.
var ThoughtSectionFmt = both("留下自己的想法：%s", "Leave a thought: %s")

// ThoughtNoteNameFmt is the file name a reader's thought starts with; the
// arguments are the source note's title and the minute the thought was begun.
var ThoughtNoteNameFmt = both("%s的想法 %s", "Thought on %s %s")

// DeclaredBy labels reverse source declarations, distinct from body backlinks.
var DeclaredBy = both("以這篇為來源的筆記", "Notes based on this")

// WholeSource labels a declaration that names no narrower location.
var WholeSource = both("整篇", "Whole note")

// ThoughtLabel names a thought of the reader's own wherever one is listed beside
// the text it answers: its first sentence stands after it.
var ThoughtLabel = both("想法", "Thought")

// PredictHint is said directly above a closed question fold, whether or not the
// reader answers: it names the one moment a guess is honest, which is before the
// answer is open. Nothing here asks for a reply.
var PredictHint = both("看答案前，可以先試著回答", "You can try answering before you open the answer")

// PredictThought is the link on that line that starts a thought of the reader's
// own about the section the question is in.
var PredictThought = both("寫成想法", "Write it as a thought")

// ThoughtBeforeAnswer is the line a thought draft opens with when its section
// holds a closed question fold, so the guess is written under it before the
// answer is read.
var ThoughtBeforeAnswer = both("看答案前，我的想法：", "My thoughts before opening the answer:")
