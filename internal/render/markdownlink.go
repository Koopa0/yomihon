package render

import (
	"fmt"

	"github.com/yuin/goldmark/ast"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

const markdownFaultAttr = "yomihonMarkdownFault"

// markdownUnlinkedAttr marks a link into a place the scan does not look. The
// page keeps its label and drops the link: no route serves what is there, so
// an anchor would only lead to a not-found page.
const markdownUnlinkedAttr = "yomihonMarkdownUnlinked"

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
		result := graph.ResolveMarkdown(owner, written, func(p string) bool { return !r.files.OutsideScan(p) }, func(p string) bool { return !r.files.MissingFile(p) })
		if result.Withheld {
			link.SetAttributeString(markdownUnlinkedAttr, true)
			return ast.WalkContinue, nil
		}
		if !result.Local || result.Invalid {
			return ast.WalkContinue, nil
		}
		if result.Kind == graph.KindUnique {
			href := rawHref(result.RelPath)
			switch {
			case r.files.IsDirectory(result.RelPath):
				href = foldersHref(result.RelPath)
			case vault.IsMarkdown(result.RelPath):
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
