package render_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

// A study path declares each branch's part in the course order by writing a
// role at the end of the heading that opens it. The declaration is grammar the
// course parser consumes, the way "##" is: the branch is called 基本觀念, and
// the syllabus lists it under that name. Read as prose, the same note must
// therefore call the section 基本觀念 too — in the words on the page, in the
// contents beside them, and in the id a fragment address reaches it by.
//
// A declaration the parser cannot read is a different matter. It stays visible,
// because the author is told about it — on the syllabus and by the judge — and
// a reader who cannot see the text a report quotes cannot act on it.
func TestHeadingDropsADeclaredRoleFromWordsContentsAndAnchor(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)

	tests := []struct {
		name    string
		body    string
		heading string
		toc     []render.TOCEntry
	}{
		{
			name:    "a declared branch is called by its own name",
			body:    "## 基本觀念 {sequence=primary}\n\n文字。\n",
			heading: `<h2 id="基本觀念">基本觀念</h2>`,
			toc:     []render.TOCEntry{{Level: 2, Text: "基本觀念", ID: "基本觀念"}},
		},
		{
			name:    "the words the author emphasised survive the declaration coming off",
			body:    "## **粗體** {sequence=local}\n\n文字。\n",
			heading: `<h2 id="粗體"><strong>粗體</strong></h2>`,
			toc:     []render.TOCEntry{{Level: 2, Text: "粗體", ID: "粗體"}},
		},
		{
			name:    "an underlined branch declares the same way",
			body:    "基本觀念 {sequence=none}\n---\n\n文字。\n",
			heading: `<h2 id="基本觀念">基本觀念</h2>`,
			toc:     []render.TOCEntry{{Level: 2, Text: "基本觀念", ID: "基本觀念"}},
		},
		{
			name:    "a level-one heading opens no branch, so nothing is taken off it",
			body:    "# 基本觀念 {sequence=primary}\n\n文字。\n",
			heading: `<h1 id="基本觀念-sequence-primary">基本觀念 {sequence=primary}</h1>`,
			toc:     []render.TOCEntry{{Level: 1, Text: "基本觀念 {sequence=primary}", ID: "基本觀念-sequence-primary"}},
		},
		{
			name:    "a role quoted in code is text about the grammar, not a declaration",
			body:    "## 宣告 `{sequence=primary}`\n\n文字。\n",
			heading: `<h2 id="宣告-sequence-primary">宣告 <code>{sequence=primary}</code></h2>`,
			toc:     []render.TOCEntry{{Level: 2, Text: "宣告 {sequence=primary}", ID: "宣告-sequence-primary"}},
		},
		{
			name:    "a role is read at the end of the line and nowhere else",
			body:    "## {sequence=primary} 開頭\n\n文字。\n",
			heading: `<h2 id="sequence-primary-開頭">{sequence=primary} 開頭</h2>`,
			toc:     []render.TOCEntry{{Level: 2, Text: "{sequence=primary} 開頭", ID: "sequence-primary-開頭"}},
		},
		{
			name:    "a value outside the three declares nothing and stays where the author can see it",
			body:    "## 基本觀念 {sequence=whatever}\n\n文字。\n",
			heading: `<h2 id="基本觀念-sequence-whatever">基本觀念 {sequence=whatever}</h2>`,
			toc:     []render.TOCEntry{{Level: 2, Text: "基本觀念 {sequence=whatever}", ID: "基本觀念-sequence-whatever"}},
		},
		{
			name:    "a branch declaring two roles declares neither",
			body:    "## 基本觀念 {sequence=primary} {sequence=local}\n\n文字。\n",
			heading: `<h2 id="基本觀念-sequence-primary-sequence-local">基本觀念 {sequence=primary} {sequence=local}</h2>`,
			toc: []render.TOCEntry{{
				Level: 2,
				Text:  "基本觀念 {sequence=primary} {sequence=local}",
				ID:    "基本觀念-sequence-primary-sequence-local",
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := r.HTML("Maps/Course.md", "", tt.body, wording.ZhHant)
			if !strings.Contains(got.HTML, tt.heading) {
				t.Errorf("the page does not carry %s\ngot:\n%s", tt.heading, got.HTML)
			}
			if diff := cmp.Diff(tt.toc, got.TOC); diff != "" {
				t.Errorf("contents mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// courseDest is a path note read as the destination of a citation: two declared
// branches, each holding a sentinel, so a cut section is told from the note.
const courseDest = "# 課程\n\n開場。\n\n## 基本觀念 {sequence=primary}\n\nBASICTEXT\n\n## 不分先後 {sequence=none}\n\nUNORDEREDTEXT\n"

func courseRenderer(t *testing.T) *render.Pipeline {
	t.Helper()
	return newRenderer(t, []graph.NoteInput{{RelPath: "Course.md"}}, nil, transclusions{"Course.md": courseDest})
}

// A branch's name is what a reader sees and therefore what they write into a
// citation. The name and the id it reaches must be the same on both sides of
// the link: the destination page stamps the id from the branch's name, and the
// scan that checks the address before the page is served reads the destination's
// source, so a section this page can reach may not be reported as missing.
func TestALinkToADeclaredBranchReachesIt(t *testing.T) {
	t.Parallel()

	got := courseRenderer(t).HTML("note.md", "", "[[Course#基本觀念]]\n", wording.ZhHant)

	if !strings.Contains(got.HTML, `href="/notes/Course.md#基本觀念"`) {
		t.Errorf("the citation does not address the branch by its name:\n%s", got.HTML)
	}
	if messages := fragmentDiagnostics(&got); len(messages) != 0 {
		t.Errorf("a branch the page can reach was reported missing: %q", messages)
	}
}

// The excerpt scan cuts a transclusion to the section its address names, and it
// reads the destination's source rather than the rendered page. It has to call
// a branch what the page calls it, or an author citing a section by the name
// they can see is shown a notice where the words should be.
func TestAnEmbedOfADeclaredBranchCutsThatBranch(t *testing.T) {
	t.Parallel()

	got := courseRenderer(t).HTML("note.md", "", "![[Course#基本觀念]]\n", wording.ZhHant)

	if !strings.Contains(got.HTML, "BASICTEXT") {
		t.Errorf("the excerpt does not carry the branch's words:\n%s", got.HTML)
	}
	if strings.Contains(got.HTML, "UNORDEREDTEXT") {
		t.Errorf("the excerpt reached past the branch it names:\n%s", got.HTML)
	}
	for _, d := range got.Diagnostics {
		if d.Kind == render.DiagEmbedFragmentMissing {
			t.Errorf("a branch the scan can cut was reported missing: %s", d.Message)
		}
	}
}

// Two branches of one course may be written with the same name and different
// roles. Their ids differ today only because the role is part of them; once it
// is not, they are two headings with one name, and the page numbers them as it
// numbers any other repeat. The cost is recorded rather than hidden: the second
// is reachable at an id no citation would write.
func TestTwoBranchesOfOneNameAreNumberedLikeAnyRepeat(t *testing.T) {
	t.Parallel()

	r := newRenderer(t, nil, nil, nil)
	body := "## 基本觀念 {sequence=primary}\n\n一。\n\n## 基本觀念 {sequence=none}\n\n二。\n"
	want := []render.TOCEntry{
		{Level: 2, Text: "基本觀念", ID: "基本觀念"},
		{Level: 2, Text: "基本觀念", ID: "基本觀念-2"},
	}
	if diff := cmp.Diff(want, r.HTML("Maps/Course.md", "", body, wording.ZhHant).TOC); diff != "" {
		t.Errorf("contents mismatch (-want +got):\n%s", diff)
	}
}

// A preview card names the section it shows by the heading its excerpt opens
// on, which is a fourth place the same name is read — and the one place it is
// read without an id being stamped from it, so nothing else would catch it.
func TestAnExcerptNamesTheBranchItOpensOn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		slice string
		want  string
	}{
		{
			name:  "a declared branch",
			slice: "## 基本觀念 {sequence=primary}\n\n文字。\n",
			want:  "基本觀念",
		},
		{
			name:  "an underlined branch",
			slice: "基本觀念 {sequence=none}\n---\n\n文字。\n",
			want:  "基本觀念",
		},
		{
			name:  "a level-one heading opens no branch",
			slice: "# 基本觀念 {sequence=primary}\n\n文字。\n",
			want:  "基本觀念 {sequence=primary}",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := render.ExcerptHeading(tt.slice); got != tt.want {
				t.Errorf("ExcerptHeading(%q) = %q, want %q", tt.slice, got, tt.want)
			}
		})
	}
}
