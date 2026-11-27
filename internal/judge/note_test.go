package judge

import (
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseNote(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		content         string
		wantNoFM        bool
		wantBadFM       bool
		wantFrontmatter map[string]fmValue
	}{
		{
			name:     "no frontmatter block is legal",
			content:  "just a transcript\nno fence\n",
			wantNoFM: true,
		},
		{
			name:      "present but unparseable is flagged bad",
			content:   "---\nlang: \"unclosed\ndomain: x\n---\nbody\n",
			wantBadFM: true,
		},
		{
			name:    "scalars and a list are read raw",
			content: "---\nlang: X\ntype: concept\ntags:\n  - a\n  - b/c\n---\nbody\n",
			wantFrontmatter: map[string]fmValue{
				"lang": {scalar: "X", scalarIsString: true},
				"type": {scalar: "concept", scalarIsString: true},
				"tags": {list: []string{"a", "b/c"}, isList: true},
			},
		},
		{
			name:    "an explicit null reads as an empty scalar",
			content: "---\ndraft:\ntype: concept\n---\n",
			wantFrontmatter: map[string]fmValue{
				"draft": {scalar: ""},
				"type":  {scalar: "concept", scalarIsString: true},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parseNote("x.md", []byte(tt.content))
			if got.noFrontmatter != tt.wantNoFM {
				t.Errorf("parseNote(%q) noFrontmatter = %v, want %v", tt.content, got.noFrontmatter, tt.wantNoFM)
			}
			if got.badFrontmatter != tt.wantBadFM {
				t.Errorf("parseNote(%q) badFrontmatter = %v, want %v", tt.content, got.badFrontmatter, tt.wantBadFM)
			}
			if diff := cmp.Diff(tt.wantFrontmatter, got.frontmatter, cmp.AllowUnexported(fmValue{})); diff != "" {
				t.Errorf("parseNote(%q) frontmatter mismatch (-want +got):\n%s", tt.content, diff)
			}
		})
	}
}

// TestCollectNotesScanBoundary pins what the diagnostics take from a scan:
// only Markdown files become notes, an image being a resource, and a private
// directory is read like any other, since what is withheld is decided after
// the read. Which entries the scan visits is vault's.
func TestCollectNotesScanBoundary(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestContract(t, root, nil)
	write(t, root, "notes/keep.md", "---\ntype: concept\n---\n")
	write(t, root, "notes/image.png", "not markdown, a linkable resource\n")
	write(t, root, "Diary/2026-07-04.md", "a day, scanned like any other note\n")

	notes, err := collectNotes(t.Context(), root)
	if err != nil {
		t.Fatalf("collectNotes(%q) = %v", root, err)
	}
	var got []string
	for _, n := range notes {
		got = append(got, n.path)
	}
	slices.Sort(got)
	if diff := cmp.Diff([]string{"Diary/2026-07-04.md", "notes/keep.md"}, got); diff != "" {
		t.Errorf("collectNotes(%q) note paths mismatch (-want +got):\n%s", root, diff)
	}
}
