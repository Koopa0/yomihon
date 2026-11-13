package nav

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/sequence"
)

// TestACourseCountsTheDraftLessonsOfItsMainLine holds the figure the course
// page's header prints. It counts out of the same lessons Planned counts — the
// main line — and nothing a side branch lists, nothing from a block declared out
// of the course and nothing from one nobody declared, because none of those is
// a lesson the head's own lesson count includes.
//
// The fixture puts a draft lesson in each of those places, so the answer is
// wrong in a different way for each rule that could be dropped. Two lessons on
// the main line must not count either: a finished one and one that reaches no
// note, since neither is a draft.
func TestACourseCountsTheDraftLessonsOfItsMainLine(t *testing.T) {
	t.Parallel()

	idx := resolver(t,
		"Writing/L01.md", "Writing/L02.md", "Writing/L03.md",
		"Writing/S01.md", "Writing/S02.md", "Writing/R01.md", "Writing/U01.md")
	drafts := draftFacts("Writing/L02.md", "Writing/S01.md", "Writing/R01.md", "Writing/U01.md")
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
	p := buildPath(pathNote("Maps/Course.md", "Course", body), idx, drafts, testArtifactPolicy(t))

	if p.Planned != 4 {
		t.Fatalf("Planned = %d, want 4; the fixture is not the course this test describes, so its Drafts figure proves nothing", p.Planned)
	}
	if p.Drafts != 1 {
		t.Errorf("Drafts = %d, want 1: L02 alone is a draft on the main line. L01 and L03 are finished, the unwritten lesson reaches no note, and S01, R01 and U01 sit outside the lessons Planned counts", p.Drafts)
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

// TestAnUnresolvedLessonIsNeverADraft states why the count asks about
// resolution as well as about the flag. The flag is read off a note's own
// frontmatter, so today only a row that resolved to a note carries one and the
// two questions cannot come apart. The tree here is built by hand rather than
// parsed, because that is the only way to hand the walk a row that carries the
// flag and no note: a course still plans that lesson, and there is no note to
// be a draft.
func TestAnUnresolvedLessonIsNeverADraft(t *testing.T) {
	t.Parallel()

	group := &PathGroup{
		Name:        "主線",
		Role:        sequence.RolePrimary,
		Projectable: true,
		Items: []PathItem{
			{Entry: &PathEntry{Name: "Written", State: sequence.EntryAccepted, Kind: EntryResolved, RelPath: "Writing/L01.md", Draft: true}},
			{Entry: &PathEntry{Name: "Planned", State: sequence.EntryAccepted, Kind: EntryUnresolved, Draft: true}},
		},
	}
	main, _, _ := projectStops([]*PathGroup{group})
	if main.drafts != 1 {
		t.Errorf("drafts = %d, want 1; a row that reaches no note has no note to be a draft", main.drafts)
	}
}

// draftFacts is the per-note facts a builder reads, with the named notes
// declaring draft: true.
func draftFacts(drafts ...string) map[string]noteFacts {
	facts := make(map[string]noteFacts, len(drafts))
	for _, rel := range drafts {
		facts[rel] = noteFacts{draft: true}
	}
	return facts
}

// draftVault is a contract that knows the draft flag, and a vault laid out to
// read it against: a course listing three lessons, a map listing two, one
// draft, one note that writes the flag false and one that leaves it out.
func draftVault(t *testing.T) (root string, contract *schema.Contract) {
	t.Helper()

	text := `schema_version = "1"

[enums]
type = ["note", "moc", "study-path"]

[enums.status]
note = ["draft", "ready"]

[fields]
required = ["title", "type"]
known = ["title", "type", "draft"]

[scan]
knowledge_dirs = ["Concepts", "Maps"]

[navigation]
path_types = ["study-path"]
map_types = ["moc"]

[artifacts]
non_instance_dirs = ["System/templates"]

[privacy]
never_egress_dirs = []

[[lifecycle]]
status = "draft"
applies_to = ["*"]
initial = true
from = []
owner = ["koopa"]
`
	contractPath := filepath.Join(t.TempDir(), "vault-schema.toml")
	if err := os.WriteFile(contractPath, []byte(text), 0o600); err != nil {
		t.Fatalf("write contract: %v", err)
	}
	contract, err := schema.LoadFile(contractPath)
	if err != nil {
		t.Fatalf("schema.LoadFile: %v", err)
	}

	root = t.TempDir()
	note := func(rel, title, kind, flag, body string) {
		front := "---\ntitle: " + title + "\ntype: " + kind + "\n" + flag
		writeNavFixture(t, root, rel, front+"---\n"+body)
	}
	note("Concepts/Done.md", "Done", "note", "draft: false\n", "body\n")
	note("Concepts/Open.md", "Open", "note", "draft: true\n", "body\n")
	note("Concepts/Blank.md", "Blank", "note", "", "body\n")
	note("Maps/Course.md", "Course", "study-path", "", "## 主線 {sequence=primary}\n\n- [[Done]]\n- [[Open]]\n- [[Blank]]\n")
	note("Maps/Shelf.md", "Shelf", "moc", "", "## Things\n\n- [[Done]]\n- [[Open]]\n")
	return root, contract
}

// draftsByName reads every course row, map row and recent-notes summary the
// model holds into one map from the note's file stem to whether it said draft,
// so a missing row is a missing key rather than a shifted index.
func draftsByName(t *testing.T, model *Model) (course, shelf, recent map[string]bool) {
	t.Helper()

	course, shelf, recent = map[string]bool{}, map[string]bool{}, map[string]bool{}
	if len(model.Paths()) != 1 || len(model.Maps()) != 1 {
		t.Fatalf("fixture produced %d paths and %d maps, want 1 and 1", len(model.Paths()), len(model.Maps()))
	}
	for _, group := range model.Paths()[0].Groups {
		for _, item := range group.Items {
			course[item.Entry.Name] = item.Entry.Draft
		}
	}
	for _, branch := range model.Maps()[0].Branches {
		for _, entry := range branch.Entries {
			shelf[entry.Name] = entry.Draft
		}
	}
	for _, summary := range model.KnowledgeNotes() {
		recent[summary.Title] = summary.Draft
	}
	return course, shelf, recent
}

// TestTheModelMarksEveryRowAndSummaryThatIsADraft holds the one answer every
// surface reads: a course row, a map row and a recent-notes summary each say
// whether the note declares draft: true, so no face reads the frontmatter
// again.
func TestTheModelMarksEveryRowAndSummaryThatIsADraft(t *testing.T) {
	t.Parallel()

	root, contract := draftVault(t)
	model := capturedModelWithJournal(t, root, contract.NavigationRoles(), contract.KnowledgeScope(), contract.ArtifactPolicy(), nil,
		contract.JournalDir(), contract.ArticleLanguage(), contract.AuthoredDate())
	course, shelf, recent := draftsByName(t, model)

	for name, got := range map[string]map[string]bool{"course": course, "map": shelf, "recent": recent} {
		if !got["Open"] || got["Done"] || got["Blank"] {
			t.Errorf("%s drafts = %v, want Open alone: Done writes draft: false and Blank leaves it out", name, got)
		}
	}
	if got := model.Paths()[0].Drafts; got != 1 {
		t.Errorf("Drafts = %d, want 1: Open is the one main-line draft", got)
	}
}
