package pages

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

// The reader's own marks as a reading page draws them: the passage wrapped in a
// <mark>, and beside its paragraph the note that holds the words. The server
// draws both on arrival and again for the answer to every change, so a page that
// has just made a mark and a page that arrives with it are the same page.

// MarkNote is one mark of the reader's as the page draws it.
type MarkNote struct {
	ID string
	// Kind is "highlight", "thought" or "question".
	Kind string
	// Words are the reader's own words, which a highlight has none of.
	Words string
	// Quote is the passage the mark stands on, as it was when the mark was made.
	Quote string
	// Resolved is a question the reader has answered.
	Resolved bool
	// Section is a mark of a whole section: it has no passage to be drawn on.
	Section bool
	// Changed is a mark whose passage is not in the note any more.
	Changed bool
	// Day is when the mark was made.
	Day  time.Time
	Lang wording.Lang
}

// Name is how the mark is named to a screen reader: the state first, then the
// first words of what it says.
func (m *MarkNote) Name() string {
	said := m.Words
	if said == "" {
		said = m.Quote
	}
	said = openingOf(said, 20)
	switch {
	case m.Kind == "question" && m.Resolved:
		return fmt.Sprintf(wording.MarksNameResolvedFmt.In(m.Lang), said)
	case m.Kind == "question":
		return fmt.Sprintf(wording.MarksNameQuestionFmt.In(m.Lang), said)
	case m.Kind == "thought":
		return fmt.Sprintf(wording.MarksNameThoughtFmt.In(m.Lang), said)
	default:
		return fmt.Sprintf(wording.MarksNameHighlightFmt.In(m.Lang), said)
	}
}

// MarkNoteID is the id of the note of a mark, which the mark's text points at.
func MarkNoteID(id string) string { return "mn-" + id }

// HasNote is whether the mark stands in the margin: a highlight on a passage is
// drawn in the text alone.
func (m *MarkNote) HasNote() bool {
	return m.Kind != "highlight" || m.Changed
}

// openingOf is the first n characters of s with every run of white space made
// one space, and a mark where it was cut.
func openingOf(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return strings.TrimSpace(string([]rune(s)[:n])) + "…"
}

// markState is the state of a mark as the stylesheet reads it: a question that
// is answered is no longer one that is open.
func (m *MarkNote) markState() string {
	if m.Kind == "question" && m.Resolved {
		return "resolved"
	}
	return m.Kind
}

// Span is the wrappers of the mark's passage: the first piece carries what a
// keyboard and a screen reader meet the mark by, and every other piece only
// belongs to it.
func (m *MarkNote) Span(start, end int) render.Span {
	attrs := fmt.Sprintf(`class="y-mark" data-mark="%s" data-kind="%s" data-state="%s"`, m.ID, m.Kind, m.markState())
	first := `<mark ` + attrs + ` tabindex="0" aria-label="` + html.EscapeString(m.Name()) + `"`
	if m.HasNote() {
		first += ` aria-describedby="` + MarkNoteID(m.ID) + `"`
	}
	return render.Span{Start: start, End: end, First: first + `>`, Rest: `<mark ` + attrs + `>`}
}

// NoteHTML is the mark's note as markup, or empty where it has none.
func (m *MarkNote) NoteHTML(ctx context.Context) (string, error) {
	if !m.HasNote() {
		return "", nil
	}
	var b bytes.Buffer
	if err := markNote(m).Render(ctx, &b); err != nil {
		return "", fmt.Errorf("drawing the note of mark %s: %w", m.ID, err)
	}
	return b.String(), nil
}

// opening is what stands in the note in place of words: the first words of the
// passage a question with none stands for.
func (m *MarkNote) opening() string {
	return fmt.Sprintf(wording.MarksOfThePassageFmt.In(m.Lang), openingOf(m.Quote, 16))
}

// lead is what stands before the day: that the question is answered, that the
// mark is of a whole section, or that its passage has changed.
func (m *MarkNote) leads() []string {
	var out []string
	if m.Kind == "question" && m.Resolved {
		out = append(out, wording.MarksResolvedLead.In(m.Lang))
	}
	if m.Section {
		out = append(out, wording.MarksSectionLead.In(m.Lang))
	}
	return out
}
