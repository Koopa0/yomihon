package render

import (
	"fmt"
	"html"
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/koopa0/yomihon/internal/graph"
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
func unwrittenTarget(target, display string) string {
	reason := html.EscapeString(fmt.Sprintf("還沒有「%s」這篇筆記", target))
	return `<span class="wikilink-broken" title="` + reason + `">` +
		html.EscapeString(display) + `<span class="y-offscreen">（` + reason + `）</span></span>`
}

// blockPlaceholder is the reserved marker substituted into markdown source for
// first-party markup that stands on its own line — a transclusion, a callout.
// An HTML comment is one indivisible raw node, so it reaches the reader as the
// standalone block its caller surrounded with blank lines, without asking the
// authored-HTML allowlist to admit a general-purpose element. renderBody
// neutralizes authored copies of this prefix before preprocess can create real
// placeholders.
func blockPlaceholder(i int) string {
	return fmt.Sprintf(`<!--yomihon-block:%d-->`, i)
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
	return fmt.Sprintf("\ue000%d\ue001", i)
}

// inlinePlaceholderRunes are stripped from authored text before any real marker
// exists, for the same reason the comment prefix is neutralized: a vault note
// containing them could otherwise select or relocate renderer-owned markup.
// They are unassigned to any character, so nothing a reader meant to see is
// lost with them.
const inlinePlaceholderRunes = "\ue000\ue001"

func substituteBlocks(htmlOut string, blocks, inline []string) string {
	for i, b := range blocks {
		htmlOut = strings.Replace(htmlOut, blockPlaceholder(i), b, 1)
	}
	for i, b := range inline {
		htmlOut = strings.Replace(htmlOut, inlinePlaceholder(i), b, 1)
	}
	return htmlOut
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
			st.kept = append(st.kept, r.convertWikilinks(st.lines[st.i], allowEmbed, col, &st.inline))
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
		col.report(Diagnostic{
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
// rendered SVG client-side — mirroring koopa0.dev's markdown.service.ts
// processMermaidDiagrams/renderMermaid DOM shape exactly (see that file's
// doc comments); the loading mechanism differs only because yomihon has no
// bundler.
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
		col.report(Diagnostic{
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
		// A backslash escape is the author writing the syntax to be shown,
		// not followed (CommonMark backslash escapes). The match is left
		// untouched for goldmark, whose own escape handling prints the
		// literal text — converting it anyway both showed a stray backslash
		// and reported a broken link the author never made.
		if escapedAt(text, start) {
			return m
		}
		embed := strings.HasPrefix(m, "!")
		raw := m
		if embed {
			raw = m[1:]
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
			embedHTML := r.renderEmbed(link, allowEmbed, col)
			*inline = append(*inline, embedHTML)
			return inlinePlaceholder(len(*inline) - 1)
		}
		linkHTML := r.renderWikilink(link, col)
		*inline = append(*inline, linkHTML)
		return inlinePlaceholder(len(*inline) - 1)
	})
}

// backtickRun reports how many backticks start at i, zero when none do.
func backtickRun(text string, i int) int {
	n := 0
	for i+n < len(text) && text[i+n] == '`' {
		n++
	}
	return n
}

// closingRun reports the end offset of the next backtick run of exactly width
// at or after i, or -1 when the text has none. A run of a different width is
// ordinary content inside the span and is stepped over.
func closingRun(text string, i, width int) int {
	for j := i; j < len(text); {
		n := backtickRun(text, j)
		switch n {
		case 0:
			j++
		case width:
			return j + n
		default:
			j += n
		}
	}
	return -1
}

// codeSpanRanges reports the byte ranges of CommonMark code spans in text: a
// run of backticks opens one and the next run of the same length closes it. A
// run with no such closer is not a span, and neither is anything after it —
// which is how the same text renders, so the two agree.
func codeSpanRanges(text string) [][2]int {
	var spans [][2]int
	for i := 0; i < len(text); {
		width := backtickRun(text, i)
		if width == 0 {
			i++
			continue
		}
		end := closingRun(text, i+width, width)
		if end < 0 {
			return spans
		}
		spans = append(spans, [2]int{i, end})
		i = end
	}
	return spans
}

// escapedAt reports whether the character at start is backslash-escaped: an
// odd-length run of '\' immediately before it. An even-length run is pairs
// of literal backslashes and escapes nothing — the counting CommonMark's own
// backslash-escape rule implies.
func escapedAt(text string, start int) bool {
	n := 0
	for i := start - 1; i >= 0 && text[i] == '\\'; i-- {
		n++
	}
	return n%2 == 1
}

// replaceOutside applies fn to every match of re that lies wholly outside the
// given ranges, leaving everything else byte-identical. fn receives the
// match's starting byte offset in text alongside the matched bytes, so a
// caller can consult the characters just before the match — returning the
// match unchanged leaves that occurrence byte-identical too.
func replaceOutside(text string, skip [][2]int, re *regexp.Regexp, fn func(start int, m string) string) string {
	inSkip := func(start, end int) bool {
		for _, s := range skip {
			if start >= s[0] && end <= s[1] {
				return true
			}
		}
		return false
	}
	var out strings.Builder
	last := 0
	for _, loc := range re.FindAllStringIndex(text, -1) {
		if inSkip(loc[0], loc[1]) {
			continue
		}
		out.WriteString(text[last:loc[0]])
		out.WriteString(fn(loc[0], text[loc[0]:loc[1]]))
		last = loc[1]
	}
	out.WriteString(text[last:])
	return out.String()
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
// A link written at a section means that section: without the fragment the
// reader arrives at the top of a note that may run for pages, with nothing on
// screen to say which part they were sent to.
//
// The fragment is built by slugify — the same function that stamps the
// destination's heading ids — because a second rule for turning heading text
// into an anchor would agree by maintenance accident and disagree by default.
// It emits Unicode letters, digits, and hyphens only, so it needs no escaping
// in either the URL or the attribute.
//
// A block address is left off: "^name" identifies a paragraph rather than a
// heading, and this renderer stamps no anchor for one, so writing the fragment
// would send the reader to a place that does not exist on the page.
//
// So is anything that is not a note. A PDF, a canvas, a picture is handed to
// the reader whole and nothing inside it is addressable from here; a fragment
// on one names a place that cannot exist, and because a viewer simply ignores
// what it does not understand, it would look like it had worked.
func sectionHref(relPath string, link graph.Wikilink) string {
	href := notesHref(relPath)
	if link.Heading == "" || link.Block != "" || !strings.HasSuffix(relPath, ".md") {
		return href
	}
	return href + "#" + slugify(link.Heading)
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
	case graph.Unique:
		//nolint:gocritic // sprintfQuotedString false positive: the quotes are HTML attribute syntax, not Go string quoting; sectionHref already percent-escapes the path
		return fmt.Sprintf(`<a href="%s" class="wikilink">%s</a>`, sectionHref(res.Path, link), html.EscapeString(link.Display))
	case graph.Ambiguous:
		col.report(Diagnostic{
			Kind: DiagWikilinkAmbiguous, Target: link.Target,
			Message: fmt.Sprintf("wikilink %q is ambiguous: %s", link.Target, strings.Join(res.Candidates, ", ")),
		})
		//nolint:gocritic // sprintfQuotedString false positive: the quotes are HTML attribute syntax, not Go string quoting; the value is already html.EscapeString'd
		return fmt.Sprintf(`<span class="wikilink-ambiguous" title="%s">%s</span>`,
			html.EscapeString(strings.Join(res.Candidates, ", ")), html.EscapeString(link.Display))
	case graph.Unresolved:
		col.report(Diagnostic{
			Kind: DiagWikilinkBroken, Target: link.Target,
			Message: fmt.Sprintf("wikilink %q does not resolve to any note or file", link.Target),
		})
		return unwrittenTarget(link.Target, link.Display)
	default:
		panic(fmt.Sprintf("render: unknown graph.Kind %d", res.Kind))
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
func (r *Pipeline) renderEmbed(link graph.Wikilink, allowEmbed embedPolicy, col *collector) string {
	if allowEmbed == embedsDenied {
		return r.renderWikilink(link, col)
	}

	target := link.Target
	res := r.idx.Resolve(target)
	switch res.Kind {
	case graph.Unresolved:
		col.report(Diagnostic{
			Kind: DiagWikilinkBroken, Target: target,
			Message: fmt.Sprintf("embed target %q does not resolve", target),
		})
		return unwrittenTarget(target, "![["+target+"]]")
	case graph.Ambiguous:
		col.report(Diagnostic{
			Kind: DiagWikilinkAmbiguous, Target: target,
			Message: fmt.Sprintf("embed target %q is ambiguous: %s", target, strings.Join(res.Candidates, ", ")),
		})
		//nolint:gocritic // sprintfQuotedString false positive: the quotes are HTML attribute syntax, not Go string quoting; the value is already html.EscapeString'd
		return fmt.Sprintf(`<span class="wikilink-ambiguous" title="%s">![[%s]]</span>`,
			html.EscapeString(strings.Join(res.Candidates, ", ")), html.EscapeString(target))
	case graph.Unique:
		if IsPicture(res.Path) {
			//nolint:gocritic // sprintfQuotedString false positive: the quotes are HTML attribute syntax, not Go string quoting; rawHref percent-escapes the path and the name is html.EscapeString'd
			return fmt.Sprintf(`<img src="%s" alt="%s">`,
				rawHref(res.Path), html.EscapeString(path.Base(res.Path)))
		}
		if !strings.HasSuffix(res.Path, ".md") {
			//nolint:gocritic // sprintfQuotedString false positive: the quotes are HTML attribute syntax, not Go string quoting; notesHref percent-escapes the path and the name is html.EscapeString'd
			return fmt.Sprintf(`<div class="embed-media">[Embedded media: <a href="%s">%s</a> — inline display not yet supported]</div>`,
				notesHref(res.Path), html.EscapeString(path.Base(res.Path)))
		}
		body, ok := r.transclusions.Transclusion(res.Path)
		if !ok {
			col.report(Diagnostic{
				Kind: DiagWikilinkBroken, Target: target,
				Message: fmt.Sprintf("embed target %q is unavailable in the captured generation", res.Path),
			})
			return fmt.Sprintf(`<span class="wikilink-broken" title="這篇筆記存在，但這次讀取時拿不到它的內容">![[%s]]</span>`,
				html.EscapeString(target))
		}
		body = embedScope(link, res.Path, body, col)
		inner := r.render(body, embedsDenied, col.page)
		col.diags = append(col.diags, inner.Diagnostics...)
		// An image inside a transcluded body was written relative to the
		// note it came from, which is rarely the note being read, so it is
		// resolved here — where that note's own path is still known —
		// rather than later against the host's directory.
		return `<div class="embed">` + resolveAssetHrefs(inner.HTML, res.Path) + `</div>`
	default:
		panic(fmt.Sprintf("render: unknown graph.Kind %d", res.Kind))
	}
}

// embedScope narrows a transcluded body to the section or block the embed's
// fragment named, which is what the fragment means: Obsidian shows only that
// slice, and an author who wrote one scoped the excerpt on purpose. A
// fragment that names nothing in the body falls back to the whole note and
// reports it — unlike a link's fragment, an embed's changes what is
// displayed, so widening the scope without saying so would present content
// the author left out as their own choice. A block address takes precedence
// over a heading when both parsed, mirroring how the plain-link fragment
// rule already resolves that conflict.
//
// The scan runs over the comment-stripped source so a heading or marker
// hidden inside an Obsidian %% comment cannot anchor a visible excerpt; the
// later render pass strips comments again, which leaves an already-stripped
// body unchanged.
func embedScope(link graph.Wikilink, resPath, body string, col *collector) string {
	switch {
	case link.Block != "":
		stripped := stripObsidianComments(body)
		if slice, ok := blockSlice(stripped, link.Block); ok {
			return slice
		}
		col.report(Diagnostic{
			Kind:    DiagEmbedFragmentMissing,
			Target:  link.Target + "#^" + link.Block,
			Message: fmt.Sprintf("embed block %q not found in %q; the whole note is shown", "^"+link.Block, resPath),
		})
	case link.Heading != "":
		stripped := stripObsidianComments(body)
		if slice, ok := headingSlice(stripped, link.Heading); ok {
			return slice
		}
		col.report(Diagnostic{
			Kind:    DiagEmbedFragmentMissing,
			Target:  link.Target + "#" + link.Heading,
			Message: fmt.Sprintf("embed heading %q not found in %q; the whole note is shown", link.Heading, resPath),
		})
	}
	return body
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

// headingSlice returns the section of body that heading names: the first
// heading line outside fenced code and authored HTML whose text folds to the
// same slug as the name — the same fold that stamps the destination page's anchors, so an
// embed and a link to one section agree on what matches — through to the
// line before the next heading of the same or a higher level. Deeper
// headings stay inside the slice, because a section owns its subsections.
// When the name appears twice the first occurrence wins, matching Obsidian's
// reading view.
func headingSlice(body, heading string) (string, bool) {
	want := slugify(heading)
	lines := strings.Split(body, "\n")
	start, level := -1, 0
	var scan blockScan
	for i, line := range lines {
		if scan.skips(line) {
			continue
		}
		m := atxHeadingLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if start < 0 {
			if slugify(m[2]) == want {
				start, level = i, len(m[1])
			}
			continue
		}
		if len(m[1]) <= level {
			return strings.Join(lines[start:i], "\n"), true
		}
	}
	if start < 0 {
		return "", false
	}
	return strings.Join(lines[start:], "\n"), true
}

// blockSlice returns the paragraph carrying the "^name" block marker: the
// contiguous run of non-blank lines around the first line outside fenced
// code that ends with the marker, which is where Obsidian attaches a block
// address. A marker written on its own line directly under a multi-line
// block reaches that block through the same expansion, since no blank line
// separates them.
func blockSlice(body, block string) (string, bool) {
	marker := "^" + block
	lines := strings.Split(body, "\n")
	at := -1
	inFence, fenceByte := false, byte(0)
	for i, line := range lines {
		if inFence {
			if fenceCloses(line, fenceByte) {
				inFence = false
			}
			continue
		}
		if open, _, ok := fenceOpen(line); ok {
			inFence, fenceByte = true, open
			continue
		}
		trimmed := strings.TrimRight(line, " \t")
		if trimmed == marker || strings.HasSuffix(trimmed, " "+marker) || strings.HasSuffix(trimmed, "\t"+marker) {
			at = i
			break
		}
	}
	if at < 0 {
		return "", false
	}
	start := at
	for start > 0 && strings.TrimSpace(lines[start-1]) != "" {
		start--
	}
	end := at + 1
	for end < len(lines) && strings.TrimSpace(lines[end]) != "" {
		end++
	}
	return strings.Join(lines[start:end], "\n"), true
}
