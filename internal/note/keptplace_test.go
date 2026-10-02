package note_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/mark"
)

// keptCourse lays down a course of two lessons under the contract the desk
// tests use: Open and Done, listed in that order.
func keptCourse(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	lessons := filepath.Join(root, "Writing", "lessons")
	if err := os.MkdirAll(lessons, 0o750); err != nil {
		t.Fatalf("mkdir lessons: %v", err)
	}
	for _, name := range []string{"Open", "Done"} {
		body := "---\ntitle: " + name + "\ntype: lesson\nstatus: draft\n---\n\nbody\n"
		if err := os.WriteFile(filepath.Join(lessons, name+".md"), []byte(body), 0o600); err != nil {
			t.Fatalf("write lesson %s: %v", name, err)
		}
	}
	maps := filepath.Join(root, "Maps")
	if err := os.MkdirAll(maps, 0o750); err != nil {
		t.Fatalf("mkdir maps: %v", err)
	}
	course := "---\ntitle: Test path\ntype: study-path\n---\n\n## Part {sequence=primary}\n\n- [[Open]]\n- [[Done]]\n"
	if err := os.WriteFile(filepath.Join(maps, "path.md"), []byte(course), 0o600); err != nil {
		t.Fatalf("write the course: %v", err)
	}
	return root
}

// TestTheReadingPageDrawsTheReadersBookmarkWhereTheirPlaceIs holds the three
// places a kept place shows on a note: the control for keeping one, on the note
// that holds it; the row of the lesson that holds it in the book beside the
// note; and neither, for a reader who kept nothing. A place kept in another
// lesson of the same book marks that lesson's row and leaves this note's control
// as it was.
func TestTheReadingPageDrawsTheReadersBookmarkWhereTheirPlaceIs(t *testing.T) {
	t.Parallel()

	const open, done = "/notes/Writing/lessons/Open.md", "/notes/Writing/lessons/Done.md"
	for _, tt := range []struct {
		name        string
		kept        mark.Continuation
		marked      bool
		page        string
		wantControl bool
		wantRowFor  string
	}{
		{name: "the note holds the place", kept: mark.Continuation{RelPath: "Writing/lessons/Open.md"}, marked: true, page: open, wantControl: true, wantRowFor: open},
		{name: "another lesson of the book holds it", kept: mark.Continuation{RelPath: "Writing/lessons/Open.md"}, marked: true, page: done, wantControl: false, wantRowFor: open},
		{name: "no place kept", page: open, wantControl: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			contract := loadHomeContract(t)
			srv := newServerKeepingPlaces(t, keptCourse(t), contract, contract.Governance(), func() (mark.Continuation, bool) { return tt.kept, tt.marked }, "/mark")
			code, body := get(t, srv.Client(), srv.URL+tt.page)
			if code != 200 {
				t.Fatalf("GET %s = %d, want 200", tt.page, code)
			}
			if !strings.Contains(body, "data-mark-control") {
				t.Fatal("the page offers no control for keeping a place, so nothing below proves where the bookmark is drawn")
			}
			if got := strings.Contains(body, " data-mark-kept"); got != tt.wantControl {
				t.Errorf("the control wears the bookmark = %v, want %v", got, tt.wantControl)
			}
			wantRows := 0
			if tt.wantRowFor != "" {
				wantRows = 1
			}
			if got := strings.Count(body, `class="y-keptplace"`); got != wantRows {
				t.Fatalf("%d rows carry the bookmark, want %d", got, wantRows)
			}
			if tt.wantRowFor == "" {
				return
			}
			opening := regexp.MustCompile(`<a class="ui-navitem[^"]*"\s+href="` + regexp.QuoteMeta(tt.wantRowFor) + `"`).FindStringIndex(body)
			if opening == nil {
				t.Fatalf("the book beside the note has no row for %s", tt.wantRowFor)
			}
			row, _, _ := strings.Cut(body[opening[1]:], "</a>")
			if !strings.Contains(row, `class="y-keptplace"`) {
				t.Errorf("the bookmark is not on the row of %s: %s", tt.wantRowFor, row)
			}
		})
	}
}
