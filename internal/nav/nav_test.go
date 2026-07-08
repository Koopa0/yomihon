package nav

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
)

// resolver builds a real *graph.Index (testing.md's "real first") from
// in-memory note paths — no disk — so parseSections resolves lesson
// wikilinks through the exact normalization and ambiguity rules production
// uses. Same technique as internal/render's tests.
func resolver(t *testing.T, paths ...string) *graph.Index {
	t.Helper()
	notes := make([]graph.NoteInput, 0, len(paths))
	for _, p := range paths {
		notes = append(notes, graph.NoteInput{Path: p})
	}
	return graph.BuildFromNotes(notes, nil)
}

// TestParseSectionsGoShape covers the pipe-format Go syllabus shape: H2/H3
// headings "slug | English | Chinese" (the English column becomes the
// label) and "- [[Lesson]]" bullets, with prose lines (範圍/★) between
// headings that must NOT become lessons, and a trailing part with no
// lessons that must be pruned away.
func TestParseSectionsGoShape(t *testing.T) {
	t.Parallel()

	idx := resolver(t, "L/Lesson A.md", "L/Lesson B.md", "L/Lesson C.md")
	statusByPath := map[string]string{
		"L/Lesson A.md": "draft",
		"L/Lesson B.md": "ready",
		// Lesson C intentionally absent -> empty status badge.
	}

	body := "> intro blockquote citing [[Lesson A]] — a quote, not a bullet\n" +
		"\n" +
		"## data-and-hardware | Data and the Hardware | 資料與硬體本質\n" +
		"\n" +
		"範圍:this is prose, not a list item, and it names [[Lesson B]].\n" +
		"★ 待補模組:bits-bytes-words (prose only).\n" +
		"\n" +
		"### text-as-bytes | Text as Bytes | 文字即 bytes\n" +
		"\n" +
		"- [[Lesson A]]\n" +
		"- [[Lesson B]]\n" +
		"\n" +
		"### alignment | Alignment | 對齊\n" +
		"\n" +
		"- [[Lesson C]]\n" +
		"\n" +
		"## empty-part | Empty Part | 空帶\n" +
		"\n" +
		"範圍:prose only, no lessons anywhere under this part.\n"

	want := []Section{
		{
			Heading: "Data and the Hardware",
			Level:   2,
			Sub: []Section{
				{
					Heading: "Text as Bytes",
					Level:   3,
					Lessons: []Lesson{
						{Text: "Lesson A", Target: "Lesson A", RelPath: "L/Lesson A.md", Status: "draft", Resolution: graph.Unique},
						{Text: "Lesson B", Target: "Lesson B", RelPath: "L/Lesson B.md", Status: "ready", Resolution: graph.Unique},
					},
				},
				{
					Heading: "Alignment",
					Level:   3,
					Lessons: []Lesson{
						{Text: "Lesson C", Target: "Lesson C", RelPath: "L/Lesson C.md", Resolution: graph.Unique},
					},
				},
			},
		},
	}

	got := parseSections(body, idx, statusByPath)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("parseSections (Go shape) mismatch (-want +got):\n%s", diff)
	}
}

// TestParseSectionsMinnaShape covers the 大家 shape: only the
// course-sequence section holds a lesson tree; the daily-loop (ordered
// list), learning-stages (a table), and gaps (task checkboxes) sections
// carry no lesson bullets and must prune away — even the gap task item that
// contains a [[wikilink]], and even the loop's ordered item that contains
// one. Lesson bullets are "- **Lx** ... · [[Lxx]]"; a "待建" bullet with no
// wikilink is not a lesson. The H1 title is ignored.
func TestParseSectionsMinnaShape(t *testing.T) {
	t.Parallel()

	idx := resolver(t, "jp/L01 Intro.md", "jp/L02 Next.md", "jp/L03 Verbs.md")
	statusByPath := map[string]string{
		"jp/L01 Intro.md": "draft",
		"jp/L02 Next.md":  "draft",
		"jp/L03 Verbs.md": "ready",
	}

	body := "# Doc Title (an H1, ignored)\n" +
		"\n" +
		"> [!info] callout prose that mentions [[Loop Link]]\n" +
		"\n" +
		"## Daily loop\n" +
		"\n" +
		"1. **step** in an ORDERED item, links [[Loop Link]] — not a lesson.\n" +
		"2. another step.\n" +
		"\n" +
		"## Stages (a table)\n" +
		"\n" +
		"| stage | 課 |\n" +
		"| --- | --- |\n" +
		"| decode | L1-3 |\n" +
		"\n" +
		"## Course sequence (order = lines)\n" +
		"\n" +
		"### Decode\n" +
		"\n" +
		"- **L1** intro · grammar · app:x · [[L01 Intro]]\n" +
		"- **L2** next · [[L02 Next]]\n" +
		"\n" +
		"### Verbs\n" +
		"\n" +
		"- **L3** verbs · [[L03 Verbs]]\n" +
		"- **L4** more · 待建\n" +
		"\n" +
		"## Gaps\n" +
		"\n" +
		"- [x] done (see [[Some Guide]])\n" +
		"- [ ] todo (spec [[Another Guide]])\n"

	want := []Section{
		{
			Heading: "Course sequence (order = lines)",
			Level:   2,
			Sub: []Section{
				{
					Heading: "Decode",
					Level:   3,
					Lessons: []Lesson{
						{Text: "L01 Intro", Target: "L01 Intro", RelPath: "jp/L01 Intro.md", Status: "draft", Resolution: graph.Unique},
						{Text: "L02 Next", Target: "L02 Next", RelPath: "jp/L02 Next.md", Status: "draft", Resolution: graph.Unique},
					},
				},
				{
					Heading: "Verbs",
					Level:   3,
					Lessons: []Lesson{
						{Text: "L03 Verbs", Target: "L03 Verbs", RelPath: "jp/L03 Verbs.md", Status: "ready", Resolution: graph.Unique},
					},
				},
			},
		},
	}

	got := parseSections(body, idx, statusByPath)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("parseSections (大家 shape) mismatch (-want +got):\n%s", diff)
	}
}

// TestParseSectionsFaultTolerance proves an unresolved or ambiguous lesson
// is still listed with the right resolution state, never silently dropped,
// and that malformed heading/list lines are ignored without panicking.
func TestParseSectionsFaultTolerance(t *testing.T) {
	t.Parallel()

	// "Dup" resolves to two files -> Ambiguous with sorted candidates.
	idx := resolver(t, "A/Dup.md", "B/Dup.md", "ok/Real.md")

	body := "## part | Part | 部\n" +
		"\n" +
		"###notaspace this is not a heading\n" +
		"#### \n" + // empty-label deeper heading, no lessons -> pruned
		"- \n" + // bare bullet, no wikilink -> not a lesson
		"- [ ] a task with a [[Real]] link -> excluded\n" +
		"- no wikilink here at all\n" +
		"\n" +
		"### mod | Module | 模組\n" +
		"\n" +
		"- [[Real]]\n" +
		"- [[Ghost]]\n" + // unresolved: still listed
		"- [[Dup]]\n" // ambiguous: still listed with candidates

	want := []Section{
		{
			Heading: "Part",
			Level:   2,
			Sub: []Section{
				{
					Heading: "Module",
					Level:   3,
					Lessons: []Lesson{
						{Text: "Real", Target: "Real", RelPath: "ok/Real.md", Resolution: graph.Unique},
						{Text: "Ghost", Target: "Ghost", Resolution: graph.Unresolved},
						{Text: "Dup", Target: "Dup", Resolution: graph.Ambiguous, Candidates: []string{"A/Dup.md", "B/Dup.md"}},
					},
				},
			},
		},
	}

	got := parseSections(body, idx, map[string]string{})
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("parseSections (fault tolerance) mismatch (-want +got):\n%s", diff)
	}
}

// TestParseHeading and TestParseLessonItem lock down the two line
// classifiers the whole rule rests on.
func TestParseHeading(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		line      string
		wantText  string
		wantLevel int
		wantOK    bool
	}{
		{name: "h2 pipe", line: "## slug | English | 中文", wantText: "slug | English | 中文", wantLevel: 2, wantOK: true},
		{name: "h3 plain", line: "### 解碼期", wantText: "解碼期", wantLevel: 3, wantOK: true},
		{name: "h1 ignored", line: "# Title", wantOK: false},
		{name: "no space", line: "###notaspace", wantOK: false},
		{name: "not a heading", line: "- [[Lesson]]", wantOK: false},
		{name: "h4 empty label", line: "#### ", wantText: "", wantLevel: 4, wantOK: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			text, level, ok := parseHeading(tt.line)
			if ok != tt.wantOK || text != tt.wantText || level != tt.wantLevel {
				t.Errorf("parseHeading(%q) = (%q, %d, %t), want (%q, %d, %t)",
					tt.line, text, level, ok, tt.wantText, tt.wantLevel, tt.wantOK)
			}
		})
	}
}

func TestParseLessonItem(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		line      string
		wantInner string
		wantOK    bool
	}{
		{name: "go bullet", line: "- [[Slices- Strings and Slices]]", wantInner: "Slices- Strings and Slices", wantOK: true},
		{name: "minna trailing link", line: "- **L1** intro · [[L01 〜は〜です]]", wantInner: "L01 〜は〜です", wantOK: true},
		{name: "star bullet", line: "* [[Alt]]", wantInner: "Alt", wantOK: true},
		{name: "ordered item excluded", line: "1. **step** links [[Loop Link]]", wantOK: false},
		{name: "task unchecked excluded", line: "- [ ] todo (spec [[Guide]])", wantOK: false},
		{name: "task checked excluded", line: "- [x] done, see [[Guide]]", wantOK: false},
		{name: "bullet without wikilink", line: "- **L21** 引用・意見 · 待建", wantOK: false},
		{name: "blockquote excluded", line: "> prose with [[Link]]", wantOK: false},
		{name: "bare bullet", line: "- ", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			inner, ok := parseLessonItem(tt.line)
			if ok != tt.wantOK || inner != tt.wantInner {
				t.Errorf("parseLessonItem(%q) = (%q, %t), want (%q, %t)",
					tt.line, inner, ok, tt.wantInner, tt.wantOK)
			}
		})
	}
}

// TestBuildFolderTree checks the browse tree mirrors the real directory
// structure (to whatever depth it has), strips only .md for display,
// surfaces a vault-root file as a RootNote, and orders the top level by
// lifecycle (Inbox before Concepts before Writing before Views), leaving
// deeper levels lexical.
func TestBuildFolderTree(t *testing.T) {
	t.Parallel()

	// vault.List order is lexical; provide the paths that way.
	paths := []string{
		"CLAUDE.md",
		"Concepts/golang/Foo.md",
		"Concepts/rust/Bar.md",
		"Inbox/note.md",
		"Views/board.base",
		"Writing/lessons/golang/Lesson.md",
	}

	wantFolders := []Folder{
		{Name: "Inbox", RelPath: "Inbox", Notes: []NoteRef{{Name: "note", RelPath: "Inbox/note.md"}}},
		{Name: "Concepts", RelPath: "Concepts", Sub: []Folder{
			{Name: "golang", RelPath: "Concepts/golang", Notes: []NoteRef{{Name: "Foo", RelPath: "Concepts/golang/Foo.md"}}},
			{Name: "rust", RelPath: "Concepts/rust", Notes: []NoteRef{{Name: "Bar", RelPath: "Concepts/rust/Bar.md"}}},
		}},
		{Name: "Writing", RelPath: "Writing", Sub: []Folder{
			{Name: "lessons", RelPath: "Writing/lessons", Sub: []Folder{
				{Name: "golang", RelPath: "Writing/lessons/golang", Notes: []NoteRef{{Name: "Lesson", RelPath: "Writing/lessons/golang/Lesson.md"}}},
			}},
		}},
		{Name: "Views", RelPath: "Views", Notes: []NoteRef{{Name: "board.base", RelPath: "Views/board.base"}}},
	}
	wantRoot := []NoteRef{{Name: "CLAUDE", RelPath: "CLAUDE.md"}}

	folders, rootNotes := buildFolderTree(paths)
	if diff := cmp.Diff(wantFolders, folders); diff != "" {
		t.Errorf("buildFolderTree folders mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(wantRoot, rootNotes); diff != "" {
		t.Errorf("buildFolderTree rootNotes mismatch (-want +got):\n%s", diff)
	}
}

// TestPlacements inverts the syllabus trees into the note -> placements map the
// sidebar reads to open itself to the current note. It covers the two cases that
// make the inversion worth building: a note listed by two different study-paths
// (two placements, one per path), and a note listed by two sections of the same
// study-path (two placements, each carrying its own heading chain). An
// unresolved lesson carries no path and must not appear.
func TestPlacements(t *testing.T) {
	t.Parallel()

	syllabi := []Syllabus{
		{
			Title: "Go", RelPath: "Maps/go-path.md",
			Sections: []Section{
				{Heading: "Part A", Level: 2, Sub: []Section{
					{Heading: "Module 1", Level: 3, Lessons: []Lesson{
						{Text: "L1", Target: "L1", RelPath: "L/L1.md", Resolution: graph.Unique},
						{Text: "Shared", Target: "Shared", RelPath: "L/Shared.md", Resolution: graph.Unique},
					}},
				}},
				{Heading: "Part B", Level: 2, Lessons: []Lesson{
					{Text: "Shared", Target: "Shared", RelPath: "L/Shared.md", Resolution: graph.Unique},
					{Text: "Ghost", Target: "Ghost", Resolution: graph.Unresolved},
				}},
			},
		},
		{
			Title: "JP", RelPath: "Maps/jp-path.md",
			Sections: []Section{
				{Heading: "Unit 1", Level: 2, Lessons: []Lesson{
					{Text: "L1", Target: "L1", RelPath: "L/L1.md", Resolution: graph.Unique},
				}},
			},
		},
	}
	m := &Model{lessonIndex: buildLessonIndex(syllabi)}

	tests := []struct {
		name    string
		relPath string
		want    []Placement
	}{
		{
			name:    "listed by two study-paths",
			relPath: "L/L1.md",
			want: []Placement{
				{SyllabusRelPath: "Maps/go-path.md", Headings: []string{"Part A", "Module 1"}},
				{SyllabusRelPath: "Maps/jp-path.md", Headings: []string{"Unit 1"}},
			},
		},
		{
			name:    "listed by two sections of one study-path",
			relPath: "L/Shared.md",
			want: []Placement{
				{SyllabusRelPath: "Maps/go-path.md", Headings: []string{"Part A", "Module 1"}},
				{SyllabusRelPath: "Maps/go-path.md", Headings: []string{"Part B"}},
			},
		},
		{name: "unresolved lesson never indexed", relPath: "Ghost", want: nil},
		{name: "unreferenced note", relPath: "L/Orphan.md", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if diff := cmp.Diff(tt.want, m.Placements(tt.relPath)); diff != "" {
				t.Errorf("Placements(%q) mismatch (-want +got):\n%s", tt.relPath, diff)
			}
		})
	}
}

// TestSiblings checks the "here" lookup returns a note's whole directory (itself
// included, for the caller to mark) in the folder tree's lexical order, reports
// the empty directory for a vault-root note, and yields nothing for an unknown
// directory.
func TestSiblings(t *testing.T) {
	t.Parallel()

	paths := []string{
		"Concepts/golang/Baz.md",
		"Concepts/golang/Foo.md",
		"Concepts/rust/Bar.md",
		"README.md",
	}
	m := &Model{dirNotes: buildDirNotes(paths)}

	tests := []struct {
		name      string
		relPath   string
		wantDir   string
		wantNotes []NoteRef
	}{
		{
			name:    "same-directory siblings include self",
			relPath: "Concepts/golang/Foo.md",
			wantDir: "Concepts/golang",
			wantNotes: []NoteRef{
				{Name: "Baz", RelPath: "Concepts/golang/Baz.md"},
				{Name: "Foo", RelPath: "Concepts/golang/Foo.md"},
			},
		},
		{
			name:      "vault-root note has the empty directory",
			relPath:   "README.md",
			wantDir:   "",
			wantNotes: []NoteRef{{Name: "README", RelPath: "README.md"}},
		},
		{name: "unknown directory", relPath: "Nowhere/x.md", wantDir: "Nowhere", wantNotes: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir, notes := m.Siblings(tt.relPath)
			if dir != tt.wantDir {
				t.Errorf("Siblings(%q) dir = %q, want %q", tt.relPath, dir, tt.wantDir)
			}
			if diff := cmp.Diff(tt.wantNotes, notes); diff != "" {
				t.Errorf("Siblings(%q) notes mismatch (-want +got):\n%s", tt.relPath, diff)
			}
		})
	}
}

// TestBuildReports checks the .md reports (directly under System/reports/)
// come first, then the daily-briefing/ HTML briefings with latest.html
// marked, and that a daily-briefing README.md, a stray .txt, and files
// outside System/reports/ are all excluded.
func TestBuildReports(t *testing.T) {
	t.Parallel()

	paths := []string{
		"Concepts/foo.md",
		"System/reports/Run-Report.md",
		"System/reports/daily-briefing/README.md",
		"System/reports/daily-briefing/koopa0-briefing-2026-07-02.html",
		"System/reports/daily-briefing/latest.html",
		"System/reports/vault-check.md",
		"System/reports/notes.txt",
	}

	want := []Report{
		{Name: "Run-Report.md", RelPath: "System/reports/Run-Report.md"},
		{Name: "vault-check.md", RelPath: "System/reports/vault-check.md"},
		{Name: "koopa0-briefing-2026-07-02.html", RelPath: "System/reports/daily-briefing/koopa0-briefing-2026-07-02.html", Briefing: true},
		{Name: "latest.html", RelPath: "System/reports/daily-briefing/latest.html", Briefing: true, Latest: true},
	}

	got := buildReports(paths)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("buildReports mismatch (-want +got):\n%s", diff)
	}
}
