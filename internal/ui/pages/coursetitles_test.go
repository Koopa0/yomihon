package pages

import (
	"bytes"
	"regexp"
	"testing"

	"github.com/a-h/templ"
	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/nav"
)

// The fixture course lists five rows in this order: one that wrote an alias
// over a note that has a title, one whose title differs from its file name, one
// whose note has no title and is linked by a folder-qualified name, one that
// resolves to no note, and a last titled lesson.
const (
	titlesCoursePath = "Maps/Course.md"
	titlesAlias      = "Slices, in my words"
	titlesIota       = "iota: The Compile-Time Constant Generator"
	titlesNoTitle    = "Lessons/no-title"
	titlesMissing    = "Lessons/not-written-yet"
	titlesLast       = "Last: The Final Lesson"
)

func renderedHTML(t *testing.T, c templ.Component) string {
	t.Helper()
	var buf bytes.Buffer
	if err := c.Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}

func submatches(re *regexp.Regexp, html string) []string {
	var out []string
	for _, m := range re.FindAllStringSubmatch(html, -1) {
		out = append(out, m[1])
	}
	return out
}

// TestCourseRowsNameTheLessonAsTheAuthorAndNoteDeclared locks the name a course
// prints for a row on every surface that names a lesson: an alias the row wrote
// wins, then the resolved note's own title, then the link text; a row that
// resolved to nothing keeps its link text. The syllabus page, the book rail, the
// step beside it and the article foot all read that one answer.
func TestCourseRowsNameTheLessonAsTheAuthorAndNoteDeclared(t *testing.T) {
	t.Parallel()
	model := modelOf(t, "testdata/coursetitles")
	chrome := recordedChrome()
	rows := []string{titlesAlias, titlesIota, titlesNoTitle, titlesMissing, titlesLast}

	t.Run("syllabus page", func(t *testing.T) {
		t.Parallel()
		view := BuildPathView(model.Path(titlesCoursePath), model.Paths(), &CourseCover{})
		html := renderedHTML(t, Syllabus(view, chrome))
		got := submatches(regexp.MustCompile(`<span class="y-lesson__title"[^>]*>([^<]*)</span>`), html)
		if diff := cmp.Diff(rows, got); diff != "" {
			t.Errorf("syllabus rows (-want +got):\n%s", diff)
		}
	})

	t.Run("syllabus cover continues with the lesson by name", func(t *testing.T) {
		t.Parallel()
		const kept = "Lessons/iota-constants.md"
		cover := CourseCover{KeptNote: kept, KeptHref: ResumeHref(kept, "top", 0)}
		view := BuildPathView(model.Path(titlesCoursePath), model.Paths(), &cover)
		if diff := cmp.Diff(titlesIota, view.Action.Lesson); diff != "" {
			t.Errorf("continue-with lesson (-want +got):\n%s", diff)
		}
	})

	// The rail is drawn from the lesson with no title, so its previous and next
	// steps exist and the unwritten row lies past the current one.
	t.Run("book rail", func(t *testing.T) {
		t.Parallel()
		rail := NewReadingRail(nav.Shell{Nav: model}, "Lessons/no-title.md", "")
		html := renderedHTML(t, readingRail(rail, chrome))
		got := submatches(regexp.MustCompile(`<span class="y-nav(?:dot|mark y-navmark--warn)" aria-hidden="true">(?:!)?</span>\s*<span[^>]*>([^<]*)</span>`), html)
		if diff := cmp.Diff(rows, got); diff != "" {
			t.Errorf("book rail rows (-want +got):\n%s", diff)
		}
		stepsNav := regexp.MustCompile(`(?s)<nav class="y-lessonsteps".*?</nav>`).FindString(html)
		steps := submatches(regexp.MustCompile(`<span>([^<]*)</span>`), stepsNav)
		if diff := cmp.Diff([]string{titlesIota, titlesLast}, steps); diff != "" {
			t.Errorf("book rail step names (-want +got):\n%s", diff)
		}
	})

	t.Run("sidebar", func(t *testing.T) {
		t.Parallel()
		sb := NewSidebar(nav.Shell{Nav: model}, "Lessons/no-title.md")
		html := renderedHTML(t, sidebar(sb, chrome))
		got := submatches(regexp.MustCompile(`<span class="y-nav(?:dot|mark y-navmark--warn)" aria-hidden="true">(?:!)?</span>\s*<span[^>]*>([^<]*)</span>`), html)
		if diff := cmp.Diff(rows, got); diff != "" {
			t.Errorf("sidebar course rows (-want +got):\n%s", diff)
		}
	})

	t.Run("article foot", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			name       string
			current    string
			prev, next string
		}{
			{"an alias names the previous step", "Lessons/iota-constants.md", titlesAlias, titlesNoTitle},
			{"a title names both steps and the unwritten row is skipped", "Lessons/no-title.md", titlesIota, titlesLast},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				rail := NewReadingRail(nav.Shell{Nav: model}, tt.current, "")
				prev, next, label, course := FooterSequence(&rail, chrome.Lang)
				if !course {
					t.Fatalf("FooterSequence(%s) answered folder adjacency, want the course", tt.current)
				}
				html := renderedHTML(t, sequenceSteps(NoteView{Prev: prev, Next: next, StepsLabel: label, StepsCourse: course}, chrome.Lang))
				got := submatches(regexp.MustCompile(`<span class="y-steps__name"[^>]*>([^<]*)</span>`), html)
				if diff := cmp.Diff([]string{tt.prev, tt.next}, got); diff != "" {
					t.Errorf("foot step names (-want +got):\n%s", diff)
				}
			})
		}
	})
}
