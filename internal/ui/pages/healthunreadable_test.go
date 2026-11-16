package pages

import (
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestTheFileNothingCouldReadIsWeighed holds the one thing this page used to
// say about a file it could not open that the judging face did not: nothing.
// Both faces answer for it now, so the row carries the weight that face gives
// the rule, the line above the table counts it, and neither depends on which
// language the chrome is read in.
//
// The expected word is written out rather than taken from the vocabulary's own
// constant: it is what a reader sees, and comparing the page against the same
// symbol the page is drawn from would hold whatever that symbol became.
func TestTheFileNothingCouldReadIsWeighed(t *testing.T) {
	t.Parallel()
	view := HealthView{Blocked: sealedVaultBlocked(t)}
	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		t.Run(string(lang), func(t *testing.T) {
			t.Parallel()
			var buf strings.Builder
			if err := Health(view, layouts.Chrome{Lang: lang}).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render the health page: %v", err)
			}
			page := buf.String()

			cells := severityCell.FindAllStringSubmatch(page, -1)
			if len(cells) != 1 {
				t.Fatalf("the page draws %d weight cells, want the one row the sealed reading produced", len(cells))
			}
			if !strings.Contains(cells[0][1], `<span class="y-severity y-severity--error" lang="en">error</span>`) {
				t.Errorf("the unreadable file's weight cell holds %q, want the judging face's error badge", cells[0][1])
			}

			shapeMatch := healthShapeLineRe.FindStringSubmatch(page)
			if shapeMatch == nil {
				t.Fatal("the page holds a finding, so it must carry the shape line")
			}
			weights := healthShapeWeightRe.FindAllStringSubmatch(shapeMatch[1], -1)
			if len(weights) != 1 {
				t.Fatalf("the shape line names %d weights, want the one the row carries: %q", len(weights), shapeMatch[1])
			}
			if word, count := weights[0][2], weights[0][3]; word != "error" || count != "1" {
				t.Errorf("the shape line counts %q %s, want error 1", word, count)
			}
		})
	}
}

// TestTheFileNothingCouldReadStaysFirstWhateverTheTablesAreOrderedBy holds what
// remains of the other half of being weighed now that the report is read by
// kind. The kinds stand in the order they are declared in, the file nothing
// could be read from first among them, and ordering moves rows inside
// a kind and never a kind past another: there is no order of the whole report by
// weight, which is what a table to each kind gives up for being able to say
// what each kind is.
func TestTheFileNothingCouldReadStaysFirstWhateverTheTablesAreOrderedBy(t *testing.T) {
	t.Parallel()
	blocked := sealedVaultBlocked(t)
	view := HealthView{
		Blocked:      blocked,
		Skipped:      []HealthSkippedSource{{Path: "Notes/Linked note.md", Reason: "symbolic link"}},
		Islands:      []HealthIslandGroup{{Dir: "Notes", Name: "Notes", Notes: []nav.NoteRef{{Name: "Alone", RelPath: "Notes/Alone.md"}}}},
		IslandCount:  1,
		SchemaFaults: []HealthNoteFindings{{Note: healthTestNote, Severity: judge.SeverityError, Count: 1}},
	}
	for _, column := range healthColumns {
		view.Sort = column
		tables := healthTables(t, &view)
		if len(tables) != 4 {
			t.Fatalf("ordered by %s the report draws %d tables, want 4", column, len(tables))
		}
		if tables[0].title != healthBlocked.title(wording.ZhHant) || tables[0].files[0] != blocked[0].Path {
			t.Errorf("ordered by %s the first table is %q over %v, want the file nothing could read", column, tables[0].title, tables[0].files)
		}
		if tables[0].weights[0] != int(judge.SeverityError) {
			t.Errorf("ordered by %s the file nothing could read is weighed %d, want error", column, tables[0].weights[0])
		}
		var titles []string
		for _, table := range tables {
			titles = append(titles, table.title)
		}
		wantTitles := []string{healthBlocked.title(wording.ZhHant), healthSkipped.title(wording.ZhHant), healthIsland.title(wording.ZhHant), healthSchemaFault.title(wording.ZhHant)}
		if diff := cmp.Diff(wantTitles, titles); diff != "" {
			t.Errorf("ordered by %s the kinds are not in the order they are declared in (-want +got):\n%s", column, diff)
		}
	}
}

// TestOnlyUncitedNotesAndNavigationFaultsAreUnweighed is the sentence healthRules'
// comment makes, held mechanically. Every field of the view that puts findings
// on the page is filled on its own and the weight cell read back, so a kind
// that stops being weighed — or one that starts — shows up here rather than in
// a comment that has quietly stopped describing the code.
//
// It walks healthFindingFields, which TestEveryFieldOfTheHealthViewIsAccountedFor
// already proves names every such field: an enumeration written out a second
// time here could go short with nothing saying so.
func TestOnlyUncitedNotesAndNavigationFaultsAreUnweighed(t *testing.T) {
	t.Parallel()
	var unweighed []string
	for name, field := range healthFindingFields {
		var view HealthView
		field.fill(&view)
		if slices.ContainsFunc(healthRowWeights(t, &view), func(weight int) bool { return weight < 0 }) {
			unweighed = append(unweighed, name)
		}
	}
	slices.Sort(unweighed)
	if diff := cmp.Diff([]string{"Islands", "NavigationFaults"}, unweighed); diff != "" {
		t.Errorf("the fields drawing a row no weight weighs disagree with what healthRules' comment says (-comment +page):\n%s", diff)
	}
}
