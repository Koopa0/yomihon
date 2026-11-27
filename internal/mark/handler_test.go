package mark_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/koopa0/yomihon/internal/mark"
	"github.com/koopa0/yomihon/internal/mark/confdir"
	"github.com/koopa0/yomihon/internal/mark/machinemarks"
	"github.com/koopa0/yomihon/internal/mark/marktest"
	"github.com/koopa0/yomihon/internal/wording"
)

func TestTheRouteKeepsWhatAReadingPageSends(t *testing.T) {
	t.Parallel()

	srv := newServer(t, nil)
	file, handler := srv.store, srv.handler
	response := postPlace(t, handler, url.Values{
		"path":     {"Notes/alpha.md"},
		"anchor":   {"a-heading"},
		"offset":   {"320"},
		"identity": {anIdentity},
	})
	if response.Code != http.StatusNoContent {
		t.Errorf("POST %s = %d, want %d", mark.BookmarkPath, response.Code, http.StatusNoContent)
	}
	got, ok := bookmarkFor(t, file, mark.NoCourse)
	if !ok {
		t.Fatal("the route answered but kept no place")
	}
	if got.RelPath != "Notes/alpha.md" || got.Anchor != "a-heading" || got.Offset != 320 {
		t.Errorf("the route kept %+v, want the place that was posted", got)
	}
	if got.At.IsZero() {
		t.Error("the kept place records no time")
	}
}

func TestTheRouteRefusesWhatItCannotKeep(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		form url.Values
		want int
	}{
		{"a path that climbs out", url.Values{"path": {"../x.md"}, "offset": {"0"}, "identity": {anIdentity}}, http.StatusUnprocessableEntity},
		{"an offset that is not a number", url.Values{"path": {"a.md"}, "offset": {"soon"}, "identity": {anIdentity}}, http.StatusUnprocessableEntity},
		{"no offset at all", url.Values{"path": {"a.md"}, "identity": {anIdentity}}, http.StatusUnprocessableEntity},
		{"no identity", url.Values{"path": {"a.md"}, "offset": {"0"}}, http.StatusUnprocessableEntity},
		{"nothing at all", url.Values{}, http.StatusUnprocessableEntity},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := newServer(t, nil)
			file, handler := srv.store, srv.handler
			response := postPlace(t, handler, tt.form)
			if response.Code != tt.want {
				t.Errorf("POST %s (%s) = %d, want %d", mark.BookmarkPath, tt.name, response.Code, tt.want)
			}
			if _, ok := bookmarkFor(t, file, mark.NoCourse); ok {
				t.Error("a refused submission kept a place anyway")
			}
		})
	}
}

// TestTheRouteRefusesABodyBeyondItsCap holds the cap the threat model states
// in public. A place is four short fields; a body past the cap is not one, and
// the reader of that document is owed the number being true.
//
// The padding rides in a field validate never looks at, which is the whole
// point: an oversized anchor is refused at 256 bytes by the shape check long
// before the body is measured, so a test built on one passed with the cap
// deleted from the handler. Both sides are asserted — over the cap and just
// under it — because a cap that refuses everything is not a cap either.
func TestTheRouteRefusesABodyBeyondItsCap(t *testing.T) {
	t.Parallel()

	// A field the handler reads nothing from, so what decides each case is the
	// size of the body and nothing else.
	const padField = "pad"

	tests := []struct {
		name    string
		padding int
		want    int
	}{
		{"past the cap", 8192, http.StatusBadRequest},
		{"inside the cap", 3000, http.StatusNoContent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := newServer(t, nil)
			file, handler := srv.store, srv.handler
			form := url.Values{
				"path":     {"Notes/alpha.md"},
				"anchor":   {"a-heading"},
				"offset":   {"0"},
				"identity": {anIdentity},
				padField:   {strings.Repeat("a", tt.padding)},
			}
			// The case only means what it says if the body really is on the
			// side of the cap the name claims.
			size := len(form.Encode())
			if tt.want == http.StatusBadRequest && size <= 4096 {
				t.Fatalf("the %q body is %d bytes, which is not past the 4096-byte cap", tt.name, size)
			}
			if tt.want == http.StatusNoContent && size > 4096 {
				t.Fatalf("the %q body is %d bytes, which is already past the 4096-byte cap", tt.name, size)
			}
			response := postPlace(t, handler, form)
			if response.Code != tt.want {
				t.Errorf("POST %s with a %d-byte body = %d, want %d", mark.BookmarkPath, size, response.Code, tt.want)
			}
			_, kept := bookmarkFor(t, file, mark.NoCourse)
			if wantKept := tt.want == http.StatusNoContent; kept != wantKept {
				t.Errorf("a %d-byte body kept a place = %t, want %t", size, kept, wantKept)
			}
		})
	}
}

// aSaidPlace is what the route says of the place kept now.
type aSaidPlace struct {
	Path   string    `json:"path"`
	Anchor string    `json:"anchor"`
	Offset int       `json:"offset"`
	At     time.Time `json:"at"`
}

var ignoreTime = cmpopts.IgnoreFields(aSaidPlace{}, "At")

// saidPlace reads the route's answer as exactly the four fields it names. The
// identity of the bytes the reader was looking at is not among them, and a
// field the page does not know is an error.
func saidPlace(t *testing.T, body []byte) aSaidPlace {
	t.Helper()
	var said aSaidPlace
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&said); err != nil {
		t.Fatalf("the answer %q is not the place: %v", body, err)
	}
	if time.Since(said.At) > time.Minute || time.Until(said.At) > time.Minute {
		t.Errorf("the place was said to be kept at %s, which is not now", said.At)
	}
	return said
}

// TestTheRouteSaysWhereThePlaceIsKept holds what a page shown again from the
// browser's cache is given to draw from: the note, the anchor, how far below it
// and when, of the place kept now, and nothing the page has to be trusted with
// beyond them. The answer must not be one the browser keeps, or the page would
// be redrawn from the staleness this reads past.
func TestTheRouteSaysWhereThePlaceIsKept(t *testing.T) {
	t.Parallel()

	srv := newServer(t, nil)
	_, handler := srv.store, srv.handler
	if posted := postPlace(t, handler, aPost("Notes/alpha.md", "first-heading")); posted.Code != http.StatusNoContent {
		t.Fatalf("POST %s = %d %q, want %d", mark.BookmarkPath, posted.Code, posted.Body.String(), http.StatusNoContent)
	}
	response := send(t, handler, http.MethodGet, mark.BookmarkPath, "", "")
	if response.Code != http.StatusOK {
		t.Fatalf("GET %s = %d, want %d", mark.BookmarkPath, response.Code, http.StatusOK)
	}
	if got, want := response.Header().Get("Cache-Control"), "no-store"; got != want {
		t.Errorf("GET %s Cache-Control = %q, want %q", mark.BookmarkPath, got, want)
	}
	if got, want := response.Header().Get("Content-Type"), "application/json; charset=utf-8"; got != want {
		t.Errorf("GET %s Content-Type = %q, want %q", mark.BookmarkPath, got, want)
	}
	if diff := cmp.Diff(aSaidPlace{Path: "Notes/alpha.md", Anchor: "first-heading", Offset: 1}, saidPlace(t, response.Body.Bytes()), ignoreTime); diff != "" {
		t.Errorf("GET %s body (-want +got):\n%s", mark.BookmarkPath, diff)
	}

	// Keeping another place replaces the first, and the answer follows.
	if posted := postPlace(t, handler, aPost("Notes/beta.md", "")); posted.Code != http.StatusNoContent {
		t.Fatalf("POST %s = %d %q, want %d", mark.BookmarkPath, posted.Code, posted.Body.String(), http.StatusNoContent)
	}
	again := send(t, handler, http.MethodGet, mark.BookmarkPath, "", "")
	if diff := cmp.Diff(aSaidPlace{Path: "Notes/beta.md", Offset: 1}, saidPlace(t, again.Body.Bytes()), ignoreTime); diff != "" {
		t.Errorf("GET %s after keeping another place (-want +got):\n%s", mark.BookmarkPath, diff)
	}
}

// TestTheRouteSaysNothingWhereNoPlaceIsKept holds the other answer: no content,
// not an empty note name, so a page cannot take silence for a place. A file the
// reader damaged answers the same, is reported to the log, and the read changes
// nothing.
func TestTheRouteSaysNothingWhereNoPlaceIsKept(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body *string
	}{
		{"no file at all", nil},
		{"a file that is not JSON", new("this is not a file yomihon wrote\n")},
		{"a place the shape refuses", new(`{"version":1,"vault":"/v","bookmarks":[{"bookmark":{"path":"../outside.md","offset":0,"identity":"` + anIdentity + `","at":"2026-09-17T10:00:00Z"}}]}`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := newServer(t, nil)
			file, handler := srv.store, srv.handler
			if tt.body != nil {
				if err := os.MkdirAll(filepath.Dir(progressPath(file)), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(progressPath(file), []byte(*tt.body), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			response := send(t, handler, http.MethodGet, mark.BookmarkPath, "", "")
			if response.Code != http.StatusNoContent {
				t.Errorf("GET %s (%s) = %d, want %d", mark.BookmarkPath, tt.name, response.Code, http.StatusNoContent)
			}
			if got, want := response.Header().Get("Cache-Control"), "no-store"; got != want {
				t.Errorf("GET %s (%s) Cache-Control = %q, want %q", mark.BookmarkPath, tt.name, got, want)
			}
			if body := response.Body.String(); body != "" {
				t.Errorf("GET %s (%s) sent %q, want an empty body", mark.BookmarkPath, tt.name, body)
			}
			if after, found, err := file.Bookmark(mark.NoCourse); found || (tt.body == nil) != (err == nil) {
				t.Errorf("reading the place (%s) left %v, %v", tt.name, found, after)
			}
			if tt.body != nil {
				if data, err := os.ReadFile(progressPath(file)); err != nil || string(data) != *tt.body {
					t.Errorf("the read changed the file to %q: %v", data, err)
				}
			}
		})
	}
}

// postPlace posts form to the bookmark route, as the button of a reading page
// does.
func postPlace(t *testing.T, handler http.Handler, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	return send(t, handler, http.MethodPost, mark.BookmarkPath, formBody, form.Encode())
}

// aPost is what a reading page sends for a place, with the fields this file's
// tests do not vary already in their ordinary spelling.
func aPost(path, anchor string) url.Values {
	return url.Values{"path": {path}, "anchor": {anchor}, "offset": {"1"}, "identity": {anIdentity}}
}

// TestTheRouteKeepsAPlaceOnlyInANoteTheVaultHolds holds what the route asks of
// the vault, and the one reply it gives to a refusal of either kind. The vault
// holds one note; the others are a note it does not have and a sentence that is
// a perfectly good path and names nothing — the shape of the text a visitor to
// a shared instance would want printed on the desk.
func TestTheRouteKeepsAPlaceOnlyInANoteTheVaultHolds(t *testing.T) {
	t.Parallel()

	places := vaultPlaces{{"Notes/real.md", ""}: true}
	// The sentence the route gives a shape it refuses, which a refusal for the
	// vault not holding the note must be indistinguishable from: the reader
	// cannot act on which field was wrong, and a visitor is told nothing about
	// what the vault holds.
	shaped := newServer(t, places).handler
	shaping := postPlace(t, shaped, aPost("../x.md", ""))
	shapeStatus, shapeRefusal := shaping.Code, shaping.Body.String()
	if shapeStatus != http.StatusUnprocessableEntity || shapeRefusal == "" {
		t.Fatalf("a path that climbs out = %d %q, want a refusal with words", shapeStatus, shapeRefusal)
	}
	tests := []struct {
		name string
		path string
		want int
	}{
		{"a note the vault holds", "Notes/real.md", http.StatusNoContent},
		{"a note the vault does not hold", "Notes/nope.md", http.StatusUnprocessableEntity},
		{"a sentence that names no note", "Visit spam.example for free prizes", http.StatusUnprocessableEntity},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := newServer(t, places)
			file, handler := srv.store, srv.handler
			posted := postPlace(t, handler, aPost(tt.path, ""))
			status, body := posted.Code, posted.Body.String()
			if status != tt.want {
				t.Errorf("POST %s over %q = %d %q, want %d", mark.BookmarkPath, tt.path, status, body, tt.want)
			}
			got, kept := bookmarkFor(t, file, mark.NoCourse)
			if wantKept := tt.want == http.StatusNoContent; kept != wantKept {
				t.Errorf("POST over %q kept a place = %t, want %t", tt.path, kept, wantKept)
			}
			if kept && got.RelPath != tt.path {
				t.Errorf("the route kept %q, want %q", got.RelPath, tt.path)
			}
			if tt.want == http.StatusUnprocessableEntity {
				if body != shapeRefusal {
					t.Errorf("a path the vault does not hold was answered %q, want the one refusal %q", body, shapeRefusal)
				}
				if strings.Contains(body, tt.path) {
					t.Errorf("the refusal %q echoes the submitted path", body)
				}
			}
		})
	}
}

// TestTheRouteAsksTheVaultAboutTheNoteAndNotTheAnchor holds the scope of the
// question. The vault answers for an anchor only where it is a heading, and a
// reading page stamps the nearest id of any kind above the window — a block
// address, a footnote. The vault here holds the note and nothing else, so a
// route that asked it about the anchor too would refuse a place the page
// honestly sent.
func TestTheRouteAsksTheVaultAboutTheNoteAndNotTheAnchor(t *testing.T) {
	t.Parallel()

	srv := newServer(t, vaultPlaces{{"Notes/real.md", ""}: true})
	file, handler := srv.store, srv.handler
	posted := postPlace(t, handler, aPost("Notes/real.md", "fn-3"))
	status, body := posted.Code, posted.Body.String()
	if status != http.StatusNoContent {
		t.Fatalf("POST %s over a real note under an id that is no heading = %d %q, want %d",
			mark.BookmarkPath, status, body, http.StatusNoContent)
	}
	if got, ok := bookmarkFor(t, file, mark.NoCourse); !ok || got.Anchor != "fn-3" {
		t.Errorf("the route kept %+v, %t, want the place under fn-3", got, ok)
	}
}

// TestAPlaceTheVaultDoesNotHoldLeavesTheKeptPlaceAlone is the half the table
// above cannot see: a refusal that wiped the file would look the same as one
// that stored nothing, and only a file that already holds a place shows it.
func TestAPlaceTheVaultDoesNotHoldLeavesTheKeptPlaceAlone(t *testing.T) {
	t.Parallel()

	srv := newServer(t, vaultPlaces{{"Notes/real.md", ""}: true})
	file, handler := srv.store, srv.handler
	if posted := postPlace(t, handler, aPost("Notes/real.md", "one")); posted.Code != http.StatusNoContent {
		t.Fatalf("keep the first place = %d %q, want %d", posted.Code, posted.Body.String(), http.StatusNoContent)
	}
	if posted := postPlace(t, handler, aPost("Notes/nope.md", "two")); posted.Code != http.StatusUnprocessableEntity {
		t.Fatalf("a note the vault does not hold = %d, want %d", posted.Code, http.StatusUnprocessableEntity)
	}
	got, ok := bookmarkFor(t, file, mark.NoCourse)
	if !ok || got.RelPath != "Notes/real.md" || got.Anchor != "one" {
		t.Errorf("after the refusal the file holds %+v, %t, want the first place", got, ok)
	}
}

// listedQuestion is a question as the list route sends it to a page.
type listedQuestion struct {
	Path   string    `json:"path"`
	Anchor string    `json:"anchor"`
	Time   time.Time `json:"time"`
}

// listedQuestions is what the list route answers, read as exactly the three
// fields a page knows.
func listedQuestions(t *testing.T, handler http.Handler) []listedQuestion {
	t.Helper()
	response := send(t, handler, http.MethodGet, mark.QuestionPath, formBody, "", "en")
	if response.Code != http.StatusOK {
		t.Fatalf("GET %s = %d %s, want 200", mark.QuestionPath, response.Code, response.Body.String())
	}
	var held []listedQuestion
	decoder := json.NewDecoder(response.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&held); err != nil {
		t.Fatalf("GET %s answered %q, which is not a list of questions: %v", mark.QuestionPath, response.Body.String(), err)
	}
	return held
}

// marked is what the question route answers a press with: whether the section
// is marked now.
func marked(t *testing.T, response *httptest.ResponseRecorder) bool {
	t.Helper()
	var got struct {
		Marked bool `json:"marked"`
	}
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &got) != nil {
		t.Fatalf("a press = %d %q, want 200 and whether the section is marked", response.Code, response.Body.String())
	}
	return got.Marked
}

// anchorsOf is the anchors of held in the order they are listed.
func anchorsOf(held []listedQuestion) []string {
	anchors := make([]string, len(held))
	for i := range held {
		anchors[i] = held[i].Anchor
	}
	return anchors
}

func TestQuestionRouteListsAndTogglesWithServerTime(t *testing.T) {
	t.Parallel()
	srv := newServer(t, nil)
	empty := send(t, srv.handler, http.MethodGet, mark.QuestionPath, formBody, "", "en")
	var none []listedQuestion
	if empty.Code != http.StatusOK || json.Unmarshal(empty.Body.Bytes(), &none) != nil || none == nil || len(none) != 0 {
		t.Fatalf("empty GET = %d %s, want an empty list and not null", empty.Code, empty.Body.String())
	}
	form := url.Values{
		"path": {"Notes/source.md"}, "anchor": {"one"},
		"time": {"2000-01-01T00:00:00Z"},
	}
	before := time.Now().Truncate(time.Millisecond)
	added := send(t, srv.handler, http.MethodPost, mark.QuestionPath, formBody, form.Encode(), "en")
	after := time.Now()
	if !marked(t, added) {
		t.Fatalf("add = %d %s", added.Code, added.Body.String())
	}
	listed := send(t, srv.handler, http.MethodGet, mark.QuestionPath, formBody, "", "en")
	held := listedQuestions(t, srv.handler)
	if len(held) != 1 {
		t.Fatalf("GET marks = %v, want the one", held)
	}
	if held[0].Path != "Notes/source.md" || held[0].Anchor != "one" {
		t.Fatalf("GET returned wrong location: %+v", held[0])
	}
	if held[0].Time.Before(before) || held[0].Time.After(after) {
		t.Fatalf("time %s was not assigned during the server request", held[0].Time)
	}
	for _, response := range []*httptest.ResponseRecorder{empty, added, listed} {
		result := response.Result()
		if result.Header.Get("Content-Type") != "application/json; charset=utf-8" || result.Header.Get("Cache-Control") != "no-store" {
			t.Errorf("JSON response headers = %v", result.Header)
		}
		if err := result.Body.Close(); err != nil {
			t.Error(err)
		}
	}
	removed := send(t, srv.handler, http.MethodPost, mark.QuestionPath, formBody, form.Encode(), "en")
	if marked(t, removed) {
		t.Fatalf("remove = %d %s", removed.Code, removed.Body.String())
	}
	if marks, err := srv.vaultMarks.Questions(); err != nil || len(marks) != 0 {
		t.Fatalf("removed mark still kept: %v, %v", marks, err)
	}
	if again := listedQuestions(t, srv.handler); len(again) != 0 {
		t.Fatalf("GET after the removal = %+v, want an empty list", again)
	}
}

// The list is of the questions of whole sections, which is what the button
// makes and takes away: a question about words, a thought and a highlight are
// kept in the same files and are not listed, and neither is one that is deleted.
func TestQuestionRouteListsOnlyQuestionsOfWholeSections(t *testing.T) {
	t.Parallel()
	srv := newServer(t, nil)
	at := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	onWords := marktest.Question("Notes/source.md", "words", at)
	onWords.Quote = "some words"
	marktest.Leave(t, srv.root, confdir.At(srv.config),
		marktest.Question("Notes/source.md", "whole", at),
		onWords,
		machinemarks.Mark{Kind: machinemarks.Thought, Path: "Notes/source.md", Anchor: "thought", Body: "a thought", At: at},
		machinemarks.Mark{Kind: machinemarks.Highlight, Path: "Notes/source.md", Anchor: "highlight", Quote: "some words", At: at},
	)
	held := listedQuestions(t, srv.handler)
	if want := []listedQuestion{{Path: "Notes/source.md", Anchor: "whole", Time: at}}; !slices.EqualFunc(held, want, func(a, b listedQuestion) bool {
		return a.Path == b.Path && a.Anchor == b.Anchor && a.Time.Equal(b.Time)
	}) {
		t.Fatalf("GET = %+v, want the one question of a whole section, made at %s", held, at)
	}
	if got := send(t, srv.handler, http.MethodPost, mark.QuestionPath, formBody, "path=Notes/source.md&anchor=whole", "en"); marked(t, got) {
		t.Fatalf("pressing the control of the kept section = %d %s, want it taken away", got.Code, got.Body.String())
	}
	if held = listedQuestions(t, srv.handler); len(held) != 0 {
		t.Fatalf("GET after the removal = %+v, want none", held)
	}
}

// What the earlier file of marks holds is listed from the first read, since the
// reader has not marked anything since the upgrade and still has those; nothing
// reads the file as empty, and no read changes it. The first change takes the
// marks in, and the file is left as it was.
func TestQuestionRouteListsTheEarlierFilesQuestionsUntilTheFirstWriteTakesThemIn(t *testing.T) {
	t.Parallel()
	srv := newServer(t, nil)
	at := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	earlier := marksJSON(t, srv.root, machinemarks.Mark{Kind: machinemarks.Question, Path: "Notes/source.md", Anchor: "one", At: at}, machinemarks.Mark{Kind: machinemarks.Question, Path: "Notes/source.md", Anchor: "two", At: at})
	writeFile(t, marksPath(srv.store), earlier)

	if got := anchorsOf(listedQuestions(t, srv.handler)); !slices.Equal(got, []string{"one", "two"}) {
		t.Fatalf("GET over the earlier file = %v, want its two questions", got)
	}
	if _, err := os.Stat(filepath.Join(srv.root, "reader-notes")); !os.IsNotExist(err) {
		t.Fatalf("reading made the reader folder: %v", err)
	}
	// Taking away a question the vault does not hold yet takes the earlier
	// marks in first, then removes it.
	if got := send(t, srv.handler, http.MethodPost, mark.QuestionPath, formBody, "path=Notes/source.md&anchor=one", "en"); marked(t, got) {
		t.Fatalf("taking away a question of the earlier file = %d %s", got.Code, got.Body.String())
	}
	if got := send(t, srv.handler, http.MethodPost, mark.QuestionPath, formBody, "path=Notes/source.md&anchor=three", "en"); !marked(t, got) {
		t.Fatalf("marking a new question = %d %s", got.Code, got.Body.String())
	}
	if got := anchorsOf(listedQuestions(t, srv.handler)); !slices.Equal(got, []string{"two", "three"}) {
		t.Fatalf("GET after the first writes = %v, want the earlier file's second question and the new one", got)
	}
	if after := contents(t, marksPath(srv.store)); after != earlier {
		t.Errorf("the earlier file of marks changed to %q", after)
	}
}

func TestQuestionRouteRefusesBadFormsAndCapsTheBody(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		body string
		want int
	}{
		{"traversal", "path=../private.md&anchor=one", http.StatusUnprocessableEntity},
		{"malformed form", "path=%zz", http.StatusBadRequest},
		{"past body cap", "path=source.md&anchor=one&pad=" + strings.Repeat("x", 4096), http.StatusBadRequest},
		{"inside body cap", "path=source.md&anchor=one&pad=" + strings.Repeat("x", 3000), http.StatusOK},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := newServer(t, nil)
			response := send(t, srv.handler, http.MethodPost, mark.QuestionPath, formBody, tt.body, "en")
			if response.Code != tt.want {
				t.Fatalf("POST = %d %s; want %d", response.Code, response.Body.String(), tt.want)
			}
			marks, err := srv.vaultMarks.Questions()
			if err != nil {
				t.Fatal(err)
			}
			if (len(marks) == 1) != (tt.want == http.StatusOK) {
				t.Fatalf("POST status %d left marks %v", response.Code, marks)
			}
			if tt.want != http.StatusOK && strings.Contains(response.Body.String(), "private.md") {
				t.Fatal("refusal echoes the submitted private path")
			}
			if tt.want != http.StatusOK {
				if _, err = os.Stat(filepath.Join(srv.root, "reader-notes")); !os.IsNotExist(err) {
					t.Errorf("a refused request made the reader folder: %v", err)
				}
			}
		})
	}
}

// A marks.json that cannot be read is what the first write would take in, so
// every route that needs it says the marks cannot be read, in the reader's
// language, and nothing is written or made.
func TestQuestionRoutePreservesCorruptFileAndReportsFailureInBothLanguages(t *testing.T) {
	t.Parallel()
	for _, lang := range []wording.Lang{wording.En, wording.ZhHant} {
		t.Run(string(lang), func(t *testing.T) {
			t.Parallel()
			srv := newServer(t, nil)
			const damaged = `[{"path":"private/secret.md"`
			writeFile(t, marksPath(srv.store), damaged)
			for _, method := range []string{http.MethodGet, http.MethodPost} {
				response := send(t, srv.handler, method, mark.QuestionPath, formBody, "path=source.md&anchor=one", string(lang))
				if response.Code != http.StatusInternalServerError || strings.TrimSpace(response.Body.String()) != wording.QuestionUnavailable.In(lang) {
					t.Errorf("%s damaged file = %d %q; want localized 500", method, response.Code, response.Body.String())
				}
			}
			data, err := os.ReadFile(marksPath(srv.store))
			if err != nil || string(data) != damaged {
				t.Fatalf("route replaced corrupt file with %s: %v", data, err)
			}
			if _, err = os.Stat(filepath.Join(srv.root, "reader-notes")); !os.IsNotExist(err) {
				t.Errorf("a refused write made the reader folder: %v", err)
			}
		})
	}
}

func TestQuestionRouteAcceptsOnlyPlacesTheVaultRenders(t *testing.T) {
	t.Parallel()
	places := vaultPlaces{{"Notes/real.md", "one"}: true, {"Notes/real.md", ""}: true}
	srv := newServer(t, places)
	for _, tt := range []struct {
		body string
		want int
	}{
		{"path=Notes/real.md&anchor=one", http.StatusOK},
		{"path=Notes/real.md", http.StatusOK},
		{"path=Notes/real.md&anchor=missing", http.StatusUnprocessableEntity},
		{"path=Visit+example.com+for+free+prizes.md&anchor=one", http.StatusUnprocessableEntity},
	} {
		if response := send(t, srv.handler, http.MethodPost, mark.QuestionPath, formBody, tt.body, "en"); response.Code != tt.want {
			t.Errorf("POST %q = %d, want %d", tt.body, response.Code, tt.want)
		}
	}
	held, err := srv.vaultMarks.Questions()
	if err != nil || len(held) != 2 {
		t.Fatalf("kept marks = %v, %v; want only the two real places", held, err)
	}
	// A place that has since disappeared can still be cleared.
	delete(places, [2]string{"Notes/real.md", "one"})
	if response := send(t, srv.handler, http.MethodPost, mark.QuestionPath, formBody, "path=Notes/real.md&anchor=one", "en"); marked(t, response) {
		t.Fatalf("clearing a vanished place = %d %s", response.Code, response.Body.String())
	}
	if held, err = srv.vaultMarks.Questions(); err != nil || len(held) != 1 || held[0].Anchor != "" {
		t.Fatalf("kept marks after the clearing = %v, %v; want only the note as a whole", held, err)
	}
}

// A request to mark where the reader folder cannot be written to is refused
// with a reason the reader can act on and with nothing written, wherever a link
// there points. A folder that is mended is written to without the server being
// started again.
func TestQuestionRouteRefusesToWriteWhereTheReaderFolderCannotBeUsed(t *testing.T) {
	t.Parallel()
	for name, spoil := range map[string]func(t *testing.T, folder string) (mustStayEmpty string){
		"a link": func(t *testing.T, folder string) string {
			t.Helper()
			target := t.TempDir()
			if err := os.Symlink(target, folder); err != nil {
				t.Skipf("no symbolic links here: %v", err)
			}
			return target
		},
		"a file": func(t *testing.T, folder string) string {
			t.Helper()
			writeFile(t, folder, "not a folder")
			return ""
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			srv := newServer(t, nil)
			folder := filepath.Join(srv.root, "reader-notes")
			target := spoil(t, folder)
			response := send(t, srv.handler, http.MethodPost, mark.QuestionPath, formBody, "path=Notes/source.md&anchor=one", "en")
			if response.Code != http.StatusUnprocessableEntity {
				t.Fatalf("POST over %s = %d %q, want 422", name, response.Code, response.Body.String())
			}
			if want := wording.MarkNotKept.In(wording.En); !strings.Contains(response.Body.String(), want) || strings.Contains(response.Body.String(), "reader-notes") {
				t.Errorf("the refusal %q is not %q alone: the reader is told the mark was not kept, and why is the log's", response.Body.String(), want)
			}
			if target != "" {
				if entries, err := os.ReadDir(target); err != nil || len(entries) != 0 {
					t.Errorf("a refused write left %v, %v where the link points", entries, err)
				}
			}
			if err := os.Remove(folder); err != nil {
				t.Fatal(err)
			}
			if response = send(t, srv.handler, http.MethodPost, mark.QuestionPath, formBody, "path=Notes/source.md&anchor=one", "en"); !marked(t, response) {
				t.Fatalf("POST after the folder was mended = %d %q, want it kept", response.Code, response.Body.String())
			}
		})
	}
}

// TestRetiredAddressesRedirectAndKeepTheMethod holds the compatibility of the
// two addresses a page drawn before the rename still posts to. A 308 keeps the
// method and the body, so a bookmark or a question sent from such a page still
// arrives; a 301 would turn the POST into a GET and lose it.
func TestRetiredAddressesRedirectAndKeepTheMethod(t *testing.T) {
	t.Parallel()
	mux := newServer(t, vaultPlaces(nil)).handler

	// The requests run in order on one mux: a redirect target that remembered
	// an earlier request's query would show on the one after it.
	for _, tt := range []struct {
		name, method, target, want string
	}{
		{"the old bookmark address", http.MethodPost, "/marks", "/bookmark"},
		{"the old bookmark address with a query", http.MethodPost, "/marks?a=1", "/bookmark?a=1"},
		{"the old bookmark address again without one", http.MethodPost, "/marks", "/bookmark"},
		{"the old question list", http.MethodGet, "/uncertainties", "/questions"},
		{"the old question toggle", http.MethodPost, "/uncertainties", "/questions"},
		{"the old question toggle with a query", http.MethodPost, "/uncertainties?b=2", "/questions?b=2"},
	} {
		request := httptest.NewRequestWithContext(t.Context(), tt.method, tt.target, strings.NewReader("path=a.md"))
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		if response.Code != http.StatusPermanentRedirect {
			t.Errorf("%s: %s %s = %d, want 308", tt.name, tt.method, tt.target, response.Code)
		}
		if got := response.Header().Get("Location"); got != tt.want {
			t.Errorf("%s: %s %s Location = %q, want %q", tt.name, tt.method, tt.target, got, tt.want)
		}
	}
}
