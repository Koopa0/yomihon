package wording

// The diagnostic panel: what the renderer found and what it did about it. Each
// says both, because a reader who sees only the fault cannot tell whether the
// page in front of them is missing something.

// The labels on the diagnostic kinds the renderer can report. Each names the
// fault in the fewest words that still distinguish it from its neighbours.
var (
	MarkdownOutsideFmt        = both("「%s」超出書庫範圍，已留下連結文字", "\"%s\" leaves the vault; the link text remains")
	DiagMarkdownMissingNote   = both("這個 Markdown 連結沒有可用的書庫目標；已留下文字並標示原因。", "This Markdown link has no usable vault target; its text remains with an explanation.")
	DiagImageMissing          = both("圖片的檔案不在書庫裡", "Picture whose file the vault does not hold")
	DiagLinkNoTarget          = both("連結沒有目標", "Link with no target")
	DiagLinkManyTargets       = both("連結有多個目標", "Link with several targets")
	DiagUnknownCallout        = both("沒見過的提示框類型", "Unrecognised callout type")
	DiagRiskyFence            = both("程式碼區塊裡的筆記語法", "Note syntax inside a code block")
	DiagEmbedFragmentGone     = both("找不到嵌入指定的段落", "The embedded section was not found")
	DiagEmbedFragmentRepeated = both("嵌入指定的段落在該筆記裡不只一個", "The embedded section name belongs to more than one section")
	DiagEmbedNotExpanded      = both("摘錄裡的嵌入改以連結呈現", "An embed inside an excerpt is shown as a link")
	DiagRenderFailed          = both("內文無法轉譯，已改顯示原始文字", "The body could not be rendered; its source text is shown instead")
	DiagHighlightFailed       = both("程式碼著色沒有完成", "Code highlighting did not finish")
	DiagLinkBlockGone         = both("找不到連結指定的區塊", "The linked block was not found")
	DiagLinkSectionGone       = both("找不到連結指定的小節", "The linked section was not found")
	DiagCommentUnclosed       = both("沒有配對的註解記號", "Unpaired comment marker")
)

// DiagCommentContainerUnclosed names silence confined to one Markdown container.
var DiagCommentContainerUnclosed = both("所在 Markdown 區塊沒有配對的註解記號", "Unpaired comment marker in a Markdown container")

// What each of those faults means, and what the page did about it. A reader
// who sees only the fault cannot tell whether the page in front of them is
// missing something.
var (
	DiagImageMissingNote = both(
		"這篇要顯示的圖片，書庫裡沒有那個檔案。圖片留在原處，旁邊標出來；yomihon 只陳述，不修復。",
		"This note shows a picture and the vault holds no file at that path. The picture is left where the author put it and marked in place; yomihon reports and never repairs.")
	DiagUnwrittenNote  = both("這個 wikilink 或嵌入的目標尚未建立。", "This wikilink or embed points at something that has not been written.")
	DiagAmbiguousNote  = both("wikilink 或嵌入目標有歧義。", "This wikilink or embed has more than one target.")
	DiagCalloutNote    = both("未知的 callout 類型；已以中性的提示框顯示。", "Unrecognised callout type; shown as a neutral callout.")
	DiagFenceNote      = both("程式碼區塊含類似筆記語法的文字；已保持原樣。", "A code block holds text that looks like note syntax; it is left as written.")
	DiagEmbedNote      = both("找不到嵌入指定的段落或區塊；沒有顯示那篇筆記的任何內容，原處只留下告示。", "The embedded section or block was not found; nothing of that note is shown, and a notice stands where the excerpt would.")
	DiagEmbedManyNote  = both("嵌入指定的名字在來源筆記裡對應到不只一個段落；顯示的是第一個，其餘未顯示。", "The embedded name matches more than one section in the source note; the first is shown and the others are not.")
	DiagEmbedDepthNote = both("嵌入只展開一層，所以摘錄裡本身寫成嵌入的引用改以連結呈現。", "Embeds expand one level, so a citation written as an embed inside an excerpt is shown as a link.")
	DiagBlockNote      = both("目標筆記沒有這個區塊；連結已改為指向整篇筆記。", "The target note has no such block; the link now points at the whole note.")
	DiagSectionNote    = both(
		"目標筆記沒有這個小節；連結位址照原樣保留，點下去會落在筆記最上方。",
		"The target note has no such section; the address is kept as written, and following it lands at the top of the note.",
	)
	DiagCommentNote         = both("這個註解記號沒有找到配對，它後面的內容全被藏了起來。", "This comment marker has no partner, and everything after it is hidden.")
	DiagRenderNote          = both("Markdown 轉譯失敗；已顯示原始內容。", "Markdown rendering failed; the source is shown as written.")
	DiagHighlightFailedNote = both(
		"這個程式碼區塊著色時沒有完成；已改以未著色、跳脫後的原文顯示。",
		"Highlighting this code block did not finish; it is shown as plain escaped text.",
	)
	DiagUnknownNote = both("內容轉譯產生未識別的診斷。", "Rendering produced a diagnostic nothing here recognises.")
)

// DiagCommentContainerNote explains that later containers remain readable.
var DiagCommentContainerNote = both("這個註解記號沒有找到配對，所在 Markdown 區塊剩下的內容被藏了起來。", "This comment marker has no partner, and the rest of its Markdown container is hidden.")

// The five reasons a figure is not shown: a label for the findings rail, and
// the note said both there and where the figure would have stood. Each note
// says what was found, why the whole figure is withheld, and what the author
// can write instead.
var (
	DiagFigureTooLarge     = both("圖解檔超過大小上限", "Figure over the size limit")
	DiagFigureTooLargeNote = both(
		"這個圖解檔的大小是 %s，超過 %s 的上限，所以沒有顯示。把檔案縮小，或拆成幾個圖解。yomihon 只陳述，不修復。",
		"This figure's file is %s, over the %s limit, so it is not shown. Make the file smaller, or split it into several figures. yomihon reports and never repairs.",
	)
	DiagFigureEncoding     = both("圖解檔不是 UTF-8", "Figure not saved as UTF-8")
	DiagFigureEncodingNote = both(
		"這個圖解檔不是有效的 UTF-8，或以 UTF-16 的位元組順序記號（BOM）開頭；遇到這個記號，瀏覽器會不管檔案宣告的編碼，所以沒有顯示。請用編輯器以 UTF-8（不含 BOM）重新存檔。yomihon 只陳述，不修復。",
		"This figure's file is not valid UTF-8, or it starts with a UTF-16 byte-order mark, which makes a browser ignore the encoding the file declares; it is not shown. Save it again as UTF-8 without a byte-order mark. yomihon reports and never repairs.",
	)
	DiagFigureNestedFrame     = both("圖解裡有嵌入的框架", "Figure holds a nested frame")
	DiagFigureNestedFrameNote = both(
		"這個圖解含有 %s。這類元素在圖解打開時就可能連到這台機器以外，所以整個圖解沒有顯示。請把內容直接寫成圖解檔裡的 HTML 與 SVG。yomihon 只陳述，不修復。",
		"This figure contains %s. An element like that can reach outside this machine as soon as the figure opens, so the figure is not shown. Write the content as HTML and SVG inside the figure's own file. yomihon reports and never repairs.",
	)
	DiagFigureLoadsElsewhere     = both("圖解會向外連線或自行跳轉", "Figure connects out or moves on its own")
	DiagFigureLoadsElsewhereNote = both(
		"這個圖解含有 %s。這類元素會讓瀏覽器預先連線、改變網址的解析基準，或自動跳到別的頁面，所以整個圖解沒有顯示。樣式請寫在檔案內的 <style>；要指向別處，用一般連結即可。yomihon 只陳述，不修復。",
		"This figure contains %s. An element like that makes the browser connect ahead, changes what addresses resolve against, or moves to another page on its own, so the figure is not shown. Put styles in a <style> element inside the file; to point somewhere else, an ordinary link is fine. yomihon reports and never repairs.",
	)
	DiagFigureScriptOutside     = both("圖解的腳本不在檔案內的 <script> 裡", "Figure script outside an inline <script>")
	DiagFigureScriptOutsideNote = both(
		"這個圖解有 %s。只有直接寫在檔案內 <script> 元素裡的腳本，才能依確切內容核可；外部腳本檔、事件屬性和 javascript 網址都無法釘住，所以整個圖解沒有顯示。請把腳本移進檔案內的 <script>，用 addEventListener 綁定事件。yomihon 只陳述，不修復。",
		"This figure has %s. Only script written inside a <script> element in the file can be approved by its exact contents; a script file, an event attribute or a javascript: address cannot be pinned, so the figure is not shown. Move the script into a <script> element in the file and attach events with addEventListener. yomihon reports and never repairs.",
	)
)
