package search

import (
	"cmp"
	"net/http"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/koopa0/yomihon/internal/lexical"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/wording"
)

// filterFloor is the number of notes up to which an answer is read through
// rather than narrowed: below it the rows that narrow it would cost more room
// than they save.
const filterFloor = 10

// The kinds an answer is told apart by, as the address spells them. The words
// the reader sees come from wording; these are only keys.
const (
	kindUnit    = "unit"
	kindCourse  = "course"
	kindIndex   = "index"
	kindDrill   = "drill"
	kindThought = "thought"
	kindNote    = "note"
	kindFile    = "file"
)

// kindOrder breaks a tie between two kinds that hold as many notes.
var kindOrder = []string{kindUnit, kindNote, kindCourse, kindIndex, kindDrill, kindThought, kindFile}

// rootPlace is the address's spelling of notes that sit at the vault's top.
const rootPlace = "."

// readState reads what a request says about how to lay the answer out. A word
// the address does not hold, or holds wrongly, reads as its default.
func readState(r *http.Request, q string) pages.SearchState {
	v := r.URL.Query()
	st := pages.SearchState{Query: q, Kind: v.Get(pages.SearchKindParam), In: v.Get(pages.SearchInParam)}
	if v.Get(pages.SearchSortParam) == pages.SearchSortRecent {
		st.Sort = pages.SearchSortRecent
	}
	if n, err := strconv.Atoi(v.Get(pages.SearchShownParam)); err == nil && n > pages.SearchStep {
		st.Shown = n
	}
	return st
}

// kindOf is the key of what a result is, from the type its note declares and
// the roles the rules give types. A type the rules give no role, and a note
// that declares none, are ordinary notes.
func kindOf(r *lexical.Result, roles schema.NavigationRoles) string {
	switch {
	case r.File:
		return kindFile
	case roles.IsLessonType(r.NoteType):
		return kindUnit
	case roles.IsPathType(r.NoteType):
		return kindCourse
	case roles.IsMapType(r.NoteType):
		return kindIndex
	}
	switch strings.ToLower(r.NoteType) {
	case "drill":
		return kindDrill
	case "thought":
		return kindThought
	}
	return kindNote
}

// kindWord is what the screen calls a kind.
func kindWord(kind string, lang wording.Lang) string {
	switch kind {
	case kindUnit:
		return wording.FacetTypeUnit.In(lang)
	case kindCourse:
		return wording.FacetTypeCourse.In(lang)
	case kindIndex:
		return wording.FacetTypeIndex.In(lang)
	case kindDrill:
		return wording.FacetTypeDrill.In(lang)
	case kindThought:
		return wording.FacetTypeThought.In(lang)
	case kindFile:
		return wording.ResultKindFile.In(lang)
	}
	return wording.FacetTypeNote.In(lang)
}

// placeOf is the folder a result is filed under, to the given depth, in the
// address's spelling: the vault's top for a note with no folder.
func placeOf(relPath string, depth int) string {
	dir := path.Dir(relPath)
	if dir == "." {
		return rootPlace
	}
	parts := strings.Split(dir, "/")
	return strings.Join(parts[:min(depth, len(parts))], "/")
}

// placeDepth is how many levels a place names.
func placeDepth(place string) int {
	if place == rootPlace || place == "" {
		return 1
	}
	return strings.Count(place, "/") + 1
}

// lastLevels is the last n folder names of a result's directory, the way a
// reader says where a note lives. Empty for a note at the vault's top.
func lastLevels(relPath string, n int) string {
	dir := path.Dir(relPath)
	if dir == "." {
		return ""
	}
	parts := strings.Split(dir, "/")
	return strings.Join(parts[max(0, len(parts)-n):], " / ")
}

// narrowed is the answer after the reader's kind and place have been applied,
// and the rows that offer the other choices.
type narrowed struct {
	results   []lexical.Result
	kinds     []pages.SearchChoice
	locations []pages.SearchChoice
}

// narrow applies the state's kind and place to every hit and counts what each
// row would hold: a kind row over the hits in the chosen place, a place row
// over the hits of the chosen kind, so no row promises a number the click
// does not deliver. Rows appear only when the whole answer is longer than a
// reader reads through.
func narrow(all []lexical.Result, st pages.SearchState, roles schema.NavigationRoles, lang wording.Lang) narrowed {
	out := narrowed{}
	inPlace := func(r *lexical.Result) bool {
		return st.In == "" || placeOf(r.RelPath, placeDepth(st.In)) == st.In
	}
	ofKind := func(r *lexical.Result) bool { return st.Kind == "" || kindOf(r, roles) == st.Kind }
	for i := range all {
		if inPlace(&all[i]) && ofKind(&all[i]) {
			out.results = append(out.results, all[i])
		}
	}
	if len(all) <= filterFloor {
		return out
	}

	byKind := map[string]int{}
	inPlaceCount := 0
	var ofKindResults []lexical.Result
	for i := range all {
		if inPlace(&all[i]) {
			byKind[kindOf(&all[i], roles)]++
			inPlaceCount++
		}
		if ofKind(&all[i]) {
			ofKindResults = append(ofKindResults, all[i])
		}
	}
	out.kinds = kindChoices(byKind, inPlaceCount, st, lang)
	out.locations = placeChoices(ofKindResults, st, lang)
	return out
}

func kindChoices(counts map[string]int, total int, st pages.SearchState, lang wording.Lang) []pages.SearchChoice {
	if len(counts) < 2 {
		return nil
	}
	kinds := make([]string, 0, len(counts))
	for k := range counts {
		kinds = append(kinds, k)
	}
	slices.SortFunc(kinds, func(a, b string) int {
		return cmp.Or(cmp.Compare(counts[b], counts[a]), cmp.Compare(slices.Index(kindOrder, a), slices.Index(kindOrder, b)))
	})
	next := func(kind string) string {
		s := st
		s.Kind, s.Shown = kind, 0
		return s.Address()
	}
	rows := []pages.SearchChoice{{Label: wording.SearchAll.In(lang), Count: total, Href: next(""), Active: st.Kind == ""}}
	for _, k := range kinds {
		rows = append(rows, pages.SearchChoice{Label: kindWord(k, lang), Count: counts[k], Href: next(k), Active: st.Kind == k})
	}
	return rows
}

// placeChoices lists where the hits live: by top folder, or by the folder
// below it when every hit shares one top folder, since a single row of choices
// narrows nothing.
func placeChoices(hits []lexical.Result, st pages.SearchState, lang wording.Lang) []pages.SearchChoice {
	depth := 1
	tops := map[string]bool{}
	for i := range hits {
		tops[placeOf(hits[i].RelPath, 1)] = true
	}
	if len(tops) == 1 {
		depth = 2
	}
	counts := map[string]int{}
	for i := range hits {
		place := placeOf(hits[i].RelPath, depth)
		if depth == 2 && placeDepth(place) < 2 {
			continue
		}
		counts[place]++
	}
	if len(counts) < 2 {
		return nil
	}
	places := make([]string, 0, len(counts))
	for p := range counts {
		places = append(places, p)
	}
	slices.SortFunc(places, func(a, b string) int {
		return cmp.Or(cmp.Compare(counts[b], counts[a]), cmp.Compare(a, b))
	})
	next := func(place string) string {
		s := st
		s.In, s.Shown = place, 0
		return s.Address()
	}
	rows := []pages.SearchChoice{{Label: wording.SearchAll.In(lang), Count: len(hits), Href: next(""), Active: st.In == ""}}
	for _, p := range places {
		label := path.Base(p)
		if p == rootPlace {
			label = wording.SearchTopLevel.In(lang)
		}
		rows = append(rows, pages.SearchChoice{Label: label, Count: counts[p], Href: next(p), Active: st.In == p})
	}
	return rows
}

// sortChoices are the two orders the answer can be read in.
func sortChoices(st pages.SearchState, lang wording.Lang) []pages.SearchChoice {
	by := func(sort string) string {
		s := st
		s.Sort, s.Shown = sort, 0
		return s.Address()
	}
	return []pages.SearchChoice{
		{Label: wording.SearchSortRelevance.In(lang), Count: -1, Href: by(""), Active: st.Sort == ""},
		{Label: wording.SearchSortRecent.In(lang), Count: -1, Href: by(pages.SearchSortRecent), Active: st.Sort == pages.SearchSortRecent},
	}
}
