package judge

import (
	"slices"
	"strings"
	"unicode"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/vault"
)

// The diagnostics extract [[wikilinks]] and file references from a note body
// with the same discipline the vault's live linker uses: a markdown parser
// locates the structures a bracket must not be read inside, while the brackets
// themselves are scanned from the raw text so link parsing never mangles them.
// Line numbers count from the top of the file, so a note with an N-line
// frontmatter block reports its first body line as line N+1.

// wikiLink is one [[target]] occurrence. offset is what makes it an occurrence
// rather than a name: one line can hold several links and one name can be
// written on several lines, so a rule says which it means with the offset.
//
// address, heading and block carry the fragment half, read the way the reading
// page reads it — heading after "#", block after "^" or "#^", address the whole
// addressing text without a display alias. target keeps the frozen stripping;
// the fragment split is the resolver's, so the two faces cannot disagree.
type wikiLink struct {
	target          string
	address         string
	heading         string
	block           string
	embed           bool
	offset          int
	line            int
	underGapHeading bool
}

// pathRef is one file reference that is not a wikilink: a markdown [text](path)
// link (resolved relative to the citing note) or a backticked path token
// (resolved relative to the vault root). code distinguishes the two.
type pathRef struct {
	target string
	line   int
	code   bool
}

// mdParser is the shared markdown parser. It is plain CommonMark with no
// extensions, matching the structure the vault's linker sees, so code spans,
// code blocks, and headings are located identically.
var mdParser = goldmark.New().Parser()

// gapMarkers are the heading marks that make the section below them a planned
// gap: a wikilink there is a forward-reference, not a broken link.
var gapMarkers = [...]string{"缺口", "待補", "待寫", "待整理", "待建"}

// inlinePlannedMarkers are the inline marks (not headings) whose line's [[X]]
// links are planned forward-references.
var inlinePlannedMarkers = [...]string{"待整理", "待建", "下一課"}

// byteRange is a half-open byte span [start, stop) into a body.
type byteRange struct {
	start, stop int
}

func (r byteRange) contains(off int) bool {
	return off >= r.start && off < r.stop
}

// heading is a heading's parsed facts: its start byte offset, its level (used
// only for relative nesting), and whether its text carries a gap mark.
type heading struct {
	start int
	level int
	gap   bool
}

// rawLink is one [[...]] pair from the raw scan: the byte offset of the opening
// bracket and the inner text between the brackets.
type rawLink struct {
	offset int
	inner  string
}

// extractWikilinks returns every [[target]] in body, skipping those inside code
// or comment zones, those the author escaped to show rather than to follow, and
// dropping bare same-file anchors, each with its 1-based file line and
// gap-section context. bodyStartLine is the file line the body begins on.
func extractWikilinks(body string, bodyStartLine int) []wikiLink {
	codeZones, headings := structure(body)
	skip := slices.Concat(codeZones, commentZones(body, codeZones))
	var links []wikiLink
	for _, raw := range rawWikilinks(body) {
		if inAnyZone(skip, raw.offset) || graph.EscapedWikilinkAt(body, raw.offset) {
			continue
		}
		target, ok := stripTarget(raw.inner)
		if !ok {
			continue // a bare anchor like [[#heading]]
		}
		parsed, _ := graph.ParseWikilink(raw.inner)
		links = append(links, wikiLink{
			target:          target,
			address:         writtenAddress(raw.inner),
			heading:         parsed.Heading,
			block:           parsed.Block,
			embed:           raw.offset > 0 && body[raw.offset-1] == '!',
			offset:          raw.offset,
			line:            bodyStartLine + strings.Count(body[:raw.offset], "\n"),
			underGapHeading: inGapSection(headings, raw.offset),
		})
	}
	return links
}

// extractPathRefs returns every checkable file reference in body: markdown
// [text](path.md) links and backticked path.md tokens. URLs, anchors, and
// percent-encoded or glob paths are left out, so only plain in-vault file
// references remain. A reference inside an Obsidian %%...%% comment is skipped,
// the same way a commented-out wikilink is: commented-out content is not a live
// reference, so it is not checked.
func extractPathRefs(body string, bodyStartLine int) []pathRef {
	src := []byte(body)
	doc := mdParser.Parse(text.NewReader(src))
	codeZones, _ := structure(body)
	comments := commentZones(body, codeZones)
	var refs []pathRef
	walkNodes(doc, func(n ast.Node) {
		switch node := n.(type) {
		case *ast.Link:
			if target, ok := fileLink(string(node.Destination)); ok {
				if off, ok := inlineOffset(node); ok && !inAnyZone(comments, off) {
					refs = append(refs, pathRef{target: target, line: bodyStartLine + strings.Count(body[:off], "\n"), code: false})
				}
			}
		case *ast.CodeSpan:
			if target, ok := backtickPath(codeSpanText(node, src)); ok {
				if off, ok := inlineOffset(node); ok && !inAnyZone(comments, off) {
					refs = append(refs, pathRef{target: target, line: bodyStartLine + strings.Count(body[:off], "\n"), code: true})
				}
			}
		}
	})
	return refs
}

// extractPlannedNames returns the concept names listed as planned under a gap
// heading anywhere in body, plus the targets of [[X]] links beside an inline
// planned marker. These are tracked forward-references: a broken link to one of
// them is planned, not missing.
func extractPlannedNames(body string) []string {
	codeZones, headings := structure(body)
	var names []string
	var item *string
	offset := 0
	for raw := range strings.Lines(body) {
		line := strings.TrimRight(raw, "\r\n")
		inCode := inAnyZone(codeZones, offset)
		inGap := inGapSection(headings, offset) && !inCode
		item, names = advancePlannedItem(item, names, line, inGap)
		if !inCode {
			names = inlinePlannedTargets(line, names)
		}
		offset += len(raw)
	}
	if item != nil {
		names = pushPlannedNames(*item, names)
	}
	return names
}

// advancePlannedItem folds one line into the running gap-list item and the
// collected names. A new list item under a gap heading flushes the previous
// item and starts one; an indented continuation extends it; any other line
// under a closed section flushes it.
func advancePlannedItem(item *string, names []string, line string, inGap bool) (nextItem *string, nextNames []string) {
	trimmed := strings.TrimLeftFunc(line, unicode.IsSpace)
	isItem := strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "+ ")
	isContinuation := item != nil && (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")) && trimmed != "" && !isItem
	switch {
	case inGap && isItem:
		if item != nil {
			names = pushPlannedNames(*item, names)
		}
		started := strings.TrimSpace(trimmed[2:])
		return &started, names
	case inGap && isContinuation:
		joined := *item + " " + trimmed
		return &joined, names
	case item != nil:
		return nil, pushPlannedNames(*item, names)
	default:
		return item, names
	}
}

// inlinePlannedTargets appends the [[X]] targets on a line beside an inline
// planned marker.
func inlinePlannedTargets(line string, names []string) []string {
	if !containsAnySubstring(line, inlinePlannedMarkers[:]) {
		return names
	}
	for _, r := range rawWikilinks(line) {
		if target, ok := stripTarget(r.inner); ok {
			names = append(names, target)
		}
	}
	return names
}

// structure locates the code span/block byte ranges to skip and the headings,
// in document order, using the shared markdown parser.
func structure(body string) ([]byteRange, []heading) {
	src := []byte(body)
	doc := mdParser.Parse(text.NewReader(src))
	var codeZones []byteRange
	var headings []heading
	walkNodes(doc, func(n ast.Node) {
		switch node := n.(type) {
		case *ast.FencedCodeBlock:
			if r, ok := linesRange(node); ok {
				codeZones = append(codeZones, r)
			}
		case *ast.CodeBlock:
			if r, ok := linesRange(node); ok {
				codeZones = append(codeZones, r)
			}
		case *ast.CodeSpan:
			if r, ok := inlineRange(node); ok {
				codeZones = append(codeZones, r)
			}
		case *ast.Heading:
			h := heading{level: node.Level, gap: headingIsGap(node, src)}
			if r, ok := linesRange(node); ok {
				h.start = r.start
				headings = append(headings, h)
			} else if r, ok := inlineRange(node); ok {
				h.start = r.start
				headings = append(headings, h)
			}
		}
	})
	return codeZones, headings
}

// walkNodes visits every node of doc in document order, calling visit as each
// node is entered. The visitor cannot fail, so neither can the traversal.
func walkNodes(doc ast.Node, visit func(ast.Node)) {
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) { //nolint:errcheck // the visitor never returns an error, so the walk cannot fail
		if entering {
			visit(n)
		}
		return ast.WalkContinue, nil
	})
}

// commentZones returns the byte ranges of Obsidian %%...%% comments. A %% inside
// a code zone is ignored first so it cannot shift the pairing of real comments;
// the remaining marks are paired in order and an unpaired trailing one is
// dropped.
func commentZones(body string, codeZones []byteRange) []byteRange {
	var marks []int
	for off := 0; ; {
		rel := strings.Index(body[off:], "%%")
		if rel < 0 {
			break
		}
		at := off + rel
		if !inAnyZone(codeZones, at) {
			marks = append(marks, at)
		}
		off = at + 2
	}
	var zones []byteRange
	for k := 0; k+1 < len(marks); k += 2 {
		zones = append(zones, byteRange{marks[k], marks[k+1] + 2})
	}
	return zones
}

// rawWikilinks scans body for [[...]] pairs, returning the byte offset of each
// opening bracket and its inner text. Inner text that spans a newline is
// dropped: a wikilink is single-line.
func rawWikilinks(body string) []rawLink {
	var out []rawLink
	i := 0
	for {
		rel := strings.Index(body[i:], "[[")
		if rel < 0 {
			break
		}
		open := i + rel
		after := open + 2
		relEnd := strings.Index(body[after:], "]]")
		if relEnd < 0 {
			break
		}
		inner := body[after : after+relEnd]
		if !strings.Contains(inner, "\n") {
			out = append(out, rawLink{offset: open, inner: inner})
		}
		i = after + relEnd + 2
	}
	return out
}

// writtenAddress is the author's whole addressing text: everything before a
// display alias, kept as written so a diagnostic can quote the address the
// author typed rather than a reconstruction of it. Only the escape a table
// cell needs in front of its pipe comes off, since that backslash is syntax
// rather than part of the name.
func writtenAddress(inner string) string {
	beforePipe, _, _ := strings.Cut(inner, "|")
	return strings.TrimSpace(strings.TrimRight(beforePipe, `\`))
}

// stripTarget reduces a wikilink's inner text to its resolution target,
// discarding a |display, #heading or ^block suffix and a trailing backslash,
// which a table cell writes to escape the display pipe. ok is false for a bare
// same-file anchor. The same stripping runs on a provenance reference, so a
// body link and a frontmatter value resolve identically.
func stripTarget(inner string) (string, bool) {
	beforePipe, _, _ := strings.Cut(inner, "|")
	beforePipe = strings.TrimRight(beforePipe, `\`)
	beforeHeading, _, _ := strings.Cut(beforePipe, "#")
	beforeBlock, _, _ := strings.Cut(beforeHeading, "^")
	target := strings.TrimSpace(beforeBlock)
	return target, target != ""
}

// inGapSection reports whether offset falls in a section opened by a gap heading
// and not yet closed by a heading at the same or a higher level.
func inGapSection(headings []heading, offset int) bool {
	gapLevel := -1
	for _, h := range headings {
		if h.start > offset {
			break
		}
		if gapLevel >= 0 && h.level <= gapLevel {
			gapLevel = -1
		}
		if h.gap {
			gapLevel = h.level
		}
	}
	return gapLevel >= 0
}

// pushPlannedNames splits one gap-list entry into concept names — dropping
// (...) / （...） annotations, then splitting on the enumeration comma 、 and
// " / " — and appends the non-empty ones.
func pushPlannedNames(entry string, out []string) []string {
	for part := range strings.SplitSeq(stripParens(entry), "、") {
		for piece := range strings.SplitSeq(part, " / ") {
			if name := strings.TrimSpace(piece); name != "" {
				out = append(out, name)
			}
		}
	}
	return out
}

// stripParens removes parenthesized annotations, ASCII and full-width, from a
// gap-list entry.
func stripParens(s string) string {
	var b strings.Builder
	depth := 0
	for _, c := range s {
		switch c {
		case '(', '（':
			depth++
		case ')', '）':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 {
				b.WriteRune(c)
			}
		}
	}
	return b.String()
}

// headingIsGap reports whether a heading's text carries any gap mark.
func headingIsGap(n *ast.Heading, src []byte) bool {
	htext := headingText(n, src)
	for _, m := range gapMarkers {
		if strings.Contains(htext, m) {
			return true
		}
	}
	return false
}

// headingText is a heading's plain text — the text of its inline content with
// the markup removed. The contents of an inline code span are excluded, because
// a gap mark inside code is quoted rather than written, and the source text a
// heading marks itself as a gap only by its prose.
func headingText(n *ast.Heading, src []byte) string {
	var b strings.Builder
	var walk func(ast.Node)
	walk = func(node ast.Node) {
		for c := node.FirstChild(); c != nil; c = c.NextSibling() {
			switch t := c.(type) {
			case *ast.Text:
				b.Write(src[t.Segment.Start:t.Segment.Stop])
			case *ast.CodeSpan:
				// A code span's content is quoted, not heading prose.
			default:
				walk(c)
			}
		}
	}
	walk(n)
	return b.String()
}

// linesRange is a block node's source span, from the start of its first line
// segment to the end of its last, or false when it has none.
func linesRange(n ast.Node) (byteRange, bool) {
	ls := n.Lines()
	if ls == nil || ls.Len() == 0 {
		return byteRange{}, false
	}
	return byteRange{ls.At(0).Start, ls.At(ls.Len() - 1).Stop}, true
}

// inlineRange is the span covering an inline node's text children, from the
// earliest child start to the latest child stop, or false when it has none.
func inlineRange(n ast.Node) (byteRange, bool) {
	start, stop, found := 0, 0, false
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		t, ok := c.(*ast.Text)
		if !ok {
			continue
		}
		if !found || t.Segment.Start < start {
			start = t.Segment.Start
		}
		if !found || t.Segment.Stop > stop {
			stop = t.Segment.Stop
		}
		found = true
	}
	if !found {
		return byteRange{}, false
	}
	return byteRange{start, stop}, true
}

// inlineOffset is a source offset on the line an inline node sits on, for
// numbering a file reference. It prefers the node's own text, which pins the
// exact line; an empty-text link (like a markdown link with no label) carries
// no text, so it falls back to a sibling on the same line, then to the line the
// enclosing block starts on. It never asks an inline node for its line segments,
// which only block nodes carry.
func inlineOffset(n ast.Node) (int, bool) {
	if off, ok := textDescendantStart(n); ok {
		return off, true
	}
	// An empty-text node lands between its siblings on the same line: the next
	// sibling's text follows it on that line, and when the node ends the line a
	// preceding sibling's text shares it.
	if next := n.NextSibling(); next != nil {
		if off, ok := textDescendantStart(next); ok {
			return off, true
		}
	}
	if prev := n.PreviousSibling(); prev != nil {
		if off, ok := textDescendantStart(prev); ok {
			return off, true
		}
	}
	// A node alone in its block has no sibling text: the block's first line
	// holds it.
	for a := n.Parent(); a != nil; a = a.Parent() {
		if r, ok := linesRange(a); ok {
			return r.start, true
		}
	}
	return 0, false
}

// textDescendantStart is the source start of a node's first text descendant in
// document order, or false when it has none.
func textDescendantStart(n ast.Node) (int, bool) {
	if t, ok := n.(*ast.Text); ok {
		return t.Segment.Start, true
	}
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if off, ok := textDescendantStart(c); ok {
			return off, true
		}
	}
	return 0, false
}

// codeSpanText is the source text between an inline code span's backticks.
func codeSpanText(n *ast.CodeSpan, src []byte) string {
	var b strings.Builder
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if t, ok := c.(*ast.Text); ok {
			b.Write(src[t.Segment.Start:t.Segment.Stop])
		}
	}
	return b.String()
}

// fileLink reports a markdown link destination that is a plain relative vault
// note reference: it drops a #fragment or ?query, trims, and accepts only a
// relative .md path.
func fileLink(dest string) (string, bool) {
	path := dest
	if i := strings.IndexAny(dest, "#?"); i >= 0 {
		path = dest[:i]
	}
	path = strings.TrimSpace(path)
	if isRelativeMdRef(path) {
		return path, true
	}
	return "", false
}

// backtickPath reports a backticked token that is a relative vault .md path.
// Unlike a markdown link it must contain a separator, so a bare foo.md in prose
// is not mistaken for a path.
func backtickPath(token string) (string, bool) {
	t := strings.TrimSpace(token)
	if strings.Contains(t, "/") && isRelativeMdRef(t) {
		return t, true
	}
	return "", false
}

// isRelativeMdRef reports whether path is a plain relative .md file reference
// worth stat-ing: it names a Markdown note by the vault's one extension test
// and is not a URL, a site-absolute or home path, a glob or placeholder, or
// percent-encoded. An uppercase spelling such as "Note.MD" names a resource
// here as it does to every other reader; a private fold on this one path made
// the judge count references no other face called notes.
func isRelativeMdRef(path string) bool {
	return path != "" &&
		vault.IsMarkdown(path) &&
		!strings.HasPrefix(path, "/") &&
		!strings.HasPrefix(path, "~") &&
		!strings.Contains(path, "://") &&
		!strings.Contains(path, "%") &&
		!strings.Contains(path, "*") &&
		!strings.Contains(path, "<") &&
		!strings.Contains(path, ">")
}

// inAnyZone reports whether off falls in any of the ranges.
func inAnyZone(zones []byteRange, off int) bool {
	for _, z := range zones {
		if z.contains(off) {
			return true
		}
	}
	return false
}

// containsAnySubstring reports whether s contains any of the marks as a
// substring. The name says substring because the standard library's
// ContainsAny asks the opposite question — whether any single rune of a set
// occurs — and a reader who knows that one would read this call site backwards.
func containsAnySubstring(s string, marks []string) bool {
	for _, m := range marks {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}
