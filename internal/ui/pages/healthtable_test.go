package pages

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/wording"
)

// countCell finds the counted number on every row of the findings table. It
// reads the cell rather than the row so a row that lost its count would be
// counted as a missing row and not as a row of nothing.
var countCell = regexp.MustCompile(`<td class="y-findings__count" data-col="[^"]*">(\d+)</td>`)

// fileCell finds what each row says it is about, in the order the rows print.
var fileCell = regexp.MustCompile(`(?s)<td class="y-findings__file"[^>]*>(.*?)</td>`)

// severityCell finds the weight on every row, empty where the row carries none.
var severityCell = regexp.MustCompile(`(?s)<td class="y-findings__severity"[^>]*>(.*?)</td>`)

var tagOrSpace = regexp.MustCompile(`(?s)<[^>]*>|\s+`)

// healthFindingField is one field of HealthView that carries findings, filled
// with enough to reach the table, beside what reaching it should produce.
type healthFindingField struct {
	// rows is how many lines the filled field draws.
	rows int
	// findings is what the counted numbers on those lines must add up to.
	findings int
	fill     func(*HealthView)
}

// healthFindingFields is every field of HealthView that puts findings on the
// page. It is written out rather than derived, because deriving it from the
// struct is exactly the thing being checked: a field added without a row would
// derive into nothing and the check would pass over it in silence.
var healthFindingFields = map[string]healthFindingField{
	"Blocked": {rows: 1, findings: 1, fill: func(v *HealthView) {
		v.Blocked = []HealthBlockedSource{{Path: "Sources/Raw.md", Reason: "permission denied"}}
	}},
	"Skipped": {rows: 1, findings: 1, fill: func(v *HealthView) {
		v.Skipped = []HealthSkippedSource{{Path: "Notes/Linked.md", Reason: "symbolic link"}}
	}},
	"Unwritten": {rows: 1, findings: 1, fill: func(v *HealthView) {
		v.Unwritten = []snapshot.HealthLink{{From: healthTestNote, Target: "Ghost"}}
	}},
	"TitleOnly": {rows: 1, findings: 1, fill: func(v *HealthView) {
		v.TitleOnly = []snapshot.HealthTitleLink{{From: healthTestNote, Target: "L02", Note: healthTestNote}}
	}},
	"Islands": {rows: 1, findings: 1, fill: func(v *HealthView) {
		v.Islands = []HealthIslandGroup{{Dir: "Notes", Name: "Notes", Notes: []nav.NoteRef{healthTestNote}}}
		v.IslandCount = 1
	}},
	"FrontmatterUnreadable": {rows: 1, findings: 1, fill: func(v *HealthView) {
		v.FrontmatterUnreadable = []HealthNoteFindings{{Note: healthTestNote, Severity: judge.SeverityError, Count: 1}}
	}},
	"SchemaFaults": {rows: 1, findings: 4, fill: func(v *HealthView) {
		v.SchemaFaults = []HealthNoteFindings{{Note: healthTestNote, Severity: judge.SeverityError, Count: 4}}
	}},
	"StatusOutsideEnum": {rows: 1, findings: 1, fill: func(v *HealthView) {
		v.StatusOutsideEnum = []HealthStatusNote{{Note: healthTestNote, Type: "lesson", Status: "seed"}}
	}},
	"StatusUnreachable": {rows: 1, findings: 1, fill: func(v *HealthView) {
		v.StatusUnreachable = []HealthStatusNote{{Note: healthTestNote, Type: "concept", Status: "published"}}
	}},
	"Collisions": {rows: 1, findings: 1, fill: func(v *HealthView) {
		v.Collisions = []HealthCollision{{Name: "Repeat", Candidates: []nav.NoteRef{
			{Name: "A/Repeat.md", RelPath: "A/Repeat.md"},
			{Name: "B/Repeat.md", RelPath: "B/Repeat.md"},
		}}}
	}},
}

// healthUnfoundFields are the fields of HealthView that carry no finding of
// their own: a number the page counts its own rows against, two statements
// that a group could not be worked out at all, the age of the reading, how the
// table is ordered, and the navigation beside it.
var healthUnfoundFields = []string{
	"IslandCount", "InstanceScopeUnknown", "SchemaScopeUnknown", "LastComplete", "Sort", "Sidebar",
}

var healthTestNote = nav.NoteRef{Name: "L01", RelPath: "Writing/lessons/go/L01.md"}

// TestEveryFieldOfTheHealthViewIsAccountedFor is the enumeration the rest of
// this file leans on. A field added to the view without a row to show it draws
// nothing, and a page that silently drops a finding is worse than one that
// never gathered it — the reader is told there is nothing there.
func TestEveryFieldOfTheHealthViewIsAccountedFor(t *testing.T) {
	t.Parallel()
	for _, field := range reflect.VisibleFields(reflect.TypeFor[HealthView]()) {
		_, shown := healthFindingFields[field.Name]
		unfound := slices.Contains(healthUnfoundFields, field.Name)
		switch {
		case shown && unfound:
			t.Errorf("HealthView.%s is listed both as carrying findings and as carrying none", field.Name)
		case !shown && !unfound:
			t.Errorf("HealthView.%s is in neither list, so nothing here says whether the table shows it", field.Name)
		}
	}
	for name := range healthFindingFields {
		if _, ok := reflect.TypeFor[HealthView]().FieldByName(name); !ok {
			t.Errorf("%q is checked as a field of HealthView and is not one", name)
		}
	}
}

// TestEveryFindingReachesARow renders each field on its own and counts what
// arrived. Several findings of one kind about one file share a row, so what has
// to add up is the counted numbers, not the number of lines.
func TestEveryFindingReachesARow(t *testing.T) {
	t.Parallel()
	for name, field := range healthFindingFields {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var view HealthView
			field.fill(&view)
			if view.clean() {
				t.Fatalf("a view holding %s reads as clean, so the page prints no table at all", name)
			}
			counts := healthRowCounts(t, &view)
			if len(counts) != field.rows {
				t.Errorf("%s drew %d rows, want %d", name, len(counts), field.rows)
			}
			if total := sum(counts); total != field.findings {
				t.Errorf("%s counted %d findings across its rows, want %d", name, total, field.findings)
			}
		})
	}
}

// TestNothingToTabulateDrawsNoTable is the other side of that enumeration. The
// page has two things to say that are not findings — that citations could not
// be evaluated, that the vocabulary could not be read — and a folder can reach
// either with every link resolving and nothing else to report. The page then
// has to say that and stop: a header row of four ordering links over no rows
// offers four controls that each do nothing.
func TestNothingToTabulateDrawsNoTable(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name   string
		view   HealthView
		reason string
	}{
		{"citations could not be evaluated", HealthView{InstanceScopeUnknown: "the index was not built"}, "the index was not built"},
		{"the vocabulary could not be read", HealthView{SchemaScopeUnknown: "the contract could not be read"}, "the contract could not be read"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.view.clean() {
				t.Fatal("the view reads as clean, so this case never reaches the part of the page under test")
			}
			page := renderHealth(t, &tt.view)
			if !strings.Contains(page, tt.reason) {
				t.Fatalf("the page does not carry %q, so it is not the page this case is about", tt.reason)
			}
			if strings.Contains(page, "y-findings") {
				t.Error("the page draws a findings table with no findings in it, so it offers four orderings that each do nothing")
			}
		})
	}
}

// TestTheTableCountsEveryFindingTheViewHolds adds the table's own numbers up
// against the lists behind it, counted a second time here rather than asked of
// the code under test.
func TestTheTableCountsEveryFindingTheViewHolds(t *testing.T) {
	t.Parallel()
	view := recordedHealthView(buildModel(t))
	held := len(view.Blocked) + len(view.Skipped) + len(view.Unwritten) + len(view.TitleOnly) +
		view.IslandCount + len(view.StatusOutsideEnum) + len(view.StatusUnreachable) + len(view.Collisions)
	for _, found := range slices.Concat(view.FrontmatterUnreadable, view.SchemaFaults) {
		held += found.Count
	}
	counts := healthRowCounts(t, &view)
	if total := sum(counts); total != held {
		t.Errorf("the table counts %d findings over %d rows; the view holds %d", total, len(counts), held)
	}
}

// TestTheOrderingSetIsClosed holds the words a header link may ask for to the
// set the request is read against. A link naming a word outside it would look
// like a control and do nothing.
func TestTheOrderingSetIsClosed(t *testing.T) {
	t.Parallel()
	asked := regexp.MustCompile(`href="\?sort=([a-z]+)"`)
	for _, name := range []string{"health-page", "health-page-english"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			data := readRecording(t, name)
			var got []string
			for _, match := range asked.FindAllStringSubmatch(data, -1) {
				got = append(got, match[1])
			}
			slices.Sort(got)
			got = slices.Compact(got)
			want := make([]string, 0, len(healthColumns))
			for _, col := range healthColumns {
				want = append(want, string(col))
			}
			slices.Sort(want)
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("the recorded header links and the ordering set disagree (-set +recording):\n%s", diff)
			}
			for _, col := range healthColumns {
				if ParseHealthColumn(string(col)) != col {
					t.Errorf("%q is a link the page offers and not a word the request reader accepts", col)
				}
			}
		})
	}
}

// TestAnUnreadableOrderingLeavesTheDefault holds the other half of that closed
// set: a word outside it is ignored rather than answered with an argument.
func TestAnUnreadableOrderingLeavesTheDefault(t *testing.T) {
	t.Parallel()
	for _, asked := range []string{"", "Finding", "severity ", "path", "../finding", "1"} {
		if got := ParseHealthColumn(asked); got != HealthByFinding {
			t.Errorf("ParseHealthColumn(%q) = %q, want the default %q", asked, got, HealthByFinding)
		}
	}
}

// TestOrderingByAColumnReordersTheRows is what the header links promise. Each
// ordering is checked by the property it claims rather than against a written
// sequence, so a fixture grown by one row does not have to be re-typed here.
func TestOrderingByAColumnReordersTheRows(t *testing.T) {
	t.Parallel()
	view := recordedHealthView(buildModel(t))

	view.Sort = HealthByFinding
	byFinding := healthRowFiles(t, &view)

	view.Sort = HealthByFile
	byFile := healthRowFiles(t, &view)
	if slices.Equal(byFinding, byFile) {
		t.Errorf("ordering by file left the rows exactly as they were: %v", byFile)
	}
	if !slices.IsSorted(byFile) {
		t.Errorf("ordering by file did not put the files in order: %v", byFile)
	}

	view.Sort = HealthBySeverity
	weights := healthRowWeights(t, &view)
	if slices.Equal(weights, healthRowWeightsUnsorted(t, &view)) {
		t.Error("the fixture's rows are already in weight order, so ordering by weight proves nothing here")
	}
	if !slices.IsSortedFunc(weights, func(a, b int) int { return b - a }) {
		t.Errorf("ordering by weight did not put the heaviest first: %v", weights)
	}

	view.Sort = HealthByCount
	counts := healthRowCounts(t, &view)
	if !slices.IsSortedFunc(counts, func(a, b int) int { return b - a }) {
		t.Errorf("ordering by count did not put the fullest first: %v", counts)
	}
}

// TestJudgeSeverityMatchesWhatTheTableShows puts the two faces side by side.
// The weight on a row is the weight the judging face gives the rule reporting
// the same thing, and the recorded findings that face emits are where that is
// read from — so a rule reweighed there turns this red instead of leaving the
// page quietly claiming the old weight.
func TestJudgeSeverityMatchesWhatTheTableShows(t *testing.T) {
	t.Parallel()
	emitted := recordedJudgeSeverities(t)
	if len(emitted) < 10 {
		t.Fatalf("only %d rules were read out of the recorded findings, so this comparison covers almost nothing", len(emitted))
	}
	for kind, rule := range healthRules {
		weights, found := emitted[rule.id]
		if !found {
			t.Errorf("%q is the rule this page mirrors for %v, and no recorded finding carries it", rule.id, kind.title(wording.En))
			continue
		}
		if !slices.Contains(weights, rule.severity) {
			t.Errorf("the table shows %v as %q; the recorded findings for %q carry %v",
				kind.title(wording.En), rule.severity, rule.id, weights)
		}
	}
}

// healthRowCounts is the counted number on each row, in the order they print.
func healthRowCounts(t *testing.T, view *HealthView) []int {
	t.Helper()
	var out []int
	for _, match := range countCell.FindAllStringSubmatch(renderHealth(t, view), -1) {
		count, err := strconv.Atoi(match[1])
		if err != nil {
			t.Fatalf("a row counted %q, which is no number: %v", match[1], err)
		}
		out = append(out, count)
	}
	if len(out) == 0 {
		t.Fatal("no row of the findings table carries a count, so nothing below can be read off it")
	}
	return out
}

// healthRowFiles is what each row says it is about, in the order they print.
func healthRowFiles(t *testing.T, view *HealthView) []string {
	t.Helper()
	var out []string
	for _, match := range fileCell.FindAllStringSubmatch(renderHealth(t, view), -1) {
		out = append(out, strings.TrimSpace(tagOrSpace.ReplaceAllString(match[1], " ")))
	}
	if len(out) == 0 {
		t.Fatal("no row of the findings table names a file")
	}
	return out
}

// healthRowWeights reads each row's weight back as a number, with a row
// carrying none sorting below every row that does — which is where the table
// puts them.
func healthRowWeights(t *testing.T, view *HealthView) []int {
	t.Helper()
	var out []int
	for _, match := range severityCell.FindAllStringSubmatch(renderHealth(t, view), -1) {
		switch word := strings.TrimSpace(tagOrSpace.ReplaceAllString(match[1], " ")); word {
		case "":
			out = append(out, -1)
		case judge.SeverityInfo.String():
			out = append(out, int(judge.SeverityInfo))
		case judge.SeverityWarn.String():
			out = append(out, int(judge.SeverityWarn))
		case judge.SeverityError.String():
			out = append(out, int(judge.SeverityError))
		default:
			t.Fatalf("a row shows %q as a weight, which is no word the judging face uses", word)
		}
	}
	if len(out) == 0 {
		t.Fatal("no row of the findings table has a weight cell")
	}
	return out
}

// healthRowWeightsUnsorted is the same list in the page's default order, which
// is what ordering by weight has to be shown to have changed.
func healthRowWeightsUnsorted(t *testing.T, view *HealthView) []int {
	t.Helper()
	unordered := *view
	unordered.Sort = HealthByFinding
	return healthRowWeights(t, &unordered)
}

// readRecording is one recorded page, read back as text.
func readRecording(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "render", name+".html")) // #nosec G304 -- a recording name from this test's own fixed list
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", name, err)
	}
	return string(data)
}

// recordedJudgeSeverities is every weight the judging face's own recorded
// findings carry, by rule. Those recordings are that face's frozen output, so
// reading them is reading what it actually emits rather than what a second
// table here says it emits.
//
// A rule can carry more than one weight — a broken link is lighter when the
// target is tracked as still to be written — so the answer is the set.
func recordedJudgeSeverities(t *testing.T) map[judge.RuleID][]judge.Severity {
	t.Helper()
	dir := filepath.Join("..", "..", "judge", "testdata", "golden")
	names, err := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	if err != nil {
		t.Fatalf("Glob(%q): %v", dir, err)
	}
	if len(names) == 0 {
		t.Fatalf("no recorded findings under %q, so nothing is being compared", dir)
	}
	out := make(map[judge.RuleID][]judge.Severity)
	for _, name := range names {
		data, readErr := os.ReadFile(name) // #nosec G304 -- a path this test globbed inside the repository
		if readErr != nil {
			t.Fatalf("ReadFile(%q): %v", name, readErr)
		}
		for line := range strings.Lines(string(data)) {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var found struct {
				RuleID   judge.RuleID `json:"rule_id"`
				Severity string       `json:"severity"`
			}
			if err := json.Unmarshal([]byte(line), &found); err != nil {
				t.Fatalf("a recorded finding in %s is not readable: %v", name, err)
			}
			weight, ok := severityNamed(found.Severity)
			if !ok {
				t.Fatalf("a recorded finding in %s carries the weight %q, which is no word the judging face uses", name, found.Severity)
			}
			if !slices.Contains(out[found.RuleID], weight) {
				out[found.RuleID] = append(out[found.RuleID], weight)
			}
		}
	}
	return out
}

// severityNamed reads a weight back from the word the wire format carries.
func severityNamed(word string) (judge.Severity, bool) {
	for _, weight := range []judge.Severity{judge.SeverityInfo, judge.SeverityWarn, judge.SeverityError} {
		if weight.String() == word {
			return weight, true
		}
	}
	return 0, false
}

func renderHealth(t *testing.T, view *HealthView) string {
	t.Helper()
	var buf bytes.Buffer
	if err := Health(*view, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render the health page: %v", err)
	}
	return buf.String()
}

func sum(values []int) int {
	total := 0
	for _, value := range values {
		total += value
	}
	return total
}
