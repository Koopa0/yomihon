package snapshot

import (
	"iter"
	"path/filepath"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

// noteBodies iterates the parsed bodies of notes, for a test that harvests the
// planned names of the corpus it built in one pass.
func noteBodies(notes []*vault.Note) iter.Seq[string] {
	return func(yield func(string) bool) {
		for _, n := range notes {
			if !yield(n.Body) {
				return
			}
		}
	}
}

// linksOf is what each note cites, by path, read through the same extractor a
// build reads a note's body with when it reads the note.
func linksOf(notes []*vault.Note) map[string][]string {
	links := make(map[string][]string, len(notes))
	for _, n := range notes {
		links[n.RelPath] = judge.LinkTargets(n.Body)
	}
	return links
}

// A citation that lands nowhere has more than one reason and more than one
// repair, and this page is worse than useless if it names the wrong one. The
// title case was found by running the page against a real vault: three notes
// that exist were listed as unwritten, because a title is never a name this
// vault resolves by and nothing here had separated that from absence.
func TestHealthSeparatesTheReasonsACitationFails(t *testing.T) {
	t.Parallel()

	notes := []*vault.Note{
		parse(t, "Concepts/written.md", "# A Written Note\n\nI exist\n"),
		parse(t, "Concepts/cites title.md", "# Cites Title\n\nsee [[A Written Note]]\n"),
		parse(t, "Concepts/cites nothing.md", "# Cites Nothing\n\nsee [[no such name]]\n"),
		parse(t, "Concepts/cites owed.md", "# Cites Owed\n\nsee [[Owed Concept]]\n"),
		parse(t, "Maps/ledger.md", "# Ledger\n\n## 缺口帳\n\n- Owed Concept\n"),
		// Two notes declaring one title: naming either of them would be the
		// guess the resolver refuses to make, so this citation stays unwritten
		// rather than being pointed at one of them.
		parse(t, "A/twin.md", "# Shared Title\n\none\n"),
		parse(t, "B/twin.md", "# Shared Title\n\ntwo\n"),
		parse(t, "Concepts/cites shared.md", "# Cites Shared\n\nsee [[Shared Title]]\n"),
	}
	idx := graph.New(notes, nil)
	planned := judge.NewPlanned(noteBodies(notes), nil)
	h := newHealth(notes, linksOf(notes), idx, planned, newBacklinks(notes, linksOf(notes), idx), schema.ArtifactPolicy{}, titlesByName(notes), schema.NavigationRoles{}, mapCoverage{})

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
	h := newHealth(notes, linksOf(notes), idx, judge.NewPlanned(noteBodies(notes), nil), newBacklinks(notes, linksOf(notes), idx), schema.ArtifactPolicy{}, titlesByName(notes), schema.NavigationRoles{}, mapCoverage{})

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
		name string
		// The two paths in the order the folder scan hands them over, written
		// out rather than sorted: this holds an answer that must not depend on
		// that order, so it cannot take the order from a comparison of its own.
		targetPath  string
		citerPath   string
		targetFirst bool
	}{
		{name: "the note writing the name down is read second", targetPath: "a-target.md", citerPath: "z-citer.md", targetFirst: true},
		{name: "the note writing the name down is read first", targetPath: "z-target.md", citerPath: "a-citer.md", targetFirst: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			target := parse(t, tt.targetPath, "# Target Note\n\nbody\n")
			citer := parse(t, tt.citerPath, "# Citer\n\nsee [[Target Note]]\n")
			notes := []*vault.Note{citer, target}
			if tt.targetFirst {
				notes = []*vault.Note{target, citer}
			}
			idx := graph.New(notes, nil)
			h := newHealth(notes, linksOf(notes), idx, judge.NewPlanned(noteBodies(notes), nil), newBacklinks(notes, linksOf(notes), idx), schema.ArtifactPolicy{}, titlesByName(notes), schema.NavigationRoles{}, mapCoverage{})

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
	writeNote(t, root, "Concepts/Alpha.md", "---\ntype: concept\n---\nsee [[No Such Note]]\n")
	writeNote(t, root, "System/templates/Card.md", "---\ntype: concept\n---\nsee [[{{concept_a}}]]\n")
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

// Two files can share a name for years without costing anyone a link, and a
// course vault repeats main.go in every lab by design. The page lists a shared
// name once a link is written against it, which is when the vault's refusal to
// guess starts to matter and the same names the judge's collision rule reports.
// Counting every shared name would answer with one row per repeated file name
// and bury what is wrong.
func TestHealthReportsSharedNamesOnlyOnceALinkUsesThem(t *testing.T) {
	t.Parallel()

	notes := []*vault.Note{
		// Nothing anywhere cites "quiet", so nothing is broken by the pair.
		parse(t, "A/quiet.md", "# Quiet A\n\none\n"),
		parse(t, "B/quiet.md", "# Quiet B\n\ntwo\n"),
		// The control: a shared name a real note does cite. An empty answer
		// cannot pass this test.
		parse(t, "A/cited.md", "# Cited A\n\nthree\n"),
		parse(t, "B/cited.md", "# Cited B\n\nfour\n"),
		// A shared resource name is cited by an embed, and its twin by path
		// only, which resolves by path and breaks nothing.
		parse(t, "Concepts/reader.md", "# Reader\n\nsee [[cited]] and ![[shot.png]] and [run](lab/a/out.txt)\n"),
	}
	resources := []string{"img/shot.png", "old/shot.png", "lab/a/out.txt", "lab/b/out.txt"}
	idx := graph.New(notes, resources)
	h := newHealth(notes, linksOf(notes), idx, judge.NewPlanned(noteBodies(notes), nil), newBacklinks(notes, linksOf(notes), idx), schema.ArtifactPolicy{}, titlesByName(notes), schema.NavigationRoles{}, mapCoverage{})

	// The names carrying the extension are absent on purpose: two files sharing
	// "cited.md" necessarily share "cited", and one repair stated twice reads
	// as two.
	want := []HealthCollision{
		{Name: "cited", Candidates: []string{"A/cited.md", "B/cited.md"}},
		{Name: "shot.png", Candidates: []string{"img/shot.png", "old/shot.png"}},
	}
	if diff := cmp.Diff(want, h.Collisions); diff != "" {
		t.Errorf("Collisions mismatch (-want +got):\n%s", diff)
	}
}

// TestDeclaredNonDefaultGapMarkKeepsHarvestedNameOffUnwritten is the lock
// that snapshot harvests against the contract's marks, not the loader
// default. A name listed under ## Gaps is owed once the contract writes
// that heading, so /health must not list it as unwritten. Nowhere stays
// on the list so the page is still evaluating.
func TestDeclaredNonDefaultGapMarkKeepsHarvestedNameOffUnwritten(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeNote(t, root, "Maps/ledger.md", "# Ledger\n\n## Gaps\n- Ghost\n")
	writeNote(t, root, "Concepts/citer.md", "# Citer\n\nSee [[Ghost]] and [[Nowhere]].\n")
	contract := testContractWithRules(t, root, "planned_gap_marks = [\"Gaps\"]\nplanned_inline_marks = []")
	store, _ := newTestStore(t, root, contract)
	h := store.Current().health

	if slices.ContainsFunc(h.Unwritten, func(l HealthLink) bool { return l.Target == "Ghost" }) {
		t.Fatalf("Ghost under declared Gaps is Unwritten; the harvest ignored the contract: %+v", h.Unwritten)
	}
	if !slices.ContainsFunc(h.Unwritten, func(l HealthLink) bool { return l.Target == "Nowhere" }) {
		t.Fatalf("Nowhere is missing from Unwritten; the page is not evaluating: %+v", h.Unwritten)
	}
}

func TestNavigationFaultMakesHealthNonempty(t *testing.T) {
	t.Parallel()
	health := Health{NavigationFaults: []nav.CoreFault{{Note: nav.NoteRef{Name: "Course", RelPath: "Maps/Course.md"}, Reason: "navigation build failed: failed"}}}
	if health.Empty() {
		t.Error("a navigation fault reads as empty Health")
	}
	if !(&Health{}).Empty() {
		t.Error("empty Health has findings")
	}
}

func TestHealthNavigationFaultsBelongToCaller(t *testing.T) {
	t.Parallel()
	fault := nav.CoreFault{Note: nav.NoteRef{Name: "Course", RelPath: "Maps/Course.md", Language: "ja"}, Reason: "navigation build failed: failed"}
	generation := &Generation{health: Health{NavigationFaults: []nav.CoreFault{fault}}}
	health := generation.Health()
	health.NavigationFaults[0].Note.Name = "changed"
	health.NavigationFaults[0].Reason = "changed"
	if diff := cmp.Diff([]nav.CoreFault{fault}, generation.Health().NavigationFaults); diff != "" {
		t.Errorf("captured fault changed (-want +got):\n%s", diff)
	}
	if (*Generation)(nil).Health().NavigationFaults != nil {
		t.Error("absent generation carries faults")
	}
}

// coverageVault is a folder whose maps reach one concept, whose other concepts
// are cited by a note that is no map or by nothing, and which has a concept
// outside the knowledge layer and a template, neither of which is its business.
func coverageVault(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	concept := func(title string) string { return "---\ntype: concept\n---\n# " + title + "\n\nWords.\n" }
	writeNote(t, root, "Concepts/Mapped.md", concept("Mapped"))
	writeNote(t, root, "Concepts/Pending.md", concept("Pending"))
	writeNote(t, root, "Concepts/Orphan.md", concept("Orphan"))
	writeNote(t, root, "Outside/Concept.md", concept("Outside"))
	writeNote(t, root, "System/templates/Template.md", concept("Template"))
	writeNote(t, root, "Writing/Essay.md", "---\ntype: writing\n---\n# Essay\n\nSee [[Pending]].\n")
	writeNote(t, root, "Maps/Go.md", "---\ntype: moc\n---\n# Go\n\n## Branch\n- [[Mapped]]\n\n## Waiting\n\nNothing here yet.\n")
	return root
}

func coverageContract(t *testing.T) *schema.Contract {
	t.Helper()
	contract, err := schema.LoadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatal(err)
	}
	return contract
}

// TestHealthSaysWhichConceptsNoMapLinksToAndWhichHeadingsHoldNoNote holds the
// two groups that say how the maps cover the folder: a concept is reached when
// a note of a map type cites it in its text, and one cited only by another kind
// of note, or by nothing, is not; a heading is empty when no note stands under
// it. A concept outside the knowledge layer and a template are no concept of the
// vault's, and neither group is a finding.
func TestHealthSaysWhichConceptsNoMapLinksToAndWhichHeadingsHoldNoNote(t *testing.T) {
	t.Parallel()

	store, _ := newTestStore(t, coverageVault(t), coverageContract(t))
	health := store.Current().Capture().Health()
	want := []nav.NoteRef{
		{Name: "Orphan", RelPath: "Concepts/Orphan.md"},
		{Name: "Pending", RelPath: "Concepts/Pending.md"},
	}
	if diff := cmp.Diff(want, health.UnmappedConcepts); diff != "" {
		t.Errorf("concepts no map links to (-want +got):\n%s", diff)
	}
	wantBranches := []HealthEmptyBranch{{Map: nav.NoteRef{Name: "Go", RelPath: "Maps/Go.md"}, Heading: "Waiting", Level: 2}}
	if diff := cmp.Diff(wantBranches, health.EmptyBranches); diff != "" {
		t.Errorf("headings no note stands under (-want +got):\n%s", diff)
	}
}

// TestAFolderThatFilesNoConceptsHasNoUnmappedConceptGroup holds the other end:
// a contract that declares no concept type has no concept corpus to cover, and
// says nothing rather than listing every note.
func TestAFolderThatFilesNoConceptsHasNoUnmappedConceptGroup(t *testing.T) {
	t.Parallel()

	store, _ := newTestStore(t, coverageVault(t), nil)
	if got := store.Current().Capture().Health().UnmappedConcepts; len(got) != 0 {
		t.Errorf("a folder with no contract lists %d unmapped concepts: %v", len(got), got)
	}
}
