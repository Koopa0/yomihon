package pages

import (
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

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

// TestOrderingByWeightPlacesTheFileNothingCouldRead is the other half of being
// weighed. A reader ordering this page by weight is asking what is worst first,
// and a file nothing could be read from is the gravest thing the judging face
// has to say — so it sorts above a lighter finding, and above the one kind that
// carries no weight at all.
func TestOrderingByWeightPlacesTheFileNothingCouldRead(t *testing.T) {
	t.Parallel()
	blocked := sealedVaultBlocked(t)
	// A second finding of the same weight sits below a lighter one in the
	// page's own order, so ordering by weight has something to move and the
	// comparison below is not satisfied by rows that were already in place.
	view := HealthView{
		Blocked:           blocked,
		Skipped:           []HealthSkippedSource{{Path: "Notes/Linked note.md", Reason: "symbolic link"}},
		Islands:           []HealthIslandGroup{{Dir: "Notes", Name: "Notes", Notes: []nav.NoteRef{{Name: "Alone", RelPath: "Notes/Alone.md"}}}},
		IslandCount:       1,
		StatusOutsideEnum: []HealthStatusNote{{Note: healthTestNote, Type: "lesson", Status: "seed"}},
		Sort:              HealthBySeverity,
	}
	want := []string{blocked[0].Path, healthTestNote.Name, "Notes/Linked note.md", "Alone"}
	if diff := cmp.Diff(want, healthRowFiles(t, &view)); diff != "" {
		t.Errorf("ordered by weight, the rows do not put the unreadable file among the heaviest, above the lighter finding and the unweighed one (-want +got):\n%s", diff)
	}
	unordered := view
	unordered.Sort = HealthByFinding
	if got := healthRowFiles(t, &unordered); slices.Equal(got, want) {
		t.Errorf("the rows are already in weight order before anything sorts them: %v", got)
	}
}

// TestOnlyTheUncitedNoteDrawsARowNoWeightWeighs is the sentence healthRules'
// comment makes, held mechanically. Every field of the view that puts findings
// on the page is filled on its own and the weight cell read back, so a kind
// that stops being weighed — or one that starts — shows up here rather than in
// a comment that has quietly stopped describing the code.
//
// It walks healthFindingFields, which TestEveryFieldOfTheHealthViewIsAccountedFor
// already proves names every such field: an enumeration written out a second
// time here could go short with nothing saying so.
func TestOnlyTheUncitedNoteDrawsARowNoWeightWeighs(t *testing.T) {
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
	if diff := cmp.Diff([]string{"Islands"}, unweighed); diff != "" {
		t.Errorf("the fields drawing a row no weight weighs disagree with what healthRules' comment says (-comment +page):\n%s", diff)
	}
}
