package render

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/koopa0/yomihon/internal/graph"
)

// sentenceLimit is how long a stretch with no sentence end is allowed to run
// before it is cut, so a paragraph that is one long clause is not lent whole to
// a listing.
const sentenceLimit = 160

// readings are the glosses a hand-written ruby carries over its base text.
// A listing shows the sentence without them: the reader who wants a reading
// opens the page, where the control for it is.
var readings = regexp.MustCompile(`(?is)<rt\b[^>]*>.*?</rt\s*>|<rp\b[^>]*>.*?</rp\s*>`)

// SectionSentence is the first sentence of the prose a section opens with, read
// from the body as it stands now. The section is the first heading whose words
// are heading, through to the next heading of any level, so a subsection's
// prose is not lent to the section above it. It is empty when no heading reads
// that way, when the section opens on nothing but a heading, or when its first
// block holds no words.
func SectionSentence(body, heading string) string {
	slice, matches := headingSlice(body, heading)
	if matches == 0 {
		return ""
	}
	stripped, _ := stripBody(slice)
	lines := strings.Split(stripped.text, "\n")
	headings := graph.Headings(stripped.text, graph.LineSkipZones(stripped.text))
	if len(headings) == 0 {
		return ""
	}
	return firstSentence(prefaceUnder(lines, headings))
}

// OpeningSentence is the first sentence of what a note's author wrote ahead of
// whatever it goes on to list, read as Opening reads it.
func OpeningSentence(body string) string {
	return firstSentence(Opening(body))
}

// firstSentence is the first sentence of the first block of markdown, as the
// words a reader would see: markup is read through the same walk the search
// text is, a ruby reading is left off, and a sentence ends where the writing
// system ends it.
func firstSentence(markdown string) string {
	plain := PlainText(readings.ReplaceAllString(markdown, ""))
	for block := range strings.SplitSeq(plain, "\n") {
		if block = strings.TrimSpace(block); block != "" {
			return cutSentence(block)
		}
	}
	return ""
}

// cutSentence ends text at its first sentence end: a full-width stop, or an
// ASCII one followed by a space or nothing, so a dotted name or a decimal does
// not end it. Closing quotes and brackets that follow the stop stay with it.
func cutSentence(text string) string {
	runes := []rune(text)
	for i, r := range runes {
		if !isSentenceEnd(runes, i, r) {
			continue
		}
		end := i + 1
		for end < len(runes) && strings.ContainsRune("」』）)”’\"'", runes[end]) {
			end++
		}
		return string(runes[:end])
	}
	if utf8.RuneCountInString(text) > sentenceLimit {
		return strings.TrimSpace(string(runes[:sentenceLimit])) + "…"
	}
	return text
}

func isSentenceEnd(runes []rune, i int, r rune) bool {
	switch r {
	case '。', '！', '？', '．':
		return true
	case '.', '!', '?':
		return i+1 == len(runes) || runes[i+1] == ' ' || runes[i+1] == '\t'
	}
	return false
}
