package note_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	htmlnode "golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/koopa0/yomihon/internal/mark"
	"github.com/koopa0/yomihon/internal/mark/machinemarks"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/note"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/wording"
)

func TestReadingIsTheWordsOfThePageAndTheIdentityOfTheBytes(t *testing.T) {
	t.Parallel()

	handler, root := thoughtRouteHandler(t, false, "")
	page, ok := handler.Reading("Source.md", wording.ZhHant)
	if !ok {
		t.Fatal("a note the vault holds is not readable")
	}
	if !strings.Contains(page.Text, "PROMPT AND EXPLANATION MUST NOT BE COPIED") || !strings.Contains(page.Text, "Chapter one") {
		t.Errorf("the words of the page are %q, want its heading and its paragraph", page.Text)
	}
	if strings.Contains(page.Text, "type:") || strings.Contains(page.Text, "---") {
		t.Errorf("the words of the page are %q, which carry the frontmatter", page.Text)
	}
	data, err := os.ReadFile(filepath.Join(root, "Source.md")) // #nosec G304 -- a file this test wrote under t.TempDir
	if err != nil {
		t.Fatal(err)
	}
	if sum := sha256.Sum256(data); page.Identity != hex.EncodeToString(sum[:]) {
		t.Errorf("identity = %s, want the SHA-256 of every byte of the file, %x", page.Identity, sum)
	}
}

func TestReadingRefusesWhatNoReaderOpens(t *testing.T) {
	t.Parallel()

	handler, _ := thoughtRouteHandler(t, false, "")
	for _, rel := range []string{"Missing.md", "../Source.md", "", "Source.txt", "/Source.md"} {
		if page, ok := handler.Reading(rel, wording.En); ok {
			t.Errorf("Reading(%q) = %+v, want a note that is not there", rel, page)
		}
	}
}

// readingLibrary is the handler for the example vault and the generation it
// answers from.
func readingLibrary(t *testing.T) (*note.Handler, *snapshot.Generation) {
	t.Helper()
	rules := readingLibraryContract(t)
	handler, snap := readingLibraryUnder(t, func() schema.Authority { return schema.CaptureAuthority(rules, rules.Governance()) })
	return handler, snap
}

// readingLibraryUnder is the same handler answering under the given authority.
func readingLibraryUnder(t *testing.T, authority func() schema.Authority) (*note.Handler, *snapshot.Generation) {
	t.Helper()
	rules := readingLibraryContract(t)
	log := slog.New(slog.DiscardHandler)
	store, source := newSnapshotStore(t, readingLibraryRoot, log, rules, rules.Governance())
	handler := note.New(&note.Sources{
		Source: source, Contract: rules, Snapshot: store.Current, Bookmark: noMark, Log: log, Authority: authority,
	})
	return handler, store.Current().Capture()
}

// TestRulesThatCloseTheCoursesCloseWhatTheRoutesAskOfThem holds that a bookmark
// in a course and a finished note that names one are judged by the courses the
// sidebar draws, not by the raw navigation model: where the rules file cannot be
// trusted there are no courses, so the courses answer none and the routes
// refuse, and write nothing. The same requests are taken under open rules, so
// the refusal is the closure and not a request that was wrong.
func TestRulesThatCloseTheCoursesCloseWhatTheRoutesAskOfThem(t *testing.T) {
	t.Parallel()

	rules := readingLibraryContract(t)
	open := func() schema.Authority { return schema.CaptureAuthority(rules, rules.Governance()) }
	closed := func() schema.Authority {
		return schema.CaptureAuthority(nil, schema.Unreadable(errors.New("the rules file cannot be read")))
	}

	_, snap := readingLibraryUnder(t, open)
	courses := snap.Navigation().Paths()
	if len(courses) == 0 {
		t.Fatal("the example vault has no course")
	}
	course := courses[0].RelPath
	mainLine, _, _ := mustUnits(t, snap, course)
	unit := mainLine[0]

	for name, tt := range map[string]struct {
		authority func() schema.Authority
		want      int
	}{
		"rules that answer":     {open, http.StatusOK},
		"rules that are closed": {closed, http.StatusUnprocessableEntity},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			handler, _ := readingLibraryUnder(t, tt.authority)
			if _, _, ok := handler.Units(course); ok != (tt.want == http.StatusOK) {
				t.Errorf("Units(%q) ok = %v under %s", course, ok, name)
			}
			if _, _, ok := handler.Rows(course); ok != (tt.want == http.StatusOK) {
				t.Errorf("Rows(%q) ok = %v under %s", course, ok, name)
			}
			configDir := t.TempDir()
			// Neither route here keeps a mark, so nothing is written into the
			// example vault the reader folder is opened over.
			marks, kept := openKept(t, openReadingVault(t, readingLibraryRoot), configDir)
			mux := http.NewServeMux()
			mark.NewHandler(marks, kept, handler, slog.New(slog.DiscardHandler)).Register(mux)

			body := `{"course":` + jsonString(course) + `,"path":` + jsonString(unit) + `,"anchor":"","offset":0,"identity":"` + strings.Repeat("a", 64) + `"}`
			for _, request := range []struct{ method, target, body string }{
				{http.MethodPut, "/bookmark", body},
				{http.MethodPut, "/finished/" + (&url.URL{Path: unit}).EscapedPath() + "?course=" + url.QueryEscape(course), ""},
			} {
				got := httptest.NewRecorder()
				req := httptest.NewRequestWithContext(t.Context(), request.method, request.target, strings.NewReader(request.body))
				req.Header.Set("Content-Type", "application/json")
				mux.ServeHTTP(got, req)
				if got.Code != tt.want {
					t.Errorf("%s %s under %s = %d %q, want %d", request.method, request.target, name, got.Code, got.Body.String(), tt.want)
				}
			}
			_, found, err := marks.Bookmark(course)
			if err != nil || found != (tt.want == http.StatusOK) {
				t.Errorf("the course's bookmark kept = %v, %v under %s", found, err, name)
			}
			if tt.want != http.StatusOK {
				if _, statErr := os.Stat(filepath.Join(marks.Dir(), "progress.json")); !errors.Is(statErr, fs.ErrNotExist) {
					t.Errorf("a refused request left progress behind: %v", statErr)
				}
			}
		})
	}
}

func mustUnits(t *testing.T, snap *snapshot.Generation, course string) (mainLine, branches []string, ok bool) {
	t.Helper()
	main, side, ok := snap.Navigation().CourseUnits(course)
	if !ok || len(main) == 0 {
		t.Fatalf("the course %q has no unit to read", course)
	}
	for _, ref := range main {
		mainLine = append(mainLine, ref.RelPath)
	}
	for _, ref := range side {
		branches = append(branches, ref.RelPath)
	}
	return mainLine, branches, ok
}

func jsonString(s string) string {
	data, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(data)
}

// TestUnitsAreWhatTheCoursesOfTheExampleVaultWalk holds the list the finished
// count and a bookmark's course are judged by against the navigation model the
// course page and the sidebar read: every course answers, each unit it names is a
// note a reader can open, a course that walks a note is one the navigation says
// walks it, and a note that is no course answers none.
func TestUnitsAreWhatTheCoursesOfTheExampleVaultWalk(t *testing.T) {
	t.Parallel()

	handler, snap := readingLibrary(t)
	model := snap.Navigation()
	courses := model.Paths()
	if len(courses) < 2 {
		t.Fatalf("the example vault has %d courses, want its two", len(courses))
	}
	walked := 0
	for i := range courses {
		mainLine, branches, ok := handler.Units(courses[i].RelPath)
		if !ok {
			t.Errorf("Units(%q) = not a course, want the course the navigation holds", courses[i].RelPath)
			continue
		}
		if len(mainLine) == 0 {
			t.Errorf("Units(%q) has an empty main line", courses[i].RelPath)
		}
		planned, rows, rowsOK := handler.Rows(courses[i].RelPath)
		if !rowsOK || planned != courses[i].Planned {
			t.Errorf("Rows(%q) = %d planned, %v; want the %d the course page prints", courses[i].RelPath, planned, rowsOK, courses[i].Planned)
		}
		if planned < len(rows) {
			t.Errorf("Rows(%q) names %d rows of %d planned", courses[i].RelPath, len(rows), planned)
		}
		for _, row := range rows {
			if !slices.Contains(mainLine, row) {
				t.Errorf("%q is a row of %q and not a unit of its main line", row, courses[i].RelPath)
			}
		}
		for _, rel := range slices.Concat(mainLine, branches) {
			walked++
			if !handler.HasPlace(rel, "") {
				t.Errorf("%q is a unit of %q and is not a note a reader can open", rel, courses[i].RelPath)
			}
			if !slices.ContainsFunc(model.PathNeighbors(rel), func(n nav.Neighbors) bool { return n.PathRelPath == courses[i].RelPath }) {
				t.Errorf("%q is a unit of %q and the navigation says no such course walks it", rel, courses[i].RelPath)
			}
		}
	}
	if walked < 10 {
		t.Fatalf("the courses walk %d units between them, so the check looked at too little to hold anything", walked)
	}
	// And the other way: a note the navigation says a course walks is a unit
	// the course names, so a unit dropped from the list does not go unnoticed.
	err := filepath.WalkDir(readingLibraryRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".md") {
			return err
		}
		rel, err := filepath.Rel(readingLibraryRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		for _, step := range model.PathNeighbors(rel) {
			mainLine, branches, _ := handler.Units(step.PathRelPath)
			if !slices.Contains(mainLine, rel) && !slices.Contains(branches, rel) {
				t.Errorf("the navigation says %q walks %q and Units does not name it", step.PathRelPath, rel)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	firstUnit, _, _ := handler.Units(courses[0].RelPath)
	for _, rel := range []string{"README.md", firstUnit[0], "Missing.md", ""} {
		if _, _, ok := handler.Units(rel); ok {
			t.Errorf("Units(%q) answered as a course", rel)
		}
	}
}

// TestEveryWordAPageShowsIsFoundWhereItIs holds what the mark routes depend on:
// a reader who selects the words of one text node of a page, in any note of the
// example vault, has chosen words Locate finds exactly, or, where the page has
// them at several places and the selection says nothing of what stands around
// them, words it does not place. A rule of TextContent that dropped or changed
// words would show here as a highlight the server refuses.
func TestEveryWordAPageShowsIsFoundWhereItIs(t *testing.T) {
	t.Parallel()

	handler, snap := readingLibrary(t)

	notes, nodes := 0, 0
	err := filepath.WalkDir(readingLibraryRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".md") {
			return err
		}
		rel, err := filepath.Rel(readingLibraryRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		page, ok := handler.Reading(rel, wording.ZhHant)
		if !ok {
			return nil
		}
		notes++
		reading, _ := snap.Note(rel)
		squeezed := strings.Join(strings.Fields(page.Text), " ")
		for _, words := range textNodes(t, snap.Render(rel, reading.Body, wording.ZhHant).HTML) {
			m := &machinemarks.Mark{Kind: machinemarks.Highlight, Path: rel, Quote: words}
			found := mark.Locate(page.Text, nil, m)
			if places(squeezed, words) > 1 {
				if found.Status != mark.Changed {
					t.Errorf("%s: the words %q of a text node are at several places of the page's words, and Locate answers status %v", rel, words, found.Status)
				}
				continue
			}
			nodes++
			if found.Status != mark.Found || !found.Exact {
				t.Errorf("%s: the words %q of a text node are not found exactly in the page's words (status %v)", rel, words, found.Status)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if notes < 10 || nodes < 100 {
		t.Fatalf("the walk read %d notes and %d text nodes it could place, so it looked at too little to hold anything", notes, nodes)
	}
}

// TestAQuoteOfTheBaseTextAcrossRubyIsFoundWhereItIs holds what a reader who
// selects across a ruby has chosen: the base text, the reading being no part of
// what is selected. Locate finds those words exactly, at the sentence they are
// in, whether the author wrote the reading's brackets or not and however many
// pairs of base and reading the ruby holds.
func TestAQuoteOfTheBaseTextAcrossRubyIsFoundWhereItIs(t *testing.T) {
	t.Parallel()

	const rel = "Notes/Snow.md"
	root := writeNotes(t, map[string]string{rel: "---\ntype: note\n---\n# Snow\n\n" +
		"国境の長いトンネルを抜けると<ruby>雪国<rp>(</rp><rt>ゆきぐに</rt><rp>)</rp></ruby>であった。夜の底が白くなった。\n\n" +
		"<ruby>東<rt>とう</rt>京<rt>きょう</rt></ruby>と<ruby>仮名</ruby>を読む。\n"})
	log := slog.New(slog.DiscardHandler)
	store, source := newSnapshotStore(t, root, log, nil, schema.Ungoverned())
	handler := note.New(&note.Sources{
		Source: source, Snapshot: store.Current, Bookmark: noMark, Log: log,
		Authority: func() schema.Authority { return schema.CaptureAuthority(nil, schema.Ungoverned()) },
	})
	page, ok := handler.Reading(rel, wording.En)
	if !ok {
		t.Fatalf("Reading(%q) is not a note a reader can open", rel)
	}

	for _, tt := range []struct {
		name, quote string
		want        mark.Status
	}{
		{"across a ruby that has brackets", "国境の長いトンネルを抜けると雪国であった。", mark.Found},
		{"the base alone", "雪国", mark.Found},
		{"from inside a base to past it", "国であった。夜の底", mark.Found},
		{"across a ruby of two pairs", "東京と", mark.Found},
		{"across a ruby that has no reading", "と仮名を読む。", mark.Found},
		{"a reading the page does not select", "雪国ゆきぐにであった", mark.Changed},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := mark.Locate(page.Text, nil, &machinemarks.Mark{Kind: machinemarks.Highlight, Path: rel, Quote: tt.quote})
			if got.Status != tt.want {
				t.Fatalf("Locate(%q) status = %v, want %v in %q", tt.quote, got.Status, tt.want, page.Text)
			}
			if tt.want != mark.Found {
				return
			}
			if !got.Exact || page.Text[got.Start:got.End] != tt.quote {
				t.Errorf("Locate(%q) = exact %v at %q, want the words themselves", tt.quote, got.Exact, page.Text[got.Start:got.End])
			}
		})
	}
}

// places is how many places words begin in text, a place that overlaps another
// counting as one more.
func places(text, words string) int {
	n := 0
	for from := 0; ; from++ {
		at := strings.Index(text[from:], words)
		if at < 0 {
			return n
		}
		n++
		from += at
	}
}

// textNodes is the words of each text node of an HTML fragment that a reader
// can select, one string each, with the white space of each run made one space
// and nothing but white space left out. A ruby reading is not selected with
// its base, so it is not among them.
func textNodes(t *testing.T, fragment string) []string {
	t.Helper()
	root, err := htmlnode.ParseFragment(strings.NewReader(fragment), &htmlnode.Node{Type: htmlnode.ElementNode, Data: "div", DataAtom: atom.Div})
	if err != nil {
		t.Fatal(err)
	}
	var words []string
	var walk func(*htmlnode.Node)
	walk = func(n *htmlnode.Node) {
		if n.Type == htmlnode.ElementNode && slices.Contains([]string{"script", "style", "template", "rp", "rt", "rtc"}, n.Data) {
			return
		}
		if n.Type == htmlnode.TextNode {
			if text := strings.Join(strings.Fields(n.Data), " "); text != "" {
				words = append(words, text)
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	for _, n := range root {
		walk(n)
	}
	return words
}
