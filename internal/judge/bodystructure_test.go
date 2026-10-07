package judge

import (
	"runtime"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

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
	if diff := cmp.Diff(want, got, cmp.AllowUnexported(note{}, fmValue{}, wikiLink{}, pathRef{}, calloutTitle{})); diff != "" {
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
	n.wikilinks = extractWikilinksWith(body, block.BodyStartLine, marks.heading)
	n.pathRefs = extractPathRefs(body, block.BodyStartLine)
	n.plannedNames = extractPlannedNamesWith(body, marks)
	n.calloutTitles = extractCalloutTitles(body, block.BodyStartLine)
	n.sequence = sequence.Parse(body, block.BodyStartLine)
	n.sectionAnchors, n.excerptSectionAnchors, n.blockAnchorLines = anchorSurface(body)
	return n
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
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data := []byte(tt.body)
			for _, marks := range []plannedMarks{defaultPlannedMarks(), {heading: []string{"Gap"}, inline: []string{"planned"}}, {}} {
				want := separateBodyExtractions(data, marks)
				got := parseNoteWithMarks("Notes/source.md", data, marks)
				if diff := cmp.Diff(want, got, cmp.AllowUnexported(note{}, fmValue{}, wikiLink{}, pathRef{}, calloutTitle{})); diff != "" {
					t.Errorf("parseNoteWithMarks(%q) differs from independent harvests (-want +got):\n%s", tt.body, diff)
				}
			}
		})
	}
}

func FuzzSharedJudgeBodyStructure(f *testing.F) {
	for _, body := range []string{"", sharedBodyFixture(), "## Gap\n- Planned\n%% [[Hidden]] %%\n[[Live#Place]] planned\n", "---\ntitle: \"unfinished\n---\n[[Live]]\n"} {
		f.Add([]byte(body))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 8192 {
			t.Skip()
		}
		marks := plannedMarks{heading: []string{"Gap"}, inline: []string{"planned"}}
		want := separateBodyExtractions(data, marks)
		got := parseNoteWithMarks("Notes/source.md", data, marks)
		if diff := cmp.Diff(want, got, cmp.AllowUnexported(note{}, fmValue{}, wikiLink{}, pathRef{}, calloutTitle{})); diff != "" {
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
	return extractPathRefsFrom(body, bodyStartLine, facts.comments)
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
	facts := inspectBody(body, nil)
	return anchorSurfaceFrom(body, facts.comments)
}
