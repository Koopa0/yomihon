package pages

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/wording"
)

// coverMain is the page's own column, so an assertion about what the cover
// says is not satisfied or broken by a word in the shared chrome around it.
func coverMain(t *testing.T, html string) string {
	t.Helper()
	main := regexp.MustCompile(`(?s)<main\b.*?</main>`).FindString(html)
	if main == "" {
		t.Fatalf("the page carries no main column; html = %q", html)
	}
	return main
}

// wordsOf is what a reader of the page sees of some markup: its tags and the
// names inside them, which a class or a label may spell with the very words a
// noun check is looking for, are not the page's words.
func wordsOf(html string) string {
	return strings.Join(strings.Fields(regexp.MustCompile(`<[^>]*>`).ReplaceAllString(html, " ")), " ")
}

// renderCover draws one path's cover in one language.
func renderCover(t *testing.T, view *PathView, lang wording.Lang) string {
	t.Helper()
	return coverMain(t, renderedHTML(t, Syllabus(*view, layouts.Chrome{Lang: lang})))
}

// queueNotes are notes of a type that is not the lesson type: a reader's own
// concepts, which a path can arrange without ever being a course.
func queueNotes() map[string]string {
	return map[string]string{
		"Writing/Queue.md":  "---\ntitle: Queue\ntype: concept\n---\nbody\n",
		"Writing/Worker.md": "---\ntitle: Worker\ntype: concept\n---\nbody\n",
		"Writing/Gate.md":   "---\ntitle: Gate\ntype: concept\n---\nbody\n",
	}
}

// TestTheCoverStatesTheCourseOnceInTheNounItIsReadIn holds the head's one line
// about size. A path of lessons says lessons, a path of anything else says
// items, the side branches are counted beside the total and never in it, and
// neither a count of parts nor a count of modules is printed — nor a figure
// beside each part, which would say the same number a second and third time on
// one screen.
func TestTheCoverStatesTheCourseOnceInTheNounItIsReadIn(t *testing.T) {
	t.Parallel()

	lessons := buildTestPath(t, "## Data {sequence=primary}\n\n"+
		"- [[Slices]]\n"+
		"- [[Arrays]]\n"+
		"\t- 選修 {sequence=local}\n"+
		"\t\t- [[Tuning]]\n"+
		"- [[GC]]\n"+
		"\n## Memory {sequence=primary}\n\n- [[Slices]]\n")
	items := buildTestPath(t, "## Data {sequence=primary}\n\n"+
		"- [[Queue]]\n"+
		"- [[Worker]]\n"+
		"\t- 選修 {sequence=local}\n"+
		"\t\t- [[Gate]]\n"+
		"- [[Queue]]\n"+
		"\n## Memory {sequence=primary}\n\n- [[Worker]]\n", queueNotes())

	if lessons.Unit != nav.UnitLesson || items.Unit != nav.UnitItem {
		t.Fatalf("units = %v and %v, want the lesson path in lessons and the queue path in items; the page below proves nothing about nouns", lessons.Unit, items.Unit)
	}

	tests := []struct {
		name      string
		path      nav.Path
		lang      wording.Lang
		extent    []string
		forbidden []string
	}{
		{"lessons in Chinese", lessons, wording.ZhHant, []string{"4 課", "支線 1 課"}, []string{"4 篇", "支線 1 篇", "模組", "2 部"}},
		{"items in Chinese", items, wording.ZhHant, []string{"4 篇", "支線 1 篇"}, []string{"課", "模組", "2 部"}},
		{"lessons in English", lessons, wording.En, []string{"4 lessons", "1 in a branch"}, []string{"items", "module", "2 parts"}},
		{"items in English", items, wording.En, []string{"4 items", "1 in a branch"}, []string{"lesson", "module", "2 parts"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			view := BuildPathView(&tt.path, []nav.Path{tt.path}, &CourseCover{})
			if diff := cmp.Diff(tt.extent, view.Extent(tt.lang)); diff != "" {
				t.Errorf("Extent() (-want +got):\n%s", diff)
			}
			html := renderCover(t, &view, tt.lang)
			row := regexp.MustCompile(`(?s)<div class="y-metarow">.*?</div>`).FindString(html)
			joined := strings.Join(tt.extent, " · ")
			if !strings.Contains(wordsOf(row), joined) {
				t.Errorf("the head does not read %q; it reads %q", joined, row)
			}
			if strings.Contains(html, "partcount") || strings.Contains(html, "pathcount") {
				t.Errorf("a part states a size of its own beside the head's; html = %q", html)
			}
			for _, forbidden := range tt.forbidden {
				if strings.Contains(wordsOf(html), forbidden) {
					t.Errorf("the cover says %q; its words are %q", forbidden, wordsOf(html))
				}
			}
		})
	}
}

// TestAPathWithNoSideBranchStatesNoBranchFigure keeps the second figure out of
// a head that has nothing to say with it.
func TestAPathWithNoSideBranchStatesNoBranchFigure(t *testing.T) {
	t.Parallel()

	path := buildTestPath(t, "## Data {sequence=primary}\n\n- [[Slices]]\n- [[Arrays]]\n")
	view := BuildPathView(&path, []nav.Path{path}, &CourseCover{})
	if diff := cmp.Diff([]string{"2 課"}, view.Extent(wording.ZhHant)); diff != "" {
		t.Errorf("Extent() (-want +got):\n%s", diff)
	}
}

// TestPartNumeralsAreTheLanguagesOwn pins what a part's heading label says. The
// numerals are the language's: hanzi, with 十 standing alone for ten and no
// leading 一 before it, and roman in English. A number the table does not write
// falls back to its digits rather than to nothing.
func TestPartNumeralsAreTheLanguagesOwn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		n      int
		zh, en string
	}{
		{1, "第一部", "Part I"},
		{2, "第二部", "Part II"},
		{9, "第九部", "Part IX"},
		{10, "第十部", "Part X"},
		{11, "第十一部", "Part XI"},
		{19, "第十九部", "Part XIX"},
		{20, "第二十部", "Part XX"},
		{21, "第二十一部", "Part XXI"},
		{99, "第九十九部", "Part XCIX"},
		{100, "第100部", "Part C"},
		{0, "第0部", "Part 0"},
	}
	for _, tt := range tests {
		v := PathBranchView{Num: tt.n}
		if got := v.PartLabel(wording.ZhHant); got != tt.zh {
			t.Errorf("PartLabel(zh, %d) = %q, want %q", tt.n, got, tt.zh)
		}
		if got := v.PartLabel(wording.En); got != tt.en {
			t.Errorf("PartLabel(en, %d) = %q, want %q", tt.n, got, tt.en)
		}
	}
}

// TestACoverRunsTheAuthorsGlossInAfterTheTitle holds the row the author wrote:
// the title and the sentence beside it are one line of text in one link, the
// sentence in the language of the path's own note and not the lesson's, and a
// row that wrote none carries no empty element.
func TestACoverRunsTheAuthorsGlossInAfterTheTitle(t *testing.T) {
	t.Parallel()

	path := buildTestPath(t, "## Data {sequence=primary}\n\n"+
		"- [[Slices]] — 啟動工作，等它完成\n"+
		"- [[Arrays]]\n"+
		"- [[Unwritten]] — 還沒寫\n")
	view := BuildPathView(&path, []nav.Path{path}, &CourseCover{OpeningLanguage: "zh-Hant"})
	html := renderCover(t, &view, wording.ZhHant)

	run := regexp.MustCompile(`(?s)<a class="y-lesson" href="/notes/Writing/Slices.md">(.*?)</a>`).FindStringSubmatch(html)
	if run == nil {
		t.Fatalf("the cover draws no link to Slices; html = %q", html)
	}
	for _, want := range []string{
		`<span class="y-lesson__text"><span class="y-lesson__title">Slices</span> <span class="y-gloss" lang="zh-Hant">— 啟動工作，等它完成</span></span>`,
	} {
		if !strings.Contains(run[1], want) {
			t.Errorf("the row does not run the gloss in after the title as one line; want %q in %q", want, run[1])
		}
	}
	arrays := regexp.MustCompile(`(?s)<a class="y-lesson" href="/notes/Writing/Arrays.md">(.*?)</a>`).FindStringSubmatch(html)
	if len(arrays) == 0 || strings.Contains(arrays[1], "y-gloss") {
		t.Errorf("a row that wrote nothing after its link still draws a gloss; row = %v", arrays)
	}
	// The author's words about a lesson that is not written yet are still
	// theirs, and still printed with the warning that it cannot be opened.
	if !strings.Contains(html, `<span class="y-gloss" lang="zh-Hant">— 還沒寫</span>`) {
		t.Errorf("the gloss of a row that reached no note is dropped; html = %q", html)
	}

	// A path whose note declared no language stamps none on the sentence, so
	// it inherits the page's the way the rest of the cover does.
	plain := BuildPathView(&path, []nav.Path{path}, &CourseCover{})
	if strings.Contains(renderCover(t, &plain, wording.ZhHant), `y-gloss" lang=`) {
		t.Errorf("a path with no declared language stamped one on the author's sentence")
	}
}

// TestTheAppendixIsListedBeneathTheContentsAndOutsideTheCourse holds the
// section the author declared out of the sequence: shown after the last part
// under their own heading, as an unnumbered list with the sentences they wrote,
// and nothing more — no count, no number, no line through its rows, and no
// place on the walk, so the verb, the counts and the steps are what they were
// without it.
func TestTheAppendixIsListedBeneathTheContentsAndOutsideTheCourse(t *testing.T) {
	t.Parallel()

	with := buildTestPath(t, "## Data {sequence=primary}\n\n- [[Slices]]\n- [[Arrays]]\n"+
		"\n## 查閱與對照 {sequence=none}\n\n"+
		"- [[Tuning]] — 找出等待的來源\n"+
		"- [[Nobody wrote this]] — 還沒寫\n"+
		"- [[Template]]\n"+
		"\nProse that links nothing.\n"+
		"\n## 只有文字 {sequence=none}\n\nJust words.\n")
	without := buildTestPath(t, "## Data {sequence=primary}\n\n- [[Slices]]\n- [[Arrays]]\n")

	view := BuildPathView(&with, []nav.Path{with}, &CourseCover{OpeningLanguage: "zh-Hant"})
	bare := BuildPathView(&without, []nav.Path{without}, &CourseCover{})

	if len(view.Appendix) != 1 || view.Appendix[0].Heading != "查閱與對照" {
		t.Fatalf("Appendix = %+v, want the one section that lists rows, under the author's own heading; a section of prose alone has nothing to list", view.Appendix)
	}
	if len(view.Branches) != 1 {
		t.Errorf("the course draws %d parts, want 1: the appendix is not a part", len(view.Branches))
	}
	if diff := cmp.Diff(bare.Extent(wording.ZhHant), view.Extent(wording.ZhHant)); diff != "" {
		t.Errorf("the appendix moved the course's count (-without +with):\n%s", diff)
	}
	if view.Action != bare.Action {
		t.Errorf("the appendix moved the verb: %+v, was %+v", view.Action, bare.Action)
	}

	html := renderCover(t, &view, wording.ZhHant)
	for _, want := range []string{
		`<h2 class="y-part"><span class="y-part__name">查閱與對照</span></h2>`,
		`<ul class="y-reflist">`,
		`<a class="y-ref" href="/notes/Writing/Tuning.md"><span class="y-ref__title">Tuning</span> <span class="y-gloss" lang="zh-Hant">— 找出等待的來源</span></a>`,
		// A row that reached nothing keeps its place, its words and the reason.
		`data-resolution="unresolved"`,
		`<span class="y-gloss" lang="zh-Hant">— 還沒寫</span>`,
		`data-resolution="non-instance"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("the appendix does not draw %q; html = %q", want, html)
		}
	}
	at := strings.Index(html, `<section class="y-part-block y-appendix">`)
	if last := strings.LastIndex(html, `id="part-`); at < 0 || at < last {
		t.Errorf("the appendix is not drawn after the last part (appendix at %d, last part heading at %d)", at, last)
	}
	appendix := html[at:]
	if end := strings.Index(appendix, "</section>"); end >= 0 {
		appendix = appendix[:end]
	}
	for _, forbidden := range []string{"<ol", "value=", "y-lesson", "y-navdot", "rel=\"next\"", "rel=\"prev\"", "partcount"} {
		if strings.Contains(appendix, forbidden) {
			t.Errorf("the appendix carries %q, which belongs to the course's walk; appendix = %q", forbidden, appendix)
		}
	}
	if strings.Contains(html, "只有文字") || strings.Contains(html, "Just words") {
		t.Errorf("a section declared out of the course that lists no row is drawn; html = %q", html)
	}
}

// TestTheWholeNoteIsTheLastLineOfTheCover holds where the way to the note
// moved: out of the row of actions and to the end of the contents, after the
// last part and after the appendix, as one quiet line.
func TestTheWholeNoteIsTheLastLineOfTheCover(t *testing.T) {
	t.Parallel()

	path := buildTestPath(t, "## Data {sequence=primary}\n\n- [[Slices]]\n- [[Arrays]]\n"+
		"\n## 查閱 {sequence=none}\n\n- [[Tuning]]\n")
	for _, tt := range []struct {
		lang wording.Lang
		want string
	}{
		{wording.ZhHant, `<a class="y-syl-read" href="/notes/Maps/Go%20path.md">整篇筆記 <span aria-hidden="true">→</span></a>`},
		{wording.En, `<a class="y-syl-read" href="/notes/Maps/Go%20path.md">The whole note <span aria-hidden="true">→</span></a>`},
	} {
		view := BuildPathView(&path, []nav.Path{path}, &CourseCover{Listenable: true})
		html := renderCover(t, &view, tt.lang)
		at := strings.Index(html, tt.want)
		if at < 0 {
			t.Fatalf("the cover in %s carries no line to the whole note; html = %q", tt.lang, html)
		}
		if strings.Count(html, "/notes/Maps/Go%20path.md") != 1 {
			t.Errorf("the way to the note is offered more than once in %s", tt.lang)
		}
		ways := regexp.MustCompile(`(?s)<div class="y-cover__ways">.*?</div>`).FindString(html)
		if strings.Contains(ways, "/notes/Maps/Go%20path.md") {
			t.Errorf("the way to the note is still among the actions in %s; actions = %q", tt.lang, ways)
		}
		if after := html[at:]; strings.Contains(after, "y-part-block") || strings.Contains(after, "y-ref") {
			t.Errorf("something follows the line to the whole note in %s; it is not the last line", tt.lang)
		}
		if before := html[:at]; !strings.Contains(before, "y-appendix") {
			t.Errorf("the line to the whole note comes before the appendix in %s", tt.lang)
		}
	}
}

// TestTheListenDoorIsOfferedOnlyWhereSomethingIsMarked holds the offer to the
// condition that makes it true: the page it leads to has something to play.
func TestTheListenDoorIsOfferedOnlyWhereSomethingIsMarked(t *testing.T) {
	t.Parallel()

	path := buildTestPath(t, "## Data {sequence=primary}\n\n- [[Slices]]\n")
	for _, tt := range []struct {
		name       string
		listenable bool
		wantDoor   bool
	}{
		{"a path with something marked", true, true},
		{"a path with nothing marked", false, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			view := BuildPathView(&path, []nav.Path{path}, &CourseCover{Listenable: tt.listenable})
			html := renderCover(t, &view, wording.ZhHant)
			if got := strings.Contains(html, `href="/listen/Maps/Go%20path.md"`); got != tt.wantDoor {
				t.Errorf("the cover offers the listening page = %t, want %t; html = %q", got, tt.wantDoor, html)
			}
			if tt.wantDoor && !strings.Contains(html, `>聆聽 <span aria-hidden="true">→</span></a>`) {
				t.Errorf("the door is not worded 聆聽 →; html = %q", html)
			}
			if got := strings.Contains(html, "聆聽這門課"); got {
				t.Errorf("the door still calls the path a course; html = %q", html)
			}
		})
	}
}

// TestASideBranchHangsFromTheRowItWasNestedUnderAndNotTheFirstOneNamedAlike
// holds how the branch finds its row: by where the row is written, since two
// rows can name one note. A branch whose row is not a drawn row of the part,
// because it hangs from a heading and not a row, stays where the author put it.
func TestASideBranchHangsFromTheRowItWasNestedUnderAndNotTheFirstOneNamedAlike(t *testing.T) {
	t.Parallel()

	twice := buildTestPath(t, "## Main {sequence=primary}\n\n"+
		"- [[Slices]]\n"+
		"- [[Slices]]\n"+
		"\t- 選修 {sequence=local}\n"+
		"\t\t- [[Tuning]]\n")
	view := BuildPathView(&twice, []nav.Path{twice}, &CourseCover{})
	rows := view.Branches[0].Items
	if len(rows) != 2 || rows[0].Entry == nil || rows[1].Entry == nil {
		t.Fatalf("the part draws %+v, want two rows and no loose branch", rows)
	}
	if len(rows[0].Entry.Branches) != 0 {
		t.Errorf("the first row named Slices carries the branch, but it hangs from the second")
	}
	if len(rows[1].Entry.Branches) != 1 || rows[1].Entry.Branches[0].Heading != "選修" {
		t.Errorf("the second row named Slices carries %+v, want the branch it was nested under", rows[1].Entry.Branches)
	}

	heading := buildTestPath(t, "## Main {sequence=primary}\n\n- [[Slices]]\n\n### 選修 {sequence=local}\n\n- [[Tuning]]\n")
	loose := BuildPathView(&heading, []nav.Path{heading}, &CourseCover{})
	items := loose.Branches[0].Items
	if len(items) != 2 || items[0].Entry == nil || items[1].Branch == nil || !items[1].Branch.Local {
		t.Fatalf("a branch opened by a heading draws %+v, want the row and then the branch as an item of its own", items)
	}
	if len(items[0].Entry.Branches) != 0 {
		t.Errorf("a branch opened by a heading was handed to the row above it")
	}
}

// TestACourseWhoseKeptPlaceIsInsideASideBranchStillFindsIt keeps the kept
// place reachable where the lesson sits on a side branch, now that the branch
// is carried by its row and no longer an item of the part.
func TestACourseWhoseKeptPlaceIsInsideASideBranchStillFindsIt(t *testing.T) {
	t.Parallel()

	path := buildTestPath(t, "## Main {sequence=primary}\n\n- [[Slices]]\n\t- 選修 {sequence=local}\n\t\t- [[Tuning]]\n- [[Arrays]]\n")
	cover := CourseCover{KeptNote: "Writing/Tuning.md", KeptHref: "/notes/Writing/Tuning.md#a-1"}
	view := BuildPathView(&path, []nav.Path{path}, &cover)
	if !view.Action.Continuing || view.Action.Lesson != "Tuning" {
		t.Errorf("Action = %+v, want the verb to go back to Tuning on the side branch", view.Action)
	}
}

// writeVaultFiles lays a small vault on disk and returns its folder.
func writeVaultFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	return root
}

// TestAPathOfNotesThatAreNotLessonsNamesNoLessonAnywhere is the end-to-end
// check that the noun is one decision read everywhere a path counts or steps:
// the cover, the book's head in the reading rail, the path index, the foot of
// an article and the sidebar's steps. The fixture is a kit of concept notes
// arranged in order — nobody's course — beside a path of real lessons, so a
// surface that kept its own noun is wrong for exactly one of them.
func TestAPathOfNotesThatAreNotLessonsNamesNoLessonAnywhere(t *testing.T) {
	t.Parallel()

	root := writeVaultFiles(t, map[string]string{
		"Maps/Queue kit.md": "---\ntitle: Queue kit\ntype: study-path\n---\n\n" +
			"## Parts {sequence=primary}\n\n- [[Queue]]\n- [[Worker]]\n- [[Gate]]\n",
		"Maps/Lessons.md": "---\ntitle: Lessons\ntype: study-path\n---\n\n" +
			"## Parts {sequence=primary}\n\n- [[L01]]\n- [[L02]]\n- [[L03]]\n",
		"Concepts/Queue.md":  "---\ntitle: Queue\ntype: concept\n---\nbody\n",
		"Concepts/Worker.md": "---\ntitle: Worker\ntype: concept\n---\nbody\n",
		"Concepts/Gate.md":   "---\ntitle: Gate\ntype: concept\n---\nbody\n",
		"Writing/L01.md":     "---\ntitle: L01\ntype: lesson\nstatus: draft\n---\nbody\n",
		"Writing/L02.md":     "---\ntitle: L02\ntype: lesson\nstatus: draft\n---\nbody\n",
		"Writing/L03.md":     "---\ntitle: L03\ntype: lesson\nstatus: draft\n---\nbody\n",
	})
	model := modelOf(t, root)
	shell := nav.Shell{Nav: model}

	tests := []struct {
		name     string
		path     string
		middle   string
		lang     wording.Lang
		extent   string
		prev     string
		next     string
		railPrev string
		// otherExtent and otherStep are the other noun's words for the same
		// places, which no surface of this path may say. An empty otherStep
		// is a noun whose step words are a prefix of the other's.
		otherExtent string
		otherStep   string
	}{
		{"items in Chinese", "Maps/Queue kit.md", "Concepts/Worker.md", wording.ZhHant, "3 篇", "上一篇", "下一篇", "上一篇：", "3 課", "一課"},
		{"lessons in Chinese", "Maps/Lessons.md", "Writing/L02.md", wording.ZhHant, "3 課", "上一課", "下一課", "上一課：", "3 篇", "一篇"},
		{"items in English", "Maps/Queue kit.md", "Concepts/Worker.md", wording.En, "3 items", "Previous", "Next", "Previous: ", "3 lessons", "lesson"},
		{"lessons in English", "Maps/Lessons.md", "Writing/L02.md", wording.En, "3 lessons", "Previous lesson", "Next lesson", "Previous lesson: ", "3 items", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			chrome := layouts.Chrome{Lang: tt.lang, Nonce: "n"}

			path := model.Path(tt.path)
			if path == nil {
				t.Fatalf("the vault holds no path at %s", tt.path)
			}
			cover := coverMain(t, renderedHTML(t, Syllabus(BuildPathView(path, model.Paths(), &CourseCover{}), chrome)))

			rail := NewReadingRail(shell, tt.middle, "")
			head := regexp.MustCompile(`<p class="y-railbook__span">([^<]*)</p>`).FindStringSubmatch(renderedHTML(t, readingRail(rail, chrome)))
			if head == nil {
				t.Fatalf("the reading rail of %s draws no book head", tt.middle)
			}

			foot := FooterSequence(&rail, tt.lang)
			steps := renderedHTML(t, sequenceSteps(NoteView{Prev: foot.Prev, Next: foot.Next, StepsLabel: foot.Label, StepsCourse: foot.Course, StepsUnit: foot.Unit}, tt.lang))

			drawer := renderedHTML(t, sidebar(NewSidebar(shell, tt.middle), chrome))

			index := renderedHTML(t, ListIndex(NewPathIndex(model.Paths(), schema.NavigationRoles{}, nav.Closure{}, ContractGoverning, tt.lang, nil), chrome))

			if head[1] != tt.extent {
				t.Errorf("the rail's book head reads %q, want %q", head[1], tt.extent)
			}
			if !strings.Contains(cover, tt.extent) {
				t.Errorf("the cover does not state %q", tt.extent)
			}
			if !strings.Contains(steps, tt.prev) || !strings.Contains(steps, tt.next) {
				t.Errorf("the foot does not offer %q and %q; html = %q", tt.prev, tt.next, steps)
			}
			if !strings.Contains(drawer, tt.railPrev) {
				t.Errorf("the sidebar's steps do not offer %q", tt.railPrev)
			}
			if !strings.Contains(index, tt.extent+"</span>") && !strings.Contains(index, tt.extent) {
				t.Errorf("the path index does not state %q for %s", tt.extent, tt.path)
			}
			for where, html := range map[string]string{"cover": cover, "foot": steps} {
				if strings.Contains(wordsOf(html), tt.otherExtent) {
					t.Errorf("the %s of %s says %q, the other noun's count", where, tt.path, tt.otherExtent)
				}
				if tt.otherStep != "" && strings.Contains(wordsOf(html), tt.otherStep) {
					t.Errorf("the %s of %s says %q, the other noun's step word", where, tt.path, tt.otherStep)
				}
			}
		})
	}
}

// TestNoKickerEndsInASeparator holds the line above a path's title on both pages
// that carry it. The line is the whole of what it says, so there is nothing
// after it for a separator to join to, and a mark left at its end reads as a
// sentence that stopped.
func TestNoKickerEndsInASeparator(t *testing.T) {
	t.Parallel()

	kicker := regexp.MustCompile(`<div class="y-syl-kicker">([^<]*)</div>`)
	for _, tt := range []struct {
		lang wording.Lang
		want string
	}{
		{wording.ZhHant, "學習路徑"},
		{wording.En, "Study path"},
	} {
		pages := map[string]string{
			"cover":  renderedHTML(t, Syllabus(PathView{Title: "Path"}, layouts.Chrome{Lang: tt.lang})),
			"listen": renderedHTML(t, Listen(ListenView{Title: "Path"}, layouts.Chrome{Lang: tt.lang})),
		}
		for name, html := range pages {
			found := kicker.FindStringSubmatch(html)
			if found == nil {
				t.Errorf("the %s in %s carries no kicker; the check below proves nothing", name, tt.lang)
				continue
			}
			if found[1] != tt.want {
				t.Errorf("the %s kicker in %s reads %q, want %q", name, tt.lang, found[1], tt.want)
			}
			if strings.HasSuffix(strings.TrimSpace(found[1]), "·") {
				t.Errorf("the %s kicker in %s ends in a separator: %q", name, tt.lang, found[1])
			}
		}
	}
}

// annotatedCourse is a path annotated the way an author writes one, drawn
// through the real navigation build: a sentence beside two of its lessons, a
// side branch hanging from the second, an appendix of one reference and a
// row nobody wrote after the last part, and something in it marked to be read
// aloud.
func annotatedCourse(t *testing.T) PathView {
	t.Helper()
	path := buildTestPath(t, "## Data {sequence=primary}\n\n"+
		"- [[Slices]] — 啟動工作，等它完成\n"+
		"- [[Arrays]] — 固定長度的序列\n"+
		"\t- 要限制等待時間時 {sequence=local}\n"+
		"\t\t- [[Tuning]] — 等待值、取消或計時器\n"+
		"- [[GC]]\n"+
		"\n## 查閱與對照 {sequence=none}\n\n"+
		"- [[Reference]] — 等待、交接與收尾的關係\n"+
		"- [[Nobody wrote this]] — 還沒寫\n",
		map[string]string{"Writing/Reference.md": "---\ntitle: Reference\ntype: concept\n---\nbody\n"})
	return BuildPathView(&path, []nav.Path{path}, &CourseCover{OpeningLanguage: "zh-Hant", Listenable: true, Here: "Writing/Arrays.md"})
}

// TestTheRailDrawsASideBranchAfterTheLessonItHangsFrom holds the order the book
// in the reading rail lists its rows in. The branch is carried by its lesson in
// the view now, and the rail has to put it back where the author wrote it:
// right after that lesson and before the next one, not after the run and not
// dropped.
func TestTheRailDrawsASideBranchAfterTheLessonItHangsFrom(t *testing.T) {
	t.Parallel()

	root := writeVaultFiles(t, map[string]string{
		"Maps/Course.md": "---\ntitle: Course\ntype: study-path\n---\n\n" +
			"## Main {sequence=primary}\n\n- [[L01]]\n- [[L02]]\n\t- 選修 {sequence=local}\n\t\t- [[S01]]\n- [[L03]]\n",
		"Writing/L01.md": "---\ntitle: L01\ntype: lesson\nstatus: draft\n---\nbody\n",
		"Writing/L02.md": "---\ntitle: L02\ntype: lesson\nstatus: draft\n---\nbody\n",
		"Writing/L03.md": "---\ntitle: L03\ntype: lesson\nstatus: draft\n---\nbody\n",
		"Writing/S01.md": "---\ntitle: S01\ntype: lesson\nstatus: draft\n---\nbody\n",
	})
	model := modelOf(t, root)
	rail := NewReadingRail(nav.Shell{Nav: model}, "Writing/L02.md", "")
	html := renderedHTML(t, readingRail(rail, layouts.Chrome{Lang: wording.ZhHant, Nonce: "n"}))

	var order []string
	for _, m := range regexp.MustCompile(`href="/notes/Writing/(L0[123]|S01)\.md"|data-book-branch="Maps/Course\.md[^"]*選修"`).FindAllStringSubmatch(html, -1) {
		if m[1] != "" {
			order = append(order, m[1])
		} else {
			order = append(order, "BRANCH")
		}
	}
	// The branch's own lesson follows the branch's heading.
	want := []string{"L01", "L02", "BRANCH", "S01", "L03"}
	if diff := cmp.Diff(want, order); diff != "" {
		t.Errorf("the rail lists the book in the wrong order (-want +got):\n%s", diff)
	}
}
