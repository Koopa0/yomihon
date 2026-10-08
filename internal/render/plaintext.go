package render

// A note body's searchable plain text, walked off the same markdown engine the
// HTML pipeline uses. It lives here rather than beside its consumer because it
// needs this package's dialect passes, and a second copy of those would be free
// to disagree with the renderer about what a note says.

import (
	"bytes"
	"slices"
	"strings"
	"unicode"

	"github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/sequence"
)

// PlainText returns the searchable plain text of a note body: prose, headings,
// table cells, task text, code-fence contents and the base and reading of
// hand-written ruby, but not the HTML tags or the callout marker syntax. A
// ruby reading is written after the block's base text so the phrase a reader
// sees stays one substring, and the reading remains findable on its own. A
// wikilink contributes both its target and its display text. The body must arrive
// with its frontmatter removed, and the text keeps its case and Unicode form.
//
// A course branch's role declaration stays in the text, although the page takes
// it off the heading it shows. Keeping it is what lets an author search for the
// notes that declare one; taking it off would mean assembling each heading's
// words here and reading a declaration back out of them, which is the page's own
// job and not this walk's. The cost is one incongruity: a search for the
// declaration finds a note whose page no longer shows those words.
func PlainText(body string) string {
	plain, _, _ := PlainBlocks(body)
	return plain
}

// Block is one block-level contribution to a note's searchable text.
type Block struct {
	// End is the exclusive end offset of the block's text.
	End int

	// Verbatim reports that this block's characters reach the reading page in
	// this order, apart from separately recorded link Insertions. False is
	// also the answer wherever the walk cannot tell,
	// so a caller may act on a true and never on a false.
	Verbatim bool

	// ContextRanges are half-open byte ranges in PlainText whose characters
	// stay adjacent on the reading page even when the whole block does not.
	// Their edges are not necessarily word boundaries: removing markup can
	// join the text on opposite sides of an edge.
	ContextRanges [][2]int

	// Heading reports that the block is a heading the page shows exactly as
	// written. The page's contents list repeats those words, so text inside
	// the heading alone cannot tell the list's copy from the body's. Such a
	// block offers no ContextRanges for the same reason.
	Heading bool
}

// PlainBlocks returns the searchable text of a note body, one Block per
// block-level contribution to that text, and the half-open byte ranges that
// came from a fenced code block. The text is byte-identical to PlainText. A
// phrase whose match starts in one block and ends in another is one the
// browser's text directive cannot find, because those words render in
// different elements; a wrap inside one paragraph is not that case.
//
// Each block also says whether the page reproduces it as written. Naming a
// run of words ahead of a match asks a browser to find that run and the match
// side by side in what it is showing, so a caller with such a use has to know
// the two are not merely present but adjacent — and this walk is the only
// place that knows, because it is the one that moves a ruby reading out of the
// sentence and leaves a footnote's mark out of the text altogether.
//
// Fence ranges sit in the same coordinate space as the text so a later match
// can tell a hit that landed in source from one that landed in prose. The
// bodies stay in the text: people search for code snippets. Eligibility of a
// fence as the excerpt is a decision for the match, not this walk.
func PlainBlocks(body string) (plain string, blocks []Block, fenceRanges [][2]int) {
	source, rewritten := plainPreprocess(body)
	return plainSourceBlocks(body, source, &rewritten, nil, nil)
}

func plainSourceBlocks(body, source string, rewritten *rewrittenLines, emissions *[]sourceEmission, insertions *[]int) (plain string, blocks []Block, fenceRanges [][2]int) {
	src := []byte(source)
	// Search retains strike/highlight delimiter bytes even though the page
	// consumes them. Observe the canonical recognizers' actual matched pairs;
	// the text walk emits those source segments around each recognized node.
	observation := delimiterObservation{
		lengths: make(map[*parser.Delimiter]int), left: make(map[*parser.Delimiter]int), rewritten: rewritten,
		corpus: make(map[ast.Node][2]text.Segment),
	}
	markdown := graph.NewBodyMarkdown(nil)
	markdown.Parser().AddOptions(&observation)
	doc := markdown.Parser().Parse(text.NewReader(src))

	w := plainWalk{blockVerbatim: true, blockContext: true, rewritten: rewritten, emissions: emissions, insertions: insertions, delimiters: observation.corpus, remnants: observation.remnants()}

	if err := ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		return walkPlain(&w, n, entering, src)
	}); err != nil {
		// Unreachable: walkPlain never returns a non-nil error. If a future
		// goldmark change ever makes Walk itself fail, fall back to the raw
		// body (whitespace-collapsed) so a note is never left unsearchable.
		// Nothing about that text is vouched for against the page.
		collapsed := strings.Join(strings.Fields(body), " ")
		if collapsed == "" {
			return "", nil, nil
		}
		return collapsed, []Block{{End: len(collapsed)}}, nil
	}
	w.closeBlock()
	w.flushReadings()
	if emissions != nil {
		raw := w.b.String()
		lead := len(raw) - len(strings.TrimLeftFunc(raw, unicode.IsSpace))
		for i := range *emissions {
			(*emissions)[i].out -= lead
			(*emissions)[i].outEnd -= lead
		}
	}
	if insertions != nil {
		raw := w.b.String()
		lead := len(raw) - len(strings.TrimLeftFunc(raw, unicode.IsSpace))
		length := len(strings.TrimSpace(raw))
		kept := (*insertions)[:0]
		for _, at := range *insertions {
			if at >= lead && at-lead <= length {
				kept = append(kept, at-lead)
			}
		}
		*insertions = kept
	}
	return w.result()
}

// reproducedByThePage reports that a node of this kind contributes the same
// characters to this text that the reading page shows, in the same place.
// Naming the kinds that do, rather than the kinds that do not, is what keeps
// a construct nobody here has met yet out of the answer: the page has passes
// of its own that take words off a heading and off a list row, spend the
// marks around a highlight, put a superscript where this text has nothing,
// and replace an embed with the note it names — and a kind this list has
// never heard of is as likely to be one of those as not.
func reproducedByThePage(kind ast.NodeKind) bool {
	switch kind {
	case ast.KindDocument, ast.KindParagraph, ast.KindTextBlock, ast.KindBlockquote,
		ast.KindList, ast.KindThematicBreak,
		ast.KindText, ast.KindEmphasis, ast.KindLink, ast.KindCodeSpan, ast.KindAutoLink,
		east.KindTable, east.KindTableHeader, east.KindTableRow, east.KindTableCell,
		east.KindTaskCheckBox:
		return true
	}
	return false
}

// spentByPage reports that the text carries characters the page consumes
// rather than shows. Search preserves the two recognized delimiter pairs,
// while the page shows only what was between them; the private-use runes are
// the ones a body loses before it is rendered at all.
func spentByPage(s string) bool {
	if strings.Contains(s, "==") || strings.Contains(s, "~~") {
		return true
	}
	return strings.ContainsAny(s, inlinePlaceholderRunes)
}

// plainWalk is the accumulator walkPlain writes. The text is what PlainText
// always returned; blocks end where each block ended before the leading and
// trailing space are trimmed off. fenceRanges are the half-open spans written
// from a fenced code block, in the same raw coordinates.
type plainWalk struct {
	b           strings.Builder
	blocks      []Block
	fenceRanges [][2]int
	// blockVerbatim is the verdict being accumulated for the block now open:
	// it starts true at each block and any doubt takes it down, so closing a
	// block is the only place that reads it and the only place that raises it
	// again. rewritten is what the preprocess rewrote, which the text carries
	// and the page does not.
	blockVerbatim bool
	blockContext  bool
	headingLevel  int
	rewritten     *rewrittenLines
	// readings holds <rt>/<rtc> text until the block's base text has been
	// closed, so a visible phrase is not split by its furigana. ruby is the
	// stack of open <ruby> elements, innermost last, each recording which of
	// its children the walk is inside; together they decide where the next
	// text node goes, and an inner ruby's end restores the state of the one
	// around it. <rp> is only a parenthesis fallback and is dropped.

	readings          strings.Builder
	emissions         *[]sourceEmission
	insertions        *[]int
	readingInsertions []int
	delimiters        map[ast.Node][2]text.Segment
	remnants          []delimiterRemnant
	readingEmissions  []sourceEmission
	ruby              []rubyChild
}

// rubyChild names which child of an open <ruby> the walk is inside: its base
// text, an <rt> or <rtc> annotation, or an <rp> parenthesis fallback. As the
// result of rubyRoute it also names where text goes, since a parenthesis
// outranks an annotation and an annotation outranks base text.
type rubyChild uint8

const (
	rubyBase rubyChild = iota
	rubyAnnotation
	rubyParen
)

// rubyRoute is where text inside the given open rubies goes. A parenthesis
// fallback anywhere around the text drops it, an annotation anywhere around
// it holds it as a reading, and otherwise it is base text — so the base and
// the reading of a ruby written inside another's annotation both stay in
// that annotation.
func rubyRoute(open []rubyChild) rubyChild {
	route := rubyBase
	for _, child := range open {
		if child == rubyParen {
			return rubyParen
		}
		if child == rubyAnnotation {
			route = rubyAnnotation
		}
	}
	return route
}

func (w *plainWalk) result() (plain string, blocks []Block, fenceRanges [][2]int) {
	raw := w.b.String()
	plain = strings.TrimSpace(raw)
	if plain == "" {
		return "", nil, nil
	}
	lead := len(raw) - len(strings.TrimLeftFunc(raw, unicode.IsSpace))
	blocks = shiftBlocks(w.blocks, lead, len(plain))
	if n := len(blocks); n == 0 || blocks[n-1].End != len(plain) {
		// Text past the last block this walk named belongs to no block it
		// saw, so there is nothing here that vouches for it.
		blocks = append(blocks, Block{End: len(plain)})
	}
	return plain, blocks, shiftRanges(w.fenceRanges, lead, len(plain))
}

// shiftBlocks maps blocks recorded in the raw builder onto the trimmed text,
// dropping empties and keeping the ends strictly increasing. Two ends that
// land on the same character are one block afterwards, and it is reproduced
// as written only if both halves were.
func shiftBlocks(blocks []Block, lead, length int) []Block {
	var out []Block
	for _, b := range blocks {
		adj := b.End - lead
		if adj <= 0 {
			continue
		}
		if adj > length {
			adj = length
		}
		if n := len(out); n > 0 && out[n-1].End >= adj {
			out[n-1].Verbatim = out[n-1].Verbatim && b.Verbatim
			continue
		}
		out = append(out, Block{End: adj, Verbatim: b.Verbatim, ContextRanges: shiftRanges(b.ContextRanges, lead, length), Heading: b.Heading})
	}
	return out
}

// shiftRanges maps half-open spans the same way, clamping each end to the
// trimmed text so a fence that sat in leading or trailing space disappears.
func shiftRanges(ranges [][2]int, lead, length int) [][2]int {
	var out [][2]int
	for _, r := range ranges {
		start, end := r[0]-lead, r[1]-lead
		if end <= 0 || start >= length {
			continue
		}
		if start < 0 {
			start = 0
		}
		if end > length {
			end = length
		}
		if start < end {
			out = append(out, [2]int{start, end})
		}
	}
	return out
}

// closeBlock records the block just written, and with it the verdict on
// whether the page reproduces that block as written. The two doubts settled
// here rather than earlier are the ones only the finished block can answer:
// readings still held are readings this block's base text was parted from,
// and the markup the page spends is visible only in the bytes that were
// written. An early return means nothing new was written, so the block is
// still open and its verdict still being accumulated.
func (w *plainWalk) closeBlock() {
	s := w.b.String()
	end := len(s)
	for end > 0 && s[end-1] == '\n' {
		end--
	}
	if end == 0 {
		return
	}
	start := 0
	if n := len(w.blocks); n > 0 {
		if w.blocks[n-1].End >= end {
			return
		}
		start = w.blocks[n-1].End
	}
	w.blocks = append(w.blocks, w.completedBlock(s, start, end))
	w.blockVerbatim = true
	w.blockContext = true
	w.headingLevel = 0
}

// completedBlock settles page equivalence after the whole block is written.
// Heading words name the heading itself and never provide nearby context.
func (w *plainWalk) completedBlock(s string, start, end int) Block {
	verbatim := w.blockVerbatim && w.readings.Len() == 0 && !spentByPage(s[start:end])
	var contexts [][2]int
	if !verbatim && w.blockContext && w.readings.Len() == 0 {
		contexts = localContextRanges(s, start, end, w.headingLevel)
	}
	heading := w.headingLevel > 0 && len(contexts) == 1 && contexts[0] == [2]int{start, end}
	if w.headingLevel > 0 {
		// A heading's own words are what its contents-list copy repeats, so
		// they vouch for nothing beside a match in it.
		contexts = nil
	}

	return Block{End: end, Verbatim: verbatim, ContextRanges: contexts, Heading: heading}
}

// localContextRanges keeps text between strike delimiters in its original
// coordinates. Unknown constructs, highlights and consumed heading roles
// offer no local context. A caller still has to find a real word boundary
// inside a range rather than treating a removed delimiter as one.
func localContextRanges(s string, start, end, headingLevel int) [][2]int {
	body := s[start:end]
	if strings.Contains(body, "==") || strings.ContainsAny(body, inlinePlaceholderRunes) ||
		(headingLevel > 0 && sequence.HeadingName(body, headingLevel) != body) {
		return nil
	}
	var ranges [][2]int
	for at := start; at < end; {
		i := strings.Index(s[at:end], "~~")
		if i < 0 {
			ranges = append(ranges, [2]int{at, end})
			break
		}
		stop := at + i
		if stop > at {
			ranges = append(ranges, [2]int{at, stop})
		}
		at = stop + 2
	}
	return ranges
}

// recordFence notes the half-open span just written from a fenced code
// block, dropping the trailing newlines closeBlock also drops so the range
// and the block end name the same last byte.
func (w *plainWalk) recordFence(start int) {
	s := w.b.String()
	end := len(s)
	for end > start && s[end-1] == '\n' {
		end--
	}
	if end <= start {
		return
	}
	w.fenceRanges = append(w.fenceRanges, [2]int{start, end})
}

// rewrittenLines names the lines plainPreprocess changed, by where each line
// starts in the text it returned. Text drawn from one of them is not what the
// page shows: the page reads the author's own construct and shows a link's
// display words, a callout's title beside an icon, or a whole other note,
// where this text carries what the rewrite left behind.
type rewrittenLines struct {
	starts       []int
	changed      []bool
	literalRoles []bool
	wikilinks    [][2]int
}

// lineAt returns the index of the line holding off.
func (r *rewrittenLines) lineAt(off int) int {
	i, exact := slices.BinarySearch(r.starts, off)
	if !exact {
		// The search answers with the first line starting past off, so the
		// line holding it is the one before that.
		i--
	}
	return i
}

// covers reports whether the line holding off was rewritten.
func (r *rewrittenLines) covers(off int) bool {
	i := r.lineAt(off)
	return i >= 0 && i < len(r.changed) && r.changed[i]
}

// literalRoleAt names callout titles: their corpus rewrite may look like a
// heading or a list, but the page shows their title as literal text.
func (r *rewrittenLines) literalRoleAt(off int) bool {
	i := r.lineAt(off)
	return i >= 0 && i < len(r.literalRoles) && r.literalRoles[i]
}

// plainPreprocess rewrites the two Obsidian-dialect constructs goldmark has no
// concept of into plain text before parsing: a wikilink or embed becomes "target
// display", both, so a filename search hits through a display alias, and a
// callout marker line loses its marker while keeping the title. It is
// fence-aware, so a link written inside a code sample stays literal. Every
// line it changed is named in the second return, because a rewrite is exactly
// where this text and the page part company.
func plainPreprocess(body string) (string, rewrittenLines) {
	// The retrieval projections report nothing: a corpus entry is not a page,
	// and a fault in a note is the reading page's news to break.
	body, _ = stripObsidianComments(body)
	body = expandInlineFootnotes(body)
	lines := strings.Split(body, "\n")
	rewritten := rewrittenLines{starts: make([]int, len(lines)), changed: make([]bool, len(lines)), literalRoles: make([]bool, len(lines))}
	wikiLines := make([][][2]int, len(lines))
	inFence := false
	var fenceByte byte
	var fenceLen int
	// Which lines are indented code is a parse's answer, asked only once a
	// line looks like a callout opener, since no other rewrite here depends
	// on it.
	var code map[int]bool
	parsed := false
	for i, line := range lines {
		switch {
		case inFence:
			if fenceCloses(line, fenceByte, fenceLen) {
				inFence = false
			}
		default:
			if marker, n, _, ok := fenceOpen(line); ok {
				inFence, fenceByte, fenceLen = true, marker, n
				continue
			}
			_, _, _, opener := calloutStart(line)
			if !parsed && opener {
				code, parsed = codeBlockLines(body), true
			}
			rewritten.literalRoles[i] = opener && !code[i]
			lines[i] = plainLine(line, code[i], &wikiLines[i])
			rewritten.changed[i] = lines[i] != line
		}
	}
	off := 0
	for i, line := range lines {
		rewritten.starts[i] = off
		for _, span := range wikiLines[i] {
			rewritten.wikilinks = append(rewritten.wikilinks, [2]int{off + span[0], off + span[1]})
		}
		off += len(line) + 1 // the newline the join puts back
	}
	return strings.Join(lines, "\n"), rewritten
}

// plainLine normalizes one non-fence line: it strips a callout marker (keeping
// the title) and rewrites wikilinks to plain "target display" text. A line an
// indented code block holds keeps its marker, because the page shows it as
// written.
func plainLine(line string, code bool, wikilinks *[][2]int) string {
	if _, _, title, ok := graph.CalloutStart(line); ok && !code {
		// Drop the marker, keep the callout's title. The body lines that follow
		// keep their quote marker and are collected as ordinary quoted text.
		line = title
	}
	return replaceWikilinksPlain(line, wikilinks)
}

// replaceWikilinksPlain replaces every [[...]]/![[...]] token in line with its
// clean target and display text (deduplicated when they are equal), using the
// same target/display split the renderer uses (graph.SplitWikilink).
func replaceWikilinksPlain(line string, wikilinks *[][2]int) string {
	var out strings.Builder
	last := 0
	for _, loc := range wikilinkToken.FindAllStringIndex(line, -1) {
		out.WriteString(line[last:loc[0]])
		start := out.Len()
		out.WriteString(plainWikilink(line[loc[0]:loc[1]]))
		*wikilinks = append(*wikilinks, [2]int{start, out.Len()})
		last = loc[1]
	}
	out.WriteString(line[last:])
	return out.String()
}

func plainWikilink(token string) string {
	inner := strings.TrimPrefix(token, "!")
	inner = inner[2 : len(inner)-2] // strip the enclosing "[[" and "]]"
	target, display, ok := graph.SplitWikilink(inner)
	switch {
	case !ok:
		return display // e.g. [[#heading]] — a same-file anchor: display only
	case target == display:
		return target
	default:
		return target + " " + display
	}
}

// walkPlain appends one AST node's contribution to w. It never returns an
// error (the ast.Walk error path in PlainBlocks is therefore unreachable).
func walkPlain(w *plainWalk, n ast.Node, entering bool, source []byte) (ast.WalkStatus, error) {
	if !entering {
		if delimiters, recognized := w.delimiters[n]; recognized {
			w.writeSource(delimiters[1], source)
		}

		if _, ok := n.(*ast.Link); ok {
			// An unresolved local link adds an out-of-sight explanation. The
			// corpus cannot resolve files, so every link exit keeps a safe cut.
			w.recordInsertion()
		}
		return ast.WalkContinue, nil
	}
	kind := n.Kind()
	// Separate block-level text so tokens from adjacent blocks (a heading then
	// its paragraph) do not run together. An HTML block is the exception: it
	// contributes nothing below, and closing a block for it would put a break
	// in the text where there has never been one.
	if n.Type() == ast.TypeBlock && kind != ast.KindHTMLBlock {
		writeSeparator(w)
	}
	// After the separator, so the doubt lands on the block this node opens
	// rather than the one it closed.
	_, corpusDelimiters := w.delimiters[n]
	if !reproducedByThePage(kind) && !corpusDelimiters {
		w.blockVerbatim = false
		if heading, ok := n.(*ast.Heading); ok {
			w.headingLevel = heading.Level
		} else {
			w.blockContext = false
		}
	}
	if delimiters, recognized := w.delimiters[n]; recognized {
		w.writeSource(delimiters[0], source)
	}
	return writePlainNode(w, n, kind, source)
}

func writePlainNode(w *plainWalk, n ast.Node, kind ast.NodeKind, source []byte) (ast.WalkStatus, error) {
	switch kind {
	case ast.KindRawHTML, ast.KindHTMLBlock:
		// The tags are not content. Text between them arrives as separate text
		// nodes rather than children, so skipping here drops only the tags.
		// Ruby is the exception: <rt> (and <rtc>) hold a reading that must
		// not sit between the base characters a reader can see.
		w.seeRawHTML(n, source)
		return ast.WalkSkipChildren, nil
	case ast.KindFencedCodeBlock, ast.KindCodeBlock:
		// Code content lives in the node's line segments, not in child Text
		// nodes; write it directly and do not descend — code contents are
		// searchable (people search for code snippets). A fenced block also
		// records the span it wrote, so a later excerpt can decline it when
		// the same words sit in prose. An indented code block is not a fence.
		w.writeCodeBlock(n, source)
		return ast.WalkSkipChildren, nil
	case ast.KindText:
		w.writeTextNode(n, source)
	case ast.KindString:
		if s, ok := n.(*ast.String); ok {
			w.writeVisible(s.Value)
		}
	case ast.KindAutoLink:
		if a, ok := n.(*ast.AutoLink); ok {
			w.writeAutoLink(a, source)
		}
	}
	return ast.WalkContinue, nil
}

func (w *plainWalk) writeCodeBlock(n ast.Node, source []byte) {
	start := w.b.Len()
	w.writeCodeLines(n, source)
	if n.Kind() == ast.KindFencedCodeBlock {
		w.recordFence(start)
	}
}

// writeSeparator closes the block just written and appends a newline unless
// the walk is empty or already ends in one. The newline is the same separator
// PlainText has always used; closing first is what lets a later match know
// which side of it each word sat on.
func writeSeparator(w *plainWalk) {
	w.closeBlock()
	w.flushReadings()
	if w.b.Len() == 0 {
		return
	}
	s := w.b.String()
	if s[len(s)-1] != '\n' {
		w.b.WriteByte('\n')
	}
}

func (w *plainWalk) seeRawHTML(n ast.Node, source []byte) {
	raw, ok := n.(*ast.RawHTML)
	if !ok {
		return
	}
	for i := range raw.Segments.Len() {
		seg := raw.Segments.At(i)
		w.seeMarkup(seg.Value(source))
	}
}

func (w *plainWalk) writeTextNode(n ast.Node, source []byte) {
	t, ok := n.(*ast.Text)
	if !ok {
		return
	}
	if w.rewritten.covers(t.Segment.Start) {
		w.blockVerbatim = false
		w.blockContext = false
	}

	raw := t.IsRaw() || t.Parent() != nil && t.Parent().Kind() == ast.KindCodeSpan
	w.writeTextSource(t.Segment, source, raw)

	if t.SoftLineBreak() || t.HardLineBreak() {
		w.writeSourceBreak(t.Segment.Stop, source)
	}
}

// writeTextSource preserves the actual source coordinates of a remaining
// delimiter. Goldmark retains its prefix segment even after consuming the
// closer's left edge; the characters agree, but display effects need its origin.
func (w *plainWalk) writeTextSource(segment text.Segment, source []byte, raw bool) {
	write := func(start, stop, offset int) {
		part := segment
		part.Start, part.Stop = start+offset, stop+offset
		if start != segment.Start {
			part.Padding = 0
		}
		if stop != segment.Stop {
			part.ForceNewline = false
		}
		if raw {
			w.writeSource(part, source)
		} else {
			w.writeProse(part, source)
		}
	}
	first, _ := slices.BinarySearchFunc(w.remnants, segment.Start, func(remnant delimiterRemnant, start int) int {
		if remnant.span.Stop <= start {
			return -1
		}
		return 1
	})
	at := segment.Start
	for _, remnant := range w.remnants[first:] {
		if remnant.span.Start >= segment.Stop {
			break
		}
		start, stop := max(at, remnant.span.Start), min(segment.Stop, remnant.span.Stop)
		if at < start {
			write(at, start, 0)
		}
		write(start, stop, remnant.offset)
		at = stop
	}
	if at < segment.Stop || at == segment.Start {
		write(at, segment.Stop, 0)
	}
}

// writeVisible appends one text node's bytes to the corpus: into the held
// readings while inside <rt>/<rtc>, nowhere while inside <rp>, and into the
// block's base text otherwise.
func (w *plainWalk) writeVisible(p []byte) {
	switch rubyRoute(w.ruby) {
	case rubyParen:
		return
	case rubyAnnotation:
		w.readings.Write(p)
	default:
		w.b.Write(p)
	}
}

func (w *plainWalk) writeBreak() {
	switch rubyRoute(w.ruby) {
	case rubyParen:
		return
	case rubyAnnotation:
		w.readings.WriteByte('\n')
	default:
		w.b.WriteByte('\n')
	}
}

// seeMarkup notes a raw HTML tag so the following text nodes are routed.
// A self-closing tag has no following text of its own and is ignored.
//
// The end tags of rt and rp may be left out, and HTML fixes where each one
// then ends: at the next annotation or parenthesis of the same ruby, or at
// that ruby's end. The same ruby's only — an annotation may hold a ruby of
// its own, whose annotation opens and closes without touching the one around
// it, so the text after the inner ruby is still the outer reading. Left
// unclosed instead, every later text node in the note would be routed into
// the annotation the scan believes it is still inside, and a paragraph far
// below a ruby would stop reaching the corpus while the page goes on showing
// it.
func (w *plainWalk) seeMarkup(raw []byte) {
	name, closing, selfClose := markupName(raw)
	if selfClose {
		return
	}
	switch name {
	case "ruby":
		if closing {
			if n := len(w.ruby); n > 0 {
				w.ruby = w.ruby[:n-1]
			}
			return
		}
		w.ruby = append(w.ruby, rubyBase)
	case "rt", "rtc":
		if closing {
			w.leaveRubyChild(rubyAnnotation)
			return
		}
		w.enterRubyChild(rubyAnnotation)
	case "rp":
		if closing {
			w.leaveRubyChild(rubyParen)
			return
		}
		w.enterRubyChild(rubyParen)
	}
}

// enterRubyChild moves the innermost open ruby into the child opening here,
// which ends whichever of its children was open before — the end tag HTML
// lets an author leave out. An annotation or parenthesis with no ruby open
// around it is treated as if one were, so its text is still held as a
// reading or dropped as a fallback rather than read as base text.
//
// Two readings written one after the other are held apart by a space, so a
// later match sees two words rather than one run. A reading that opens inside
// another ruby's annotation is that annotation continuing, and gets none.
func (w *plainWalk) enterRubyChild(child rubyChild) {
	if len(w.ruby) == 0 {
		w.ruby = append(w.ruby, rubyBase)
	}
	top := len(w.ruby) - 1
	if child == rubyAnnotation && rubyRoute(w.ruby[:top]) == rubyBase && w.readings.Len() > 0 {
		s := w.readings.String()
		if s[len(s)-1] != ' ' && s[len(s)-1] != '\n' {
			w.readings.WriteByte(' ')
		}
	}
	w.ruby[top] = child
}

// leaveRubyChild returns the innermost open ruby to its base text when the
// end tag names the child it is inside. An end tag for a child already ended,
// whether by a sibling that opened or by never having opened, changes
// nothing.
func (w *plainWalk) leaveRubyChild(child rubyChild) {
	if n := len(w.ruby); n > 0 && w.ruby[n-1] == child {
		w.ruby[n-1] = rubyBase
	}
}

// flushReadings writes held ruby readings as their own block after the base
// text they came from. A later match on the base phrase can then land on the
// sentence the page shows, and a match on the reading still finds the note.
//
// That is the whole of the reordering, so it is also where the block it makes
// is refused: the page shows these readings one at a time, each beside the
// characters it belongs to, and never as the run of words written here.
func (w *plainWalk) flushReadings() {
	if w.readings.Len() == 0 {
		return
	}
	if w.b.Len() > 0 {
		s := w.b.String()
		if s[len(s)-1] != '\n' {
			w.b.WriteByte('\n')
		}
	}
	if w.emissions != nil {
		for _, e := range w.readingEmissions {
			e.out += w.b.Len()
			e.outEnd += w.b.Len()
			*w.emissions = append(*w.emissions, e)
		}
		w.readingEmissions = nil
	}
	if w.insertions != nil {
		for _, at := range w.readingInsertions {
			*w.insertions = append(*w.insertions, w.b.Len()+at)
		}
		w.readingInsertions = nil
	}
	w.b.WriteString(w.readings.String())
	w.readings.Reset()
	w.blockVerbatim = false
	w.closeBlock()
}

// markupName reads the tag name out of one raw HTML segment. Comments,
// processing instructions and a fragment that is not a tag report no name.
func markupName(raw []byte) (name string, closing, selfClose bool) {
	raw = bytes.TrimSpace(raw)
	if len(raw) < 3 || raw[0] != '<' || raw[len(raw)-1] != '>' {
		return "", false, false
	}
	inner := strings.TrimSpace(string(raw[1 : len(raw)-1]))
	if inner == "" || inner[0] == '!' || inner[0] == '?' {
		return "", false, false
	}
	if strings.HasPrefix(inner, "/") {
		closing = true
		inner = strings.TrimSpace(inner[1:])
	}
	if strings.HasSuffix(inner, "/") {
		selfClose = true
		inner = strings.TrimSpace(strings.TrimSuffix(inner, "/"))
	}
	if inner == "" {
		return "", closing, selfClose
	}
	nameEnd := len(inner)
	for i, c := range inner {
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			nameEnd = i
			break
		}
	}
	return strings.ToLower(inner[:nameEnd]), closing, selfClose
}
