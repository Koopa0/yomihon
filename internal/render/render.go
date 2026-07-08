// Package render turns Obsidian-dialect markdown into HTML.
//
// Fault-tolerant by contract: it renders what it can and reports
// what it can't via Diagnostics — it never fixes a note, never fails the
// whole render, and never returns a blank page. Raw HTML passes through
// unsanitized because the vault is a trusted, local-only corpus;
// Japanese lesson bodies are hand-written <ruby> markup that must
// survive.
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
	goldmarkhtml "github.com/yuin/goldmark/renderer/html"

	"github.com/koopa0/yomihon/internal/graph"
)

// Resolver is the minimal wikilink-resolution capability render needs.
// Defined here, in the consumer: internal/graph's concrete *Index
// satisfies this structurally, with no explicit binding needed.
type Resolver interface {
	Resolve(name string) graph.Resolution
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
// couldn't cleanly handle. Display-only: kurodo reports, it
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

// Renderer is a configured, reusable markdown pipeline for one vault.
type Renderer struct {
	root string
	idx  Resolver
	md   goldmark.Markdown
}

// New builds the rendering pipeline for the vault rooted at root: GFM
// (tables, task lists, strikethrough), raw HTML passthrough (Japanese
// lesson bodies rely on hand-written <ruby>/<rt> markup that must survive
// byte-for-byte), and the ==highlight== inline extension. idx resolves
// wikilink/embed targets against the vault-wide symbol table
// (internal/graph); it must not be nil.
func New(root string, idx Resolver) *Renderer {
	if idx == nil {
		panic("render: New requires a non-nil Resolver")
	}
	return &Renderer{
		root: root,
		idx:  idx,
		md: goldmark.New(
			goldmark.WithExtensions(extension.GFM, highlightExtension{}, codeBlockExtension{}, tableWrapExtension{}),
			goldmark.WithRendererOptions(goldmarkhtml.WithUnsafe()),
		),
	}
}

// HTML renders one note's body: the markdown-to-HTML pipeline, plus the
// two passes that only make sense once at the top level (never
// recursively, for callouts or embeds — see their doc comments): the
// body-first-H1 removal (the page's own title comes from frontmatter, a
// duplicate leading H1 in the body would show it twice) and CJK-safe
// heading slug assignment + TOC collection, which must run over the
// final, fully-assembled HTML (embeds and callouts already spliced in)
// so heading ids stay unique across the whole page in one pass, rather
// than colliding across independently-slugged sub-renders.
func (r *Renderer) HTML(body string) Result {
	body = removeBodyFirstH1(body)
	res := r.render(body, embedsAllowed)
	htmlOut, toc := assignHeadingSlugs(res.HTML)
	res.HTML = htmlOut
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
func (r *Renderer) render(body string, allowEmbed embedPolicy) Result {
	var diags []Diagnostic
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

// removeBodyFirstH1 drops a leading level-1 ATX heading: the page's title
// comes from frontmatter (rendered separately by the caller), so a body
// that opens with its own "# Heading" would show the title twice. Only
// the very first non-blank line qualifies — any other H1 later in the
// document (or inside a callout/embed, which never reach this function —
// see HTML's doc comment) is left untouched.
func removeBodyFirstH1(body string) string {
	lines := strings.Split(body, "\n")
	i := 0
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	if i >= len(lines) || !strings.HasPrefix(lines[i], "# ") {
		return body
	}
	return strings.Join(slices.Delete(slices.Clone(lines), i, i+1), "\n")
}
