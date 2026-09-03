package lexical

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// A reader typed the name of a folder standing in their own sidebar and was
// told there was nothing there. Three of them read that as the search being
// broken, and it is hard to argue: 微積分 was on screen while 微積分 returned
// nothing. Where a note lives is something its author wrote, so it belongs in
// what they can find the note by.
func TestFolderNamesFindTheNotesInThem(t *testing.T) {
	t.Parallel()
	idx := NewIndex([]Document{
		{RelPath: "微積分/第三週 極限與連續.md", Title: "第三週 極限與連續", PlainText: "epsilon-delta 一定會考"},
		{RelPath: "微積分/第四週 導數.md", Title: "第四週 導數", PlainText: "連結回第三週"},
		{RelPath: "普通心理學/記憶.md", Title: "記憶", PlainText: "記憶分成短期跟長期"},
	}, validArtifactPolicy(t))

	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{
			name:  "a folder name reaches everything under it",
			query: "微積分",
			want:  []string{"微積分/第三週 極限與連續.md", "微積分/第四週 導數.md"},
		},
		{
			name:  "so does a folder holding one note",
			query: "普通心理學",
			want:  []string{"普通心理學/記憶.md"},
		},
		{
			name:  "a word in the text still finds it the way it always did",
			query: "記憶",
			want:  []string{"普通心理學/記憶.md"},
		},
		{
			name:  "a name in neither the path nor the text still finds nothing",
			query: "統計學",
			want:  nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			results, _, err := idx.SearchN(Parse(tt.query), -1)
			if err != nil {
				t.Fatalf("Search(%q) error = %v", tt.query, err)
			}
			got := make([]string, 0, len(results))
			for _, r := range results {
				got = append(got, r.RelPath)
			}
			if len(got) == 0 {
				got = nil
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Search(%q) mismatch (-want +got):\n%s", tt.query, diff)
			}
		})
	}
}

// Widening what can be found must not move what could be found already. A note
// whose words match is the better answer than one that merely sits in a folder
// of that name, so path hits are appended after every other group rather than
// mixed into them.
func TestPathHitsAreAppendedAfterTextHits(t *testing.T) {
	t.Parallel()
	idx := NewIndex([]Document{
		// Sorts first by path, so if path hits were not held back it would lead.
		{RelPath: "A索引/note.md", Title: "note", PlainText: "無關內容"},
		// Sorts last by path, and is the one whose text actually says it.
		{RelPath: "Z筆記/深入.md", Title: "深入", PlainText: "這篇談索引的設計"},
	}, validArtifactPolicy(t))

	results, _, err := idx.SearchN(Parse("索引"), -1)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	got := make([]string, 0, len(results))
	for _, r := range results {
		got = append(got, r.RelPath)
	}
	want := []string{"Z筆記/深入.md", "A索引/note.md"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Search(\"索引\") order mismatch (-want +got):\n%s", diff)
	}
}
