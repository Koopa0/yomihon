package judge

import "testing"

// TestExistsCountsNotesRatherThanMatches holds the number the human answer
// states to the notes that answered, not the rows: a note can answer to a
// name more than once, by its filename and its title or a title and an alias.
func TestExistsCountsNotesRatherThanMatches(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name    string
		matches []existsMatch
		want    int
	}{
		{
			name:    "one note answering twice",
			matches: []existsMatch{{Path: "A.md", Field: "filename"}, {Path: "A.md", Field: "title"}},
			want:    1,
		},
		{
			name:    "two notes, one of them answering twice",
			matches: []existsMatch{{Path: "A.md", Field: "filename"}, {Path: "A.md", Field: "alias"}, {Path: "B.md", Field: "title"}},
			want:    2,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := notesAmong(tt.matches); got != tt.want {
				t.Errorf("notesAmong(%v) = %d, want %d", tt.matches, got, tt.want)
			}
		})
	}
}
