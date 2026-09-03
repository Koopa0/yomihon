package render

// The package's one reading of an inline code span. Two passes over a source line
// depend on it: the %% comment strip, which must not take a displayed percent
// sign for a marker, and the wikilink pass, which must not convert quoted syntax.

// backtickRun reports how many backticks start at i, zero when none do.
func backtickRun(text string, i int) int {
	n := 0
	for i+n < len(text) && text[i+n] == '`' {
		n++
	}
	return n
}

// closingRun reports the end offset of the next backtick run of exactly width
// at or after i, or -1 when the text has none. A run of a different width is
// ordinary content inside the span and is stepped over.
func closingRun(text string, i, width int) int {
	for j := i; j < len(text); {
		n := backtickRun(text, j)
		switch n {
		case 0:
			j++
		case width:
			return j + n
		default:
			j += n
		}
	}
	return -1
}

// codeSpanAt reads the run of backticks starting at i, where text[i] is one, and
// answers with the offset just past whatever that run accounts for and whether it
// found the closing run of its own length. An unpaired run accounts for the
// backticks alone, which is what the CommonMark spec says and what goldmark draws:
// it does not stop later runs from pairing with each other.
func codeSpanAt(text string, i int) (end int, isSpan bool) {
	width := backtickRun(text, i)
	if end := closingRun(text, i+width, width); end >= 0 {
		return end, true
	}
	return i + width, false
}

// codeSpanRanges reports the byte ranges of the code spans in text, in order.
// What lies inside one is the author showing something and is left as written.
func codeSpanRanges(text string) [][2]int {
	var spans [][2]int
	for i := 0; i < len(text); {
		if text[i] != '`' {
			i++
			continue
		}
		end, isSpan := codeSpanAt(text, i)
		if isSpan {
			spans = append(spans, [2]int{i, end})
		}
		i = end
	}
	return spans
}
