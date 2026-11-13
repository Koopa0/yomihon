package pages

import (
	"bytes"
	"encoding/json"
	"html"
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
	"NavigationFaults": {rows: 1, findings: 1, fill: func(v *HealthView) {
		v.NavigationFaults = []nav.CoreFault{{Note: healthTestNote, Reason: "navigation build failed: walk"}}
	}},
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
	"Collisions": {rows: 1, findings: 1, fill: func(v *HealthView) {
		v.Collisions = []HealthCollision{{Name: "Repeat", Candidates: []nav.NoteRef{
			{Name: "A/Repeat.md", RelPath: "A/Repeat.md"},
			{Name: "B/Repeat.md", RelPath: "B/Repeat.md"},
		}}}
	}},
}

// healthUnfoundFields are the fields of HealthView that carry no finding of
// their own: a number the page counts its own rows against, two statements
// that a group could not be worked out at all, the age of the reading, the
// standing notices that say the reading is not the folder as it is now — drawn
// above the table, where sorting and paging cannot move them — how the table
// is ordered, which stretch of it is on screen, and the navigation beside it.
//
// Two more are not findings by their nature: how the folder's maps cover it,
// said in two groups of their own that appear only when something is in them.
var healthUnfoundFields = []string{
	"IslandCount", "InstanceScopeUnknown", "SchemaScopeUnknown", "LastComplete", "Notices", "Sort", "Page", "Sidebar",
	"UnmappedConcepts", "EmptyBranches",
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
		view.IslandCount + len(view.Collisions)
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

// groupTables is one page's tables, in the order they stand: the heading each is
// drawn under and what its rows say, read off the page.
type groupTable struct {
	title    string
	files    []string
	weights  []int
	counts   []int
	hasAbout bool
}

var groupTitle = regexp.MustCompile(`(?s)<h2 class="y-healthgroup__title"[^>]*>(.*?)</h2>`)

// healthTables reads the findings groups of a rendered view: a table to each
// kind, under that kind's own heading.
func healthTables(t *testing.T, view *HealthView) []groupTable {
	t.Helper()
	var out []groupTable
	for _, chunk := range strings.Split(renderHealth(t, view), `<section class="y-healthgroup"`)[1:] {
		if !strings.Contains(chunk, `class="y-findings"`) {
			continue
		}
		table := groupTable{hasAbout: strings.Contains(chunk, `class="y-healthabout"`)}
		if m := groupTitle.FindStringSubmatch(chunk); m != nil {
			table.title = html.UnescapeString(strings.TrimSpace(tagOrSpace.ReplaceAllString(m[1], " ")))
		}
		for _, m := range fileCell.FindAllStringSubmatch(chunk, -1) {
			table.files = append(table.files, strings.TrimSpace(tagOrSpace.ReplaceAllString(m[1], " ")))
		}
		for _, m := range countCell.FindAllStringSubmatch(chunk, -1) {
			count, err := strconv.Atoi(m[1])
			if err != nil {
				t.Fatalf("a row counted %q, which is no number: %v", m[1], err)
			}
			table.counts = append(table.counts, count)
		}
		for _, m := range severityCell.FindAllStringSubmatch(chunk, -1) {
			switch word := strings.TrimSpace(tagOrSpace.ReplaceAllString(m[1], " ")); word {
			case "":
				table.weights = append(table.weights, -1)
			case judge.SeverityWarn.String():
				table.weights = append(table.weights, int(judge.SeverityWarn))
			case judge.SeverityError.String():
				table.weights = append(table.weights, int(judge.SeverityError))
			default:
				t.Fatalf("a row shows %q as a weight, which is no word the judging face uses", word)
			}
		}
		out = append(out, table)
	}
	return out
}

// orderedView is a report whose two multi-row kinds are in neither file order,
// weight order nor count order as they are gathered, so that each ordering has
// something to move and what it moved is not what it found.
func orderedView() HealthView {
	note := func(name string) nav.NoteRef { return nav.NoteRef{Name: name, RelPath: "Notes/" + name + ".md"} }
	return HealthView{
		Unwritten: []snapshot.HealthLink{
			{From: note("Zed"), Target: "a"}, {From: note("Zed"), Target: "b"},
			{From: note("Alpha"), Target: "c"},
			{From: note("Mid"), Target: "d"}, {From: note("Mid"), Target: "e"}, {From: note("Mid"), Target: "f"},
		},
		SchemaFaults: []HealthNoteFindings{
			{Note: note("Zed"), Severity: judge.SeverityWarn, Count: 1},
			{Note: note("Alpha"), Severity: judge.SeverityError, Count: 2},
			{Note: note("Mid"), Severity: judge.SeverityError, Count: 5},
		},
		Sort: HealthByFinding,
	}
}

// TestOrderingByAColumnReordersTheRowsOfEachKindAndNeverMixesTheKinds is what
// the header links promise now that the report is read by kind. Each ordering is
// checked by the property it claims, inside each table, rather than against a
// written sequence; and the tables themselves, which are the kinds, stand in
// the same order whatever the rows are ordered by.
func TestOrderingByAColumnReordersTheRowsOfEachKindAndNeverMixesTheKinds(t *testing.T) {
	t.Parallel()
	view := orderedView()
	view.Sort = HealthByFinding
	byFinding := healthTables(t, &view)
	if len(byFinding) != 2 {
		t.Fatalf("the fixture draws %d tables, want the unwritten links and the schema faults", len(byFinding))
	}
	titles := func(tables []groupTable) []string {
		var out []string
		for _, table := range tables {
			out = append(out, table.title)
		}
		return out
	}

	view.Sort = HealthByFile
	byFile := healthTables(t, &view)
	view.Sort = HealthBySeverity
	byWeight := healthTables(t, &view)
	view.Sort = HealthByCount
	byCount := healthTables(t, &view)

	for name, tables := range map[string][]groupTable{"file": byFile, "weight": byWeight, "count": byCount} {
		if diff := cmp.Diff(titles(byFinding), titles(tables)); diff != "" {
			t.Errorf("ordering by %s moved a kind: the tables are no longer in the order the kinds are declared in (-default +got):\n%s", name, diff)
		}
	}
	for i, table := range byFile {
		if !slices.IsSorted(table.files) {
			t.Errorf("ordering by file did not put the files of %q in order: %v", table.title, table.files)
		}
		if slices.Equal(table.files, byFinding[i].files) {
			t.Errorf("ordering by file left the rows of %q exactly as they were: %v", table.title, table.files)
		}
	}
	for i, table := range byWeight {
		if !slices.IsSortedFunc(table.weights, func(a, b int) int { return b - a }) {
			t.Errorf("ordering by weight did not put the heaviest of %q first: %v", table.title, table.weights)
		}
		if i == 1 && slices.Equal(table.weights, byFinding[i].weights) {
			t.Errorf("the fixture's schema faults are already in weight order, so ordering by weight proves nothing here: %v", table.weights)
		}
	}
	for i, table := range byCount {
		if !slices.IsSortedFunc(table.counts, func(a, b int) int { return b - a }) {
			t.Errorf("ordering by count did not put the fullest of %q first: %v", table.title, table.counts)
		}
		if slices.Equal(table.counts, byFinding[i].counts) {
			t.Errorf("the rows of %q are already in count order, so ordering by count proves nothing here: %v", table.title, table.counts)
		}
	}
}

// TestEveryKindIsATableUnderItsOwnNameWithOnlyItsExplanationFolded holds what
// the fold may hold. Each kind of finding is a section with its own name, a fold
// that starts closed and carries the sentence saying what the kind means, and a
// table; the sentence is in the fold and nowhere else, and everything found —
// the file, the evidence, the weight, the count — is in the table, outside it.
func TestEveryKindIsATableUnderItsOwnNameWithOnlyItsExplanationFolded(t *testing.T) {
	t.Parallel()
	for name, field := range healthFindingFields {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
				var view HealthView
				field.fill(&view)
				var buf bytes.Buffer
				if err := Health(view, layouts.Chrome{Lang: lang}).Render(t.Context(), &buf); err != nil {
					t.Fatal(err)
				}
				page := buf.String()
				sections := strings.Split(page, `<section class="y-healthgroup"`)[1:]
				if len(sections) != 1 {
					t.Fatalf("%s: %d groups drawn for one filled field, want 1", lang, len(sections))
				}
				section := sections[0]
				kind := healthRows(&view, lang)[0].Kind
				if m := groupTitle.FindStringSubmatch(section); m == nil || html.UnescapeString(strings.TrimSpace(tagOrSpace.ReplaceAllString(m[1], " "))) != kind.title(lang) {
					t.Errorf("%s: the group is not named %q: %s", lang, kind.title(lang), section)
				}
				fold, table, found := strings.Cut(section, "</details>")
				if !found || !strings.Contains(fold, `<details class="y-healthabout">`) || strings.Contains(fold, "<details class=\"y-healthabout\" open") {
					t.Fatalf("%s: the group has no closed fold: %s", lang, section)
				}
				if !strings.Contains(fold, `>`+wording.HealthAbout.In(lang)+`</summary>`) && !strings.Contains(fold, wording.HealthAbout.In(lang)) {
					t.Errorf("%s: the fold is not named %q: %s", lang, wording.HealthAbout.In(lang), fold)
				}
				lede := html.EscapeString(view.kindLede(kind, lang))
				if !strings.Contains(fold, lede) {
					t.Errorf("%s: the fold does not hold the explanation %q: %s", lang, lede, fold)
				}
				if strings.Contains(table, lede) {
					t.Errorf("%s: the explanation is drawn outside the fold too", lang)
				}
				for _, cell := range []string{`class="y-findings__file"`, `class="y-findings__count"`} {
					if strings.Contains(fold, cell) || !strings.Contains(table, cell) {
						t.Errorf("%s: %s is not drawn in the table outside the fold", lang, cell)
					}
				}
			}
		})
	}
}

// healthRows is the rows a view gathers, for a test that needs to know what
// kind a filled field produced.
func healthRows(view *HealthView, lang wording.Lang) []healthRow {
	return view.rows(lang)
}

// TestTheStandingOfUnreadableFilesIsNotFoldedAway holds the one fact a group
// states about itself: how old the rest of the page is when files could not be
// read. It is a diagnosis and not an explanation, so it is in the group's own
// line, outside the fold, whether or not a read has ever been complete.
func TestTheStandingOfUnreadableFilesIsNotFoldedAway(t *testing.T) {
	t.Parallel()
	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		for _, last := range []string{"", "2026-07-10 09:30"} {
			view := HealthView{Blocked: []HealthBlockedSource{{Path: "A.md", Reason: "permission denied"}}, LastComplete: last}
			var buf bytes.Buffer
			if err := Health(view, layouts.Chrome{Lang: lang}).Render(t.Context(), &buf); err != nil {
				t.Fatal(err)
			}
			fold, outside, _ := strings.Cut(buf.String(), "</details>")
			want := view.kindStanding(healthBlocked, lang)
			if want == "" || (last != "" && !strings.Contains(want, last)) {
				t.Fatalf("%s: the standing of the unreadable files is %q for a last complete read of %q", lang, want, last)
			}
			if !strings.Contains(outside, `<p class="y-healthgroup__standing">`+html.EscapeString(want)+`</p>`) {
				t.Errorf("%s: the standing %q is not drawn in the group's own line", lang, want)
			}
			if strings.Contains(fold, html.EscapeString(want)) {
				t.Errorf("%s: the standing %q is behind the fold", lang, want)
			}
		}
	}
}

// TestTheCoverageGroupsAppearOnlyWhenTheyHoldSomethingAndAreNoFindings holds the
// two groups that say how the maps cover the folder. Each draws only when it has
// something in it; neither is a table, a weight or a count; and neither makes a
// folder with nothing wrong with it less than clear, so the all-clear is still
// said above them.
func TestTheCoverageGroupsAppearOnlyWhenTheyHoldSomethingAndAreNoFindings(t *testing.T) {
	t.Parallel()
	concept := nav.NoteRef{Name: "Idea", RelPath: "Concepts/Idea.md", Language: "en"}
	branch := snapshot.HealthEmptyBranch{Map: nav.NoteRef{Name: "Go", RelPath: "Maps/Go.md"}, Heading: "Waiting", Level: 2}
	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		concepts, branches := wording.HealthUnmappedConceptsTitle.In(lang), wording.HealthEmptyBranchesTitle.In(lang)
		for _, tt := range []struct {
			name             string
			view             HealthView
			wantConcepts     bool
			wantBranches     bool
			wantInsideBranch string
		}{
			{name: "neither", view: HealthView{}},
			{name: "concepts only", view: HealthView{UnmappedConcepts: []nav.NoteRef{concept}}, wantConcepts: true},
			{name: "branches only", view: HealthView{EmptyBranches: []snapshot.HealthEmptyBranch{branch}}, wantBranches: true, wantInsideBranch: "Waiting"},
			{name: "both", view: HealthView{UnmappedConcepts: []nav.NoteRef{concept}, EmptyBranches: []snapshot.HealthEmptyBranch{branch}}, wantConcepts: true, wantBranches: true, wantInsideBranch: "Waiting"},
		} {
			var buf bytes.Buffer
			if err := Health(tt.view, layouts.Chrome{Lang: lang}).Render(t.Context(), &buf); err != nil {
				t.Fatal(err)
			}
			page := buf.String()
			if got := strings.Contains(page, `>`+concepts+`</h2>`); got != tt.wantConcepts {
				t.Errorf("%s %s: unmapped-concept group drawn = %v, want %v", lang, tt.name, got, tt.wantConcepts)
			}
			if got := strings.Contains(page, `>`+branches+`</h2>`); got != tt.wantBranches {
				t.Errorf("%s %s: empty-branch group drawn = %v, want %v", lang, tt.name, got, tt.wantBranches)
			}
			if got := strings.Contains(page, `data-health-coverage="concepts"`); got != tt.wantConcepts {
				t.Errorf("%s %s: the concept group's marker = %v, want %v", lang, tt.name, got, tt.wantConcepts)
			}
			if tt.wantInsideBranch != "" && !strings.Contains(page, `>`+tt.wantInsideBranch+`</span>`) {
				t.Errorf("%s %s: the empty heading %q is not named", lang, tt.name, tt.wantInsideBranch)
			}
			for _, drawn := range []string{"<table", `class="y-severity`, `class="y-healthshape`} {
				if strings.Contains(page, drawn) {
					t.Errorf("%s %s: a coverage group draws %q: a table, a weight or a total", lang, tt.name, drawn)
				}
			}
			if !strings.Contains(page, `data-nothing="health"`) {
				t.Errorf("%s %s: the coverage groups took the all-clear away from a folder with nothing wrong", lang, tt.name)
			}
		}
	}
}

// TestTheCoverageGroupsStandAfterTheLastTableOfTheReport holds where they are
// drawn when there are findings too: after every table, and on the last page of
// a report that runs to several, so a reader stepping through the report meets
// them once and not on every page.
func TestTheCoverageGroupsStandAfterTheLastTableOfTheReport(t *testing.T) {
	t.Parallel()
	branch := snapshot.HealthEmptyBranch{Map: nav.NoteRef{Name: "Go", RelPath: "Maps/Go.md"}, Heading: "Waiting", Level: 2}
	view := pagedHealthView(pagedReport)
	view.EmptyBranches = []snapshot.HealthEmptyBranch{branch}
	lastPage := (pagedReport + healthPageSize - 1) / healthPageSize
	for _, asked := range []PageNumber{1, 2, PageNumber(lastPage), AllPages} {
		view.Page = asked
		page := renderHealth(t, &view)
		drawn := strings.Contains(page, `data-health-coverage="branches"`)
		if want := asked == PageNumber(lastPage) || asked == AllPages; drawn != want {
			t.Errorf("page %v draws the coverage groups = %v, want %v", asked, drawn, want)
		}
		if drawn && strings.Index(page, `data-health-coverage="branches"`) < strings.LastIndex(page, `<table class="y-findings">`) {
			t.Errorf("page %v draws the coverage groups before a table of findings", asked)
		}
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

func TestNavigationFaultRowIsLinkedUnweighedAndExplained(t *testing.T) {
	t.Parallel()
	fault := nav.CoreFault{Note: nav.NoteRef{Name: "<Course & notes>", RelPath: "Maps/Course.md", Language: "ja"}, Reason: "navigation build failed: <script>alert(1)</script> & error"}
	view := HealthView{NavigationFaults: []nav.CoreFault{fault}}
	want := []healthRow{{Kind: healthNavigationFault, File: fault.Note, Detail: []healthDetail{{Text: fault.Reason, Machine: true}}, Count: 1}}
	if diff := cmp.Diff(want, view.gather(wording.En)); diff != "" {
		t.Errorf("navigation row (-want +got):\n%s", diff)
	}
	for _, chrome := range []struct {
		lang         wording.Lang
		title, guide string
	}{
		{wording.ZhHant, "導覽建構失敗", "這些筆記的導覽無法建構，因此未列入導覽。筆記仍可閱讀與搜尋，其他筆記的導覽仍可使用。"},
		{wording.En, "Navigation could not be built", "Navigation could not be built for these notes, so their navigation entries are omitted. The notes remain readable and searchable; other notes’ navigation remains available."},
	} {
		lang := chrome.lang
		var output bytes.Buffer
		if err := Health(view, layouts.Chrome{Lang: lang}).Render(t.Context(), &output); err != nil {
			t.Fatal(err)
		}
		page := output.String()
		for _, text := range []string{`href="/notes/Maps/Course.md"`, `lang="ja"`, "&lt;Course &amp; notes&gt;", "&lt;script&gt;alert(1)&lt;/script&gt; &amp; error", chrome.title, chrome.guide} {
			if !strings.Contains(page, text) {
				t.Errorf("%s page lacks %q", lang, text)
			}
		}
		if strings.Contains(page, "<script>alert(1)</script>") {
			t.Error("panic reason was not escaped")
		}
		if !slices.Contains(healthKinds, healthNavigationFault) {
			t.Error("navigation kind omitted from the guide")
		}
		if got := healthNavigationFault.title(lang); got != chrome.title {
			t.Errorf("navigation title = %q", got)
		}
		if got := view.kindLede(healthNavigationFault, lang); got != chrome.guide {
			t.Errorf("navigation guide = %q", got)
		}
	}
}
