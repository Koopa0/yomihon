package graph_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
)

func TestResolvePathSuffix(t *testing.T) {
	t.Parallel()
	idx := graph.BuildFromNotes([]graph.NoteInput{
		{RelPath: "Notes/Projects/Atlas/README.md"},
		{RelPath: "Notes/Other/README.md"},
		{RelPath: "README.md"},
	}, []string{"Assets/Atlas/chart.svg"})
	tests := []struct {
		name string
		want graph.Resolution
	}{
		{name: "Atlas/README", want: graph.Resolution{Kind: graph.KindUnique, RelPath: "Notes/Projects/Atlas/README.md"}},
		{name: "Projects/Atlas/README", want: graph.Resolution{Kind: graph.KindUnique, RelPath: "Notes/Projects/Atlas/README.md"}},
		{name: "Atlas/README.md", want: graph.Resolution{Kind: graph.KindUnique, RelPath: "Notes/Projects/Atlas/README.md"}},
		{name: " atlas/readme ", want: graph.Resolution{Kind: graph.KindUnique, RelPath: "Notes/Projects/Atlas/README.md"}},
		{name: "README", want: graph.Resolution{Kind: graph.KindAmbiguous, Candidates: []string{"Notes/Other/README.md", "Notes/Projects/Atlas/README.md", "README.md"}}},
		{name: "Nope/README", want: graph.Resolution{Kind: graph.KindUnresolved}},
		{name: "Atlas/chart.svg", want: graph.Resolution{Kind: graph.KindUnique, RelPath: "Assets/Atlas/chart.svg"}},
		{name: "Atlas/chart", want: graph.Resolution{Kind: graph.KindUnresolved}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := idx.Resolve(tt.name)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Resolve(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

func TestResolvePathSuffixSegmentBoundary(t *testing.T) {
	t.Parallel()
	idx := graph.BuildFromNotes([]graph.NoteInput{
		{RelPath: "Notes/NotAtlas/README.md"},
		{RelPath: "Notes/Atlas/MyREADME.md"},
	}, nil)
	for _, query := range []string{"Atlas/README", "las/README", "Atlas//MyREADME", "Atlas/../NotAtlas/README", `NotAtlas\README`, "NotAtlas/%52EADME"} {
		t.Run(query, func(t *testing.T) {
			t.Parallel()
			got := idx.Resolve(query)
			if diff := cmp.Diff(graph.Resolution{Kind: graph.KindUnresolved}, got); diff != "" {
				t.Errorf("Resolve(%q) mismatch (-want +got):\n%s", query, diff)
			}
		})
	}
}

func TestResolvePathSuffixAmbiguityNamesEachFileOnce(t *testing.T) {
	t.Parallel()
	idx := graph.BuildFromNotes([]graph.NoteInput{
		{RelPath: "Z/Atlas/README.md"},
		{RelPath: "A/Atlas/README.md"},
		{RelPath: "Z/Atlas/README.md"},
	}, []string{"A/Atlas/README.md"})
	want := graph.Resolution{Kind: graph.KindAmbiguous, Candidates: []string{"A/Atlas/README.md", "Z/Atlas/README.md"}}
	if diff := cmp.Diff(want, idx.Resolve("Atlas/README")); diff != "" {
		t.Errorf("Resolve(%q) mismatch (-want +got):\n%s", "Atlas/README", diff)
	}
}

func TestResolvePathSuffixExactPriority(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		notes []graph.NoteInput
		query string
		want  graph.Resolution
	}{
		{name: "unique alias", notes: []graph.NoteInput{{RelPath: "A.md", Aliases: []string{"Atlas/README"}}, {RelPath: "Notes/Atlas/README.md"}}, query: "Atlas/README", want: graph.Resolution{Kind: graph.KindUnique, RelPath: "A.md"}},
		{name: "ambiguous aliases", notes: []graph.NoteInput{{RelPath: "A.md", Aliases: []string{"Atlas/README"}}, {RelPath: "B.md", Aliases: []string{"Atlas/README"}}, {RelPath: "Notes/Atlas/README.md"}}, query: "Atlas/README", want: graph.Resolution{Kind: graph.KindAmbiguous, Candidates: []string{"A.md", "B.md"}}},
		{name: "full path wins", notes: []graph.NoteInput{{RelPath: "Atlas/README.md"}, {RelPath: "Notes/Atlas/README.md"}}, query: "Atlas/README", want: graph.Resolution{Kind: graph.KindUnique, RelPath: "Atlas/README.md"}},
		{name: "alias joins full key", notes: []graph.NoteInput{{RelPath: "Atlas/README.md"}, {RelPath: "A.md", Aliases: []string{"Atlas/README"}}, {RelPath: "Notes/Atlas/README.md"}}, query: "Atlas/README", want: graph.Resolution{Kind: graph.KindAmbiguous, Candidates: []string{"A.md", "Atlas/README.md"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := graph.BuildFromNotes(tt.notes, nil).Resolve(tt.query)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Resolve(%q) mismatch (-want +got):\n%s", tt.query, diff)
			}
		})
	}
}

func TestResolvePathSuffixPreservesWholeCollisionSet(t *testing.T) {
	t.Parallel()
	idx := graph.BuildFromNotes([]graph.NoteInput{{RelPath: "A/Atlas/README.md"}, {RelPath: "B/Atlas/README.md"}}, []string{"A/Assets/chart.svg", "B/Assets/chart.svg"})
	want := map[string][]string{
		"readme":    {"A/Atlas/README.md", "B/Atlas/README.md"},
		"readme.md": {"A/Atlas/README.md", "B/Atlas/README.md"},
		"chart.svg": {"A/Assets/chart.svg", "B/Assets/chart.svg"},
	}
	for _, query := range []string{"Atlas/README", "Assets/chart.svg", "README", "Nope/README"} {
		idx.Resolve(query)
	}
	if diff := cmp.Diff(want, idx.Collisions()); diff != "" {
		t.Errorf("Collisions() mismatch (-want +got):\n%s", diff)
	}
}
