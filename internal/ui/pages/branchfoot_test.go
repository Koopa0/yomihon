package pages

import (
	"regexp"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/wording"
)

// footView is the part of a note's view the foot is read from, filled from the
// same calls the handler makes.
func footView(model *nav.Model, current string, lang wording.Lang) (Foot, NoteView) {
	rail := NewReadingRail(nav.Shell{Nav: model}, current, "")
	foot := FooterSequence(&rail, lang)
	view := NoteView{
		RelPath: current, Title: current, Prev: foot.Prev, Next: foot.Next, StepsLabel: foot.Label, StepsCourse: foot.Course, StepsUnit: foot.Unit,
		StepsPath: foot.Path, StepsBack: foot.Back, StepsOnward: foot.Onward, StepsToContents: foot.ToContents, StepsAsides: foot.Asides,
	}
	return foot, view
}

// footFor draws the foot the page draws for the note at current.
func footFor(t *testing.T, model *nav.Model, current string, lang wording.Lang) (foot Foot, html string) {
	t.Helper()
	foot, view := footView(model, current, lang)
	return foot, renderedHTML(t, sequenceSteps(view, lang))
}

// footVault is a course whose main line is C01 to C04 with a side branch of one
// lesson hanging from C02, a longer one from C03, and one from C04, the last,
// beside a folder that sorts differently from the course on purpose.
func footVault(t *testing.T) *nav.Model {
	t.Helper()
	lesson := func(title string) string {
		return "---\ntitle: " + title + "\ntype: lesson\nstatus: draft\n---\nbody\n"
	}
	return modelOf(t, writeVaultFiles(t, map[string]string{
		"Maps/Course.md": "---\ntitle: Course\ntype: study-path\n---\n\n" +
			"## 主線 {sequence=primary}\n\n" +
			"- [[C01]]\n" +
			"- [[C02]]\n" +
			"\t- 單課支線 {sequence=local}\n" +
			"\t\t- [[B1]]\n" +
			"- [[C03]]\n" +
			"\t- 長支線 {sequence=local}\n" +
			"\t\t- [[L1]]\n" +
			"\t\t- [[L2]]\n" +
			"- [[C04]]\n" +
			"\t- 尾端支線 {sequence=local}\n" +
			"\t\t- [[E1]]\n",
		"Writing/C01.md": lesson("C01"), "Writing/C02.md": lesson("C02"), "Writing/C03.md": lesson("C03"), "Writing/C04.md": lesson("C04"),
		"Writing/B1.md": lesson("B1"), "Writing/L1.md": lesson("L1"), "Writing/L2.md": lesson("L2"), "Writing/E1.md": lesson("E1"),
		// Not in the course, in the same folder, between its lessons by name.
		"Writing/C02a.md": "---\ntitle: C02a\ntype: note\n---\nbody\n",
		// A course of one lesson with files either side of it in its folder, and
		// its own note beside another map.
		"Maps/Solo.md":  "---\ntitle: Solo\ntype: study-path\n---\n\n## 主線 {sequence=primary}\n\n- [[Only]]\n",
		"Maps/Other.md": "---\ntitle: Other\ntype: moc\n---\nbody\n",
		"Solo/A.md":     "---\ntitle: A\ntype: note\n---\nbody\n",
		"Solo/Only.md":  lesson("Only"),
		"Solo/Z.md":     "---\ntitle: Z\ntype: note\n---\nbody\n",
		// A course of one lesson that has a side branch: no step of its own, and
		// still something to point at.
		"Maps/Lone.md":        "---\ntitle: Lone\ntype: study-path\n---\n\n## 主線 {sequence=primary}\n\n- [[Lone lesson]]\n\t- 支線 {sequence=local}\n\t\t- [[Lone branch]]\n",
		"Lone/Lone lesson.md": lesson("Lone lesson"),
		"Lone/Lone branch.md": lesson("Lone branch"),
	}))
}

// TestAWayOffASideBranchIsLabelledAndIsNeverAStep draws the foot of each end of
// each kind of side branch, in both languages, and holds what the reader is
// shown: where the branch's first lesson goes back to, where its last goes on
// to, the contents where there is no main-line lesson to go on to, and the aside
// beside the lesson a branch hangs from. None of them takes a step's place or
// a step's relation, because none of them is a step in the course.
func TestAWayOffASideBranchIsLabelledAndIsNeverAStep(t *testing.T) {
	t.Parallel()

	model := footVault(t)
	const (
		c02 = `<a class="y-steps__link y-steps__link--prev" href="/notes/Writing/C01.md" rel="prev">`
	)
	tests := []struct {
		name    string
		current string
		lang    wording.Lang
		want    []string
		absent  []string
	}{
		{
			name:    "a branch of one lesson goes back and on, in Chinese",
			current: "Writing/B1.md",
			lang:    wording.ZhHant,
			want: []string{
				`<a class="y-steps__link y-steps__link--prev y-steps__link--back" href="/notes/Writing/C02.md"><span class="y-steps__role"><span class="y-steps__dir" aria-hidden="true">←</span> 回到</span> <span class="y-steps__name">C02</span></a>`,
				`<a class="y-steps__link y-steps__link--next y-steps__link--onward" href="/notes/Writing/C03.md"><span class="y-steps__role">回到主線：</span> <span class="y-steps__name">C03 <span class="y-steps__dir" aria-hidden="true">→</span></span></a>`,
			},
			absent: []string{`rel="prev"`, `rel="next"`, "y-steps__aside", "上一課", "下一課", "上一份"},
		},
		{
			name:    "a branch of one lesson goes back and on, in English",
			current: "Writing/B1.md",
			lang:    wording.En,
			want: []string{
				`<span class="y-steps__dir" aria-hidden="true">←</span> Back to</span> <span class="y-steps__name">C02</span>`,
				`<span class="y-steps__role">Back to the main line: </span> <span class="y-steps__name">C03 <span class="y-steps__dir" aria-hidden="true">→</span></span>`,
			},
			absent: []string{`rel="prev"`, `rel="next"`},
		},
		{
			name:    "the first lesson of a longer branch goes back and still steps on within it",
			current: "Writing/L1.md",
			lang:    wording.ZhHant,
			want: []string{
				`y-steps__link--back" href="/notes/Writing/C03.md">`,
				`href="/notes/Writing/L2.md" rel="next"`,
			},
			absent: []string{"y-steps__link--onward", `rel="prev"`},
		},
		{
			name:    "the last lesson of a longer branch steps back within it and goes on",
			current: "Writing/L2.md",
			lang:    wording.ZhHant,
			want: []string{
				`href="/notes/Writing/L1.md" rel="prev"`,
				`y-steps__link--onward" href="/notes/Writing/C04.md">`,
			},
			absent: []string{"y-steps__link--back", `rel="next"`},
		},
		{
			name:    "a branch hung from the last lesson goes on to the contents",
			current: "Writing/E1.md",
			lang:    wording.ZhHant,
			want: []string{
				`<a class="y-steps__link y-steps__link--next y-steps__link--onward" href="/courses/Maps/Course.md?from=Writing%2FE1.md"><span class="y-steps__role">回到目錄</span></a>`,
				`y-steps__link--back" href="/notes/Writing/C04.md">`,
			},
			absent: []string{`rel="next"`},
		},
		{
			name:    "the contents in English",
			current: "Writing/E1.md",
			lang:    wording.En,
			want:    []string{`<span class="y-steps__role">Back to the contents</span>`},
		},
		{
			name:    "the lesson a branch hangs from keeps its steps and points at the branch",
			current: "Writing/C02.md",
			lang:    wording.ZhHant,
			want: []string{
				c02,
				`href="/notes/Writing/C03.md" rel="next"`,
				`<p class="y-steps__aside"><span class="y-steps__asidelabel">支線：</span><a class="y-steps__asidelink" href="/notes/Writing/B1.md">B1</a></p>`,
			},
			absent: []string{"y-steps__link--back", "y-steps__link--onward"},
		},
		{
			name:    "the aside in English",
			current: "Writing/C02.md",
			lang:    wording.En,
			want:    []string{`<span class="y-steps__asidelabel">Branch: </span><a class="y-steps__asidelink" href="/notes/Writing/B1.md">B1</a>`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, html := footFor(t, model, tt.current, tt.lang)
			for _, want := range tt.want {
				if !strings.Contains(html, want) {
					t.Errorf("the foot of %s does not carry %q; html = %q", tt.current, want, html)
				}
			}
			for _, absent := range tt.absent {
				if strings.Contains(html, absent) {
					t.Errorf("the foot of %s carries %q; html = %q", tt.current, absent, html)
				}
			}
		})
	}
}

// TestALessonWithNothingButABranchToPointAtStillDrawsAFoot keeps the landmark
// from being withheld for want of a step when the lesson has an aside to offer.
func TestALessonWithNothingButABranchToPointAtStillDrawsAFoot(t *testing.T) {
	t.Parallel()

	foot, html := footFor(t, footVault(t), "Lone/Lone lesson.md", wording.ZhHant)
	if foot.Prev.RelPath != "" || foot.Next.RelPath != "" || len(foot.Asides) != 1 {
		t.Fatalf("the fixture's lesson has foot %+v, want no step and one aside, so the check below proves nothing", foot)
	}
	if !strings.Contains(html, `<p class="y-steps__aside">`) {
		t.Errorf("a lesson whose only offer is a branch draws no foot; html = %q", html)
	}
}

// TestTheFootNamesTheCourseWithALinkToItsContents holds the printed label: the
// path's title alone, as a link to its contents with the lesson marked, while
// the landmark keeps the name of the step onward, so a reader who listens is not
// told one name twice and the sidebar's order landmark is not the foot's.
func TestTheFootNamesTheCourseWithALinkToItsContents(t *testing.T) {
	t.Parallel()

	model := footVault(t)
	foot, html := footFor(t, model, "Writing/C03.md", wording.ZhHant)
	if want := `<p class="y-steps__source"><a class="y-steps__path" href="/courses/Maps/Course.md?from=Writing%2FC03.md">Course</a></p>`; !strings.Contains(html, want) {
		t.Errorf("the foot does not name the course with a link to its contents; html = %q", html)
	}
	if want := `aria-label="` + foot.Label + `"`; !strings.Contains(html, want) || foot.Label != "Course 從此步往下" {
		t.Errorf("the foot's landmark is not named for the step onward (%q); html = %q", foot.Label, html)
	}
	if visible := regexp.MustCompile(`<p class="y-steps__source">.*?</p>`).FindString(html); strings.Contains(visible, "從此步往下") {
		t.Errorf("the printed label still says the step onward: %q", visible)
	}
	if strings.Contains(html, `aria-label="Course 的順序"`) {
		t.Errorf("the foot's landmark has the sidebar's name, so a screen reader hears one name twice")
	}
}

// TestALessonTheCourseTeachesNeverFallsBackToTheFolder holds the other half of
// the foot's choice. A course's declared order can contradict the folder's
// completely, so a lesson of it with no step to offer shows no step at all and
// never the file beside it. What is not a lesson of a course keeps the folder:
// a note in no course, the course's own note, and a note two courses teach
// where nothing picks one — each of them has no order the course declared for
// it.
func TestALessonTheCourseTeachesNeverFallsBackToTheFolder(t *testing.T) {
	t.Parallel()

	model := footVault(t)

	only, html := footFor(t, model, "Solo/Only.md", wording.ZhHant)
	if !only.Course || only.Prev.RelPath != "" || only.Next.RelPath != "" {
		t.Errorf("the only lesson of a course has foot %+v, want the course's own, with no step", only)
	}
	if strings.Contains(html, "y-steps") || strings.Contains(html, "同資料夾") {
		t.Errorf("the only lesson of a course draws a foot, or the folder's: %q", html)
	}
	if prev, next := model.FolderStep("Solo/Only.md"); prev.RelPath == "" || next.RelPath == "" {
		t.Fatalf("the folder offers (%q, %q) beside the lesson, so the check above proves nothing about falling back", prev.RelPath, next.RelPath)
	}

	for _, keeps := range []struct {
		name, current string
	}{
		{"a note in no course", "Writing/C02a.md"},
		{"the course's own note", "Maps/Solo.md"},
	} {
		foot, _ := footFor(t, model, keeps.current, wording.ZhHant)
		if foot.Course || foot.Label != "同資料夾的前後檔案" {
			t.Errorf("%s has foot %+v, want the folder's", keeps.name, foot)
		}
		if foot.Prev.RelPath == "" && foot.Next.RelPath == "" {
			t.Errorf("%s has no folder neighbour at all, so keeping the folder is not what the foot did", keeps.name)
		}
	}
}

// TestNothingStandsAboveTheTitle holds the head of a note to its title. The
// sidebar already says which course the note is in and which part, or which
// folder, so a line over the heading said it twice: the running head of a
// course's lesson and the folder's breadcrumb of any other note are both gone,
// and the first thing in the head is the heading itself.
func TestNothingStandsAboveTheTitle(t *testing.T) {
	t.Parallel()

	model := footVault(t)
	for _, current := range []string{"Writing/C03.md", "Writing/L1.md", "Writing/C02a.md", "Maps/Solo.md"} {
		for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
			_, view := footView(model, current, lang)
			view.Title = "T"
			html := renderedHTML(t, articleHead(view, lang))
			if !strings.HasPrefix(html, `<h1 class="y-title">T</h1>`) {
				t.Errorf("the head of %s in %s does not open on the title; html = %q", current, lang, html)
			}
			if strings.Contains(html, "y-crumbs") {
				t.Errorf("the head of %s in %s still draws a path or a running head; html = %q", current, lang, html)
			}
		}
	}
}

// TestTheFootIsTheCoursesOnlyWhereTheRailResolvedABook holds the foot to the
// rail's own resolution of a book: a note two courses teach resolves none, and
// keeps the folder's foot.
func TestTheFootIsTheCoursesOnlyWhereTheRailResolvedABook(t *testing.T) {
	t.Parallel()

	model := buildStepsModel(t)
	rail := NewReadingRail(nav.Shell{Nav: model}, "Course/C01.md", "")
	if rail.Kind == ReadingRailBook {
		t.Fatalf("the rail resolved a book for a note two courses teach, so this proves nothing")
	}
	if foot := FooterSequence(&rail, wording.ZhHant); foot.Course {
		t.Errorf("the foot is the course's where no book was resolved: %+v", foot)
	}
}
