package render

import (
	"fmt"
	"strings"

	"github.com/yuin/goldmark/ast"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

const markdownFaultAttr = "yomihonMarkdownFault"

type markdownFault struct {
	reason string
	class  string
}

// resolveMarkdownLinks handles parsed Markdown links, never generated links or
// authored HTML. Child nodes stay intact, including formatted or empty labels.
func (r *Pipeline) resolveMarkdownLinks(doc ast.Node, owner string, col *collector) {
	err := ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		link, ok := node.(*ast.Link)
		if !entering || !ok {
			return ast.WalkContinue, nil
		}
		written := string(link.Destination)
		result := r.idx.ResolveMarkdown(owner, written, func(string) bool { return true }, func(string) bool { return true })
		if !result.Local || result.Withheld {
			return ast.WalkContinue, nil
		}
		if result.Kind == graph.KindUnique {
			href := rawHref(result.RelPath)
			if vault.IsMarkdown(result.RelPath) {
				href = notesHref(result.RelPath)
			}
			link.Destination = []byte(href + result.Suffix)
			return ast.WalkContinue, nil
		}
		kind := DiagMarkdownBroken
		className := "wikilink-broken"
		reason := fmt.Sprintf(wording.UnwrittenFileFmt.In(col.page.lang), written)
		message := fmt.Sprintf("Markdown path %q resolves to no file", written)
		if result.Invalid {
			reason = fmt.Sprintf(wording.MarkdownInvalidFmt.In(col.page.lang), written)
			message = fmt.Sprintf("Markdown path %q cannot be parsed as a vault path", written)
		}
		if result.Outside {
			reason = fmt.Sprintf(wording.MarkdownOutsideFmt.In(col.page.lang), written)
			message = fmt.Sprintf("Markdown path %q leaves the vault", written)
		}
		if result.Kind == graph.KindAmbiguous {
			kind = DiagMarkdownAmbiguous
			className = "wikilink-ambiguous"
			reason = fmt.Sprintf(wording.MarkdownSeveralFmt.In(col.page.lang), written, strings.Join(result.Candidates, ", "))
			message = fmt.Sprintf("Markdown path %q names several files: %s", written, strings.Join(result.Candidates, ", "))
		}
		col.report(&Diagnostic{Kind: kind, Target: written, Message: message})
		link.SetAttributeString(markdownFaultAttr, markdownFault{reason: reason, class: className})
		return ast.WalkContinue, nil
	})
	if err != nil {
		panic("render: Markdown link walk returned an unexpected error")
	}
}
