package snapshot

import (
	"cmp"
	"slices"
	"strings"
	"time"

	"github.com/koopa0/yomihon/internal/vault"
)

// MarkedQuestion is one place the reader marked as a question, as the list of
// marks needs it: where, and when it was marked. The marks themselves are kept
// by the reading face; this is the copy it hands in.
type MarkedQuestion struct {
	RelPath string
	Anchor  string
	At      time.Time
}

// MarkItem is one item of the list of marks. Exactly one of Note and Question
// is set: a note of the rules' answer type that declares draft: true, or a
// marked question. At is what orders it: the question's time, or the note's
// own declared creation date, zero when it declares none.
type MarkItem struct {
	Note     *Reading
	Question *MarkedQuestion
	At       time.Time
}

// Path is the vault path the item names.
func (m *MarkItem) Path() string {
	if m.Question != nil {
		return m.Question.RelPath
	}
	return m.Note.RelPath
}

func (m *MarkItem) anchor() string {
	if m.Question != nil {
		return m.Question.Anchor
	}
	return ""
}

// MarkItems is the one list of marks, shared by every face that lists it: the
// notes of this generation's answer type that declare draft: true, interleaved
// with every question handed in, newest first. A note with no declared date
// sorts after every dated item; ties fall to path order, then anchor. A
// question is kept even when its note or its place is gone, because the stored
// identity is the reader's own record.
//
// It grants nothing about who may see an item: a caller writing for a program
// filters by its own authority before it resolves any item further.
func (g *Generation) MarkItems(questions []MarkedQuestion) []MarkItem {
	var items []MarkItem
	notes := g.NotesOfType(g.NavigationRoles().AnswerType())
	for i := range notes {
		if notes[i].Draft {
			items = append(items, MarkItem{Note: &notes[i], At: notes[i].Created})
		}
	}
	for i := range questions {
		items = append(items, MarkItem{Question: &questions[i], At: questions[i].At})
	}
	slices.SortStableFunc(items, func(a, b MarkItem) int {
		return cmp.Or(b.At.Compare(a.At), vault.ComparePaths(a.Path(), b.Path()), strings.Compare(a.anchor(), b.anchor()))
	})
	return items
}
