package pages

import (
	"bytes"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/ui/layouts"
)

func TestSyllabusUsesTheBranchTreeAsItsHeadingOutline(t *testing.T) {
	t.Parallel()

	view := PathView{
		Title: "Path",
		Branches: []PathBranchView{{
			Anchor: "part-1", Ordinal: "I", Heading: "Part", Depth: 0,
			Sub: []PathBranchView{{
				Num: 1, Heading: "Module", Depth: 1,
				Sub: []PathBranchView{{
					Num: 2, Heading: "Unit", Depth: 2,
					Sub: []PathBranchView{{Num: 3, Heading: "Topic", Depth: 3}},
				}},
			}},
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

	path := nav.Map{
		Title:   "Go path",
		RelPath: "Maps/Go path.md",
		Branches: []nav.Branch{
			{
				Heading: "Data", Level: 2,
				Subbranches: []nav.Branch{
					{
						Heading: "Text", Level: 3,
						Entries: []nav.Entry{
							{Text: "Slices", RelPath: "Writing/Slices.md", Status: schema.SealStatus},
							{Text: "Arrays", RelPath: "Writing/Arrays.md", Status: "draft"},
						},
					},
				},
			},
			{
				Heading: "Memory", Level: 2,
				Entries: []nav.Entry{
					{Text: "GC", RelPath: "Writing/GC.md", Status: schema.SealStatus},
					{Text: "Template", Target: "Template", Kind: nav.EntryNonInstance},
					{Text: "Unwritten", Target: "Unwritten", Kind: nav.EntryUnresolved},
				},
			},
		},
	}

	got := BuildPathView(&path, []nav.Map{path})

	want := PathView{
		Title:      "Go path",
		RelPath:    "Maps/Go path.md",
		GuideHref:  "/notes/Maps/Go%20path.md",
		SealTarget: schema.SealStatus,
		Paths: []PathLink{
			{Title: "Go path", RelPath: "Maps/Go path.md", Entries: 5, Active: true},
		},
		Parts:   2,
		Modules: 1, // only "Data" has a sub-branch; "Memory" holds an entry directly
		Entries: 5,
		Ready:   2, // Slices + GC sit at the seal; Arrays is draft
		Branches: []PathBranchView{
			{
				Anchor: "part-1", Ordinal: "I", Num: 1, Heading: "Data", Depth: 0,
				Total: 2,
				Sub: []PathBranchView{
					{
						Num: 1, Heading: "Text", Depth: 1, Total: 2,
						Entries: []PathEntryView{
							{Text: "Slices", Href: "/notes/Writing/Slices.md", Status: schema.SealStatus, Sealed: true},
							{Text: "Arrays", Href: "/notes/Writing/Arrays.md", Status: "draft"},
						},
					},
				},
			},
			{
				Anchor: "part-2", Ordinal: "II", Num: 2, Heading: "Memory", Depth: 0,
				Total: 3,
				Entries: []PathEntryView{
					{Text: "GC", Href: "/notes/Writing/GC.md", Status: schema.SealStatus, Sealed: true},
					{Text: "Template", Kind: nav.EntryNonInstance},
					{Text: "Unwritten", Kind: nav.EntryUnresolved},
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
