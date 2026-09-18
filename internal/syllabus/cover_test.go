package syllabus_test

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/mark"
	"github.com/koopa0/yomihon/internal/wording"
)

// coverOpening finds the paragraph block a course prints under its title, or
// reports that the page drew none.
var coverOpening = regexp.MustCompile(`(?s)<div class="y-cover__opening[^"]*"[^>]*>(.*?)</div>`)

// coverAction finds the one verb the cover offers, whole, so a failure can
// state the link as the reader met it.
var coverAction = regexp.MustCompile(`(?s)<a class="y-cover__open"[^>]*>.*?</a>`)

// writeCourse writes a vault holding one study path and the lessons it lists.
// opening is whatever the course note carries above its first heading — empty
// for a note that opens straight on one.
func writeCourse(t *testing.T, root, opening string, rows ...string) {
	t.Helper()
	lessonDir := filepath.Join(root, "Writing", "lessons", "golang")
	if err := os.MkdirAll(lessonDir, 0o750); err != nil {
		t.Fatalf("mkdir the lesson folder: %v", err)
	}
	for _, name := range []string{"First", "Second", "Third"} {
		lesson := "---\ntitle: " + name + "\ntype: lesson\ndomain: golang\nstatus: draft\n---\n\nbody\n"
		if err := os.WriteFile(filepath.Join(lessonDir, name+".md"), []byte(lesson), 0o600); err != nil {
			t.Fatalf("write the lesson %s: %v", name, err)
		}
	}
	elsewhere := "---\ntitle: Elsewhere\ntype: concept\ndomain: golang\n---\n\nbody\n"
	if err := os.WriteFile(filepath.Join(root, "Elsewhere.md"), []byte(elsewhere), 0o600); err != nil {
		t.Fatalf("write the note outside the course: %v", err)
	}
	mapsDir := filepath.Join(root, "Maps")
	if err := os.MkdirAll(mapsDir, 0o750); err != nil {
		t.Fatalf("mkdir Maps: %v", err)
	}
	course := "---\ntitle: Go path\ntype: study-path\ndomain: golang\n---\n" + opening +
		"## data | Data | 資料 {sequence=primary}\n\n" + strings.Join(rows, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(mapsDir, "Go path.md"), []byte(course), 0o600); err != nil {
		t.Fatalf("write the study path: %v", err)
	}
}

const coursePage = "/syllabus/Maps/Go path.md"

// TestTheCoverPrintsTheCourseNotesOwnOpening holds the two halves of one rule:
// a course prints the words its author wrote above the first heading, and a
// course note that wrote none prints nothing in their place — while still
// offering the verb that opens it. Without the first row the second passes for
// a page that prints no opening at all, and the needle it looks for could have
// stopped matching anything.
func TestTheCoverPrintsTheCourseNotesOwnOpening(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name    string
		opening string
		want    string
	}{
		{
			name:    "a note whose author wrote an opening",
			opening: "\nWhat this course is, and who it is for.\n\n",
			want:    "<p>What this course is, and who it is for.</p>",
		},
		{
			name: "a note that opens on its first heading",
			// Not even a blank line of its own: the note goes straight from
			// the frontmatter to the heading that declares its first part.
			opening: "",
			want:    "",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writeCourse(t, root, tt.opening, "- [[First]]", "- [[Second]]", "- [[Third]]")
			srv := newServer(t, root)

			code, body := get(t, srv.Client(), srv.URL+coursePage)
			if code != http.StatusOK {
				t.Fatalf("GET %s status = %d, want 200", coursePage, code)
			}
			got := coverOpening.FindStringSubmatch(body)
			switch {
			case tt.want == "" && got != nil:
				t.Errorf("the cover printed an opening for a note that has none: %q", got[0])
			case tt.want != "" && got == nil:
				t.Fatalf("the cover printed no opening; page = %q", body)
			case tt.want != "" && !strings.Contains(got[1], tt.want):
				t.Errorf("the cover printed %q, want it to carry %q", got[1], tt.want)
			}
			// The verb is the same offer either way: an author who wrote no
			// opening has not taken the way into their own course away.
			action := coverAction.FindString(body)
			if action == "" {
				t.Fatalf("the cover offers no way into the course; page = %q", body)
			}
			if !strings.Contains(action, `href="/notes/Writing/lessons/golang/First.md"`) {
				t.Errorf("the verb leads somewhere other than the first lesson: %s", action)
			}
		})
	}
}

// TestTheCoverOpensWhereTheReaderLeftOff holds which lesson the one verb leads
// to, and which word it wears, against every answer a kept place can give a
// course: one inside it, one outside it, and none at all.
func TestTheCoverOpensWhereTheReaderLeftOff(t *testing.T) {
	t.Parallel()

	const first = "/notes/Writing/lessons/golang/First.md"
	for _, tt := range []struct {
		name     string
		kept     mark.Continuation
		marked   bool
		wantWord string
		wantHref string
		wantSaid string
	}{
		{
			name:     "no place kept at all",
			wantWord: "start",
			wantHref: first,
		},
		{
			name:     "a place kept in a lesson this course lists",
			kept:     mark.Continuation{RelPath: "Writing/lessons/golang/Third.md", Anchor: "a-later-look", Offset: 820},
			marked:   true,
			wantWord: "continue",
			wantHref: "/notes/Writing/lessons/golang/Third.md?at=820#a-later-look",
			wantSaid: "Third",
		},
		{
			// The course does not teach this note, so it has nothing to say
			// about where the reader is in it.
			name:     "a place kept in a note this course does not list",
			kept:     mark.Continuation{RelPath: "Elsewhere.md", Offset: 40},
			marked:   true,
			wantWord: "start",
			wantHref: first,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writeCourse(t, root, "\nAn opening.\n\n", "- [[First]]", "- [[Second]]", "- [[Third]]")
			srv := newServerWithMark(t, root, tt.kept, tt.marked)

			code, body := get(t, srv.Client(), srv.URL+coursePage)
			if code != http.StatusOK {
				t.Fatalf("GET %s status = %d, want 200", coursePage, code)
			}
			action := coverAction.FindString(body)
			if action == "" {
				t.Fatalf("the cover offers no way into the course; page = %q", body)
			}
			if !strings.Contains(action, `data-course-action="`+tt.wantWord+`"`) {
				t.Errorf("the verb says something other than %q: %s", tt.wantWord, action)
			}
			if !strings.Contains(action, `href="`+tt.wantHref+`"`) {
				t.Errorf("the verb leads somewhere other than %s: %s", tt.wantHref, action)
			}
			// The reader's own word for it travels with the token, so a page
			// that carried the right machine word and the wrong sentence is a
			// failure here rather than something only a reader would meet.
			said := wording.CourseStartReading.In(wording.ZhHant)
			if tt.wantWord == "continue" {
				said = wording.CourseContinueReading.In(wording.ZhHant)
			}
			if !strings.Contains(action, said) {
				t.Errorf("the verb reads something other than %q: %s", said, action)
			}
			switch {
			case tt.wantSaid != "" && !strings.Contains(action, `<span class="y-cover__lesson">`+tt.wantSaid+`</span>`):
				t.Errorf("the verb names no lesson, or names the wrong one: %s", action)
			case tt.wantSaid == "" && strings.Contains(action, "y-cover__lesson"):
				t.Errorf("the verb names a lesson beside a word that already points at the next one: %s", action)
			}
		})
	}
}

// TestTheCourseStartsAtALessonThatCanBeOpened keeps the verb off a row that
// reached no note. Such a row is drawn — the course lists it and the page shows
// the course as written — but it is not somewhere a reader can be sent, so the
// course starts at the first row that is.
func TestTheCourseStartsAtALessonThatCanBeOpened(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeCourse(t, root, "\nAn opening.\n\n", "- [[Ghost Lesson]]", "- [[Second]]", "- [[Third]]")
	srv := newServer(t, root)

	code, body := get(t, srv.Client(), srv.URL+coursePage)
	if code != http.StatusOK {
		t.Fatalf("GET %s status = %d, want 200", coursePage, code)
	}
	// The unwritten row is on the page: without it this course would simply
	// have three lessons and the sentence below would be about nothing.
	if !strings.Contains(body, "Ghost Lesson") {
		t.Fatalf("the course no longer lists its unwritten first row; page = %q", body)
	}
	action := coverAction.FindString(body)
	if !strings.Contains(action, `href="/notes/Writing/lessons/golang/Second.md"`) {
		t.Errorf("the verb does not open the first lesson that exists: %s", action)
	}
}
