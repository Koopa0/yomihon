package snapshot

import (
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/schema"
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
	h := newHealth(notes, idx, planned, newBacklinks(notes, idx), schema.ArtifactPolicy{}, titlesByName(notes))

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

// A hundred and thirty-seven uncited notes is a true answer nobody reads, and
// sixty of them being one course's transcripts is the fact that makes the rest
// legible. Grouping drops nothing — every note that had a row still has one —
// and puts the largest group first, so the shape is visible before the rows.
func TestHealthGroupsIslandsByFolderWithoutDroppingAny(t *testing.T) {
	t.Parallel()

	notes := []*vault.Note{
		parse(t, "Sources/course/one.md", "a\n"),
		parse(t, "Sources/course/two.md", "b\n"),
		parse(t, "Sources/course/three.md", "c\n"),
		parse(t, "Writing/essay.md", "d\n"),
		parse(t, "root.md", "e\n"),
	}
	idx := graph.New(notes, nil)
	h := newHealth(notes, idx, judge.NewPlanned(noteBodies(notes)), newBacklinks(notes, idx), schema.ArtifactPolicy{}, titlesByName(notes))

	want := []HealthIslandGroup{
		{Dir: "Sources/course", Notes: []nav.NoteRef{
			{Name: "one", RelPath: "Sources/course/one.md"},
			{Name: "three", RelPath: "Sources/course/three.md"},
			{Name: "two", RelPath: "Sources/course/two.md"},
		}},
		{Dir: "", Notes: []nav.NoteRef{{Name: "root", RelPath: "root.md"}}},
		{Dir: "Writing", Notes: []nav.NoteRef{{Name: "essay", RelPath: "Writing/essay.md"}}},
	}
	if diff := cmp.Diff(want, h.Islands); diff != "" {
		t.Errorf("Islands mismatch (-want +got):\n%s", diff)
	}

	total := 0
	for _, g := range h.Islands {
		total += len(g.Notes)
	}
	if total != len(notes) {
		t.Errorf("grouping lost rows: %d notes in, %d out", len(notes), total)
	}
}

// A note reached only by its title is spared the island list, and the order the
// folder happened to be scanned in is no part of that. The answer used to be
// read out of a table the same pass was still filling, so a note escaped only
// when the note writing its name down sorted ahead of it: rename either file
// and the same vault reported a note as reaching nothing, sending its reader
// to write a link that is already written.
func TestHealthSparesATitleReferencedNoteWhicheverOrderItIsScannedIn(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		targetPath string
		citerPath  string
	}{
		{name: "the note writing the name down is read second", targetPath: "a-target.md", citerPath: "z-citer.md"},
		{name: "the note writing the name down is read first", targetPath: "z-target.md", citerPath: "a-citer.md"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			notes := []*vault.Note{
				parse(t, tt.targetPath, "---\ntitle: Target Note\n---\n\nbody\n"),
				parse(t, tt.citerPath, "---\ntitle: Citer\n---\n\nsee [[Target Note]]\n"),
			}
			// Notes reach this in the order the folder scan yields them.
			slices.SortFunc(notes, func(a, b *vault.Note) int {
				return vault.ComparePaths(a.RelPath, b.RelPath)
			})
			idx := graph.New(notes, nil)
			h := newHealth(notes, idx, judge.NewPlanned(noteBodies(notes)), newBacklinks(notes, idx), schema.ArtifactPolicy{}, titlesByName(notes))

			// The citer alone: nothing writes its name down anywhere, while the
			// target's name is written in the citer either way round.
			want := []HealthIslandGroup{{Dir: "", Notes: []nav.NoteRef{
				{Name: nav.Label(tt.citerPath), RelPath: tt.citerPath},
			}}}
			if diff := cmp.Diff(want, h.Islands); diff != "" {
				t.Errorf("Islands mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// A template's citations name its own slots. Reading every note the same way
// put a placeholder like [[{{concept_a}}]] on the page as a note somebody owes
// — a repair nobody can make, since filling that slot is what using the
// template means. The contract already declares which folders hold artifacts
// rather than instances, and this page reads it now.
func TestHealthIgnoresCitationsFromArtifacts(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeNote(t, root, "Concepts/Alpha.md", "---\ntitle: Alpha\ntype: concept\nstatus: draft\n---\nsee [[No Such Note]]\n")
	writeNote(t, root, "System/templates/Card.md", "---\ntitle: Card\ntype: concept\nstatus: ready\n---\nsee [[{{concept_a}}]]\n")
	contract := testContract(t, root)
	store, _ := newTestStore(t, root, contract)

	h := store.Current().Health()
	var targets []string
	for _, l := range h.Unwritten {
		targets = append(targets, l.Target)
	}
	// The control: a citation from a real note that lands nowhere must still be
	// listed, so an empty answer above cannot pass by the page reporting
	// nothing at all.
	if !slices.Contains(targets, "No Such Note") {
		t.Fatalf("a genuine unwritten citation is missing; targets = %v", targets)
	}
	if slices.Contains(targets, "{{concept_a}}") {
		t.Errorf("a template's own slot is listed as a note somebody owes; targets = %v", targets)
	}
}

// Two files can share a name for years before the first link to it is typed,
// and that link failing is the moment this page exists to prevent. Reading the
// answer off the citations anyone happened to write found the pair only after
// the damage: a name nobody had cited yet was invisible, while the page
// promised to report names two files share.
func TestHealthReportsSharedNamesNobodyHasLinkedTo(t *testing.T) {
	t.Parallel()

	notes := []*vault.Note{
		// Nothing anywhere cites "quiet": the pair stays invisible until
		// somebody writes the link that will not resolve.
		parse(t, "A/quiet.md", "---\ntitle: Quiet A\n---\n\none\n"),
		parse(t, "B/quiet.md", "---\ntitle: Quiet B\n---\n\ntwo\n"),
		// The control: a shared name a real note does cite, which the old
		// reading already found. An empty answer cannot pass this test.
		parse(t, "A/cited.md", "---\ntitle: Cited A\n---\n\nthree\n"),
		parse(t, "B/cited.md", "---\ntitle: Cited B\n---\n\nfour\n"),
		parse(t, "Concepts/reader.md", "---\ntitle: Reader\n---\n\nsee [[cited]]\n"),
	}
	idx := graph.New(notes, nil)
	h := newHealth(notes, idx, judge.NewPlanned(noteBodies(notes)), newBacklinks(notes, idx), schema.ArtifactPolicy{}, titlesByName(notes))

	// The names carrying the extension are absent on purpose: two files sharing
	// "cited.md" necessarily share "cited", and one repair stated twice reads
	// as two.
	want := []HealthCollision{
		{Name: "cited", Candidates: []string{"A/cited.md", "B/cited.md"}},
		{Name: "quiet", Candidates: []string{"A/quiet.md", "B/quiet.md"}},
	}
	if diff := cmp.Diff(want, h.Collisions); diff != "" {
		t.Errorf("Collisions mismatch (-want +got):\n%s", diff)
	}
}
