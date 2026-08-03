package nav

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// The syllabus is written forward: planned lessons sit in the list as warning
// rows long before they exist. Stepping through the course must move between
// lessons the reader can open, so a warning row keeps its place on the page
// and contributes no stop — and a note two paths both teach gets one answer
// per path, never a guess about which course the reader is walking.
func TestNeighbors(t *testing.T) {
	t.Parallel()

	ref := func(name, rel string) NoteRef { return NoteRef{Name: name, RelPath: rel} }
	entry := func(name, rel string) Entry {
		return Entry{Text: name, RelPath: rel, Kind: EntryResolved}
	}
	goPath := Map{
		Title:   "Go 課綱",
		RelPath: "Maps/go.md",
		Type:    "study-path",
		Branches: []Branch{{
			Heading: "Data", Level: 2,
			Entries: []Entry{
				entry("L01 Slices", "Writing/lessons/golang/Slices.md"),
				{Text: "L02 Planned", Kind: EntryUnresolved},
				entry("L03 Maps", "Writing/lessons/golang/Maps.md"),
			},
			Subbranches: []Branch{{
				Heading: "Text", Level: 3,
				Entries: []Entry{
					{Text: "Template-only", Kind: EntryNonInstance},
					entry("L04 Strings", "Writing/lessons/golang/Strings.md"),
				},
			}},
		}},
	}
	jpPath := Map{
		Title:   "日本語 學習路徑",
		RelPath: "Maps/jp.md",
		Type:    "study-path",
		Branches: []Branch{{
			Heading: "初級", Level: 2,
			Entries: []Entry{
				entry("L01 て形", "Writing/lessons/japanese/Te.md"),
				entry("L01 Slices", "Writing/lessons/golang/Slices.md"),
			},
		}},
	}
	m := &Model{paths: []Map{goPath, jpPath}}

	tests := []struct {
		name    string
		relPath string
		want    []Neighbors
	}{
		{
			name:    "a planned lesson between two written ones is not a stop",
			relPath: "Writing/lessons/golang/Maps.md",
			want: []Neighbors{{
				PathTitle: "Go 課綱", PathRelPath: "Maps/go.md",
				// L02 is unresolved, so the step back lands on L01, and the
				// step forward crosses into the subbranch, skipping the
				// template row the same way.
				Prev: ref("L01 Slices", "Writing/lessons/golang/Slices.md"),
				Next: ref("L04 Strings", "Writing/lessons/golang/Strings.md"),
			}},
		},
		{
			name:    "the opening lesson of a path has no previous",
			relPath: "Writing/lessons/japanese/Te.md",
			want: []Neighbors{{
				PathTitle: "日本語 學習路徑", PathRelPath: "Maps/jp.md",
				Next: ref("L01 Slices", "Writing/lessons/golang/Slices.md"),
			}},
		},
		{
			name:    "the closing lesson of a path has no next",
			relPath: "Writing/lessons/golang/Strings.md",
			want: []Neighbors{{
				PathTitle: "Go 課綱", PathRelPath: "Maps/go.md",
				Prev: ref("L03 Maps", "Writing/lessons/golang/Maps.md"),
			}},
		},
		{
			name:    "a lesson two paths teach answers once per path",
			relPath: "Writing/lessons/golang/Slices.md",
			want: []Neighbors{
				{
					PathTitle: "Go 課綱", PathRelPath: "Maps/go.md",
					Next: ref("L03 Maps", "Writing/lessons/golang/Maps.md"),
				},
				{
					PathTitle: "日本語 學習路徑", PathRelPath: "Maps/jp.md",
					Prev: ref("L01 て形", "Writing/lessons/japanese/Te.md"),
				},
			},
		},
		{
			name:    "a note no path teaches has no answer",
			relPath: "Concepts/plain.md",
			want:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := m.Neighbors(tt.relPath)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Neighbors(%q) mismatch (-want +got):\n%s", tt.relPath, diff)
			}
		})
	}
}
