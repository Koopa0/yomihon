package wording

// The words of what the reader hands out of yomihon as text: the marks of
// `yomihon marks --export`, and the same shapes when a page copies a mark or a
// group of them. A heading is written without its Markdown marker, which is
// not a matter of language and is put on by whoever writes the heading.

// ExportLede is the one line under the title that says what the links in the
// text are relative to.
var ExportLede = both("連結相對書庫的根目錄。", "Links are relative to the vault's root folder.")

// ExportOpenQuestions heads the questions the reader has not resolved.
var ExportOpenQuestions = both("未解決的疑問", "Open questions")

// ExportThoughts heads the thoughts.
var ExportThoughts = both("想法", "Thoughts")

// ExportHighlights heads the highlights.
var ExportHighlights = both("劃線", "Highlights")

// ExportResolvedQuestions heads the questions the reader has resolved.
var ExportResolvedQuestions = both("已解決的疑問", "Resolved questions")

// CiteSectionSep stands between a note's link and the name of the section a
// citation is from, as a comma does in the language the line is written in.
var CiteSectionSep = both("，", ", ")
