package render

import "github.com/koopa0/yomihon/internal/graph"

// codeSpanRanges retains the presentation policy of line-local quotation.
func codeSpanRanges(text string) [][2]int { return graph.CodeSpanRanges(text) }
