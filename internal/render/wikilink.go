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

// unwrittenTarget renders a link whose target does not exist yet. The styling
// gives it a help cursor, which is a promise that pausing on it explains
// something, so the explanation has to be attached here — without it the reader
// hovers, then clicks, then reads nothing, and concludes the page is broken
// rather than that the note is unwritten. It says which name failed, because
// the words on the page are often not that name, and it says it the way a
// person would rather than in the vocabulary of the link syntax.
// A pause is a mouse gesture, so the same sentence goes into the page as text
// a reader who is listening receives, and one who is on a phone can reach. The
// words are carried out of sight rather than dropped: read aloud, the passage
// says the name is unwritten right where the name occurs, which is what the
// cursor promises to anyone who can point at it.
// heading is the section name the link's fragment parsed to, empty when the
// author wrote none. When present, the explanation also states that the text
// after "#" was read as a section name: a file whose own name contains the
// mark can never be linked whole, and without this sentence the reader is
// told a shorter name than the one they typed failed, with nothing saying
// where the rest of it went.
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
// note, so an absence can be described as the thing the author was reaching
// for. The kinds it recognises are the ones whose extension this program
// already reads when it chooses how to display something: a name it does not
// recognise is left a note, which is what every such name was called before.
//
// Nothing wider is inferred from the presence of a dot. The vault holds a note
// called "Go sync.Pool", and a rule that took any trailing dotted word for a
// file type would tell its author no such picture exists.
func namesAFile(target string) bool {
	return IsPicture(target) || IsPDF(target)
}

// degradedSpan is the one shape every citation the page could not follow takes:
// the words the author wrote, an explanation a pause reveals, and the same
// explanation again where a reader who is listening receives it. The two
// carry one string because a pointer and a voice must not be told different
// things, and the class is what says which kind of failure this was.
func degradedSpan(class, display, reason string, lang wording.Lang) string {
	escaped := html.EscapeString(reason)
	return `<span class="` + class + `" title="` + escaped + `">` +
		html.EscapeString(display) + `<span class="` + offscreenNoteClass + `">` +
		html.EscapeString(wording.ParenOpen.In(lang)) + escaped +
		html.EscapeString(wording.ParenClose.In(lang)) + `</span></span>`
}

// titleOnlyTarget says the name reached a note's declared title. The note is
// there; the name is not one a link follows, which is what the vault's own
// reader does with it too.
//
// One holder is named. Several are counted rather than listed: this sentence
// is carried in a title attribute and read aloud out of an offscreen span, and
// a list of paths spoken mid-sentence is not something anyone can follow. The
// panel beside the article names them all, where a reader can look down the
// list at their own pace.
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

// ambiguousTarget says the name placed more than one file, and which. The
// sentence goes to both places the reader may meet it — the pause a pointer
// makes, and the words carried out of sight for anyone listening — from one
// source, so the two cannot come to say different things. The element stays
// non-interactive and takes no tab stop: the route for a reader who is not
// using a pointer is the marker they can see and the panel beside the article,
// and a focus stop that opened nothing would be an affordance in name only.
//
// This is not what a citation naming a note's title is told. That name reaches
// a note under a name links do not follow and an alias repairs it; this name
// is followed and arrives in several places at once.
func ambiguousTarget(target, display string, candidates []string, lang wording.Lang) string {
	reason := fmt.Sprintf(wording.AmbiguousTargetFmt.In(lang), target, strings.Join(candidates, ", "))
	return degradedSpan("wikilink-ambiguous", display, reason, lang)
}

// offscreenNoteClass names the element the explanation above is carried out of
// sight in. The heading scan removes elements of this name before it reads a
// heading's words, so the sentence explaining a link never becomes the name of
// the section around it; the two are written against one constant so they
// cannot drift apart. A note that authors the same markup is display input and
// keeps it — that text arrives escaped, which is not what this names.
const offscreenNoteClass = "y-offscreen"

// fragmentMiss says how a link's fragment failed to place. The two misses
// degrade differently on purpose and their sentences differ with them: a
// missing block withdraws the address and the link leads to the whole note, a
// missing section keeps the address exactly as the author wrote it.
type fragmentMiss uint8

const (
	fragmentPlaced fragmentMiss = iota
	fragmentSectionMissing
	fragmentBlockMissing
)

// degradedLink renders a link whose note was found and whose fragment was not.
// It stays an anchor: the address still leads somewhere real, which is a
// smaller fault than a name with no target at all, and the reader should be
// able to follow it. What it adds is the reason, carried the way an unwritten
// name carries its own — a title for whoever can point at it, and an offscreen
// sentence for whoever is listening. The panel beside the article states the
// same fact about the same link, but a reader who finds it there has already
// followed the link and arrived somewhere they did not mean to be.
func degradedLink(href string, link graph.Wikilink, miss fragmentMiss, lang wording.Lang) string {
	reason := wording.BlockNotFound.In(lang)
	if miss == fragmentSectionMissing {
		reason = fmt.Sprintf(wording.SectionNotFoundFmt.In(lang), link.Heading)
	}
	escaped := html.EscapeString(reason)
	return `<a href="` + href + `" class="wikilink wikilink-degraded" title="` + escaped + `">` +
		html.EscapeString(link.Display) + `<span class="` + offscreenNoteClass + `">` + html.EscapeString(wording.ParenOpen.In(lang)) + escaped + html.EscapeString(wording.ParenClose.In(lang)) + `</span></a>`
}

// The marker delimiters below are written by the placeholder builders and
// read back by the substitution pass, and they live in one place so the two
// sides cannot drift: a marker planted under one spelling and sought under
// another would leave renderer-owned markup unredeemed on the page.
const (
	blockMarkOpen   = "<!--yomihon-block:"
	blockMarkClose  = "-->"
	inlineMarkOpen  = "\ue000"
	inlineMarkClose = "\ue001"
	wideMarkOpen    = "\ue002"
	wideMarkClose   = "\ue003"
)

// blockPlaceholder is the reserved marker substituted into markdown source for
// first-party markup that stands on its own line — a transclusion, a callout.
// An HTML comment is one indivisible raw node, so it reaches the reader as the
// standalone block its caller surrounded with blank lines, without asking the
// authored-HTML allowlist to admit a general-purpose element. renderBody
// neutralizes authored copies of this prefix before preprocess can create real
// placeholders.
func blockPlaceholder(i int) string {
	return blockMarkOpen + strconv.Itoa(i) + blockMarkClose
}

// inlinePlaceholder is the marker for first-party markup that belongs inside a
// line — a rendered wikilink. It cannot be the comment above: a comment opening
// a line is a markdown HTML block, which swallows the rest of the paragraph
// with it, so prose that began with a link arrived unwrapped and ran into the
// paragraph below it on the page. These are private-use code points instead,
// which the markdown pass carries through as ordinary text and therefore wraps
// exactly as it wraps the words beside them. The index is delimited at both
// ends so one marker can never be found inside another.
func inlinePlaceholder(i int) string {
	return inlineMarkOpen + strconv.Itoa(i) + inlineMarkClose
}

// blockMarkupPlaceholder is the marker for first-party markup found inside a
// line whose rendered form is nonetheless a block — a transclusion's container
// written mid-sentence. It travels through the markdown pass exactly as the
// inline marker does, wrapped into whatever paragraph the words around it
// make, and the substitution pass then parts that paragraph around it. A div
// left inside <p> is markup a browser repairs by closing the paragraph at the
// div, which turned the rest of the sentence into a bare text node no
// paragraph owned. The delimiters are a second pair of unassigned private-use
// code points, so the two marker kinds can never be read as one another.
func blockMarkupPlaceholder(i int) string {
	return wideMarkOpen + strconv.Itoa(i) + wideMarkClose
}

// markerIsBlockShaped is the one decision of which marker pair an inline
// substitution travels under, asked by the pass that plants a marker and
// again by the pass that redeems it, so the two cannot disagree. It reads the
// markup itself: every string substituted this way is written by this
// renderer — authored text arrives escaped — and a div is the one block
// element any of them opens with; links, anchors, degraded citations, and
// inline images are phrasing and stay in their sentence.
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
// exists, for the same reason the comment prefix is neutralized: a vault note
// containing them could otherwise select or relocate renderer-owned markup.
// Both marker alphabets are covered. They are unassigned to any character, so
// nothing a reader meant to see is lost with them.
const inlinePlaceholderRunes = "\ue000\ue001\ue002\ue003"

// blockMarkupMarker matches one planted block-markup marker in rendered HTML.
// The runes are stripped from authored text before markers exist, so every
// occurrence is the renderer's own.
var blockMarkupMarker = regexp.MustCompile("\ue002\\d+\ue003")

// substituteBlocks redeems every marker the preprocessing passes planted, in
// one walk over the rendered document. It used to redeem them one at a time,
// each substitution rescanning and reallocating the whole document, which
// priced a page at its citation count multiplied by its length — a heavily
// cited note took seconds to render, and its cost quadrupled each time it
// doubled. One walk prices the page at its length.
//
// What is redeemed is unchanged by the walk's shape: each planted index once,
// at its first occurrence in the document — goldmark decides where that is; a
// link inside a footnote definition renders at the foot of the document
// however early it was planted — and
// only under the marker pair the markup's own shape selects. Bytes that
// merely resemble a marker pass through as written: an index nothing planted,
// digits no builder spells (the builders write canonical decimal), a pair
// whose shape disagrees with its markup. Redeemed markup is spliced and never
// rescanned — nothing the renderer substitutes carries a marker of its own,
// because nested bodies redeem theirs before arriving and authored text has
// the marker alphabet stripped and the comment prefix neutralized before any
// marker exists.
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
		// Each family's cursor is re-aimed only past its own consumed or
		// refused opening: no family's opening delimiter can occur inside
		// another family's marker — the comment marker is plain text with no
		// private-use rune in it, and the two rune pairs put their runes only
		// at a marker's ends, around plain digits — so redeeming one family's
		// marker never steps over another's pending opening.
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

// markerIndex reads the planted index and closing delimiter that complete a
// marker whose opening delimiter ends just before after. The builders write
// indexes in canonical decimal, so any other spelling — an empty run, a
// leading zero, a run longer than any planted count — is document text that
// happens to open like a marker, and names none. end is the offset just past
// the closing delimiter, relative to after.
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
// selects which pair is being read, and the marker redeems only when its
// markup's own shape agrees — an index that travels under one pair is never
// surrendered to the other's spelling.
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
// markers it carries: the words before the marker keep a paragraph of their
// own, the marker stands between paragraphs where its block belongs, and the
// words after it open another. A side that is only whitespace gets no
// paragraph at all — an empty <p></p> is not something the author wrote.
//
// Only the bare <p> the markdown pass emits is read; the renderer's own
// paragraphs carry a class and are already outside any marker. Positions
// without a paragraph — a table cell, a tight list item — hold a div legally
// and are left alone. A heading is also left alone: what a section title
// containing an excerpt should even mean has no answer here, so the marker
// stays where the author put it.
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

// fenceOpen reports whether line opens a fenced code block: its marker
// byte ('`' or '~') and the info string following it (trimmed), when
// line's first non-whitespace characters are 3+ of the same fence
// character.
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

// looksRisky reports whether line contains one of the dialect patterns
// (wikilink, callout, GFM table row) the preprocessing passes would
// otherwise convert — worth a single warning when found inside a fence,
// never acted on.
func looksRisky(line string) bool {
	if strings.Contains(line, "[[") {
		return true
	}
	if calloutStartPattern.MatchString(line) {
		return true
	}
	return pipeTableLine.MatchString(line)
}

// preprocess is the single fence-aware, line-based pass that converts
// this vault's Obsidian dialect elements goldmark's own parser has no
// concept of — wikilinks, embeds, callouts, and mermaid fences — before
// goldmark ever runs. It tracks fenced-code-block state (``` / ~~~) and
// treats its contents as literal source. Obsidian comments have already been
// removed by the shared source transform. At most one risky-fence
// diagnostic is recorded per call (one warning, not one per
// occurrence) — this dedup is scoped to a single
// preprocess call, not threaded across the recursive calls render makes
// for a callout body or a transcluded embed, so a note with risky fences
// in more than one such structurally-separate region can produce more
// than one diagnostic overall. That is deliberate rather than a
// limitation: the warning names a place the author has to look at, and a
// region is that place, so each one saying so once is the accurate
// report and a single page-wide warning would hide the others.
func (r *Pipeline) preprocess(body string, allowEmbed embedPolicy, col *collector) (out string, blocks, inline []string) {
	st := &preprocessState{lines: strings.Split(body, "\n")}
	st.kept = make([]string, 0, len(st.lines))

	for st.i < len(st.lines) {
		switch {
		case st.inFence:
			r.scanFenceLine(st, col)
		case r.tryOpenFence(st):
			// handled: either entered a fence, or fully consumed a
			// mermaid block — see tryOpenFence.
		case r.tryConsumeCallout(st, allowEmbed, col):
			// handled: a known-type callout block was consumed.
		default:
			line := r.convertWikilinks(st.lines[st.i], allowEmbed, col, &st.inline)
			// A transcluded body's blocks are the blocks of the note it came
			// from, so an excerpt brings no addresses into the page reading
			// it — otherwise a name the host note does not have would land a
			// reader inside someone else's paragraph. Embeds being allowed is
			// exactly the state of being the note's own text.
			//
			// Links are converted first: a wikilink is a placeholder by the
			// time the address is looked for, so a caret written inside one is
			// never mistaken for the line's own address.
			if allowEmbed == embedsAllowed {
				line = markBlockAnchor(line, col.page, &st.inline)
			}
			st.kept = append(st.kept, line)
			st.i++
		}
	}
	return strings.Join(st.kept, "\n"), st.blocks, st.inline
}

// preprocessState carries preprocess's running scan state so the
// per-construct handlers below (fence, mermaid, callout) can share it
// without a long, repeated parameter list.
type preprocessState struct {
	lines []string
	i     int

	kept   []string
	blocks []string
	inline []string

	inFence            bool
	fenceByte          byte
	riskyFenceReported bool
}

// scanFenceLine handles one line while st.inFence is true: either it
// closes the fence, or it's fence content, checked once for a
// risky-looking pattern worth a single warning (see preprocess's doc
// comment on the dedup scope).
func (r *Pipeline) scanFenceLine(st *preprocessState, col *collector) {
	line := st.lines[st.i]
	switch {
	case fenceCloses(line, st.fenceByte):
		st.inFence = false
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

// tryOpenFence opens a fence at the current line — a ```mermaid fence is
// instead fully consumed by consumeMermaid and replaced with a block
// placeholder, never left open for scanFenceLine. Reports whether the
// current line was a fence opener at all.
func (r *Pipeline) tryOpenFence(st *preprocessState) bool {
	marker, info, ok := fenceOpen(st.lines[st.i])
	if !ok {
		return false
	}
	if strings.EqualFold(info, "mermaid") {
		r.consumeMermaid(st, marker)
		return true
	}
	st.inFence, st.fenceByte = true, marker
	st.kept = append(st.kept, st.lines[st.i])
	st.i++
	return true
}

// consumeMermaid consumes a ```mermaid fence's body up to and including
// its closing fence (the opening line, at st.lines[st.i], is already
// known to be one) and replaces the whole block with a div.mermaid-diagram
// placeholder carrying the raw source twice: once as the element's text
// content (the no-JS/SSR-fallback presentation — html.EscapeString, so it
// stays valid HTML and human-readable) and once URL-encoded in
// data-mermaid-code (net/url.QueryEscape's output charset — letters,
// digits, "-_.~%+" — never needs further HTML-attribute escaping, so no
// double-encoding risk). assets/js/diagrams.js reads that attribute,
// URL-decodes it, and replaces the element's content with mermaid's
// rendered SVG client-side; the two encodings exist so the same source
// survives both as readable fallback text and as a byte-exact attribute
// round-trip.
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
	st.blocks = append(st.blocks, block)
	st.kept = append(st.kept, "", blockPlaceholder(len(st.blocks)-1), "")
}

// tryConsumeCallout consumes a known-type callout block starting at the
// current line. An unknown type records its Diagnostic but reports
// false — the line is left for ordinary per-line processing and
// goldmark's own native blockquote parsing, so nothing is silently
// dropped (see preprocess's caller doc).
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
	calloutHTML := r.renderCallout(bucket, defaultTitle, fold, title, strings.Join(bodyLines, "\n"), allowEmbed, col)
	st.blocks = append(st.blocks, calloutHTML)
	st.kept = append(st.kept, "", blockPlaceholder(len(st.blocks)-1), "")
	return true
}

// convertWikilinks scans one source line for [[...]] and ![[...]] and replaces
// each with its rendered form.
func (r *Pipeline) convertWikilinks(text string, allowEmbed embedPolicy, col *collector, inline *[]string) string {
	// A code span is quoted text: the author is showing the reader what a
	// wikilink looks like, not making one. Converting it substituted a
	// renderer-owned placeholder that goldmark then escaped as the span's
	// literal content, so the sentence explaining the syntax printed an
	// internal comment instead — and the link the author never made was
	// resolved, so a note that names a target it does not link to collected a
	// broken-link diagnostic for it.
	spans := codeSpanRanges(text)
	return replaceOutside(text, spans, wikilinkToken, func(start int, m string) string {
		embed := strings.HasPrefix(m, "!")
		raw := m
		open := start
		if embed {
			raw = m[1:]
			open++
		}
		// A backslash escape is the author writing the syntax to be shown,
		// not followed. The match is left untouched for goldmark, whose own
		// escape handling prints the literal text — converting it anyway both
		// showed a stray backslash and reported a broken link the author never
		// made. The same rule decides what the adjudicator counts as a
		// citation, which is why it is answered in one place.
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

// replaceOutside applies fn to every match of re that lies wholly outside the
// given ranges, leaving everything else byte-identical. fn receives the
// match's starting byte offset in text alongside the matched bytes, so a
// caller can consult the characters just before the match — returning the
// match unchanged leaves that occurrence byte-identical too.
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

// withinAny reports whether the half-open range start..end lies wholly inside
// one of the given ranges. Both the pass that rewrites wikilinks and the scan
// that looks for the embeds among them have to ignore the same quoted text, so
// they ask the same question here rather than each keeping an answer.
func withinAny(ranges [][2]int, start, end int) bool {
	for _, r := range ranges {
		if start >= r[0] && end <= r[1] {
			return true
		}
	}
	return false
}

// escapedVaultPath percent-escapes each segment of a vault-relative path
// individually (so a literal "/" in the escaped output can never be mistaken
// for a path separator) while leaving "/" itself as the separator between
// segments — matching how the {path...} route patterns expect to receive it
// back.
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
// A link written at a section or a block means that part of the note: without
// the fragment the reader arrives at the top of a note that may run for pages,
// with nothing on screen to say which part they were sent to. It returns a
// string safe to write straight into an href attribute.
//
// A section's fragment is built by slugify — the same function that stamps the
// destination's heading ids — because a second rule for turning heading text
// into an anchor would agree by maintenance accident and disagree by default.
// It emits Unicode letters, digits, and hyphens only, so it needs no escaping.
//
// A block address takes precedence over a section name when the author wrote
// both, the same order the excerpt scan resolves that conflict in.
//
// The two kinds are then checked differently, because only one of the scans is
// authoritative about what the destination answers to. A block anchor is
// stamped by the same excerpt scan that validates it, so a block the scan
// cannot find is a block the page does not carry, and the address is withdrawn
// rather than sending a reader who was promised one paragraph to the top of a
// note. A heading id is stamped by a later pass over the rendered HTML, and
// that pass sees headings this line scan does not — inside a blockquote,
// inside a list item — so the scan can report that it found nothing and cannot
// rule that nothing is there. A section address therefore always survives, and
// a miss is reported only when a second, deliberately generous scan agrees the
// name is nowhere. Where the destination transcludes another note, both scans
// read that note as well, because a heading arriving through an embed is
// written in a different file and stamped on this page all the same; see
// embedBringsHeading for how far that reading goes. Under-reporting a broken
// fragment costs a diagnostic, while over-reporting one withdraws a working
// link and tells the reader a section they can see does not exist.
//
// Where the destination's body was not captured nothing is claimed either
// way — yomihon cannot tell a name that is absent from one it did not read,
// and reporting either would be a statement about a note it never saw.
//
// A block fragment is percent-escaped, because a block name is whatever the
// author typed and can carry a quote or a space, and the caret it opens with is
// not a character a URL may spell literally. A browser resolving the fragment
// decodes it before looking for the id, so the escaped address still finds the
// anchor the page wrote unescaped.
//
// Anything that is not a note gets no fragment at all. A PDF, a canvas, a
// picture is handed to the reader whole and nothing inside it is addressable
// from here; a fragment on one names a place that cannot exist, and because a
// viewer simply ignores what it does not understand, it would look like it had
// worked.
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
		addressed := href + "#" + slugify(link.Heading)
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

// renderWikilink renders a plain (non-embed) [[target|display]]. The
// output is always a single short open/close tag pair around escaped text.
// Its caller still routes it through a reserved placeholder: authored raw
// HTML and renderer-owned markup must never share the same trust decision.
//
// Only a name that placed exactly one file gets a fragment. A name that placed
// none has no page for the fragment to be an offset into, and a name that
// placed several is not answered here at all — this renderer never picks one
// of them.
func (r *Pipeline) renderWikilink(link graph.Wikilink, col *collector) string {
	res := r.idx.Resolve(link.Target)
	switch res.Kind {
	case graph.KindUnique:
		href, miss := r.sectionHref(res.RelPath, link, col)
		if miss != fragmentPlaced {
			return degradedLink(href, link, miss, col.page.lang)
		}
		//nolint:gocritic // sprintfQuotedString false positive: the quotes are HTML attribute syntax, not Go string quoting; sectionHref returns an attribute-safe string, path and fragment both percent-escaped
		return fmt.Sprintf(`<a href="%s" class="wikilink">%s</a>`, href, html.EscapeString(link.Display))
	case graph.KindAmbiguous:
		col.report(&Diagnostic{
			Kind: DiagWikilinkAmbiguous, Target: link.Target,
			Message: fmt.Sprintf("wikilink %q is ambiguous: %s", link.Target, strings.Join(res.Candidates, ", ")),
		})
		return ambiguousTarget(link.Target, link.Display, res.Candidates, col.page.lang)
	case graph.KindUnresolved:
		// The name found nothing, which is as far as the resolver goes: a
		// title is deliberately not a name a link follows. Whether the name is
		// nonetheless some note's title changes what the reader should do —
		// nothing needs writing, an alias on the note they meant makes this
		// link work — so it changes what the page says.
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

// renderEmbed renders ![[target]] for the parsed link. A unique
// markdown-note target is transcluded through the same rendering pipeline
// (render), wrapped in a distinctly-styled container — but only one level
// deep: when allowEmbed is embedsDenied (this call is itself inside an
// already-transcluded note's body), the embed renders as plain
// wikilink-style text instead of expanding again (see embedPolicy's doc
// comment for why this alone prevents runaway/cyclic chains). A fragment
// on the link narrows the transclusion to the named section or block —
// see embedScope. A unique picture target paints inline
// from the raw-bytes route — the same place a markdown image of the same
// file already loads from, so the two spellings of one request agree.
// Any other unique non-markdown target (PDF, canvas, ...) gets a labelled
// placeholder rather than being drawn into the note's body; the placeholder's
// filename links to the file's own page, so the reader can still open it.
// Ambiguous/unresolved get the same diagnostic-styled treatment as a broken
// wikilink.
func (r *Pipeline) renderEmbed(link graph.Wikilink, source string, allowEmbed embedPolicy, col *collector) string {
	if allowEmbed == embedsDenied {
		// Transclusion stops one level down, so this excerpt's own citation is
		// shown as a link. It leads where the author pointed, which is why it
		// is still worth following; what is lost is that they asked for the
		// words themselves, and the excerpt around this says so once.
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
			//nolint:gocritic // sprintfQuotedString false positive: the quotes are HTML attribute syntax, not Go string quoting; rawHref percent-escapes the path and the name is html.EscapeString'd
			return fmt.Sprintf(`<img src="%s" alt="%s">`,
				rawHref(res.RelPath), html.EscapeString(path.Base(res.RelPath)))
		}
		if !vault.IsMarkdown(res.RelPath) {
			//nolint:gocritic // sprintfQuotedString false positive: the quotes are HTML attribute syntax, not Go string quoting; notesHref percent-escapes the path and the name is html.EscapeString'd
			return fmt.Sprintf(`<div class="embed-media">[Embedded media: <a href="%s">%s</a> — inline display not yet supported]</div>`,
				notesHref(res.RelPath), html.EscapeString(path.Base(res.RelPath)))
		}
		body, ok := r.transclusions.Transclusion(res.RelPath)
		if !ok {
			col.report(&Diagnostic{
				Kind: DiagWikilinkBroken, Target: target,
				Message: fmt.Sprintf("embed target %q is unavailable in the captured generation", res.RelPath),
			})
			return degradedSpan("wikilink-broken", source, wording.EmbedUnreadable.In(col.page.lang), col.page.lang)
		}
		body, unmatched, repeated := embedScope(link, res.RelPath, body, col)
		// The excerpt is recorded exactly as cut, at the one point where
		// expansion happens: the freshness stamp derives from these entries,
		// and an entry wider or narrower than what is spliced in would move
		// the stamp for words the page never showed, or hold it still while
		// they changed.
		col.page.transcluded = append(col.page.transcluded, transcludedExcerpt{
			path:      res.RelPath,
			unmatched: unmatched,
			repeated:  repeated,
			slice:     body,
		})
		inner := r.render(body, embedsDenied, col.page)
		col.diags = append(col.diags, inner.Diagnostics...)
		heldBack := false
		for _, d := range inner.Diagnostics {
			if d.Kind == DiagEmbedNotExpanded {
				heldBack = true
				break
			}
		}
		// An image inside a transcluded body was written relative to the
		// note it came from, which is rarely the note being read, so it is
		// resolved here — where that note's own path is still known —
		// rather than later against the host's directory.
		return `<div class="` + embedClass(unmatched) + `">` + embedSourceLine(res.RelPath, col.page.lang) +
			widenedNotice(unmatched, col.page.lang) +
			repeatedNotice(link.Heading, repeated, col.page.lang) + notExpandedNotice(heldBack, col.page.lang) +
			resolveAssetHrefs(inner.HTML, res.RelPath) + `</div>`
	default:
		panic("render: unknown graph.Kind: " + res.Kind.String())
	}
}

// embedScope narrows a transcluded body to the section or block the embed's
// fragment named, which is what the fragment means: an author who wrote one
// scoped the excerpt on purpose. A fragment that matches nothing in the body
// falls back to the whole note and reports it — unlike a link's fragment, an
// embed's changes what is displayed, so widening the scope without saying so
// would present content the author left out as their own choice. Where the
// excerpt's exact edges are unruled, the narrower reading is taken and the
// scan states it: see headingSlice and blockSlice. A block address takes
// precedence over a heading when both parsed, mirroring how the plain-link
// fragment rule already resolves that conflict.
//
// This is where a transcluded body's Obsidian %% comments come off, and the
// only place they do: a heading or marker hidden inside a comment cannot
// anchor a visible excerpt, and every branch below — the one that takes the
// whole note included — hands back source the scan has already been over, so
// the render that follows has nothing left to remove. Reading the same text
// twice is what has to be avoided: the second pass would be free to reopen a
// marker the first had ruled to be literal text, and would hide words the
// first one kept.
//
// A marker left open in the transcluded note is reported here too, onto the
// page doing the citing. An excerpt that stops mid-sentence looks exactly like
// one that ended, and the reader is looking at this page rather than at the
// note the words came from.
func embedScope(link graph.Wikilink, resPath, body string, col *collector) (scoped, unmatched string, repeated int) {
	stripped, unclosed := stripObsidianComments(body)
	if unclosed != 0 {
		unclosedDiagnostic := unclosedCommentDiagnostic(unclosed)
		col.report(&unclosedDiagnostic)
	}
	switch {
	case link.Block != "":
		if slice, ok := blockSlice(stripped, link.Block); ok {
			return slice, "", 0
		}
		col.report(&Diagnostic{
			Kind:    DiagEmbedFragmentMissing,
			Target:  link.Target,
			Block:   link.Block,
			Message: fmt.Sprintf("no block in %q matched %q; the whole note is shown", resPath, "^"+link.Block),
		})
		return stripped, "#^" + link.Block, 0
	case link.Heading != "":
		if slice, matches := headingSlice(stripped, link.Heading); matches > 0 {
			if matches > 1 {
				col.report(&Diagnostic{
					Kind:    DiagEmbedFragmentRepeated,
					Target:  link.Target,
					Section: link.Heading,
					Message: fmt.Sprintf("%d headings in %q matched %q; the first is shown", matches, resPath, link.Heading),
				})
			}
			return slice, "", matches
		}
		col.report(&Diagnostic{
			Kind:    DiagEmbedFragmentMissing,
			Target:  link.Target,
			Section: link.Heading,
			Message: fmt.Sprintf("no heading in %q matched %q; the whole note is shown", resPath, link.Heading),
		})
		return stripped, "#" + link.Heading, 0
	}
	return stripped, "", 0
}

// atxHeadingLine matches an ATX heading the way goldmark will read it: up to
// three spaces of indent, one to six '#' characters, then whitespace before
// the text. A '#' run glued to text is not a heading in CommonMark and is
// not one here.
var atxHeadingLine = regexp.MustCompile(`^ {0,3}(#{1,6})[ \t]+(.*)$`)

// The HTML block start conditions of the CommonMark spec, minus the one for a
// bare complete tag alone on its own line. A block opened by any of these
// hands its lines to the reader as written, so a '#' line inside one is text
// rather than a section boundary — the line is quoted markup in a note about
// markup, or a comment, as often as it is anything else.
//
// The omitted condition is the one that cannot interrupt a paragraph, and
// telling those two readings apart needs paragraph state this scan does not
// keep. Leaving it out costs a '#' line inside such a block, and buys never
// mistaking an ordinary sentence that carries a tag for the start of one.
var (
	htmlBlockRawText = regexp.MustCompile(`(?i)^ {0,3}<(script|pre|style|textarea)([ \t>]|$)`)
	htmlBlockRawEnd  = regexp.MustCompile(`(?i)</(script|pre|style|textarea)>`)
	htmlBlockComment = regexp.MustCompile(`^ {0,3}<!--`)
	htmlBlockInstr   = regexp.MustCompile(`^ {0,3}<\?`)
	htmlBlockDecl    = regexp.MustCompile(`^ {0,3}<![A-Za-z]`)
	htmlBlockCDATA   = regexp.MustCompile(`^ {0,3}<!\[CDATA\[`)
	htmlBlockElement = regexp.MustCompile(`(?i)^ {0,3}</?(address|article|aside|base|basefont|blockquote|body|caption|center|col|colgroup|dd|details|dialog|dir|div|dl|dt|fieldset|figcaption|figure|footer|form|frame|frameset|h1|h2|h3|h4|h5|h6|head|header|hr|html|iframe|legend|li|link|main|menu|menuitem|nav|noframes|ol|optgroup|option|p|param|search|section|summary|table|tbody|td|tfoot|th|thead|title|tr|track|ul)([ \t]|/?>|$)`)
)

// htmlBlockOpen reports whether line opens an authored HTML block, and returns
// the test for the line that closes it. The raw-text, comment, instruction,
// declaration, and CDATA blocks close on their own end marker — which may sit
// on the opening line itself — while an element block runs to the next blank
// line.
func htmlBlockOpen(line string) (closes func(string) bool, ok bool) {
	switch {
	case htmlBlockRawText.MatchString(line):
		return htmlBlockRawEnd.MatchString, true
	case htmlBlockComment.MatchString(line):
		return closesOn("-->"), true
	case htmlBlockInstr.MatchString(line):
		return closesOn("?>"), true
	case htmlBlockCDATA.MatchString(line):
		return closesOn("]]>"), true
	case htmlBlockDecl.MatchString(line):
		return closesOn(">"), true
	case htmlBlockElement.MatchString(line):
		return blankLine, true
	}
	return nil, false
}

func closesOn(marker string) func(string) bool {
	return func(line string) bool { return strings.Contains(line, marker) }
}

func blankLine(line string) bool { return strings.TrimSpace(line) == "" }

// blockScan carries the running state a line-by-line section scan needs to
// tell a real heading from a heading-looking line: fenced code and authored
// HTML blocks both hand their contents to the reader as written, so a line
// inside either is content and never a boundary. The zero value starts a scan.
type blockScan struct {
	inFence    bool
	fenceByte  byte
	htmlCloses func(string) bool
}

// skips advances the scan by one line and reports whether that line is inside
// a fenced code block or an authored HTML block — including the line that
// opens or closes one, which belongs to the block rather than to the prose
// around it.
func (s *blockScan) skips(line string) bool {
	switch {
	case s.inFence:
		if fenceCloses(line, s.fenceByte) {
			s.inFence = false
		}
		return true
	case s.htmlCloses != nil:
		if s.htmlCloses(line) {
			s.htmlCloses = nil
		}
		return true
	}
	if marker, _, ok := fenceOpen(line); ok {
		s.inFence, s.fenceByte = true, marker
		return true
	}
	if closes, ok := htmlBlockOpen(line); ok {
		if !closes(line) {
			s.htmlCloses = closes
		}
		return true
	}
	return false
}

// setextUnderline matches the line that underlines a heading written without
// '#' marks, and reports the level it makes: '=' is a level-1 heading, '-' a
// level-2 one.
var setextUnderline = regexp.MustCompile(`^ {0,3}(=+|-+)[ \t]*$`)

// The line shapes that are not running prose, and therefore cannot be the text
// an underline turns into a heading: a quote, a list item, a break rule, and
// an indented code line. Anything else that is not blank continues a
// paragraph.
var (
	quotedLine       = regexp.MustCompile(`^ {0,3}>`)
	listItemLine     = regexp.MustCompile(`^ {0,3}(?:[-*+]|\d{1,9}[.)])(?:[ \t]|$)`)
	breakRuleLine    = regexp.MustCompile(`^ {0,3}((\*[ \t]*){3,}|(_[ \t]*){3,}|(-[ \t]*){3,})$`)
	indentedCodeLine = regexp.MustCompile(`^ {4,}\S`)
)

// sectionHeading is one heading a scan found: the line its section opens on,
// its level, and the source text its anchor is folded from. An underlined
// heading opens on the first line of the text, not on the underline.
type sectionHeading struct {
	line  int
	level int
	text  string
}

// scanHeadings reports every heading in lines, in document order, reading them
// the way the page that displays them does: '#'-marked and underlined headings
// both count, and a heading-looking line inside fenced code or an authored
// HTML block counts as neither.
//
// An underline only makes a heading of running prose, so the shapes that open
// a different construct — a quote, a list item, a break rule, an indented code
// line — end the run of lines an underline could claim. Where the reading is
// genuinely ambiguous the scan keeps the plainer one, which costs an
// underlined heading and never invents one.
func scanHeadings(lines []string) []sectionHeading {
	var out []sectionHeading
	var scan blockScan
	paragraph := -1
	for i, line := range lines {
		if scan.skips(line) {
			paragraph = -1
			continue
		}
		if m := atxHeadingLine.FindStringSubmatch(line); m != nil {
			out = append(out, sectionHeading{line: i, level: len(m[1]), text: m[2]})
			paragraph = -1
			continue
		}
		switch {
		case paragraph >= 0 && setextUnderline.MatchString(line):
			level := 2
			if strings.HasPrefix(strings.TrimSpace(line), "=") {
				level = 1
			}
			out = append(out, sectionHeading{line: paragraph, level: level, text: strings.Join(lines[paragraph:i], "\n")})
			paragraph = -1
		case blankLine(line), quotedLine.MatchString(line), listItemLine.MatchString(line),
			breakRuleLine.MatchString(line), setextUnderline.MatchString(line),
			paragraph < 0 && indentedCodeLine.MatchString(line):
			paragraph = -1
		case paragraph < 0:
			paragraph = i
		}
	}
	return out
}

// headingSlice returns the section of body that heading names: the first
// heading whose text folds to the same slug as the name, through to the line
// before the next heading of the same or a higher level. Deeper headings stay
// inside the slice, because a section owns its subsections. When the name
// appears twice the first occurrence wins, matching Obsidian's reading view.
//
// The name is folded through slugify, the same function that stamps the
// destination page's anchors, over heading text reduced the same way that pass
// reduces it: a link contributes the words it displays, a ruby reading drops
// out, tags and character references resolve. So the spellings the destination
// page lists in its own table of contents are the spellings an embed of that
// section accepts.
//
// One spelling does not survive the trip, because it is read from source
// rather than from the rendered page: a heading carrying a markdown link keeps
// the address the rendered heading drops. It reports, it does not truncate.
func headingSlice(body, heading string) (slice string, matches int) {
	want := slugify(heading)
	lines := strings.Split(body, "\n")
	headings := scanHeadings(lines)
	for i, h := range headings {
		if slugify(headingSourceText(h.text)) != want {
			continue
		}
		matches++
		if matches > 1 {
			// The later ones are counted, not cut: the excerpt is the first,
			// and what the rest are for is to say how many there were.
			continue
		}
		slice = strings.Join(lines[h.line:], "\n")
		for _, next := range headings[i+1:] {
			if next.level <= h.level {
				slice = strings.Join(lines[h.line:next.line], "\n")
				break
			}
		}
	}
	return slice, matches
}

// headingSourceText reduces a heading's markdown source to the text the page
// stamps its anchor from. A wikilink contributes what it displays, which is
// the target itself unless the author wrote a display alias — the rendered
// heading shows exactly those words, and a reader copying the section's name
// off the page copies them too.
func headingSourceText(raw string) string {
	displayed := wikilinkToken.ReplaceAllStringFunc(raw, func(token string) string {
		inner := strings.TrimPrefix(token, "!")
		_, display, _ := graph.SplitWikilink(inner[2 : len(inner)-2])
		return display
	})
	return headingInnerText(displayed)
}

// blockSlice returns the block carrying the "^name" marker: the run of
// non-blank lines around the first line outside fenced code that ends with the
// marker, stopping at a list item's own line so a marker written on one item
// addresses that item rather than the list it sits in. Where the marked line
// is a continuation, the run reaches back to the line its block opens on — a
// marker written under a multi-line block reaches that block the same way,
// since no blank line separates them.
//
// The address is matched through the fold both fragment kinds share, so
// "^Quote1" and "#^quote1" are one name. Only case and Unicode form fold: the
// rest of an address is an identifier the author chose, and "^quote-1" and
// "^quote1" are two of them.
//
// Nothing external rules on how wide a block reference reaches, so the narrow
// reading is taken: the reader asked for the line the author marked, and
// widening it here would be this renderer choosing an excerpt the author did
// not.
func blockSlice(body, block string) (string, bool) {
	lines := strings.Split(body, "\n")
	at := blockMarkerLine(lines, block)
	if at < 0 {
		return "", false
	}
	start := at
	for start > 0 && !listItemLine.MatchString(lines[start]) && strings.TrimSpace(lines[start-1]) != "" {
		start--
	}
	end := at + 1
	for end < len(lines) && strings.TrimSpace(lines[end]) != "" && !listItemLine.MatchString(lines[end]) {
		end++
	}
	return strings.Join(lines[start:end], "\n"), true
}

// blockMarkerLine reports which line carries the marker naming block, or -1
// when the note has no such marker. A marker written inside a fenced block is
// code rather than an address, so the scan tracks fences as it walks.
func blockMarkerLine(lines []string, block string) int {
	want := foldFragment("^" + block)
	inFence, fenceByte := false, byte(0)
	for i, line := range lines {
		// A fence is looked for with any quote marker taken off it, because a
		// fence written inside a callout opens one: that body is read on its
		// own with the markers stripped, and a line of code in it is code.
		unquoted := quotePrefix.ReplaceAllString(line, "")
		if inFence {
			if fenceCloses(unquoted, fenceByte) {
				inFence = false
			}
			continue
		}
		if open, _, ok := fenceOpen(unquoted); ok {
			inFence, fenceByte = true, open
			continue
		}
		if unanchorableLine(line) {
			continue
		}
		trimmed := foldFragment(strings.TrimRight(line, " \t"))
		if trimmed == want || strings.HasSuffix(trimmed, " "+want) || strings.HasSuffix(trimmed, "\t"+want) {
			return i
		}
	}
	return -1
}

// embedClass marks an excerpt whose fragment matched nothing, so the widening
// is visible in the article rather than only in the diagnostics face.
func embedClass(unmatched string) string {
	if unmatched == "" {
		return "embed"
	}
	return "embed embed--widened"
}

// embedSourceLine opens an excerpt with the name of the note its words came
// from, as a link there. It stands with the widening and repetition notices
// and reads like them, because it answers the same order of question — not
// what the words say, but what this block is: an excerpt is another note's
// words, and the page around it otherwise never says whose. The name is the
// file's own, the one a citation resolves by, so what the reader clicks is
// what an author would type.
func embedSourceLine(relPath string, lang wording.Lang) string {
	name := strings.TrimSuffix(path.Base(relPath), ".md")
	return `<p class="embed__source">` + html.EscapeString(wording.EmbedSourceFrom.In(lang)) +
		`<a href="` + notesHref(relPath) + `">` + html.EscapeString(name) + `</a></p>`
}

// widenedNotice states, inside the excerpt itself, that the author's fragment
// found nothing and the whole source note stands in its place. The reading
// page's diagnostics face already reports it, but that face lives in the right
// rail: narrow viewports collapse it and the widest ones put it beside the
// article rather than in it. The words the author scoped out are on the page
// either way, so the account of why has to be on the page too.
func widenedNotice(unmatched string, lang wording.Lang) string {
	if unmatched == "" {
		return ""
	}
	return `<p class="embed__widened">` + html.EscapeString(wording.EmbedWidenedBefore.In(lang)) + `<code>` + html.EscapeString(unmatched) + `</code>` + html.EscapeString(wording.EmbedWidenedAfter.In(lang)) + `</p>`
}

// repeatedNotice states, above the excerpt, that the fragment named a section
// the source note carries more than once and that the first of them is what
// follows. Without it the reader has no way to tell an excerpt that was chosen
// from an excerpt that was the only candidate, and those are different things
// to be looking at.
//
// It sits where the widening notice sits and reads the same way, because both
// answer the one question an excerpt cannot answer for itself: why these words
// and not others. Nothing is said for a single match, which is the ordinary
// case and would drown the notice that matters.
func repeatedNotice(heading string, matches int, lang wording.Lang) string {
	if matches < 2 {
		return ""
	}
	return `<p class="embed__note">` +
		html.EscapeString(fmt.Sprintf(wording.EmbedRepeatedHeadingFmt.In(lang), matches, heading)) +
		`</p>`
}

// notExpandedNotice states that a citation inside this excerpt was written as
// an embed and is shown as a link. It is said once for the excerpt rather than
// beside each link, because the reason is a property of the excerpt — its
// contents come from another file, and going a level deeper would let one
// citation pull in a chain of them.
func notExpandedNotice(heldBack bool, lang wording.Lang) string {
	if !heldBack {
		return ""
	}
	return `<p class="embed__note">` + html.EscapeString(wording.EmbedNotExpanded.In(lang)) + `</p>`
}

// headingAnchorMayExist reports whether any line of body could stamp the id a
// section address names. It is deliberately generous where headingSlice is
// exact: it strips blockquote markers and list markers before looking, and it
// does not track fenced code, because its only job is to stop a miss being
// reported about a heading the rendered page really carries. A false yes costs
// one unreported broken fragment; a false no withdraws nothing but tells a
// reader a section they are looking at is not there.
// Both ways of writing a heading count here, because the page renders both. An
// underline makes a heading out of the paragraph above it, so this carries the
// lines of the paragraph it is inside and offers them up when an underline
// arrives. Reading only the '#' form was the same mistake as reading only
// unquoted lines, one step further in: the page stamped an id for a heading
// underlined inside a quote, and a link to it was reported broken while the
// reader was looking straight at the section it named.
func headingAnchorMayExist(body, heading string) bool {
	want := slugify(heading)
	var paragraph []string
	for line := range strings.SplitSeq(body, "\n") {
		candidate := withoutQuoteAndListMarkers(line)
		if m := atxHeadingLine.FindStringSubmatch(candidate); m != nil {
			if slugify(headingSourceText(m[2])) == want {
				return true
			}
			paragraph = nil
			continue
		}
		// An underline is offered the paragraph before anything else can claim
		// the same characters: a row of dashes closing a paragraph underlines
		// it rather than drawing a rule, which is the order the page reads
		// them in too.
		if len(paragraph) > 0 && setextUnderline.MatchString(candidate) {
			if slugify(headingSourceText(strings.Join(paragraph, "\n"))) == want {
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

// withoutQuoteAndListMarkers reduces a line to the text a heading could have
// been written as, by removing however many quote markers are nested around it
// and one list marker. The page keeps a heading inside both, so a scan that
// stopped at either would miss ids that page really stamps.
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
// inside one of the notes it embeds. The scans that validate a section address
// read one note's own source, so a heading written in an embedded file is one
// they cannot see at any level of generosity — while the page stamps its id all
// the same, because the embed really is expanded into it.
//
// Where a body embedded anything, such a name used to be left unreported
// wholesale. That bought silence about the headings an embed genuinely brings
// and paid for it with silence about every name that was nowhere at all: one
// citation of a section that does not exist anywhere goes unmentioned for as
// long as the destination happens to embed something. Reading the embedded
// file instead separates the two, and costs one resolution per embed.
//
// The walk goes exactly one level, which is not a limit chosen here but the one
// the render already has: an embed inside an embedded body renders as plain
// link text rather than expanding, so its headings never reach the page and
// must not count. The same rule decides which occurrences are embeds at all —
// quoted syntax and escaped brackets are shown, not followed — so this reads
// the source the way the pass that expands it will.
//
// Only a note can bring headings. A picture, a PDF, an embed that resolves to
// nothing, and a body this generation never captured all bring none, and a
// fragment naming one of them is as absent as if the embed were not there.
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

// headingBroughtBy reports whether the heading is inside the part of an
// embedded body that the embed actually shows. An embed carrying a fragment
// narrows the excerpt on purpose, so a heading outside that slice never
// reaches the page and a link naming it is naming something absent; an embed
// whose fragment matches nothing widens back to the whole note, and then
// everything in it does arrive. Both readings are the ones embedScope applies
// when it cuts the excerpt for display.
func headingBroughtBy(link graph.Wikilink, embedded, heading string) bool {
	scoped := embedded
	switch {
	case link.Block != "":
		if slice, ok := blockSlice(embedded, link.Block); ok {
			scoped = slice
		}
	case link.Heading != "":
		if slice, matches := headingSlice(embedded, link.Heading); matches > 0 {
			scoped = slice
		}
	}
	if _, found := headingSlice(scoped, heading); found > 0 {
		return true
	}
	return headingAnchorMayExist(scoped, heading)
}
