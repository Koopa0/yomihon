package render

import "github.com/koopa0/yomihon/internal/graph"

// codeSpanAt uses the graph-owned line dialect without another body parser.
func codeSpanAt(text string, i int) (end int, isSpan bool) {
	return graph.CodeSpanAt(text, i)
}

// codeSpanRanges retains the presentation policy of line-local quotation.
func codeSpanRanges(text string) [][2]int { return graph.CodeSpanRanges(text) }
