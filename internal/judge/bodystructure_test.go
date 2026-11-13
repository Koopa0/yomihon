package judge

import (
	"runtime"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/koopa0/yomihon/internal/sequence"
	"github.com/koopa0/yomihon/internal/vault"
)

func sharedBodyFixture() string {
	return strings.Repeat("## Visible heading\n\n[[Live]] [Manual](Manual.md) `quoted-path.md`\n\n> [!note] A title\n> A passage.\n\n%% [[Hidden]]\n## Hidden heading\n%%\n\n```text\n[[Code]]\n```\n\n", 32)
}

func TestParseNoteSharesBodyStructure(t *testing.T) {
	if err := judgeBodyStructureLock("."); err != nil {
		t.Fatalf("caught: shared body structure lock: %v", err)
	}
	body := sharedBodyFixture()
	data := []byte(body)
	marks := defaultPlannedMarks()
	want := separateBodyExtractions(data, marks)
	got := parseNoteWithMarks("Notes/source.md", data, marks)
	if diff := diffHarvests(body, &want, &got); diff != "" {
		t.Fatalf("parseNoteWithMarks differs from independent harvests (-want +got):\n%s", diff)
	}
}

func BenchmarkParseNoteSharedBody(b *testing.B) {
	body := sharedBodyFixture()
	data := []byte(body)
	marks := defaultPlannedMarks()
	for _, bb := range []struct {
		name string
		read func()
	}{
		{name: "separate", read: func() { runtime.KeepAlive(separateBodyExtractions(data, marks)) }},
		{name: "shared", read: func() { runtime.KeepAlive(parseNoteWithMarks("Notes/source.md", data, marks)) }},
		{name: "one marked and three unmarked structures", read: func() {
			code, headings := structure(body, marks.heading)
			runtime.KeepAlive(code)
			runtime.KeepAlive(headings)
			for range 3 {
				code, headings := structure(body, nil)
				runtime.KeepAlive(code)
				runtime.KeepAlive(headings)
			}
		}},
	} {
		b.Run(bb.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				bb.read()
			}
		})
	}
}

// separateBodyExtractions gives each harvest a fresh structure, so sharing
// mutable heading storage or confusing two bodies cannot change the answer.
func separateBodyExtractions(data []byte, marks plannedMarks) note {
	n := parseFrontmatter("Notes/source.md", data)
	block, _ := vault.SplitFrontmatter(data)
	body := string(block.Body)
	n.body = body
	n.title, _ = titleByHand(body)
	n.wikilinks = extractWikilinksWith(body, block.BodyStartLine, marks.heading)
	n.pathRefs = extractPathRefs(body, block.BodyStartLine)
	_, n.images, n.unusedDefinitions = extractReferencesFrom(body, block.BodyStartLine, inspectBody(body, nil).comments)
	n.plannedNames = extractPlannedNamesWith(body, marks)
	n.calloutTitles = extractCalloutTitles(body, block.BodyStartLine)
	n.sequence = sequence.Parse(body, block.BodyStartLine)
	n.sectionAnchors, n.excerptSectionAnchors, n.blockAddresses = anchorSurface(body)
	return n
}

// titleByHand is the title oracle: the first level-one ATX heading, written out
// from the heading rule alone and sharing no code with the title the note is
// read by, so a fault in that reading cannot also be in the answer it is held
// to. It models plain bodies only. A body holding a marker that changes what a
// reader is shown (a code fence or span, an HTML block or comment, a %% or $$
// span, or an underline that could turn a paragraph into a heading) is a
// construct the oracle does not model, and modelled is false: the heading rule
// for those is held by the cases of graph's TestTitleSkipsWhatTheReaderNeverSees.
func titleByHand(body string) (title string, modelled bool) {
	if strings.ContainsAny(body, "`~<%$=") {
		return "", false
	}
	for line := range strings.SplitSeq(body, "\n") {
		indent := len(line) - len(strings.TrimLeft(line, " "))
		if indent > 3 || len(line) < indent+2 || line[indent] != '#' || (line[indent+1] != ' ' && line[indent+1] != '\t') {
			continue
		}
		words := strings.Trim(line[indent+2:], " \t")
		// A closing run of '#' belongs to the heading's marks only when white
		// space sits before it inside the words; a run that is the whole of
		// the words, or is glued to text, is text.
		if bare := strings.TrimRight(words, "#"); len(bare) < len(words) && bare != "" && strings.ContainsAny(bare[len(bare)-1:], " \t") {
			words = strings.TrimRight(bare, " \t")
		}
		if folded := strings.Join(strings.Fields(words), " "); folded != "" {
			return folded, true
		}
	}
	return "", true
}

// diffHarvests compares the independent harvests with the shared parse. The
// title is compared whenever the oracle models the body, and left out only for
// a body it does not.
func diffHarvests(body string, want, got *note) string {
	options := []cmp.Option{cmp.AllowUnexported(note{}, fmValue{}, wikiLink{}, pathRef{}, imageRef{}, linkDefinition{}, calloutTitle{})}
	if _, modelled := titleByHand(body); !modelled {
		options = append(options, cmpopts.IgnoreFields(note{}, "title"))
	}
	return cmp.Diff(want, got, options...)
}

func TestTitleByHandReadsTheFirstLevelOneHeading(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, body, want string
		modelled         bool
	}{
		{"a bare heading", "# 00", "00", true},
		{"the first of two", "# First\n\n# Second\n", "First", true},
		{"a lesser heading is passed over", "## Lesser\n# Real\n", "Real", true},
		{"a closing run is marks", "# Title ##  \n", "Title", true},
		{"a closing run glued to text is text", "# Title#\n", "Title#", true},
		{"a run that is the whole of the words is text", "# #\n", "#", true},
		{"a tab opens the words", "#\tTab title\n", "Tab title", true},
		{"three spaces of indent are a heading, four are code", "    # Code\n   # Real\n", "Real", true},
		{"no space after the mark is not a heading", "#Tag\n# Real\n", "Real", true},
		{"seven marks are not a heading", "####### x\n", "", true},
		{"a heading with no words is passed over", "# \n#\t\n# Second\n", "Second", true},
		{"white space folds", "#   a \t b\n", "a b", true},
		{"no heading", "just text\n", "", true},
		{"a fence is not modelled", "```\n# hidden\n```\n# Real\n", "", false},
		{"an underline is not modelled", "Title\n=====\n", "", false},
		{"a comment is not modelled", "%%\n# hidden\n%%\n", "", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, modelled := titleByHand(tt.body)
			if got != tt.want || modelled != tt.modelled {
				t.Errorf("titleByHand(%q) = %q, %v, want %q, %v", tt.body, got, modelled, tt.want, tt.modelled)
			}
		})
	}
}

func TestSharedJudgeBodyKeepsIndependentExtractions(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ name, body string }{
		{"empty", ""},
		{"mixed", sharedBodyFixture()},
		{"commented boundary", "## Gap\n- Planned\n%%\n## Closed\n%%\n[[Future]] planned\n"},
		{"code and comment delimiters", "`%%` [[Live]]\n\n```text\n%% [[Hidden]]\n```\n\n> [!note] Title\n> `Code`\n"},
		{"crlf frontmatter", "---\r\ntitle: Source\r\n---\r\n## Gap\r\n[[Live]]\r\n"},
		{"invalid yaml", "---\ntitle: \"unfinished\n---\n[[Live]]\n"},
		{"contract heading marks", "## Gap\n- Planned\n[[Target]] planned\n## Stop\n[[Other]]\n"},
		{"a plain titled note", "---\ntype: note\n---\n# Plain title\n\n## Gap\n[[Live]] planned\n"},
		{"two level-one headings", "# First\n\n[[Live]]\n\n# Second\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data := []byte(tt.body)
			for _, marks := range []plannedMarks{defaultPlannedMarks(), {heading: []string{"Gap"}, inline: []string{"planned"}}, {}} {
				want := separateBodyExtractions(data, marks)
				got := parseNoteWithMarks("Notes/source.md", data, marks)
				block, _ := vault.SplitFrontmatter(data)
				if diff := diffHarvests(string(block.Body), &want, &got); diff != "" {
					t.Errorf("parseNoteWithMarks(%q) differs from independent harvests (-want +got):\n%s", tt.body, diff)
				}
			}
		})
	}
}

func FuzzSharedJudgeBodyStructure(f *testing.F) {
	for _, body := range []string{"", sharedBodyFixture(), "## Gap\n- Planned\n%% [[Hidden]] %%\n[[Live#Place]] planned\n", "---\ntitle: \"unfinished\n---\n[[Live]]\n", "# 00", "# First\n\n# Second\n", "## Lesser\n#\tTab ##\n", "#  a \t  b  #\n"} {
		f.Add([]byte(body))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 8192 {
			t.Skip()
		}
		marks := plannedMarks{heading: []string{"Gap"}, inline: []string{"planned"}}
		want := separateBodyExtractions(data, marks)
		got := parseNoteWithMarks("Notes/source.md", data, marks)
		block, _ := vault.SplitFrontmatter(data)
		if diff := diffHarvests(string(block.Body), &want, &got); diff != "" {
			t.Errorf("parseNoteWithMarks(%q) differs from independent harvests (-want +got):\n%s", data, diff)
		}
	})
}

// extractCalloutTitles walks a note body the way the page classifies a
// callout: fences and comments are not titles, an unknown type is a
// blockquote, and a recognised opening's title is the text after `[!type]`.
func extractCalloutTitles(body string, bodyStartLine int) []calloutTitle {
	facts := inspectBody(body, nil)
	return extractCalloutTitlesFrom(body, bodyStartLine, facts.comments)
}

// extractPathRefs returns every checkable file reference in body: markdown
// [text](path.md) links, decoded once, and backticked path.md tokens. URLs,
// anchors, glob paths and malformed escapes are left out, so only in-vault
// file references remain. A reference inside an Obsidian %%...%% comment is skipped,
// the same way a commented-out wikilink is: commented-out content is not a live
// reference, so it is not checked.
func extractPathRefs(body string, bodyStartLine int) []pathRef {
	facts := inspectBody(body, nil)
	refs, _, _ := extractReferencesFrom(body, bodyStartLine, facts.comments)
	return refs
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
func anchorSurface(body string) (sections, excerptSections map[string]bool, blockAddresses []string) {
	facts := inspectBody(body, nil)
	return anchorSurfaceFrom(body, facts.comments)
}
