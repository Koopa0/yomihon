package render

import (
	"fmt"

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
	//nolint:errcheck // the visitor never returns an error, so the walk cannot fail
	_ = ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		link, ok := node.(*ast.Link)
		if !entering || !ok {
			return ast.WalkContinue, nil
		}
		written := string(link.Destination)
		result := graph.ResolveMarkdown(owner, written, func(string) bool { return true }, func(p string) bool { return !r.files.MissingFile(p) })
		if !result.Local || result.Invalid || result.Withheld {
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
		if !result.Checkable {
			return ast.WalkContinue, nil
		}
		reason := fmt.Sprintf(wording.UnwrittenFileFmt.In(col.page.lang), written)
		message := fmt.Sprintf("Markdown path %q resolves to no file", written)
		if result.Outside {
			reason = fmt.Sprintf(wording.MarkdownOutsideFmt.In(col.page.lang), written)
			message = fmt.Sprintf("Markdown path %q leaves the vault", written)
		}
		col.report(&Diagnostic{Kind: DiagMarkdownBroken, Target: written, Message: message})
		link.SetAttributeString(markdownFaultAttr, markdownFault{reason: reason, class: "wikilink-broken"})
		return ast.WalkContinue, nil
	})
}
