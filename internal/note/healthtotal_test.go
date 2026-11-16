package note_test

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// statedTotal is the number the health page's shape line states for the whole
// report, which is zero where the page draws no shape line because it has
// nothing to tabulate.
var statedTotal = regexp.MustCompile(`<span class="y-healthshape__total">(\d+) `)

// healthClaim is the number the page states.
func healthClaim(t *testing.T, page string) int {
	t.Helper()
	claim := statedTotal.FindStringSubmatch(page)
	if claim == nil {
		return 0
	}
	count, err := strconv.Atoi(claim[1])
	if err != nil {
		t.Fatalf("the page states %q findings, which is no number: %v", claim[1], err)
	}
	return count
}

// TestTheHealthPageStatesWhatItsTableHolds is the one instrument standing
// between two faces of the same number. The page says how many things the
// folder has to answer for above its table and lists them with a count on each
// line; if those two are ever worked out separately, they disagree the day one
// of them learns about a kind of finding the other does not.
//
// The comparison is made on bytes a reader could have read: the stated total,
// and the sum of the lines the table actually drew.
func TestTheHealthPageStatesWhatItsTableHolds(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for rel, body := range map[string]string{
		// Three objections in one note — an undeclared key, a slug the schema
		// rejects, a type it does not know — plus a citation nothing answers.
		"Concepts/golang/Faulty.md": "---\ntype: memorandum\nslug: Not Kebab\nnot_a_field: 1\ncreated: 2026-06-01\n---\n# Faulty\n\nbody with [[Ghost]]\n",
		// A note no text reaches, which the page counts one line at a time
		// where the schema faults above share one.
		"Notes/Island.md": "---\ntype: memorandum\ncreated: 2026-06-01\n---\n# Island\n\nalone\n",
	} {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	srv := newServerWithContract(t, root, loadHomeContract(t))

	code, page := get(t, srv.Client(), srv.URL+"/health")
	if code != http.StatusOK {
		t.Fatalf("health status = %d, want 200", code)
	}
	claimed := healthClaim(t, page)

	drawn := 0
	rows := rowCount.FindAllStringSubmatch(page, -1)
	if len(rows) == 0 {
		t.Fatal("the findings table drew no line, so the comparison below would hold at zero either way")
	}
	for _, row := range rows {
		count, convErr := strconv.Atoi(row[1])
		if convErr != nil {
			t.Fatalf("a findings row counts %q, which is no number: %v", row[1], convErr)
		}
		drawn += count
	}
	if drawn == 0 {
		t.Fatal("the table's lines add up to nothing, so this fixture states nothing for the desk to agree with")
	}
	if claimed != drawn {
		t.Errorf("the page states %d findings and the table's %d lines hold %d", claimed, len(rows), drawn)
	}
}

func TestHealthDoesNotCountNavigationEntriesAsIslands(t *testing.T) {
	t.Parallel()
	for _, unlisted := range []bool{false, true} {
		name := "complete course"
		if unlisted {
			name = "unlisted lesson"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			notes := map[string]string{
				"Maps/Course.md":    "---\ntype: study-path\ncreated: 2026-06-01\n---\n# Course\n\n## Unit\n- [[Listed]]\n",
				"Maps/Overview.md":  "---\ntype: moc\ncreated: 2026-06-01\n---\n# Overview\n\n- [[Listed]]\n",
				"Writing/Listed.md": "---\nslug: listed\ntype: lesson\ncreated: 2026-06-01\n---\n# Listed\n\nLesson text.\n",
			}
			want := 0
			if unlisted {
				want = 1
				notes["Writing/Unlisted.md"] = "---\nslug: unlisted\ntype: lesson\ncreated: 2026-06-01\n---\n# Unlisted\n\nLesson text.\n"
			}
			srv := newServerWithContract(t, writeNotes(t, notes), loadHomeContract(t))
			code, page := get(t, srv.Client(), srv.URL+"/health?page=all")
			if code != http.StatusOK {
				t.Fatalf("GET /health status = %d, want 200", code)
			}
			if got := healthClaim(t, page); got != want {
				t.Errorf("caught: stated total = %d, want %d", got, want)
			}
			rows := rowCount.FindAllStringSubmatch(page, -1)
			if len(rows) != want {
				t.Errorf("caught: health table rows = %d, want %d", len(rows), want)
			}
			if unlisted && !strings.Contains(page, `href="/notes/Writing/Unlisted.md"`) {
				t.Error("caught: unlisted lesson is missing from Health")
			}
		})
	}
}
