package graph

import (
	"strings"
)

// FirstH1 returns the words of the first level-1 heading in body, read by the
// same line walk every face reads headings with, so a heading inside a fence
// or an authored HTML block is not one. A heading with no words is passed over.
func FirstH1(body string) (string, bool) {
	for _, heading := range Headings(body, LineSkipZones(body)) {
		if heading.Level != 1 {
			continue
		}
		if words := strings.Join(strings.Fields(heading.Text), " "); words != "" {
			return words, true
		}
	}
	return "", false
}

// TitleOf is the title a note is read by: the words of its first level-1
// heading, and when it has none the file name without its extension. A note
// writes no title of its own.
func TitleOf(relPath, body string) string {
	if heading, ok := FirstH1(body); ok {
		return heading
	}
	base := relPath
	if _, after, found := strings.CutLast(relPath, "/"); found {
		base = after
	}
	return strings.TrimSuffix(base, ".md")
}
