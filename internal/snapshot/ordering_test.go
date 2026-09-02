package snapshot

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

// A reader who numbered their notes reads them in that order, and every list a
// generation publishes has to agree about it. Comparing code points puts the
// tenth lesson between the first and the second, so the sidebar showed the
// course in reading order while the backlinks panel, the island list and the
// shared-title answer showed the same lessons shuffled — with nothing on the
// page to say which of them was lying.
func TestEveryListSortsPathsTheWayTheirNumbersRead(t *testing.T) {
	t.Parallel()

	t.Run("the notes inside one island group", func(t *testing.T) {
		t.Parallel()
		notes := []*vault.Note{
			parse(t, "Sources/course/第10課.md", "a\n"),
			parse(t, "Sources/course/第9課.md", "b\n"),
		}
		idx := graph.New(notes, nil)
		h := newHealth(notes, idx, judge.NewPlanned(noteBodies(notes)), newBacklinks(notes, idx), schema.ArtifactPolicy{}, titlesByName(notes))

		want := []nav.NoteRef{
			{Name: "第9課", RelPath: "Sources/course/第9課.md"},
			{Name: "第10課", RelPath: "Sources/course/第10課.md"},
		}
		if len(h.Islands) != 1 {
			t.Fatalf("Islands = %+v, want one folder group", h.Islands)
		}
		if diff := cmp.Diff(want, h.Islands[0].Notes); diff != "" {
			t.Errorf("island group order mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("the notes citing one note", func(t *testing.T) {
		t.Parallel()
		notes := []*vault.Note{
			parse(t, "Concepts/target.md", "the cited note\n"),
			parse(t, "Writing/第10課.md", "see [[target]]\n"),
			parse(t, "Writing/第9課.md", "see [[target]]\n"),
		}
		idx := graph.New(notes, nil)

		want := []nav.NoteRef{
			{Name: "第9課", RelPath: "Writing/第9課.md"},
			{Name: "第10課", RelPath: "Writing/第10課.md"},
		}
		if diff := cmp.Diff(want, newBacklinks(notes, idx).To("Concepts/target.md")); diff != "" {
			t.Errorf("backlink order mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("the citations that land nowhere", func(t *testing.T) {
		t.Parallel()
		notes := []*vault.Note{
			parse(t, "Writing/第10課.md", "see [[no such name]]\n"),
			parse(t, "Writing/第9課.md", "see [[no such name]]\n"),
		}
		idx := graph.New(notes, nil)
		h := newHealth(notes, idx, judge.NewPlanned(noteBodies(notes)), newBacklinks(notes, idx), schema.ArtifactPolicy{}, titlesByName(notes))

		want := []string{"Writing/第9課.md", "Writing/第10課.md"}
		got := make([]string, 0, len(h.Unwritten))
		for _, link := range h.Unwritten {
			got = append(got, link.From.RelPath)
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("unwritten-citation order mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("the notes sharing one declared title", func(t *testing.T) {
		t.Parallel()
		notes := []*vault.Note{
			parse(t, "Writing/第10課.md", "---\ntitle: 課\n---\n\na\n"),
			parse(t, "Writing/第9課.md", "---\ntitle: 課\n---\n\nb\n"),
		}

		want := []nav.NoteRef{
			{Name: "第9課", RelPath: "Writing/第9課.md"},
			{Name: "第10課", RelPath: "Writing/第10課.md"},
		}
		if diff := cmp.Diff(want, titlesByName(notes)[graph.NormalizeKey("課")]); diff != "" {
			t.Errorf("title holder order mismatch (-want +got):\n%s", diff)
		}
	})
}
