package render

// This file holds the package's one reading of an inline code span. Two passes
// over a source line depend on it: the %% comment strip, which must not take a
// percent sign an author is displaying for a marker, and the wikilink pass,
// which must not convert syntax an author is quoting. A second reading of the
// same rule would agree by maintenance accident and disagree by default, and
// the two disagreeing is what put rendered markup inside a reader's code.

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

// codeSpanAt reads the run of backticks starting at i, where text[i] is one.
// It answers with the offset just past whatever that run accounts for, and
// whether the run found the closing run of its own length that makes the two a
// span.
//
// A run that finds no closer accounts for the backticks themselves and nothing
// more, so a reader carries on from immediately after them. That is what the
// CommonMark spec says — a backtick string left unpaired is literal text, and
// it does not stop the strings after it from pairing with each other — and so
// it is what goldmark draws on the page. A scan that gave up at an unpaired
// run would call the rest of the line prose while the reader is looking at
// code.
//
// The caller finds i by looking for a backtick, which is what lets every
// answer advance past i: a run is at least one character wide, and a span is
// wider still.
func codeSpanAt(text string, i int) (end int, isSpan bool) {
	width := backtickRun(text, i)
	if end := closingRun(text, i+width, width); end >= 0 {
		return end, true
	}
	return i + width, false
}

// codeSpanRanges reports the byte ranges of the code spans in text, in the
// order they occur. What lies inside one is the author showing something and
// is left as written; what lies outside is theirs to mean, and the dialect
// passes read it.
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
