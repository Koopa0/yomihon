package egress

import (
	"errors"
	"slices"
	"strings"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/vault"
)

// Masked stands where Body removed a link. One token for every link, whatever it
// addressed and however long it was, so that the output says something was taken
// out and nothing about what: not the name, not the display text, not whether it
// was an embed or an image.
const Masked = "⟦withheld⟧"

// ErrWithheldNote is the refusal to give the body of a note the gate does not
// let out. The note's path is asked of the gate before anything is read from
// the body, so it comes the same whether or not the note exists.
var ErrWithheldNote = errors.New("this note lies under ground the rules file withholds from agent-facing output")

// ErrFrontmatter is the refusal for a body that still opens with its note's
// frontmatter block. A body is the text after that block; the fields it held
// leave through the gate one by one, and the block is not text to mask.
var ErrFrontmatter = errors.New("a body is a note's text after its frontmatter block, and this one still has the block")

// Body is body with every link that could reach withheld ground replaced by
// Masked, and every other byte as it was. A link is what the page's own parser
// reads as one: an inline link or an image with its text, alt text and title, a
// reference link, a reference definition wherever the parser found it, an
// autolink, and a wikilink or an embed with its alias and its heading or block
// part. The whole node goes, so nothing is left beside the token.
//
// The links are found by the parser the page renders with, which says where
// each one begins and ends, and not by a reading of Markdown of this package's
// own; a link a code span or a code fence quotes is not one to that parser and
// stays as the text it is. A wikilink is found by the extractor the gate was
// opened with. Raw HTML is not read for its attributes: an element or a comment
// whose text spells a withheld folder, in any of the forms Spells knows, goes
// whole.
//
// Masking one link can make another: the token that replaces a wikilink inside
// an embed leaves an embed the extractor reads. And the page parses a body only
// after it has taken its comments off and its own runes out, which can join a
// link that was apart in the text as written, such as [[Sec%%x%%ret Plan]]. So
// what was masked is read again, as it stands and as the page reads it
// (render.PageSource), and when either reading still finds a link into withheld
// ground, the link has no place in body to mask and the whole body is Masked.
//
// What could reach withheld ground is decided as the page decides where a link
// goes. A Markdown destination is resolved from the note that wrote it with the
// function the page resolves with, so it is masked whether or not anything is
// there; a wikilink's target is asked of names, which are those of the generation the
// output is read from, as Spells asks.
//
// body is the text after the note's frontmatter block, and a body that still
// opens with one is refused, since a block is not text a mask can be laid over.
// The refusal for a note the gate does not let out is ErrWithheldNote.
//
// places is how many times Masked was written, which the caller says before the
// body: a note that itself contains the token is told from a masked one by that
// count, which counting the token in the output could not do.
//
// A withheld note named in prose with no link around it is not masked, nor is a
// link inside code. What this keeps out of the output is a withheld note's
// content and its address, and the address is what a link writes.
func (g *Gate) Body(rel, body string, names Names) (masked string, places int, err error) {
	readable := g.Readable(names)
	if !readable(rel) {
		return "", 0, ErrWithheldNote
	}
	if _, has := vault.SplitFrontmatter([]byte(body)); has {
		return "", 0, ErrFrontmatter
	}
	spans, placed := g.reachingSpans(rel, body, names, readable)
	if !placed {
		// A link that cannot be placed cannot be masked in place, and what it
		// addressed must not be shown.
		return Masked, 1, nil
	}
	masked, places = mask(body, spans)
	if g.stillReaches(rel, masked, names, readable) {
		// What is left still links into withheld ground, and that link has no
		// place in body to mask, so nothing of it is shown.
		return Masked, 1, nil
	}
	return masked, places, nil
}

// stillReaches is whether masked, read as it stands and as the page parses it,
// still has a link into withheld ground. Masking one link can make another:
// a wikilink inside an embed keeps the extractor from reading the embed, and
// the token in its place leaves one it reads. The page also parses the text
// only after its comments come off and its own runes come out, which can join
// a link that was apart. Either way the link is new in masked, so it is read
// for in masked and not in body.
func (g *Gate) stillReaches(rel, masked string, names Names, readable func(string) bool) bool {
	for _, text := range []string{masked, render.PageSource(masked)} {
		if left, placed := g.reachingSpans(rel, text, names, readable); !placed || len(left) > 0 {
			return true
		}
	}
	return false
}

// reachingSpans is where in body a link goes into withheld ground, as byte
// spans: the wikilinks the gate's extractor finds, and what the page's parser
// reads as a link, an image, a definition or raw HTML. It is false when a link
// is reported at a place the body does not have.
func (g *Gate) reachingSpans(rel, body string, names Names, readable func(string) bool) (spans [][2]int, placed bool) {
	add := func(start, end int) bool {
		if start < 0 || end > len(body) || start >= end {
			return false
		}
		spans = append(spans, [2]int{start, end})
		return true
	}
	tails := tailSpellings(g.withheld)
	for _, link := range g.links(body) {
		if names.NameWithin(link.Target, readable) && !spellsWithheldDir(link.Target, tails) {
			continue
		}
		if !add(link.Start, link.End) {
			return nil, false
		}
	}
	for _, link := range render.SourceLinks(body) {
		if link.HTML {
			if !spellsWithheldDir(body[min(max(link.Start, 0), len(body)):min(max(link.End, 0), len(body))], tails) {
				continue
			}
		} else if !g.reaches(rel, link.Destination, readable) {
			continue
		}
		if !add(link.Start, link.End) {
			return nil, false
		}
	}
	return spans, true
}

// reaches is whether a destination written in the note rel goes into withheld
// ground. The page's own resolver says so for a path from the note; a backslash
// read as a slash, which is how a browser takes one, is asked the same way. A
// path spelled from the vault's root, with or without a leading slash, is the
// full path of a folder and is the spelling check's.
func (g *Gate) reaches(rel, destination string, readable func(string) bool) bool {
	if spellsWithheldDir(destination, g.withheld) {
		return true
	}
	absent := func(string) bool { return false }
	for _, spelling := range []struct{ from, address string }{
		{rel, destination},
		{rel, strings.ReplaceAll(destination, `\`, "/")},
	} {
		if graph.ResolveMarkdown(spelling.from, spelling.address, readable, absent).Withheld {
			return true
		}
	}
	return false
}

// tailSpellings is every way an address written from a folder inside the vault
// can spell a withheld folder: its path from the root and each tail of it, so
// that Notes/Private is also Private to a note in Notes.
func tailSpellings(dirs []string) []string {
	var out []string
	for _, dir := range dirs {
		parts := strings.Split(dir, "/")
		for i := range parts {
			out = append(out, strings.Join(parts[i:], "/"))
		}
	}
	return out
}

// mask replaces each span of body with Masked, and says how many it wrote. Spans
// that overlap are one link read twice, or a link inside another's text, and are
// one mask; spans that only touch stay two.
func mask(body string, spans [][2]int) (masked string, places int) {
	if len(spans) == 0 {
		return body, 0
	}
	slices.SortFunc(spans, func(a, b [2]int) int {
		if a[0] != b[0] {
			return a[0] - b[0]
		}
		return b[1] - a[1]
	})
	var out strings.Builder
	out.Grow(len(body))
	at := 0
	for i := 0; i < len(spans); {
		start, end := spans[i][0], spans[i][1]
		for i++; i < len(spans) && spans[i][0] < end; i++ {
			end = max(end, spans[i][1])
		}
		out.WriteString(body[at:start])
		out.WriteString(Masked)
		places++
		at = end
	}
	out.WriteString(body[at:])
	return out.String(), places
}
