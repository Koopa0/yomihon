package pages

import (
	"path"
	"slices"
	"strings"
	"time"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

// recentItems is how many notes the desk's band of recent changes lists: a
// glance at what moved last, with the folders a way to the rest.
const recentItems = 5

// NewRecentShelf is the desk's band of the notes that changed last on disk,
// newest first: each note's name, the day it changed and the folder it is in,
// followed by the word draft for a note that declares itself one.
// It reads the times the scan captured, so drawing it opens no file.
//
// A vault with no more notes than the band holds has nothing to single out: the
// band would list all of it, which the folders already offer, and it is not
// drawn. A note whose time could not be read has no place in an order by time
// and is left out. A copy or a fresh checkout stamps its files one after another
// within the same second or so, and the newest of them are then whichever were
// written last, not what anybody changed. So a note that changed in the same
// second as one the band has no room for is not listed either: which of the two
// made the cut is the copy's order and not the changes. Notes that share a
// second all inside the band are listed. Where that leaves nothing — a vault
// fresh from a copy that nobody has changed since — the band is not drawn.
func NewRecentShelf(notes []nav.NoteSummary, now time.Time, lang wording.Lang, articleLang ArticleLanguageFor) Shelf {
	notes = slices.DeleteFunc(slices.Clone(notes), func(n nav.NoteSummary) bool { return n.Modified.IsZero() })
	if len(notes) <= recentItems {
		return Shelf{}
	}
	slices.SortStableFunc(notes, func(a, b nav.NoteSummary) int {
		if r := b.Modified.Compare(a.Modified); r != 0 {
			return r
		}
		return vault.ComparePaths(a.RelPath, b.RelPath)
	})
	second := func(n *nav.NoteSummary) time.Time { return n.Modified.Truncate(time.Second) }
	cut := second(&notes[recentItems])
	notes = notes[:recentItems]
	for len(notes) > 0 && second(&notes[len(notes)-1]).Equal(cut) {
		notes = notes[:len(notes)-1]
	}
	if len(notes) == 0 {
		return Shelf{}
	}
	rows := make([]Row, 0, len(notes))
	for i := range notes {
		n := &notes[i]
		var where []string
		if folder := path.Dir(n.RelPath); folder != "." {
			where = append(where, folder)
		}
		if n.Draft {
			where = append(where, wording.Draft.In(lang))
		}
		rows = append(rows, Row{
			Text:     n.Title,
			Href:     notesHref(n.RelPath),
			When:     listDate(n.Modified.In(now.Location()), now),
			ByTitle:  true,
			Where:    strings.Join(where, " · "),
			Language: rowLanguage(articleLang, n.RelPath),
		})
	}
	return Shelf{Title: wording.RecentlyChanged.In(lang), Rows: rows}
}

// leads reports whether block opens the desk. A reader who kept a place is
// opened on that place; one who kept none is opened on the courses, which are
// what a desk is for, and only where the vault has some to open: a block with
// nothing to link to would lead with an apology.
func (v *HomeView) leads(block *DeskBlock) bool {
	return !v.Continue.Show && block.Mode == courseMode && len(shelfRows(&block.Shelf, deskBlockItems)) > 0
}

// deskWay is one of the ways in that stand below the lead, and whether it
// takes the desk's whole width.
type deskWay struct {
	Block DeskBlock
	Whole bool
}

// ways lays out the blocks that do not lead. They stand two to a row, as
// shelves side by side, except the folders, which are the way into everything
// and run across the desk as one strip of their own wherever they come in the
// order. A block that would be left alone in a row takes the row rather than
// leave half the desk empty beside it.
func (v *HomeView) ways() []deskWay {
	var ways []deskWay
	half := 0
	for i := range v.Blocks {
		block := v.Blocks[i]
		if v.leads(&block) {
			continue
		}
		whole := block.Mode == folderMode
		if !whole {
			half++
		}
		ways = append(ways, deskWay{Block: block, Whole: whole})
	}
	if half%2 == 1 {
		for i := len(ways) - 1; i >= 0; i-- {
			if !ways[i].Whole {
				ways[i].Whole = true
				break
			}
		}
	}
	return ways
}
