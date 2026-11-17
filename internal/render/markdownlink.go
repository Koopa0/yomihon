package render

import (
	"fmt"
	"net/url"
	"strings"

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
	// href is set when the file was found and only its fragment was not: the
	// link stays an anchor, because it still leads somewhere real.
	href string
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
		if strings.HasPrefix(written, "#") {
			r.resolveOwnFragment(link, written, col)
			return ast.WalkContinue, nil
		}
		result := graph.ResolveMarkdown(owner, written, func(p string) bool { return !r.files.OutsideScan(p) }, func(p string) bool { return !r.files.MissingFile(p) })
		if result.Withheld {
			link.SetAttributeString(markdownUnlinkedAttr, true)
			return ast.WalkContinue, nil
		}
		if !result.Local || result.Invalid {
			return ast.WalkContinue, nil
		}
		if result.Kind == graph.KindUnique {
			r.placeFile(link, &result, col)
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

// placeFile points a link at the route that serves the file it names, and a
// note's fragment at the heading it names.
func (r *Pipeline) placeFile(link *ast.Link, result *graph.MarkdownResolution, col *collector) {
	href := rawHref(result.RelPath)
	switch {
	case r.files.IsDirectory(result.RelPath):
		href = foldersHref(result.RelPath)
	case vault.IsMarkdown(result.RelPath):
		href = notesHref(result.RelPath)
	}
	link.Destination = []byte(href + result.Suffix)
	if vault.IsMarkdown(result.RelPath) && !r.files.IsDirectory(result.RelPath) {
		query, fragment, _ := strings.Cut(result.Suffix, "#")
		r.placeFragment(link, result.RelPath, result.RelPath, href, query, fragment, col)
	}
}

// resolveOwnFragment places a link whose address is only a fragment, which names
// a place in the note the link is written in.
func (r *Pipeline) resolveOwnFragment(link *ast.Link, written string, col *collector) {
	if !vault.IsMarkdown(col.relPath) {
		return
	}
	href := ""
	if !col.onPage {
		href = notesHref(col.relPath)
	}
	r.placeFragment(link, col.relPath, "", href, "", written[1:], col)
}

// markdownAddress reads the fragment of a Markdown link as the place it names
// in a note: a section, or a block when it opens with "^". ok is false for a
// fragment that names none, which is empty or "top", the top of the page.
func markdownAddress(fragment string) (address graph.Wikilink, ok bool) {
	if fragment == "" || strings.EqualFold(fragment, "top") {
		return address, false
	}
	if decoded, err := url.PathUnescape(fragment); err == nil {
		fragment = decoded
	}
	address.Heading, address.Block = graph.ParseFragment(fragment)
	return address, address.Heading != "" || address.Block != ""
}

// FragmentGap is what a Markdown link's fragment fails to find in the note at
// relPath, which is the finding the reading page reports for it, or nil when
// it lands. The page and the check ask the one lookup, so the check cannot say
// a link is missing that the page follows.
func (r *Pipeline) FragmentGap(relPath, fragment string) *Diagnostic {
	address, ok := markdownAddress(fragment)
	if !ok {
		return nil
	}
	address.Target = relPath
	col := &collector{page: &composition{lang: wording.En}, relPath: relPath}
	if _, miss := r.sectionHref(relPath, address, col); miss == fragmentPlaced || len(col.diags) == 0 {
		return nil
	}
	return &col.diags[0]
}

// placeFragment points a Markdown link's fragment at the id the destination
// stamps, by the lookup a wikilink's fragment takes, so an address written as a
// heading's words, or the way GitHub spells it, lands. One that reaches nothing
// keeps the address as written and shows why, as a wikilink does.
func (r *Pipeline) placeFragment(link *ast.Link, relPath, target, href, query, fragment string, col *collector) {
	address, ok := markdownAddress(fragment)
	if !ok {
		return
	}
	address.Target = target
	placed, miss := r.sectionHref(relPath, address, col)
	switch miss {
	case fragmentPlaced:
		if _, id, found := strings.Cut(placed, "#"); found {
			link.Destination = []byte(href + query + "#" + id)
		}
	case fragmentSectionMissing, fragmentBlockMissing:
		reason := wording.BlockNotFound.In(col.page.lang)
		if miss == fragmentSectionMissing {
			reason = fmt.Sprintf(wording.SectionNotFoundFmt.In(col.page.lang), address.Heading)
		}
		link.SetAttributeString(markdownFaultAttr, markdownFault{reason: reason, class: "wikilink wikilink-degraded", href: string(link.Destination)})
	}
}
