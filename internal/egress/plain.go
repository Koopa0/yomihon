package egress

import (
	"net/url"
	"regexp"
	"slices"
	"strings"
)

// plainLink is one link read out of text as plain text: where its bytes are,
// every name or path it can be taken as addressing, and the address as it was
// written, which is what a withheld folder's name is looked for in.
type plainLink struct {
	start, end int
	targets    []string
	written    []string
	// path is whether targets are file destinations, which mean something
	// relative to the note that wrote them, and not names.
	path bool
}

// plainLinks reads the links a piece of text spells, as plain text rather than
// as Markdown: a title is written out as it stands, and a body is shown as it
// stands, so a code span or a backslash that keeps a page from rendering a link
// keeps nothing out of the list. It reads each [[…]] and ![[…]]; each inline
// link and image ](…), with the link text before it; each reference definition
// and each use of one; each autolink; and each href and src. A link written
// across lines is not one, as no renderer reads it as one.
//
// Spells asks what these address, and Body masks where they sit, so the two
// read one grammar.
func plainLinks(text string) []plainLink {
	var links []plainLink
	links = appendWikilinks(links, text)
	links = appendInlineLinks(links, text)
	links = appendReferences(links, text)
	links = appendAutolinks(links, text)
	links = appendHTMLLinks(links, text)
	return links
}

// plainTargets is every name or path the links of text address, as Spells asks.
func plainTargets(text string) []string {
	var targets []string
	for _, link := range plainLinks(text) {
		targets = append(targets, link.targets...)
	}
	return targets
}

func appendWikilinks(links []plainLink, text string) []plainLink {
	for from := 0; ; {
		at := strings.Index(text[from:], "[[")
		if at < 0 {
			return links
		}
		open := from + at
		end := lineEnd(text, open)
		closeAt := strings.Index(text[open+len("[["):end], "]]")
		if closeAt < 0 {
			// No "]]" on the rest of the line closes this "[[" or any after it.
			from = end
			continue
		}
		closing := open + len("[[") + closeAt
		if last := strings.LastIndex(text[open+len("[["):closing], "[["); last >= 0 {
			// The link opens at the last "[[" before the "]]".
			open += len("[[") + last
		}
		inner := text[open+len("[[") : closing]
		// A table cell writes the pipe as \|, and the backslash is not part of the name.
		name, _, _ := strings.Cut(inner, "|")
		name = strings.TrimRight(name, `\`)
		written := name
		name, _, _ = strings.Cut(name, "#")
		name, _, _ = strings.Cut(name, "^")
		name = strings.TrimSpace(name)
		start := open
		if start > 0 && text[start-1] == '!' {
			start--
		}
		links = append(links, plainLink{start: start, end: closing + len("]]"), targets: []string{name}, written: []string{written}})
		from = closing + len("]]")
	}
}

// appendInlineLinks reads each ](…) with the link text that opens it, in one
// pass: a "[" is remembered until the "]" that closes it, and a "]" followed by
// "(" is a link whose text begins at the "[" it closes. What a link's
// destination takes is not read again for brackets of its own.
func appendInlineLinks(links []plainLink, text string) []plainLink {
	var opens []int
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '\\':
			i++
		case '\n':
			opens = opens[:0]
		case '[':
			opens = append(opens, i)
		case ']':
			start := i
			if n := len(opens); n > 0 {
				start = opens[n-1]
				opens = opens[:n-1]
				if start > 0 && text[start-1] == '!' {
					start--
				}
			}
			if i+1 >= len(text) || text[i+1] != '(' {
				continue
			}
			destination, end := inlineLink(text, i+len("]("))
			links = append(links, plainLink{start: start, end: end, targets: destinationTargets(destination), written: []string{destination}, path: true})
			i = end - 1
		}
	}
	return links
}

// referenceDefinition is a line [label]: destination, up to three spaces in.
var referenceDefinition = regexp.MustCompile(`(?m)^( {0,3})\[([^\]\n]+)\]:[ \t]*(.*)$`)

// referenceUse is a bracketed run, optionally followed by a second: [text][label],
// [label][] and [label] are the three ways a link names its definition.
var referenceUse = regexp.MustCompile(`\[([^\[\]\n]*)\](?:\[([^\[\]\n]*)\])?`)

func appendReferences(links []plainLink, text string) []plainLink {
	defined := map[string]plainLink{}
	var lines [][2]int
	for _, match := range referenceDefinition.FindAllStringSubmatchIndex(text, -1) {
		destination, _ := linkDestination(text[match[6]:match[7]])
		link := plainLink{start: match[0] + match[3] - match[2], end: match[1], targets: destinationTargets(destination), written: []string{destination}, path: true}
		links = append(links, link)
		defined[referenceLabel(text[match[4]:match[5]])] = link
		lines = append(lines, [2]int{match[0], match[1]})
	}
	if len(defined) == 0 {
		return links
	}
	for _, match := range referenceUse.FindAllStringSubmatchIndex(text, -1) {
		if slices.ContainsFunc(lines, func(line [2]int) bool { return match[0] >= line[0] && match[0] < line[1] }) ||
			(match[1] < len(text) && text[match[1]] == '(') {
			// The definition's own brackets, or an inline link's text.
			continue
		}
		label := text[match[2]:match[3]]
		if match[4] >= 0 && match[5] > match[4] {
			label = text[match[4]:match[5]]
		}
		definition, ok := defined[referenceLabel(label)]
		if !ok {
			continue
		}
		start := match[0]
		if start > 0 && text[start-1] == '!' {
			start--
		}
		links = append(links, plainLink{start: start, end: match[1], targets: definition.targets, written: definition.written, path: true})
	}
	return links
}

// referenceLabel is a link label as a definition and its uses compare it: its
// spaces collapsed and read past what hides a word from a comparison.
func referenceLabel(label string) string {
	return foldDisguises(strings.Join(strings.Fields(label), " "))
}

// autolink is a URI in angle brackets.
var autolink = regexp.MustCompile(`<([A-Za-z][A-Za-z0-9+.\-]{1,31}:[^\s<>]*)>`)

func appendAutolinks(links []plainLink, text string) []plainLink {
	for _, match := range autolink.FindAllStringSubmatchIndex(text, -1) {
		address := text[match[2]:match[3]]
		links = append(links, plainLink{start: match[0], end: match[1], targets: destinationTargets(address), written: []string{address}})
	}
	return links
}

// htmlLinkAttribute is an href or src attribute's value, double-, single- or
// unquoted.
var htmlLinkAttribute = regexp.MustCompile(`(?i)\b(?:href|src)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'>]+))`)

// anchorClose ends an <a> element.
var anchorClose = regexp.MustCompile(`(?i)</a\s*>`)

// longestTag is how far back from an attribute its tag is looked for. A tag
// longer than this is not one a note writes.
const longestTag = 4096

func appendHTMLLinks(links []plainLink, text string) []plainLink {
	matches := htmlLinkAttribute.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return links
	}
	closers := anchorClose.FindAllStringIndex(text, -1)
	for _, match := range matches {
		var value string
		for group := 1; group <= 3; group++ {
			if match[2*group] >= 0 {
				value = text[match[2*group]:match[2*group+1]]
			}
		}
		start, end := htmlTag(text, match[0], match[1], closers)
		links = append(links, plainLink{start: start, end: end, targets: destinationTargets(value), written: []string{value}, path: true})
	}
	return links
}

// htmlTag is the tag an attribute at from…to belongs to and, for an anchor, the
// text it holds and its closing tag: the words an anchor displays are the link's
// own, so they go with it. closers are the offsets of every </a> in text.
func htmlTag(text string, from, to int, closers [][]int) (start, end int) {
	open := strings.LastIndexByte(text[max(0, from-longestTag):from], '<')
	if open < 0 {
		return from, to
	}
	open += max(0, from-longestTag)
	if strings.IndexByte(text[open:from], '>') >= 0 {
		return from, to
	}
	end = to
	if gt := strings.IndexByte(text[to:min(len(text), to+longestTag)], '>'); gt >= 0 {
		end = to + gt + 1
	}
	if name := text[open+1 : min(len(text), open+3)]; len(name) == 2 && (name[0] == 'a' || name[0] == 'A') && strings.ContainsRune(" \t\r\n>", rune(name[1])) {
		if at, _ := slices.BinarySearchFunc(closers, end, func(closer []int, target int) int { return closer[0] - target }); at < len(closers) {
			end = closers[at][1]
		}
	}
	return open, end
}

// destinationTargets is each name or path a link destination can be taken as: as
// written, without its fragment or query, and percent-decoded. One that climbs
// out of its folder needs no resolving here: NameWithin's gate refuses any path
// that climbs.
func destinationTargets(destination string) []string {
	destination, _, _ = strings.Cut(destination, "#")
	destination, _, _ = strings.Cut(destination, "?")
	targets := []string{destination}
	if decoded, err := url.PathUnescape(destination); err == nil {
		targets = append(targets, decoded)
	}
	return targets
}

// inlineLink is the destination of the inline link whose "(" has just been read,
// at from, and where the link ends: past the ")" that closes it when it has one
// on its line, after an optional title, else at the end of the destination.
func inlineLink(text string, from int) (destination string, end int) {
	destination, used := linkDestination(text[from:])
	end = from + used
	i := skipBlanks(text, end)
	if i < len(text) {
		var closer byte
		switch text[i] {
		case '"', '\'':
			closer = text[i]
		case '(':
			closer = ')'
		}
		if closer != 0 {
			window := text[i+1 : lineEnd(text, i+1)]
			if at := strings.IndexByte(window, closer); at >= 0 {
				i = skipBlanks(text, i+1+at+1)
			}
		}
	}
	if i < len(text) && text[i] == ')' {
		return destination, i + 1
	}
	return destination, end
}

func skipBlanks(text string, from int) int {
	for from < len(text) && (text[from] == ' ' || text[from] == '\t') {
		from++
	}
	return from
}

// lineEnd is the offset of the end of the line that from is on.
func lineEnd(text string, from int) int {
	if at := strings.IndexByte(text[from:], '\n'); at >= 0 {
		return from + at
	}
	return len(text)
}

// linkDestination is the link destination that starts text, by CommonMark's
// rule: after any leading space, a destination opened by < runs to the >
// that closes it, spaces included; any other runs to the first space or
// control character, or to a ) that closes no ( it opened. A backslash escapes
// the character after it. used is how much of text the destination and the
// space before it take.
func linkDestination(text string) (destination string, used int) {
	trimmed := strings.TrimLeft(text, " \t\n")
	lead := len(text) - len(trimmed)
	var b strings.Builder
	if rest, angled := strings.CutPrefix(trimmed, "<"); angled {
		for i := 0; i < len(rest); i++ {
			switch c := rest[i]; {
			case c == '\\' && i+1 < len(rest):
				i++
				b.WriteByte(rest[i])
			case c == '>':
				return b.String(), lead + 1 + i + 1
			case c == '\n':
				return b.String(), lead + 1 + i
			default:
				b.WriteByte(c)
			}
		}
		return b.String(), len(text)
	}
	depth := 0
	for i := 0; i < len(trimmed); i++ {
		switch c := trimmed[i]; {
		case c == '\\' && i+1 < len(trimmed):
			i++
			b.WriteByte(trimmed[i])
		case c <= ' ' || c == 0x7f:
			return b.String(), lead + i
		case c == '(':
			depth++
			b.WriteByte(c)
		case c == ')':
			if depth == 0 {
				return b.String(), lead + i
			}
			depth--
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
	}
	return b.String(), len(text)
}
