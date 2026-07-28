// Package render owns projections of Obsidian-dialect markdown. Pipeline turns
// it into HTML for reading; PlainText and PlainSections expose the same parsed
// dialect to lexical and semantic retrieval without duplicating a parser.
//
// Fault-tolerant by contract: it renders what it can and reports
// what it can't via Diagnostics — it never fixes a note, never fails the
// whole render, and never returns a blank page. Authored HTML is inert display
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
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"

	"github.com/koopa0/yomihon/internal/graph"
)

// Resolver is the minimal wikilink-resolution capability render needs.
// Defined here, in the consumer: internal/graph's concrete *Index
// satisfies this structurally, with no explicit binding needed.
type Resolver interface {
	Resolve(name string) graph.Resolution
}

// Transclusions is the captured note-body set used by embed expansion. The
// concrete reading snapshot implements this narrow consumer-owned capability,
// so link resolution and transclusion bodies can come from one generation.
type Transclusions interface {
	Transclusion(path string) (body string, ok bool)
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
	// DiagUnknownCallout means a "> [!type]" callout's type is not one
	// of the recognized callout types; it was rendered as a plain
	// blockquote instead of being dropped.
	DiagUnknownCallout DiagnosticKind = "unknown-callout"
	// DiagRiskyFence means a fenced code block's content looks like
	// wikilink/callout/table syntax the dialect passes would otherwise
	// convert; it was left untouched. At most one per render call (see
	// the preprocessing pass doc).
	DiagRiskyFence DiagnosticKind = "risky-fence"
	// DiagRenderFailed means goldmark's own Convert returned an error —
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
	idx           Resolver
	transclusions Transclusions
	md            goldmark.Markdown
}

// New builds a rendering pipeline from one generation's link resolver and
// captured transclusion bodies. It enables GFM (tables, task lists, and
// strikethrough), the inert authored-markup subset used by Japanese lessons,
// and the ==highlight== inline extension. Both capabilities must describe the
// same generation and must not be nil.
func New(idx Resolver, transclusions Transclusions) *Pipeline {
	if idx == nil {
		panic("render: New requires a non-nil Resolver")
	}
	if transclusions == nil {
		panic("render: New requires non-nil Transclusions")
	}
	return &Pipeline{
		idx:           idx,
		transclusions: transclusions,
		md: goldmark.New(
			goldmark.WithExtensions(extension.GFM, highlightExtension{}, codeBlockExtension{}, tableWrapExtension{}, safeMarkupExtension{}),
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
func (r *Pipeline) HTML(relPath, title, body string) Result {
	body = stripObsidianComments(body)
	body = removeBodyFirstH1(title, body)
	res := r.renderBody(body, embedsAllowed)
	htmlOut, toc := assignHeadingSlugs(res.HTML)
	res.HTML = resolveAssetHrefs(htmlOut, relPath)
	res.TOC = toc
	return res
}

// render is the shared markdown-to-HTML core: the dialect preprocessing
// passes (wikilinks, embeds, callouts, mermaid fences — see wikilink.go
// and callout.go) followed by the goldmark conversion. It is used both
// by the top-level HTML() call and recursively, for a callout's own body
// and a transcluded embed's body — "the same rendering pipeline" the
// callout dialect rule calls for, so nested formatting and nested
// wikilinks work inside a callout or an embed exactly as they do at the
// top level.
func (r *Pipeline) render(body string, allowEmbed embedPolicy) Result {
	return r.renderBody(stripObsidianComments(body), allowEmbed)
}

func (r *Pipeline) renderBody(body string, allowEmbed embedPolicy) Result {
	var diags []Diagnostic
	// This prefix belongs to preprocess, never to vault text. Neutralizing an
	// authored copy before placeholders exist prevents source from selecting or
	// relocating renderer-owned HTML during substituteBlocks.
	body = strings.ReplaceAll(body, "<!--yomihon-block:", "&lt;!--yomihon-block:")
	source, blocks := r.preprocess(body, allowEmbed, &diags)

	var buf bytes.Buffer
	if err := r.md.Convert([]byte(source), &buf); err != nil {
		// Never fail the whole render. This is normally
		// unreachable — goldmark's Convert only errors if a configured
		// renderer returns one, and the default HTML renderer writing to
		// a bytes.Buffer never fails — but the fallback keeps the page
		// non-blank even if a future extension breaks that assumption.
		diags = append(diags, Diagnostic{
			Kind:    DiagRenderFailed,
			Message: fmt.Sprintf("markdown render failed: %v", err),
		})
		return Result{HTML: "<pre>" + html.EscapeString(body) + "</pre>", Diagnostics: diags}
	}

	return Result{HTML: substituteBlocks(buf.String(), blocks), Diagnostics: diags}
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
func removeBodyFirstH1(title, body string) string {
	lines := strings.Split(body, "\n")
	i := 0
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	if i >= len(lines) || !strings.HasPrefix(lines[i], "# ") {
		return body
	}
	heading := strings.TrimSpace(strings.TrimPrefix(lines[i], "# "))
	if heading != strings.TrimSpace(title) {
		return body
	}
	return strings.Join(slices.Delete(slices.Clone(lines), i, i+1), "\n")
}
