package syllabus_test

import (
	"log/slog"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/syllabus"
)

// spokenText is what the page hands the voice for one paragraph: the attribute
// the runtime reads, which is the observable a listener actually receives.
var spokenText = regexp.MustCompile(`data-tts="([^"]*)"`)

// lessonHeading is one lesson's heading on the listening page, with the note it
// leads to.
var lessonHeading = regexp.MustCompile(`<h2 class="y-listen__title"><a href="([^"]*)">([^<]*)</a></h2>`)

// listenVault writes a course of two lessons that mark three and two
// paragraphs, plus every kind of note the page must leave out: a lesson no
// course row names, a row the course does name whose note type is not the
// lesson type, a row under a heading that declared no sequence, and a row under
// a branch the grammar draws but does not sequence. The last two are what the
// taught rule is for; without them nothing here would notice its loss. The main
// line also plans one lesson nobody has written, between the two that are.
func listenVault(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	lessonDir := filepath.Join(root, "Writing", "lessons", "golang")
	mapsDir := filepath.Join(root, "Maps")
	for _, dir := range []string{lessonDir, mapsDir} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	write := func(dir, name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	note := func(title, noteType string, paragraphs ...string) string {
		b := strings.Builder{}
		b.WriteString("---\ntitle: " + title + "\ntype: " + noteType + "\ndomain: golang\nstatus: ready\n" +
			"created: 2026-06-01\nupdated: 2026-06-01\n---\n\nunmarked opening line\n")
		for _, paragraph := range paragraphs {
			b.WriteString("\n<!-- read-aloud: ja -->\n" + paragraph + "\n")
		}
		return b.String()
	}
	write(lessonDir, "First.md", note("First", "lesson", "いち。", "に。", "さん。"))
	write(lessonDir, "Second.md", note("Second", "lesson", "し。", "ご。"))
	// Marked, and never taught by this course.
	write(lessonDir, "Untaught.md", note("Untaught", "lesson", "ろく。"))
	// Taught, marked, and not a lesson: the reading page gives it no speaker
	// either, so the course cannot be listened past the vault's own rule.
	write(lessonDir, "Concept.md", note("Concept", "concept", "なな。"))
	// Listed under branches the course does not sequence.
	write(lessonDir, "Undeclared.md", note("Undeclared", "lesson", "はち。"))
	write(lessonDir, "Unsequenced.md", note("Unsequenced", "lesson", "きゅう。"))
	write(mapsDir, "Path.md", "---\ntitle: Go path\ntype: study-path\ndomain: golang\nstatus: evergreen\n"+
		"created: 2026-06-01\nupdated: 2026-06-01\n---\n\n"+
		"## line | Line | 線 {sequence=primary}\n\n- [[First]]\n- [[Second]]\n- [[Nowhere]]\n- [[Concept]]\n\n"+
		"## aside | Aside | 旁註\n\n- [[Undeclared]]\n\n"+
		"## loose | Loose | 散 {sequence=none}\n\n- [[Unsequenced]]\n")
	return root
}

// listenServer serves the listening page over one published generation of root.
func listenServer(t *testing.T, root string, status syllabus.LessonTypes) *httptest.Server {
	t.Helper()
	contract, err := schema.LoadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("schema.LoadFile = %v", err)
	}
	store := newGenerationStore(t, root, contract)
	mux := http.NewServeMux()
	syllabus.New(func() syllabus.RequestSnapshot {
		snap := store.Current().Capture()
		return syllabus.RequestSnapshot{
			Shell:      nav.Shell{Nav: snap.Navigation(), Governed: true},
			Generation: snap,
			Status:     status,
		}
	}, slog.New(slog.DiscardHandler)).Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// listenColumn is the page's own column, so a failure quotes what the reader
// would have seen rather than every byte of the shared chrome around it.
func listenColumn(t *testing.T, page string) string {
	t.Helper()
	const open = `<div class="y-listen">`
	at := strings.Index(page, open)
	if at < 0 {
		t.Fatalf("the response carries no listening column at all")
	}
	return page[at:min(at+600, len(page))]
}

// lessonTypes answers the one question the listening page asks the contract.
type lessonTypes string

func (l lessonTypes) IsLessonType(noteType string) bool { return noteType == string(l) }

// TestTheListeningPageCarriesEveryMarkedParagraphOnceInCourseOrder is the
// page's whole claim. The wanted list is written out by hand rather than
// gathered the way the page gathers it: a list derived the way the code derives
// it would agree with the code however wrong both were. Compared as a sequence,
// so a paragraph said twice, a lesson dropped, and two lessons swapped each
// fail as a different diff.
func TestTheListeningPageCarriesEveryMarkedParagraphOnceInCourseOrder(t *testing.T) {
	t.Parallel()

	srv := listenServer(t, listenVault(t), lessonTypes("lesson"))
	code, page := get(t, srv.Client(), srv.URL+"/listen/Maps/Path.md")
	if code != http.StatusOK {
		t.Fatalf("GET the listening page status = %d, want 200", code)
	}

	var spoken []string
	for _, match := range spokenText.FindAllStringSubmatch(page, -1) {
		spoken = append(spoken, match[1])
	}
	want := []string{"いち。", "に。", "さん。", "し。", "ご。"}
	if diff := cmp.Diff(want, spoken); diff != "" {
		t.Errorf("the course is read aloud in the wrong words or the wrong order (-want +got):\n%s", diff)
	}

	var headings [][2]string
	for _, match := range lessonHeading.FindAllStringSubmatch(page, -1) {
		headings = append(headings, [2]string{match[2], match[1]})
	}
	wantHeadings := [][2]string{
		{"First", "/notes/Writing/lessons/golang/First.md"},
		{"Second", "/notes/Writing/lessons/golang/Second.md"},
	}
	if diff := cmp.Diff(wantHeadings, headings); diff != "" {
		t.Errorf("the lessons are not the ones the course teaches, in its order, each leading to its own note (-want +got):\n%s", diff)
	}

	// Named on their own as well as by their absence above, so a failure says
	// which rule stopped holding rather than only that the list moved.
	for _, withheld := range []struct {
		why    string
		spoken string
	}{
		{"a lesson no row of the course names", "ろく。"},
		{"a row the course names whose note type is not the lesson type", "なな。"},
		{"a row under a heading that declared no sequence", "はち。"},
		{"a row under a branch the course draws but does not sequence", "きゅう。"},
	} {
		if strings.Contains(page, withheld.spoken) {
			t.Errorf("the page reads %s aloud, which is %s", withheld.spoken, withheld.why)
		}
	}
}

// TestACourseThatMarksNothingSaysSo covers the page a course with no marked
// paragraph draws. Nothing at all would read as a page that failed to load, and
// a bar with nothing to play is a control that cannot work.
func TestACourseThatMarksNothingSaysSo(t *testing.T) {
	t.Parallel()

	// The same vault, with a status vocabulary that files nothing as a lesson:
	// every row is then taught and none of them contributes a paragraph.
	srv := listenServer(t, listenVault(t), lessonTypes("nothing-is-a-lesson"))
	code, page := get(t, srv.Client(), srv.URL+"/listen/Maps/Path.md")
	if code != http.StatusOK {
		t.Fatalf("GET the listening page status = %d, want 200", code)
	}
	if !strings.Contains(page, "這條路徑沒有標記朗讀的段落。") {
		t.Errorf("a course with nothing marked says nothing about it; its own column reads %q", listenColumn(t, page))
	}
	for _, absent := range []string{"data-tts=", "data-readaloud-controls", "data-readaloud-bar"} {
		if strings.Contains(page, absent) {
			t.Errorf("a course with nothing marked still carries %s, so the page offers a bar that can play nothing", absent)
		}
	}
}

// TestAGenerationWithNoStatusAuthorityTeachesNothing holds the absent-contract
// case to the same answer the reading page gives: without the vocabulary that
// names the lesson type, nothing is claimed to be a lesson.
func TestAGenerationWithNoStatusAuthorityTeachesNothing(t *testing.T) {
	t.Parallel()

	srv := listenServer(t, listenVault(t), nil)
	code, page := get(t, srv.Client(), srv.URL+"/listen/Maps/Path.md")
	if code != http.StatusOK {
		t.Fatalf("GET the listening page status = %d, want 200", code)
	}
	if n := strings.Count(page, "data-tts="); n != 0 {
		t.Errorf("a generation carrying no status vocabulary read %d paragraphs aloud, want none; its own column reads %q", n, listenColumn(t, page))
	}
}

// TestAnUnknownCourseRefusesTheWayTheCoursePageDoes keeps the two addresses
// that take the same name answering the same way.
func TestAnUnknownCourseRefusesTheWayTheCoursePageDoes(t *testing.T) {
	t.Parallel()

	srv := listenServer(t, listenVault(t), lessonTypes("lesson"))
	for _, target := range []string{"/listen/Maps/nothing-here.md", "/syllabus/Maps/nothing-here.md"} {
		code, page := get(t, srv.Client(), srv.URL+target)
		if code != http.StatusNotFound {
			t.Errorf("GET %s status = %d, want 404", target, code)
		}
		if !strings.Contains(page, `class="y-recovery"`) {
			t.Errorf("GET %s answered without the shared recovery page", target)
		}
	}
}

// TestTheListeningPageNamesALessonAsTheCoursePageDoes holds that a lesson's
// heading here is the name its row prints on the course page: the alias its
// row wrote, else the note's declared title, else the link text. Before, the
// page read the note's title with a file-name fallback, so an aliased row and
// a row over an untitled note read differently on the two pages.
func TestTheListeningPageNamesALessonAsTheCoursePageDoes(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	lessonDir := filepath.Join(root, "Writing", "lessons", "golang")
	mapsDir := filepath.Join(root, "Maps")
	for _, dir := range []string{lessonDir, mapsDir} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	lesson := func(name, title string) {
		head := "---\n"
		if title != "" {
			head += "title: \"" + title + "\"\n"
		}
		head += "type: lesson\ndomain: golang\nstatus: ready\ncreated: 2026-06-01\nupdated: 2026-06-01\n---\n\n"
		body := head + "<!-- read-aloud: ja -->\nいち。\n"
		if err := os.WriteFile(filepath.Join(lessonDir, name+".md"), []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	lesson("First", "First: the title")
	lesson("Second", "Second: the title")
	lesson("Third", "")
	path := "---\ntitle: Go path\ntype: study-path\ndomain: golang\nstatus: evergreen\ncreated: 2026-06-01\nupdated: 2026-06-01\n---\n\n" +
		"## line | Line | 線 {sequence=primary}\n\n- [[First|Alias words]]\n- [[Second]]\n- [[Writing/lessons/golang/Third]]\n"
	if err := os.WriteFile(filepath.Join(mapsDir, "Path.md"), []byte(path), 0o600); err != nil {
		t.Fatalf("write path: %v", err)
	}

	srv := listenServer(t, root, lessonTypes("lesson"))
	code, page := get(t, srv.Client(), srv.URL+"/listen/Maps/Path.md")
	if code != http.StatusOK {
		t.Fatalf("GET the listening page status = %d, want 200", code)
	}
	var got []string
	for _, match := range lessonHeading.FindAllStringSubmatch(page, -1) {
		got = append(got, match[2])
	}
	want := []string{"Alias words", "Second: the title", "Writing/lessons/golang/Third"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("lesson headings (-want +got):\n%s", diff)
	}
}

// agreementVault is a course of one lesson, written by build. The lesson's
// body is whatever the case under test needs it to be, so the cover and the
// listening page are asked about the same bytes.
func agreementVault(t *testing.T, lessonBody string, extra map[string]string) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"Writing/lessons/golang/First.md": "---\ntitle: First\ntype: lesson\ndomain: golang\nstatus: ready\ncreated: 2026-06-01\nupdated: 2026-06-01\n---\n\n" + lessonBody,
		"Maps/Path.md": "---\ntitle: Go path\ntype: study-path\ndomain: golang\nstatus: evergreen\ncreated: 2026-06-01\nupdated: 2026-06-01\n---\n\n" +
			"## line | Line | 線 {sequence=primary}\n\n- [[First]]\n",
	}
	maps.Copy(files, extra)
	for rel, content := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	return root
}

// TestTheCoverOffersListeningExactlyWhereTheListeningPageHasSomethingToPlay
// holds the offer to the page it leads to. The two are asked of the same
// course and the same bytes and must agree, whichever way: a cover that offers
// a page saying there is nothing to hear is a dead end, and a cover that hides
// one that plays is a door nobody can find. The cases are the ways the answer
// can fall — a marked paragraph, none at all, the marker's word in prose that
// is not a marker, a marker that arrives through an embed of another note, and
// a generation that names no lesson type, which teaches nothing.
func TestTheCoverOffersListeningExactlyWhereTheListeningPageHasSomethingToPlay(t *testing.T) {
	t.Parallel()

	const marked = "<!-- read-aloud: ja -->\nいち。\n"
	tests := []struct {
		name   string
		body   string
		extra  map[string]string
		status syllabus.LessonTypes
		want   bool
	}{
		{name: "a lesson with a marked paragraph", body: marked, status: lessonTypes("lesson"), want: true},
		{name: "lessons that mark nothing", body: "いち。\n", status: lessonTypes("lesson"), want: false},
		{name: "the marker's word in prose that is no marker", body: "Write read-aloud: ja before a paragraph to have it spoken.\n", status: lessonTypes("lesson"), want: false},
		{name: "a marker for a language the page does not speak", body: "<!-- read-aloud: zh -->\nいち。\n", status: lessonTypes("lesson"), want: false},
		{
			name:   "a marker that arrives through an embed",
			body:   "![[Marked source]]\n",
			extra:  map[string]string{"Writing/lessons/golang/Marked source.md": "---\ntitle: Marked source\ntype: lesson\ndomain: golang\nstatus: ready\ncreated: 2026-06-01\nupdated: 2026-06-01\n---\n\n" + marked},
			status: lessonTypes("lesson"),
			want:   true,
		},
		{name: "a generation that names no lesson type", body: marked, status: nil, want: false},
		{name: "a vault that files nothing as a lesson", body: marked, status: lessonTypes("nothing-is-a-lesson"), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := listenServer(t, agreementVault(t, tt.body, tt.extra), tt.status)

			_, listenPage := get(t, srv.Client(), srv.URL+"/listen/Maps/Path.md")
			plays := strings.Contains(listenPage, "data-tts=")
			_, cover := get(t, srv.Client(), srv.URL+"/syllabus/Maps/Path.md")
			offers := strings.Contains(cover, `href="/listen/Maps/Path.md"`)

			if plays != tt.want {
				t.Fatalf("the listening page plays something = %t, want %t; the case does not describe the bytes it was written for, so what the cover does proves nothing", plays, tt.want)
			}
			if offers != plays {
				t.Errorf("the cover offers the listening page = %t while the page plays something = %t", offers, plays)
			}
		})
	}
}
