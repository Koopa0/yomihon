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
var ThoughtSectionFmt = both("留想法：%s", "Leave a thought: %s")

// DeclaredBy labels reverse source declarations, distinct from body backlinks.
var DeclaredBy = both("以這篇為來源的筆記", "Notes based on this")

// WholeSource labels a declaration that names no narrower location.
var WholeSource = both("整篇", "Whole note")
