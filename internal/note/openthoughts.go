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

const openThoughtsAddress = "/open-thoughts"

func (h *Handler) openThoughts(w http.ResponseWriter, r *http.Request) {
	lang := origin.Language(r)
	snap := h.sources.Snapshot().Capture()
	order := pages.ParseOpenOrder(r.URL.Query().Get("order"))
	shelf, fault := h.openThoughtShelf(snap, lang, 0, order)
	view := pages.ListIndexView{Mode: "open-thoughts", Notice: fault, Shelf: shelf, Order: openOrderChoices(order, lang)}
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
// not keeps the marks-only sentence. The shelf states no count: how many things
// were left open is not a debt the page presents.
func (h *Handler) openThoughtShelf(snap *snapshot.Generation, lang wording.Lang, limit int, order pages.OpenOrder) (shelf pages.Shelf, fault string) {
	shelf = pages.Shelf{
		Title: wording.OpenThoughtsTitle.In(lang), Href: openThoughtsAddress, Open: true,
		Lede: wording.OpenThoughtsMarksLede.In(lang), Empty: wording.OpenThoughtsMarksEmpty.In(lang),
	}
	if snap.NavigationRoles().AnswerType() != "" {
		shelf.Lede, shelf.Empty = wording.OpenThoughtsLede.In(lang), ""
	}
	candidates, fault := h.openCandidates(snap, lang)
	orderOpenCandidates(candidates, order)
	shelf.Rows = openRows(candidates, limit, lang, snap, newPlaceResolver(snap))
	if fault != "" {
		// A failed source is not an empty source.
		shelf.Empty = ""
	}
	return shelf, fault
}

// openCandidates reads every stored mark and asks the snapshot for the one
// selection of open items, already newest first. Mark identities remain
// candidates even when their note or place is gone. The fault is set when the
// mark file cannot be read.
func (h *Handler) openCandidates(snap *snapshot.Generation, lang wording.Lang) (candidates []openThoughtCandidate, fault string) {
	var marks []snapshot.OpenMark
	if h.sources.Uncertainties != nil {
		kept, err := h.sources.Uncertainties()
		if err != nil {
			fault = wording.UncertaintyUnavailable.In(lang)
		}
		marks = OpenMarks(kept)
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
func OpenMarks(kept []mark.Uncertainty) []snapshot.OpenMark {
	marks := make([]snapshot.OpenMark, 0, len(kept))
	for i := range kept {
		marks = append(marks, snapshot.OpenMark{RelPath: kept[i].RelPath, Anchor: kept[i].Anchor, At: kept[i].At})
	}
	return marks
}

// openRows draws the ordered candidates as up to limit rows, all of them when
// limit is zero. Only the rows drawn are read for their sentence, so the desk's
// five cost five notes however many things are open.
func openRows(candidates []openThoughtCandidate, limit int, lang wording.Lang, snap *snapshot.Generation, places *placeResolver) []pages.Row {
	var rows []pages.Row
	for i := range candidates {
		if limit > 0 && len(rows) >= limit {
			break
		}
		if candidates[i].kept != nil {
			rows = append(rows, openMarkRow(candidates[i].kept, places, lang))
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
func openMarkRow(kept *snapshot.OpenMark, places *placeResolver, lang wording.Lang) pages.Row {
	resolved := places.note(kept.RelPath)
	row := pages.Row{Text: kept.RelPath, When: kept.At.Format(time.DateOnly)}
	if resolved.readable {
		row.Text = cmp.Or(resolved.reading.Title, kept.RelPath)
		row.Language = resolved.reading.Language
		row.Href = pages.ResumeHref(kept.RelPath, "", 0)
		if places.hasPlace(kept.RelPath, kept.Anchor) {
			row.Place = places.label(kept.RelPath, kept.Anchor, lang)
			row.Href = pages.ResumeHref(kept.RelPath, kept.Anchor, 0)
			row.Mark = wording.UncertaintyControl.In(lang)
			row.Opening = places.opening(kept.RelPath, kept.Anchor)
			return row
		}
		row.Mark = wording.UncertaintyPlaceNotFound.In(lang)
		if kept.Anchor != "" {
			// The anchor is all that tells two lost marks in one note apart.
			row.Place = pages.RowPlace{Text: "#" + kept.Anchor, Language: resolved.reading.Language}
		}
	} else {
		row.Mark = wording.UncertaintyNoteNotFound.In(lang)
	}
	row.Fault = true
	row.Removal = &pages.UncertaintyRemoval{Endpoint: mark.UncertaintyAddress, Path: kept.RelPath, Anchor: kept.Anchor,
		Label: wording.UncertaintyClearControl.In(lang), Unavailable: wording.UncertaintyUnavailable.In(lang),
		Failed: wording.UncertaintyNotStored.In(lang), Cleared: wording.UncertaintyCleared.In(lang), Language: string(lang),
	}
	return row
}
