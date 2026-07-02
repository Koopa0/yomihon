package render

import (
	"fmt"
	"html"
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/koopa0/kurodo/internal/graph"
	"github.com/koopa0/kurodo/internal/vault"
)

// blockPlaceholder is the marker substituted into the markdown source at
// a callout/embed/mermaid site and replaced with the real (already
// rendered) HTML after goldmark converts. A short, empty, open-then-close
// tag pair is safe whether goldmark's parser sees it as a standalone HTML
// block (surrounded by blank lines, as callouts and mermaid blocks are
// emitted) or as two raw inline HTML tokens mid-line (as an inline embed
// can be) — unlike substituting the real HTML directly inline, which
// risks goldmark re-parsing already-rendered markup as markdown text.
func blockPlaceholder(i int) string {
	return fmt.Sprintf(`<div data-kurodo-block="%d"></div>`, i)
}

func substituteBlocks(htmlOut string, blocks []string) string {
	for i, b := range blocks {
		htmlOut = strings.Replace(htmlOut, blockPlaceholder(i), b, 1)
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
// otherwise convert — worth a single warning when found inside a fence
// (docs/vault-model.md's Code fence 安全 section), never acted on.
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
// Obsidian %%...%% comment state as it scans, and treats neither as
// syntax to convert; %% state is line-spanning, carried in inComment
// across the whole call (see convertLine). At most one risky-fence
// diagnostic is recorded per call (docs/vault-model.md's "一次 build
// warning", not one per occurrence) — this dedup is scoped to a single
// preprocess call, not threaded across the recursive calls render makes
// for a callout body or a transcluded embed, so a note with risky fences
// in more than one such structurally-separate region can produce more
// than one diagnostic overall. That is a deliberate simplicity trade:
// each region gets its own accurate warning, at the cost of occasional
// (harmless) redundancy across regions, rather than plumbing a shared
// counter through every recursive render call for a rare case.
func (r *Renderer) preprocess(body string, allowEmbed embedPolicy, diags *[]Diagnostic) (out string, blocks []string) {
	st := &preprocessState{lines: strings.Split(body, "\n")}
	st.kept = make([]string, 0, len(st.lines))

	for st.i < len(st.lines) {
		switch {
		case st.inFence:
			r.scanFenceLine(st, diags)
		case r.tryOpenFence(st):
			// handled: either entered a fence, or fully consumed a
			// mermaid block — see tryOpenFence.
		case r.tryConsumeCallout(st, allowEmbed, diags):
			// handled: a known-type callout block was consumed.
		default:
			st.kept = append(st.kept, r.convertLine(st.lines[st.i], &st.inComment, allowEmbed, diags, &st.blocks))
			st.i++
		}
	}
	return strings.Join(st.kept, "\n"), st.blocks
}

// preprocessState carries preprocess's running scan state so the
// per-construct handlers below (fence, mermaid, callout) can share it
// without a long, repeated parameter list.
type preprocessState struct {
	lines []string
	i     int

	kept   []string
	blocks []string

	inFence            bool
	fenceByte          byte
	riskyFenceReported bool
	inComment          bool
}

// scanFenceLine handles one line while st.inFence is true: either it
// closes the fence, or it's fence content, checked once for a
// risky-looking pattern worth a single warning (see preprocess's doc
// comment on the dedup scope).
func (r *Renderer) scanFenceLine(st *preprocessState, diags *[]Diagnostic) {
	line := st.lines[st.i]
	switch {
	case fenceCloses(line, st.fenceByte):
		st.inFence = false
	case !st.riskyFenceReported && looksRisky(line):
		st.riskyFenceReported = true
		*diags = append(*diags, Diagnostic{
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
func (r *Renderer) tryOpenFence(st *preprocessState) bool {
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
// double-encoding risk). assets/js/kurodo.js reads that attribute,
// URL-decodes it, and replaces the element's content with mermaid's
// rendered SVG client-side — mirroring koopa0.dev's markdown.service.ts
// processMermaidDiagrams/renderMermaid DOM shape exactly (see that file's
// doc comments); the loading mechanism differs only because kurodo has no
// bundler.
func (r *Renderer) consumeMermaid(st *preprocessState, marker byte) {
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
func (r *Renderer) tryConsumeCallout(st *preprocessState, allowEmbed embedPolicy, diags *[]Diagnostic) bool {
	typ, fold, title, ok := calloutStart(st.lines[st.i])
	if !ok {
		return false
	}
	bucket, defaultTitle := calloutBucketOf(typ)
	if bucket == bucketUnknown {
		*diags = append(*diags, Diagnostic{
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
	calloutHTML := r.renderCallout(bucket, defaultTitle, fold, title, strings.Join(bodyLines, "\n"), allowEmbed, diags)
	st.blocks = append(st.blocks, calloutHTML)
	st.kept = append(st.kept, "", blockPlaceholder(len(st.blocks)-1), "")
	return true
}

// convertLine handles one line's %%...%% comment state and delegates the
// non-commented segments to convertWikilinks. inComment persists across
// calls (the caller passes the same pointer for every line of one
// preprocess call), so a comment opened on one line and closed on a
// later one is honored — Obsidian comments are not restricted to a
// single line.
func (r *Renderer) convertLine(line string, inComment *bool, allowEmbed embedPolicy, diags *[]Diagnostic, blocks *[]string) string {
	parts := strings.Split(line, "%%")
	var b strings.Builder
	for i, part := range parts {
		if i > 0 {
			b.WriteString("%%")
		}
		if *inComment {
			b.WriteString(part)
		} else {
			b.WriteString(r.convertWikilinks(part, allowEmbed, diags, blocks))
		}
		if i < len(parts)-1 {
			*inComment = !*inComment
		}
	}
	return b.String()
}

// convertWikilinks scans text (one line, or one %%-delimited segment of a
// line — never spanning a newline) for [[...]] and ![[...]] and replaces
// each with its rendered form.
func (r *Renderer) convertWikilinks(text string, allowEmbed embedPolicy, diags *[]Diagnostic, blocks *[]string) string {
	return wikilinkToken.ReplaceAllStringFunc(text, func(m string) string {
		embed := strings.HasPrefix(m, "!")
		raw := m
		if embed {
			raw = m[1:]
		}
		inner := raw[2 : len(raw)-2]
		target, display, ok := graph.SplitWikilink(inner)
		if !ok {
			// [[#heading]] or [[^block]] stripped to empty: a same-file
			// anchor jump, not a cross-file link — render the original
			// display text as plain text, don't attempt to resolve it.
			return html.EscapeString(display)
		}
		if embed {
			blockHTML := r.renderEmbed(target, allowEmbed, diags)
			*blocks = append(*blocks, blockHTML)
			return blockPlaceholder(len(*blocks) - 1)
		}
		return r.renderWikilink(target, display, diags)
	})
}

// notesHref builds the reading page's URL for a vault-relative path,
// percent-escaping each path segment individually (so a literal "/" in
// the escaped output can never be mistaken for a path separator) while
// leaving "/" itself as the separator between segments — matching how
// GET /notes/{path...} expects to receive it back.
func notesHref(p string) string {
	segments := strings.Split(p, "/")
	for i, s := range segments {
		segments[i] = url.PathEscape(s)
	}
	return "/notes/" + strings.Join(segments, "/")
}

// renderWikilink renders a plain (non-embed) [[target|display]]. The
// output is always a single short open/close tag pair around escaped
// text, so — unlike an embed's output — it is safe to splice directly
// into the line instead of going through the block-placeholder
// indirection.
func (r *Renderer) renderWikilink(target, display string, diags *[]Diagnostic) string {
	res := r.idx.Resolve(target)
	switch res.Kind {
	case graph.Unique:
		//nolint:gocritic // sprintfQuotedString false positive: the quotes are HTML attribute syntax, not Go string quoting; notesHref already percent-escapes the path
		return fmt.Sprintf(`<a href="%s" class="wikilink">%s</a>`, notesHref(res.Path), html.EscapeString(display))
	case graph.Ambiguous:
		*diags = append(*diags, Diagnostic{
			Kind: DiagWikilinkAmbiguous, Target: target,
			Message: fmt.Sprintf("wikilink %q is ambiguous: %s", target, strings.Join(res.Candidates, ", ")),
		})
		//nolint:gocritic // sprintfQuotedString false positive: the quotes are HTML attribute syntax, not Go string quoting; the value is already html.EscapeString'd
		return fmt.Sprintf(`<span class="wikilink-ambiguous" title="%s">%s</span>`,
			html.EscapeString(strings.Join(res.Candidates, ", ")), html.EscapeString(display))
	case graph.Unresolved:
		*diags = append(*diags, Diagnostic{
			Kind: DiagWikilinkBroken, Target: target,
			Message: fmt.Sprintf("wikilink %q does not resolve to any note or file", target),
		})
		return fmt.Sprintf(`<span class="wikilink-broken">%s</span>`, html.EscapeString(display))
	default:
		panic(fmt.Sprintf("render: unknown graph.Kind %d", res.Kind))
	}
}

// renderEmbed renders ![[target]]. A unique markdown-note target is
// transcluded through the same rendering pipeline (render), wrapped in a
// distinctly-styled container — but only one level deep: when allowEmbed
// is embedsDenied (this call is itself inside an already-transcluded
// note's body), the embed renders as plain wikilink-style text instead of
// expanding again (see embedPolicy's doc comment for why this alone
// prevents runaway/cyclic chains). A unique non-markdown target (image,
// PDF, canvas, ...) gets a placeholder: this repo has no route yet that
// serves raw vault files — wiring one is a real, separate design decision
// (path validation, MIME handling, what's safe to expose even on
// loopback) out of scope here. Ambiguous/unresolved get the same
// diagnostic-styled treatment as a broken wikilink.
func (r *Renderer) renderEmbed(target string, allowEmbed embedPolicy, diags *[]Diagnostic) string {
	if allowEmbed == embedsDenied {
		return r.renderWikilink(target, target, diags)
	}

	res := r.idx.Resolve(target)
	switch res.Kind {
	case graph.Unresolved:
		*diags = append(*diags, Diagnostic{
			Kind: DiagWikilinkBroken, Target: target,
			Message: fmt.Sprintf("embed target %q does not resolve", target),
		})
		return fmt.Sprintf(`<span class="wikilink-broken">![[%s]]</span>`, html.EscapeString(target))
	case graph.Ambiguous:
		*diags = append(*diags, Diagnostic{
			Kind: DiagWikilinkAmbiguous, Target: target,
			Message: fmt.Sprintf("embed target %q is ambiguous: %s", target, strings.Join(res.Candidates, ", ")),
		})
		//nolint:gocritic // sprintfQuotedString false positive: the quotes are HTML attribute syntax, not Go string quoting; the value is already html.EscapeString'd
		return fmt.Sprintf(`<span class="wikilink-ambiguous" title="%s">![[%s]]</span>`,
			html.EscapeString(strings.Join(res.Candidates, ", ")), html.EscapeString(target))
	case graph.Unique:
		if !strings.HasSuffix(res.Path, ".md") {
			return fmt.Sprintf(`<div class="embed-media">［嵌入媒體：<span>%s</span>，尚未支援直接顯示］</div>`,
				html.EscapeString(path.Base(res.Path)))
		}
		n, err := vault.ReadNote(r.root, res.Path)
		if err != nil {
			*diags = append(*diags, Diagnostic{
				Kind: DiagWikilinkBroken, Target: target,
				Message: fmt.Sprintf("embed target %q could not be read: %v", res.Path, err),
			})
			return fmt.Sprintf(`<span class="wikilink-broken">![[%s]]</span>`, html.EscapeString(target))
		}
		inner := r.render(n.Body, embedsDenied)
		*diags = append(*diags, inner.Diagnostics...)
		return `<div class="embed">` + inner.HTML + `</div>`
	default:
		panic(fmt.Sprintf("render: unknown graph.Kind %d", res.Kind))
	}
}
