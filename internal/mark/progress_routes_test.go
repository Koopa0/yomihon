package mark_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/mark"
)

const (
	goCourse   = "Courses/go.md"
	rustCourse = "Courses/rust.md"
)

func asJSON(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// aSchool is notes with courses: each course path and what it walks.
type aSchool struct {
	notes

	courses map[string]aCourse
}

// aCourse is a course as the navigation model holds it: how many rows its main
// line plans, the notes of those a reader can open (a note listed twice is two
// rows) and the notes of its side branches.
type aCourse struct {
	planned  int
	rows     []string
	branches []string
}

// Units is each note the course walks once, the main line apart from the side
// branches.
func (s aSchool) Units(course string) (mainLine, branches []string, ok bool) {
	walked, ok := s.courses[course]
	for _, row := range walked.rows {
		if !slices.Contains(mainLine, row) {
			mainLine = append(mainLine, row)
		}
	}
	return mainLine, walked.branches, ok
}

func (s aSchool) Rows(course string) (planned int, rows []string, ok bool) {
	walked, ok := s.courses[course]
	return walked.planned, walked.rows, ok
}

// theSchool is one vault with two courses, a note in neither, a note one course
// has on a side branch, and a course that plans a row it has not written.
func theSchool() aSchool {
	return aSchool{
		notes: notes{
			"Notes/a.md": "a", "Notes/b.md": "b", "Notes/c.md": "c", "Notes/side.md": "s",
			"Notes/loose.md": "l", "Notes/other.md": "o",
		},
		courses: map[string]aCourse{
			// Four rows are planned and one of them is not written yet, so three
			// can be opened, and b is listed twice.
			goCourse:   {planned: 5, rows: []string{"Notes/a.md", "Notes/b.md", "Notes/c.md", "Notes/b.md"}, branches: []string{"Notes/side.md"}},
			rustCourse: {planned: 1, rows: []string{"Notes/other.md"}},
		},
	}
}

func aBookmarkBody(course, path string) map[string]any {
	return map[string]any{
		"course": course, "path": path, "anchor": "the-topic-particle", "offset": 420, "identity": anIdentity,
	}
}

func putBookmark(t *testing.T, handler http.Handler, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	return send(t, handler, http.MethodPut, mark.Paths.Bookmark, jsonBody, asJSON(t, body))
}

// goBookmark is where the bookmark of the Go course is asked of and cleared.
var goBookmark = mark.Paths.Bookmark + "?course=" + url.QueryEscape(goCourse)

// moved is what setting a bookmark answers.
type moved struct {
	Bookmark mark.Bookmark  `json:"bookmark"`
	Previous *mark.Bookmark `json:"previous"`
}

func decodeMoved(t *testing.T, response *httptest.ResponseRecorder) moved {
	t.Helper()
	if response.Code != http.StatusOK {
		t.Fatalf("setting a bookmark = %d %q, want 200", response.Code, response.Body.String())
	}
	var got moved
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatalf("the answer %q is not a moved bookmark: %v", response.Body.String(), err)
	}
	return got
}

// asRequest is a bookmark of the Go course as a page sends it to put it back.
func asRequest(b *mark.Bookmark) map[string]any {
	return map[string]any{
		"course": goCourse, "path": b.RelPath, "anchor": b.Anchor, "offset": b.Offset, "identity": b.Identity, "at": b.At,
	}
}

func TestBookmarkRouteSetsMovesAndPutsBackTheBookmarkOfACourse(t *testing.T) {
	t.Parallel()

	srv := newServer(t, theSchool())
	store, routes := srv.store, srv.handler

	first := putBookmark(t, routes, aBookmarkBody(goCourse, "Notes/a.md"))
	if got, want := first.Header().Get("Cache-Control"), "no-store"; got != want {
		t.Errorf("Cache-Control = %q, want %q", got, want)
	}
	if !strings.Contains(first.Body.String(), `"previous":null`) {
		t.Errorf("setting the first bookmark of a course answered %q, want previous null", first.Body.String())
	}
	set := decodeMoved(t, first)
	if want := (mark.Bookmark{RelPath: "Notes/a.md", Anchor: "the-topic-particle", Offset: 420, Identity: anIdentity}); set.Bookmark.RelPath != want.RelPath ||
		set.Bookmark.Anchor != want.Anchor || set.Bookmark.Offset != want.Offset || set.Bookmark.Identity != want.Identity {
		t.Errorf("the bookmark answered = %+v, want the one sent", set.Bookmark)
	}
	if set.Previous != nil || time.Since(set.Bookmark.At) > time.Minute || time.Until(set.Bookmark.At) > time.Minute {
		t.Errorf("previous = %v, at = %v; want none, and the server's clock", set.Previous, set.Bookmark.At)
	}
	if kept, found, err := store.Bookmark(goCourse); err != nil || !found || kept != set.Bookmark {
		t.Errorf("the course's bookmark = %+v, %v, %v; want the answered one", kept, found, err)
	}
	if _, found, err := store.Bookmark(mark.NoCourse); err != nil || found {
		t.Errorf("a course's bookmark became the one for no course: %v, %v", found, err)
	}

	second := decodeMoved(t, putBookmark(t, routes, aBookmarkBody(goCourse, "Notes/b.md")))
	if second.Previous == nil || *second.Previous != set.Bookmark {
		t.Fatalf("the move answered previous = %+v, want the first bookmark", second.Previous)
	}
	// Setting the previous one back, with the time it had, sets it as it was.
	if back := decodeMoved(t, putBookmark(t, routes, asRequest(second.Previous))); back.Bookmark != set.Bookmark {
		t.Errorf("the bookmark put back = %+v, want %+v", back.Bookmark, set.Bookmark)
	}
}

func TestBookmarkRouteKeepsOneForEachCourseAndOneForNone(t *testing.T) {
	t.Parallel()

	srv := newServer(t, theSchool())
	store, routes := srv.store, srv.handler
	for _, set := range []struct{ course, path string }{
		{goCourse, "Notes/a.md"},
		{rustCourse, "Notes/other.md"},
		{mark.NoCourse, "Notes/loose.md"},
		{goCourse, "Notes/side.md"}, // a side branch is a unit the course walks
	} {
		if got := putBookmark(t, routes, aBookmarkBody(set.course, set.path)); got.Code != http.StatusOK {
			t.Fatalf("a bookmark of %q in %q = %d %q", set.path, set.course, got.Code, got.Body.String())
		}
	}
	for course, want := range map[string]string{goCourse: "Notes/side.md", rustCourse: "Notes/other.md", mark.NoCourse: "Notes/loose.md"} {
		if kept, found, err := store.Bookmark(course); err != nil || !found || kept.RelPath != want {
			t.Errorf("the bookmark of %q = %q, %v, %v; want %q", course, kept.RelPath, found, err, want)
		}
	}
}

func TestBookmarkRouteSaysWhereACourseIsKeptAndClearsItAnswersWhatItCleared(t *testing.T) {
	t.Parallel()

	srv := newServer(t, theSchool())
	store, routes := srv.store, srv.handler
	if got := send(t, routes, http.MethodGet, goBookmark, jsonBody, ""); got.Code != http.StatusNoContent {
		t.Fatalf("GET before any bookmark = %d, want 204", got.Code)
	}
	if got := send(t, routes, http.MethodDelete, goBookmark, jsonBody, ""); got.Code != http.StatusNotFound {
		t.Fatalf("DELETE where none is kept = %d, want 404", got.Code)
	}
	if contents(t, progressPath(store)) != "" {
		t.Error("asking after a bookmark that is not there made a file")
	}

	set := decodeMoved(t, putBookmark(t, routes, aBookmarkBody(goCourse, "Notes/a.md"))).Bookmark
	if got := send(t, routes, http.MethodGet, mark.Paths.Bookmark, jsonBody, ""); got.Code != http.StatusNoContent {
		t.Errorf("GET for no course while only a course has one = %d, want 204", got.Code)
	}
	said := send(t, routes, http.MethodGet, goBookmark, jsonBody, "")
	if got, want := saidPlace(t, said.Body.Bytes()), (aSaidPlace{Path: "Notes/a.md", Anchor: "the-topic-particle", Offset: 420}); !cmp.Equal(got, want, ignoreTime) {
		t.Errorf("GET = %+v, want %+v", got, want)
	}

	cleared := send(t, routes, http.MethodDelete, goBookmark, jsonBody, "")
	var removed mark.Bookmark
	if cleared.Code != http.StatusOK || json.Unmarshal(cleared.Body.Bytes(), &removed) != nil || removed != set {
		t.Fatalf("DELETE = %d %q, want 200 and the bookmark that was kept", cleared.Code, cleared.Body.String())
	}
	if got := send(t, routes, http.MethodDelete, goBookmark, jsonBody, ""); got.Code != http.StatusNotFound {
		t.Errorf("clearing twice = %d, want 404", got.Code)
	}
	if got := send(t, routes, http.MethodGet, goBookmark, jsonBody, ""); got.Code != http.StatusNoContent {
		t.Errorf("GET after clearing = %d, want 204", got.Code)
	}
}

// A refusal changes nothing, and a request that is refused before it is read
// is answered by a status that says which of the rules it broke.
func TestBookmarkRoutesRefuseWhatTheyWillNotKeepAndWriteNothing(t *testing.T) {
	t.Parallel()

	body := func(change map[string]any) string {
		fields := aBookmarkBody(goCourse, "Notes/a.md")
		for key, value := range change {
			if value == nil {
				delete(fields, key)
				continue
			}
			fields[key] = value
		}
		return asJSON(t, fields)
	}
	for name, tt := range map[string]struct {
		contentType, body string
		want              int
	}{
		"a course that does not walk the note":   {"application/json", body(map[string]any{"path": "Notes/other.md"}), http.StatusUnprocessableEntity},
		"a note no course walks, in a course":    {"application/json", body(map[string]any{"path": "Notes/loose.md"}), http.StatusUnprocessableEntity},
		"a course nothing answers to":            {"application/json", body(map[string]any{"course": "Courses/nope.md"}), http.StatusUnprocessableEntity},
		"a course that climbs out of the vault":  {"application/json", body(map[string]any{"course": "../escape.md"}), http.StatusUnprocessableEntity},
		"a note the vault does not hold":         {"application/json", body(map[string]any{"path": "Notes/gone.md"}), http.StatusUnprocessableEntity},
		"a time a day ahead":                     {"application/json", body(map[string]any{"at": time.Now().Add(24 * time.Hour)}), http.StatusUnprocessableEntity},
		"a time that is no time":                 {"application/json", body(map[string]any{"at": time.Time{}}), http.StatusUnprocessableEntity},
		"a time that is not a time":              {"application/json", body(map[string]any{"at": "yesterday"}), http.StatusBadRequest},
		"a field nobody knows":                   {"application/json", body(map[string]any{"colour": "blue"}), http.StatusBadRequest},
		"two values":                             {"application/json", body(nil) + body(nil), http.StatusBadRequest},
		"a body that is not JSON":                {"application/json", "course=" + goCourse, http.StatusBadRequest},
		"a form":                                 {"application/x-www-form-urlencoded", "path=Notes/a.md&offset=1", http.StatusUnsupportedMediaType},
		"text":                                   {"text/plain", body(nil), http.StatusUnsupportedMediaType},
		"no content type":                        {"", body(nil), http.StatusUnsupportedMediaType},
		"a body past what a bookmark ever needs": {"application/json", body(map[string]any{"anchor": strings.Repeat("a", 5000)}), http.StatusRequestEntityTooLarge},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			srv := newServer(t, theSchool())
			store, routes := srv.store, srv.handler
			decodeMoved(t, putBookmark(t, routes, aBookmarkBody(rustCourse, "Notes/other.md")))
			before := contents(t, progressPath(store))

			got := send(t, routes, http.MethodPut, mark.Paths.Bookmark, tt.contentType, tt.body)
			if got.Code != tt.want {
				t.Errorf("PUT = %d %q, want %d", got.Code, got.Body.String(), tt.want)
			}
			if after := contents(t, progressPath(store)); after != before {
				t.Errorf("a refusal changed the file:\nbefore %s\nafter  %s", before, after)
			}
		})
	}
}

func TestBookmarkRoutesRefuseACourseThatIsMalformedOrNamedTwiceWithoutReadingAnything(t *testing.T) {
	t.Parallel()

	srv := newServer(t, theSchool())
	store, routes := srv.store, srv.handler
	decodeMoved(t, putBookmark(t, routes, aBookmarkBody(goCourse, "Notes/a.md")))
	before := contents(t, progressPath(store))
	for _, tt := range []struct{ method, target string }{
		{http.MethodGet, mark.Paths.Bookmark + "?course=../escape.md"},
		{http.MethodGet, mark.Paths.Bookmark + "?course=" + goCourse + "&course=" + rustCourse},
		{http.MethodDelete, mark.Paths.Bookmark + "?course=../escape.md"},
		{http.MethodDelete, mark.Paths.Bookmark + "?course=" + goCourse + "&course=" + rustCourse},
	} {
		if got := send(t, routes, tt.method, tt.target, jsonBody, ""); got.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s %s = %d, want 422", tt.method, tt.target, got.Code)
		}
	}
	if after := contents(t, progressPath(store)); after != before {
		t.Errorf("a refusal changed the file:\nbefore %s\nafter  %s", before, after)
	}
}

// finishedAnswer is what finishing or reopening a note answers.
type finishedAnswer struct {
	Path     string     `json:"path"`
	Finished bool       `json:"finished"`
	At       *time.Time `json:"at"`
	Course   *struct {
		Path     string `json:"path"`
		Units    int    `json:"units"`
		Finished int    `json:"finished"`
	} `json:"course"`
}

func finishing(t *testing.T, handler http.Handler, method, note, query string) (finishedAnswer, *httptest.ResponseRecorder) {
	t.Helper()
	target := mark.Paths.Finished + "/" + note
	if query != "" {
		target += "?" + query
	}
	response := send(t, handler, method, target, jsonBody, "")
	var got finishedAnswer
	if response.Code == http.StatusOK {
		if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
			t.Fatalf("the answer %q is not a finished note: %v", response.Body.String(), err)
		}
	}
	return got, response
}

func TestFinishedRoutesSayANoteIsFinishedAndKeepTheFirstTime(t *testing.T) {
	t.Parallel()

	srv := newServer(t, theSchool())
	store, routes := srv.store, srv.handler
	first, response := finishing(t, routes, http.MethodPut, "Notes/a.md", "")
	if response.Code != http.StatusOK || !first.Finished || first.Path != "Notes/a.md" || first.At == nil || time.Since(*first.At) > time.Minute {
		t.Fatalf("PUT = %d %q, want finished at about now", response.Code, response.Body.String())
	}
	if got, want := response.Header().Get("Cache-Control"), "no-store"; got != want {
		t.Errorf("Cache-Control = %q, want %q", got, want)
	}
	if strings.Contains(response.Body.String(), `"course"`) {
		t.Errorf("an answer to a request that named no course holds a course: %s", response.Body.String())
	}
	if at, finished, err := store.FinishedAt("Notes/a.md"); err != nil || !finished || !at.Equal(*first.At) {
		t.Errorf("the store says finished = %v at %v, %v; want the time answered", finished, at, err)
	}
	before := contents(t, progressPath(store))

	again, _ := finishing(t, routes, http.MethodPut, "Notes/a.md", "")
	if again.At == nil || !again.At.Equal(*first.At) {
		t.Errorf("finishing a finished note answered at %v, want the first time %v", again.At, first.At)
	}
	if got := contents(t, progressPath(store)); got != before {
		t.Errorf("saying again what is so changed the file:\nbefore %s\nafter  %s", before, got)
	}

	reopened, response := finishing(t, routes, http.MethodDelete, "Notes/a.md", "")
	if response.Code != http.StatusOK || reopened.Finished || reopened.At != nil || reopened.Path != "Notes/a.md" {
		t.Fatalf("DELETE = %d %q, want not finished and no time", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), `"at"`) {
		t.Errorf("a note that is not finished answered a time: %s", response.Body.String())
	}
	if _, finished, err := store.FinishedAt("Notes/a.md"); err != nil || finished {
		t.Errorf("after reopening, finished = %v, %v", finished, err)
	}
	again, response = finishing(t, routes, http.MethodDelete, "Notes/a.md", "")
	if response.Code != http.StatusOK || again.Finished {
		t.Errorf("reopening a note that is not finished = %d %q, want the same answer", response.Code, response.Body.String())
	}
}

// A note that is deleted or renamed can still be said not to be finished, in
// the course it was read in or out of any: the entry the reader made is theirs
// to take back, and the vault is not asked.
func TestAFinishedNoteWhoseFileIsGoneCanStillBeReopened(t *testing.T) {
	t.Parallel()

	for name, query := range map[string]string{
		"read outside any course": "",
		"read in a course":        "course=" + goCourse,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			school := theSchool()
			srv := newServer(t, school)
			store, mux := srv.store, srv.handler
			if _, response := finishing(t, mux, http.MethodPut, "Notes/a.md", query); response.Code != http.StatusOK {
				t.Fatalf("PUT = %d %q, want 200", response.Code, response.Body.String())
			}
			// The file is deleted, and the course no longer lists it.
			delete(school.notes, "Notes/a.md")
			course := school.courses[goCourse]
			course.rows = slices.DeleteFunc(slices.Clone(course.rows), func(row string) bool { return row == "Notes/a.md" })
			school.courses[goCourse] = course

			reopened, response := finishing(t, mux, http.MethodDelete, "Notes/a.md", query)
			if response.Code != http.StatusOK || reopened.Finished || reopened.Path != "Notes/a.md" {
				t.Fatalf("DELETE = %d %q, want 200 and not finished", response.Code, response.Body.String())
			}
			if _, finished, err := store.FinishedAt("Notes/a.md"); err != nil || finished {
				t.Errorf("after reopening, finished = %v, %v; want the entry gone", finished, err)
			}
			if strings.Contains(contents(t, progressPath(store)), "Notes/a.md") {
				t.Errorf("the file still holds the note: %s", contents(t, progressPath(store)))
			}
		})
	}
}

// The count a course answers is of the rows on its main line, those not written
// yet included, and of the rows the reader has finished: a note listed twice is
// two rows, a note on a side branch is not one, and a note of another course is
// not either.
func TestFinishedRoutesAnswerTheCountsOfTheCourseTheRequestNames(t *testing.T) {
	t.Parallel()

	srv := newServer(t, theSchool())
	_, routes := srv.store, srv.handler
	query := "course=" + url.QueryEscape(goCourse)
	for _, step := range []struct {
		method, note    string
		units, finished int
	}{
		{http.MethodPut, "Notes/a.md", 5, 1},
		{http.MethodPut, "Notes/b.md", 5, 3},
		{http.MethodPut, "Notes/side.md", 5, 3},
		{http.MethodDelete, "Notes/a.md", 5, 2},
		{http.MethodDelete, "Notes/side.md", 5, 2},
	} {
		got, response := finishing(t, routes, step.method, step.note, query)
		if response.Code != http.StatusOK || got.Course == nil {
			t.Fatalf("%s %s = %d %q, want 200 and a course", step.method, step.note, response.Code, response.Body.String())
		}
		if got.Course.Path != goCourse || got.Course.Units != step.units || got.Course.Finished != step.finished {
			t.Errorf("after %s %s the course = %+v, want %d units and %d finished", step.method, step.note, *got.Course, step.units, step.finished)
		}
	}
	// A note finished in one course is finished in the other that walks it,
	// and the other course counts what is on its own main line.
	got, _ := finishing(t, routes, http.MethodPut, "Notes/other.md", "course="+url.QueryEscape(rustCourse))
	if got.Course == nil || got.Course.Units != 1 || got.Course.Finished != 1 {
		t.Errorf("the other course = %+v, want one unit and one finished", got.Course)
	}
}

// Saying a note is not finished asks only for the shape of what is named, so
// what is refused for PUT because the vault or the course does not answer to it
// is answered for DELETE, and writes nothing.
func TestFinishedRoutesRefuseWhatTheyWillNotSayAndWriteNothing(t *testing.T) {
	t.Parallel()

	for name, tt := range map[string]struct {
		note, query string
		put, del    int
	}{
		"a note the course does not walk":     {"Notes/other.md", "course=" + goCourse, http.StatusUnprocessableEntity, http.StatusOK},
		"a note no course walks, in a course": {"Notes/loose.md", "course=" + goCourse, http.StatusUnprocessableEntity, http.StatusOK},
		"a course nothing answers to":         {"Notes/a.md", "course=Courses/nope.md", http.StatusUnprocessableEntity, http.StatusOK},
		"a course that climbs out":            {"Notes/a.md", "course=../escape.md", http.StatusUnprocessableEntity, http.StatusUnprocessableEntity},
		"two courses":                         {"Notes/a.md", "course=" + goCourse + "&course=" + rustCourse, http.StatusUnprocessableEntity, http.StatusUnprocessableEntity},
		"a note the vault does not hold":      {"Notes/gone.md", "", http.StatusNotFound, http.StatusOK},
		"a path with a line break in it":      {"Notes/a%0A.md", "", http.StatusUnprocessableEntity, http.StatusUnprocessableEntity},
	} {
		for method, want := range map[string]int{http.MethodPut: tt.put, http.MethodDelete: tt.del} {
			t.Run(name+" "+method, func(t *testing.T) {
				t.Parallel()
				srv := newServer(t, theSchool())
				store, routes := srv.store, srv.handler
				before := contents(t, progressPath(store))
				_, response := finishing(t, routes, method, tt.note, tt.query)
				if response.Code != want {
					t.Errorf("%s = %d %q, want %d", method, response.Code, response.Body.String(), want)
				}
				if after := contents(t, progressPath(store)); after != before {
					t.Errorf("a refusal changed the file:\nbefore %q\nafter  %q", before, after)
				}
			})
		}
	}
}

// A file the reader damaged is theirs to recover: every route that would have
// replaced it says it was not saved, and leaves it as it was.
func TestProgressRoutesLeaveADamagedFileAsItIsAndSayItWasNotSaved(t *testing.T) {
	t.Parallel()

	srv := newServer(t, theSchool())
	store, routes := srv.store, srv.handler
	const damaged = `{"version":1,"bookmarks":[`
	writeFile(t, progressPath(store), damaged)

	for _, tt := range []struct {
		name   string
		answer *httptest.ResponseRecorder
	}{
		{"setting a bookmark", putBookmark(t, routes, aBookmarkBody(goCourse, "Notes/a.md"))},
		{"clearing a bookmark", send(t, routes, http.MethodDelete, goBookmark, jsonBody, "")},
		{"finishing a note", mustFinish(t, routes, http.MethodPut)},
		{"reopening a note", mustFinish(t, routes, http.MethodDelete)},
	} {
		if tt.answer.Code != http.StatusInternalServerError {
			t.Errorf("%s over a damaged file = %d %q, want 500", tt.name, tt.answer.Code, tt.answer.Body.String())
		}
	}
	if got := send(t, routes, http.MethodGet, goBookmark, jsonBody, ""); got.Code != http.StatusNoContent {
		t.Errorf("GET over a damaged file = %d, want 204 (there is nowhere to continue from)", got.Code)
	}
	if got := contents(t, progressPath(store)); got != damaged {
		t.Errorf("the damaged file was changed to %q", got)
	}
}

func mustFinish(t *testing.T, routes http.Handler, method string) *httptest.ResponseRecorder {
	t.Helper()
	_, response := finishing(t, routes, method, "Notes/a.md", "")
	return response
}

// Promise 1: the routes write progress under the configuration directory and
// nothing into the folder being read.
func TestProgressRoutesWriteNothingIntoTheFolderBeingRead(t *testing.T) {
	t.Parallel()

	srv := newServer(t, theSchool())
	vaultRoot, configDir, store, mux := srv.root, srv.config, srv.store, srv.handler
	writeFile(t, filepath.Join(vaultRoot, "Notes", "a.md"), "a")
	before := tree(t, vaultRoot)

	first := decodeMoved(t, putBookmark(t, mux, aBookmarkBody(goCourse, "Notes/a.md"))).Bookmark
	putBookmark(t, mux, aBookmarkBody(goCourse, "Notes/b.md"))
	send(t, mux, http.MethodDelete, goBookmark, jsonBody, "")
	putBookmark(t, mux, asRequest(&first))
	finishing(t, mux, http.MethodPut, "Notes/a.md", "course="+url.QueryEscape(goCourse))
	finishing(t, mux, http.MethodDelete, "Notes/a.md", "")
	finishing(t, mux, http.MethodPut, "Notes/b.md", "")

	if diff := cmp.Diff(before, tree(t, vaultRoot)); diff != "" {
		t.Errorf("the routes changed the folder being read (-before +after):\n%s", diff)
	}
	if got := contents(t, progressPath(store)); got == "" || !strings.HasPrefix(progressPath(store), configDir) {
		t.Errorf("progress was kept at %s holding %q, want a file under %s", progressPath(store), got, configDir)
	}
}

// undoing is a bookmark of the Go course as a page sends it to put it back,
// saying which bookmark it expects to replace: the one set at a time, or none
// for nil.
func undoing(b *mark.Bookmark, replaces any) map[string]any {
	request := asRequest(b)
	request["replaces"] = replaces
	return request
}

// A page that undoes a move or a clear must not overwrite what another page did
// since: the undo says what it expects to replace and is refused with 409 where
// that is not what is kept.
func TestAnUndoThatFindsAnotherBookmarkKeptIsRefusedAndLosesNoOtherPagesMove(t *testing.T) {
	t.Parallel()

	t.Run("a move undone after another page moved it again", func(t *testing.T) {
		t.Parallel()
		srv := newServer(t, theSchool())
		store, routes := srv.store, srv.handler
		first := decodeMoved(t, putBookmark(t, routes, aBookmarkBody(goCourse, "Notes/a.md"))).Bookmark
		// Page A moves a to b; page B, which saw b, moves b to c. A reader's two
		// acts are more than a millisecond apart, which is how an undo tells them
		// apart, so the test waits as a reader would.
		movedByA := decodeMoved(t, putBookmark(t, routes, aBookmarkBody(goCourse, "Notes/b.md")))
		time.Sleep(2 * time.Millisecond)
		decodeMoved(t, putBookmark(t, routes, aBookmarkBody(goCourse, "Notes/c.md")))
		afterB := contents(t, progressPath(store))

		// Page A undoes: it expects to replace its own b.
		undone := putBookmark(t, routes, undoing(movedByA.Previous, movedByA.Bookmark.At))
		if undone.Code != http.StatusConflict {
			t.Fatalf("an undo over another page's move = %d %q, want 409", undone.Code, undone.Body.String())
		}
		if got := contents(t, progressPath(store)); got != afterB {
			t.Errorf("the refused undo changed the file:\nbefore %s\nafter  %s", afterB, got)
		}
		if first.RelPath != "Notes/a.md" {
			t.Fatal("the first bookmark is not the one the test set")
		}
	})

	t.Run("a clear undone after another page set one", func(t *testing.T) {
		t.Parallel()
		srv := newServer(t, theSchool())
		store, routes := srv.store, srv.handler
		decodeMoved(t, putBookmark(t, routes, aBookmarkBody(goCourse, "Notes/a.md")))
		var cleared mark.Bookmark
		if got := send(t, routes, http.MethodDelete, goBookmark, jsonBody, ""); got.Code != http.StatusOK || json.Unmarshal(got.Body.Bytes(), &cleared) != nil {
			t.Fatalf("DELETE = %d %q", got.Code, got.Body.String())
		}
		decodeMoved(t, putBookmark(t, routes, aBookmarkBody(goCourse, "Notes/c.md")))
		afterB := contents(t, progressPath(store))

		// Page A undoes its clear: it expects the course to have none.
		undone := putBookmark(t, routes, undoing(&cleared, nil))
		if undone.Code != http.StatusConflict {
			t.Fatalf("an undo of a clear over another page's bookmark = %d %q, want 409", undone.Code, undone.Body.String())
		}
		if got := contents(t, progressPath(store)); got != afterB {
			t.Errorf("the refused undo changed the file:\nbefore %s\nafter  %s", afterB, got)
		}
	})

	t.Run("an undo that finds what it expected is taken", func(t *testing.T) {
		t.Parallel()
		srv := newServer(t, theSchool())
		store, routes := srv.store, srv.handler
		set := decodeMoved(t, putBookmark(t, routes, aBookmarkBody(goCourse, "Notes/a.md"))).Bookmark
		afterSet := contents(t, progressPath(store))
		time.Sleep(2 * time.Millisecond)
		moved := decodeMoved(t, putBookmark(t, routes, aBookmarkBody(goCourse, "Notes/b.md")))
		back := putBookmark(t, routes, undoing(moved.Previous, moved.Bookmark.At))
		if back.Code != http.StatusOK || contents(t, progressPath(store)) != afterSet {
			t.Errorf("an undo of the last move = %d %q, want it taken and the file as it was", back.Code, back.Body.String())
		}

		var cleared mark.Bookmark
		if got := send(t, routes, http.MethodDelete, goBookmark, jsonBody, ""); json.Unmarshal(got.Body.Bytes(), &cleared) != nil || cleared != set {
			t.Fatalf("DELETE = %d %q", got.Code, got.Body.String())
		}
		again := putBookmark(t, routes, undoing(&cleared, nil))
		if again.Code != http.StatusOK || contents(t, progressPath(store)) != afterSet {
			t.Errorf("an undo of a clear = %d %q, want it taken and the file as it was", again.Code, again.Body.String())
		}
	})
}

// replaces is a bookmark's time, or null; a request that leaves it out expects
// nothing in particular, and one that holds something else is not read.
func TestTheWireFormOfWhatAnUndoExpects(t *testing.T) {
	t.Parallel()

	for name, tt := range map[string]struct {
		fields string
		want   int
	}{
		"left out, which expects nothing": {``, http.StatusOK},
		"null, which expects none":        {`,"replaces":null`, http.StatusConflict},
		"a time that is not the one kept": {`,"replaces":"2001-01-01T00:00:00Z"`, http.StatusConflict},
		"something that is no time":       {`,"replaces":"yesterday"`, http.StatusBadRequest},
		"a number":                        {`,"replaces":5`, http.StatusBadRequest},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			srv := newServer(t, theSchool())
			store, routes := srv.store, srv.handler
			decodeMoved(t, putBookmark(t, routes, aBookmarkBody(goCourse, "Notes/a.md")))
			before := contents(t, progressPath(store))
			body := `{"course":"` + goCourse + `","path":"Notes/b.md","anchor":"","offset":0,"identity":"` + anIdentity + `"` + tt.fields + `}`
			got := send(t, routes, http.MethodPut, mark.Paths.Bookmark, jsonBody, body)
			if got.Code != tt.want {
				t.Errorf("PUT = %d %q, want %d", got.Code, got.Body.String(), tt.want)
			}
			if tt.want != http.StatusOK && contents(t, progressPath(store)) != before {
				t.Error("a refusal changed the file")
			}
		})
	}
}

// The time a bookmark is set at is the server's, in UTC and to the millisecond,
// so a page that sends it round again through a Date puts back the time that was
// kept.
func TestABookmarkIsStampedInUTCToTheMillisecond(t *testing.T) {
	t.Parallel()

	srv := newServer(t, theSchool())
	store, routes := srv.store, srv.handler
	set := decodeMoved(t, putBookmark(t, routes, aBookmarkBody(goCourse, "Notes/a.md")))
	if at := set.Bookmark.At; at.Location() != time.UTC || !at.Equal(at.Truncate(time.Millisecond)) {
		t.Errorf("the bookmark was set at %s, want UTC and to the millisecond", at.Format(time.RFC3339Nano))
	}
	if !regexp.MustCompile(`"at": "[^".]*(\.\d{1,3})?Z"`).MatchString(contents(t, progressPath(store))) {
		t.Errorf("the file holds %s, want a UTC time with at most three decimals", contents(t, progressPath(store)))
	}
	finished, _ := finishing(t, routes, http.MethodPut, "Notes/a.md", "")
	if finished.At == nil || finished.At.Location() != time.UTC || !finished.At.Equal(finished.At.Truncate(time.Millisecond)) {
		t.Errorf("the note was finished at %v, want UTC and to the millisecond", finished.At)
	}
}

// A bookmark put back with a time that is not in UTC, or that has finer than
// milliseconds, is kept as the server stamps one: the file holds the same
// instant in UTC to the millisecond.
func TestABookmarkPutBackIsKeptInUTCToTheMillisecond(t *testing.T) {
	t.Parallel()

	srv := newServer(t, theSchool())
	store, routes := srv.store, srv.handler
	zone := time.FixedZone("UTC+9", 9*60*60)
	at := time.Now().Add(-time.Hour).In(zone).Truncate(time.Second).Add(123456 * time.Microsecond)
	body := aBookmarkBody(goCourse, "Notes/a.md")
	body["at"] = at.Format(time.RFC3339Nano)
	set := decodeMoved(t, putBookmark(t, routes, body))
	want := at.UTC().Truncate(time.Millisecond)
	if set.Bookmark.At.Location() != time.UTC || !set.Bookmark.At.Equal(want) {
		t.Errorf("the bookmark answered %s, want %s", set.Bookmark.At.Format(time.RFC3339Nano), want.Format(time.RFC3339Nano))
	}
	if held := contents(t, progressPath(store)); !strings.Contains(held, `"at": "`+want.Format("2006-01-02T15:04:05.000Z07:00")+`"`) || strings.Contains(held, "+09:00") || strings.Contains(held, ".123456") {
		t.Errorf("the file holds %s, want the time as %s", held, want.Format(time.RFC3339Nano))
	}
}
