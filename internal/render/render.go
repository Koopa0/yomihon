// Package render answers three questions about one vault file, in one place so
// every face of the program answers them alike: how a note reads, what words it
// contributes to a search, and what kind of file it is. It reports what it
// cannot render rather than repairing it, and authored HTML is inert display
// input: the ruby subset survives, and executable markup is shown as text.
package render

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
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

// Titles answers which notes declare a given title. A title is not a name a link
// resolves by, so this never affects resolution: it is how a page that already
// failed to resolve a name can say the note is there under a name links do not
// follow. It returns display names rather than paths, which is all a sentence needs.
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

	// DiagWikilinkTitleOnly means the target names some note's declared title,
	// which is not a name a link resolves by, so the link is broken either way.
	// It is a separate kind because the repair is: nothing needs writing, and an
	// alias on the note the citation meant makes the existing link work.
	DiagWikilinkTitleOnly DiagnosticKind = "wikilink-title-only"

	// DiagTitleTruncatedAtHash means this note's own title is exactly what its
	// filename becomes when cut where YAML starts a comment in an unquoted value.
	// It is an observation, not an accusation: a title deliberately written short
	// in quotes produces the same coincidence, and nothing parsed tells them apart.
	DiagTitleTruncatedAtHash DiagnosticKind = "title-truncated-at-hash"
	// DiagUnknownCallout means a "> [!type]" callout's type is not one
	// of the recognized callout types; it was rendered as a plain
	// blockquote instead of being dropped.
	DiagUnknownCallout DiagnosticKind = "unknown-callout"
	// DiagRiskyFence means a fenced code block's content looks like the
	// wikilink, callout or table syntax the dialect passes would otherwise
	// convert; it was left untouched. At most one per render call.
	DiagRiskyFence DiagnosticKind = "risky-fence"
	// DiagEmbedFragmentMissing means an embed named a section or block its
	// target's captured body does not contain, so nothing of that note is shown
	// and a notice stands where the excerpt would. The author named one place;
	// widening to the whole note would present a scope they never chose as
	// their own.
	DiagEmbedFragmentMissing DiagnosticKind = "embed-fragment-missing"
	// DiagEmbedFragmentRepeated means an embed named a section its target carries
	// more than once. The first is shown, and the count is reported because an
	// excerpt chosen from several looks exactly like the only one there was.
	DiagEmbedFragmentRepeated DiagnosticKind = "embed-fragment-repeated"
	// DiagEmbedNotExpanded means an embed written inside a transcluded body was
	// rendered as an ordinary link, since transclusion stops one level down.
	DiagEmbedNotExpanded DiagnosticKind = "embed-not-expanded"
	// DiagLinkFragmentMissing means a plain link named a block its target's
	// captured body does not carry. The link is left leading to the note itself,
	// since an address the destination stamps no anchor for would promise a block
	// and land the reader somewhere else.
	DiagLinkFragmentMissing DiagnosticKind = "link-fragment-missing"
	// DiagLinkSectionMissing means a plain link named a section neither scan of
	// its target's captured body could find. The address is kept exactly as
	// written, because a heading id is stamped by a pass that sees headings these
	// scans do not, so a miss is a name they failed to find rather than one the
	// page is certain to lack. That is why it is a separate kind from a missing
	// block, which withdraws the author's address.
	DiagLinkSectionMissing DiagnosticKind = "link-section-missing"
	// DiagCommentUnclosed means a "%%" comment marker never met a second one, so
	// everything after it is hidden from the page. Obsidian hides it too, so the
	// words are not restored; the reader is told where the silence begins.
	DiagCommentUnclosed DiagnosticKind = "comment-unclosed"
	// DiagRenderFailed means the markdown renderer returned an error. It is
	// normally unreachable, and kept so an extension that breaks that assumption
	// produces a visible diagnostic rather than a blank page.
	DiagRenderFailed DiagnosticKind = "render-failed"
)

// Diagnostic is one rendering-time note about content the dialect passes could
// not cleanly handle. Display-only: yomihon reports, it never fixes or rejects.
// Deciding that something is worth surfacing is a rendering decision, which is
// why it lives here and not beside the resolver.
type Diagnostic struct {
	Kind    DiagnosticKind
	Target  string // the offending wikilink target / callout type / etc.
	Message string // human-readable

	// Section and Block carry the two halves of an address a link wrote after
	// "#", so a panel can say how the link was read; Target stays the bare name,
	// which is what other readers of that field look planned names up by. The two
	// are never both set, since "#^name" is a block address and anything else
	// after "#" is a section name.
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
	// TitleAnchor is the id the page's visible title has to carry, set only when
	// this render removed an authored opening heading saying the same thing. That
	// heading was a place a link could name, so the anchor moves to where its
	// words now are. It is empty when no such heading was written.
	TitleAnchor string
	// TranscludedIdentity is one digest over everything this render pulled in
	// from other notes: each excerpt expanded, in the order the page shows them,
	// bound to its source note and the scope decision that cut it. Empty when
	// nothing was transcluded. The reading page stamps it beside its content
	// identity, so a freshness answer can say whether a reload would deliver
	// different transcluded words.
	TranscludedIdentity string
}

// embedPolicy caps embed transclusion at exactly one level deep: an embedded
// note's own ![[...]] renders as plain wikilink text. That alone makes runaway
// and cyclic chains impossible, so no visited set is needed.
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
// captured transclusion bodies, both of which describe the same generation and
// must not be nil. It enables GFM, footnotes, the inert authored-markup subset
// the Japanese lessons use, and the ==highlight== inline extension. Footnotes
// are enabled so "[^name]" is not read as an ordinary reference link, which
// resolved the reference against the definition's prose.
func New(idx *graph.Index, transclusions Transclusions, titles Titles) *Pipeline {
	if idx == nil {
		panic("render: New requires a non-nil *graph.Index")
	}
	if transclusions == nil {
		panic("render: New requires a non-nil Transclusions")
	}
	if titles == nil {
		panic("render: New requires a non-nil Titles")
	}
	return &Pipeline{
		idx:           idx,
		transclusions: transclusions,
		titles:        titles,
		md: goldmark.New(
			goldmark.WithExtensions(
				extension.GFM,
				// The extension is told only what to prefix the ids with, per body,
				// so several bodies on one page do not share a first note's id.
				extension.NewFootnote(extension.WithFootnoteIDPrefixFunction(footnoteRegionPrefix)),
				highlightExtension{}, codeBlockExtension{}, tableWrapExtension{}, safeMarkupExtension{},
			),
		),
	}
}

// HTML renders one note's body: the markdown pipeline, plus the three passes that
// only make sense once at the top level — removing a leading H1 that duplicates
// the page's title, assigning heading slugs and collecting the table of contents
// over the assembled HTML so ids stay unique, and resolving assets. relPath is
// required because markdown writes an image path relative to its own note.
func (r *Pipeline) HTML(relPath, title, body string, lang wording.Lang) Result {
	return r.HTMLIn(hostRegion, relPath, title, body, lang)
}

// HTMLIn is HTML for a body placed on a page already carrying another separately
// rendered one, so the caller names the region this one occupies. What that buys
// is bounded: footnote ids are named under region and nested regions carry it,
// while heading ids are not qualified. region must be distinct per body and
// derived from the page rather than a running process, so two readers of one
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
	htmlOut, toc := assignHeadingIDs(res.HTML, titleAnchor)
	res.HTML = resolveAssetHrefs(htmlOut, relPath)
	res.TOC = toc
	res.TitleAnchor = titleAnchor
	res.TranscludedIdentity = transcludedIdentity(page.transcluded)
	return res
}

// hostRegion is the note's own body: the first thing rendered on a page and the
// only region whose footnote ids are left bare.
const hostRegion = ""

// composition is the state shared by every body one render assembles. Each body
// is parsed on its own and numbers its footnotes from one, so base names every
// region it hands out and qualifies footnote ids and nothing else. The counter
// belongs to the render rather than the process, since two requests for one note
// have to produce the same bytes. blocks is the set of block addresses already
// anchored, held here because only something the bodies share keeps one unique.
type composition struct {
	base    string
	regions int
	blocks  map[string]bool
	// transcluded records every excerpt this assembly expanded, in document
	// order. Only something the separately parsed bodies share accounts for all.
	transcluded []transcludedExcerpt
	// lang is the language this render's own sentences are written in. It sits
	// here rather than on the pipeline, which belongs to a scan and serves readers
	// who may each have chosen differently.
	lang wording.Lang
}

// transcludedExcerpt is one embed a render actually expanded, recorded as the
// page consumed it: the source note, how many places answered the address the
// cut was made at, and the sliced bytes. Identical bytes from two notes are two
// different excerpts, because an image inside each resolves against its own
// note's directory; the same bytes cut as the only candidate and as the first
// of several are two excerpts too, because the page says which it is showing.
type transcludedExcerpt struct {
	path    string
	matches int
	slice   string
}

// transcludedIdentity is one digest over everything a render pulled in from other
// notes. Every value is length-delimited before hashing, so two different
// collections cannot frame the same byte stream. Nothing transcluded yields the
// empty string rather than a digest of an empty list.
func transcludedIdentity(excerpts []transcludedExcerpt) string {
	if len(excerpts) == 0 {
		return ""
	}
	var framed []byte
	for i := range excerpts {
		e := &excerpts[i]
		framed = frameValue(framed, e.path)
		framed = frameValue(framed, strconv.Itoa(e.matches))
		framed = frameValue(framed, e.slice)
	}
	sum := sha256.Sum256(framed)
	return hex.EncodeToString(sum[:])
}

// frameValue appends one length-delimited value to the byte stream the digest
// reads, so a value can never bleed into its neighbour.
func frameValue(framed []byte, value string) []byte {
	framed = binary.BigEndian.AppendUint64(framed, uint64(len(value)))
	return append(framed, value...)
}

func (c *composition) nextRegion() string {
	c.regions++
	return c.base + "y" + strconv.Itoa(c.regions) + "-"
}

// claimBlockAnchor reports whether id is still free on this page, taking it when
// it is. Bodies are assembled in document order, so the first block written under
// a repeated address keeps it, which is what a browser would do anyway.
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

// render is the shared markdown-to-HTML core: the dialect preprocessing passes
// followed by the goldmark conversion. It serves the top-level HTML call and
// recurses for a transcluded embed's body, which is another note's text carrying
// footnotes defined in the note it came from: that body is its own region with
// its own footnote id space, under whatever region this render was given. A
// callout's body is not among them — it is the note's own text and is read by
// the note's own parse. The body arrives with its Obsidian %% comments already
// removed, and a second pass could reopen a marker ruled literal.
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
		// Never fail the whole render. This is normally unreachable, but the
		// fallback keeps the page non-blank if an extension breaks that.
		col.report(&Diagnostic{
			Kind:    DiagRenderFailed,
			Message: fmt.Sprintf("markdown render failed: %v", err),
		})
		return Result{HTML: "<pre>" + html.EscapeString(body) + "</pre>", Diagnostics: col.diags}
	}

	return Result{HTML: substituteBlocks(buf.String(), blocks, inline), Diagnostics: col.diags}
}

// removeBodyFirstH1 drops a leading level-1 ATX heading when the page already
// shows that same text as its title. Only the very first non-blank line qualifies,
// and a note displayed under its filename keeps its heading. The second return is
// the anchor that heading would have been given, and only when one was removed:
// the title is then the only thing on the page still saying those words.
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
	return strings.Join(slices.Delete(slices.Clone(lines), i, i+1), "\n"), graph.SectionID(heading)
}
