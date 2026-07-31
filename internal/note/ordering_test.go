package note_test

import (
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// The sidebar is how a reader walks a numbered folder, and it was handing them
// 第三課 above 第二課 — code-point order, where 三 is U+4E09 and 二 is U+4E8C.
// A teacher hit it on two lessons and said a full semester of it would make the
// sidebar something she stopped trusting.
//
// This asserts the order on the page rather than in the comparison function,
// because the comparison being right proves nothing if nothing calls it.
func TestSidebarListsNumberedLessonsInLessonOrder(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dir := filepath.Join(root, "教案")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	// Written to disk in an order that is neither the reading order nor the
	// code-point one, so neither can pass by accident.
	for _, name := range []string{"第十課 溶液", "第二課 空氣", "第一課 物質", "第三課 水", "第十二課 有機物"} {
		body := "# " + name + "\n\n本文。\n"
		if err := os.WriteFile(filepath.Join(dir, name+".md"), []byte(body), 0o600); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", name, err)
		}
	}

	srv := newServerWithContract(t, root, loadHomeContract(t))
	code, body := get(t, srv.URL+"/notes/教案/第一課 物質.md")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want %d", code, http.StatusOK)
	}

	asideStart := strings.Index(body, `<aside class="y-rail-left"`)
	if asideStart < 0 {
		t.Fatal("response has no left sidebar")
	}
	asideEnd := strings.Index(body[asideStart:], `</aside>`)
	if asideEnd < 0 {
		t.Fatal("response has no complete left sidebar")
	}
	sidebar := body[asideStart : asideStart+asideEnd]

	want := []string{"第一課 物質", "第二課 空氣", "第三課 水", "第十課 溶液", "第十二課 有機物"}
	got := make([]string, 0, len(want))
	for cursor := 0; ; {
		best, bestName := -1, ""
		for _, name := range want {
			if at := strings.Index(sidebar[cursor:], name); at >= 0 && (best < 0 || at < best) {
				best, bestName = at, name
			}
		}
		if best < 0 {
			break
		}
		if !slices.Contains(got, bestName) {
			got = append(got, bestName)
		}
		cursor += best + len(bestName)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("sidebar lesson order mismatch (-want +got):\n%s", diff)
	}
}
