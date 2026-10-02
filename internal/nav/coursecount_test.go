package nav

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/sequence"
)

// TestACourseCountsTheUnsettledLessonsOfItsMainLine holds the figure the course
// page's header prints. It counts out of the same lessons Planned counts — the
// main line — and nothing a side branch lists, nothing from a block declared out
// of the course and nothing from one nobody declared, because none of those is
// a lesson the head's own lesson count includes.
//
// The fixture puts a lesson at an unsettled status in each of those places, so
// the answer is wrong in a different way for each rule that could be dropped.
// Two lessons on the main line must not count either: one that states no status
// and one that reaches no note, since neither has a status to be unsettled at.
func TestACourseCountsTheUnsettledLessonsOfItsMainLine(t *testing.T) {
	t.Parallel()

	idx := resolver(t,
		"Writing/L01.md", "Writing/L02.md", "Writing/L03.md",
		"Writing/S01.md", "Writing/S02.md", "Writing/R01.md", "Writing/U01.md")
	status := map[string]string{
		"Writing/L01.md": "ready",
		"Writing/L02.md": "draft",
		"Writing/S01.md": "draft",
		"Writing/S02.md": "ready",
		"Writing/R01.md": "draft",
		"Writing/U01.md": "draft",
	}
	body := "## 主線 {sequence=primary}\n\n" +
		"- [[L01]]\n" +
		"- [[L02]]\n" +
		"\t- 支線 {sequence=local}\n" +
		"\t\t- [[S01]]\n" +
		"\t\t- [[S02]]\n" +
		"- [[L03]]\n" +
		"- [[Nobody wrote this one]]\n" +
		"\n## 日常 {sequence=none}\n\n- [[R01]]\n" +
		"\n## 忘了宣告\n\n- [[U01]]\n"
	p := buildPath(pathNote("Maps/Course.md", "Course", body), idx, statusFacts(status, "ready"), testArtifactPolicy(t))

	if p.Planned != 4 {
		t.Fatalf("Planned = %d, want 4; the fixture is not the course this test describes, so its Unsettled figure proves nothing", p.Planned)
	}
	if p.Unsettled != 1 {
		t.Errorf("Unsettled = %d, want 1: L02 alone is on the main line at an unsettled status. L01 is settled, L03 states no status, the unwritten lesson reaches no note, and S01, R01 and U01 sit outside the lessons Planned counts", p.Unsettled)
	}
}

// TestABranchSaysWhetherASurfaceDrawsIt and its companion below pin the two
// answers the course page and the navigation rail both read, so neither has to
// re-derive one from Role and Projectable.
func TestABranchSaysWhetherASurfaceDrawsIt(t *testing.T) {
	t.Parallel()

	idx := resolver(t, "Writing/L01.md")
	body := "## 結構\n\n" +
		"### 主線 {sequence=primary}\n\n- [[L01]]\n" +
		"\n## 日常 {sequence=none}\n\n- [[L01]]\n"
	p := buildPath(pathNote("Maps/Course.md", "Course", body), idx, nil, testArtifactPolicy(t))

	if len(p.Groups) != 2 {
		t.Fatalf("groups = %d, want the structural heading and the block declared out of the course", len(p.Groups))
	}
	structural, declaredOut := p.Groups[0], p.Groups[1]
	if !structural.Drawn() {
		t.Error("a heading that carries a declared branch is not drawn, so its parts would be orphaned")
	}
	if declaredOut.Drawn() {
		t.Error("a block the author declared out of the course is drawn as part of it")
	}
	if len(structural.Items) != 1 || structural.Items[0].Group == nil {
		t.Fatalf("structural branch holds %d items, want one nested branch", len(structural.Items))
	}
	if !structural.Items[0].Group.Drawn() {
		t.Error("the declared branch under the heading is not drawn")
	}
}

func TestABranchSaysWhichRowsItTeaches(t *testing.T) {
	t.Parallel()

	idx := resolver(t, "Writing/L01.md")
	body := "## 主線 {sequence=primary}\n\n- [[L01]]\n" +
		"\n## 日常 {sequence=none}\n\n- [[L01]]\n"
	p := buildPath(pathNote("Maps/Course.md", "Course", body), idx, nil, testArtifactPolicy(t))

	taught := p.Groups[0]
	untaught := p.Groups[1]
	if taught.Items[0].Entry.State != sequence.EntryAccepted {
		t.Fatalf("the fixture's main-line row is %v, not an accepted one, so this test asserts nothing", taught.Items[0].Entry.State)
	}
	if !taught.Teaches(taught.Items[0].Entry) {
		t.Error("a row the grammar accepted on the main line is not one of the course's lessons")
	}
	if untaught.Teaches(untaught.Items[0].Entry) {
		t.Error("a row inside a block declared out of the course reads as one of its lessons")
	}
	if taught.Teaches(nil) {
		t.Error("a branch teaches a row that is not there")
	}
}

// TestAnUnresolvedLessonIsNeverUnsettled states why the count asks about
// resolution as well as about the status. A status is a fact read off a note's
// own frontmatter, so today only a row that resolved to a note carries one and
// the two questions cannot come apart. The tree here is built by hand rather
// than parsed, because that is the only way to hand the walk a row that carries
// a status and no note: a course still plans that lesson, and there is no note
// whose state it could be.
func TestAnUnresolvedLessonIsNeverUnsettled(t *testing.T) {
	t.Parallel()

	group := &PathGroup{
		Name:        "主線",
		Role:        sequence.RolePrimary,
		Projectable: true,
		Items: []PathItem{
			{Entry: &PathEntry{Name: "Written", State: sequence.EntryAccepted, Kind: EntryResolved, RelPath: "Writing/L01.md", Status: "draft"}},
			{Entry: &PathEntry{Name: "Planned", State: sequence.EntryAccepted, Kind: EntryUnresolved, Status: "draft"}},
		},
	}
	main, _, _ := projectStops([]*PathGroup{group})
	if main.unsettled != 1 {
		t.Errorf("unsettled = %d, want 1; a row that reaches no note has no note to be unsettled", main.unsettled)
	}
}

// statusFacts is a fixture's statuses as the per-note facts a builder reads.
// The statuses in settled are the ones its contract declares settled.
func statusFacts(status map[string]string, settled ...string) map[string]noteFacts {
	facts := make(map[string]noteFacts, len(status))
	for rel, s := range status {
		facts[rel] = noteFacts{status: s, settled: slices.Contains(settled, s)}
	}
	return facts
}

// settledVault is a contract whose lifecycle rows write the settled key on
// whichever of draft and ready the caller names, and a vault laid out to read
// it against: a course listing three lessons, a map listing two, one note at
// each status and one that states none.
func settledVault(t *testing.T, settled ...string) (root string, contract *schema.Contract) {
	t.Helper()

	row := func(status, from string) string {
		text := "[[lifecycle]]\nstatus = \"" + status + "\"\napplies_to = [\"*\"]\ninitial = " + map[bool]string{true: "true", false: "false"}[from == ""] +
			"\nfrom = [" + from + "]\nowner = [\"koopa\"]\n"
		if slices.Contains(settled, status) {
			text += "settled = true\n"
		}
		return text
	}
	text := `schema_version = "1"

[enums]
type = ["note", "moc", "study-path"]

[enums.status]
note = ["draft", "ready"]

[fields]
required = ["title", "type"]
known = ["title", "type", "status"]

[scan]
knowledge_dirs = ["Concepts", "Maps"]

[navigation]
path_types = ["study-path"]
map_types = ["moc"]

[artifacts]
non_instance_dirs = ["System/templates"]

[privacy]
never_egress_dirs = []

` + row("draft", `"ready"`) + "\n" + row("ready", `"draft"`)
	contractPath := filepath.Join(t.TempDir(), "vault-schema.toml")
	if err := os.WriteFile(contractPath, []byte(text), 0o600); err != nil {
		t.Fatalf("write contract: %v", err)
	}
	contract, err := schema.LoadFile(contractPath)
	if err != nil {
		t.Fatalf("schema.LoadFile: %v", err)
	}

	root = t.TempDir()
	note := func(rel, title, kind, status, body string) {
		front := "---\ntitle: " + title + "\ntype: " + kind + "\n"
		if status != "" {
			front += "status: " + status + "\n"
		}
		writeNavFixture(t, root, rel, front+"---\n"+body)
	}
	note("Concepts/Done.md", "Done", "note", "ready", "body\n")
	note("Concepts/Open.md", "Open", "note", "draft", "body\n")
	note("Concepts/Blank.md", "Blank", "note", "", "body\n")
	note("Maps/Course.md", "Course", "study-path", "", "## 主線 {sequence=primary}\n\n- [[Done]]\n- [[Open]]\n- [[Blank]]\n")
	note("Maps/Shelf.md", "Shelf", "moc", "", "## Things\n\n- [[Done]]\n- [[Open]]\n")
	return root, contract
}

// builtFrom captures the vault settledVault laid out and builds the model the
// way the product does, with the contract's own settlement.
func builtFrom(t *testing.T, root string, contract *schema.Contract) *Model {
	t.Helper()
	return capturedModelWithJournal(t, root, contract.NavigationRoles(), contract.KnowledgeScope(), contract.ArtifactPolicy(), nil,
		contract.JournalDir(), contract.ArticleLanguage(), contract.AuthoredDate(), contract.Settlement())
}

// settledByName reads every course row, map row and recent-notes summary the
// model holds into one map from the note's file stem to what each said about it,
// so a missing row is a missing key rather than a shifted index.
func settledByName(t *testing.T, model *Model) (course, shelf, recent map[string]bool) {
	t.Helper()

	course, shelf, recent = map[string]bool{}, map[string]bool{}, map[string]bool{}
	if len(model.Paths()) != 1 || len(model.Maps()) != 1 {
		t.Fatalf("fixture produced %d paths and %d maps, want 1 and 1", len(model.Paths()), len(model.Maps()))
	}
	for _, group := range model.Paths()[0].Groups {
		for _, item := range group.Items {
			course[item.Entry.Name] = item.Entry.Settled
		}
	}
	for _, branch := range model.Maps()[0].Branches {
		for _, entry := range branch.Entries {
			shelf[entry.Name] = entry.Settled
		}
	}
	for _, summary := range model.KnowledgeNotes() {
		recent[summary.Title] = summary.Settled
	}
	return course, shelf, recent
}

// TestTheModelMarksEveryRowAndSummaryWithTheContractsSettlement holds the one
// answer every surface reads: a course row, a map row and a recent-notes
// summary each say whether their status is one the contract settles, so no face
// asks the contract again or names a status of its own.
func TestTheModelMarksEveryRowAndSummaryWithTheContractsSettlement(t *testing.T) {
	t.Parallel()

	root, contract := settledVault(t, "ready")
	model := builtFrom(t, root, contract)
	course, shelf, recent := settledByName(t, model)

	for name, got := range map[string]map[string]bool{"course": course, "map": shelf} {
		if !got["Done"] || got["Open"] {
			t.Errorf("%s rows settled = %v, want Done settled and Open not: the contract settles ready alone", name, got)
		}
	}
	if !recent["Done"] || recent["Open"] || recent["Blank"] {
		t.Errorf("recent summaries settled = %v, want Done settled, and Open and Blank not", recent)
	}
	if got := model.Paths()[0].Unsettled; got != 1 {
		t.Errorf("Unsettled = %d, want 1: Open is the one main-line lesson with a status the contract does not settle, and Blank states none", got)
	}
}

// TestACourseOverAContractThatSettlesNothingCountsNoException keeps the
// fallback honest: a contract that never wrote the key leaves every row
// unsettled, so every status stays marked, and the count of exceptions is zero
// rather than the number of lessons that happen to carry a status.
func TestACourseOverAContractThatSettlesNothingCountsNoException(t *testing.T) {
	t.Parallel()

	root, contract := settledVault(t)
	if contract.DeclaresSettled() {
		t.Fatal("the fixture contract declares a settled status, so it is not the fallback this test is about")
	}
	model := builtFrom(t, root, contract)
	course, shelf, recent := settledByName(t, model)

	for name, got := range map[string]map[string]bool{"course": course, "map": shelf, "recent": recent} {
		for note, settled := range got {
			if settled {
				t.Errorf("%s: %s is settled, want nothing settled where the contract declares nothing", name, note)
			}
		}
	}
	if got := model.Paths()[0].Unsettled; got != 0 {
		t.Errorf("Unsettled = %d, want 0: with nothing settled every lesson would be an exception, which says nothing", got)
	}
}
