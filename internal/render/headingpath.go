package render

import (
	"strings"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/wording"
)

// IsHeadingPath reports whether a fragment carries two or more nonempty
// heading names. Empty segments leave a hash as literal heading text, so a
// heading ending in a hash keeps the single-name reading it already had.
func IsHeadingPath(heading string) bool {
	if !strings.Contains(heading, "#") {
		return false
	}
	for part := range strings.SplitSeq(heading, "#") {
		if strings.TrimSpace(part) == "" {
			return false
		}
	}
	return true
}

// HeadingPath resolves an ordered ancestry path against the headings the
// captured destination actually displays, including its one-level embeds.
// The result is the assigned leaf id, so repeated names keep their suffixes.
// Links within the destination are displayed without checking their fragments:
// following them would recurse through mutually citing notes. No vault is read.
func (r *Pipeline) HeadingPath(relPath, heading string) (id string, found bool) {
	headings, ok := r.capturedOutline(relPath)
	if !ok {
		return "", false
	}
	return leafID(headings, heading)
}

// capturedOutline is every heading the captured destination displays. A
// generation's captured bodies never change, so one note's outline is drawn
// once however many links in a page or a check name a path into it.
func (r *Pipeline) capturedOutline(relPath string) ([]TOCEntry, bool) {
	if known, ok := r.outlines.Load(relPath); ok {
		if headings, ok := known.([]TOCEntry); ok {
			return headings, true
		}
	}
	body, ok := r.transclusions.Transclusion(relPath)
	if !ok {
		return nil, false
	}
	headings := r.outline(relPath, body)
	r.outlines.Store(relPath, headings)
	return headings, true
}

// hostOutline is the outline of the note this page is reading, drawn from the
// body being rendered rather than any captured version of it, once per render.
func (r *Pipeline) hostOutline(col *collector) []TOCEntry {
	if col.page.hostOutline == nil {
		headings := r.outline(col.relPath, col.body)
		col.page.hostOutline = &headings
	}
	return *col.page.hostOutline
}

// outline renders body as relPath's own page would and lists every heading it
// stamps, in document order.
func (r *Pipeline) outline(relPath, body string) []TOCEntry {
	stripped, _ := stripBody(body)
	page := &composition{lang: wording.ZhHant, headingLookup: true}
	res := r.renderBody(bodyInput{path: relPath, text: stripped.text, address: stripped.address}, embedsAllowed, page, hostRegion)
	var headings []TOCEntry
	_, _ = stampHeadings(res.HTML, "", &headings)
	return headings
}

// leafID is the assigned id of the first heading the path names.
func leafID(headings []TOCEntry, heading string) (id string, found bool) {
	matches := matchingHeadingPath(headings, heading)
	if len(matches) == 0 {
		return "", false
	}
	return headings[matches[0]].ID, true
}

// matchingHeadingPath keeps the active ancestry in document order. Requested
// parents can skip intervening levels, but never cross a closed branch or
// change order. A repeated complete path keeps the first, like a single name.
func matchingHeadingPath(headings []TOCEntry, heading string) []int {
	parts := strings.Split(heading, "#")
	for i, part := range parts {
		if strings.TrimSpace(part) == "" {
			return nil
		}
		parts[i] = graph.SectionID(part)
	}
	var ancestors []TOCEntry
	var matches []int
	for i, h := range headings {
		for len(ancestors) > 0 && ancestors[len(ancestors)-1].Level >= h.Level {
			ancestors = ancestors[:len(ancestors)-1]
		}
		if graph.SectionID(h.Text) == parts[len(parts)-1] && pathParentsMatch(ancestors, parts[:len(parts)-1]) {
			matches = append(matches, i)
		}
		ancestors = append(ancestors, h)
	}
	return matches
}

func pathParentsMatch(ancestors []TOCEntry, parts []string) bool {
	next := 0
	for _, h := range ancestors {
		if next < len(parts) && graph.SectionID(h.Text) == parts[next] {
			next++
		}
	}
	return next == len(parts)
}
