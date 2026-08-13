package pages

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/vault"
)

func TestSyllabusUsesTheBranchTreeAsItsHeadingOutline(t *testing.T) {
	t.Parallel()

	view := PathView{
		Title: "Path",
		Branches: []PathBranchView{{
			Anchor: "part-1", Ordinal: "I", Heading: "Part", Depth: 0,
			Items: []PathItemView{{Branch: &PathBranchView{
				Num: 1, Heading: "Module", Depth: 1,
				Items: []PathItemView{{Branch: &PathBranchView{
					Num: 2, Heading: "Unit", Depth: 2,
					Items: []PathItemView{{Branch: &PathBranchView{Num: 3, Heading: "Topic", Depth: 3}}},
				}}},
			}}},
		}},
	}

	var out bytes.Buffer
	if err := Syllabus(view, layouts.Chrome{}).Render(t.Context(), &out); err != nil {
		t.Fatalf("render syllabus: %v", err)
	}
	html := out.String()
	for _, want := range []string{
		`<h1 class="y-title">Path</h1>`,
		`<h2 class="y-part" id="part-1">`,
		`<h3 class="y-module__label">`,
		`<h4 class="y-module__label">`,
		`<h5 class="y-module__label">`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("syllabus heading outline missing %q; html = %q", want, html)
		}
	}
	if strings.Contains(html, `<div class="y-part"`) || strings.Contains(html, `<div class="y-module__label"`) {
		t.Errorf("syllabus renders visual-only branch headings; html = %q", html)
	}
}

// TestBuildPathView pins the pure transform's contract: a branch's total counts
// linked and warning rows, Ready counts only sealed resolved entries, and
// document order is preserved at every level. The fixture is two parts, one
// with a module and one with entries attached directly (no module), so the
// tallies and the modules count are hand-derivable and non-tautological.
func TestBuildPathView(t *testing.T) {
	t.Parallel()

	// The course is written the way an author writes one and read through the
	// declared-sequence grammar, so the page is asserted against the same
	// interpretation navigation uses — not a hand-built tree that could agree
	// with a broken reading.
	body := "## Data {sequence=primary}\n\n" +
		"### Text {sequence=primary}\n\n" +
		"- [[Slices]]\n" +
		"- [[Arrays]]\n" +
		"\n## Memory {sequence=primary}\n\n" +
		"- [[GC]]\n" +
		"\t- 選修 {sequence=local}\n" +
		"\t\t- [[Tuning]]\n" +
		"- [[Template]]\n" +
		"- [[Unwritten]]\n"
	path := buildTestPath(t, body)

	got := BuildPathView(&path, []nav.Path{path})

	want := PathView{
		Title:      "Go path",
		RelPath:    "Maps/Go path.md",
		GuideHref:  "/notes/Maps/Go%20path.md",
		SealTarget: schema.SealStatus,
		Paths: []PathLink{
			// The side branch is not part of the main line, so the planned
			// total is five, not six.
			{Title: "Go path", RelPath: "Maps/Go path.md", Entries: 5, Active: true},
		},
		Parts:   2,
		Modules: 2, // the "Text" module under Data, and the side branch under GC
		Entries: 5,
		Ready:   2, // Slices + GC sit at the seal; Arrays is draft
		Branches: []PathBranchView{
			{
				Anchor: "part-1", Ordinal: "I", Num: 1, Heading: "Data", Depth: 0,
				Items: []PathItemView{{Branch: &PathBranchView{
					Num: 1, Heading: "Text", Depth: 1, Total: 2,
					Items: []PathItemView{
						{Entry: &PathEntryView{Text: "Slices", Href: "/notes/Writing/Slices.md", Status: schema.SealStatus, Sealed: true}},
						{Entry: &PathEntryView{Text: "Arrays", Href: "/notes/Writing/Arrays.md", Status: "draft"}},
					},
				}}},
			},
			{
				Anchor: "part-2", Ordinal: "II", Num: 2, Heading: "Memory", Depth: 0,
				Total: 3,
				Items: []PathItemView{
					{Entry: &PathEntryView{Text: "GC", Href: "/notes/Writing/GC.md", Status: schema.SealStatus, Sealed: true}},
					// The side branch is drawn where the author put it: under
					// the lesson it hangs from, before the next main lesson.
					{Branch: &PathBranchView{
						Num: 1, Heading: "選修", Depth: 1, Local: true, Total: 1,
						Items: []PathItemView{
							{Entry: &PathEntryView{Text: "Tuning", Href: "/notes/Writing/Tuning.md", Status: "draft"}},
						},
					}},
					{Entry: &PathEntryView{Text: "Template", Kind: nav.EntryNonInstance}},
					{Entry: &PathEntryView{Text: "Unwritten", Kind: nav.EntryUnresolved}},
				},
			},
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("BuildPathView() mismatch (-want +got):\n%s", diff)
	}
}

func TestRoman(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   int
		want string
	}{
		{name: "one", in: 1, want: "I"},
		{name: "four", in: 4, want: "IV"},
		{name: "eight", in: 8, want: "VIII"},
		{name: "nine", in: 9, want: "IX"},
		{name: "twelve", in: 12, want: "XII"},
		{name: "non-positive falls back to decimal", in: 0, want: "0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := roman(tt.in); got != tt.want {
				t.Errorf("roman(%d) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// buildTestPath writes a small vault holding one study path and the lessons it
// names, then reads it back through the real navigation build. The page is
// asserted against the interpretation the product actually produces, so a test
// cannot agree with a projection the product would never make.
func buildTestPath(t *testing.T, body string) nav.Path {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"Maps/Go path.md":              "---\ntitle: Go path\ntype: study-path\ndomain: golang\n---\n\n" + body,
		"Writing/Slices.md":            "---\ntitle: Slices\ntype: lesson\nstatus: " + schema.SealStatus + "\n---\nbody\n",
		"Writing/Arrays.md":            "---\ntitle: Arrays\ntype: lesson\nstatus: draft\n---\nbody\n",
		"Writing/GC.md":                "---\ntitle: GC\ntype: lesson\nstatus: " + schema.SealStatus + "\n---\nbody\n",
		"Writing/Tuning.md":            "---\ntitle: Tuning\ntype: lesson\nstatus: draft\n---\nbody\n",
		"System/templates/Template.md": "---\ntitle: Template\ntype: lesson\nstatus: draft\n---\nbody\n",
	}
	for rel, content := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Errorf("Reader.Close() error = %v", closeErr)
		}
	})
	scan, err := reader.ScanComplete(t.Context())
	if err != nil {
		t.Fatalf("ScanComplete() error = %v", err)
	}
	notes := make(map[string]*vault.Note)
	noteList := make([]*vault.Note, 0, len(scan.Files()))
	for _, entry := range scan.Files() {
		data, readErr := reader.ReadFile(t.Context(), entry)
		if readErr != nil {
			t.Fatalf("ReadFile() error = %v", readErr)
		}
		note := vault.Parse(entry.Path(), data)
		notes[entry.Path()] = note
		noteList = append(noteList, note)
	}
	contract, err := schema.LoadFile(filepath.Join("..", "..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("schema.LoadFile = %v", err)
	}
	model := nav.New(
		scan.Files(), notes, graph.New(noteList, nil),
		contract.NavigationRoles(), contract.KnowledgeScope(), contract.ArtifactPolicy(),
	)
	paths := model.Paths()
	if len(paths) != 1 {
		t.Fatalf("fixture produced %d study paths, want 1", len(paths))
	}
	return paths[0]
}
