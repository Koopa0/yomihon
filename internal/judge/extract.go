package judge

import (
	"cmp"
	"slices"
	"strings"
	"unicode"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/schema"
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
// link (resolved relative to its note after one percent-decode) or a backticked
// path token (resolved from the root or its note). code distinguishes the two;
// target keeps the authored spelling for the diagnostic's identity.
type pathRef struct {
	target string
	line   int
	code   bool
}

// mdParser recognizes isolated callout titles with the fixed body grammar.
// Whole-body extraction and heading recognition borrow immutable BodyFacts.
var mdParser = graph.NewBodyMarkdown(nil).Parser()

// plannedMarks are the heading and inline marks one extraction reads. They
// come from the vault contract through [schema]; a contract that omits the
// keys is loaded with today's dialect defaults, and a contract that writes
// an empty list tracks no mark.
type plannedMarks struct {
	heading []string
	inline  []string
}

func defaultPlannedMarks() plannedMarks {
	return plannedMarks{
		heading: schema.DefaultPlannedGapMarks(),
		inline:  schema.DefaultPlannedInlineMarks(),
	}
}

func plannedMarksFrom(c *schema.Contract) plannedMarks {
	if c == nil {
		return defaultPlannedMarks()
	}
	heading, inline := c.PlannedMarks()
	return plannedMarks{heading: heading, inline: inline}
}

// byteRange is a half-open byte span into a body. The type is graph's, so
// pairing cannot drift from the reading sequence uses.
type byteRange = graph.Span

// heading is a heading's parsed facts: its start byte offset, its level (used
// only for relative nesting), and whether its text carries a gap mark.
type heading struct {
	start int
	level int
	gap   bool
}

// bodyStructure holds one body's code, comments and spoken headings. Its
// slices stay read-only while the independent extractors borrow them.
type bodyStructure struct {
	body     graph.BodyFacts
	skip     []byteRange
	comments bodyComments
	headings []heading
}

// bodyComments carries the already-bound projection into fragment consumers.
// Its facts and original zones are immutable for this note's extraction.
type bodyComments struct {
	body  graph.BodyFacts
	zones []byteRange
}

func inspectBody(body string, headingMarks []string) bodyStructure {
	return inspectBodyFacts(graph.ReadBody(body), headingMarks)
}

func inspectBodyFacts(parsed graph.BodyFacts, headingMarks []string) bodyStructure {
	codeZones, headings := structureFrom(parsed, headingMarks)
	var comments []byteRange
	for span := range parsed.Comments() {
		comments = append(comments, span)
	}
	return bodyStructure{
		body:     parsed,
		skip:     slices.Concat(codeZones, comments),
		comments: bodyComments{body: parsed, zones: comments},
		headings: spokenHeadings(headings, comments),
	}
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
	return extractWikilinksWith(body, bodyStartLine, defaultPlannedMarks().heading)
}

func extractWikilinksWith(body string, bodyStartLine int, headingMarks []string) []wikiLink {
	facts := inspectBody(body, headingMarks)
	return extractWikilinksFrom(body, bodyStartLine, &facts)
}

func extractWikilinksFrom(body string, bodyStartLine int, facts *bodyStructure) []wikiLink {
	headings, skip := facts.headings, facts.skip
	var links []wikiLink
	for _, raw := range rawWikilinks(body) {
		if graph.In(skip, raw.offset) || graph.EscapedWikilinkAt(body, raw.offset) {
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

func extractPathRefsFrom(_ string, bodyStartLine int, comments bodyComments) []pathRef {
	return extractPathRefsFacts(comments.body, bodyStartLine)
}

func extractPathRefsFacts(facts graph.BodyFacts, bodyStartLine int) []pathRef {
	body := facts.Source()
	type occurrence struct {
		offset int
		ref    pathRef
	}
	var occurrences []occurrence
	for destination := range facts.Destinations() {
		if destination.Image || !facts.EmittedAt(destination.Offset) {
			continue
		}
		if target, ok := fileLink(destination.Target); ok {
			occurrences = append(occurrences, occurrence{offset: destination.Offset, ref: pathRef{target: target, line: bodyStartLine + strings.Count(body[:destination.Offset], "\n")}})
		}
	}
	for literal := range facts.CodeLiterals() {
		if target, ok := backtickPath(literal.Text); ok && facts.EmittedAt(literal.Span.Start) {
			occurrences = append(occurrences, occurrence{offset: literal.Span.Start, ref: pathRef{target: target, line: bodyStartLine + strings.Count(body[:literal.Span.Start], "\n"), code: true}})
		}
	}
	slices.SortStableFunc(occurrences, func(a, b occurrence) int { return cmp.Compare(a.offset, b.offset) })
	var refs []pathRef
	for _, occurrence := range occurrences {
		refs = append(refs, occurrence.ref)
	}
	return refs
}

// extractPlannedNames returns the concept names listed as planned under a gap
// heading anywhere in body, plus the targets of [[X]] links beside an inline
// planned marker. These are tracked forward-references: a broken link to one of
// them is planned, not missing.
func extractPlannedNames(body string) []string {
	marks := defaultPlannedMarks()
	return extractPlannedNamesWith(body, marks)
}

// extractPlannedNamesWith harvests only where the author is speaking: a marker
// or a target inside code is quoted syntax and one inside an Obsidian comment
// is content the author took back, exactly as the link extraction reads them,
// so a declaration that is merely shown cannot soften another note's link.
func extractPlannedNamesWith(body string, marks plannedMarks) []string {
	facts := inspectBody(body, marks.heading)
	return extractPlannedNamesFrom(body, marks, &facts)
}

func extractPlannedNamesFrom(body string, marks plannedMarks, facts *bodyStructure) []string {
	headings := facts.headings
	spoken := blankZones(body, facts.skip)
	var names []string
	var item *string
	offset := 0
	for raw := range strings.Lines(spoken) {
		line := strings.TrimRight(raw, "\r\n")
		item, names = advancePlannedItem(item, names, line, inGapSection(headings, offset))
		names = inlinePlannedTargets(line, names, marks.inline)
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
func inlinePlannedTargets(line string, names, inlineMarks []string) []string {
	if !containsAnySubstring(line, inlineMarks) {
		return names
	}
	for _, r := range rawWikilinks(line) {
		if target, ok := stripTarget(r.inner); ok {
			names = append(names, target)
		}
	}
	return names
}

// blankZones replaces the bytes of every zone with spaces, keeping newlines and
// keeping the body's length, so what is left reads as the note's prose while
// every offset still addresses the original note — which is what lets a harvest
// blank a zone and go on asking the headings located in the body itself.
func blankZones(body string, zones []byteRange) string {
	if len(zones) == 0 {
		return body
	}
	b := []byte(body)
	for _, z := range zones {
		for i := z.Start; i < z.Stop; i++ {
			if b[i] != '\n' {
				b[i] = ' '
			}
		}
	}
	return string(b)
}

// structure locates the code span/block byte ranges to skip and the headings,
// in document order, using the shared markdown parser.
func structure(body string, headingMarks []string) ([]byteRange, []heading) {
	return structureFrom(graph.ReadBody(body), headingMarks)
}

func structureFrom(facts graph.BodyFacts, headingMarks []string) ([]byteRange, []heading) {
	var codeZones []byteRange
	var headings []heading
	for code := range facts.Codes() {
		codeZones = append(codeZones, code.Span)
	}
	for definition := range facts.Footnotes() {
		if !definition.Emitted {
			codeZones = append(codeZones, definition.Span)
		}
	}
	for found := range facts.Headings() {
		if facts.EmittedAt(found.Span.Start) {
			headings = append(headings, heading{start: found.Span.Start, level: found.Level, gap: containsAnySubstring(found.Text, headingMarks)})
		}
	}
	return codeZones, headings
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

// spokenHeadings drops the headings the author took back inside an Obsidian
// comment, so a section is bounded by the same authored content the harvest
// reads. A heading Obsidian hides is no more a boundary than a commented-out
// link is a link: it neither opens a section nor closes one. The result reuses
// the caller's storage, so the filtered list replaces the one passed in rather
// than standing beside it.
func spokenHeadings(headings []heading, comments []byteRange) []heading {
	return slices.DeleteFunc(headings, func(h heading) bool { return graph.In(comments, h.start) })
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

// fileLink admits a Markdown note path after decoding it once, retaining its
// original spelling for later findings. A raw #fragment or ?query is omitted
// before decoding so encoded delimiters remain filename characters.
func fileLink(dest string) (string, bool) {
	result := graph.ParseMarkdownPath("source.md", dest)
	if !result.Checkable {
		return "", false
	}
	return strings.TrimSuffix(dest, result.Suffix), true
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
// checked as a code token: it names a Markdown note by the vault's extension test
// and is not a URL, a site-absolute or home path, a glob or placeholder, or
// percent-encoded. An uppercase spelling such as "Note.MD" names a resource
// here as it does to every other reader; a private fold on this one path made
// the judge count references no other face called notes.
func isRelativeMdRef(path string) bool {
	return !strings.Contains(path, "%") && isMarkdownPathRef(path)
}

func isMarkdownPathRef(path string) bool {
	return path != "" &&
		vault.IsMarkdown(path) &&
		!strings.HasPrefix(path, "/") &&
		!strings.HasPrefix(path, "~") &&
		!strings.Contains(path, "://") &&
		!strings.Contains(path, "*") &&
		!strings.Contains(path, "<") &&
		!strings.Contains(path, ">")
}

// containsAnySubstring reports whether s contains any of the marks as a
// substring. Both sides are folded to NFC first, so a heading written with a
// combining mark hits the composed spelling the contract declared. Case is
// not folded — two marks that differ only in case are two declarations.
// The name says substring because the standard library's ContainsAny asks
// the opposite question — whether any single rune of a set occurs — and a
// reader who knows that one would read this call site backwards.
func containsAnySubstring(s string, marks []string) bool {
	folded := vault.NormalizeNFC(s)
	for _, m := range marks {
		if m == "" {
			continue
		}
		if strings.Contains(folded, vault.NormalizeNFC(m)) {
			return true
		}
	}
	return false
}
