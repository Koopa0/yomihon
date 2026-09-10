package render

// A note body's searchable plain text, walked off the same markdown engine the
// HTML pipeline uses. It lives here rather than beside its consumer because it
// needs this package's dialect passes, and a second copy of those would be free
// to disagree with the renderer about what a note says.

import (
	"bytes"
	"strings"
	"unicode"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
)

// plainParser is a minimal goldmark parser used only to walk a note body for
// PlainText. Table and TaskList are on so cell and task text arrive as clean text
// nodes; linkify is off, so a bare URL stays a plain text node indexed verbatim.
// Footnotes are on because a definition whose text has no spaces — every CJK one —
// otherwise parses as a link reference definition and no search could find it.
var plainParser = goldmark.New(goldmark.WithExtensions(extension.Table, extension.TaskList, extension.Footnote)).Parser()

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

// PlainBlocks returns the searchable text of a note body, the exclusive end
// offset of each block-level contribution in that text, and the half-open
// byte ranges that came from a fenced code block. The text is byte-identical
// to PlainText. A phrase whose match starts in one block and ends in another
// is one the browser's text directive cannot find, because those words render
// in different elements; a wrap inside one paragraph is not that case.
//
// Fence ranges sit in the same coordinate space as the text so a later match
// can tell a hit that landed in source from one that landed in prose. The
// bodies stay in the text: people search for code snippets. Eligibility of a
// fence as the excerpt is a decision for the match, not this walk.
func PlainBlocks(body string) (plain string, blockEnds []int, fenceRanges [][2]int) {
	src := []byte(plainPreprocess(body))
	doc := plainParser.Parse(text.NewReader(src))

	var w plainWalk
	if err := ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		return walkPlain(&w, n, entering, src)
	}); err != nil {
		// Unreachable: walkPlain never returns a non-nil error. If a future
		// goldmark change ever makes Walk itself fail, fall back to the raw
		// body (whitespace-collapsed) so a note is never left unsearchable.
		collapsed := strings.Join(strings.Fields(body), " ")
		if collapsed == "" {
			return "", nil, nil
		}
		return collapsed, []int{len(collapsed)}, nil
	}
	w.closeBlock()
	w.flushReadings()
	return w.result()
}

// plainWalk is the accumulator walkPlain writes. The text is what PlainText
// always returned; blockEnds are the exclusive ends of each block before the
// leading and trailing space are trimmed off. fenceRanges are the half-open
// spans written from a fenced code block, in the same raw coordinates.
type plainWalk struct {
	b           strings.Builder
	blockEnds   []int
	fenceRanges [][2]int
	// readings holds <rt>/<rtc> text until the block's base text has been
	// closed, so a visible phrase is not split by its furigana. rubyAnno and
	// rubyParen are the open-tag depths that decide where the next text node
	// goes; <rp> is only a parenthesis fallback and is dropped.
	readings  strings.Builder
	rubyAnno  int
	rubyParen int
}

func (w *plainWalk) result() (plain string, blockEnds []int, fenceRanges [][2]int) {
	raw := w.b.String()
	plain = strings.TrimSpace(raw)
	if plain == "" {
		return "", nil, nil
	}
	lead := len(raw) - len(strings.TrimLeftFunc(raw, unicode.IsSpace))
	blockEnds = shiftEnds(w.blockEnds, lead, len(plain))
	if n := len(blockEnds); n == 0 || blockEnds[n-1] != len(plain) {
		blockEnds = append(blockEnds, len(plain))
	}
	return plain, blockEnds, shiftRanges(w.fenceRanges, lead, len(plain))
}

// shiftEnds maps exclusive ends recorded in the raw builder onto the
// trimmed text, dropping empties and keeping them strictly increasing.
func shiftEnds(ends []int, lead, length int) []int {
	var out []int
	for _, end := range ends {
		adj := end - lead
		if adj <= 0 {
			continue
		}
		if adj > length {
			adj = length
		}
		if n := len(out); n > 0 && out[n-1] >= adj {
			continue
		}
		out = append(out, adj)
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

func (w *plainWalk) closeBlock() {
	s := w.b.String()
	end := len(s)
	for end > 0 && s[end-1] == '\n' {
		end--
	}
	if end == 0 {
		return
	}
	if n := len(w.blockEnds); n > 0 && w.blockEnds[n-1] >= end {
		return
	}
	w.blockEnds = append(w.blockEnds, end)
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

// plainPreprocess rewrites the two Obsidian-dialect constructs goldmark has no
// concept of into plain text before parsing: a wikilink or embed becomes "target
// display", both, so a filename search hits through a display alias, and a
// callout marker line loses its marker while keeping the title. It is
// fence-aware, so a link written inside a code sample stays literal.
func plainPreprocess(body string) string {
	// The retrieval projections report nothing: a corpus entry is not a page,
	// and a fault in a note is the reading page's news to break.
	body, _ = stripObsidianComments(body)
	lines := strings.Split(body, "\n")
	inFence := false
	var fenceByte byte
	var fenceLen int
	for i, line := range lines {
		switch {
		case inFence:
			if fenceCloses(line, fenceByte, fenceLen) {
				inFence = false
			}
		default:
			if marker, n, _, ok := fenceOpen(line); ok {
				inFence, fenceByte, fenceLen = true, marker, n
			} else {
				lines[i] = plainLine(line)
			}
		}
	}
	return strings.Join(lines, "\n")
}

// plainLine normalizes one non-fence line: it strips a callout marker (keeping
// the title) and rewrites wikilinks to plain "target display" text.
func plainLine(line string) string {
	if m := calloutStartPattern.FindStringSubmatch(line); m != nil {
		// Drop the marker, keep the callout's title. The body lines that follow
		// keep their quote marker and are collected as ordinary quoted text.
		line = m[3]
	}
	return replaceWikilinksPlain(line)
}

// replaceWikilinksPlain replaces every [[...]]/![[...]] token in line with its
// clean target and display text (deduplicated when they are equal), using the
// same target/display split the renderer uses (graph.SplitWikilink).
func replaceWikilinksPlain(line string) string {
	return wikilinkToken.ReplaceAllStringFunc(line, func(token string) string {
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
	})
}

// walkPlain appends one AST node's contribution to w. It never returns an
// error (the ast.Walk error path in PlainBlocks is therefore unreachable).
func walkPlain(w *plainWalk, n ast.Node, entering bool, source []byte) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	switch n.Kind() {
	case ast.KindRawHTML:
		// The tags are not content. Text between them arrives as separate text
		// nodes rather than children, so skipping here drops only the tags.
		// Ruby is the exception: <rt> (and <rtc>) hold a reading that must
		// not sit between the base characters a reader can see.
		if raw, ok := n.(*ast.RawHTML); ok {
			for i := range raw.Segments.Len() {
				seg := raw.Segments.At(i)
				w.seeMarkup(seg.Value(source))
			}
		}
		return ast.WalkSkipChildren, nil
	case ast.KindHTMLBlock:
		return ast.WalkSkipChildren, nil
	case ast.KindFencedCodeBlock, ast.KindCodeBlock:
		// Code content lives in the node's line segments, not in child Text
		// nodes; write it directly and do not descend — code contents are
		// searchable (people search for code snippets). A fenced block also
		// records the span it wrote, so a later excerpt can decline it when
		// the same words sit in prose. An indented code block is not a fence.
		writeSeparator(w)
		start := w.b.Len()
		writeBlockLines(&w.b, n, source)
		if n.Kind() == ast.KindFencedCodeBlock {
			w.recordFence(start)
		}
		return ast.WalkSkipChildren, nil
	case ast.KindText:
		if t, ok := n.(*ast.Text); ok {
			w.writeVisible(t.Value(source))
			if t.SoftLineBreak() || t.HardLineBreak() {
				w.writeBreak()
			}
		}
	case ast.KindString:
		if s, ok := n.(*ast.String); ok {
			w.writeVisible(s.Value)
		}
	case ast.KindAutoLink:
		if a, ok := n.(*ast.AutoLink); ok {
			w.writeVisible(a.URL(source))
		}
	default:
		if n.Type() == ast.TypeBlock {
			// Separate block-level text so tokens from adjacent blocks (a
			// heading then its paragraph) do not run together.
			writeSeparator(w)
		}
	}
	return ast.WalkContinue, nil
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

// writeVisible appends one text node's bytes to the corpus: into the held
// readings while inside <rt>/<rtc>, nowhere while inside <rp>, and into the
// block's base text otherwise.
func (w *plainWalk) writeVisible(p []byte) {
	switch {
	case w.rubyParen > 0:
		return
	case w.rubyAnno > 0:
		w.readings.Write(p)
	default:
		w.b.Write(p)
	}
}

func (w *plainWalk) writeBreak() {
	switch {
	case w.rubyParen > 0:
		return
	case w.rubyAnno > 0:
		w.readings.WriteByte('\n')
	default:
		w.b.WriteByte('\n')
	}
}

// seeMarkup notes a raw HTML tag so the following text nodes are routed.
// A self-closing tag has no following text of its own and is ignored.
func (w *plainWalk) seeMarkup(raw []byte) {
	name, closing, selfClose := markupName(raw)
	if selfClose {
		return
	}
	switch name {
	case "rt", "rtc":
		if closing {
			if w.rubyAnno > 0 {
				w.rubyAnno--
			}
			return
		}
		if w.rubyAnno == 0 && w.readings.Len() > 0 {
			s := w.readings.String()
			if s[len(s)-1] != ' ' && s[len(s)-1] != '\n' {
				w.readings.WriteByte(' ')
			}
		}
		w.rubyAnno++
	case "rp":
		if closing {
			if w.rubyParen > 0 {
				w.rubyParen--
			}
			return
		}
		w.rubyParen++
	}
}

// flushReadings writes held ruby readings as their own block after the base
// text they came from. A later match on the base phrase can then land on the
// sentence the page shows, and a match on the reading still finds the note.
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
	w.b.WriteString(w.readings.String())
	w.readings.Reset()
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

// writeBlockLines appends the raw source of a node's line segments (used for
// code blocks, whose content is not held as child text nodes).
func writeBlockLines(b *strings.Builder, n ast.Node, source []byte) {
	lines := n.Lines()
	for i := range lines.Len() {
		seg := lines.At(i)
		b.Write(seg.Value(source))
	}
}
