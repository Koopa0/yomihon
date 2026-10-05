package judge

import (
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
	body := sharedBodyFixture()
	data := []byte(body)
	allocations := testing.AllocsPerRun(10, func() { parseNote("Notes/source.md", data) })
	t.Logf("parseNote allocations = %.0f", allocations)
	if allocations > 14000 {
		t.Errorf("parseNote allocations = %.0f, want at most 14000 for the shared body", allocations)
	}
}

func BenchmarkParseNoteSharedBody(b *testing.B) {
	data := []byte(sharedBodyFixture())
	b.ReportAllocs()
	for b.Loop() {
		parseNote("Notes/source.md", data)
	}
}

// separateBodyExtractions gives each harvest a fresh structure, so sharing
// mutable heading storage or confusing two bodies cannot change the answer.
func separateBodyExtractions(data []byte, marks plannedMarks) note {
	n := parseFrontmatter("Notes/source.md", data)
	block, _ := vault.SplitFrontmatter(data)
	body := string(block.Body)
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
