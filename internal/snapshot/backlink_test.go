package snapshot

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/vault"
)

// Every trial reader who knew what a link graph is asked the same question and
// could not get it answered: standing on a note, who cites this? The resolver
// already held the edges — it could only report the broken ones. This reads
// them the other way.
func TestBacklinks(t *testing.T) {
	t.Parallel()

	notes := []*vault.Note{
		parse(t, "Concepts/target.md", "the cited note\n"),
		parse(t, "Concepts/citing one.md", "see [[target]] and [[target]] again\n"),
		parse(t, "Concepts/citing two.md", "also [[target|the other name]]\n"),
		parse(t, "Concepts/self.md", "I am [[self]]\n"),
		parse(t, "Concepts/quoted.md", "the syntax is `[[target]]` and\n```\n[[target]]\n```\n"),
		parse(t, "Concepts/island.md", "nothing points here\n"),
		parse(t, "Concepts/ambiguous cite.md", "see [[twin]]\n"),
		parse(t, "A/twin.md", "one\n"),
		parse(t, "B/twin.md", "two\n"),
	}
	idx := graph.New(notes, nil)
	b := newBacklinks(notes, idx)

	tests := []struct {
		name    string
		relPath string
		want    []nav.NoteRef
	}{
		{
			name:    "each citing note appears once, however many times it links",
			relPath: "Concepts/target.md",
			want: []nav.NoteRef{
				{Name: "citing one", RelPath: "Concepts/citing one.md"},
				{Name: "citing two", RelPath: "Concepts/citing two.md"},
			},
		},
		{
			name:    "a note citing itself is not told about it",
			relPath: "Concepts/self.md",
		},
		{
			name:    "a note nothing cites is an island",
			relPath: "Concepts/island.md",
		},
		{
			name:    "an ambiguous name records no edge against a guess",
			relPath: "A/twin.md",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if diff := cmp.Diff(tt.want, b.To(tt.relPath)); diff != "" {
				t.Errorf("To(%q) mismatch (-want +got):\n%s", tt.relPath, diff)
			}
		})
	}
}

func parse(t *testing.T, relPath, body string) *vault.Note {
	t.Helper()
	return vault.Parse(relPath, []byte(body))
}
