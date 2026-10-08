package graph

// CodeSpanAt preserves the dialect's line-local quotation policy. It is used
// by presentation rewrites, not as a substitute for canonical body code facts.
func CodeSpanAt(value string, offset int) (end int, paired bool) {
	width := bodyBacktickRun(value, offset)
	for at := offset + width; at < len(value); {
		n := bodyBacktickRun(value, at)
		switch n {
		case 0:
			at++
		case width:
			return at + n, true
		default:
			at += n
		}
	}
	return offset + width, false
}

func bodyBacktickRun(value string, offset int) int {
	n := 0
	for offset+n < len(value) && value[offset+n] == '`' {
		n++
	}
	return n
}

// CodeSpanRanges returns the paired spans of the line-local dialect policy.
// Canonical multiline and container facts belong to ReadBody instead.
func CodeSpanRanges(value string) [][2]int {
	var spans [][2]int
	for at := 0; at < len(value); {
		if value[at] != '`' {
			at++
			continue
		}
		end, paired := CodeSpanAt(value, at)
		if paired {
			spans = append(spans, [2]int{at, end})
		}
		at = end
	}
	return spans
}
