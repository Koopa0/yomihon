package note_test

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// findingsRow is one line of the whole-folder findings table: what it is about,
// and how many findings of its kind that file carries.
var findingsRow = regexp.MustCompile(`(?s)<tr><td class="y-findings__file".*?</tr>`)

var rowCount = regexp.MustCompile(`<td class="y-findings__count"[^>]*>(\d+)</td>`)

// TestOneNoteWithSeveralSchemaFaultsCountsThemAll holds the number the table's
// one line for that note stands for. The page gathers a note the schema
// objected to as a single line, because what a reader does about it is open
// that note once; the line therefore has to say how much is waiting there, and
// a page that showed a note with five faults exactly as it showed a note with
// one would be hiding the difference a reader is choosing on.
//
// The fixture writes three separate objections into one note: a type the
// schema does not declare, a key it does not know, and a slug whose shape it
// rejects.
func TestOneNoteWithSeveralSchemaFaultsCountsThemAll(t *testing.T) {
	t.Parallel()

	const rel = "Concepts/golang/Faulty.md"
	const body = "---\ntitle: Faulty\ntype: memorandum\ndomain: golang\nstatus: draft\nslug: Not Kebab\nnot_a_field: 1\ncreated: 2026-06-01\nupdated: 2026-06-01\n---\n\nbody\n"
	srv := newServerWithContract(t, writeOneNote(t, rel, body), loadHomeContract(t))

	code, page := get(t, srv.Client(), srv.URL+"/health")
	if code != http.StatusOK {
		t.Fatalf("health status = %d, want 200", code)
	}
	rows := findingsRow.FindAllString(page, -1)
	if len(rows) == 0 {
		t.Fatal("the health page has no findings table at all, so nothing below reads what it says")
	}
	counted := 0
	seen := 0
	for _, row := range rows {
		if !strings.Contains(row, "schema 有話說的筆記") {
			continue
		}
		seen++
		match := rowCount.FindStringSubmatch(row)
		if match == nil {
			t.Fatalf("the schema row carries no count: %s", row)
		}
		count, err := strconv.Atoi(match[1])
		if err != nil {
			t.Fatalf("the schema row counted %q, which is no number: %v", match[1], err)
		}
		counted += count
	}
	if seen != 1 {
		t.Fatalf("the page drew %d rows for one note the schema objected to, want exactly 1", seen)
	}
	if counted != 3 {
		t.Errorf("the row for a note with three schema faults counts %d of them", counted)
	}
}
