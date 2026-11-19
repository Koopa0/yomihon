package inspect

import (
	"slices"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/wording"
)

// Outcome is what asking for one note comes to.
type Outcome uint8

const (
	// Found is a note this view can say.
	Found Outcome = iota
	// Missing is a path that names no note.
	Missing
	// Withheld is a path the filter does not let out. It is the same answer
	// whether or not a note is there, so asking cannot tell the two apart.
	Withheld
)

// Where a note's title came from.
const (
	FromHeading  = "heading"
	FromTitle    = "title"
	FromFileName = "file name"
)

// Course is a course a note belongs to, with the notes either side of it.
type Course struct {
	Title string `json:"title"`
	Path  string `json:"path"`
	Prev  *Entry `json:"prev,omitempty"`
	Next  *Entry `json:"next,omitempty"`
}

// Placement is an index that lists a note, under which headings.
type Placement struct {
	Title    string   `json:"title"`
	Path     string   `json:"path"`
	Headings []string `json:"headings"`
}

// Section is a heading of the note's own text.
type Section struct {
	Level int    `json:"level"`
	Text  string `json:"text"`
}

// Finding is something the render found wrong with a note: a link that goes
// nowhere, say, and the name it was written with.
type Finding struct {
	Kind   string `json:"kind"`
	Target string `json:"target,omitempty"`
}

// Note is how one note is read.
type Note struct {
	Path string `json:"path"`
	// Title is empty when the words of the title cannot be said.
	Title     string `json:"title,omitempty"`
	TitleFrom string `json:"title_from,omitempty"`
	Type      string `json:"type,omitempty"`
	Domain    string `json:"domain,omitempty"`
	Draft     bool   `json:"draft"`
	Language  string `json:"language,omitempty"`

	Courses   []Course    `json:"courses"`
	Indexes   []Placement `json:"indexes"`
	BasedOn   []Entry     `json:"based_on"`
	BasedOnBy []Entry     `json:"based_on_by"`
	LinksOut  []Entry     `json:"links_out"`
	LinksIn   []Entry     `json:"links_in"`
	Sections  []Section   `json:"sections"`
	Findings  []Finding   `json:"findings"`
}

// Note says how the note at path is read. path is vault-relative and
// canonical. The path is asked of the filter before anything is looked up, so
// that a path the filter withholds is answered alike whether or not a note is
// there.
func (v View) Note(path string) (Note, Outcome) {
	if !v.allows(path) {
		return Note{}, Withheld
	}
	reading, ok := v.gen.Note(path)
	if !ok || !v.readable(path) {
		return Note{}, Missing
	}
	out := Note{
		Path:      path,
		Type:      reading.Type,
		Domain:    reading.Domain,
		Draft:     reading.Draft,
		Language:  reading.Language,
		Courses:   []Course{},
		Indexes:   []Placement{},
		BasedOn:   []Entry{},
		BasedOnBy: []Entry{},
		LinksOut:  []Entry{},
		LinksIn:   []Entry{},
		Sections:  []Section{},
		Findings:  []Finding{},
	}
	if !v.spells(reading.Title, reading.Body) {
		out.Title, out.TitleFrom = reading.Title, v.titleFrom(path, reading.Body)
	}
	navigation := v.gen.Navigation()
	for _, neighbors := range navigation.PathNeighbors(path) {
		if !v.nameable(neighbors.PathRelPath, neighbors.PathTitle) {
			continue
		}
		out.Courses = append(out.Courses, Course{
			Title: neighbors.PathTitle, Path: neighbors.PathRelPath,
			Prev: v.entry(neighbors.Prev), Next: v.entry(neighbors.Next),
		})
	}
	for _, placement := range navigation.Placements(path) {
		if placed, ok := v.placement(navigation, placement); ok {
			out.Indexes = append(out.Indexes, placed)
		}
	}
	out.BasedOn = v.entries(v.gen.BasedOn(path))
	out.BasedOnBy = v.entries(v.gen.BasedOnBy(path))
	out.LinksIn = v.entries(v.gen.CitedBy(path))
	out.LinksOut = v.linksOut(reading.Body)
	for _, heading := range graph.Headings(reading.Body, graph.LineSkipZones(reading.Body)) {
		text := graph.PlainHeadingWords(heading.Text)
		if !v.spells(heading.Text, reading.Body) && !v.spells(text, reading.Body) {
			out.Sections = append(out.Sections, Section{Level: heading.Level, Text: text})
		}
	}
	for _, diagnostic := range v.gen.Render(path, reading.Body, wording.En).Diagnostics {
		if !v.spells(diagnostic.Target, reading.Body) {
			out.Findings = append(out.Findings, Finding{Kind: string(diagnostic.Kind), Target: diagnostic.Target})
		}
	}
	return out, Found
}

func (v View) titleFrom(path, body string) string {
	if _, ok := graph.FirstH1(body); ok {
		return FromHeading
	}
	if v.gen.DeclaredTitle(path) != "" {
		return FromTitle
	}
	return FromFileName
}

// entry is a note's neighbour as an entry, or nil when there is none or it
// cannot be named.
func (v View) entry(ref nav.NoteRef) *Entry {
	if ref.RelPath == "" || !v.nameable(ref.RelPath, ref.Name) {
		return nil
	}
	return &Entry{Title: ref.Name, Path: ref.RelPath}
}

func (v View) entries(refs []nav.NoteRef) []Entry {
	out := []Entry{}
	for _, ref := range refs {
		if entry := v.entry(ref); entry != nil {
			out = append(out, *entry)
		}
	}
	slices.SortFunc(out, compareEntries)
	return out
}

// placement is an index that lists a note, left out whole when the index or
// any heading it was listed under cannot be named.
func (v View) placement(navigation *nav.Model, placed nav.Placement) (Placement, bool) {
	index := navigation.Map(placed.MapRelPath)
	if index == nil || !v.nameable(index.RelPath, index.Title) {
		return Placement{}, false
	}
	headings := []string{}
	for _, heading := range placed.Headings {
		if v.spells(heading, v.bodyOf(index.RelPath)) {
			return Placement{}, false
		}
		headings = append(headings, heading)
	}
	return Placement{Title: index.Title, Path: index.RelPath, Headings: headings}, true
}

// linksOut is the notes a body links to by name, each once, that can be said.
func (v View) linksOut(body string) []Entry {
	seen := map[string]bool{}
	out := []Entry{}
	index := v.gen.Graph()
	for _, target := range judge.LinkTargets(body) {
		resolved := index.Resolve(target)
		if resolved.Kind != graph.KindUnique || seen[resolved.RelPath] {
			continue
		}
		seen[resolved.RelPath] = true
		reading, ok := v.gen.Note(resolved.RelPath)
		if ok && v.nameable(resolved.RelPath, reading.Title) {
			out = append(out, Entry{Title: reading.Title, Path: resolved.RelPath})
		}
	}
	slices.SortFunc(out, compareEntries)
	return out
}
