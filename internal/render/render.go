// Package render owns projections of Obsidian-dialect markdown. Pipeline turns
// it into HTML for reading; PlainText and PlainSections expose the same parsed
// dialect to lexical and semantic retrieval without duplicating a parser.
//
// Fault-tolerant by contract over note content: it renders what it can and
// reports what it can't via Diagnostics — it never fixes a note, never fails
// the whole render over anything the note's own bytes could cause, and never
// goes quiet without saying why. A note can still render to nothing, because
// an unclosed %% comment hides everything after it and Obsidian hides it
// too; what the reader then receives is an account of which marker did it,
// rather than an empty page and no explanation.
//
// That promise does not reach an internal enum invariant: a graph.Kind
// outside Unresolved, Unique, and Ambiguous is not a value any note's
// content can produce, so reaching one is this package's own bug rather
// than a fault in what it read, and it panics there instead of rendering
// past it silently.
// Authored HTML is inert display
// input: the Japanese lesson dialect's ruby/rt/rp/br subset survives, while
// executable or automatically loading markup is shown as text.
//
// Wikilinks, embeds, and callouts are not CommonMark syntax, so they are
// handled as string/line-based passes over the markdown source before
// goldmark ever parses it — the only approach that can see the raw
// "[[" / "> [!" text goldmark's own parser has no concept of.
// ==highlight== is the exception: it is implemented as a
// real goldmark inline extension (see highlight.go) because goldmark's
// own trigger-based inline dispatch already skips code spans for free,
// which a blind regex pass would not.
//
// Wikilink/embed resolution semantics are internal/graph's job, not
// this package's: graph decides what a name resolves to (matching
// Obsidian's observed resolution behavior); render decides what
// to draw for each of graph's three outcomes.
package render

import (
	"bytes"
	"fmt"
	"html"
	"slices"
	"strconv"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/wording"
)

// Transclusions is the captured note-body set used by embed expansion. The
// concrete reading snapshot implements this narrow consumer-owned capability,
// so link resolution and transclusion bodies can come from one generation.
type Transclusions interface {
	Transclusion(path string) (body string, ok bool)
}

// Titles answers which notes declare a given title. A title is not a name a
// link resolves by, so this never affects resolution — it is how a page that
// has already failed to resolve a name can tell a reader the note is right
// there under a name links do not follow, instead of that it was never
// written.
//
// It returns display names rather than paths because that is all the sentence
// on the page needs, and a capability that hands over more than its consumer
// uses is a wider coupling than the consumer asked for.
type Titles interface {
	TitledBy(name string) []string
}

// DiagnosticKind classifies one rendering-time Diagnostic.
type DiagnosticKind string

const (
	// DiagWikilinkBroken means a [[wikilink]] or ![[embed]] target does
	// not resolve to any note or file.
	DiagWikilinkBroken DiagnosticKind = "wikilink-broken"
	// DiagWikilinkAmbiguous means a target resolves to more than one
	// file; the candidates are listed, never guessed at.
	DiagWikilinkAmbiguous DiagnosticKind = "wikilink-ambiguous"

	// DiagWikilinkTitleOnly means the target names some note's declared title.
	// A title is not a name a link resolves by — the vault's own reader fails
	// on one too, and this program reproduces that — so the link is broken
	// either way. It is a separate kind because the repair is not: nothing has
	// to be written, and an alias on the note the citation meant makes the
	// existing link work. Telling a reader there is no such note when the note
	// is right there, under a name they nearly used, is the one thing this
	// kind exists to stop.
	DiagWikilinkTitleOnly DiagnosticKind = "wikilink-title-only"

	// DiagTitleTruncatedAtHash means this note's own title is exactly what its
	// filename becomes when cut at the first whitespace followed by a "#" —
	// which is where YAML starts a comment in an unquoted value. It is an
	// observation rather than an accusation: a title deliberately written
	// short, in quotes, produces the same coincidence, and nothing in the
	// parsed frontmatter can tell the two apart.
	DiagTitleTruncatedAtHash DiagnosticKind = "title-truncated-at-hash"
	// DiagUnknownCallout means a "> [!type]" callout's type is not one
	// of the recognized callout types; it was rendered as a plain
	// blockquote instead of being dropped.
	DiagUnknownCallout DiagnosticKind = "unknown-callout"
	// DiagRiskyFence means a fenced code block's content looks like
	// wikilink/callout/table syntax the dialect passes would otherwise
	// convert; it was left untouched. At most one per render call (see
	// the preprocessing pass doc).
	DiagRiskyFence DiagnosticKind = "risky-fence"
	// DiagEmbedFragmentMissing means an embed named a section or block
	// inside its target ("![[note#heading]]", "![[note#^block]]") that the
	// captured body does not contain; the whole note was shown instead.
	// Unlike a link's fragment, an embed's fragment changes what is
	// displayed, so falling back without saying so would present the wrong
	// scope as the author's own choice.
	DiagEmbedFragmentMissing DiagnosticKind = "embed-fragment-missing"
	// DiagEmbedFragmentRepeated means an embed named a section its target
	// carries more than once ("![[note#heading]]" where two headings reduce to
	// the same name). The first is shown, which is what the page has always
	// done and what the address can mean at all; the count is reported because
	// an excerpt chosen from several looks exactly like the only one there
	// was, and the author is the one who can tell them apart.
	DiagEmbedFragmentRepeated DiagnosticKind = "embed-fragment-repeated"
	// DiagEmbedNotExpanded means an embed written inside a transcluded body
	// was rendered as an ordinary link. Transclusion stops one level down, so
	// what the reader sees is a citation where the author wrote an excerpt —
	// a difference nothing on the page would otherwise account for.
	DiagEmbedNotExpanded DiagnosticKind = "embed-not-expanded"
	// DiagLinkFragmentMissing means a plain link named a block inside its
	// target ("[[note#^block]]") that the captured body does not carry. The
	// link is left leading to the note itself: a block address answers to an
	// anchor the destination page stamps, and writing one the page has no
	// anchor for would tell the reader they are going to a block and land them
	// somewhere else.
	DiagLinkFragmentMissing DiagnosticKind = "link-fragment-missing"
	// DiagLinkSectionMissing means a plain link named a section inside its
	// target ("[[note#heading]]") that neither scan of the captured body could
	// find. Unlike a block address, the address is kept exactly as written: a
	// heading id is stamped by a pass over rendered HTML that sees headings
	// these scans do not, so a miss here is a name they failed to find rather
	// than one the page is certain to lack. The link therefore still works
	// wherever the scans were merely blind, and lands at the top of the note
	// only where the section genuinely is not there.
	//
	// It is a separate kind from a missing block for exactly that reason: one
	// withdraws the author's address and the other leaves it standing, so a
	// panel that named them alike would tell the reader the wrong thing about
	// one of them.
	DiagLinkSectionMissing DiagnosticKind = "link-section-missing"
	// DiagCommentUnclosed means a "%%" comment marker never met a second one,
	// so everything written after it is hidden from the page. Obsidian hides it
	// too, which is why the words are not simply restored here: the note reads
	// the same in both, and the reader is told where the silence begins instead
	// of being left to work out why a page they wrote is blank.
	DiagCommentUnclosed DiagnosticKind = "comment-unclosed"
	// DiagRenderFailed means goldmark's own renderer returned an error —
	// normally unreachable (see render's fallback), kept only so a
	// future extension that breaks that assumption still produces a
	// visible diagnostic instead of a panic or a blank page.
	DiagRenderFailed DiagnosticKind = "render-failed"
)

// Diagnostic is one rendering-time note about content the dialect passes
// couldn't cleanly handle. Display-only: yomihon reports, it
// never fixes or rejects. It lives in this package rather than
// internal/graph because a diagnostic is fundamentally a rendering-time
// decision — it is render, not graph, that decides an unresolved link,
// an unknown callout type, or a risky fence pattern is worth surfacing to
// the reader; graph only answers "does this name resolve".
type Diagnostic struct {
	Kind    DiagnosticKind
	Target  string // the offending wikilink target / callout type / etc.
	Message string // human-readable

	// Section and Block carry the two halves of an address a link wrote after
	// "#", so a panel listing the note's faults can say how the link was read.
	// Target stays the bare name in every case — other readers of that field
	// look planned names up by it, and a composite string matches none of them,
	// which fails by finding nothing rather than by reporting anything.
	//
	// The two are never both set: this dialect reads "#^name" as a block
	// address and anything else after "#" as a section name, so a link has one
	// or neither. Both are empty when the author wrote no address at all, and
	// on every diagnostic about something other than a link.
	Section string
	Block   string
}

// TOCEntry is one heading in document order, with the id assigned to it
// (see heading.go's CJK-safe slug algorithm).
type TOCEntry struct {
	Level int
	Text  string
	ID    string
}

// Result is everything one HTML render produces.
type Result struct {
	HTML        string
	Diagnostics []Diagnostic
	TOC         []TOCEntry
	// TitleAnchor is the id the page's visible title must carry, and is set
	// only when this render removed an authored opening heading that said the
	// same thing. That heading was a real place in the document and a link
	// could name it; dropping it as a duplicate took the place away while
	// leaving the words on screen, so the anchor moves to where those words
	// now are. It is empty when no such heading was written, because there is
	// then nothing for it to be evidence of.
	TitleAnchor string
}

// embedPolicy caps embed transclusion at exactly one level deep: an
// embedded note's own ![[...]] occurrences render as plain wikilink-style
// text rather than being further transcluded. This alone makes runaway
// or cyclic embed chains structurally impossible — no visited-set is
// needed, because the recursion can only ever go one level before the
// policy flips to embedsDenied.
type embedPolicy int

const (
	embedsAllowed embedPolicy = iota
	embedsDenied
)

// Pipeline is a configured, reusable markdown pipeline for one captured vault
// generation.
type Pipeline struct {
	idx           *graph.Index
	transclusions Transclusions
	titles        Titles
	md            goldmark.Markdown
}

// New builds a rendering pipeline from one generation's link resolver and
// captured transclusion bodies. It enables GFM (tables, task lists,
// strikethrough, and autolinked bare URLs), footnotes, the inert
// authored-markup subset used by Japanese lessons, and the ==highlight==
// inline extension. Both capabilities must describe the same generation and
// must not be nil.
//
// Footnotes are a parser concern rather than one of this package's own dialect
// passes, and enabling them is what stops "[^name]" being read as an ordinary
// reference link: that reading resolved the reference against the definition's
// prose, so a note about the scope of a study became a link to a page that
// does not exist, and the note's own text was swallowed as the link's
// destination. With the extension, the reference and its definition are two
// ends of one anchor pair inside the page, which needs no script to work.
func New(idx *graph.Index, transclusions Transclusions, titles Titles) *Pipeline {
	if idx == nil {
		panic("render: New requires a non-nil *graph.Index")
	}
	if transclusions == nil {
		panic("render: New requires non-nil Transclusions")
	}
	if titles == nil {
		panic("render: New requires non-nil Titles")
	}
	return &Pipeline{
		idx:           idx,
		transclusions: transclusions,
		titles:        titles,
		md: goldmark.New(
			goldmark.WithExtensions(
				extension.GFM,
				// The footnote extension keeps owning what a footnote is and
				// how it is numbered; it is only told what to prefix the ids
				// with, per body, so the several bodies one page assembles do
				// not all call their first note the same thing.
				extension.NewFootnote(extension.WithFootnoteIDPrefixFunction(footnoteRegionPrefix)),
				highlightExtension{}, codeBlockExtension{}, tableWrapExtension{}, safeMarkupExtension{},
			),
		),
	}
}

// HTML renders one note's body: the markdown-to-HTML pipeline, plus the
// three passes that only make sense once at the top level (never
// recursively, for callouts or embeds — see their doc comments): the
// body-first-H1 removal (the page's own title comes from frontmatter, a
// duplicate leading H1 in the body would show it twice); CJK-safe
// heading slug assignment + TOC collection, which must run over the
// final, fully-assembled HTML (embeds and callouts already spliced in)
// so heading ids stay unique across the whole page in one pass, rather
// than colliding across independently-slugged sub-renders; and asset
// resolution.
//
// relPath is where this body lives in the vault, and it is required
// because a body alone cannot be rendered correctly: markdown writes an
// image path relative to the note that mentions it, so the same bytes
// mean different files depending on which note they came from. A
// transcluded body is resolved against its own note before it is spliced
// in, so what arrives here is already routed and passes through
// untouched.
func (r *Pipeline) HTML(relPath, title, body string, lang wording.Lang) Result {
	return r.HTMLIn(hostRegion, relPath, title, body, lang)
}

// HTMLIn is HTML for a body that will be placed on a page already carrying
// another separately rendered one — a concept sheet opening over the lesson
// that cites it, cloned into the same document the moment the reader asks for
// it. Both bodies then live in one id space, so the caller names the region
// this one occupies.
//
// What that buys is bounded, and worth stating exactly: the footnote ids this
// render mints — the references, their definitions, and the same for any
// callout or transclusion inside it — are named under region, and nested
// regions carry it too. It is not a general id namespace. Heading ids are
// unique within one render and are not qualified by region, so two bodies
// composed onto one page can still stamp the same heading id, and neither
// knows anything about the fixed ids the surrounding chrome uses.
//
// region must be distinct for each such body on a page and must be derived
// from the page rather than from a running process, so two readers of one
// lesson receive the same bytes.
func (r *Pipeline) HTMLIn(region, relPath, title, body string, lang wording.Lang) Result {
	page := &composition{base: region, lang: lang}
	body, unclosedComment := stripObsidianComments(body)
	body, titleAnchor := removeBodyFirstH1(title, body)
	res := r.renderBody(body, embedsAllowed, page, region)
	res.Diagnostics = appendUnclosedComment(res.Diagnostics, unclosedComment)
	// The anchor the page title inherits is claimed before any body heading is
	// slugged, so a section further down that reduces to the same name is the
	// one that has to move aside.
	htmlOut, toc := assignHeadingSlugs(res.HTML, titleAnchor)
	res.HTML = resolveAssetHrefs(htmlOut, relPath)
	res.TOC = toc
	res.TitleAnchor = titleAnchor
	return res
}

// hostRegion is the note's own body: the first thing rendered on a page and
// the only region whose footnote ids are left bare. A page with no callout and
// no transclusion — nearly every page — therefore carries the plain ids the
// syntax is normally written with.
const hostRegion = ""

// composition is the state shared by every body one render assembles. Each of
// those bodies is parsed on its own and spliced in afterwards, so each numbers
// its footnotes from one; without something to tell them apart the assembled
// result carries "fn:1" several times and a browser resolving a fragment
// reaches whichever came first.
//
// base is the region this whole assembly was given, and every region it hands
// out is named under it: a callout inside a concept sheet gets the sheet's
// prefix, not one that would sit alongside the lesson's own callouts. It
// qualifies footnote ids and nothing else.
//
// The counter belongs to the render rather than the process: two requests for
// one note must produce the same bytes, so nothing here may depend on how many
// pages were rendered before. Regions are numbered in the order they are
// assembled, which is document order, so the same input always yields the same
// ids — including when one note is transcluded twice, since those are two
// occurrences and take two numbers.
//
// blocks is the set of block addresses this assembly has already anchored. It
// belongs to the page rather than to one body for the same reason the counter
// does: the bodies are parsed separately and spliced together, so only
// something they share can keep one address from being stamped on two elements
// of the finished document.
type composition struct {
	base    string
	regions int
	blocks  map[string]bool
	// lang is the language this render's own sentences are written in. It sits
	// here rather than on the pipeline because a pipeline belongs to a scan and
	// a language belongs to a request: one pipeline serves every reader, and
	// each of them may have chosen differently.
	lang wording.Lang
}

func (c *composition) nextRegion() string {
	c.regions++
	return c.base + "y" + strconv.Itoa(c.regions) + "-"
}

// claimBlockAnchor reports whether id is still free on this page, taking it
// when it is. Bodies are assembled in document order, so the first block
// written under a repeated address is the one that keeps it — the reading the
// excerpt scan already takes, and the one a browser would take of a repeated id
// regardless.
func (c *composition) claimBlockAnchor(id string) bool {
	if c.blocks[id] {
		return false
	}
	if c.blocks == nil {
		c.blocks = map[string]bool{}
	}
	c.blocks[id] = true
	return true
}

// collector gathers one region's diagnostics and carries the page that region
// belongs to, so a nested render can claim its own footnote id space without
// another parameter on every function between here and there.
type collector struct {
	diags []Diagnostic
	page  *composition
}

func (c *collector) report(d *Diagnostic) { c.diags = append(c.diags, *d) }

// footnoteRegionAttr names the document attribute carrying one region's
// footnote id prefix. The renderer writes nothing for a document node's
// attributes, so this reaches the footnote extension and nothing else.
const footnoteRegionAttr = "yomihonFootnoteRegion"

// footnoteRegionPrefix is the single owner of footnote id naming: goldmark's
// own extension asks for the prefix and this answers from the document being
// rendered. Nothing here re-implements footnote parsing or numbering.
func footnoteRegionPrefix(n ast.Node) []byte {
	doc := n.OwnerDocument()
	if doc == nil {
		return nil
	}
	value, ok := doc.AttributeString(footnoteRegionAttr)
	if !ok {
		return nil
	}
	prefix, ok := value.([]byte)
	if !ok {
		return nil
	}
	return prefix
}

// render is the shared markdown-to-HTML core: the dialect preprocessing
// passes (wikilinks, embeds, callouts, mermaid fences — see wikilink.go
// and callout.go) followed by the goldmark conversion. It is used both
// by the top-level HTML() call and recursively, for a callout's own body
// and a transcluded embed's body — "the same rendering pipeline" the
// callout dialect rule calls for, so nested formatting and nested
// wikilinks work inside a callout or an embed exactly as they do at the
// top level.
//
// Each such body is a region of its own: it is parsed separately, so it gets
// its own footnote id space, named under whatever region this render was
// itself given. One render therefore keeps its footnote ids distinct — and a
// caller composing several renders onto one page says where each of them sits
// (see HTMLIn), because this package cannot see the others.
//
// The body arrives with its Obsidian %% comments already removed, and this
// does not remove them again. A callout's lines are part of the host note's
// own source and came off the scan HTMLIn ran; a transcluded body is scanned
// by embedScope, which is where it is first read. Scanning either a second
// time would let this pass reopen a marker the first had ruled to be literal
// text — a percent sign an author displayed inside a code span — and hide
// words that were meant to stay.
func (r *Pipeline) render(body string, allowEmbed embedPolicy, page *composition) Result {
	return r.renderBody(body, allowEmbed, page, page.nextRegion())
}

func (r *Pipeline) renderBody(body string, allowEmbed embedPolicy, page *composition, region string) Result {
	col := &collector{page: page}
	// This prefix belongs to preprocess, never to vault text. Neutralizing an
	// authored copy before placeholders exist prevents source from selecting or
	// relocating renderer-owned HTML during substituteBlocks.
	body = strings.ReplaceAll(body, "<!--yomihon-block:", "&lt;!--yomihon-block:")
	body = strings.Map(func(r rune) rune {
		if strings.ContainsRune(inlinePlaceholderRunes, r) {
			return -1
		}
		return r
	}, body)
	source, blocks, inline := r.preprocess(body, allowEmbed, col)

	// Parse and render as two steps rather than one Convert call, which is
	// exactly what Convert does, so this region's id prefix can be attached to
	// the document the footnote extension will ask about.
	src := []byte(source)
	doc := r.md.Parser().Parse(text.NewReader(src))
	doc.SetAttributeString(footnoteRegionAttr, []byte(region))

	var buf bytes.Buffer
	if err := r.md.Renderer().Render(&buf, src, doc); err != nil {
		// Never fail the whole render. This is normally
		// unreachable — rendering only errors if a configured
		// renderer returns one, and the default HTML renderer writing to
		// a bytes.Buffer never fails — but the fallback keeps the page
		// non-blank even if a future extension breaks that assumption.
		col.report(&Diagnostic{
			Kind:    DiagRenderFailed,
			Message: fmt.Sprintf("markdown render failed: %v", err),
		})
		return Result{HTML: "<pre>" + html.EscapeString(body) + "</pre>", Diagnostics: col.diags}
	}

	return Result{HTML: substituteBlocks(buf.String(), blocks, inline), Diagnostics: col.diags}
}

// removeBodyFirstH1 drops a leading level-1 ATX heading when the page is
// already showing that same text as its title, which is the only reason to
// drop it: the reader would otherwise see the title twice. Only the very first
// non-blank line qualifies — any other H1 later in the document (or inside a
// callout or embed, which never reach this function — see HTML's doc comment)
// is left untouched.
//
// A note whose frontmatter declares no title is displayed under its filename,
// and its opening heading is then the only place the document says what it is.
// Removing that heading destroyed the sentence and put a filename in its
// place — on an ordinary folder, where nothing carries frontmatter, that was
// every file.
//
// The second return is the anchor that heading would have been given, and is
// returned only when one was actually removed. A link may name that section,
// and after the removal the only thing on the page still saying those words is
// the title, so the title is where the anchor has to go. Where no such heading
// was written there is nothing to inherit, and claiming an anchor anyway would
// be inventing a section the author never wrote.
func removeBodyFirstH1(title, body string) (stripped, anchor string) {
	lines := strings.Split(body, "\n")
	i := 0
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	if i >= len(lines) || !strings.HasPrefix(lines[i], "# ") {
		return body, ""
	}
	heading := strings.TrimSpace(strings.TrimPrefix(lines[i], "# "))
	if heading != strings.TrimSpace(title) {
		return body, ""
	}
	return strings.Join(slices.Delete(slices.Clone(lines), i, i+1), "\n"), slugify(heading)
}
