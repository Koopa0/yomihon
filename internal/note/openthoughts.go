package note

import (
	"cmp"
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/koopa0/yomihon/internal/mark"
	"github.com/koopa0/yomihon/internal/origin"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/vault"
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
	shelf, fault := h.openThoughtShelf(r.Context(), snap, lang)
	view := pages.ListIndexView{Mode: "open-thoughts", Kicker: shelf.Count, Notice: fault, Shelf: shelf}
	if err := pages.ListIndex(view, layouts.ChromeFromRequest(r, shelf.Title)).Render(r.Context(), w); err != nil {
		h.sources.Log.Log(r.Context(), origin.WriteFailureLevel(r, err), "write open thoughts", "error", err)
	}
}

// openThoughtShelf interleaves marks with notes in the declared role's initial
// stages. A live status read removes a note immediately after the existing
// writer changes it, even while the scanner still holds its earlier status.
func (h *Handler) openThoughtShelf(ctx context.Context, snap *snapshot.Generation, lang wording.Lang) (pages.Shelf, string) {
	contract := h.sources.Contract
	role := contract.AnswerType()
	var initial []string
	for _, status := range contract.Statuses(role) {
		if role != "" && contract.DeclaresInitial(role, status) {
			initial = append(initial, status)
		}
	}
	shelf := pages.Shelf{
		Title: wording.OpenThoughtsTitle.In(lang), Href: openThoughtsAddress,
		Lede: wording.OpenThoughtsMarksLede.In(lang), Empty: wording.OpenThoughtsMarksEmpty.In(lang),
	}
	if len(initial) > 0 {
		stages := strings.Join(initial, wording.ListSeparator.In(lang))
		shelf.Lede = fmt.Sprintf(wording.OpenThoughtsLedeFmt.In(lang), role, stages)
		shelf.Empty = fmt.Sprintf(wording.OpenThoughtsEmptyFmt.In(lang), role, stages)
	}
	rows, fault := h.openNoteRows(ctx, snap, role, lang)
	if h.sources.Uncertainties != nil {
		marks, err := h.sources.Uncertainties()
		if err != nil {
			fault = statedOnce(fault, wording.UncertaintyUnavailable.In(lang))
		} else {
			for _, kept := range marks {
				rows = append(rows, openMarkRow(&kept, snap, lang))
			}
		}
	}
	slices.SortStableFunc(rows, func(a, b openThoughtRow) int {
		return cmp.Or(b.at.Compare(a.at), vault.ComparePaths(a.row.Href, b.row.Href), strings.Compare(a.row.Text, b.row.Text))
	})
	for _, item := range rows {
		shelf.Rows = append(shelf.Rows, item.row)
	}
	if fault == "" {
		count := wording.OpenThoughtsCount
		if len(rows) == 1 {
			count = wording.OpenThoughtsCountOne
		}
		shelf.Count = fmt.Sprintf(count.In(lang), len(rows))
	} else {
		// A failed source is not an empty source or a complete count.
		shelf.Empty = ""
	}
	return shelf, fault
}

func (h *Handler) openNoteRows(ctx context.Context, snap *snapshot.Generation, role string, lang wording.Lang) ([]openThoughtRow, string) {
	var rows []openThoughtRow
	fault := ""
	for _, reading := range snap.NotesOfType(role) {
		status, err := h.sources.ObservedStatus(ctx, reading.RelPath)
		if err != nil {
			fault = wording.OpenThoughtsReadFailed.In(lang)
			continue
		}
		if !h.sources.Contract.DeclaresInitial(role, status) {
			continue
		}
		text := cmp.Or(reading.Title, reading.RelPath)
		if declarations := snap.BasedOnDeclarations(reading.RelPath); len(declarations) > 0 {
			text += " — " + strings.Join(declarations, "; ")
		}
		row := pages.Row{Text: text, Href: pages.ResumeHref(reading.RelPath, "", 0), Language: reading.Language}
		if !reading.Updated.IsZero() {
			row.When = reading.Updated.Format(time.DateOnly)
		}
		rows = append(rows, openThoughtRow{row: row, at: reading.Updated})
	}
	return rows, fault
}

func openMarkRow(kept *mark.Uncertainty, snap *snapshot.Generation, lang wording.Lang) openThoughtRow {
	text := kept.RelPath
	reading, found := snap.Note(kept.RelPath)
	if found {
		text = cmp.Or(reading.Title, text)
	}
	if kept.Anchor != "" {
		text += " #" + kept.Anchor
	}
	row := pages.Row{
		Text: text, Href: pages.ResumeHref(kept.RelPath, kept.Anchor, 0),
		When: kept.At.Format(time.DateOnly), Mark: wording.UncertaintyControl.In(lang), Language: reading.Language,
	}
	if !found {
		row.Mark = wording.MarkNoteGone.In(lang)
		row.Fault = true
	}
	return openThoughtRow{row: row, at: kept.At}
}
