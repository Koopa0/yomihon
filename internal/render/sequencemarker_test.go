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
			heading: `<h3 id="基本觀念" data-level="2">基本觀念</h3>`,
			toc:     []render.TOCEntry{{Level: 2, Text: "基本觀念", ID: "基本觀念"}},
		},
		{
			name:    "the words the author emphasised survive the declaration coming off",
			body:    "## **粗體** {sequence=local}\n\n文字。\n",
			heading: `<h3 id="粗體" data-level="2"><strong>粗體</strong></h3>`,
			toc:     []render.TOCEntry{{Level: 2, Text: "粗體", ID: "粗體"}},
		},
		{
			name:    "an underlined branch declares the same way",
			body:    "基本觀念 {sequence=none}\n---\n\n文字。\n",
			heading: `<h3 id="基本觀念" data-level="2">基本觀念</h3>`,
			toc:     []render.TOCEntry{{Level: 2, Text: "基本觀念", ID: "基本觀念"}},
		},
		{
			name:    "a level-one heading opens no branch, so nothing is taken off it",
			body:    "# 基本觀念 {sequence=primary}\n\n文字。\n",
			heading: `<h2 id="基本觀念-sequence-primary" data-level="1">基本觀念 {sequence=primary}</h2>`,
			toc:     []render.TOCEntry{{Level: 1, Text: "基本觀念 {sequence=primary}", ID: "基本觀念-sequence-primary"}},
		},
		{
			name:    "a role quoted in code is text about the grammar, not a declaration",
			body:    "## 宣告 `{sequence=primary}`\n\n文字。\n",
			heading: `<h3 id="宣告-sequence-primary" data-level="2">宣告 <code>{sequence=primary}</code></h3>`,
			toc:     []render.TOCEntry{{Level: 2, Text: "宣告 {sequence=primary}", ID: "宣告-sequence-primary"}},
		},
		{
			name:    "a heading that is only a declaration keeps it, because nothing else would be left",
			body:    "## {sequence=primary}\n\n文字。\n",
			heading: `<h3 id="sequence-primary" data-level="2">{sequence=primary}</h3>`,
			toc:     []render.TOCEntry{{Level: 2, Text: "{sequence=primary}", ID: "sequence-primary"}},
		},
		{
			name:    "a role is read at the end of the line and nowhere else",
			body:    "## {sequence=primary} 開頭\n\n文字。\n",
			heading: `<h3 id="sequence-primary-開頭" data-level="2">{sequence=primary} 開頭</h3>`,
			toc:     []render.TOCEntry{{Level: 2, Text: "{sequence=primary} 開頭", ID: "sequence-primary-開頭"}},
		},
		{
			name:    "a value outside the three declares nothing and stays where the author can see it",
			body:    "## 基本觀念 {sequence=whatever}\n\n文字。\n",
			heading: `<h3 id="基本觀念-sequence-whatever" data-level="2">基本觀念 {sequence=whatever}</h3>`,
			toc:     []render.TOCEntry{{Level: 2, Text: "基本觀念 {sequence=whatever}", ID: "基本觀念-sequence-whatever"}},
		},
		{
			name:    "a branch declaring two roles declares neither",
			body:    "## 基本觀念 {sequence=primary} {sequence=local}\n\n文字。\n",
			heading: `<h3 id="基本觀念-sequence-primary-sequence-local" data-level="2">基本觀念 {sequence=primary} {sequence=local}</h3>`,
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

// A group-container list row declares a role the same way a heading does, and
// the reading page has to lose that declaration the same way: the reader meets
// the row's words. sequence.HeadingName is the whole test — a line that is only
// the marker stays, and a marker quoted in code keeps the closing tag that
// follows it. "The line" is the <li>'s own text up to its nested list.
func TestListRowDropsADeclaredRole(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)

	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "a container row is called by its own words",
			body: "- If your notes are not all in English {sequence=local}\n\t- child\n",
			want: "<li>If your notes are not all in English<ul>",
		},
		{
			name: "a row that is only a declaration keeps it, because nothing else would be left",
			body: "- {sequence=local}\n\t- child\n",
			want: "<li>{sequence=local}\n<ul>",
		},
		{
			name: "a role quoted in code is text about the grammar, not a declaration",
			body: "- a row naming `{sequence=local}`\n\t- child\n",
			want: "<li>a row naming <code>{sequence=local}</code>\n<ul>",
		},
		{
			name: "a loose container still loses the declaration on its own line",
			body: "- If your notes are not all in English {sequence=local}\n\n\t- child\n",
			want: "<li>\n<p>If your notes are not all in English</p>\n<ul>",
		},
		{
			name: "the words the author emphasised survive the declaration coming off",
			body: "- **bold row** {sequence=none}\n\t- child\n",
			want: "<li><strong>bold row</strong><ul>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := r.HTML("Maps/Course.md", "", tt.body, wording.ZhHant)
			if !strings.Contains(got.HTML, tt.want) {
				t.Errorf("the page does not carry %s\ngot:\n%s", tt.want, got.HTML)
			}
		})
	}
}

// A descendant's words are not the container's line. Passing the whole <li>
// to HeadingName would leave the marker — the nested list sits after it —
// so this is the case that fails if "the line" is read too wide.
func TestListRowMarkerDoesNotReadDescendantText(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)
	body := "- If your notes are not all in English {sequence=local}\n\t- child still showing `{sequence=primary}`\n"
	got := r.HTML("Maps/Course.md", "", body, wording.ZhHant)
	if !strings.Contains(got.HTML, "<li>If your notes are not all in English<ul>") {
		t.Errorf("the container kept its marker or lost its words:\n%s", got.HTML)
	}
	if !strings.Contains(got.HTML, "<code>{sequence=primary}</code>") {
		t.Errorf("a quoted marker on the child was taken off:\n%s", got.HTML)
	}
}

// goldmark reads a run of '#' closing an ATX heading as part of the marks
// rather than the words, so the page neither shows it nor folds it into the id.
// Every scan that reads the same heading from its source has to drop it too:
// a declaration is read at the end of a line, and a closing run left on the end
// hides it from the scans while the page has already taken it off.
func TestAClosingRunOfMarksIsNotPartOfTheName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		heading string
		id      string
		cited   string
	}{
		{
			name:    "a declared branch closed by a run of marks",
			heading: "## 範例 {sequence=primary} ##",
			id:      "範例",
			cited:   "範例",
		},
		{
			name:    "the closing run need not match the opening one",
			heading: "## 範例 {sequence=primary} ###",
			id:      "範例",
			cited:   "範例",
		},
		{
			name:    "a heading closed by a run and declaring nothing",
			heading: "## 範例 ##",
			id:      "範例",
			cited:   "範例",
		},
		{
			name:    "a mark with no space before it closes nothing, so the role is not at the end",
			heading: "## 範例 {sequence=primary}#",
			id:      "範例-sequence-primary",
			cited:   "範例 {sequence=primary}#",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dest := "# 課程\n\n" + tt.heading + "\n\nBASICTEXT\n\n## 後記\n\nAFTERTEXT\n"
			r := newRenderer(t, []graph.NoteInput{{RelPath: "Course.md"}}, nil, transclusions{"Course.md": dest})

			page := r.HTML("Course.md", "", dest, wording.ZhHant)
			if want := `<h3 id="` + tt.id + `" data-level="2">`; !strings.Contains(page.HTML, want) {
				t.Errorf("the page does not stamp %s\n%s", want, page.HTML)
			}

			link := r.HTML("note.md", "", "[[Course#"+tt.cited+"]]\n", wording.ZhHant)
			if want := `href="/notes/Course.md#` + tt.id + `"`; !strings.Contains(link.HTML, want) {
				t.Errorf("the citation does not address %s\n%s", want, link.HTML)
			}
			if messages := fragmentDiagnostics(&link); len(messages) != 0 {
				t.Errorf("a section the page stamps was reported missing: %q", messages)
			}

			embed := r.HTML("note.md", "", "![[Course#"+tt.cited+"]]\n", wording.ZhHant)
			if !strings.Contains(embed.HTML, "BASICTEXT") || strings.Contains(embed.HTML, "AFTERTEXT") {
				t.Errorf("the excerpt is not the section that was named:\n%s", embed.HTML)
			}
		})
	}
}
