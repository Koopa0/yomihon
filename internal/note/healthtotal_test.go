package note_test

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
)

// railFootFindings is the number the foot of every rail states, carried on the
// element beside the words so a reader of this file does not have to parse a
// sentence in two languages to learn what the foot claims.
var railFootFindings = regexp.MustCompile(`data-rail-foot-findings="(\d+)"`)

// TestTheRailFootStatesWhatTheHealthTableHolds is the one instrument standing
// between two faces of the same number. Every page's rail says how many things
// the folder has to answer for, and the health page lists them with a count on
// each line; if those two are ever worked out separately, they disagree the day
// one of them learns about a kind of finding the other does not.
//
// Both numbers are read off the health page itself, so the comparison is made
// on bytes a reader could have read: the rail's claim, and the sum of the lines
// the table actually drew.
func TestTheRailFootStatesWhatTheHealthTableHolds(t *testing.T) {
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

	code, page := get(t, srv.Client(), srv.URL+"/health")
	if code != http.StatusOK {
		t.Fatalf("health status = %d, want 200", code)
	}

	claim := railFootFindings.FindStringSubmatch(page)
	if claim == nil {
		t.Fatal("the health page carries no rail foot, so nothing here reads what the rail claims")
	}
	claimed, err := strconv.Atoi(claim[1])
	if err != nil {
		t.Fatalf("the rail foot claims %q findings, which is no number: %v", claim[1], err)
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
		t.Fatal("the table's lines add up to nothing, so this fixture states nothing for the rail to agree with")
	}
	if claimed != drawn {
		t.Errorf("the rail foot claims %d findings and the table's %d lines hold %d", claimed, len(rows), drawn)
	}
}
