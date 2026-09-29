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
	shelf, fault := h.openThoughtShelf(r.Context(), snap, lang, 0)
	view := pages.ListIndexView{Mode: "open-thoughts", Kicker: shelf.Count, Notice: fault, Shelf: shelf}
	if err := pages.ListIndex(view, layouts.ChromeFromRequest(r, shelf.Title)).Render(r.Context(), w); err != nil {
		h.sources.Log.Log(r.Context(), origin.WriteFailureLevel(r, err), "write open thoughts", "error", err)
	}
}

// openThoughtCandidate is one row that may be shown. rel is set for a note,
// whose declared status is confirmed against the file only if the row is shown.
type openThoughtCandidate struct {
	openThoughtRow
	rel string
}

// openThoughtShelf interleaves marks with notes in the declared role's initial
// stages. Candidates come from the snapshot alone; a live status read then
// confirms only the note rows actually shown, so a note the existing writer
// just moved on leaves the list at once without every request parsing every
// note of the role. limit bounds the rows returned; zero means all of them.
// The count is the snapshot's, less any candidate the live read removed.
func (h *Handler) openThoughtShelf(ctx context.Context, snap *snapshot.Generation, lang wording.Lang, limit int) (pages.Shelf, string) {
	contract := h.sources.Contract
	role := snap.NavigationRoles().AnswerType()
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
	fault := ""
	var candidates []openThoughtCandidate
	for _, reading := range snap.NotesOfType(role) {
		if contract.DeclaresInitial(role, reading.Status) {
			candidates = append(candidates, openThoughtCandidate{openThoughtRow: openNoteRow(&reading, snap), rel: reading.RelPath})
		}
	}
	if h.sources.Uncertainties != nil {
		marks, err := h.sources.Uncertainties()
		if err != nil {
			fault = statedOnce(fault, wording.UncertaintyUnavailable.In(lang))
		} else {
			for _, kept := range marks {
				// A mark whose note this snapshot does not hold is never shown:
				// its path is text nothing in the vault vouches for.
				if row, ok := openMarkRow(&kept, snap, lang); ok {
					candidates = append(candidates, openThoughtCandidate{openThoughtRow: row})
				}
			}
		}
	}
	slices.SortStableFunc(candidates, func(a, b openThoughtCandidate) int {
		return cmp.Or(b.at.Compare(a.at), vault.ComparePaths(a.row.Href, b.row.Href), strings.Compare(a.row.Text, b.row.Text))
	})
	total := len(candidates)
	for _, item := range candidates {
		if limit > 0 && len(shelf.Rows) >= limit {
			break
		}
		if item.rel != "" {
			status, err := h.sources.ObservedStatus(ctx, item.rel)
			if err != nil {
				fault = wording.OpenThoughtsReadFailed.In(lang)
				total--
				continue
			}
			if !contract.DeclaresInitial(role, status) {
				total--
				continue
			}
		}
		shelf.Rows = append(shelf.Rows, item.row)
	}
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

func openNoteRow(reading *snapshot.Reading, snap *snapshot.Generation) openThoughtRow {
	text := cmp.Or(reading.Title, reading.RelPath)
	if declarations := snap.BasedOnDeclarations(reading.RelPath); len(declarations) > 0 {
		text += " — " + strings.Join(declarations, "; ")
	}
	row := pages.Row{Text: text, Href: pages.ResumeHref(reading.RelPath, "", 0), Language: reading.Language, Wrap: true}
	if !reading.Updated.IsZero() {
		row.When = reading.Updated.Format(time.DateOnly)
	}
	return openThoughtRow{row: row, at: reading.Updated}
}

// openMarkRow reports false for a mark whose note is not in the snapshot.
func openMarkRow(kept *mark.Uncertainty, snap *snapshot.Generation, lang wording.Lang) (openThoughtRow, bool) {
	reading, found := snap.Note(kept.RelPath)
	if !found {
		return openThoughtRow{}, false
	}
	text := cmp.Or(reading.Title, kept.RelPath)
	if kept.Anchor != "" {
		text += " #" + kept.Anchor
	}
	row := pages.Row{
		Text: text, Href: pages.ResumeHref(kept.RelPath, kept.Anchor, 0), Wrap: true,
		When: kept.At.Format(time.DateOnly), Mark: wording.UncertaintyControl.In(lang), Language: reading.Language,
	}
	return openThoughtRow{row: row, at: kept.At}, true
}
