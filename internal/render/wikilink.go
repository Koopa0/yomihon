package render

import (
	"fmt"
	"html"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

// unwrittenTarget renders a link whose target does not exist yet, saying which
// name failed both in a title a pointer reveals and in offscreen text a reader
// who is listening receives. heading is the section name the fragment parsed
// to; when present the explanation says so, since otherwise a shorter name than
// the one the author typed is reported failing with nothing saying why.
func unwrittenTarget(target, display, heading string, lang wording.Lang) string {
	fmtString := wording.UnwrittenTargetFmt
	if namesAFile(target) {
		fmtString = wording.UnwrittenFileFmt
	}
	reason := fmt.Sprintf(fmtString.In(lang), target)
	if heading != "" {
		reason += fmt.Sprintf(wording.UnwrittenHeadingFmt.In(lang), heading)
	}
	return degradedSpan("wikilink-broken", display, reason, lang)
}

// namesAFile reports whether a citation's target names a file rather than a
// note, so an absence can be described as what the author was reaching for. It
// recognises the extensions this program already reads and infers nothing wider
// from a dot: a note called "Go sync.Pool" is not a picture.
func namesAFile(target string) bool {
	return IsPicture(target) || IsPDF(target)
}

// degradedSpan is the one shape every citation the page could not follow takes:
// the author's words, an explanation a pause reveals, and the same string again
// offscreen, so a pointer and a voice are never told different things.
func degradedSpan(class, display, reason string, lang wording.Lang) string {
	escaped := html.EscapeString(reason)
	return `<span class="` + class + `" title="` + escaped + `">` +
		html.EscapeString(display) + `<span class="` + offscreenNoteClass + `">` +
		html.EscapeString(wording.ParenOpen.In(lang)) + escaped +
		html.EscapeString(wording.ParenClose.In(lang)) + `</span></span>`
}

// titleOnlyTarget says the name reached a note's declared title, which is not a
// name a link follows. One holder is named and several are counted rather than
// listed, because this sentence is spoken aloud and a list of paths read
// mid-sentence follows nobody; the panel beside the article names them all.
func titleOnlyTarget(target, display string, held []string, lang wording.Lang) string {
	var reason string
	if len(held) == 1 {
		reason = fmt.Sprintf(wording.TitleOnlyTargetFmt.In(lang), target, held[0])
	} else {
		reason = fmt.Sprintf(wording.TitleOnlySeveralFmt.In(lang), target, len(held))
	}
	escaped := html.EscapeString(reason)
	return `<span class="wikilink-broken wikilink-title-only" title="` + escaped + `">` +
		html.EscapeString(display) + `<span class="` + offscreenNoteClass + `">` +
		html.EscapeString(wording.ParenOpen.In(lang)) + escaped + html.EscapeString(wording.ParenClose.In(lang)) + `</span></span>`
}

// ambiguousTarget says the name placed more than one file, and which, in both
// the pointer's pause and the offscreen text, from one source. The element
// stays non-interactive and takes no tab stop, since a focus stop that opened
// nothing would be an affordance in name only.
func ambiguousTarget(target, display string, candidates []string, lang wording.Lang) string {
	reason := fmt.Sprintf(wording.AmbiguousTargetFmt.In(lang), target, strings.Join(candidates, ", "))
	return degradedSpan("wikilink-ambiguous", display, reason, lang)
}

// offscreenNoteClass names the element an explanation is carried out of sight
// in. The heading scan removes elements of this name before reading a heading's
// words, so an explanation never becomes the name of the section around it.
const offscreenNoteClass = "y-offscreen"

// fragmentMiss says how a link's fragment failed to place. A missing block
// withdraws the address and the link leads to the whole note; a missing section
// keeps the address exactly as the author wrote it.
type fragmentMiss uint8

const (
	fragmentPlaced fragmentMiss = iota
	fragmentSectionMissing
	fragmentBlockMissing
)

// degradedLink renders a link whose note was found and whose fragment was not.
// It stays an anchor, because the address still leads somewhere real, and adds
// the reason the way an unwritten name carries its own: a title for whoever can
// point at it, an offscreen sentence for whoever is listening.
func degradedLink(href string, link graph.Wikilink, miss fragmentMiss, lang wording.Lang) string {
	reason := wording.BlockNotFound.In(lang)
	if miss == fragmentSectionMissing {
		reason = fmt.Sprintf(wording.SectionNotFoundFmt.In(lang), link.Heading)
	}
	escaped := html.EscapeString(reason)
	return `<a href="` + attributeEscaper.Replace(href) + `" class="wikilink wikilink-degraded" title="` + escaped + `">` +
		html.EscapeString(link.Display) + `<span class="` + offscreenNoteClass + `">` + html.EscapeString(wording.ParenOpen.In(lang)) + escaped + html.EscapeString(wording.ParenClose.In(lang)) + `</span></a>`
}

// The marker delimiters below are written by the placeholder builders and read
// back by the substitution pass, in one place so the two cannot drift.
const (
	blockMarkOpen   = "<!--yomihon-block:"
	blockMarkClose  = "-->"
	inlineMarkOpen  = "\ue000"
	inlineMarkClose = "\ue001"
	wideMarkOpen    = "\ue002"
	wideMarkClose   = "\ue003"
)

// blockPlaceholder is the reserved marker substituted into markdown source for
// first-party markup standing on its own line — a transclusion, a callout. An
// HTML comment is one indivisible raw node, so it arrives as a standalone block
// without the authored-HTML allowlist admitting a general-purpose element.
func blockPlaceholder(i int) string {
	return blockMarkOpen + strconv.Itoa(i) + blockMarkClose
}

// inlinePlaceholder is the marker for first-party markup that belongs inside a
// line — a rendered wikilink. It cannot be the comment above, because a comment
// opening a line is a markdown HTML block that swallows the paragraph. These are
// private-use code points, carried through as ordinary text and wrapped with the
// words beside them, delimited at both ends so no marker nests in another.
func inlinePlaceholder(i int) string {
	return inlineMarkOpen + strconv.Itoa(i) + inlineMarkClose
}

// blockMarkupPlaceholder is the marker for first-party markup found inside a
// line whose rendered form is a block — a transclusion's container written
// mid-sentence. It travels like the inline marker and the substitution pass then
// parts the paragraph around it, because a div left inside <p> makes a browser
// close the paragraph and orphan the rest of the sentence. Its delimiters are a
// second pair of private-use code points, unreadable as the first.
func blockMarkupPlaceholder(i int) string {
	return wideMarkOpen + strconv.Itoa(i) + wideMarkClose
}

// markerIsBlockShaped is the one decision of which marker pair an inline
// substitution travels under, asked by the pass that plants a marker and by the
// pass that redeems it. It reads the markup itself: every string substituted
// this way is renderer-written, and a div is the one block element any opens with.
func markerIsBlockShaped(markup string) bool {
	return strings.HasPrefix(markup, "<div")
}

// placeholderFor spells the marker an inline substitution is planted under,
// putting the index between the delimiter pair its markup's shape selects.
func placeholderFor(i int, markup string) string {
	if markerIsBlockShaped(markup) {
		return blockMarkupPlaceholder(i)
	}
	return inlinePlaceholder(i)
}

// inlinePlaceholderRunes are stripped from authored text before any real marker
// exists, since a note containing them could otherwise select or relocate
// renderer-owned markup. They are unassigned, so nothing readable is lost.
const inlinePlaceholderRunes = "\ue000\ue001\ue002\ue003"

// blockMarkupMarker matches one planted block-markup marker in rendered HTML.
// The runes are stripped from authored text before markers exist, so every
// occurrence is the renderer's own.
var blockMarkupMarker = regexp.MustCompile("\ue002\\d+\ue003")

// substituteBlocks redeems every marker the preprocessing passes planted, in one
// walk over the rendered document, which prices a page at its length rather than
// at its length times its citation count. Each planted index is redeemed once, at
// its first occurrence, and only under the marker pair its markup's own shape
// selects; bytes that merely resemble a marker pass through as written. Redeemed
// markup is spliced and never rescanned.
func substituteBlocks(htmlOut string, blocks, inline []string) string {
	htmlOut = partParagraphsAtBlockMarkup(htmlOut)
	if len(blocks) == 0 && len(inline) == 0 {
		return htmlOut
	}
	var out strings.Builder
	grown := len(htmlOut)
	for _, b := range blocks {
		grown += len(b)
	}
	for _, b := range inline {
		grown += len(b)
	}
	out.Grow(grown)

	usedBlock := make([]bool, len(blocks))
	usedInline := make([]bool, len(inline))
	pos := 0
	nextComment := strings.Index(htmlOut, blockMarkOpen)
	nextInline := strings.Index(htmlOut, inlineMarkOpen)
	nextWide := strings.Index(htmlOut, wideMarkOpen)
	splice := func(cand, end int, markup string) {
		out.WriteString(htmlOut[pos:cand])
		out.WriteString(markup)
		pos = end
	}
	for {
		cand := leftmostMark(nextComment, nextInline, nextWide)
		if cand < 0 {
			break
		}
		// Each family's cursor is re-aimed only past its own consumed or refused
		// opening. No family's opening delimiter occurs inside another's marker,
		// so redeeming one never steps over another's pending opening.
		switch cand {
		case nextComment:
			if markup, end, ok := redeemBlockAt(htmlOut, cand, blocks, usedBlock); ok {
				splice(cand, end, markup)
			}
			nextComment = nextMark(htmlOut, blockMarkOpen, max(pos, cand+1))
		case nextInline:
			if markup, end, ok := redeemInlineAt(htmlOut, cand, inline, usedInline, false); ok {
				splice(cand, end, markup)
			}
			nextInline = nextMark(htmlOut, inlineMarkOpen, max(pos, cand+1))
		default:
			if markup, end, ok := redeemInlineAt(htmlOut, cand, inline, usedInline, true); ok {
				splice(cand, end, markup)
			}
			nextWide = nextMark(htmlOut, wideMarkOpen, max(pos, cand+1))
		}
	}
	out.WriteString(htmlOut[pos:])
	return out.String()
}

// leftmostMark picks the leftmost pending marker opening among the three
// families, each -1 when its family has no further occurrence; -1 means the
// document holds no candidate at all.
func leftmostMark(a, b, c int) int {
	best := a
	if b >= 0 && (best < 0 || b < best) {
		best = b
	}
	if c >= 0 && (best < 0 || c < best) {
		best = c
	}
	return best
}

// nextMark reports where a family's opening delimiter next occurs at or after
// from, -1 when it never does again.
func nextMark(doc, open string, from int) int {
	i := strings.Index(doc[from:], open)
	if i < 0 {
		return -1
	}
	return from + i
}

// markerIndex reads the planted index and closing delimiter completing a marker
// whose opening delimiter ends just before after. The builders write canonical
// decimal, so any other spelling is document text that happens to open like a
// marker and names none. end is the offset just past the closer, relative to after.
func markerIndex(after, closing string) (idx, end int, ok bool) {
	w := 0
	for w < len(after) && '0' <= after[w] && after[w] <= '9' {
		w++
	}
	if w == 0 || w > 9 || (after[0] == '0' && w > 1) {
		return 0, 0, false
	}
	if !strings.HasPrefix(after[w:], closing) {
		return 0, 0, false
	}
	for _, c := range []byte(after[:w]) {
		idx = idx*10 + int(c-'0')
	}
	return idx, w + len(closing), true
}

// redeemBlockAt redeems the block marker opening at cand when it completes
// with an index that was planted and not yet redeemed, marking it redeemed
// and returning its markup with the document offset just past the marker.
func redeemBlockAt(doc string, cand int, blocks []string, used []bool) (markup string, end int, ok bool) {
	idx, n, matched := markerIndex(doc[cand+len(blockMarkOpen):], blockMarkClose)
	if !matched || idx >= len(blocks) || used[idx] {
		return "", 0, false
	}
	used[idx] = true
	return blocks[idx], cand + len(blockMarkOpen) + n, true
}

// redeemInlineAt is redeemBlockAt for the two private-use marker pairs. wide
// selects the pair, and a marker redeems only when its markup's shape agrees, so
// an index travelling under one pair is never surrendered to the other.
func redeemInlineAt(doc string, cand int, inline []string, used []bool, wide bool) (markup string, end int, ok bool) {
	open, closing := inlineMarkOpen, inlineMarkClose
	if wide {
		open, closing = wideMarkOpen, wideMarkClose
	}
	idx, n, matched := markerIndex(doc[cand+len(open):], closing)
	if !matched || idx >= len(inline) || used[idx] || markerIsBlockShaped(inline[idx]) != wide {
		return "", 0, false
	}
	used[idx] = true
	return inline[idx], cand + len(open) + n, true
}

// partParagraphsAtBlockMarkup opens every paragraph around the block-markup
// markers it carries: the words before keep a paragraph, the marker stands
// between paragraphs, and the words after open another. A side that is only
// whitespace gets no paragraph. Only the bare <p> the markdown pass emits is
// read; a table cell or tight list item holds a div legally and is left alone,
// and so is a heading, where an excerpt in a section title has no meaning here.
func partParagraphsAtBlockMarkup(htmlOut string) string {
	if !strings.Contains(htmlOut, "\ue002") {
		return htmlOut
	}
	var out strings.Builder
	rest := 0
	for {
		open := strings.Index(htmlOut[rest:], "<p>")
		if open < 0 {
			break
		}
		open += rest
		closing := strings.Index(htmlOut[open:], "</p>")
		if closing < 0 {
			break
		}
		closing += open
		inner := htmlOut[open+len("<p>") : closing]
		out.WriteString(htmlOut[rest:open])
		if strings.Contains(inner, "\ue002") {
			writePartedParagraph(&out, inner)
		} else {
			out.WriteString(htmlOut[open : closing+len("</p>")])
		}
		rest = closing + len("</p>")
	}
	out.WriteString(htmlOut[rest:])
	return out.String()
}

// writePartedParagraph writes one paragraph's content with each block-markup
// marker set free between paragraph-wrapped runs of the remaining words.
func writePartedParagraph(out *strings.Builder, inner string) {
	last := 0
	for _, loc := range blockMarkupMarker.FindAllStringIndex(inner, -1) {
		writeParagraphSegment(out, inner[last:loc[0]])
		out.WriteString(inner[loc[0]:loc[1]])
		last = loc[1]
	}
	writeParagraphSegment(out, inner[last:])
}

// writeParagraphSegment wraps one run of a parted paragraph's words, and
// writes nothing for a run that holds no words to wrap.
func writeParagraphSegment(out *strings.Builder, segment string) {
	if strings.TrimSpace(segment) == "" {
		return
	}
	out.WriteString("<p>")
	out.WriteString(segment)
	out.WriteString("</p>")
}

var (
	pipeTableLine = regexp.MustCompile(`^\s*\|.*\|\s*$`)
	wikilinkToken = regexp.MustCompile(`!?\[\[[^\[\]]+\]\]`)
)

// fenceOpen reports whether line opens a fenced code block, with its marker byte
// ('`' or '~') and the trimmed info string, when line's first non-whitespace
// characters are three or more of the same fence character.
func fenceOpen(line string) (marker byte, info string, ok bool) {
	t := strings.TrimLeft(line, " \t")
	switch {
	case strings.HasPrefix(t, "```"):
		marker = '`'
	case strings.HasPrefix(t, "~~~"):
		marker = '~'
	default:
		return 0, "", false
	}
	return marker, strings.TrimSpace(strings.TrimLeft(t, string(marker))), true
}

// fenceCloses reports whether line is a bare fence-close line for marker:
// once trimmed, every character is marker and there are at least 3.
func fenceCloses(line string, marker byte) bool {
	t := strings.TrimSpace(line)
	if len(t) < 3 {
		return false
	}
	return strings.Count(t, string(marker)) == len(t)
}

// htmlBlockKind is one of the HTML blocks CommonMark ends at a particular
// string rather than at an empty line: what opens it, and the text whose
// appearance anywhere on a later line ends it.
type htmlBlockKind struct {
	opens *regexp.Regexp
	ends  string
}

// htmlBlockKinds is the closed set of block openings that survive an empty
// line, which is the set a scan stopping mid-block has to be able to close.
// CommonMark's other two HTML blocks — the known-tag one and the bare-tag one —
// are deliberately absent, because an empty line ends both and a callout's body
// is laid into its note with one after it.
//
// The comment and declaration kinds are here on purpose rather than left to
// chance. Both happen to be terminated today by the marker this renderer plants
// to close a callout, which carries "-->" and ">" in its own spelling; that is a
// coincidence of how the marker is written, not a property of the block, and it
// would go away the moment the marker were respelled.
var htmlBlockKinds = []htmlBlockKind{
	{regexp.MustCompile(`(?i)^ {0,3}<pre(?:[ \t/>]|$)`), "</pre>"},
	{regexp.MustCompile(`(?i)^ {0,3}<script(?:[ \t/>]|$)`), "</script>"},
	{regexp.MustCompile(`(?i)^ {0,3}<style(?:[ \t/>]|$)`), "</style>"},
	{regexp.MustCompile(`(?i)^ {0,3}<textarea(?:[ \t/>]|$)`), "</textarea>"},
	{regexp.MustCompile(`^ {0,3}<!--`), "-->"},
	{regexp.MustCompile(`^ {0,3}<\?`), "?>"},
	{regexp.MustCompile(`^ {0,3}<!\[CDATA\[`), "]]>"},
	{regexp.MustCompile(`^ {0,3}<![A-Za-z]`), ">"},
}

// leadingSpace is the indentation a line carries, which a close this scan writes
// for that line's block has to repeat to stay inside the same container.
func leadingSpace(line string) string {
	return line[:len(line)-len(strings.TrimLeft(line, " \t"))]
}

// looksRisky reports whether line contains one of the dialect patterns the
// preprocessing passes would otherwise convert — worth one warning when found
// inside a fence, never acted on.
func looksRisky(line string) bool {
	if strings.Contains(line, "[[") {
		return true
	}
	if calloutStartPattern.MatchString(line) {
		return true
	}
	return pipeTableLine.MatchString(line)
}

// preprocess is the single fence-aware, line-based pass that converts the
// Obsidian dialect elements goldmark's parser has no concept of — wikilinks,
// embeds, callouts, mermaid fences — before goldmark runs, treating fence contents
// as literal source. One note is one call, so the markup it plants is numbered
// once across everything that note holds. At most one risky-fence diagnostic is
// recorded per scan: a callout's body is scanned on its own, and a transcluded
// embed is a call of its own, so each has its own budget.
func (r *Pipeline) preprocess(body string, allowEmbed embedPolicy, col *collector) (out string, blocks, inline []string) {
	st := &preprocessState{
		lines:  strings.Split(body, "\n"),
		quoted: r.indentedCodeLines(body),
		marks:  &markers{},
	}
	r.scan(st, allowEmbed, col)
	// Joined as scanned rather than through source: nothing follows a note's own
	// last line, so a block still open there has nothing left to swallow and the
	// end of the document ends it. Writing a close here would put a line the
	// author never typed into their last block.
	return strings.Join(st.kept, "\n"), st.marks.blocks, st.marks.inline
}

// scan reads st's lines to their end, consuming each dialect construct it
// recognizes and keeping every other line as its author wrote it.
func (r *Pipeline) scan(st *preprocessState, allowEmbed embedPolicy, col *collector) {
	st.kept = make([]string, 0, len(st.lines))

	for st.i < len(st.lines) {
		switch {
		case st.inFence:
			r.scanFenceLine(st, col)
		// While an HTML block is still running, every line belongs to it: a fence
		// marker or a callout opener inside one is raw text, not the start of
		// anything, so neither pass may claim it.
		case st.htmlEnd == "" && r.tryOpenFence(st):
			// handled: either entered a fence, or fully consumed a
			// mermaid block — see tryOpenFence.
		case st.htmlEnd == "" && r.tryConsumeCallout(st, allowEmbed, col):
			// handled: a known-type callout block was consumed.
		default:
			// An indented code block hands its line to the reader as written, so
			// a bracket pair on it is syntax being shown and stays as typed. The
			// address a line carries is read there all the same, because a code
			// block is somewhere in the note a reader can be sent to and the
			// adjudicator counts one written there. A block opener shown that way
			// is being displayed rather than opened, so it starts nothing this
			// scan would afterwards have to close.
			line := st.lines[st.i]
			if !st.quoted[st.i] {
				st.trackHTMLBlock(line)
				line = r.convertWikilinks(line, allowEmbed, col, &st.marks.inline)
			}
			// A transcluded body's blocks belong to the note it came from, so an
			// excerpt brings no addresses into the page reading it; embeds being
			// allowed is exactly the state of being the note's own text. Links
			// convert first, so a caret inside one is never read as an address.
			if allowEmbed == embedsAllowed {
				line = markBlockAnchor(line, col.page, &st.marks.inline)
			}
			st.kept = append(st.kept, line)
			st.i++
		}
	}
}

// markers is the table of renderer-written markup one note's preprocessing
// plants, kept apart from the scan that plants it because a callout's body is
// read by a nested scan and both number into this one table. An index therefore
// names the same markup wherever it was planted, which is what the substitution
// pass reads it as.
type markers struct {
	blocks []string
	inline []string
}

// plantBlock files markup that stands on its own line and answers with the
// index its marker carries.
func (m *markers) plantBlock(markup string) int {
	m.blocks = append(m.blocks, markup)
	return len(m.blocks) - 1
}

// preprocessState carries one scan's running state so the per-construct
// handlers below (fence, mermaid, callout) can share it without a long,
// repeated parameter list. A callout's body is read by a scan with a state of
// its own over that body's lines, sharing only the marker table.
type preprocessState struct {
	lines []string
	i     int

	// quoted holds the lines an indented code block shows as written. It is
	// read once per body rather than per line, since answering it needs the
	// whole body's block structure.
	quoted map[int]bool

	kept  []string
	marks *markers

	inFence   bool
	fenceByte byte
	// htmlEnd is the text whose appearance ends the HTML block this scan is
	// inside, empty when it is inside none.
	htmlEnd string
	// pendingClose is the whole line, its container's indentation included, that
	// would end whatever block the scan is currently inside. A fenced code block
	// and an HTML block both set it, and never both at once, since neither can
	// open inside the other. Empty when the scan is inside no such block.
	pendingClose       string
	riskyFenceReported bool
}

// source is the scanned lines as one piece of markdown, with any block the scan
// never left closed off at the end. A callout's body is written inside a
// blockquote, and a block inside one is closed by the end of that quote; the
// body's source is laid into the note's own, so a block left open would read on
// past it and take the rest of the note — the renderer's own closing markup
// included — as its own content. Only a caller that lays this source in front
// of more text needs that; the note's own scan joins its lines untouched.
func (st *preprocessState) source() string {
	if st.pendingClose == "" {
		return strings.Join(st.kept, "\n")
	}
	closed := make([]string, 0, len(st.kept)+1)
	closed = append(closed, st.kept...)
	closed = append(closed, st.pendingClose)
	return strings.Join(closed, "\n")
}

// trackHTMLBlock records whether line leaves an HTML block open. It only reads
// the line; what the line renders as is decided elsewhere, so noticing a block
// here changes nothing about the page unless the scan stops while one is open.
func (st *preprocessState) trackHTMLBlock(line string) {
	if st.htmlEnd != "" {
		if strings.Contains(line, st.htmlEnd) {
			st.htmlEnd, st.pendingClose = "", ""
		}
		return
	}
	for i := range htmlBlockKinds {
		kind := &htmlBlockKinds[i]
		opener := kind.opens.FindStringIndex(line)
		if opener == nil {
			continue
		}
		// The end may stand on the opening line, which is where every read-aloud
		// marker and every marker this renderer plants ends.
		if strings.Contains(line[opener[1]:], kind.ends) {
			return
		}
		st.htmlEnd = kind.ends
		st.pendingClose = leadingSpace(line) + kind.ends
		return
	}
}

// scanFenceLine handles one line while st.inFence is true: it either closes the
// fence or is fence content, checked once for a risky-looking pattern.
func (r *Pipeline) scanFenceLine(st *preprocessState, col *collector) {
	line := st.lines[st.i]
	switch {
	case fenceCloses(line, st.fenceByte):
		st.inFence, st.pendingClose = false, ""
	case !st.riskyFenceReported && looksRisky(line):
		st.riskyFenceReported = true
		col.report(&Diagnostic{
			Kind:    DiagRiskyFence,
			Message: "wikilink/callout/table syntax found inside a fenced code block; left untouched",
		})
	}
	st.kept = append(st.kept, line)
	st.i++
}

// tryOpenFence opens a fence at the current line and reports whether the line was
// a fence opener. A ```mermaid fence is instead consumed whole by consumeMermaid,
// never left open for scanFenceLine.
func (r *Pipeline) tryOpenFence(st *preprocessState) bool {
	marker, info, ok := fenceOpen(st.lines[st.i])
	if !ok {
		return false
	}
	if strings.EqualFold(info, "mermaid") {
		r.consumeMermaid(st, marker)
		return true
	}
	// A close of this scan's own writing has to look like the opener: as many
	// marker characters, since a shorter run does not close a longer fence, and
	// the same indentation, since a fence opened inside a list item is closed
	// from inside that item — a close at the margin ends the item instead and
	// opens a fence of its own.
	opener := strings.TrimLeft(st.lines[st.i], " \t")
	run := len(opener) - len(strings.TrimLeft(opener, string(marker)))
	st.inFence, st.fenceByte = true, marker
	st.pendingClose = leadingSpace(st.lines[st.i]) + strings.Repeat(string(marker), run)
	st.kept = append(st.kept, st.lines[st.i])
	st.i++
	return true
}

// consumeMermaid consumes a ```mermaid fence up to and including its closing
// fence and replaces the block with a div.mermaid-diagram placeholder carrying
// the raw source twice: HTML-escaped as the element's text, which is what a
// reader without scripting sees, and URL-encoded in data-mermaid-code, whose
// charset needs no further attribute escaping. The browser side decodes that
// attribute and replaces the content with the rendered diagram.
func (r *Pipeline) consumeMermaid(st *preprocessState, marker byte) {
	st.i++
	start := st.i
	for st.i < len(st.lines) && !fenceCloses(st.lines[st.i], marker) {
		st.i++
	}
	src := strings.Join(st.lines[start:st.i], "\n")
	if st.i < len(st.lines) {
		st.i++ // consume the closing fence line
	}
	//nolint:gocritic // sprintfQuotedString false positive: the quotes are HTML attribute syntax, not Go string quoting; url.QueryEscape's output charset (letters, digits, "-_.~%+") never needs further escaping
	block := fmt.Sprintf(
		`<div class="mermaid-diagram" data-mermaid-code="%s">%s</div>`,
		url.QueryEscape(src),
		html.EscapeString(src),
	)
	st.kept = append(st.kept, "", blockPlaceholder(st.marks.plantBlock(block)), "")
}

// tryConsumeCallout consumes a known-type callout block starting at the current
// line. An unknown type records a diagnostic and reports false, leaving the line
// to goldmark's own blockquote parsing so nothing is silently dropped.
func (r *Pipeline) tryConsumeCallout(st *preprocessState, allowEmbed embedPolicy, col *collector) bool {
	typ, fold, title, ok := calloutStart(st.lines[st.i])
	if !ok {
		return false
	}
	bucket, defaultTitle := calloutBucketOf(typ)
	if bucket == bucketUnknown {
		col.report(&Diagnostic{
			Kind:    DiagUnknownCallout,
			Target:  typ,
			Message: fmt.Sprintf("unknown callout type %q; rendered as a plain blockquote", typ),
		})
		return false
	}

	st.i++
	var bodyLines []string
	for st.i < len(st.lines) && strings.HasPrefix(strings.TrimSpace(st.lines[st.i]), ">") {
		if _, _, _, isNew := calloutStart(st.lines[st.i]); isNew {
			break
		}
		bodyLines = append(bodyLines, quotePrefix.ReplaceAllString(st.lines[st.i], ""))
		st.i++
	}

	// The two halves of the callout's own markup are planted as markers and the
	// body's source is laid between them, so the note's one parse sees the whole
	// of what its author wrote — a footnote reference and the definition it names
	// among it, wherever either was written. The body is scanned by a scan of its
	// own because it has its own fence state, its own reading of which lines an
	// indented code block shows as written, and its own diagnostic budget; the
	// two scans plant into one marker table.
	open, closing := calloutShell(bucket, defaultTitle, fold, title)
	openMark := st.marks.plantBlock(open)
	bodySource := strings.Join(bodyLines, "\n")
	body := &preprocessState{
		lines:  bodyLines,
		quoted: r.indentedCodeLines(bodySource),
		marks:  st.marks,
	}
	r.scan(body, allowEmbed, col)
	closeMark := st.marks.plantBlock(closing)

	st.kept = append(st.kept, "", blockPlaceholder(openMark), "", body.source(), "", blockPlaceholder(closeMark), "")
	return true
}

// convertWikilinks scans one source line for [[...]] and ![[...]] and replaces
// each with its rendered form.
func (r *Pipeline) convertWikilinks(text string, allowEmbed embedPolicy, col *collector, inline *[]string) string {
	// A code span is quoted text: the author is showing what a wikilink looks
	// like, not making one. Converting it would print a placeholder in place of
	// the syntax and report a broken link the author never wrote.
	spans := codeSpanRanges(text)
	return replaceOutside(text, spans, wikilinkToken, func(start int, m string) string {
		embed := strings.HasPrefix(m, "!")
		raw := m
		open := start
		if embed {
			raw = m[1:]
			open++
		}
		// A backslash escape is the author writing the syntax to be shown, not
		// followed, so the match is left to goldmark's own escape handling. The
		// same rule decides what the adjudicator counts as a citation.
		if graph.EscapedWikilinkAt(text, open) {
			return m
		}
		inner := raw[2 : len(raw)-2]
		link, ok := graph.ParseWikilink(inner)
		if !ok {
			// [[#heading]] or [[^block]] stripped to empty: a same-file
			// anchor jump, not a cross-file link — render the original
			// display text as plain text, don't attempt to resolve it.
			return html.EscapeString(link.Display)
		}
		if embed {
			embedHTML := r.renderEmbed(link, m, allowEmbed, col)
			*inline = append(*inline, embedHTML)
			return placeholderFor(len(*inline)-1, embedHTML)
		}
		linkHTML := r.renderWikilink(link, col)
		*inline = append(*inline, linkHTML)
		return placeholderFor(len(*inline)-1, linkHTML)
	})
}

// replaceOutside applies fn to every match of re lying wholly outside the given
// ranges, leaving everything else byte-identical. fn receives the match's
// starting byte offset alongside the matched bytes, so a caller can look behind it.
func replaceOutside(text string, skip [][2]int, re *regexp.Regexp, fn func(start int, m string) string) string {
	var out strings.Builder
	last := 0
	for _, loc := range re.FindAllStringIndex(text, -1) {
		if withinAny(skip, loc[0], loc[1]) {
			continue
		}
		out.WriteString(text[last:loc[0]])
		out.WriteString(fn(loc[0], text[loc[0]:loc[1]]))
		last = loc[1]
	}
	out.WriteString(text[last:])
	return out.String()
}

// withinAny reports whether the half-open range start..end lies wholly inside one
// of the given ranges, so the rewrite pass and the embed scan ignore one set of
// quoted text rather than each keeping an answer.
func withinAny(ranges [][2]int, start, end int) bool {
	for _, r := range ranges {
		if start >= r[0] && end <= r[1] {
			return true
		}
	}
	return false
}

// escapedVaultPath percent-escapes each segment of a vault-relative path on its
// own, leaving "/" as the separator, so a literal slash inside a segment can
// never be read as one and the route patterns receive the path they expect.
func escapedVaultPath(p string) string {
	segments := strings.Split(p, "/")
	for i, s := range segments {
		segments[i] = url.PathEscape(s)
	}
	return strings.Join(segments, "/")
}

// notesHref builds the reading page's URL for a vault-relative path.
func notesHref(p string) string {
	return "/notes/" + escapedVaultPath(p)
}

// rawHref builds the raw-bytes URL for a vault-relative path — the route a
// browser fetches when the file itself, not a page about it, is wanted.
func rawHref(p string) string {
	return "/raw/" + escapedVaultPath(p)
}

// sectionHref is notesHref plus the place inside the destination a link named.
// What it returns is a URL, which the attribute it goes into escapes on its own.
// A section's fragment is built by the same call that stamps the destination's
// heading ids; a block address takes precedence when the author
// wrote both, and is percent-escaped.
// Only the block scan is authoritative, so a block it cannot find is withdrawn,
// while a section address always survives and a miss is reported only when a
// second, generous scan agrees the name is nowhere. Nothing is claimed about an
// uncaptured body, and anything that is not a note gets no fragment at all.
func (r *Pipeline) sectionHref(relPath string, link graph.Wikilink, col *collector) (string, fragmentMiss) {
	href := notesHref(relPath)
	if !vault.IsMarkdown(relPath) {
		return href, fragmentPlaced
	}
	switch {
	case link.Block != "":
		address := "^" + link.Block
		body, ok := r.transclusions.Transclusion(relPath)
		if !ok {
			return href, fragmentPlaced
		}
		// A probe into another note reports only on the address it came to
		// check. Whatever that note's own markers do is its own page's news.
		strippedTarget, _ := stripObsidianComments(body)
		if _, found := blockSlice(strippedTarget, link.Block); !found {
			col.report(&Diagnostic{
				Kind:    DiagLinkFragmentMissing,
				Target:  link.Target,
				Block:   link.Block,
				Message: fmt.Sprintf("no block in %q matched %q; the link leads to the note itself", relPath, address),
			})
			return href, fragmentBlockMissing
		}
		return href + "#" + url.PathEscape(blockAnchorID(address)), fragmentPlaced
	case link.Heading != "":
		addressed := href + "#" + graph.SectionID(link.Heading)
		body, ok := r.transclusions.Transclusion(relPath)
		if !ok {
			return addressed, fragmentPlaced
		}
		stripped, _ := stripObsidianComments(body)
		if _, found := headingSlice(stripped, link.Heading); found > 0 {
			return addressed, fragmentPlaced
		}
		if headingAnchorMayExist(stripped, link.Heading) {
			return addressed, fragmentPlaced
		}
		if r.embedBringsHeading(stripped, link.Heading) {
			return addressed, fragmentPlaced
		}
		col.report(&Diagnostic{
			Kind:    DiagLinkSectionMissing,
			Target:  link.Target,
			Section: link.Heading,
			Message: fmt.Sprintf("no heading in %q matched %q; the address is left as written and may land at the top of the note", relPath, link.Heading),
		})
		return addressed, fragmentSectionMissing
	}
	return href, fragmentPlaced
}

// renderWikilink renders a plain (non-embed) [[target|display]] as one open/close
// tag pair around escaped text, routed through a reserved placeholder so authored
// raw HTML and renderer-owned markup never share a trust decision. Only a name
// that placed exactly one file gets a fragment; a name placing several is not
// answered here, since this renderer never picks one of them.
func (r *Pipeline) renderWikilink(link graph.Wikilink, col *collector) string {
	res := r.idx.Resolve(link.Target)
	switch res.Kind {
	case graph.KindUnique:
		href, miss := r.sectionHref(res.RelPath, link, col)
		if miss != fragmentPlaced {
			return degradedLink(href, link, miss, col.page.lang)
		}
		//nolint:gocritic // sprintfQuotedString false positive: the quotes are HTML attribute syntax, not Go string quoting; the href is percent-escaped as a URL and then escaped for the attribute, and the name is html.EscapeString'd
		return fmt.Sprintf(`<a href="%s" class="wikilink">%s</a>`, attributeEscaper.Replace(href), html.EscapeString(link.Display))
	case graph.KindAmbiguous:
		col.report(&Diagnostic{
			Kind: DiagWikilinkAmbiguous, Target: link.Target,
			Message: fmt.Sprintf("wikilink %q is ambiguous: %s", link.Target, strings.Join(res.Candidates, ", ")),
		})
		return ambiguousTarget(link.Target, link.Display, res.Candidates, col.page.lang)
	case graph.KindUnresolved:
		// The name found nothing, which is as far as the resolver goes: a title
		// is deliberately not a name a link follows. Whether it is nonetheless
		// some note's title changes the repair, so it changes what the page says.
		if held := r.titles.TitledBy(link.Target); len(held) > 0 {
			col.report(&Diagnostic{
				Kind: DiagWikilinkTitleOnly, Target: link.Target, Section: link.Heading,
				Message: fmt.Sprintf("wikilink %q names the title of: %s", link.Target, strings.Join(held, ", ")),
			})
			return titleOnlyTarget(link.Target, link.Display, held, col.page.lang)
		}
		col.report(&Diagnostic{
			Kind: DiagWikilinkBroken, Target: link.Target, Section: link.Heading,
			Message: fmt.Sprintf("wikilink %q does not resolve to any note or file", link.Target),
		})
		return unwrittenTarget(link.Target, link.Display, link.Heading, col.page.lang)
	default:
		panic("render: unknown graph.Kind: " + res.Kind.String())
	}
}

// renderEmbed renders ![[target]] for the parsed link. A unique markdown-note
// target is transcluded through the same pipeline, but only one level deep: with
// allowEmbed embedsDenied it renders as plain wikilink text instead. A fragment
// narrows the transclusion to the named section or block. A unique picture paints
// inline from the raw-bytes route; any other non-markdown target gets a labelled
// placeholder, and an ambiguous or unresolved one degrades like a broken link.
func (r *Pipeline) renderEmbed(link graph.Wikilink, source string, allowEmbed embedPolicy, col *collector) string {
	if allowEmbed == embedsDenied {
		// Transclusion stops one level down, so this excerpt's own citation is
		// shown as a link, and the excerpt around it says so once.
		col.report(&Diagnostic{
			Kind: DiagEmbedNotExpanded, Target: link.Target, Section: link.Heading,
			Message: fmt.Sprintf("embed of %q inside a transcluded body was rendered as a link", link.Target),
		})
		return r.renderWikilink(link, col)
	}

	target := link.Target
	res := r.idx.Resolve(target)
	switch res.Kind {
	case graph.KindUnresolved:
		col.report(&Diagnostic{
			Kind: DiagWikilinkBroken, Target: target, Section: link.Heading,
			Message: fmt.Sprintf("embed target %q does not resolve", target),
		})
		return unwrittenTarget(target, source, link.Heading, col.page.lang)
	case graph.KindAmbiguous:
		col.report(&Diagnostic{
			Kind: DiagWikilinkAmbiguous, Target: target,
			Message: fmt.Sprintf("embed target %q is ambiguous: %s", target, strings.Join(res.Candidates, ", ")),
		})
		return ambiguousTarget(target, source, res.Candidates, col.page.lang)
	case graph.KindUnique:
		if IsPicture(res.RelPath) {
			//nolint:gocritic // sprintfQuotedString false positive: the quotes are HTML attribute syntax, not Go string quoting; the href is percent-escaped as a URL and then escaped for the attribute, and the name is html.EscapeString'd
			return fmt.Sprintf(`<img src="%s" alt="%s">`,
				attributeEscaper.Replace(rawHref(res.RelPath)), html.EscapeString(path.Base(res.RelPath)))
		}
		if !vault.IsMarkdown(res.RelPath) {
			return mediaStub(res.RelPath, col.page.lang)
		}
		body, ok := r.transclusions.Transclusion(res.RelPath)
		if !ok {
			col.report(&Diagnostic{
				Kind: DiagWikilinkBroken, Target: target,
				Message: fmt.Sprintf("embed target %q is unavailable in the captured generation", res.RelPath),
			})
			return degradedSpan("wikilink-broken", source, wording.EmbedUnreadable.In(col.page.lang), col.page.lang)
		}
		slice, matches := embedScope(link, res.RelPath, body, col)
		if matches == 0 {
			// The author named one place in the note and the note has no such
			// place, so nothing of it is shown: the block says which address
			// failed, and the provenance line is the way on to the note for a
			// reader who wants the rest. It is not recorded as transcluded,
			// because no words of the note reached the page.
			return `<div class="` + embedClass(true) + `">` + embedSourceLine(res.RelPath, col.page.lang) +
				withheldNotice(res.RelPath, fragmentOf(link), col.page.lang) + `</div>`
		}
		// The excerpt is recorded exactly as cut, at the one point where expansion
		// happens: the freshness stamp derives from these entries, so a wider or
		// narrower one would move for words the page never showed.
		col.page.transcluded = append(col.page.transcluded, transcludedExcerpt{
			path:    res.RelPath,
			matches: matches,
			slice:   slice,
		})
		inner := r.render(slice, embedsDenied, col.page)
		col.diags = append(col.diags, inner.Diagnostics...)
		heldBack := false
		for _, d := range inner.Diagnostics {
			if d.Kind == DiagEmbedNotExpanded {
				heldBack = true
				break
			}
		}
		// An image inside a transcluded body was written relative to the note it
		// came from, so it is resolved here, where that path is still known.
		return `<div class="` + embedClass(false) + `">` + embedSourceLine(res.RelPath, col.page.lang) +
			repeatedNotice(link.Heading, matches, col.page.lang) + notExpandedNotice(heldBack, col.page.lang) +
			resolveAssetHrefs(inner.HTML, res.RelPath) + `</div>`
	default:
		panic("render: unknown graph.Kind: " + res.Kind.String())
	}
}

// embedScope cuts a transcluded body to the section or block the embed's
// fragment named, with the one cut every excerpt is made with, and reports on
// the cut. An address the note does not answer to comes back with no matches
// and nothing cut: the author named one place, and the whole note would be a
// wider answer than the one they wrote. A section name the note carries more
// than once is cut at the first and counted, so the page can say which one it
// is showing. Where the excerpt's edges are unruled the narrower reading is
// taken. This is the only place a transcluded body's Obsidian %% comments come
// off, so no later pass can reopen a marker this one ruled literal.
func embedScope(link graph.Wikilink, resPath, body string, col *collector) (scoped string, matches int) {
	stripped, unclosed := stripObsidianComments(body)
	if unclosed != 0 {
		unclosedDiagnostic := unclosedCommentDiagnostic(unclosed)
		col.report(&unclosedDiagnostic)
	}
	scoped, matches = excerptOf(stripped, fragmentOf(link))
	switch {
	case matches == 0 && link.Block != "":
		col.report(&Diagnostic{
			Kind:    DiagEmbedFragmentMissing,
			Target:  link.Target,
			Block:   link.Block,
			Message: fmt.Sprintf("no block in %q matched %q; the excerpt is withheld", resPath, "^"+link.Block),
		})
	case matches == 0:
		col.report(&Diagnostic{
			Kind:    DiagEmbedFragmentMissing,
			Target:  link.Target,
			Section: link.Heading,
			Message: fmt.Sprintf("no heading in %q matched %q; the excerpt is withheld", resPath, link.Heading),
		})
	case matches > 1:
		col.report(&Diagnostic{
			Kind:    DiagEmbedFragmentRepeated,
			Target:  link.Target,
			Section: link.Heading,
			Message: fmt.Sprintf("%d headings in %q matched %q; the first is shown", matches, resPath, link.Heading),
		})
	}
	return scoped, matches
}

// embedClass marks the block an embed leaves behind when its fragment matched
// nothing, so a withheld excerpt is visible in the article rather than only in
// the diagnostics face.
func embedClass(withheld bool) string {
	if withheld {
		return "embed embed--withheld"
	}
	return "embed"
}

// mediaStub stands where an embedded file the page cannot show inline would
// have been: the sentence saying so, carrying a link to the file's own page
// where it names it, so the reader is left a way to the thing they asked for.
// The sentence comes from the dictionary the rest of this article's own
// sentences come from, in the language the reader chose; it and the element
// around the file's name are this program's own bytes, while the name inside is
// the author's and is escaped.
func mediaStub(relPath string, lang wording.Lang) string {
	//nolint:gocritic // sprintfQuotedString false positive: the quotes are HTML attribute syntax, not Go string quoting; the href is percent-escaped as a URL and then escaped for the attribute, and the name is html.EscapeString'd
	named := fmt.Sprintf(`<a href="%s">%s</a>`,
		attributeEscaper.Replace(notesHref(relPath)), html.EscapeString(path.Base(relPath)))
	return `<div class="embed-media">` + fmt.Sprintf(wording.EmbedMediaFmt.In(lang), named) + `</div>`
}

// embedSourceLine opens an excerpt with the name of the note its words came from,
// as a link there, because the page around it otherwise never says whose words
// these are. The name is the file's own, the one a citation resolves by.
func embedSourceLine(relPath string, lang wording.Lang) string {
	return `<p class="embed__source">` + html.EscapeString(wording.EmbedSourceFrom.In(lang)) +
		`<a href="` + attributeEscaper.Replace(notesHref(relPath)) + `">` + html.EscapeString(noteName(relPath)) + `</a></p>`
}

// withheldNotice states, where the excerpt would have stood, which address the
// note does not answer to. The diagnostics rail says so too, but it collapses
// on a narrow viewport, and a block that shows nothing says nothing about
// whether anything was asked for unless it says this.
func withheldNotice(relPath, fragment string, lang wording.Lang) string {
	return `<p class="embed__note">` + html.EscapeString(ExcerptWithheld(relPath, fragment, lang)) + `</p>`
}

// repeatedNotice states, above the excerpt, that the fragment named a section the
// source note carries more than once and that the first is what follows, so a
// reader can tell a chosen excerpt from the only candidate. Nothing is said for a
// single match, which would drown the notice that matters.
func repeatedNotice(heading string, matches int, lang wording.Lang) string {
	if matches < 2 {
		return ""
	}
	return `<p class="embed__note">` +
		html.EscapeString(fmt.Sprintf(wording.EmbedRepeatedHeadingFmt.In(lang), matches, heading)) +
		`</p>`
}

// notExpandedNotice states that a citation inside this excerpt was written as an
// embed and is shown as a link. It is said once for the excerpt, because the
// reason is a property of the excerpt rather than of any one link in it.
func notExpandedNotice(heldBack bool, lang wording.Lang) string {
	if !heldBack {
		return ""
	}
	return `<p class="embed__note">` + html.EscapeString(wording.EmbedNotExpanded.In(lang)) + `</p>`
}

// headingAnchorMayExist reports whether any line of body could stamp the id a
// section address names. It is deliberately generous where headingSlice is exact,
// because its only job is to stop a miss being reported about a heading the page
// really carries: a false yes costs one unreported fragment, a false no tells a
// reader a section they can see is not there. Both ways of writing a heading
// count, an underlined paragraph included.
func headingAnchorMayExist(body, heading string) bool {
	want := graph.SectionID(heading)
	var paragraph []string
	for line := range strings.SplitSeq(body, "\n") {
		candidate := withoutQuoteAndListMarkers(line)
		if m := atxHeadingLine.FindStringSubmatch(candidate); m != nil {
			if graph.SectionID(headingSourceText(m[2], len(m[1]))) == want {
				return true
			}
			paragraph = nil
			continue
		}
		// A row of dashes closing a paragraph underlines it rather than drawing a
		// rule, which is the order the page reads them in too.
		if len(paragraph) > 0 && setextUnderline.MatchString(candidate) {
			if graph.SectionID(headingSourceText(strings.Join(paragraph, "\n"), setextLevel(candidate))) == want {
				return true
			}
			paragraph = nil
			continue
		}
		if candidate == "" {
			paragraph = nil
			continue
		}
		paragraph = append(paragraph, candidate)
	}
	return false
}

// withoutQuoteAndListMarkers reduces a line to the text a heading could have been
// written as, removing nested quote markers and one list marker. The page keeps a
// heading inside both, so a scan stopping at either would miss ids it stamps.
func withoutQuoteAndListMarkers(line string) string {
	candidate := strings.TrimSpace(line)
	for quotedLine.MatchString(candidate) {
		candidate = strings.TrimSpace(strings.TrimPrefix(candidate, ">"))
	}
	if loc := listItemLine.FindString(candidate); loc != "" {
		candidate = strings.TrimSpace(candidate[len(loc):])
	}
	return candidate
}

// embedBringsHeading reports whether a heading absent from this body arrives
// inside one of the notes it embeds, which the scans reading one note's own source
// cannot see while the page stamps its id all the same. The walk goes exactly one
// level, the limit the render already has, and reads the source the way the pass
// that expands it will. Only a note can bring headings.
func (r *Pipeline) embedBringsHeading(body, heading string) bool {
	spans := codeSpanRanges(body)
	for _, loc := range wikilinkToken.FindAllStringIndex(body, -1) {
		if body[loc[0]] != '!' || withinAny(spans, loc[0], loc[1]) {
			continue
		}
		if graph.EscapedWikilinkAt(body, loc[0]+1) {
			continue
		}
		link, ok := graph.ParseWikilink(body[loc[0]+3 : loc[1]-2])
		if !ok {
			continue
		}
		res := r.idx.Resolve(link.Target)
		if res.Kind != graph.KindUnique || !vault.IsMarkdown(res.RelPath) {
			continue
		}
		embedded, ok := r.transclusions.Transclusion(res.RelPath)
		if !ok {
			continue
		}
		stripped, _ := stripObsidianComments(embedded)
		if headingBroughtBy(link, stripped, heading) {
			return true
		}
	}
	return false
}

// headingBroughtBy reports whether the heading is inside the part of an embedded
// body the embed actually shows: a fragment narrows the excerpt, so a heading
// outside it never reaches the page, and a fragment matching nothing withholds
// the excerpt, so nothing of that body reaches the page at all. Both readings
// are the ones embedScope applies for display, from the same cut.
func headingBroughtBy(link graph.Wikilink, embedded, heading string) bool {
	scoped, matches := excerptOf(embedded, fragmentOf(link))
	if matches == 0 {
		return false
	}
	if _, found := headingSlice(scoped, heading); found > 0 {
		return true
	}
	return headingAnchorMayExist(scoped, heading)
}
