package note

import (
	"cmp"
	"net/http"
	"slices"
	"time"

	"github.com/koopa0/yomihon/internal/mark"
	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

const openThoughtsAddress = pages.MarksAddress

// mend puts back what git took out of this computer's file of marks, as a page
// that reads the marks is drawn, and tells the reader nothing of it.
func (h *Handler) mend() {
	if h.sources.Kept != nil {
		h.sources.Kept.Mend()
	}
}

// openThoughts draws the page of the reader's marks.
func (h *Handler) openThoughts(w http.ResponseWriter, r *http.Request) {
	h.mend()
	lang := origin.Language(r)
	snap := h.sources.Snapshot().Capture()
	order := pages.ParseOpenOrder(r.URL.Query().Get("order"))
	authority := h.sources.Authority()
	shelf, fault := h.openThoughtShelf(snap, authority.Governed(), lang, 0, order)
	view := pages.ListIndexView{Mode: "marks", Notice: fault, Shelf: shelf, Order: openOrderChoices(order, lang), Sidebar: pages.NewLibrarySidebar(h.projectShell(authority, snap), pages.LibraryMarks, "")}
	if err := pages.ListIndex(view, layouts.ChromeFromRequest(r, shelf.Title)).Render(r.Context(), w); err != nil {
		h.sources.Log.Log(r.Context(), origin.WriteFailureLevel(r, err), "write open thoughts", "error", err)
	}
}

// openOrderChoices is the choice of order the shelf offers, with the one it is
// in marked.
func openOrderChoices(current pages.OpenOrder, lang wording.Lang) pages.ShelfOrder {
	words := map[pages.OpenOrder]wording.Phrase{
		pages.OpenNewest:   wording.OpenOrderNewest,
		pages.OpenBySource: wording.OpenOrderSource,
		pages.OpenOldest:   wording.OpenOrderOldest,
	}
	order := pages.ShelfOrder{Label: wording.OpenOrderLabel.In(lang)}
	for _, choice := range pages.OpenOrders {
		order.Choices = append(order.Choices, pages.ShelfOrderChoice{Value: string(choice), Text: words[choice].In(lang), Pressed: choice == current})
	}
	return order
}

// openThoughtCandidate is one thing the reader left open, before it is drawn:
// a note of the contract's answer type that is still a draft, or a marked
// location still to be resolved against the generation. at is the day it is
// dated by and source the note it came from, which are what the shelf is
// ordered by.
type openThoughtCandidate struct {
	note   *snapshot.Reading
	kept   *snapshot.OpenMark
	at     time.Time
	source string
}

// openThoughtShelf interleaves marks with the declared role's notes that still
// declare draft: true, all read from one snapshot, in the order asked for. limit
// bounds the rows returned; zero means all of them.
//
// A vault whose contract declares the role says so in the lede; one that does
// not keeps the marks-only sentence. A folder with no rules file declares
// nothing and reads the built-in role, so it keeps the marks-only sentence
// until a note of that type is actually open. The shelf states no count: how
// many things were left open is not a debt the page presents.
func (h *Handler) openThoughtShelf(snap *snapshot.Generation, governed bool, lang wording.Lang, limit int, order pages.OpenOrder) (shelf pages.Shelf, fault string) {
	shelf = pages.Shelf{
		Title: wording.OpenThoughtsTitle.In(lang), Href: openThoughtsAddress, Open: true,
		Lede: wording.OpenThoughtsMarksLede.In(lang), Empty: wording.OpenThoughtsMarksEmpty.In(lang),
	}
	candidates, fault := h.openCandidates(snap, lang)
	if snap.NavigationRoles().AnswerType() != "" && (governed || slices.ContainsFunc(candidates, func(c openThoughtCandidate) bool { return c.note != nil })) {
		shelf.Lede, shelf.Empty = wording.OpenThoughtsLede.In(lang), ""
	}
	orderOpenCandidates(candidates, order)
	_, notWritable := h.questionAddress()
	shelf.Rows = openRows(candidates, limit, lang, snap, newPlaceResolver(snap), notWritable == "")
	if fault != "" {
		// A failed source is not an empty source.
		shelf.Empty = ""
	}
	return shelf, fault
}

// OpenCount is how many things the reader has left open in this generation: the
// rows the marks page lists, which the sidebar's row for it states. A mark file
// that cannot be read counts for nothing here; the marks page is where that is
// said.
func OpenCount(snap *snapshot.Generation, questions func() ([]mark.Mark, error)) int {
	marks, err := openMarksOf(questions)
	if err != nil {
		return len(snap.OpenItems(nil))
	}
	return len(snap.OpenItems(marks))
}

// openMarksOf reads the reader's stored marks as the open items the snapshot
// selects among. A nil source is a process with nowhere to keep any.
func openMarksOf(questions func() ([]mark.Mark, error)) (marks []snapshot.OpenMark, err error) {
	if questions == nil {
		return nil, nil
	}
	kept, err := questions()
	return OpenMarks(kept), err
}

// openCandidates reads every stored mark and asks the snapshot for the one
// selection of open items, already newest first. Mark identities remain
// candidates even when their note or place is gone. The fault is set when the
// mark file cannot be read.
func (h *Handler) openCandidates(snap *snapshot.Generation, lang wording.Lang) (candidates []openThoughtCandidate, fault string) {
	marks, err := openMarksOf(h.sources.Questions)
	if err != nil {
		fault = wording.QuestionUnavailable.In(lang)
	}
	for _, item := range snap.OpenItems(marks) {
		if item.Mark != nil {
			candidates = append(candidates, openThoughtCandidate{kept: item.Mark, at: item.At, source: item.Mark.RelPath})
			continue
		}
		candidates = append(candidates, openThoughtCandidate{note: item.Note, at: item.At, source: openSourceKey(snap, item.Note.RelPath)})
	}
	return candidates, fault
}

// openSourceKey is the note a thought came from, which is what groups it with
// the places marked in that note: the first source it declares, or itself when
// it declares none.
func openSourceKey(snap *snapshot.Generation, rel string) string {
	if sources := snap.BasedOn(rel); len(sources) > 0 {
		return cmp.Or(sources[0].RelPath, sources[0].Name)
	}
	return rel
}

// orderOpenCandidates puts candidates in the order asked for. They arrive
// newest first, ties by path, and a stable sort keeps that order inside every
// group the others make. A thought that declares no date stays after every
// dated item whichever way the dates run: it has no day to be early or late.
func orderOpenCandidates(candidates []openThoughtCandidate, order pages.OpenOrder) {
	switch order {
	case pages.OpenOldest:
		slices.SortStableFunc(candidates, func(a, b openThoughtCandidate) int {
			if a.at.IsZero() != b.at.IsZero() {
				return cmp.Compare(boolWeight(a.at.IsZero()), boolWeight(b.at.IsZero()))
			}
			return a.at.Compare(b.at)
		})
	case pages.OpenBySource:
		slices.SortStableFunc(candidates, func(a, b openThoughtCandidate) int {
			return vault.ComparePaths(a.source, b.source)
		})
	case pages.OpenNewest:
	}
}

func boolWeight(b bool) int {
	if b {
		return 1
	}
	return 0
}

// OpenMarks is the copy of the reader's stored marks the open selection reads.
func OpenMarks(kept []mark.Mark) []snapshot.OpenMark {
	marks := make([]snapshot.OpenMark, 0, len(kept))
	for i := range kept {
		marks = append(marks, snapshot.OpenMark{RelPath: kept[i].RelPath, Anchor: kept[i].Anchor, At: kept[i].At})
	}
	return marks
}

// openRows draws the ordered candidates as up to limit rows, all of them when
// limit is zero. Only the rows drawn are read for their sentence, so the desk's
// five cost five notes however many things are open.
func openRows(candidates []openThoughtCandidate, limit int, lang wording.Lang, snap *snapshot.Generation, places *placeResolver, removable bool) []pages.Row {
	var rows []pages.Row
	for i := range candidates {
		if limit > 0 && len(rows) >= limit {
			break
		}
		if candidates[i].kept != nil {
			rows = append(rows, openMarkRow(candidates[i].kept, places, lang, removable))
			continue
		}
		rows = append(rows, openNoteRow(candidates[i].note, snap, lang))
	}
	return rows
}

// openNoteRow draws a thought of the reader's own: what it is, the day it was
// written, its first sentence as it stands, and the notes it says it came from.
func openNoteRow(reading *snapshot.Reading, snap *snapshot.Generation, lang wording.Lang) pages.Row {
	row := pages.Row{
		Text:     cmp.Or(reading.Title, reading.RelPath),
		Href:     pages.ResumeHref(reading.RelPath, "", 0),
		Language: reading.Language,
		Mark:     wording.OpenThoughtKind.In(lang),
		Thought:  true,
		Draft:    reading.Draft,
		Opening:  render.OpeningSentence(reading.Body),
		Sources:  openSources(snap, reading.RelPath, lang),
	}
	if !reading.Created.IsZero() {
		row.When = reading.Created.Format(time.DateOnly)
	}
	return row
}

// openSources are the notes a thought declares it came from, each as the place
// in it the author named when they named one, and the note alone when not.
func openSources(snap *snapshot.Generation, rel string, lang wording.Lang) []pages.RowSource {
	groups, _ := snap.DeclaredSources(rel, lang)
	var sources []pages.RowSource
	for i := range groups {
		group := &groups[i]
		language := ""
		if source, ok := snap.Note(group.RelPath); ok {
			language = source.Language
		}
		if group.RelPath == "" {
			sources = append(sources, pages.RowSource{Text: group.Name})
			continue
		}
		if group.WholeFile || len(group.Locations) == 0 {
			sources = append(sources, pages.RowSource{Text: group.Name, Href: pages.ResumeHref(group.RelPath, "", 0), Language: language})
		}
		for _, location := range group.Locations {
			sources = append(sources, pages.RowSource{
				Text:     group.Name + " › " + location.Label,
				Href:     pages.ResumeHref(group.RelPath, location.Fragment, 0),
				Language: language,
			})
		}
	}
	return sources
}

// openMarkRow projects a stored identity without moving or deleting it.
func openMarkRow(kept *snapshot.OpenMark, places *placeResolver, lang wording.Lang, removable bool) pages.Row {
	resolved := places.note(kept.RelPath)
	row := pages.Row{Text: kept.RelPath, When: kept.At.Format(time.DateOnly)}
	if resolved.readable {
		row.Text = cmp.Or(resolved.reading.Title, kept.RelPath)
		row.Language = resolved.reading.Language
		row.Href = pages.ResumeHref(kept.RelPath, "", 0)
		if places.hasPlace(kept.RelPath, kept.Anchor) {
			row.Place = places.label(kept.RelPath, kept.Anchor, lang)
			row.Href = pages.ResumeHref(kept.RelPath, kept.Anchor, 0)
			row.Mark = wording.QuestionKind.In(lang)
			row.Opening = places.opening(kept.RelPath, kept.Anchor)
			return row
		}
		row.Mark = wording.QuestionPlaceNotFound.In(lang)
		if kept.Anchor != "" {
			// The anchor is all that tells two lost marks in one note apart.
			row.Place = pages.RowPlace{Text: "#" + kept.Anchor, Language: resolved.reading.Language}
		}
	} else {
		row.Mark = wording.NoteNotFound.In(lang)
	}
	row.Fault = true
	if !removable {
		return row
	}
	row.Removal = &pages.QuestionRemoval{Endpoint: mark.Paths.Question, Path: kept.RelPath, Anchor: kept.Anchor,
		Label: wording.QuestionClearControl.In(lang), Unavailable: wording.QuestionUnavailable.In(lang),
		Failed: wording.QuestionNotStored.In(lang), Cleared: wording.QuestionCleared.In(lang), Language: string(lang),
	}
	return row
}

// questionAddress is where the page posts a section's question to, and why it
// cannot where the reader folder cannot be written to: the control is then not
// drawn, and the page says why in the word the reader folder's check gave.
func (h *Handler) questionAddress() (address, writable string) {
	if h.sources.QuestionAddress == "" {
		return "", ""
	}
	if h.sources.Kept != nil {
		if why := h.sources.Kept.Writable(); why != mark.WritableOK {
			return "", why
		}
	}
	return h.sources.QuestionAddress, ""
}
