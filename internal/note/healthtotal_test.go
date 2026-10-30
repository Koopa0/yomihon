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

// deskHealthFindings is the number the desk's way to the health page states,
// carried on the element beside the words so a reader of this file does not
// have to parse a sentence in two languages to learn what the desk claims.
var deskHealthFindings = regexp.MustCompile(`data-home-health-findings="(\d+)"`)

// deskHealthClaim is the number the desk states.
func deskHealthClaim(t *testing.T, page string) int {
	t.Helper()
	claim := deskHealthFindings.FindStringSubmatch(page)
	if claim == nil {
		t.Fatal("the page carries no way to the health page from the desk")
	}
	count, err := strconv.Atoi(claim[1])
	if err != nil {
		t.Fatalf("the desk claims %q findings, which is no number: %v", claim[1], err)
	}
	return count
}

// TestTheDeskStatesWhatTheHealthTableHolds is the one instrument standing
// between two faces of the same number. The desk says how many things the
// folder has to answer for, and the health page lists them with a count on
// each line; if those two are ever worked out separately, they disagree the day
// one of them learns about a kind of finding the other does not.
//
// The comparison is made on bytes a reader could have read: the desk's claim,
// and the sum of the lines the table actually drew.
func TestTheDeskStatesWhatTheHealthTableHolds(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for rel, body := range map[string]string{
		// Three objections in one note — an undeclared key, a slug the schema
		// rejects, a type it does not know — plus a citation nothing answers.
		"Concepts/golang/Faulty.md": "---\ntitle: Faulty\ntype: memorandum\ndomain: golang\nstatus: draft\nslug: Not Kebab\nnot_a_field: 1\ncreated: 2026-06-01\nupdated: 2026-06-01\n---\n\nbody with [[Ghost]]\n",
		// A note no text reaches, which the page counts one line at a time
		// where the schema faults above share one.
		"Notes/Island.md": "---\ntitle: Island\ntype: memorandum\ndomain: golang\nstatus: draft\ncreated: 2026-06-01\nupdated: 2026-06-01\n---\n\nalone\n",
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

	code, desk := get(t, srv.Client(), srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("desk status = %d, want 200", code)
	}
	claimed := deskHealthClaim(t, desk)
	code, page := get(t, srv.Client(), srv.URL+"/health")
	if code != http.StatusOK {
		t.Fatalf("health status = %d, want 200", code)
	}

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
		t.Errorf("the desk claims %d findings and the table's %d lines hold %d", claimed, len(rows), drawn)
	}
}

func TestHealthAndDeskDoNotCountNavigationEntriesAsIslands(t *testing.T) {
	t.Parallel()
	for _, unlisted := range []bool{false, true} {
		name := "complete course"
		if unlisted {
			name = "unlisted lesson"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			notes := map[string]string{
				"Maps/Course.md":    "---\ntitle: Course\ntype: study-path\ndomain: golang\nstatus: archived\ncreated: 2026-06-01\nupdated: 2026-06-01\n---\n\n## Unit\n- [[Listed]]\n",
				"Maps/Overview.md":  "---\ntitle: Overview\ntype: moc\ndomain: golang\nstatus: archived\ncreated: 2026-06-01\nupdated: 2026-06-01\n---\n\n- [[Listed]]\n",
				"Writing/Listed.md": "---\ntitle: Listed\nslug: listed\ntype: lesson\ndomain: golang\nstatus: draft\ncreated: 2026-06-01\nupdated: 2026-06-01\n---\n\nLesson text.\n",
			}
			want := 0
			if unlisted {
				want = 1
				notes["Writing/Unlisted.md"] = "---\ntitle: Unlisted\nslug: unlisted\ntype: lesson\ndomain: golang\nstatus: draft\ncreated: 2026-06-01\nupdated: 2026-06-01\n---\n\nLesson text.\n"
			}
			srv := newServerWithContract(t, writeNotes(t, notes), loadHomeContract(t))
			code, desk := get(t, srv.Client(), srv.URL+"/")
			if code != http.StatusOK {
				t.Fatalf("GET / status = %d, want 200", code)
			}
			if got := deskHealthClaim(t, desk); got != want {
				t.Errorf("caught: desk count = %d, want %d", got, want)
			}
			code, page := get(t, srv.Client(), srv.URL+"/health?page=all")
			if code != http.StatusOK {
				t.Fatalf("GET /health status = %d, want 200", code)
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
