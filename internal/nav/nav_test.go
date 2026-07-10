package nav

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/schema"
)

func writeNavFixture(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatalf("mkdir %s: %v", rel, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func TestBuildMapTypesAndReversePlacements(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeNavFixture(t, root, "Concepts/go/Target.md", "---\ntitle: Target\ntype: concept\ndomain: golang\nstatus: growing\n---\nbody\n")
	writeNavFixture(t, root, "A/Dup.md", "body\n")
	writeNavFixture(t, root, "B/Dup.md", "body\n")

	fixtures := []struct {
		rel, title, noteType, domain string
	}{
		{rel: "Maps/Course.md", title: "Course", noteType: "study-path", domain: "golang"},
		{rel: "Maps/A-zeta.md", title: "Zeta", noteType: "moc", domain: "golang"},
		{rel: "Maps/Beta.md", title: "Beta", noteType: "topic-map", domain: "japanese"},
		{rel: "Maps/Z-alpha.md", title: "Alpha", noteType: "source-map", domain: "golang"},
	}
	for _, f := range fixtures {
		writeNavFixture(t, root, f.rel, fmt.Sprintf("---\ntitle: %s\ntype: %s\ndomain: %s\n---\n## Shelf\n- [[Target]]\n- [[Ghost]]\n- [[Dup]]\n", f.title, f.noteType, f.domain))
	}

	idx, err := graph.Build(root)
	if err != nil {
		t.Fatalf("graph.Build: %v", err)
	}
	model, err := Build(root, idx, nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	wantPaths := []Map{{
		Title: "Course", RelPath: "Maps/Course.md", Domain: "golang", Type: "study-path",
		Branches: []Branch{{Heading: "Shelf", Level: 2, Entries: []Entry{{Text: "Target", Target: "Target", RelPath: "Concepts/go/Target.md", Status: "growing"}}}},
	}}
	if diff := cmp.Diff(wantPaths, model.Paths); diff != "" {
		t.Errorf("Build Paths mismatch (-want +got):\n%s", diff)
	}
	wantMaps := []Map{
		{Title: "Alpha", RelPath: "Maps/Z-alpha.md", Domain: "golang", Type: "source-map", Branches: wantPaths[0].Branches},
		{Title: "Zeta", RelPath: "Maps/A-zeta.md", Domain: "golang", Type: "moc", Branches: wantPaths[0].Branches},
		{Title: "Beta", RelPath: "Maps/Beta.md", Domain: "japanese", Type: "topic-map", Branches: wantPaths[0].Branches},
	}
	if diff := cmp.Diff(wantMaps, model.Maps); diff != "" {
		t.Errorf("Build Maps mismatch (-want +got):\n%s", diff)
	}

	wantPlacements := []Placement{
		{MapRelPath: "Maps/Course.md", Headings: []string{"Shelf"}},
		{MapRelPath: "Maps/Z-alpha.md", Headings: []string{"Shelf"}},
		{MapRelPath: "Maps/A-zeta.md", Headings: []string{"Shelf"}},
		{MapRelPath: "Maps/Beta.md", Headings: []string{"Shelf"}},
	}
	if diff := cmp.Diff(wantPlacements, model.Placements("Concepts/go/Target.md")); diff != "" {
		t.Errorf("Placements(Target) mismatch (-want +got):\n%s", diff)
	}
	if got := model.Placements("Ghost"); len(got) != 0 {
		t.Errorf("Placements(Ghost) = %v, want empty", got)
	}
}

func TestBuildJournalFromScannerMtimes(t *testing.T) {
	t.Parallel()

	t.Run("newest five without frontmatter", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		mtimes := make(map[string]time.Time)
		for day := 1; day <= 7; day++ {
			rel := fmt.Sprintf("Diary/2026-07-%02d.md", day)
			writeNavFixture(t, root, rel, fmt.Sprintf("# Day %d\n", day))
			mtimes[rel] = time.Date(2026, time.July, day, 8, 0, 0, 0, time.UTC)
		}
		model, err := Build(root, resolver(t), mtimes)
		if err != nil {
			t.Fatalf("Build: %v", err)
		}
		want := []JournalEntry{
			{Title: "2026-07-07", RelPath: "Diary/2026-07-07.md", Modified: mtimes["Diary/2026-07-07.md"]},
			{Title: "2026-07-06", RelPath: "Diary/2026-07-06.md", Modified: mtimes["Diary/2026-07-06.md"]},
			{Title: "2026-07-05", RelPath: "Diary/2026-07-05.md", Modified: mtimes["Diary/2026-07-05.md"]},
			{Title: "2026-07-04", RelPath: "Diary/2026-07-04.md", Modified: mtimes["Diary/2026-07-04.md"]},
			{Title: "2026-07-03", RelPath: "Diary/2026-07-03.md", Modified: mtimes["Diary/2026-07-03.md"]},
		}
		if diff := cmp.Diff(want, model.Journal); diff != "" {
			t.Errorf("Build Journal mismatch (-want +got):\n%s", diff)
		}
	})

	for _, name := range []string{"absent", "empty"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writeNavFixture(t, root, "Notes/ordinary.md", "ordinary note\n")
			if name == "empty" {
				if err := os.Mkdir(filepath.Join(root, "Diary"), 0o750); err != nil {
					t.Fatalf("mkdir Diary: %v", err)
				}
			}
			model, err := Build(root, resolver(t), nil)
			if err != nil {
				t.Fatalf("Build: %v", err)
			}
			if len(model.Journal) != 0 {
				t.Errorf("Build Journal = %v, want empty", model.Journal)
			}
		})
	}
}

// TestBuildCarriesScannerMtimes proves Home's freshness data comes from the
// scanner-owned capture handed to Build, not from another stat inside nav or a
// request handler. The recorded time deliberately differs from the file's real
// mtime, so reading the disk through a second path makes this test fail.
func TestBuildCarriesScannerMtimes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	rel := "Concepts/go/Channels.md"
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := "---\ntitle: Channels\ntype: concept\nstatus: growing\n---\n\nbody\n"
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write note: %v", err)
	}

	captured := time.Date(2026, time.July, 9, 14, 30, 0, 0, time.UTC)
	model, err := Build(root, resolver(t, rel), map[string]time.Time{rel: captured})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	want := []NoteSummary{{
		Title:    "Channels",
		RelPath:  rel,
		Type:     "concept",
		Status:   "growing",
		Modified: captured,
	}}
	if diff := cmp.Diff(want, model.KnowledgeNotes); diff != "" {
		t.Errorf("Build KnowledgeNotes mismatch (-want +got):\n%s", diff)
	}
}

// resolver builds a real *graph.Index (testing.md's "real first") from
// in-memory note paths — no disk — so parseBranches resolves entry
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

// TestParseBranchesGoShape covers the pipe-format Go map shape: H2/H3
// headings "slug | English | Chinese" (the English column becomes the
// label) and "- [[Entry]]" bullets, with prose lines (範圍/★) between
// headings that must NOT become entries, and a trailing part with no
// entries that must be pruned away.
func TestParseBranchesGoShape(t *testing.T) {
	t.Parallel()

	idx := resolver(t, "L/Entry A.md", "L/Entry B.md", "L/Entry C.md")
	statusByPath := map[string]string{
		"L/Entry A.md": "draft",
		"L/Entry B.md": schema.SealStatus,
		// Entry C intentionally absent -> empty status badge.
	}

	body := "> intro blockquote citing [[Entry A]] — a quote, not a bullet\n" +
		"\n" +
		"## data-and-hardware | Data and the Hardware | 資料與硬體本質\n" +
		"\n" +
		"範圍:this is prose, not a list item, and it names [[Entry B]].\n" +
		"★ 待補模組:bits-bytes-words (prose only).\n" +
		"\n" +
		"### text-as-bytes | Text as Bytes | 文字即 bytes\n" +
		"\n" +
		"- [[Entry A]]\n" +
		"- [[Entry B]]\n" +
		"\n" +
		"### alignment | Alignment | 對齊\n" +
		"\n" +
		"- [[Entry C]]\n" +
		"\n" +
		"## empty-part | Empty Part | 空帶\n" +
		"\n" +
		"範圍:prose only, no entries anywhere under this part.\n"

	want := []Branch{
		{
			Heading: "Data and the Hardware",
			Level:   2,
			Sub: []Branch{
				{
					Heading: "Text as Bytes",
					Level:   3,
					Entries: []Entry{
						{Text: "Entry A", Target: "Entry A", RelPath: "L/Entry A.md", Status: "draft"},
						{Text: "Entry B", Target: "Entry B", RelPath: "L/Entry B.md", Status: schema.SealStatus},
					},
				},
				{
					Heading: "Alignment",
					Level:   3,
					Entries: []Entry{
						{Text: "Entry C", Target: "Entry C", RelPath: "L/Entry C.md"},
					},
				},
			},
		},
	}

	got := parseBranches(body, idx, statusByPath)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("parseBranches (Go shape) mismatch (-want +got):\n%s", diff)
	}
}

// TestParseBranchesMinnaShape covers the 大家 shape: only the
// course-sequence branch holds an entry tree; the daily-loop (ordered
// list), learning levels (a table), and gaps (task checkboxes) branches
// carry no entry bullets and must prune away — even the gap task item that
// contains a [[wikilink]], and even the loop's ordered item that contains
// one. Entry bullets are "- **Lx** ... · [[Lxx]]"; a "待建" bullet with no
// wikilink is not an entry. The H1 title is ignored.
func TestParseBranchesMinnaShape(t *testing.T) {
	t.Parallel()

	idx := resolver(t, "jp/L01 Intro.md", "jp/L02 Next.md", "jp/L03 Verbs.md")
	statusByPath := map[string]string{
		"jp/L01 Intro.md": "draft",
		"jp/L02 Next.md":  "draft",
		"jp/L03 Verbs.md": schema.SealStatus,
	}

	body := "# Doc Title (an H1, ignored)\n" +
		"\n" +
		"> [!info] callout prose that mentions [[Loop Link]]\n" +
		"\n" +
		"## Daily loop\n" +
		"\n" +
		"1. **step** in an ORDERED item, links [[Loop Link]] — not an entry.\n" +
		"2. another step.\n" +
		"\n" +
		"## Learning levels (a table)\n" +
		"\n" +
		"| level | 課 |\n" +
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

	want := []Branch{
		{
			Heading: "Course sequence (order = lines)",
			Level:   2,
			Sub: []Branch{
				{
					Heading: "Decode",
					Level:   3,
					Entries: []Entry{
						{Text: "L01 Intro", Target: "L01 Intro", RelPath: "jp/L01 Intro.md", Status: "draft"},
						{Text: "L02 Next", Target: "L02 Next", RelPath: "jp/L02 Next.md", Status: "draft"},
					},
				},
				{
					Heading: "Verbs",
					Level:   3,
					Entries: []Entry{
						{Text: "L03 Verbs", Target: "L03 Verbs", RelPath: "jp/L03 Verbs.md", Status: schema.SealStatus},
					},
				},
			},
		},
	}

	got := parseBranches(body, idx, statusByPath)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("parseBranches (大家 shape) mismatch (-want +got):\n%s", diff)
	}
}

// TestParseBranchesFaultTolerance proves unresolved and ambiguous rows stay out
// of navigation while a uniquely resolved neighbor remains, and malformed
// heading/list lines are ignored without panicking.
func TestParseBranchesFaultTolerance(t *testing.T) {
	t.Parallel()

	// "Dup" resolves to two files -> Ambiguous with sorted candidates.
	idx := resolver(t, "A/Dup.md", "B/Dup.md", "ok/Real.md")

	body := "## part | Part | 部\n" +
		"\n" +
		"###notaspace this is not a heading\n" +
		"#### \n" + // empty-label deeper heading, no entries -> pruned
		"- \n" + // bare bullet, no wikilink -> not an entry
		"- [ ] a task with a [[Real]] link -> excluded\n" +
		"- no wikilink here at all\n" +
		"\n" +
		"### mod | Module | 模組\n" +
		"\n" +
		"- [[Real]]\n" +
		"- [[Ghost]]\n" + // unresolved: stays on the map note, absent here
		"- [[Dup]]\n" // ambiguous: stays on the map note, absent here

	want := []Branch{
		{
			Heading: "Part",
			Level:   2,
			Sub: []Branch{
				{
					Heading: "Module",
					Level:   3,
					Entries: []Entry{{Text: "Real", Target: "Real", RelPath: "ok/Real.md"}},
				},
			},
		},
	}

	got := parseBranches(body, idx, map[string]string{})
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("parseBranches (fault tolerance) mismatch (-want +got):\n%s", diff)
	}
}

// TestParseHeading and TestParseEntryItem lock down the two line
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
		{name: "not a heading", line: "- [[Entry]]", wantOK: false},
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

func TestParseEntryItem(t *testing.T) {
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
			inner, ok := parseEntryItem(tt.line)
			if ok != tt.wantOK || inner != tt.wantInner {
				t.Errorf("parseEntryItem(%q) = (%q, %t), want (%q, %t)",
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
		"Writing/entries/golang/Entry.md",
	}

	wantFolders := []Folder{
		{Name: "Inbox", RelPath: "Inbox", Notes: []NoteRef{{Name: "note", RelPath: "Inbox/note.md"}}},
		{Name: "Concepts", RelPath: "Concepts", Sub: []Folder{
			{Name: "golang", RelPath: "Concepts/golang", Notes: []NoteRef{{Name: "Foo", RelPath: "Concepts/golang/Foo.md"}}},
			{Name: "rust", RelPath: "Concepts/rust", Notes: []NoteRef{{Name: "Bar", RelPath: "Concepts/rust/Bar.md"}}},
		}},
		{Name: "Writing", RelPath: "Writing", Sub: []Folder{
			{Name: "entries", RelPath: "Writing/entries", Sub: []Folder{
				{Name: "golang", RelPath: "Writing/entries/golang", Notes: []NoteRef{{Name: "Entry", RelPath: "Writing/entries/golang/Entry.md"}}},
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

// TestPlacements inverts the map trees into the note -> placements map the
// sidebar reads to open itself to the current note. It covers the two cases that
// make the inversion worth building: a note listed by two different study-paths
// (two placements, one per path), and a note listed by two branches of the same
// study-path (two placements, each carrying its own heading chain). An
// unresolved entry carries no path and must not appear.
func TestPlacements(t *testing.T) {
	t.Parallel()

	paths := []Map{
		{
			Title: "Go", RelPath: "Maps/go-path.md",
			Branches: []Branch{
				{Heading: "Part A", Level: 2, Sub: []Branch{
					{Heading: "Module 1", Level: 3, Entries: []Entry{
						{Text: "L1", Target: "L1", RelPath: "L/L1.md"},
						{Text: "Shared", Target: "Shared", RelPath: "L/Shared.md"},
					}},
				}},
				{Heading: "Part B", Level: 2, Entries: []Entry{
					{Text: "Shared", Target: "Shared", RelPath: "L/Shared.md"},
				}},
			},
		},
		{
			Title: "JP", RelPath: "Maps/jp-path.md",
			Branches: []Branch{
				{Heading: "Unit 1", Level: 2, Entries: []Entry{
					{Text: "L1", Target: "L1", RelPath: "L/L1.md"},
				}},
			},
		},
	}
	m := &Model{placementIndex: buildPlacementIndex(paths)}

	tests := []struct {
		name    string
		relPath string
		want    []Placement
	}{
		{
			name:    "listed by two study-paths",
			relPath: "L/L1.md",
			want: []Placement{
				{MapRelPath: "Maps/go-path.md", Headings: []string{"Part A", "Module 1"}},
				{MapRelPath: "Maps/jp-path.md", Headings: []string{"Unit 1"}},
			},
		},
		{
			name:    "listed by two branches of one study-path",
			relPath: "L/Shared.md",
			want: []Placement{
				{MapRelPath: "Maps/go-path.md", Headings: []string{"Part A", "Module 1"}},
				{MapRelPath: "Maps/go-path.md", Headings: []string{"Part B"}},
			},
		},
		{name: "unresolved entry never indexed", relPath: "Ghost", want: nil},
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
