// Package inspect is how yomihon says it reads a vault and a note: what is in
// the vault, and for one note its title and where it came from, its place in
// the courses and indexes, what it is based on and what points at it, its
// sections and what the render found wrong with it.
//
// The command and the page are meant to say the same things, so both read
// this one structure rather than each working it out. A View is made in one of
// two ways. ForReader keeps everything, for the reader's own page. ForAgent
// keeps what an egress gate lets out, for a program: there is no View without
// one or the other, so no caller can leave the filter off by leaving it out.
package inspect

import (
	"slices"
	"strings"

	"github.com/koopa0/yomihon/internal/egress"
	"github.com/koopa0/yomihon/internal/snapshot"
)

// Gate is what ForAgent asks of the egress gate: may this path be named, and
// does this text name ground that may not be. *egress.Gate is the production
// implementation, and a View built from it is the agent-facing one.
type Gate interface {
	Allows(rel string) bool
	WithoutRules() bool
	Readable(scan egress.Scan) func(rel string) bool
	Spells(text, body string, names egress.Names) bool
}

// View is one generation read through one filter.
type View struct {
	gen *snapshot.Generation
	// noRules is whether the folder has no rules file, which the gate is the
	// authority on for a program.
	noRules  bool
	allows   func(rel string) bool
	readable func(rel string) bool
	spells   func(text, body string) bool
}

// ForReader is the whole generation, for the reader's own page. What a rules
// file withholds from a program is not withheld from the person whose room
// this is.
func ForReader(gen *snapshot.Generation) View {
	return View{
		gen:      gen,
		noRules:  gen.PrivacyPolicy().WithoutRules(),
		allows:   func(string) bool { return true },
		readable: func(string) bool { return true },
		spells:   func(string, string) bool { return false },
	}
}

// ForAgent is the generation as far as gate lets it out. Every path it names
// passes Readable, and every title, heading and name passes Spells, asked of
// the body it was read from: a title that is the plain words of a heading
// which linked to withheld ground names it no longer as a link, and the body
// is where that link is found.
func ForAgent(gen *snapshot.Generation, gate Gate) View {
	return View{
		gen:      gen,
		noRules:  gate.WithoutRules(),
		allows:   gate.Allows,
		readable: gate.Readable(gen),
		spells:   func(text, body string) bool { return gate.Spells(text, body, gen) },
	}
}

// Entry is a note named by its title and its path.
type Entry struct {
	Title string `json:"title"`
	Path  string `json:"path"`
}

// Skip is a path the scan saw and did not read, and why.
type Skip struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// Vault is how a vault is read.
type Vault struct {
	// NoRules is whether the folder has no rules file.
	NoRules bool `json:"no_rules"`
	// Notes is how many notes can be named.
	Notes int `json:"notes"`
	// Courses and Indexes are the ones that can be named, by title and path.
	Courses []Entry `json:"courses"`
	Indexes []Entry `json:"indexes"`
	// Skipped is what the scan passed over.
	Skipped []Skip `json:"skipped"`
}

// Vault says how the whole vault is read. A note that cannot be named is in
// no count and no list.
func (v View) Vault() Vault {
	out := Vault{
		NoRules: v.noRules,
		Courses: []Entry{},
		Indexes: []Entry{},
		Skipped: []Skip{},
	}
	for _, file := range v.gen.Files() {
		path := file.Path()
		if strings.HasSuffix(path, ".md") && !v.gen.SkipsNote(path) && v.readable(path) {
			out.Notes++
		}
	}
	navigation := v.gen.Navigation()
	for _, course := range navigation.Paths() {
		if v.nameable(course.RelPath, course.Title) {
			out.Courses = append(out.Courses, Entry{Title: course.Title, Path: course.RelPath})
		}
	}
	for _, index := range navigation.Maps() {
		if v.nameable(index.RelPath, index.Title) {
			out.Indexes = append(out.Indexes, Entry{Title: index.Title, Path: index.RelPath})
		}
	}
	for _, skipped := range v.gen.Skipped() {
		if v.allows(skipped.Path) && !v.spells(skipped.Path, "") {
			out.Skipped = append(out.Skipped, Skip{Path: skipped.Path, Reason: skipped.Reason})
		}
	}
	slices.SortFunc(out.Courses, compareEntries)
	slices.SortFunc(out.Indexes, compareEntries)
	return out
}

func compareEntries(a, b Entry) int {
	if c := strings.Compare(a.Path, b.Path); c != 0 {
		return c
	}
	return strings.Compare(a.Title, b.Title)
}

// nameable is whether a note can be said by its path and its title, the title
// asked of the body of the note it belongs to.
func (v View) nameable(path, title string) bool {
	return v.readable(path) && !v.spells(title, v.bodyOf(path))
}

// bodyOf is the text of the note at path, empty for a path that is not a note.
func (v View) bodyOf(path string) string {
	reading, _ := v.gen.Note(path)
	return reading.Body
}
