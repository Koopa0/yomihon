package render

import "strings"

// A block that needs more room than the text's width names, in data-wide,
// every reading measure it does not fit, and the stylesheet sets a block that
// names the measure the page is reading at to the wide measure instead. These
// are the measures: the article's --measure-read at each of the three text
// sizes, 660, 700 and 740px, and the 31 reading ems an article tagged as
// neither Chinese, Japanese nor Korean takes instead, at the reading sizes of
// 17, 19 and 21px. Each is read at one text size, which says how large the
// block's own type is there. The order is the attribute's.
var readingMeasures = []readingMeasure{
	{token: "latin-xl", width: 31 * 21, size: textXL},
	{token: "latin-l", width: 31 * 19, size: textL},
	{token: "latin-m", width: 31 * 17, size: textM},
	{token: "xl", width: 740, size: textXL},
	{token: "l", width: 700, size: textL},
	{token: "m", width: 660, size: textM},
}

// readingMeasure is one measure a block can stand at: its token in data-wide,
// its width in pixels, and the text size it is read at.
type readingMeasure struct {
	token string
	width int
	size  textSize
}

// textSize is one of the reader's three text sizes. Each picks a pixel size
// from the editorial scale (--fs-ed-*), indexed by it.
type textSize int

const (
	textM textSize = iota
	textL
	textXL
)

// wideFor names, in the order of readingMeasures, every measure misses says the
// block does not fit, or nothing when it fits all of them.
func wideFor(misses func(readingMeasure) bool) string {
	var tokens []string
	for _, m := range readingMeasures {
		if misses(m) {
			tokens = append(tokens, m.token)
		}
	}
	return strings.Join(tokens, " ")
}

// wideAttribute is the attribute a block that names any measure carries, with
// its leading space, or nothing.
func wideAttribute(wide string) string {
	if wide == "" {
		return ""
	}
	return ` data-wide="` + wide + `"`
}
