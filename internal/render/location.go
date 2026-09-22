package render

import (
	"fmt"
	"html"
	"regexp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/wording"
)

// SourceLocation is an authored place in a declared source. A missing place
// keeps its label but has no fragment: following it opens the source itself.
type SourceLocation struct {
	Label      string
	Fragment   string
	Reason     string
	Diagnostic *Diagnostic
}

// locatedHeading reads only heading tags stamped by assignHeadingIDs, including
// headings brought by an embed. The destination's rendered bytes own which
// addresses exist; a second Markdown scan could promise an anchor inside code.
var locatedHeading = regexp.MustCompile(`(?s)<h[1-6] id="([^"]*)" data-level="[1-6]">(.*?)</h[1-6]>`)

// DeclaredLocation resolves against the exact rendering the source page uses.
// display is only an explicit alias; without one a heading supplies its own
// words, while a block keeps the author's address.
func DeclaredLocation(result *Result, title string, link graph.Wikilink, display string, lang wording.Lang) SourceLocation {
	id := graph.SectionID(link.Heading)
	label := link.Heading
	found := false
	switch {
	case link.Block != "":
		label = "^" + link.Block
		id = blockAnchorID(label)
		for _, anchor := range anchorAttribute.FindAllStringSubmatch(result.HTML, -1) {
			if html.UnescapeString(anchor[1]) == id {
				found = true
				break
			}
		}
	case result.TitleAnchor == id:
		label, found = title, true
	default:
		for _, heading := range locatedHeading.FindAllStringSubmatch(result.HTML, -1) {
			if html.UnescapeString(heading[1]) == id {
				label, found = headingInnerText(heading[2]), true
				break
			}
		}
	}
	if display != "" {
		label = display
	}
	location := SourceLocation{Label: label}
	if found {
		location.Fragment = id
		return location
	}
	diag := &Diagnostic{Target: link.Target}
	if link.Block != "" {
		diag.Kind, diag.Block = DiagLinkFragmentMissing, link.Block
		location.Reason = wording.BlockNotFound.In(lang)
		diag.Message = fmt.Sprintf("no rendered block matched %q; the link leads to the note itself", "^"+link.Block)
	} else {
		diag.Kind, diag.Section = DiagLinkSectionMissing, link.Heading
		location.Reason = fmt.Sprintf(wording.SectionNotFoundFmt.In(lang), link.Heading)
		diag.Message = fmt.Sprintf("no rendered heading matched %q; the link leads to the note itself", link.Heading)
	}
	location.Diagnostic = diag
	return location
}
