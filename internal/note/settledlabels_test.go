package note_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

// settledHomeContract is the desk tests' contract with settled written on the
// lifecycle row of each named status. It edits the shared file's rows in text,
// so there is no second contract to drift from it, and a status with no row
// fails the test rather than settling nothing.
func settledHomeContract(t *testing.T, settled ...string) *schema.Contract {
	t.Helper()
	base, err := os.ReadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("read the shared contract: %v", err)
	}
	const header = "\n[[lifecycle]]\n"
	rows := strings.Split(string(base), header)
	matched := map[string]bool{}
	for i := 1; i < len(rows); i++ {
		for _, status := range settled {
			if strings.HasPrefix(rows[i], `status = "`+status+`"`) {
				rows[i] = strings.TrimRight(rows[i], "\n") + "\nsettled = true\n"
				matched[status] = true
			}
		}
	}
	for _, status := range settled {
		if !matched[status] {
			t.Fatalf("the shared contract has no lifecycle row for %q", status)
		}
	}
	path := filepath.Join(t.TempDir(), "vault-schema.toml")
	if err = os.WriteFile(path, []byte(strings.Join(rows, header)), 0o600); err != nil { // #nosec G703 -- a fixed contract path under this test's own temporary directory
		t.Fatalf("write the settled contract: %v", err)
	}
	contract, err := schema.LoadFile(path)
	if err != nil {
		t.Fatalf("schema.LoadFile: %v", err)
	}
	return contract
}

// TestTheFoldersAndTheRailMarkOnlyTheNotesNotYetSettled drives the label rule
// through the real server: a vault with one lesson at the settled status and one
// still at draft. The recent list on the folders page and the book beside a note
// print the draft word for the open lesson and nothing for the finished one, and
// the same vault under a contract that settles nothing prints both.
func TestTheFoldersAndTheRailMarkOnlyTheNotesNotYetSettled(t *testing.T) {
	t.Parallel()

	vaultWith := func(t *testing.T) string {
		t.Helper()
		root := keptCourse(t)
		done := filepath.Join(root, "Writing", "lessons", "Done.md")
		if err := os.WriteFile(done, []byte("---\ntitle: Done\ntype: lesson\nstatus: ready\n---\n\nbody\n"), 0o600); err != nil {
			t.Fatalf("settle a lesson: %v", err)
		}
		return root
	}
	rowOf := func(body, title string) string {
		_, row, found := strings.Cut(body, `data-home-recent-note`)
		for found && !strings.Contains(strings.SplitN(row, "</a>", 2)[0], ">"+title+"<") {
			_, row, found = strings.Cut(row, `data-home-recent-note`)
		}
		if !found {
			t.Fatalf("the recent list has no row for %q", title)
		}
		row, _, _ = strings.Cut(row, "</a>")
		return row
	}

	t.Run("a contract that settles ready", func(t *testing.T) {
		t.Parallel()
		contract := settledHomeContract(t, "ready")
		srv := newServerKeepingPlaces(t, vaultWith(t), contract, contract.Governance(), noMark, "")

		_, folders := get(t, srv.Client(), srv.URL+"/folders")
		if row := rowOf(folders, "Open"); !strings.Contains(row, `<span class="ui-status">draft</span>`) {
			t.Errorf("the open lesson's recent row prints no status: %s", row)
		}
		if row := rowOf(folders, "Done"); strings.Contains(row, "ui-status") {
			t.Errorf("the finished lesson's recent row prints a status: %s", row)
		}

		_, page := get(t, srv.Client(), srv.URL+"/notes/Writing/lessons/Open.md")
		if rail := railOf(t, page); strings.Contains(rail, `>ready<`) {
			t.Errorf("the book prints the finished lesson's status: %s", rail)
		}
	})

	t.Run("a contract that settles nothing", func(t *testing.T) {
		t.Parallel()
		contract := loadHomeContract(t)
		srv := newServerKeepingPlaces(t, vaultWith(t), contract, contract.Governance(), noMark, "")

		_, folders := get(t, srv.Client(), srv.URL+"/folders")
		if row := rowOf(folders, "Done"); !strings.Contains(row, `<span class="ui-status">ready</span>`) {
			t.Errorf("the fallback prints no status for the finished lesson, so the case above proves nothing: %s", row)
		}
		_, page := get(t, srv.Client(), srv.URL+"/notes/Writing/lessons/Open.md")
		if !strings.Contains(railOf(t, page), `<span class="ui-navitem__count">ready</span>`) {
			t.Error("the fallback book prints no status for the finished lesson, so the rail case above proves nothing")
		}
	})
}

// railOf is the left rail of a reading page: the book beside the note.
func railOf(t *testing.T, page string) string {
	t.Helper()
	_, rail, found := strings.Cut(page, `id="_y-nav-rail"`)
	if !found {
		t.Fatal("the page has no left rail")
	}
	rail, _, _ = strings.Cut(rail, "</aside>")
	return rail
}
