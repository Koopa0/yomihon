package wording

// The diagnostic panel: what the renderer found and what it did about it. Each
// says both, because a reader who sees only the fault cannot tell whether the
// page in front of them is missing something.

// StatusOutsideEnumChip marks a status the schema does not declare.
var StatusOutsideEnumChip = both("不在 schema 允許清單中", "Not in the schema's list")

var (
	DiagUnwrittenNote = both("這個 wikilink 或嵌入的目標尚未建立。", "This wikilink or embed points at something that has not been written.")
	DiagAmbiguousNote = both("wikilink 或嵌入目標有歧義。", "This wikilink or embed has more than one target.")
	DiagCalloutNote   = both("未知的 callout 類型；已改以一般引用區塊顯示。", "Unrecognised callout type; shown as an ordinary quote block.")
	DiagFenceNote     = both("程式碼區塊含類似筆記語法的文字；已保持原樣。", "A code block holds text that looks like note syntax; it is left as written.")
	DiagEmbedNote     = both("找不到嵌入指定的段落或區塊；已改顯示整篇筆記。", "The embedded section or block was not found; the whole note is shown instead.")
	DiagBlockNote     = both("目標筆記沒有這個區塊；連結已改為指向整篇筆記。", "The target note has no such block; the link now points at the whole note.")
	DiagSectionNote   = both(
		"目標筆記沒有這個小節；連結位址照原樣保留，點下去會落在筆記最上方。",
		"The target note has no such section; the address is kept as written, and following it lands at the top of the note.",
	)
	DiagCommentNote = both("這個註解記號沒有找到配對，它後面的內容全被藏了起來。", "This comment marker has no partner, and everything after it is hidden.")
	DiagRenderNote  = both("Markdown 轉譯失敗；已顯示原始內容。", "Markdown rendering failed; the source is shown as written.")
	DiagUnknownNote = both("內容轉譯產生未識別的診斷。", "Rendering produced a diagnostic nothing here recognises.")
)
