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
	view := pages.ListIndexView{Mode: "open-thoughts", Count: shelf.Count, Notice: fault, Shelf: shelf}
	if err := pages.ListIndex(view, layouts.ChromeFromRequest(r, shelf.Title)).Render(r.Context(), w); err != nil {
		h.sources.Log.Log(r.Context(), origin.WriteFailureLevel(r, err), "write open thoughts", "error", err)
	}
}

// openThoughtCandidate is one row that may be shown. rel is set for a note,
// whose declared status is confirmed against the file only if the row is shown.
type openThoughtCandidate struct {
	openThoughtRow

	rel    string
	kept   *mark.Uncertainty
	path   string
	anchor string
}

// openThoughtShelf interleaves marks with notes in the declared role's initial
// stages. Candidates come from the snapshot alone; a live status read then
// confirms only the note rows actually shown, so a note the existing writer
// just moved on leaves the list at once without every request parsing every
// note of the role. limit bounds the rows returned; zero means all of them.
// The count is the snapshot's, less any candidate the live read removed.
func (h *Handler) openThoughtShelf(ctx context.Context, snap *snapshot.Generation, lang wording.Lang, limit int) (shelf pages.Shelf, fault string) {
	contract := h.sources.Contract
	role := snap.NavigationRoles().AnswerType()
	var initial []string
	for _, status := range contract.Statuses(role) {
		if role != "" && contract.DeclaresInitial(role, status) {
			initial = append(initial, status)
		}
	}
	shelf = pages.Shelf{
		Title: wording.OpenThoughtsTitle.In(lang), Href: openThoughtsAddress,
		Lede: wording.OpenThoughtsMarksLede.In(lang), Empty: wording.OpenThoughtsMarksEmpty.In(lang),
	}
	if len(initial) > 0 {
		stages := strings.Join(initial, wording.ListSeparator.In(lang))
		shelf.Lede = fmt.Sprintf(wording.OpenThoughtsLedeFmt.In(lang), role, stages)
		shelf.Empty = fmt.Sprintf(wording.OpenThoughtsEmptyFmt.In(lang), role, stages)
	}
	candidates, fault := h.openCandidates(snap, role, lang)
	slices.SortStableFunc(candidates, func(a, b openThoughtCandidate) int {
		return cmp.Or(b.at.Compare(a.at), vault.ComparePaths(a.path, b.path), strings.Compare(a.anchor, b.anchor))
	})
	rows, total, readFault := h.confirmOpenRows(ctx, candidates, role, limit, lang, newPlaceResolver(snap))
	shelf.Rows = rows
	fault = cmp.Or(readFault, fault)
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

// openCandidates reads the snapshot's notes of the role in an initial status
// and every stored mark. Mark identities remain candidates even when their
// note or place is gone. The fault is set when the mark file cannot be read.
//
// A note's snapshot status selects it, so a note moved back to an initial
// status reappears only after the next scan (at most one scan interval); a
// move out of one removes it at once through the live read.
func (h *Handler) openCandidates(snap *snapshot.Generation, role string, lang wording.Lang) (candidates []openThoughtCandidate, fault string) {
	notes := snap.NotesOfType(role)
	for i := range notes {
		if h.sources.Contract.DeclaresInitial(role, notes[i].Status) {
			candidates = append(candidates, openThoughtCandidate{openThoughtRow: openNoteRow(&notes[i], snap), rel: notes[i].RelPath, path: notes[i].RelPath})
		}
	}
	if h.sources.Uncertainties == nil {
		return candidates, ""
	}
	marks, err := h.sources.Uncertainties()
	if err != nil {
		return candidates, wording.UncertaintyUnavailable.In(lang)
	}
	for i := range marks {
		if !newPlaceResolver(snap).hasPlace(marks[i].RelPath, marks[i].Anchor) {
			continue
		}
		candidates = append(candidates, openThoughtCandidate{
			openThoughtRow: openThoughtRow{at: marks[i].At},
			kept:           &marks[i], path: marks[i].RelPath, anchor: marks[i].Anchor,
		})
	}
	return candidates, ""
}

// confirmOpenRows walks the sorted candidates and returns up to limit rows
// (all when limit is zero), reading the live status of each note row it is
// about to show. total is the candidate count less any note the live read
// removed or could not read; a candidate past the limit is counted as it stands.
func (h *Handler) confirmOpenRows(ctx context.Context, candidates []openThoughtCandidate, role string, limit int, lang wording.Lang, places *placeResolver) (rows []pages.Row, total int, fault string) {
	total = len(candidates)
	for i := range candidates {
		if limit > 0 && len(rows) >= limit {
			break
		}
		if rel := candidates[i].rel; rel != "" {
			status, err := h.sources.ObservedStatus(ctx, rel)
			if err != nil {
				fault = wording.OpenThoughtsReadFailed.In(lang)
				total--
				continue
			}
			if !h.sources.Contract.DeclaresInitial(role, status) {
				total--
				continue
			}
		}
		row := candidates[i].row
		if candidates[i].kept != nil {
			row = openMarkRow(candidates[i].kept, places, lang).row
		}
		rows = append(rows, row)
	}
	return rows, total, fault
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

// openMarkRow projects a stored identity without moving or deleting it.
func openMarkRow(kept *mark.Uncertainty, places *placeResolver, lang wording.Lang) openThoughtRow {
	resolved := places.note(kept.RelPath)
	row := pages.Row{Text: kept.RelPath, Wrap: true, When: kept.At.Format(time.DateOnly)}
	if resolved.readable {
		row.Text = cmp.Or(resolved.reading.Title, kept.RelPath)
		row.Language = resolved.reading.Language
		if kept.Anchor != "" {
			row.Text += " #" + kept.Anchor
		}
		row.Href = pages.ResumeHref(kept.RelPath, "", 0)
		if places.hasPlace(kept.RelPath, kept.Anchor) {
			row.Href = pages.ResumeHref(kept.RelPath, kept.Anchor, 0)
			row.Mark = wording.UncertaintyControl.In(lang)
			return openThoughtRow{row: row, at: kept.At}
		}
		row.Mark = wording.UncertaintyPlaceNotFound.In(lang)
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
