package mark_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/mark"
	"github.com/koopa0/yomihon/internal/wording"
)

// newHandler is the route over a vault that holds every note, so a test that
// is about a field's shape or the body's size is not also about the vault.
func newHandler(t *testing.T) (*mark.Store, http.Handler) {
	t.Helper()
	return newHandlerFor(t, nil)
}

// newHandlerFor is the route over a vault that holds the places given. A nil
// map is the vault that holds every one.
func newHandlerFor(t *testing.T, places vaultPlaces) (*mark.Store, http.Handler) {
	t.Helper()
	file := newStore(t)
	mux := http.NewServeMux()
	mark.NewHandler(file, places, slog.New(slog.DiscardHandler)).Register(mux)
	return file, mux
}

func post(t *testing.T, handler http.Handler, form url.Values) *http.Response {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, mark.Paths.Bookmark, strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder.Result()
}

func TestTheRouteKeepsWhatAReadingPageSends(t *testing.T) {
	t.Parallel()

	file, handler := newHandler(t)
	response := post(t, handler, url.Values{
		"path":     {"Notes/alpha.md"},
		"anchor":   {"a-heading"},
		"offset":   {"320"},
		"identity": {anIdentity},
	})
	defer func() {
		if err := response.Body.Close(); err != nil {
			t.Error(err)
		}
	}()
	if response.StatusCode != http.StatusNoContent {
		t.Errorf("POST %s = %d, want %d", mark.Paths.Bookmark, response.StatusCode, http.StatusNoContent)
	}
	got, ok := heldBookmark(t, file)
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
		// newHandler's vault holds every note, so what refuses these two is the
		// shape of the path and not the vault failing to find a name.
		{"a path carrying a line feed", url.Values{"path": {"Notes/a\nb.md"}, "offset": {"0"}, "identity": {anIdentity}}, http.StatusUnprocessableEntity},
		{"a path carrying a NUL and a line feed", url.Values{"path": {"Notes/a\x00b\nc.md"}, "offset": {"0"}, "identity": {anIdentity}}, http.StatusUnprocessableEntity},
		{"an offset that is not a number", url.Values{"path": {"a.md"}, "offset": {"soon"}, "identity": {anIdentity}}, http.StatusUnprocessableEntity},
		{"no offset at all", url.Values{"path": {"a.md"}, "identity": {anIdentity}}, http.StatusUnprocessableEntity},
		{"no identity", url.Values{"path": {"a.md"}, "offset": {"0"}}, http.StatusUnprocessableEntity},
		{"nothing at all", url.Values{}, http.StatusUnprocessableEntity},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			file, handler := newHandler(t)
			response := post(t, handler, tt.form)
			defer func() {
				if err := response.Body.Close(); err != nil {
					t.Error(err)
				}
			}()
			if response.StatusCode != tt.want {
				t.Errorf("POST %s (%s) = %d, want %d", mark.Paths.Bookmark, tt.name, response.StatusCode, tt.want)
			}
			if _, ok := heldBookmark(t, file); ok {
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
			file, handler := newHandler(t)
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
			response := post(t, handler, form)
			defer func() {
				if err := response.Body.Close(); err != nil {
					t.Error(err)
				}
			}()
			if response.StatusCode != tt.want {
				t.Errorf("POST %s with a %d-byte body = %d, want %d", mark.Paths.Bookmark, size, response.StatusCode, tt.want)
			}
			_, kept := heldBookmark(t, file)
			if wantKept := tt.want == http.StatusNoContent; kept != wantKept {
				t.Errorf("a %d-byte body kept a place = %t, want %t", size, kept, wantKept)
			}
		})
	}
}

// TestOnlyPostChangesThePlace holds the methods. A place is a change, and only
// a POST makes one; GET reads it, and anything else is refused with the two the
// route does answer named.
func TestOnlyPostChangesThePlace(t *testing.T) {
	t.Parallel()

	for _, method := range []string{http.MethodPut, http.MethodDelete, http.MethodPatch} {
		t.Run(method, func(t *testing.T) {
			t.Parallel()
			file, handler := newHandler(t)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequestWithContext(t.Context(), method, mark.Paths.Bookmark, strings.NewReader(aPost("Notes/alpha.md", "a").Encode()))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			handler.ServeHTTP(recorder, request)
			response := recorder.Result()
			defer func() {
				if err := response.Body.Close(); err != nil {
					t.Error(err)
				}
			}()
			if response.StatusCode != http.StatusMethodNotAllowed {
				t.Errorf("%s %s = %d, want %d", method, mark.Paths.Bookmark, response.StatusCode, http.StatusMethodNotAllowed)
			}
			if got, want := response.Header.Get("Allow"), "GET, HEAD, POST"; got != want {
				t.Errorf("%s %s Allow = %q, want %q", method, mark.Paths.Bookmark, got, want)
			}
			if _, ok := heldBookmark(t, file); ok {
				t.Errorf("%s kept a place", method)
			}
		})
	}
}

// get asks the route where the place kept now is.
func get(t *testing.T, handler http.Handler) *http.Response {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, mark.Paths.Bookmark, http.NoBody))
	return recorder.Result()
}

// TestTheRouteSaysWhereThePlaceIsKept holds what a page shown again from the
// browser's cache is given to draw from: the note and the anchor of the place
// kept now, and nothing the page has to be trusted with beyond them. The answer
// must not be one the browser keeps, or the page would be redrawn from the
// staleness this reads past.
func TestTheRouteSaysWhereThePlaceIsKept(t *testing.T) {
	t.Parallel()

	_, handler := newHandler(t)
	if status, body := replyTo(t, handler, aPost("Notes/alpha.md", "first-heading")); status != http.StatusNoContent {
		t.Fatalf("POST %s = %d %q, want %d", mark.Paths.Bookmark, status, body, http.StatusNoContent)
	}
	response := get(t, handler)
	defer func() {
		if err := response.Body.Close(); err != nil {
			t.Error(err)
		}
	}()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET %s = %d, want %d", mark.Paths.Bookmark, response.StatusCode, http.StatusOK)
	}
	if got, want := response.Header.Get("Cache-Control"), "no-store"; got != want {
		t.Errorf("GET %s Cache-Control = %q, want %q", mark.Paths.Bookmark, got, want)
	}
	if got, want := response.Header.Get("Content-Type"), "application/json; charset=utf-8"; got != want {
		t.Errorf("GET %s Content-Type = %q, want %q", mark.Paths.Bookmark, got, want)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read the reply: %v", err)
	}
	if diff := cmp.Diff(`{"path":"Notes/alpha.md","anchor":"first-heading"}`+"\n", string(body)); diff != "" {
		t.Errorf("GET %s body (-want +got):\n%s", mark.Paths.Bookmark, diff)
	}

	// Keeping another place replaces the first, and the answer follows.
	if status, body := replyTo(t, handler, aPost("Notes/beta.md", "")); status != http.StatusNoContent {
		t.Fatalf("POST %s = %d %q, want %d", mark.Paths.Bookmark, status, body, http.StatusNoContent)
	}
	again := get(t, handler)
	defer func() {
		if closeErr := again.Body.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	}()
	body, err = io.ReadAll(again.Body)
	if err != nil {
		t.Fatalf("read the reply: %v", err)
	}
	if diff := cmp.Diff(`{"path":"Notes/beta.md","anchor":""}`+"\n", string(body)); diff != "" {
		t.Errorf("GET %s after keeping another place (-want +got):\n%s", mark.Paths.Bookmark, diff)
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
			file, handler := newHandler(t)
			if tt.body != nil {
				if err := os.MkdirAll(filepath.Dir(progressPath(file)), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(progressPath(file), []byte(*tt.body), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			response := get(t, handler)
			defer func() {
				if err := response.Body.Close(); err != nil {
					t.Error(err)
				}
			}()
			if response.StatusCode != http.StatusNoContent {
				t.Errorf("GET %s (%s) = %d, want %d", mark.Paths.Bookmark, tt.name, response.StatusCode, http.StatusNoContent)
			}
			if got, want := response.Header.Get("Cache-Control"), "no-store"; got != want {
				t.Errorf("GET %s (%s) Cache-Control = %q, want %q", mark.Paths.Bookmark, tt.name, got, want)
			}
			if body, err := io.ReadAll(response.Body); err != nil || len(body) != 0 {
				t.Errorf("GET %s (%s) sent %q, %v, want an empty body", mark.Paths.Bookmark, tt.name, body, err)
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

// aPost is what a reading page sends for a place, with the fields this file's
// tests do not vary already in their ordinary spelling.
func aPost(path, anchor string) url.Values {
	return url.Values{"path": {path}, "anchor": {anchor}, "offset": {"1"}, "identity": {anIdentity}}
}

// replyTo posts form and returns the status and the body the route answered.
func replyTo(t *testing.T, handler http.Handler, form url.Values) (status int, body string) {
	t.Helper()
	response := post(t, handler, form)
	defer func() {
		if err := response.Body.Close(); err != nil {
			t.Error(err)
		}
	}()
	text, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read the reply: %v", err)
	}
	return response.StatusCode, string(text)
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
	_, shaped := newHandlerFor(t, places)
	shapeStatus, shapeRefusal := replyTo(t, shaped, aPost("../x.md", ""))
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
			file, handler := newHandlerFor(t, places)
			status, body := replyTo(t, handler, aPost(tt.path, ""))
			if status != tt.want {
				t.Errorf("POST %s over %q = %d %q, want %d", mark.Paths.Bookmark, tt.path, status, body, tt.want)
			}
			got, kept := heldBookmark(t, file)
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

	file, handler := newHandlerFor(t, vaultPlaces{{"Notes/real.md", ""}: true})
	status, body := replyTo(t, handler, aPost("Notes/real.md", "fn-3"))
	if status != http.StatusNoContent {
		t.Fatalf("POST %s over a real note under an id that is no heading = %d %q, want %d",
			mark.Paths.Bookmark, status, body, http.StatusNoContent)
	}
	if got, ok := heldBookmark(t, file); !ok || got.Anchor != "fn-3" {
		t.Errorf("the route kept %+v, %t, want the place under fn-3", got, ok)
	}
}

// TestAPlaceTheVaultDoesNotHoldLeavesTheKeptPlaceAlone is the half the table
// above cannot see: a refusal that wiped the file would look the same as one
// that stored nothing, and only a file that already holds a place shows it.
func TestAPlaceTheVaultDoesNotHoldLeavesTheKeptPlaceAlone(t *testing.T) {
	t.Parallel()

	file, handler := newHandlerFor(t, vaultPlaces{{"Notes/real.md", ""}: true})
	if status, body := replyTo(t, handler, aPost("Notes/real.md", "one")); status != http.StatusNoContent {
		t.Fatalf("keep the first place = %d %q, want %d", status, body, http.StatusNoContent)
	}
	if status, _ := replyTo(t, handler, aPost("Notes/nope.md", "two")); status != http.StatusUnprocessableEntity {
		t.Fatalf("a note the vault does not hold = %d, want %d", status, http.StatusUnprocessableEntity)
	}
	got, ok := heldBookmark(t, file)
	if !ok || got.RelPath != "Notes/real.md" || got.Anchor != "one" {
		t.Errorf("after the refusal the file holds %+v, %t, want the first place", got, ok)
	}
}

// TestTheRouteRefusesAVaultItIsNotGiven holds the constructor's own guard,
// which the question route has too: a route with no vault to ask would
// either accept everything or fail at the first request, and neither is a
// refusal anyone chose.
func TestTheRouteRefusesAVaultItIsNotGiven(t *testing.T) {
	t.Parallel()

	defer func() {
		if recover() == nil {
			t.Error("mark.NewHandler accepted a nil Places")
		}
	}()
	mark.NewHandler(newStore(t), nil, slog.New(slog.DiscardHandler))
}

// vaultPlaces answers for one vault. A nil map accepts every place.
type vaultPlaces map[[2]string]bool

func (p vaultPlaces) HasPlace(rel, anchor string) bool {
	return p == nil || p[[2]string{rel, anchor}]
}

// Reading is not asked of these tests: the bookmark and question routes find
// no words. It answers that every note is readable and holds none.
func (p vaultPlaces) Reading(string, wording.Lang) (mark.Page, bool) {
	return mark.Page{}, true
}

// listedQuestion is a question as the list route sends it to a page.
type listedQuestion struct {
	Path   string    `json:"path"`
	Anchor string    `json:"anchor"`
	Time   time.Time `json:"time"`
}

func questionRequest(t *testing.T, handler http.Handler, method, body, lang string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), method, mark.Paths.Question, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(&http.Cookie{Name: "yomihon_lang", Value: lang, Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func TestQuestionRouteListsAndTogglesWithServerTime(t *testing.T) {
	t.Parallel()
	file, handler := newHandler(t)
	empty := questionRequest(t, handler, http.MethodGet, "", "en")
	if empty.Code != http.StatusOK || empty.Body.String() != "[]\n" {
		t.Fatalf("empty GET = %d %s", empty.Code, empty.Body.String())
	}
	form := url.Values{
		"path": {"Notes/source.md"}, "anchor": {"one"},
		"time": {"2000-01-01T00:00:00Z"},
	}
	before := time.Now()
	added := questionRequest(t, handler, http.MethodPost, form.Encode(), "en")
	after := time.Now()
	if added.Code != http.StatusOK || added.Body.String() != "{\"marked\":true}\n" {
		t.Fatalf("add = %d %s", added.Code, added.Body.String())
	}
	listed := questionRequest(t, handler, http.MethodGet, "", "en")
	if listed.Code != http.StatusOK {
		t.Fatalf("GET = %d %s", listed.Code, listed.Body.String())
	}
	var held []listedQuestion
	if err := json.Unmarshal(listed.Body.Bytes(), &held); err != nil || len(held) != 1 {
		t.Fatalf("GET marks = %v, %v", held, err)
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
	removed := questionRequest(t, handler, http.MethodPost, form.Encode(), "en")
	if removed.Code != http.StatusOK || removed.Body.String() != "{\"marked\":false}\n" {
		t.Fatalf("remove = %d %s", removed.Code, removed.Body.String())
	}
	if marks, err := file.List(mark.Question); err != nil || len(marks) != 0 {
		t.Fatalf("removed mark still stored: %v, %v", marks, err)
	}
}

func TestQuestionRouteRefusesBadFormsAndCapsTheBody(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		body string
		want int
	}{
		{"missing path", "anchor=one", http.StatusUnprocessableEntity},
		{"traversal", "path=../private.md&anchor=one", http.StatusUnprocessableEntity},
		{"invalid anchor", "path=source.md&anchor=one%23two", http.StatusUnprocessableEntity},
		{"invalid utf-8 anchor", "path=source.md&anchor=x%FF", http.StatusUnprocessableEntity},
		{"malformed form", "path=%zz", http.StatusBadRequest},
		{"past body cap", "path=source.md&anchor=one&pad=" + strings.Repeat("x", 4096), http.StatusBadRequest},
		{"inside body cap", "path=source.md&anchor=one&pad=" + strings.Repeat("x", 3000), http.StatusOK},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			file, handler := newHandler(t)
			response := questionRequest(t, handler, http.MethodPost, tt.body, "en")
			if response.Code != tt.want {
				t.Fatalf("POST = %d %s; want %d", response.Code, response.Body.String(), tt.want)
			}
			marks, err := file.List(mark.Question)
			if err != nil {
				t.Fatal(err)
			}
			if (len(marks) == 1) != (tt.want == http.StatusOK) {
				t.Fatalf("POST status %d left marks %v", response.Code, marks)
			}
			if tt.want != http.StatusOK && strings.Contains(response.Body.String(), "private.md") {
				t.Fatal("refusal echoes the submitted private path")
			}
		})
	}
}

func TestQuestionRoutePreservesCorruptFileAndReportsFailureInBothLanguages(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		lang string
		want string
	}{
		{"en", "Marks cannot be read right now. The existing file was left unchanged.\n"},
		{"zh-Hant", "目前無法讀取標記，原有檔案未更動。\n"},
	} {
		t.Run(tt.lang, func(t *testing.T) {
			t.Parallel()
			file, handler := newHandler(t)
			if err := os.MkdirAll(filepath.Dir(marksPath(file)), 0o700); err != nil {
				t.Fatal(err)
			}
			const damaged = `[{"path":"private/secret.md"`
			if err := os.WriteFile(marksPath(file), []byte(damaged), 0o600); err != nil {
				t.Fatal(err)
			}
			for _, method := range []string{http.MethodGet, http.MethodPost} {
				response := questionRequest(t, handler, method, "path=source.md&anchor=one", tt.lang)
				if response.Code != http.StatusInternalServerError || response.Body.String() != tt.want {
					t.Errorf("%s damaged file = %d %q; want localized 500", method, response.Code, response.Body.String())
				}
			}
			data, err := os.ReadFile(marksPath(file))
			if err != nil || string(data) != damaged {
				t.Fatalf("route replaced corrupt file with %s: %v", data, err)
			}
		})
	}
}

func TestQuestionRouteRejectsOtherWriteMethods(t *testing.T) {
	t.Parallel()
	for _, method := range []string{http.MethodPut, http.MethodPatch, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			t.Parallel()
			file, handler := newHandler(t)
			response := questionRequest(t, handler, method, "path=source.md&anchor=one", "en")
			if response.Code != http.StatusMethodNotAllowed {
				t.Fatalf("%s = %d, want 405", method, response.Code)
			}
			if _, err := os.Stat(marksPath(file)); !os.IsNotExist(err) {
				t.Fatalf("%s created storage: %v", method, err)
			}
		})
	}
}

func TestQuestionRouteRefusesInvalidUTF8RepeatedlyAndStaysReadable(t *testing.T) {
	t.Parallel()
	_, handler := newHandler(t)
	for range 2 {
		if response := questionRequest(t, handler, http.MethodPost, "path=source.md&anchor=x%FF", "en"); response.Code != http.StatusUnprocessableEntity {
			t.Fatalf("POST invalid UTF-8 = %d, want 422", response.Code)
		}
	}
	if response := questionRequest(t, handler, http.MethodGet, "", "en"); response.Code != http.StatusOK {
		t.Fatalf("GET after the refusals = %d %s, want 200", response.Code, response.Body.String())
	}
}

func TestQuestionRouteAcceptsOnlyPlacesTheVaultRenders(t *testing.T) {
	t.Parallel()
	places := vaultPlaces{{"Notes/real.md", "one"}: true, {"Notes/real.md", ""}: true}
	file, handler := newHandlerFor(t, places)
	for _, tt := range []struct {
		body string
		want int
	}{
		{"path=Notes/real.md&anchor=one", http.StatusOK},
		{"path=Notes/real.md", http.StatusOK},
		{"path=Notes/real.md&anchor=missing", http.StatusUnprocessableEntity},
		{"path=Visit+example.com+for+free+prizes.md&anchor=one", http.StatusUnprocessableEntity},
	} {
		if response := questionRequest(t, handler, http.MethodPost, tt.body, "en"); response.Code != tt.want {
			t.Errorf("POST %q = %d, want %d", tt.body, response.Code, tt.want)
		}
	}
	held, err := file.List(mark.Question)
	if err != nil || len(held) != 2 {
		t.Fatalf("stored marks = %v, %v; want only the two real places", held, err)
	}
	// A place that has since disappeared can still be cleared.
	delete(places, [2]string{"Notes/real.md", "one"})
	if response := questionRequest(t, handler, http.MethodPost, "path=Notes/real.md&anchor=one", "en"); response.Code != http.StatusOK || response.Body.String() != "{\"marked\":false}\n" {
		t.Fatalf("clearing a vanished place = %d %s", response.Code, response.Body.String())
	}
}

// TestRetiredAddressesRedirectAndKeepTheMethod holds the compatibility of the
// two addresses a page drawn before the rename still posts to. A 308 keeps the
// method and the body, so a bookmark or a question sent from such a page still
// arrives; a 301 would turn the POST into a GET and lose it.
func TestRetiredAddressesRedirectAndKeepTheMethod(t *testing.T) {
	t.Parallel()
	log := slog.New(slog.DiscardHandler)
	file := newStore(t)
	mux := http.NewServeMux()
	mark.NewHandler(file, vaultPlaces(nil), log).Register(mux)

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
