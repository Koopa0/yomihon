package judge

import (
	"html"
	"regexp"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/sequence"
	"github.com/koopa0/yomihon/internal/vault"
)

// The fragment rules judge the half of a link after its "#": a name that
// resolves to a real note can still address a section or a block the note does
// not have. The matching reproduces how the reading page answers a fragment,
// and the golden fixtures pin the agreement; where the two faces could read a
// corner differently, this one takes the quieter reading, since a warning the
// page would not raise tells the author their page is broken while it looks
// fine in front of them. A transclusion's fragment is judged under its own two
// rules, because the page reads it more strictly than a link's: the excerpt is
// cut by one exact line scan over the embedded note's own body, with no
// generous second look and no heading arriving through a further transclusion,
// and a name that scan cannot find is reported as an excerpt the page could
// not cut.

// foldAddress folds an address the way both fragment kinds fold on the reading
// page: Unicode form and letter case, and nothing else. Every other difference
// is one the author chose.
func foldAddress(s string) string {
	return strings.ToLower(vault.NormalizeNFC(s))
}

// sectionDrop matches every run of characters a heading id drops: anything
// that is not a Unicode letter or digit collapses to a single hyphen.
var sectionDrop = regexp.MustCompile(`[^\p{L}\p{N}]+`)

// sectionSlug reduces a section name to the id the destination page stamps for
// a heading: fold, keep letters and digits, collapse every other run to one
// hyphen, trim the ends, and fall back to "section" when nothing is left. Both
// sides of the comparison run it, so a link and a heading cannot drift apart.
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

// anchorSurface reads one body into what its page answers a fragment with:
// the set of section ids a link could be sent to, the set the excerpt scan
// cuts a transclusion to, and the folded lines that could carry a "^name"
// block address. Obsidian comments come off first, the way the page strips
// them before it looks, because a heading or an address hidden in a comment
// is not on the page a reader arrives at. A study path's branch is named the
// way the page names it too: the role it declares at the end of its heading is
// grammar the course parser consumes, so the id is stamped from the words
// without it, and a citation reaches the branch by the name a reader sees.
func anchorSurface(body string) (sections, excerptSections map[string]bool, blockLines []string) {
	stripped := withoutCommentZones(body)
	sections = make(map[string]bool)
	collectParsedHeadings(stripped, sections)
	collectGenerousHeadings(stripped, sections)
	excerptSections = make(map[string]bool)
	collectExcerptHeadings(stripped, excerptSections)
	return sections, excerptSections, collectBlockLines(stripped)
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
		into[sectionSlug(headingWords(sequence.HeadingName(raw, h.Level)))] = true
	})
}

// The line shapes the generous scan strips or reads. They are the reading
// page's own patterns for the same scan, kept literal here so the two faces
// read one line the same way.
var (
	atxHeadingText   = regexp.MustCompile(`^ {0,3}(#{1,6})[ \t]+(.*?)(?:[ \t]+#+)?[ \t]*$`)
	setextUnderline  = regexp.MustCompile(`^ {0,3}(=+|-+)[ \t]*$`)
	quotedLinePrefix = regexp.MustCompile(`^ {0,3}>`)
	listItemPrefix   = regexp.MustCompile(`^ {0,3}(?:[-*+]|\d{1,9}[.)])(?:[ \t]|$)`)
)

// setextLevel is the level an underline makes, for a line the caller has
// already recognized as one: '=' underlines a level-1 heading, '-' a level-2
// one. A course branch is a heading from level 2 to 6, so the two readings part
// company over a declaration written on an underlined title.
func setextLevel(line string) int {
	if strings.HasPrefix(strings.TrimSpace(line), "=") {
		return 1
	}
	return 2
}

// collectGenerousHeadings adds what a deliberately generous line reading
// accepts as a heading: quote markers and one list marker stripped, both
// heading forms read, no fence or HTML-block tracking. The reading page runs
// the same scan before claiming a section is missing, so mirroring its
// generosity keeps this face quiet wherever that page is.
func collectGenerousHeadings(body string, into map[string]bool) {
	var paragraph []string
	for line := range strings.SplitSeq(body, "\n") {
		candidate := withoutQuoteAndListMarks(line)
		if m := atxHeadingText.FindStringSubmatch(candidate); m != nil {
			into[sectionSlug(headingWords(sequence.HeadingName(m[2], len(m[1]))))] = true
			paragraph = nil
			continue
		}
		if len(paragraph) > 0 && setextUnderline.MatchString(candidate) {
			name := sequence.HeadingName(strings.Join(paragraph, "\n"), setextLevel(candidate))
			into[sectionSlug(headingWords(name))] = true
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

// The line shapes that are not running prose, and therefore cannot be the
// text an underline turns into a heading, beyond the quote and list shapes
// above: a break rule, and an indented code line. They are the reading page's
// own patterns for its excerpt scan, kept literal here for the same reason
// the generous scan's are.
var (
	breakRuleLine    = regexp.MustCompile(`^ {0,3}((\*[ \t]*){3,}|(_[ \t]*){3,}|(-[ \t]*){3,})$`)
	indentedCodeLine = regexp.MustCompile(`^ {4,}\S`)
)

// The HTML block start conditions of the CommonMark spec that a line scan can
// recognise without paragraph state: every one but the bare complete tag alone
// on its line, which cannot interrupt a paragraph and so needs state this scan
// does not keep. A block opened by any of these hands its lines to the reader
// as written, so a heading-shaped line inside one is text rather than a
// section boundary; the excerpt scan on the reading page skips the same lines.
var (
	htmlBlockRawText = regexp.MustCompile(`(?i)^ {0,3}<(script|pre|style|textarea)([ \t>]|$)`)
	htmlBlockRawEnd  = regexp.MustCompile(`(?i)</(script|pre|style|textarea)>`)
	htmlBlockComment = regexp.MustCompile(`^ {0,3}<!--`)
	htmlBlockInstr   = regexp.MustCompile(`^ {0,3}<\?`)
	htmlBlockDecl    = regexp.MustCompile(`^ {0,3}<![A-Za-z]`)
	htmlBlockCDATA   = regexp.MustCompile(`^ {0,3}<!\[CDATA\[`)
	htmlBlockElement = regexp.MustCompile(`(?i)^ {0,3}</?(address|article|aside|base|basefont|blockquote|body|caption|center|col|colgroup|dd|details|dialog|dir|div|dl|dt|fieldset|figcaption|figure|footer|form|frame|frameset|h1|h2|h3|h4|h5|h6|head|header|hr|html|iframe|legend|li|link|main|menu|menuitem|nav|noframes|ol|optgroup|option|p|param|search|section|summary|table|tbody|td|tfoot|th|thead|title|tr|track|ul)([ \t]|/?>|$)`)
)

// htmlBlockOpens reports whether a line opens an authored HTML block, and
// returns the test for the line that closes it. The raw-text, comment,
// instruction, declaration, and CDATA blocks close on their own end marker,
// which may sit on the opening line itself; an element block runs to the next
// blank line.
func htmlBlockOpens(line string) (closes func(string) bool, ok bool) {
	switch {
	case htmlBlockRawText.MatchString(line):
		return htmlBlockRawEnd.MatchString, true
	case htmlBlockComment.MatchString(line):
		return lineContaining("-->"), true
	case htmlBlockInstr.MatchString(line):
		return lineContaining("?>"), true
	case htmlBlockCDATA.MatchString(line):
		return lineContaining("]]>"), true
	case htmlBlockDecl.MatchString(line):
		return lineContaining(">"), true
	case htmlBlockElement.MatchString(line):
		return func(line string) bool { return strings.TrimSpace(line) == "" }, true
	}
	return nil, false
}

// lineContaining is the closing test of a block that ends on a marker.
func lineContaining(marker string) func(string) bool {
	return func(line string) bool { return strings.Contains(line, marker) }
}

// excerptScan carries the running state the excerpt scan needs to tell a
// heading from a heading-shaped line inside fenced code or an authored HTML
// block, whose contents reach the reader as written. The zero value starts a
// scan.
type excerptScan struct {
	inFence    bool
	fenceByte  byte
	htmlCloses func(string) bool
}

// skips advances the scan by one line and reports whether that line belongs
// to a fenced code block or an authored HTML block, the lines that open and
// close one included.
func (s *excerptScan) skips(line string) bool {
	switch {
	case s.inFence:
		if blockFenceCloses(line, s.fenceByte) {
			s.inFence = false
		}
		return true
	case s.htmlCloses != nil:
		if s.htmlCloses(line) {
			s.htmlCloses = nil
		}
		return true
	}
	if marker, ok := blockFenceOpens(line); ok {
		s.inFence, s.fenceByte = true, marker
		return true
	}
	if closes, ok := htmlBlockOpens(line); ok {
		if !closes(line) {
			s.htmlCloses = closes
		}
		return true
	}
	return false
}

// collectExcerptHeadings adds the id of every heading the reading page's
// excerpt scan finds when it cuts a transclusion to a section: a '#'-marked
// heading at up to three spaces of indent, an underlined one made of the run
// of prose above it, and nothing inside fenced code or an authored HTML block.
// A heading inside a quote or a list item is not cut to, because the scan
// strips neither marker. An underline only makes a heading of running prose:
// a blank line, a quote, a list item, a break rule, another underline, or an
// indented code line opening the run ends what it could claim.
func collectExcerptHeadings(body string, into map[string]bool) {
	var scan excerptScan
	paragraph := -1
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		if scan.skips(line) {
			paragraph = -1
			continue
		}
		if m := atxHeadingText.FindStringSubmatch(line); m != nil {
			into[sectionSlug(headingWords(sequence.HeadingName(m[2], len(m[1]))))] = true
			paragraph = -1
			continue
		}
		switch {
		case paragraph >= 0 && setextUnderline.MatchString(line):
			name := sequence.HeadingName(strings.Join(lines[paragraph:i], "\n"), setextLevel(line))
			into[sectionSlug(headingWords(name))] = true
			paragraph = -1
		case strings.TrimSpace(line) == "", quotedLinePrefix.MatchString(line), listItemPrefix.MatchString(line),
			breakRuleLine.MatchString(line), setextUnderline.MatchString(line),
			paragraph < 0 && indentedCodeLine.MatchString(line):
			paragraph = -1
		case paragraph < 0:
			paragraph = i
		}
	}
}

// oneQuoteMark matches the single leading quote marker the block scan peels
// before asking whether a line opens or closes a fence, because a fence
// written inside a callout is read as a fence when that body renders.
var oneQuoteMark = regexp.MustCompile(`^\s*>\s?`)

// collectBlockLines keeps the folded text of every line that could answer a
// block address, so a link's "^name" matches the reading the destination page
// uses. A line inside a fence is code, and a row opening with a pipe is table
// syntax whose tail the renderer drops. Only lines carrying a caret are kept.
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

// fragmentFindings judges the fragment half of every link and transclusion
// whose name resolved to exactly one markdown note. A name that resolved to
// nothing or to several files already carries its own finding, and a non-note
// has no sections or blocks to address. A bare same-file fragment never
// reaches here.
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

// fragmentFinding judges one occurrence, reporting false when its fragment
// places or when the occurrence is out of these rules' scope. A block address
// is judged ahead of a section name when the author wrote both, which is the
// order the destination page resolves that conflict in, and both kinds answer
// a block by the one scan that stamps block anchors. A section is answered
// two ways: a transclusion's by the excerpt scan alone, since that is all the
// page cuts with, and a link's by the wider reading the page gives an address
// it only has to land somewhere on.
func fragmentFinding(n *note, link *wikiLink, idx *graph.Index, byPath map[string]*note) (Finding, bool) {
	if link.heading == "" && link.block == "" {
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
	if link.embed {
		if target.excerptSectionAnchors[want] {
			return Finding{}, false
		}
		return sectionMissing(n, link, res.RelPath), true
	}
	if target.sectionAnchors[want] || transclusionBringsSection(target, idx, byPath, want) {
		return Finding{}, false
	}
	return sectionMissing(n, link, res.RelPath), true
}

// transclusionBringsSection reports whether a section absent from a note's own
// body arrives through a note it transcludes. The page expands a transclusion
// one level, so the walk stops at that level too. It skips the narrowing the
// transclusion's own fragment applies, which can only keep it quieter than the
// page, never louder.
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

// sectionMissing is a link or a transclusion whose note is real and whose
// section is not. The reading page keeps a link's address as written, so the
// reader lands at the top of the note; for a transclusion it has no excerpt
// to cut and says so where the words would have been. Each is its own rule,
// so a baseline that has accepted one cannot swallow the other, and the
// fingerprint keys on the rule with the address for the same reason.
func sectionMissing(n *note, link *wikiLink, resolved string) Finding {
	rule, written := "link.section_missing", "[["
	evidence := "the note exists and none of its headings answers the section name"
	action := "fix the section name after #, or add the heading to the target note"
	if link.embed {
		rule, written = "embed.section_missing", "![["
		evidence = "the note exists and none of its headings answers the section name, so there is no excerpt to cut"
		action = "fix the section name after #, or add the heading to the embedded note"
	}
	return Finding{
		RuleID:          rule,
		Severity:        SeverityWarn,
		Path:            n.path,
		Line:            new(link.line),
		Message:         written + link.address + "]] resolves, but no heading matches \"" + link.heading + "\"",
		Evidence:        evidence,
		SuggestedAction: action,
		SourceRule:      sourceYomihon,
		Target:          new(link.address),
		ResolvedTo:      new(resolved),
		Fingerprint:     fingerprint(rule, n.path, link.address),
	}
}

// blockMissing is a link or a transclusion whose note is real and whose block
// address names no line. The reading page withdraws a link's address, so the
// link leads to the whole note; for a transclusion it has no excerpt to cut
// and says so where the words would have been. The two rules are kept apart
// the way the section rules are.
func blockMissing(n *note, link *wikiLink, resolved string) Finding {
	rule, written := "link.block_missing", "[["
	evidence := "the note exists and no line in it ends with the block address"
	action := "fix the block name after ^, or write the address at the end of the intended line"
	if link.embed {
		rule, written = "embed.block_missing", "![["
		evidence = "the note exists and no line in it ends with the block address, so there is no excerpt to cut"
		action = "fix the block name after ^, or write the address at the end of the line the excerpt should show"
	}
	return Finding{
		RuleID:          rule,
		Severity:        SeverityWarn,
		Path:            n.path,
		Line:            new(link.line),
		Message:         written + link.address + "]] resolves, but no line carries the address ^" + link.block,
		Evidence:        evidence,
		SuggestedAction: action,
		SourceRule:      sourceYomihon,
		Target:          new(link.address),
		ResolvedTo:      new(resolved),
		Fingerprint:     fingerprint(rule, n.path, link.address),
	}
}
