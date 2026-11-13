package graph

import "testing"

func TestResolvesOnlyByAlias(t *testing.T) {
	t.Parallel()

	idx := BuildFromNotes([]NoteInput{
		{RelPath: "Notes/Successor.md", Aliases: []string{"Old name", "Notes/Gone"}},
		{RelPath: "Notes/Plain.md"},
		{RelPath: "Notes/Echo.md", Aliases: []string{"Echo"}},
		{RelPath: "Notes/Left.md", Aliases: []string{"twin"}},
		{RelPath: "Notes/Right.md", Aliases: []string{"twin"}},
	}, []string{"assets/Old name.png"})

	tests := []struct {
		name    string
		target  string
		want    string
		wantHit bool
	}{
		{"an alias no file is named", "Old name", "Notes/Successor.md", true},
		{"an alias folds like any name", "old NAME", "Notes/Successor.md", true},
		{"an alias spelled as a path no file has", "Notes/Gone", "Notes/Successor.md", true},
		{"a file's own name", "Plain", "", false},
		{"a path to a file", "Notes/Plain", "", false},
		{"an alias that repeats the file's own name", "Echo", "", false},
		{"a name nothing answers to", "Nowhere", "", false},
		{"an alias two notes share", "twin", "", false},
		{"a name a resource is called by", "Old name.png", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := idx.ResolvesOnlyByAlias(tt.target)
			if ok != tt.wantHit || got != tt.want {
				t.Errorf("ResolvesOnlyByAlias(%q) = %q, %v, want %q, %v", tt.target, got, ok, tt.want, tt.wantHit)
			}
		})
	}
}
