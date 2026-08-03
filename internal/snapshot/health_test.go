package snapshot

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/vault"
)

// A citation that lands nowhere has more than one reason and more than one
// repair, and this page is worse than useless if it names the wrong one. The
// title case was found by running the page against a real vault: three notes
// that exist were listed as unwritten, because a title is never a name this
// vault resolves by and nothing here had separated that from absence.
func TestHealthSeparatesTheReasonsACitationFails(t *testing.T) {
	t.Parallel()

	notes := []*vault.Note{
		parse(t, "Concepts/written.md", "---\ntitle: A Written Note\n---\n\nI exist\n"),
		parse(t, "Concepts/cites title.md", "---\ntitle: Cites Title\n---\n\nsee [[A Written Note]]\n"),
		parse(t, "Concepts/cites nothing.md", "---\ntitle: Cites Nothing\n---\n\nsee [[no such name]]\n"),
		parse(t, "Concepts/cites owed.md", "---\ntitle: Cites Owed\n---\n\nsee [[Owed Concept]]\n"),
		parse(t, "Maps/ledger.md", "---\ntitle: Ledger\n---\n\n## 缺口帳\n\n- Owed Concept\n"),
		// Two notes declaring one title: naming either of them would be the
		// guess the resolver refuses to make, so this citation stays unwritten
		// rather than being pointed at one of them.
		parse(t, "A/twin.md", "---\ntitle: Shared Title\n---\n\none\n"),
		parse(t, "B/twin.md", "---\ntitle: Shared Title\n---\n\ntwo\n"),
		parse(t, "Concepts/cites shared.md", "---\ntitle: Cites Shared\n---\n\nsee [[Shared Title]]\n"),
	}
	idx := graph.New(notes, nil)
	planned := judge.NewPlanned(noteBodies(notes))
	h := newHealth(notes, idx, planned, newBacklinks(notes, idx))

	wantTitleOnly := []HealthTitleLink{{
		From:   nav.NoteRef{Name: "cites title", RelPath: "Concepts/cites title.md"},
		Target: "A Written Note",
		Note:   nav.NoteRef{Name: "written", RelPath: "Concepts/written.md"},
	}}
	if diff := cmp.Diff(wantTitleOnly, h.TitleOnly); diff != "" {
		t.Errorf("TitleOnly mismatch (-want +got):\n%s", diff)
	}

	wantUnwritten := []HealthLink{
		{From: nav.NoteRef{Name: "cites nothing", RelPath: "Concepts/cites nothing.md"}, Target: "no such name"},
		{From: nav.NoteRef{Name: "cites shared", RelPath: "Concepts/cites shared.md"}, Target: "Shared Title"},
	}
	if diff := cmp.Diff(wantUnwritten, h.Unwritten); diff != "" {
		t.Errorf("Unwritten mismatch (-want +got):\n%s", diff)
	}
}
