package note

import (
	"context"
	"strings"

	"github.com/koopa0/yomihon/internal/mark"
	"github.com/koopa0/yomihon/internal/mark/machinemarks"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/wording"
)

// markedBody lays the reader's highlights, thoughts and questions over the
// rendered text of a note: each passage wrapped where the reader chose it, and
// each note after the paragraph it is about, so that a page with no script
// reads as the page with one does. The marks of a whole section, and those whose
// passage the note no longer has, are not on a passage: they are handed back by
// the section they belong to, for the margin beside that heading to carry after
// it, and the ones with no section left are drawn at the end of the text.
//
// The words are the note's words as the page now shows them, the same that a
// selection is placed in, so a passage is found where the reader chose it and
// not where its offsets used to be. Where the marks cannot be read the page is
// served without them, and the script that keeps them says so when it cannot.
func (h *Handler) markedBody(ctx context.Context, body string, n *snapshot.Reading, sections []string, lang wording.Lang) (marked string, bySection map[string][]string) {
	kept := h.sources.Kept
	if kept == nil {
		return body, nil
	}
	h.mend()
	live, err := kept.Live(n.RelPath)
	if err != nil || len(live) == 0 {
		return body, nil
	}
	text := render.TextContent(body)
	var (
		spans []render.Span
		notes []render.Note
		tail  strings.Builder
	)
	bySection = make(map[string][]string)
	for i := range live {
		m := &live[i]
		drawn := noteOf(m, lang)
		place := mark.Locate(text, sections, m)
		switch {
		case place.Status == mark.Found:
			spans = append(spans, drawn.Span(place.Start, place.End))
			if html := h.noteHTML(ctx, drawn, n.RelPath); html != "" {
				notes = append(notes, render.Note{At: place.End, HTML: html})
			}
		case place.Status == mark.SectionOnly && m.Quote == "" && m.Body == "":
			// The mark the margin's own button makes and takes away: the button
			// says whether it is there, and it has no words to put in a note.
		default:
			drawn.Changed = place.Status == mark.Changed
			drawn.Section = m.Quote == ""
			html := h.noteHTML(ctx, drawn, n.RelPath)
			switch {
			case html == "":
			case m.Anchor != "" && (place.SectionKept || place.Status == mark.SectionOnly):
				bySection[m.Anchor] = append(bySection[m.Anchor], html)
			default:
				tail.WriteString(html)
			}
		}
	}
	return render.Overlay(body, spans, notes, tail.String()), bySection
}

// noteOf is a mark as the page draws it.
func noteOf(m *machinemarks.Mark, lang wording.Lang) *pages.MarkNote {
	return &pages.MarkNote{
		ID: m.ID, Kind: string(m.Kind), Words: m.Body, Quote: m.Quote,
		Resolved: !m.Resolved.IsZero(), Day: m.At, Lang: lang,
	}
}

// noteHTML draws the note of a mark, and says to the log, not to the reader,
// when it cannot: the mark is then still in the text.
func (h *Handler) noteHTML(ctx context.Context, m *pages.MarkNote, rel string) string {
	html, err := m.NoteHTML(ctx)
	if err != nil {
		h.sources.Log.Error("draw the note of a mark", "path", rel, "error", err)
		return ""
	}
	return html
}

// Note is the note of a mark as the page draws it, in lang, for the routes that
// answer a change with what the page now shows. It is empty where the mark has
// none: a highlight on a passage is drawn in the text alone.
func (h *Handler) Note(ctx context.Context, m *machinemarks.Mark, placement string, lang wording.Lang) string {
	drawn := noteOf(m, lang)
	drawn.Changed = placement == "changed"
	drawn.Section = m.Quote == ""
	return h.noteHTML(ctx, drawn, m.Path)
}
