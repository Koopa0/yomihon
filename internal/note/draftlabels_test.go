package note_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/wording"
)

// TestTheFoldersAndTheRailMarkOnlyDrafts drives the label rule through the
// real server: a vault with one lesson that declares draft: true and one that
// does not. The recent list on the folders page and the book beside a note
// print the draft word for the draft and nothing for the finished lesson.
func TestTheFoldersAndTheRailMarkOnlyDrafts(t *testing.T) {
	t.Parallel()

	root := keptCourse(t)
	for name, flag := range map[string]string{"Open": "draft: true\n", "Done": ""} {
		body := "---\ntitle: " + name + "\ntype: lesson\n" + flag + "---\n\nbody\n"
		if err := os.WriteFile(filepath.Join(root, "Writing", "lessons", name+".md"), []byte(body), 0o600); err != nil {
			t.Fatalf("write lesson %s: %v", name, err)
		}
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
	draft := wording.Draft.In(wording.ZhHant)
	contract := loadHomeContract(t)
	srv := newServerKeepingPlaces(t, root, contract, contract.Governance(), noMark, "")

	_, folders := get(t, srv.Client(), srv.URL+"/folders")
	if row := rowOf(folders, "Open"); !strings.Contains(row, `<span class="ui-status">`+draft+`</span>`) {
		t.Errorf("the draft lesson's recent row is not marked: %s", row)
	}
	if row := rowOf(folders, "Done"); strings.Contains(row, "ui-status") {
		t.Errorf("the finished lesson's recent row carries a mark: %s", row)
	}

	_, page := get(t, srv.Client(), srv.URL+"/notes/Writing/lessons/Done.md")
	rail := railOf(t, page)
	if !strings.Contains(rail, `<span class="ui-navitem__count">`+draft+`</span>`) {
		t.Errorf("the book does not mark the draft lesson beside a finished one: %s", rail)
	}
	if strings.Count(rail, draft) != 1 {
		t.Errorf("the book marks %d rows, want the one draft: %s", strings.Count(rail, draft), rail)
	}
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
