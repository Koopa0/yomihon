package egress

import (
	"html"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"github.com/koopa0/yomihon/internal/graph"
)

// Scan is what a walk left out by its own rules, which the rules file's
// boundary cannot know: the vault's .gitignore and the built-in skips.
type Scan interface {
	OutsideScan(rel string) bool
}

// Names resolves a link's target to the notes it could reach. NameWithin
// reports whether every note the name could be is in ground allowed lets out,
// so a name that could reach withheld ground is not within.
type Names interface {
	Scan
	NameWithin(target string, allowed func(rel string) bool) bool
}

// Readable is the gate every path passes before anything about it is resolved
// into an output: the privacy declaration lets it out and the scan looks
// there. scan answers for the last, since it knows what its walk left out as
// well as what the rules file excludes.
func (g *Gate) Readable(scan Scan) func(rel string) bool {
	return func(rel string) bool {
		return g.Allows(rel) && !scan.OutsideScan(rel)
	}
}

// Spells reports whether text cites, or spells a link name body cites, that
// could reach withheld ground. A heading or a title is the author's words, and
// a wikilink inside one is shown as the name it was written with, so a heading
// citing a withheld note would say that note's path in the output, and its
// anchor, made from the same words, would say it again. A title is its note's
// first heading as written, so the links it writes are read from the title
// itself; a marked location's heading arrives rendered, so its links are found
// in body.
//
// The links are read with the extractor the gate was opened with and with the
// plain-text reader below, each catching what the other does not.
func (g *Gate) Spells(text, body string, names Names) bool {
	if spellsWithheldDir(text, g.withheld) {
		return true
	}
	readable := g.Readable(names)
	targets := slices.Concat(Targets(g.links(text)), plainTargets(text))
	if slices.ContainsFunc(targets, func(target string) bool { return !names.NameWithin(target, readable) }) {
		return true
	}
	folded := graph.FoldFragment(text)
	for _, target := range Targets(g.links(body)) {
		name := graph.FoldFragment(strings.TrimSpace(target))
		if name != "" && strings.Contains(folded, name) && !names.NameWithin(target, readable) {
			return true
		}
	}
	return false
}

// spellsWithheldDir reports whether text, read past the ways it can be
// disguised, spells a withheld directory as a name followed by a slash: at
// the start of the text or after a character that is neither a letter nor a
// digit, so that "AI/" is matched and "OpenAI/" is not. This holds whatever
// syntax the text is written in, so a title naming withheld ground in plain
// words is withheld with the rest. A withheld note's full path contains its
// directory and a slash, so this covers every such path too. A note's bare
// file name is not matched: the author's own words that happen to share one
// are listed as written. The boundary is judged on the folded text, after
// what draws nothing is gone, so an invisible letter cannot stand in front of
// the name.
func spellsWithheldDir(text string, withheldDirs []string) bool {
	folded := foldDisguises(text)
	for _, dir := range withheldDirs {
		name := foldDisguises(dir) + "/"
		for from := 0; ; {
			at := strings.Index(folded[from:], name)
			if at < 0 {
				break
			}
			at += from
			before, _ := utf8.DecodeLastRuneInString(folded[:at])
			if at == 0 || !unicode.IsLetter(before) && !unicode.IsNumber(before) {
				return true
			}
			from = at + 1
		}
	}
	return false
}

// foldDisguises reads text past what would hide a word from a comparison:
// HTML character references and percent-escapes, decoded until nothing more
// decodes; characters that draw nothing — format characters, variation
// selectors, other default-ignorable code points — dropped before the text
// is composed and again after; compatibility forms, such as fullwidth letters,
// made plain by NFKC; combining marks left after that dropped; a backslash
// and the solidus's look-alikes read as a slash; and case folded. A letter of
// another script that only looks like a Latin one is not read as that letter.
func foldDisguises(text string) string {
	for range 4 {
		decoded := percentDecode(html.UnescapeString(text))
		if decoded == text {
			break
		}
		text = decoded
	}
	text = norm.NFKC.String(strings.Map(dropDrawsNothing, text))
	return strings.Map(func(r rune) rune {
		switch {
		case unicode.Is(unicode.Mn, r):
			return -1
		case r == '\\', r == 0x2044, r == 0x2215, r == 0x2571, r == 0x29f8:
			return '/'
		}
		return foldRune(dropDrawsNothing(r))
	}, text)
}

// dropDrawsNothing drops a character that draws nothing where it stands.
func dropDrawsNothing(r rune) rune {
	if r < 0 || unicode.In(r, unicode.Cf, unicode.Variation_Selector, unicode.Other_Default_Ignorable_Code_Point) {
		return -1
	}
	return r
}

// percentDecode replaces each %XX escape in text with the byte it names,
// leaving any % that does not begin one as it stands.
func percentDecode(text string) string {
	if !strings.Contains(text, "%") {
		return text
	}
	var b strings.Builder
	for i := 0; i < len(text); i++ {
		if text[i] == '%' && i+2 < len(text) {
			if value, err := strconv.ParseUint(text[i+1:i+3], 16, 8); err == nil {
				b.WriteByte(byte(value))
				i += 2
				continue
			}
		}
		b.WriteByte(text[i])
	}
	return b.String()
}

// foldRune is the smallest rune of r's case-folding orbit, so two strings
// strings.EqualFold calls equal map to the same runes.
func foldRune(r rune) rune {
	smallest := r
	for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
		smallest = min(smallest, f)
	}
	return smallest
}
