package egress

import (
	"path"
	"regexp"
	"slices"
	"strings"
)

// Masked stands where Body removed a link. One token for every link, whatever it
// addressed and however long it was, so that the output says something was taken
// out and nothing about what: not the name, not the display text, not whether it
// was an embed or an image.
const Masked = "⟦withheld⟧"

// Body is body with every link that could reach withheld ground replaced by
// Masked, and every other byte as it was. A link is anything a page renders as
// one, or would if the author had not quoted it: a wikilink with its alias and
// heading or block part, an embed, an inline link with its text, an image, a
// reference link and its definition, an autolink, and an href or src with the
// words an anchor shows. The whole link goes, display text included, since a
// display text can itself spell the name.
//
// What could reach withheld ground is the question Spells asks of a title:
// names.NameWithin is false for the target, or the target spells a withheld
// folder. The links are read twice, with the extractor the gate was opened with
// and as plain text, since each finds what the other does not: the extractor
// reads the page's links and the plain reader reads the ones a code span, a
// comment or a backslash keeps the page from rendering, which are still bytes in
// the output.
//
// rel is the path of the note the body is from, which a link to a file means
// something relative to: ../Notes/A.md is a note in a folder beside this one's,
// and is read as that note and not as a path that climbs out of the vault.
//
// places is how many times Masked was written, which the caller says before the
// body: a note that itself contains the token is told from a masked one by that
// count, which counting the token in the output could not do.
//
// A withheld note named in prose with no link around it is not masked. What
// this keeps out of the output is a withheld note's content and its address, and
// the address is what a link writes.
func (g *Gate) Body(rel, body string, names Names) (masked string, places int) {
	readable := g.Readable(names)
	dir := path.Dir(rel)
	reaches := func(link plainLink) bool {
		return slices.ContainsFunc(link.targets, func(target string) bool {
			if link.path {
				target = fromNote(dir, target)
			}
			return !names.NameWithin(target, readable)
		}) || slices.ContainsFunc(link.written, func(address string) bool { return spellsWithheldDir(address, g.withheld) })
	}
	var spans [][2]int
	for _, link := range g.links(body) {
		if !reaches(plainLink{targets: []string{link.Target}, written: []string{link.Target}}) {
			continue
		}
		if link.Start < 0 || link.End > len(body) || link.Start >= link.End {
			// A link that cannot be placed cannot be masked in place, and what
			// it addressed must not be shown.
			return Masked, 1
		}
		spans = append(spans, [2]int{link.Start, link.End})
	}
	for _, link := range plainLinks(body) {
		if reaches(link) {
			spans = append(spans, [2]int{link.start, link.end})
		}
	}
	return mask(body, spans)
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

// scheme is the start of an address with a scheme of its own.
var scheme = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.\-]*:`)

// fromNote is a file destination as a vault path: one that starts at the root,
// or that climbs, is resolved from the folder dir of the note that wrote it. A
// destination that climbs out of the vault is left as written, which no path
// the gate lets out is, and one with a scheme of its own is not a file here.
func fromNote(dir, target string) string {
	switch {
	case scheme.MatchString(target):
		return target
	case strings.HasPrefix(target, "/"):
		return strings.TrimLeft(target, "/")
	case strings.HasPrefix(target, "./"), strings.HasPrefix(target, "../"), strings.Contains(target, "/../"):
		joined := path.Join(dir, target)
		if joined == ".." || strings.HasPrefix(joined, "../") {
			return target
		}
		return joined
	}
	return target
}
