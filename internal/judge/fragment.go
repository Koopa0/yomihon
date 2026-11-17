package judge

import (
	"slices"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
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

// headingWords reduces a heading's source text to the words the page stamps
// an id from. It is the reading page's fold, not a second copy: a heading
// that names a generic type must answer here the same way the contents list
// names it, or this face refuses a link the page follows.
func headingWords(raw string) string {
	return render.HeadingWords(raw)
}

func anchorSurfaceFrom(body string, comments []byteRange) (sections, excerptSections map[string]bool, blockAddresses []string) {
	stripped := withoutCommentZones(body, comments)
	sections = make(map[string]bool)
	collectParsedHeadings(stripped, sections)
	collectGenerousHeadings(stripped, sections)
	excerptSections = make(map[string]bool)
	collectExcerptHeadings(stripped, excerptSections)
	return sections, excerptSections,
		collectBlockAddresses(stripped, render.BlockAddressLines(strings.Split(body, "\n"), stripped))
}

// withoutCommentZones is the body with its comment spans cut out, located by
// the same zones the link extraction skips, so the two readings of one note
// hide the same text. A comment the author wrapped over several lines leaves
// those line endings behind: a note is read by line here and on the page, and
// gluing the words on either side of a hidden passage into one line would make
// a paragraph, a heading and the run a block address sits in out of text nobody
// wrote that way.
func withoutCommentZones(body string, zones []byteRange) string {
	if len(zones) == 0 {
		return body
	}
	var b strings.Builder
	last := 0
	for _, z := range zones {
		b.WriteString(body[last:z.Start])
		b.WriteString(strings.Repeat("\n", strings.Count(body[z.Start:z.Stop], "\n")))
		last = z.Stop
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
			raw = body[r.Start:r.Stop]
		}
		into[graph.SectionID(headingWords(sequence.HeadingName(raw, h.Level)))] = true
	})
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
		if m := graph.ATXHeading.FindStringSubmatch(candidate); m != nil {
			into[graph.SectionID(headingWords(sequence.HeadingName(m[2], len(m[1]))))] = true
			paragraph = nil
			continue
		}
		if len(paragraph) > 0 && graph.SetextUnderline.MatchString(candidate) {
			name := sequence.HeadingName(strings.Join(paragraph, "\n"), graph.SetextLevel(candidate))
			into[graph.SectionID(headingWords(name))] = true
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
	for graph.QuotedLine.MatchString(candidate) {
		candidate = strings.TrimSpace(strings.TrimPrefix(candidate, ">"))
	}
	if mark := graph.ListItemLine.FindString(candidate); mark != "" {
		candidate = strings.TrimSpace(candidate[len(mark):])
	}
	return candidate
}

// collectExcerptHeadings adds the id of every heading the reading page's
// excerpt scan finds when it cuts a transclusion to a section. It is that
// page's own heading grammar, read from where both faces read it, over the
// ranges a line walk hides: a heading-shaped line inside fenced code or an
// authored HTML block is neither face's heading. A heading inside a quote or
// a list item is not cut to, because the grammar strips neither marker.
func collectExcerptHeadings(body string, into map[string]bool) {
	for _, h := range graph.Headings(body, graph.LineSkipZones(body)) {
		into[graph.SectionID(headingWords(sequence.HeadingName(h.Text, h.Level)))] = true
	}
}

// collectBlockAddresses keeps each extracted, folded address that could answer a
// block address, so a link's "^name" matches the reading the destination page
// uses. A line inside a fence is code, a recognised callout's opening line is
// consumed as the title, a row opening with a pipe is table syntax whose tail
// the renderer drops — unless an indented code block shows either as written,
// which the page's own parse answers — and a caret a code span owns is quoted
// text.
// Only supported addresses are kept. address is these same lines carrying
// the blank-or-not shape the author wrote, which is what the code-span question
// is asked over: this face hides a comment with a different scan than the page
// does, and a run edge read from either strip would be a different edge here
// than there.
func collectBlockAddresses(body string, address []string) []string {
	var out []string
	inFence, fenceByte, fenceLen := false, byte(0), 0
	lines := strings.Split(body, "\n")
	owned := render.CodeSpanOwnedAddresses(address)
	unanchorable := render.UnanchorableLines(body)
	for i, line := range lines {
		unquoted := graph.QuotePrefix.ReplaceAllString(line, "")
		if inFence {
			if graph.FenceCloses(unquoted, fenceByte, fenceLen) {
				inFence = false
			}
			continue
		}
		if marker, n, ok := graph.FenceOpens(unquoted); ok {
			inFence, fenceByte, fenceLen = true, marker, n
			continue
		}
		// The shared extractor skips lines without carets before asking its
		// grammar, so structural questions run only for supported tokens.
		marker := graph.BlockAddress(line)
		if marker == "" {
			continue
		}
		if unanchorable(i, line) || owned[i] {
			continue
		}
		out = append(out, graph.FoldFragment(marker))
	}
	return out
}

// blockAddressed compares the extracted, folded addresses with the requested
// folded name. Whole-line suffixes do not declare an address.
func blockAddressed(addresses []string, want string) bool {
	return slices.Contains(addresses, want)
}

// fragmentFindings judges the fragment half of every link and transclusion
// whose name resolved to exactly one markdown note. A name that resolved to
// nothing or to several files already carries its own finding, and a non-note
// has no sections or blocks to address. A bare same-file fragment never
// reaches here.
func fragmentFindings(notes []note, unreadable []unreadableEntry, idx *graph.Index) []Finding {
	byPath := make(map[string]*note, len(notes))
	for i := range notes {
		byPath[notes[i].path] = &notes[i]
	}
	unread := make(map[string]bool, len(unreadable))
	for _, entry := range unreadable {
		unread[entry.path] = true
	}
	var out []Finding
	paths := fragmentPaths{idx: idx, notes: byPath, unread: unread}
	for i := range notes {
		n := &notes[i]
		for l := range n.wikilinks {
			link := &n.wikilinks[l]
			if link.block == "" && render.IsHeadingPath(link.heading) {
				if f, reported := paths.finding(n, link); reported {
					out = append(out, f)
				}
				continue
			}
			if f, reported := fragmentFinding(n, link, idx, byPath, unread, &paths); reported {
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
// it only has to land somewhere on, then an id the page stamps or the GitHub
// spelling of an anchor, asked of the page's own outline so the two cannot
// count a repeat differently.
func fragmentFinding(
	n *note,
	link *wikiLink,
	idx *graph.Index,
	byPath map[string]*note,
	unread map[string]bool,
	paths *fragmentPaths,
) (Finding, bool) {
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
		if blockAddressed(target.blockAddresses, graph.FoldFragment("^"+link.block)) {
			return Finding{}, false
		}
		return blockMissing(n, link, res.RelPath), true
	}
	want := graph.SectionID(link.heading)
	if link.embed {
		if target.excerptSectionAnchors[want] {
			return Finding{}, false
		}
		return sectionMissing(n, link, res.RelPath), true
	}
	if target.sectionAnchors[want] || transclusionMayBringSection(target, idx, byPath, unread, want) {
		return Finding{}, false
	}
	if paths.anchoredHeading(res.RelPath, link.heading) {
		return Finding{}, false
	}
	return sectionMissing(n, link, res.RelPath), true
}

// transclusionMayBringSection reports whether a section absent from a note's
// own body arrives through a note it transcludes, or might. The page expands a
// transclusion one level, so the walk stops at that level too. It skips the
// narrowing the transclusion's own fragment applies, which can only keep it
// quieter than the page, never louder.
//
// A transclusion of a file nothing could be read from answers "might", and this
// one link says nothing, while every other link in the run is still judged.
// That is narrower than withholding the rule, and it can be: the rules that go
// silent over a run conclude that something is nowhere in the vault, which no
// single link can be asked about, whereas here exactly one address has exactly
// one place left to look and that place could not be opened.
func transclusionMayBringSection(
	target *note,
	idx *graph.Index,
	byPath map[string]*note,
	unread map[string]bool,
	want string,
) bool {
	for _, link := range target.wikilinks {
		if !link.embed {
			continue
		}
		res := idx.Resolve(link.target)
		if res.Kind != graph.KindUnique || !vault.IsMarkdown(res.RelPath) {
			continue
		}
		if unread[res.RelPath] {
			return true
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
	rule, written := RuleID("link.section_missing"), "[["
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
	rule, written := RuleID("link.block_missing"), "[["
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

// markdownFragmentFindings judges the fragment of every Markdown link whose
// path names a note this run holds, and of every link written as only a
// fragment, which names a place in its own note. The question goes to the
// reading page's own lookup, so the page and this face cannot answer it
// differently, and the finding is the one the page reports for the same link.
// A target the contract withholds, a note that could not be read, a path that
// names no note and a source that is itself withheld are not judged: the first
// two are not this rule's to say, the next has its own finding, and the last
// is not written about at all.
func markdownFragmentFindings(notes []note, unreadable []unreadableEntry, idx *graph.Index, scan vault.Scan, authority scanAuthority) []Finding {
	byPath := make(map[string]*note, len(notes))
	for i := range notes {
		byPath[notes[i].path] = &notes[i]
	}
	unread := make(map[string]bool, len(unreadable))
	for _, entry := range unreadable {
		unread[entry.path] = true
	}
	paths := fragmentPaths{idx: idx, notes: byPath, unread: unread}
	var out []Finding
	for i := range notes {
		n := &notes[i]
		if !authority.egressAllowed(n.path) {
			continue
		}
		for _, ref := range n.pathRefs {
			rel, ok := markdownFragmentTarget(n.path, &ref, scan, authority)
			if !ok || byPath[rel] == nil || unread[rel] || embedsUnseenGround(rel, idx, byPath, unread, authority) {
				continue
			}
			if gap := paths.pipeline().FragmentGap(rel, ref.fragment); gap != nil {
				out = append(out, markdownFragmentMissing(n, &ref, rel, gap))
			}
		}
	}
	return out
}

// markdownFragmentTarget is the note a Markdown link's fragment is read in: the
// one its path resolves to, or the source itself for a link written as only a
// fragment. ok is false for a reference with no fragment, one written in code,
// and a path that reaches no single readable note.
func markdownFragmentTarget(source string, ref *pathRef, scan vault.Scan, authority scanAuthority) (rel string, ok bool) {
	if ref.code || ref.fragment == "" {
		return "", false
	}
	if ref.target == "" {
		return source, true
	}
	result := graph.ResolveMarkdown(source, ref.target, authority.readable, scan.Contains)
	if !result.Checkable || result.Withheld || result.Outside || result.Kind != graph.KindUnique {
		return "", false
	}
	return result.RelPath, true
}

// embedsUnseenGround reports whether a note, or any note it embeds at any
// depth, is one this run could not read or may not write about. The page draws
// an embedded note into the note that embeds it, so a heading or block address
// the note's own body lacks may be one of theirs, and calling it missing would
// say something about words that were never read or are withheld. The walk
// keeps a note it has been to, so a ring of embeds ends.
func embedsUnseenGround(rel string, idx *graph.Index, byPath map[string]*note, unread map[string]bool, authority scanAuthority) bool {
	seen := map[string]bool{rel: true}
	for queue := []string{rel}; len(queue) > 0; queue = queue[1:] {
		for _, link := range byPath[queue[0]].wikilinks {
			if !link.embed {
				continue
			}
			res := idx.Resolve(link.target)
			if res.Kind != graph.KindUnique || !vault.IsMarkdown(res.RelPath) || seen[res.RelPath] {
				continue
			}
			if unread[res.RelPath] || byPath[res.RelPath] == nil || !authority.readable(res.RelPath) {
				return true
			}
			seen[res.RelPath] = true
			queue = append(queue, res.RelPath)
		}
	}
	return false
}

// markdownFragmentMissing is a Markdown link whose note is real and whose
// section or block is not, under the same rules a wikilink's fragment is
// judged by. The target is the link as written, fragment included, which is
// what tells two links to different places in one note apart.
func markdownFragmentMissing(n *note, ref *pathRef, resolved string, gap *render.Diagnostic) Finding {
	address := ref.target + "#" + ref.fragment
	rule := RuleID("link.section_missing")
	message := "link to " + address + " resolves, but no heading matches \"" + gap.Section + "\""
	evidence := "the note exists and none of its headings answers the fragment"
	action := "fix the fragment after #, or add the heading to the target note"
	if gap.Kind == render.DiagLinkFragmentMissing {
		rule = "link.block_missing"
		message = "link to " + address + " resolves, but no line carries the address ^" + gap.Block
		evidence = "the note exists and no line in it ends with the block address"
		action = "fix the block name after #^, or write the address at the end of the intended line"
	}
	return Finding{
		RuleID:          rule,
		Severity:        SeverityWarn,
		Path:            n.path,
		Line:            new(ref.line),
		Message:         message,
		Evidence:        evidence,
		SuggestedAction: action,
		SourceRule:      sourceYomihon,
		Target:          new(address),
		ResolvedTo:      new(resolved),
		Fingerprint:     fingerprint(rule, n.path, address),
	}
}
