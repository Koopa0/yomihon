package pages

import (
	"strings"

	"github.com/a-h/templ"

	"github.com/koopa0/yomihon/internal/wording"
)

// ListenView is one course as something to be listened to. Its paragraphs
// arrive already rendered: a read-aloud marker survives only into a note's
// HTML, and this page shows the notes' own elements rather than a second
// rendering of them.
type ListenView struct {
	// Title is the page's own heading, which names the course it reads.
	Title string
	// Course is the course's own title, which is also the words on the way
	// back to it.
	Course string
	// PathHref is the course page this one was reached from.
	PathHref string
	// Lessons are the taught lessons that mark at least one paragraph, in
	// course order. A lesson marking none is not an empty heading here.
	Lessons []ListenLesson
}

// ListenLesson is one lesson's marked paragraphs, under a heading that leads
// to the lesson itself.
type ListenLesson struct {
	Title      string
	Href       string
	Paragraphs []string
}

// listenAttrs carries the read-aloud bar's words, plus the sentence saying
// what speech synthesis cannot be asked for. The sentence belongs here and not
// on a note: this is the page a reader arrives at wanting a course played to
// them, and so the page where they would look for a scrubber and find none.
func listenAttrs(v ListenView, lang wording.Lang) templ.Attributes {
	attrs := readAloudAttrs(v.marked(), lang)
	if attrs == nil {
		return nil
	}
	attrs["data-readaloud-limits"] = wording.ReadAloudLimits.In(lang)
	return attrs
}

// marked is the page's own read-aloud HTML, joined so the bar's words are
// decided by the same question the script asks: is there anything here that
// can speak?
func (v ListenView) marked() string {
	var b strings.Builder
	for _, lesson := range v.Lessons {
		for _, paragraph := range lesson.Paragraphs {
			b.WriteString(paragraph)
		}
	}
	return b.String()
}
