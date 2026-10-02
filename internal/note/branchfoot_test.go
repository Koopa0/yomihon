package note_test

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestASideBranchLessonLeavesItsBranchByTheCoursesOwnPages holds, through the
// real server, what the pages carry for a lesson on a side branch: the line
// above its title naming the book and the part and saying it is a branch, the
// labelled way back to the lesson it hangs from, the labelled way on to the
// main line's next lesson, and neither of them offered as a previous or a next
// lesson. The lesson the branch hangs from points at it and keeps the main
// line's own next, and a note no course teaches keeps the folder's neighbours
// and no running head.
//
// The view is assembled by the handler, so a field it stopped copying from the
// foot or the head would leave the page without it while every test of the
// pieces stayed green; this is where that would show.
func TestASideBranchLessonLeavesItsBranchByTheCoursesOwnPages(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	lesson := func(title string) string {
		return "---\ntitle: " + title + "\ntype: lesson\ndomain: golang\nstatus: draft\n---\n\nbody\n"
	}
	const dir = "Writing/lessons/golang/"
	write(dir+"Setup.md", lesson("Setup"))
	write(dir+"Basics.md", lesson("Basics"))
	write(dir+"Extra.md", lesson("Extra"))
	write(dir+"Wrapping up.md", lesson("Wrapping up"))
	write(dir+"Epilogue.md", lesson("Epilogue"))
	// In the same folder, taught by nothing.
	write(dir+"Zebra.md", "---\ntitle: Zebra\ntype: note\ndomain: golang\n---\n\nbody\n")
	write("Maps/Go course.md", "---\ntitle: Go course\ntype: study-path\ndomain: golang\n---\n\n"+
		"## start | Start | 開始 {sequence=primary}\n\n"+
		"- [[Setup]]\n"+
		"- [[Basics]]\n"+
		"\t- 選修 {sequence=local}\n"+
		"\t\t- [[Extra]]\n"+
		"- [[Wrapping up]]\n"+
		"\t- 後記 {sequence=local}\n"+
		"\t\t- [[Epilogue]]\n")

	srv := newServerWithContract(t, root, loadHomeContract(t))
	page := func(rel string) string {
		t.Helper()
		code, body := get(t, srv.Client(), srv.URL+"/notes/"+rel)
		if code != http.StatusOK {
			t.Fatalf("GET %s status = %d, want 200", rel, code)
		}
		return body
	}
	foot := func(body string) string {
		t.Helper()
		found := regexp.MustCompile(`(?s)<nav class="y-steps[^"]*".*?</nav>`).FindString(body)
		if found == "" {
			t.Fatalf("the page carries no foot at all")
		}
		return found
	}

	branch := page(dir + "Extra.md")
	for _, want := range []string{
		// The line above the title: the book, its part, and the word for a branch,
		// as one link to the contents with this lesson marked.
		`<div class="y-crumbs y-crumbs--course"><a class="y-crumbs__link" href="/syllabus/Maps/Go%20course.md?from=Writing%2Flessons%2Fgolang%2FExtra.md">Go course · Start · 支線</a></div>`,
	} {
		if !strings.Contains(branch, want) {
			t.Errorf("the branch lesson's head does not carry %q", want)
		}
	}
	branchFoot := foot(branch)
	for _, want := range []string{
		`y-steps__link--back" href="/notes/Writing/lessons/golang/Basics.md"`,
		`y-steps__link--onward" href="/notes/Writing/lessons/golang/Wrapping%20up.md"`,
	} {
		if !strings.Contains(branchFoot, want) {
			t.Errorf("the branch lesson's foot does not carry %q; foot = %q", want, branchFoot)
		}
	}
	if strings.Contains(branchFoot, `rel="next"`) || strings.Contains(branchFoot, `rel="prev"`) || strings.Contains(branchFoot, "同資料夾") {
		t.Errorf("the branch lesson's foot offers a step in the course's order, or the folder's; foot = %q", branchFoot)
	}

	anchorFoot := foot(page(dir + "Basics.md"))
	for _, want := range []string{
		// The course is named by the title alone, as a link to its contents.
		`<p class="y-steps__source"><a class="y-steps__path" href="/syllabus/Maps/Go%20course.md?from=Writing%2Flessons%2Fgolang%2FBasics.md">Go course</a></p>`,
		`href="/notes/Writing/lessons/golang/Wrapping%20up.md" rel="next"`,
		`<p class="y-steps__aside"><span class="y-steps__asidelabel">支線：</span><a class="y-steps__asidelink" href="/notes/Writing/lessons/golang/Extra.md">Extra</a></p>`,
	} {
		if !strings.Contains(anchorFoot, want) {
			t.Errorf("the lesson the branch hangs from does not carry %q; foot = %q", want, anchorFoot)
		}
	}

	// A branch hung from the course's last lesson has no main-line lesson to go
	// on to, so its way on is the contents.
	last := foot(page(dir + "Epilogue.md"))
	for _, want := range []string{
		`y-steps__link--back" href="/notes/Writing/lessons/golang/Wrapping%20up.md"`,
		`<a class="y-steps__link y-steps__link--next y-steps__link--onward" href="/syllabus/Maps/Go%20course.md?from=Writing%2Flessons%2Fgolang%2FEpilogue.md"><span class="y-steps__role">回到目錄</span></a>`,
	} {
		if !strings.Contains(last, want) {
			t.Errorf("the last branch lesson does not carry %q; foot = %q", want, last)
		}
	}

	plain := page(dir + "Zebra.md")
	if strings.Contains(plain, "y-crumbs--course") {
		t.Errorf("a note no course teaches carries a running head")
	}
	if !strings.Contains(foot(plain), "同資料夾的前後檔案") {
		t.Errorf("a note no course teaches lost the folder's foot")
	}
}
