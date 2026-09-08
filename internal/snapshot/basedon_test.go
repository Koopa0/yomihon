package snapshot

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/vault"
)

func TestProjectBasedOn(t *testing.T) {
	t.Parallel()

	notes := []*vault.Note{
		parse(t, "Book notes.md", "the notebook\n"),
		parse(t, "Concepts/Source model.md", "---\nbased_on: \"[[Book notes]]\"\n---\n\nno body link\n"),
		parse(t, "Concepts/Bare name.md", "---\nbased_on:\n  - Book notes\n---\n\nno body link\n"),
		parse(t, "Concepts/Ambiguous.md", "---\nbased_on: \"[[twin]]\"\n---\n\nno body link\n"),
		parse(t, "A/twin.md", "one\n"),
		parse(t, "B/twin.md", "two\n"),
		parse(t, "Concepts/Missing.md", "---\nbased_on: \"[[nowhere]]\"\n---\n\nno body link\n"),
		parse(t, "Concepts/Display.md", "---\nbased_on: \"[[Book notes#heading|shown]]\"\n---\n\nno body link\n"),
	}
	byPath := make(map[string]*vault.Note, len(notes))
	for _, n := range notes {
		byPath[n.RelPath] = n
	}
	idx := graph.New(notes, nil)

	tests := []struct {
		name    string
		relPath string
		want    []nav.NoteRef
	}{
		{
			name:    "a unique wikilink is a title and a path",
			relPath: "Concepts/Source model.md",
			want:    []nav.NoteRef{{Name: "Book notes", RelPath: "Book notes.md"}},
		},
		{
			name:    "a unique bare name is a title and a path",
			relPath: "Concepts/Bare name.md",
			want:    []nav.NoteRef{{Name: "Book notes", RelPath: "Book notes.md"}},
		},
		{
			name:    "an ambiguous value stays the author's text",
			relPath: "Concepts/Ambiguous.md",
			want:    []nav.NoteRef{{Name: "[[twin]]"}},
		},
		{
			name:    "an unresolved value stays the author's text",
			relPath: "Concepts/Missing.md",
			want:    []nav.NoteRef{{Name: "[[nowhere]]"}},
		},
		{
			name:    "display and heading suffixes strip before resolve",
			relPath: "Concepts/Display.md",
			want:    []nav.NoteRef{{Name: "Book notes", RelPath: "Book notes.md"}},
		},
		{
			name:    "a note that declared nothing has no sources",
			relPath: "Book notes.md",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := projectBasedOn(byPath[tt.relPath], idx)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("projectBasedOn(%q) mismatch (-want +got):\n%s", tt.relPath, diff)
			}
		})
	}
}

func TestBasedOnDoesNotCountAsABodyBacklink(t *testing.T) {
	t.Parallel()
	notes := []*vault.Note{
		parse(t, "Concepts/source.md", "the source\n"),
		parse(t, "Concepts/derived.md", "---\nbased_on: \"[[source]]\"\n---\n\nno body link\n"),
	}
	idx := graph.New(notes, nil)
	if refs := newBacklinks(notes, idx).To("Concepts/source.md"); len(refs) != 0 {
		t.Errorf("a based_on declaration counted as a body citation: %v", refs)
	}
}
