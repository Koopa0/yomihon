package note

import (
	"cmp"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/koopa0/yomihon/internal/mark"
	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/wording"
)

const openThoughtsAddress = "/open-thoughts"

type openThoughtRow struct {
	row pages.Row
	at  time.Time
}

func (h *Handler) openThoughts(w http.ResponseWriter, r *http.Request) {
	lang := origin.Language(r)
	snap := h.sources.Snapshot().Capture()
	shelf, fault := h.openThoughtShelf(snap, lang, 0)
	view := pages.ListIndexView{Mode: "open-thoughts", Count: shelf.Count, Notice: fault, Shelf: shelf}
	if err := pages.ListIndex(view, layouts.ChromeFromRequest(r, shelf.Title)).Render(r.Context(), w); err != nil {
		h.sources.Log.Log(r.Context(), origin.WriteFailureLevel(r, err), "write open thoughts", "error", err)
	}
}

// openThoughtCandidate is one row that may be shown: a note, or a marked
// location still to be resolved against the generation.
type openThoughtCandidate struct {
	openThoughtRow

	kept *snapshot.OpenMark
}

// openThoughtShelf interleaves marks with the declared role's notes that still
// declare draft: true, all read from one snapshot. limit bounds the rows
// returned; zero means all of them.
//
// A vault whose contract declares the role says so in the lede, and the count
// beside the title states an empty shelf; one that does not keeps the
// marks-only sentences.
func (h *Handler) openThoughtShelf(snap *snapshot.Generation, lang wording.Lang, limit int) (shelf pages.Shelf, fault string) {
	shelf = pages.Shelf{
		Title: wording.OpenThoughtsTitle.In(lang), Href: openThoughtsAddress,
		Lede: wording.OpenThoughtsMarksLede.In(lang), Empty: wording.OpenThoughtsMarksEmpty.In(lang),
	}
	if snap.NavigationRoles().AnswerType() != "" {
		shelf.Lede, shelf.Empty = wording.OpenThoughtsLede.In(lang), ""
	}
	candidates, fault := h.openCandidates(snap, lang)
	shelf.Rows = openRows(candidates, limit, lang, newPlaceResolver(snap))
	total := len(candidates)
	if fault == "" {
		count := wording.OpenThoughtsCount
		if total == 1 {
			count = wording.OpenThoughtsCountOne
		}
		shelf.Count = fmt.Sprintf(count.In(lang), total)
	} else {
		// A failed source is not an empty source or a complete count.
		shelf.Empty = ""
	}
	return shelf, fault
}

// openCandidates reads every stored mark and asks the snapshot for the one
// selection of open items, already in shelf order. Mark identities remain
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
			candidates = append(candidates, openThoughtCandidate{at: item.At, kept: item.Mark})
			continue
		}
		candidates = append(candidates, openThoughtCandidate{openThoughtRow: openNoteRow(item.Note, snap)})
	}
	return candidates, fault
}

// OpenMarks is the copy of the reader's stored marks the open selection reads.
func OpenMarks(kept []mark.Uncertainty) []snapshot.OpenMark {
	marks := make([]snapshot.OpenMark, 0, len(kept))
	for i := range kept {
		marks = append(marks, snapshot.OpenMark{RelPath: kept[i].RelPath, Anchor: kept[i].Anchor, At: kept[i].At})
	}
	return marks
}

// openRows projects the sorted candidates into up to limit rows, all of them
// when limit is zero.
func openRows(candidates []openThoughtCandidate, limit int, lang wording.Lang, places *placeResolver) []pages.Row {
	var rows []pages.Row
	for i := range candidates {
		if limit > 0 && len(rows) >= limit {
			break
		}
		row := candidates[i].row
		if candidates[i].kept != nil {
			row = openMarkRow(candidates[i].kept, places, lang).row
		}
		rows = append(rows, row)
	}
	return rows
}

func openNoteRow(reading *snapshot.Reading, snap *snapshot.Generation) openThoughtRow {
	text := cmp.Or(reading.Title, reading.RelPath)
	if declarations := snap.BasedOnDeclarations(reading.RelPath); len(declarations) > 0 {
		text += " — " + strings.Join(declarations, "; ")
	}
	row := pages.Row{Text: text, Href: pages.ResumeHref(reading.RelPath, "", 0), Language: reading.Language, Wrap: true}
	if !reading.Created.IsZero() {
		row.When = reading.Created.Format(time.DateOnly)
	}
	return openThoughtRow{row: row, at: reading.Created}
}

// openMarkRow projects a stored identity without moving or deleting it.
func openMarkRow(kept *snapshot.OpenMark, places *placeResolver, lang wording.Lang) openThoughtRow {
	resolved := places.note(kept.RelPath)
	row := pages.Row{Text: kept.RelPath, Wrap: true, When: kept.At.Format(time.DateOnly)}
	if resolved.readable {
		row.Text = cmp.Or(resolved.reading.Title, kept.RelPath)
		row.Language = resolved.reading.Language
		row.Href = pages.ResumeHref(kept.RelPath, "", 0)
		if places.hasPlace(kept.RelPath, kept.Anchor) {
			row.Place = places.label(kept.RelPath, kept.Anchor, lang)
			row.Href = pages.ResumeHref(kept.RelPath, kept.Anchor, 0)
			row.Mark = wording.UncertaintyControl.In(lang)
			return openThoughtRow{row: row, at: kept.At}
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
	return openThoughtRow{row: row, at: kept.At}
}
