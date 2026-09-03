package judge

import (
	"html"
	"regexp"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/vault"
)

// The fragment rules judge the half of a link after its "#": a name that
// resolves to a real note can still address a section or a block the note
// does not have, and the reading page already tells its reader so when it
// renders the link. These rules say the same thing on the agent surface, so
// the two faces cannot disagree about one vault.
//
// The matching reproduces how that page answers a fragment — the fold both
// fragment kinds share, the deliberately generous second reading of section
// names, and the one level of transclusion the page really expands — and the
// golden fixtures pin the agreement. Where the two faces could read a corner
// differently, this one takes the quieter reading: a fragment warning the
// page would not raise tells the author their page is broken while the page
// in front of them looks fine.
//
// A transclusion's fragment is deliberately out of scope here. The page does
// validate it, but it degrades differently — the excerpt widens and says so
// in the article — so its finding is its own rule when it lands, not a link
// finding wearing the wrong name.

// foldAddress folds an address the way both fragment kinds fold on the
// reading page: Unicode form and letter case, and nothing else. A name
// written decomposed by the filesystem and composed by an editor, or typed in
// different capitals than the destination wrote, is one name; every other
// difference is one the author chose.
func foldAddress(s string) string {
	return strings.ToLower(vault.NormalizeNFC(s))
}

// sectionDrop matches every run of characters a heading id drops: anything
// that is not a Unicode letter or digit collapses to a single hyphen.
var sectionDrop = regexp.MustCompile(`[^\p{L}\p{N}]+`)

// sectionSlug reduces a section name to the id the destination page stamps
// for a heading: fold, keep letters and digits, collapse every other run to
// one hyphen, trim the ends, and fall back to "section" when nothing is
// left. It is the same reduction on both sides of the comparison, so what a
// link writes and what a heading answers to cannot drift apart.
func sectionSlug(s string) string {
	s = strings.Trim(sectionDrop.ReplaceAllString(foldAddress(s), "-"), "-")
	if s == "" {
		return "section"
	}
	return s
}

var (
	// wikilinkInHeading matches a wikilink or transclusion written inside a
	// heading's text, whose display words are what the rendered heading shows.
	wikilinkInHeading = regexp.MustCompile(`!?\[\[[^\[\]]+\]\]`)
	// rubyAnnotation matches a ruby reading or its fallback parentheses, which
	// the rendered heading carries beside its base text and its id drops.
	rubyAnnotation = regexp.MustCompile(`(?s)<rt[^>]*>.*?</rt>|<rp[^>]*>.*?</rp>`)
	// markupTag matches any remaining tag, dropped alone so its content stays.
	markupTag = regexp.MustCompile(`<[^>]+>`)
)

// headingWords reduces a heading's source text to the words its rendered
// form shows, which are the words the page folds an anchor id from: a
// wikilink contributes what it displays, a ruby annotation's reading drops
// out with its tags, every other tag drops alone, and character references
// resolve to the characters they name.
func headingWords(raw string) string {
	displayed := wikilinkInHeading.ReplaceAllStringFunc(raw, func(token string) string {
		inner := strings.TrimPrefix(token, "!")
		_, display, _ := graph.SplitWikilink(inner[2 : len(inner)-2])
		return display
	})
	displayed = rubyAnnotation.ReplaceAllString(displayed, "")
	return html.UnescapeString(markupTag.ReplaceAllString(displayed, ""))
}

// anchorSurface reads one body into what its page answers a link fragment
// with: the set of section ids a reader could be sent to, and the folded
// lines that could carry a "^name" block address. Obsidian comments come off
// first, the way the page strips them before it looks, because a heading or
// an address hidden in a comment is not on the page a reader arrives at.
func anchorSurface(body string) (sections map[string]bool, blockLines []string) {
	stripped := withoutCommentZones(body)
	sections = make(map[string]bool)
	collectParsedHeadings(stripped, sections)
	collectGenerousHeadings(stripped, sections)
	return sections, collectBlockLines(stripped)
}

// withoutCommentZones is the body with its comment spans cut out, located by
// the same zones the link extraction skips, so the two readings of one note
// hide the same text.
func withoutCommentZones(body string) string {
	codeZones, _ := structure(body)
	zones := commentZones(body, codeZones)
	if len(zones) == 0 {
		return body
	}
	var b strings.Builder
	last := 0
	for _, z := range zones {
		b.WriteString(body[last:z.start])
		last = z.stop
	}
	b.WriteString(body[last:])
	return b.String()
}

// collectParsedHeadings adds the id of every heading the markdown parser
// sees: either heading form, at any quote or list nesting, and never a
// heading-shaped line inside code or an authored HTML block. This is how the
// destination page really stamps its ids, since it renders the same tree. A
// heading with no text still stamps the fallback id, so it is added too.
func collectParsedHeadings(body string, into map[string]bool) {
	src := []byte(body)
	doc := mdParser.Parse(text.NewReader(src))
	walkNodes(doc, func(n ast.Node) {
		h, ok := n.(*ast.Heading)
		if !ok {
			return
		}
		raw := ""
		if r, ok := linesRange(h); ok {
			raw = body[r.start:r.stop]
		}
		into[sectionSlug(headingWords(raw))] = true
	})
}

// The line shapes the generous scan strips or reads. They are the reading
// page's own patterns for the same scan, kept literal here so the two faces
// read one line the same way.
var (
	atxHeadingText   = regexp.MustCompile(`^ {0,3}#{1,6}[ \t]+(.*)$`)
	setextUnderline  = regexp.MustCompile(`^ {0,3}(=+|-+)[ \t]*$`)
	quotedLinePrefix = regexp.MustCompile(`^ {0,3}>`)
	listItemPrefix   = regexp.MustCompile(`^ {0,3}(?:[-*+]|\d{1,9}[.)])(?:[ \t]|$)`)
)

// collectGenerousHeadings adds what a deliberately generous line reading
// accepts as a heading: quote markers and one list marker stripped, both
// heading forms read, and no fence or HTML-block tracking at all. The
// reading page runs this same scan before it claims a section is missing,
// because its ids are stamped over rendered output that keeps headings
// inside quotes and list items; erring toward finding one costs a silent
// miss, while erring the other way reports a section the reader can see.
// Mirroring the generosity keeps this face quiet wherever that page is.
func collectGenerousHeadings(body string, into map[string]bool) {
	var paragraph []string
	for line := range strings.SplitSeq(body, "\n") {
		candidate := withoutQuoteAndListMarks(line)
		if m := atxHeadingText.FindStringSubmatch(candidate); m != nil {
			into[sectionSlug(headingWords(m[1]))] = true
			paragraph = nil
			continue
		}
		if len(paragraph) > 0 && setextUnderline.MatchString(candidate) {
			into[sectionSlug(headingWords(strings.Join(paragraph, "\n")))] = true
			paragraph = nil
			continue
		}
		if candidate == "" {
			paragraph = nil
			continue
		}
		paragraph = append(paragraph, candidate)
	}
}

// withoutQuoteAndListMarks reduces a line to the text a heading could have
// been written as, removing however many quote markers are nested around it
// and one list marker.
func withoutQuoteAndListMarks(line string) string {
	candidate := strings.TrimSpace(line)
	for quotedLinePrefix.MatchString(candidate) {
		candidate = strings.TrimSpace(strings.TrimPrefix(candidate, ">"))
	}
	if mark := listItemPrefix.FindString(candidate); mark != "" {
		candidate = strings.TrimSpace(candidate[len(mark):])
	}
	return candidate
}

// oneQuoteMark matches the single leading quote marker the block scan peels
// before asking whether a line opens or closes a fence, because a fence
// written inside a callout is read as a fence when that body renders.
var oneQuoteMark = regexp.MustCompile(`^\s*>\s?`)

// collectBlockLines keeps the folded text of every line that could answer a
// block address, trailing whitespace off, so a link's "^name" can be matched
// against the exact reading the destination page uses. A line inside a
// fenced code block is code rather than an address; a row that opens with a
// pipe is table syntax whose tail the renderer drops, so no address written
// there survives onto the page. Only lines carrying a caret are kept, since
// no other line can end in an address.
func collectBlockLines(body string) []string {
	var out []string
	inFence, fenceByte := false, byte(0)
	for line := range strings.SplitSeq(body, "\n") {
		unquoted := oneQuoteMark.ReplaceAllString(line, "")
		if inFence {
			if blockFenceCloses(unquoted, fenceByte) {
				inFence = false
			}
			continue
		}
		if marker, ok := blockFenceOpens(unquoted); ok {
			inFence, fenceByte = true, marker
			continue
		}
		if strings.HasPrefix(strings.TrimLeft(unquoted, " \t"), "|") {
			continue
		}
		trimmed := strings.TrimRight(line, " \t")
		if !strings.Contains(trimmed, "^") {
			continue
		}
		out = append(out, foldAddress(trimmed))
	}
	return out
}

// blockFenceOpens reports whether a line opens a fenced code block, and with
// which marker byte.
func blockFenceOpens(line string) (byte, bool) {
	t := strings.TrimLeft(line, " \t")
	switch {
	case strings.HasPrefix(t, "```"):
		return '`', true
	case strings.HasPrefix(t, "~~~"):
		return '~', true
	default:
		return 0, false
	}
}

// blockFenceCloses reports whether a line closes the open fence: trimmed, at
// least three characters, all of them the fence marker.
func blockFenceCloses(line string, marker byte) bool {
	t := strings.TrimSpace(line)
	return len(t) >= 3 && strings.Count(t, string(marker)) == len(t)
}

// blockAddressed reports whether any collected line answers the folded
// address: the line is the address alone, or ends with it after a space or a
// tab. The suffix reading is the destination page's own, which is what lets
// an address written in the "#^" spelling keep interior spaces.
func blockAddressed(lines []string, want string) bool {
	for _, line := range lines {
		if line == want || strings.HasSuffix(line, " "+want) || strings.HasSuffix(line, "\t"+want) {
			return true
		}
	}
	return false
}

// fragmentFindings judges the fragment half of every plain link whose name
// resolved to exactly one markdown note. A name that resolved to nothing or
// to several files already has its own finding, and adding a fragment
// verdict on top would report one repair twice; a picture or any other
// non-note has no sections or blocks to address; and a bare same-file
// fragment never reaches here, because the page renders it as plain text and
// resolves nothing.
func fragmentFindings(notes []note, idx *graph.Index) []Finding {
	byPath := make(map[string]*note, len(notes))
	for i := range notes {
		byPath[notes[i].path] = &notes[i]
	}
	var out []Finding
	for i := range notes {
		n := &notes[i]
		for l := range n.wikilinks {
			link := &n.wikilinks[l]
			if f, reported := fragmentFinding(n, link, idx, byPath); reported {
				out = append(out, f)
			}
		}
	}
	return out
}

// fragmentFinding judges one link occurrence, reporting false when its
// fragment places or when the link is out of these rules' scope. A block
// address is judged ahead of a section name when the author wrote both,
// which is the order the destination page resolves that conflict in.
func fragmentFinding(n *note, link *wikiLink, idx *graph.Index, byPath map[string]*note) (Finding, bool) {
	if link.embed || (link.heading == "" && link.block == "") {
		return Finding{}, false
	}
	res := idx.Resolve(link.target)
	if res.Kind != graph.KindUnique || !vault.IsMarkdown(res.RelPath) {
		return Finding{}, false
	}
	target := byPath[res.RelPath]
	if target == nil {
		return Finding{}, false
	}
	if link.block != "" {
		if blockAddressed(target.blockAnchorLines, foldAddress("^"+link.block)) {
			return Finding{}, false
		}
		return blockMissing(n, link, res.RelPath), true
	}
	want := sectionSlug(link.heading)
	if target.sectionAnchors[want] || transclusionBringsSection(target, idx, byPath, want) {
		return Finding{}, false
	}
	return sectionMissing(n, link, res.RelPath), true
}

// transclusionBringsSection reports whether a section absent from a note's
// own body arrives through a note it transcludes. The page expands a
// transclusion one level and stamps ids for the headings it brings, so a
// name found there is a name the destination answers to, and the walk stops
// at that one level exactly where the expansion does. The transclusion's own
// fragment narrows what the page shows, and this reading deliberately skips
// that narrowing: it can only keep quiet about a miss the page would report,
// never claim one the page would not.
func transclusionBringsSection(target *note, idx *graph.Index, byPath map[string]*note, want string) bool {
	for _, link := range target.wikilinks {
		if !link.embed {
			continue
		}
		res := idx.Resolve(link.target)
		if res.Kind != graph.KindUnique || !vault.IsMarkdown(res.RelPath) {
			continue
		}
		if embedded := byPath[res.RelPath]; embedded != nil && embedded.sectionAnchors[want] {
			return true
		}
	}
	return false
}

// sectionMissing is a link whose note is real and whose section is not: the
// reading page keeps the address as written, so the reader lands at the top
// of the note, and it reports the miss the same way this does.
func sectionMissing(n *note, link *wikiLink, resolved string) Finding {
	return Finding{
		RuleID:          "link.section_missing",
		Severity:        SeverityWarn,
		Path:            n.path,
		Line:            new(link.line),
		Message:         "[[" + link.address + "]] resolves, but no heading matches \"" + link.heading + "\"",
		Evidence:        "the note exists and none of its headings answers the section name",
		SuggestedAction: "fix the section name after #, or add the heading to the target note",
		SourceRule:      sourceYomihon,
		Target:          new(link.address),
		ResolvedTo:      new(resolved),
		Fingerprint:     fingerprint("link.section_missing", n.path, link.address),
	}
}

// blockMissing is a link whose note is real and whose block address names no
// line: the reading page withdraws the address, so the link leads to the
// whole note, and it reports the miss the same way this does.
func blockMissing(n *note, link *wikiLink, resolved string) Finding {
	return Finding{
		RuleID:          "link.block_missing",
		Severity:        SeverityWarn,
		Path:            n.path,
		Line:            new(link.line),
		Message:         "[[" + link.address + "]] resolves, but no line carries the address ^" + link.block,
		Evidence:        "the note exists and no line in it ends with the block address",
		SuggestedAction: "fix the block name after ^, or write the address at the end of the intended line",
		SourceRule:      sourceYomihon,
		Target:          new(link.address),
		ResolvedTo:      new(resolved),
		Fingerprint:     fingerprint("link.block_missing", n.path, link.address),
	}
}
