package pages

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/wording"
)

// healthShapeLineRe finds the whole shape line as one block, so its absence and
// its content are both read from the same place.
var healthShapeLineRe = regexp.MustCompile(`(?s)<p class="y-healthshape">(.*?)</p>`)

// healthShapeWeightRe finds one weight entry inside that line: the link it
// carries, the word the judge names the weight by, and the count beside it.
var healthShapeWeightRe = regexp.MustCompile(`<a class="y-healthshape__weight" href="([^"]*)"><span class="y-severity y-severity--[a-z]+" lang="en">([a-z]+)</span> (\d+)</a>`)

// healthShapeFilesRe finds the plain files count that closes the line.
var healthShapeFilesRe = regexp.MustCompile(`<span>([^<]*)</span>$`)

// healthFileIdentityRe reads what one file cell of the findings table actually
// names: the href of a note's own link where the row has one, and the plain
// path otherwise — the same two spellings healthRow.fileKey keeps apart, read
// back a second time from the rendered page rather than from the struct.
var healthFileIdentityRe = regexp.MustCompile(`href="([^"]*)"|<span class="y-findings__path">([^<]*)</span>`)

// TestHealthShapeLineCountsMatchTheTable is the lock the issue asks for: the
// shape line's own numbers, read back off the page, equal what a second count
// of the table's rows produces — by weight and by file — so the two can never
// print different answers about the same table.
func TestHealthShapeLineCountsMatchTheTable(t *testing.T) {
	t.Parallel()
	view := recordedHealthView(buildModel(t))

	page := renderHealth(t, &view)
	shapeMatch := healthShapeLineRe.FindStringSubmatch(page)
	if shapeMatch == nil {
		t.Fatal("the fixture holds findings, so the page must carry the shape line")
	}
	shape := shapeMatch[1]

	// By weight: sum the table's own count cells under each weight its own
	// severity cells carry, and compare against what the line shows.
	weights := healthRowWeights(t, &view)
	counts := healthRowCounts(t, &view)
	if len(weights) != len(counts) {
		t.Fatalf("read %d weight cells and %d count cells, want the same number", len(weights), len(counts))
	}
	want := make(map[string]int)
	for i, w := range weights {
		if w < 0 {
			continue
		}
		want[judge.Severity(w).String()] += counts[i]
	}

	got := make(map[string]int)
	matches := healthShapeWeightRe.FindAllStringSubmatch(shape, -1)
	if len(matches) == 0 {
		t.Fatal("the fixture carries weighed findings, so the shape line must name at least one weight")
	}
	for _, m := range matches {
		href, word, countText := m[1], m[2], m[3]
		if href != HealthBySeverity.href() {
			t.Errorf("the %s entry links to %q, want %q — the ordering the header row already answers", word, href, HealthBySeverity.href())
		}
		n, err := strconv.Atoi(countText)
		if err != nil {
			t.Fatalf("the %s entry counts %q, which is no number: %v", word, countText, err)
		}
		got[word] = n
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("the shape line's weight tally disagrees with a second count of the table (-table +line):\n%s", diff)
	}

	// By file: dedupe the table's own file cells and compare against the
	// number the line closes with.
	files := healthRowFileIdentities(t, page)
	wantFiles := make(map[string]struct{}, len(files))
	for _, f := range files {
		wantFiles[f] = struct{}{}
	}
	filesMatch := healthShapeFilesRe.FindStringSubmatch(shape)
	if filesMatch == nil {
		t.Fatal("the shape line does not close with a plain files count")
	}
	gotFiles := strings.Fields(filesMatch[1])[0]
	if gotFiles != strconv.Itoa(len(wantFiles)) {
		t.Errorf("the shape line says %q files, a second count of the table's file cells says %d", gotFiles, len(wantFiles))
	}
}

// TestHealthShapeLineOrdersWeightsHeaviestFirst is the same direction the
// severity column itself sorts by: a reader who wants to know how bad things
// are reads the worst of it first.
func TestHealthShapeLineOrdersWeightsHeaviestFirst(t *testing.T) {
	t.Parallel()
	view := recordedHealthView(buildModel(t))
	page := renderHealth(t, &view)
	shapeMatch := healthShapeLineRe.FindStringSubmatch(page)
	if shapeMatch == nil {
		t.Fatal("the fixture holds findings, so the page must carry the shape line")
	}
	var words []string
	for _, m := range healthShapeWeightRe.FindAllStringSubmatch(shapeMatch[1], -1) {
		words = append(words, m[2])
	}
	want := []string{judge.SeverityError.String(), judge.SeverityWarn.String()}
	if diff := cmp.Diff(want, words); diff != "" {
		t.Errorf("the shape line's weight order disagrees with heaviest-first (-want +got):\n%s", diff)
	}
}

// TestHealthShapeLineCountsAFileNoWeightWeighs holds the file total open past
// what the weights beside it add up to: a note nothing cites is the kind no
// rule reports, so its rows carry no weight, and the line still has to count
// them among the files the table names. Two of them, in two folders, because a
// single row would leave the count agreeing with the number of rows by
// accident.
func TestHealthShapeLineCountsAFileNoWeightWeighs(t *testing.T) {
	t.Parallel()
	view := HealthView{
		Islands: []HealthIslandGroup{
			{Dir: "Notes", Name: "Notes", Notes: []nav.NoteRef{{Name: "Alone", RelPath: "Notes/Alone.md"}}},
			{Dir: "Drafts", Name: "Drafts", Notes: []nav.NoteRef{{Name: "Aside", RelPath: "Drafts/Aside.md"}}},
		},
		IslandCount: 2,
	}
	page := renderHealth(t, &view)
	shapeMatch := healthShapeLineRe.FindStringSubmatch(page)
	if shapeMatch == nil {
		t.Fatal("the fixture holds findings, so the page must carry the shape line")
	}
	if matches := healthShapeWeightRe.FindAllStringSubmatch(shapeMatch[1], -1); len(matches) != 0 {
		t.Errorf("neither finding here carries a weight, so the line should name none; it named %v", matches)
	}
	filesMatch := healthShapeFilesRe.FindStringSubmatch(shapeMatch[1])
	if filesMatch == nil {
		t.Fatal("the shape line does not close with a plain files count")
	}
	if got := strings.Fields(filesMatch[1])[0]; got != "2" {
		t.Errorf("two unweighed findings about two files should still count 2 files; the line says %q", got)
	}
}

// TestHealthShapeLineFilesTextAgreesWithTheHeaderWord pins the exact words the
// files count closes with, in both languages and at the singular boundary —
// the one number plural() treats differently from every other.
func TestHealthShapeLineFilesTextAgreesWithTheHeaderWord(t *testing.T) {
	t.Parallel()
	view := HealthView{Blocked: []HealthBlockedSource{{Path: "Sources/Raw.md", Reason: "permission denied"}}}
	for _, tt := range []struct {
		lang wording.Lang
		want string
	}{
		{wording.ZhHant, "1 個檔案"},
		{wording.En, "1 file"},
	} {
		t.Run(string(tt.lang), func(t *testing.T) {
			t.Parallel()
			var buf strings.Builder
			if err := Health(view, layouts.Chrome{Lang: tt.lang}).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render the health page: %v", err)
			}
			shapeMatch := healthShapeLineRe.FindStringSubmatch(buf.String())
			if shapeMatch == nil {
				t.Fatal("a blocked source is a finding, so the page must carry the shape line")
			}
			filesMatch := healthShapeFilesRe.FindStringSubmatch(shapeMatch[1])
			if filesMatch == nil {
				t.Fatal("the shape line does not close with a plain files count")
			}
			if filesMatch[1] != tt.want {
				t.Errorf("the shape line reads %q, want %q", filesMatch[1], tt.want)
			}
		})
	}
}

// TestHealthShapeLineIsAbsentWithNoRowsToCount holds the other half of the
// contract: a page with no findings table draws no shape line either, rather
// than one naming zero of everything.
func TestHealthShapeLineIsAbsentWithNoRowsToCount(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		view HealthView
	}{
		{"nothing to report", HealthView{}},
		{"citations could not be evaluated", HealthView{InstanceScopeUnknown: "the index was not built"}},
		{"the vocabulary could not be read", HealthView{SchemaScopeUnknown: "the contract could not be read"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			page := renderHealth(t, &tt.view)
			if healthShapeLineRe.MatchString(page) {
				t.Error("the page draws a shape line over a table that draws no rows")
			}
		})
	}
}

// TestHealthShapeLineWeightWordsStayEnglish holds the same rule the table's
// own severity badges follow: the judge's vocabulary does not translate, in
// either language the chrome around it speaks.
func TestHealthShapeLineWeightWordsStayEnglish(t *testing.T) {
	t.Parallel()
	view := recordedHealthView(buildModel(t))
	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		t.Run(string(lang), func(t *testing.T) {
			t.Parallel()
			var buf strings.Builder
			if err := Health(view, layouts.Chrome{Lang: lang}).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render the health page: %v", err)
			}
			shapeMatch := healthShapeLineRe.FindStringSubmatch(buf.String())
			if shapeMatch == nil {
				t.Fatal("the fixture holds findings, so the page must carry the shape line")
			}
			matches := healthShapeWeightRe.FindAllStringSubmatch(shapeMatch[1], -1)
			if len(matches) == 0 {
				t.Fatal("the fixture carries weighed findings, so the shape line must name at least one weight")
			}
			for _, m := range matches {
				if _, ok := severityNamed(m[2]); !ok {
					t.Errorf("the shape line names the weight %q, which is no word the judging face uses", m[2])
				}
			}
		})
	}
}

// healthRowFileIdentities is what each row of the findings table says it is
// about, read as the identity healthRow.fileKey would compare rather than as
// display text: the href of a note's own link where the row has one, and the
// plain path otherwise.
func healthRowFileIdentities(t *testing.T, page string) []string {
	t.Helper()
	var out []string
	for _, cell := range fileCell.FindAllStringSubmatch(page, -1) {
		m := healthFileIdentityRe.FindStringSubmatch(cell[1])
		if m == nil {
			t.Fatalf("a file cell reads %q, which names neither a note link nor a path", cell[1])
		}
		if m[1] != "" {
			out = append(out, "note:"+m[1])
			continue
		}
		out = append(out, "path:"+m[2])
	}
	if len(out) == 0 {
		t.Fatal("no row of the findings table names a file")
	}
	return out
}
