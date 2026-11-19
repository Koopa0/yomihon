package mark_test

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/mark"
	"github.com/koopa0/yomihon/internal/wording"
)

const sourceText = "In Japanese, the topic particle marks what the sentence is about. A second sentence follows."

// aVault is the notes a route can place a mark in: each path with the words its
// page holds. Every note has one place, "section-one", and the note as a whole.
type aVault map[string]string

func (v aVault) HasPlace(rel, anchor string) bool {
	_, ok := v[rel]
	return ok && (anchor == "" || anchor == "section-one")
}

func (v aVault) Reading(rel string, _ wording.Lang) (mark.Page, bool) {
	text, ok := v[rel]
	return mark.Page{Text: text, Identity: anIdentity}, ok
}

// Units and Rows are not asked of the mark routes.
func (v aVault) Units(string) (mainLine, branches []string, ok bool) {
	return nil, nil, false
}

func (v aVault) Rows(string) (planned int, rows []string, ok bool) {
	return 0, nil, false
}

// answer is what a route sends: the mark, and where it stands in its note.
type answer struct {
	mark.Mark

	Placement string `json:"placement"`
}

func newMarkRoutes(t *testing.T) (*mark.Store, aVault, http.Handler) {
	t.Helper()
	store := newStore(t)
	notes := aVault{"Notes/source.md": sourceText}
	mux := http.NewServeMux()
	mark.NewHandler(store, notes, slog.New(slog.DiscardHandler)).Register(mux)
	return store, notes, mux
}

func asJSON(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func write(t *testing.T, handler http.Handler, method, target, contentType, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), method, target, strings.NewReader(body))
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func writeJSON(t *testing.T, handler http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	return write(t, handler, method, target, "application/json", body)
}

func decodeAnswer(t *testing.T, response *httptest.ResponseRecorder) answer {
	t.Helper()
	var got answer
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatalf("the answer %q is not a mark: %v", response.Body.String(), err)
	}
	return got
}

func aHighlightBody() map[string]any {
	return map[string]any{
		"kind": "highlight", "path": "Notes/source.md", "anchor": "section-one",
		"quote": "the topic particle", "before": "In Japanese,", "after": "marks what the sentence is about",
	}
}

func TestMarkRouteKeepsEachKindAndAnswersWithTheMark(t *testing.T) {
	t.Parallel()

	for name, tt := range map[string]struct {
		fields    map[string]any
		placement string
	}{
		"a highlight": {aHighlightBody(), "found"},
		"a question on words": {map[string]any{
			"kind": "question", "path": "Notes/source.md", "anchor": "section-one",
			"quote": "A second sentence", "reason": "why a second?",
		}, "found"},
		"a question on a section": {map[string]any{
			"kind": "question", "path": "Notes/source.md", "anchor": "section-one",
		}, "section"},
		"a thought on words": {map[string]any{
			"kind": "thought", "path": "Notes/source.md", "anchor": "section-one",
			"quote": "marks what the sentence is about", "body": "The topic, not the subject.",
		}, "found"},
		"a thought on the whole note": {map[string]any{
			"kind": "thought", "path": "Notes/source.md", "body": "A note about the note.",
		}, "section"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			store, _, routes := newMarkRoutes(t)
			started := time.Now()
			response := writeJSON(t, routes, http.MethodPost, mark.Paths.Mark, asJSON(t, tt.fields))
			if response.Code != http.StatusCreated {
				t.Fatalf("status = %d, body %q; want 201", response.Code, response.Body.String())
			}
			got := decodeAnswer(t, response)
			if got.Placement != tt.placement {
				t.Errorf("placement = %q, want %q", got.Placement, tt.placement)
			}
			if got.Identity != anIdentity {
				t.Errorf("identity = %q, want the note's own, which the server reads and the page does not send", got.Identity)
			}
			if got.At.Before(started) || got.At.After(time.Now()) {
				t.Errorf("at = %v, want the server's clock between %v and now", got.At, started)
			}
			held, err := store.List()
			if err != nil || len(held) != 1 || held[0].ID != got.ID {
				t.Fatalf("kept = %+v, %v; want the mark that was answered", held, err)
			}
			if diff := cmp.Diff(held[0], got.Mark); diff != "" {
				t.Errorf("the answer is not what is kept (-kept +answer):\n%s", diff)
			}
			if got := response.Header().Get("Cache-Control"); got != "no-store" {
				t.Errorf("Cache-Control = %q, want no-store", got)
			}
		})
	}
}

func TestMarkRouteKeepsOnlyWordsTheNoteHas(t *testing.T) {
	t.Parallel()

	with := func(edit func(map[string]any)) map[string]any {
		fields := aHighlightBody()
		edit(fields)
		return fields
	}
	for name, fields := range map[string]map[string]any{
		"words the note never had":       with(func(f map[string]any) { f["quote"] = "the subject particle" }),
		"a note nobody holds":            with(func(f map[string]any) { f["path"] = "Notes/gone.md" }),
		"an anchor the note has not":     with(func(f map[string]any) { f["anchor"] = "elsewhere" }),
		"a highlight of nothing":         with(func(f map[string]any) { delete(f, "quote") }),
		"a kind nobody has":              with(func(f map[string]any) { f["kind"] = "bookmark" }),
		"no kind":                        with(func(f map[string]any) { delete(f, "kind") }),
		"a thought that says nothing":    {"kind": "thought", "path": "Notes/source.md"},
		"a reason on a highlight":        with(func(f map[string]any) { f["reason"] = "why" }),
		"a path out of the vault":        with(func(f map[string]any) { f["path"] = "../x.md" }),
		"a quote over the bound":         with(func(f map[string]any) { f["quote"] = strings.Repeat("a", 9<<10) }),
		"words in a section that is not": {"kind": "question", "path": "Notes/source.md", "anchor": "elsewhere"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			store, _, routes := newMarkRoutes(t)
			response := writeJSON(t, routes, http.MethodPost, mark.Paths.Mark, asJSON(t, fields))
			if response.Code != http.StatusUnprocessableEntity {
				t.Errorf("status = %d, body %q; want 422", response.Code, response.Body.String())
			}
			if store.Stored() {
				t.Error("a refused mark left a file")
			}
		})
	}
}

func TestMarkRouteKeepsAHighlightOnceAndEveryThoughtAsNew(t *testing.T) {
	t.Parallel()

	store, _, routes := newMarkRoutes(t)
	first := writeJSON(t, routes, http.MethodPost, mark.Paths.Mark, asJSON(t, aHighlightBody()))
	second := writeJSON(t, routes, http.MethodPost, mark.Paths.Mark, asJSON(t, aHighlightBody()))
	if first.Code != http.StatusCreated || second.Code != http.StatusOK {
		t.Fatalf("statuses = %d, %d; want 201, then 200 for what is kept", first.Code, second.Code)
	}
	if a, b := decodeAnswer(t, first), decodeAnswer(t, second); a.ID != b.ID {
		t.Errorf("the second answer names %s, want the mark kept first, %s", b.ID, a.ID)
	}
	thought := map[string]any{"kind": "thought", "path": "Notes/source.md", "body": "again"}
	for range 2 {
		if got := writeJSON(t, routes, http.MethodPost, mark.Paths.Mark, asJSON(t, thought)).Code; got != http.StatusCreated {
			t.Errorf("a thought answered %d, want 201 each time", got)
		}
	}
	if held := listed(t, store); len(held) != 3 {
		t.Errorf("%d marks are kept, want a highlight and two thoughts", len(held))
	}
}

// TestMarkRouteAnswersWordsThatLookLikeMarkupAsText holds the only part of the
// browser's promise this route can keep: it sends JSON, in which what a reader
// wrote is the string they wrote and not a byte of markup.
func TestMarkRouteAnswersWordsThatLookLikeMarkupAsText(t *testing.T) {
	t.Parallel()

	hostile := `<script>alert(1)</script><img src=x onerror=alert(1)> &amp; "quoted" </p>`
	store := newStore(t)
	notes := aVault{"Notes/source.md": "before " + hostile + " after"}
	mux := http.NewServeMux()
	mark.NewHandler(store, notes, slog.New(slog.DiscardHandler)).Register(mux)

	fields := map[string]any{
		"kind": "question", "path": "Notes/source.md", "anchor": "section-one",
		"quote": hostile, "reason": hostile,
	}
	response := writeJSON(t, mux, http.MethodPost, mark.Paths.Mark, asJSON(t, fields))
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body %q", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, want JSON, which a page does not treat as markup", got)
	}
	if bytes.ContainsAny(response.Body.Bytes(), "<>") {
		t.Errorf("the answer carries a markup byte: %s", response.Body.String())
	}
	if got := decodeAnswer(t, response); got.Quote != hostile || got.Reason != hostile {
		t.Errorf("the words came back as %q and %q, want what was sent", got.Quote, got.Reason)
	}
}

func TestMarkRoutePutsBackWhatWasRemovedAsItWas(t *testing.T) {
	t.Parallel()

	store, _, routes := newMarkRoutes(t)
	created := writeJSON(t, routes, http.MethodPost, mark.Paths.Mark, asJSON(t, aHighlightBody()))
	kept := decodeAnswer(t, created)
	before, err := store.List()
	if err != nil {
		t.Fatal(err)
	}

	removed := writeJSON(t, routes, http.MethodDelete, mark.Paths.Mark+"/"+kept.ID, "")
	if removed.Code != http.StatusOK {
		t.Fatalf("DELETE = %d, %q; want 200", removed.Code, removed.Body.String())
	}
	if gone := decodeAnswer(t, removed); gone.ID != kept.ID || gone.Placement != "found" {
		t.Errorf("DELETE answered %+v, want the mark as it was", gone)
	}
	if held := listed(t, store); len(held) != 0 {
		t.Fatalf("%d marks are kept after the removal", len(held))
	}

	back := writeJSON(t, routes, http.MethodPut, mark.Paths.Mark+"/"+kept.ID, removed.Body.String())
	if back.Code != http.StatusOK {
		t.Fatalf("PUT = %d, %q; want 200", back.Code, back.Body.String())
	}
	after := listed(t, store)
	if diff := cmp.Diff(before, after); diff != "" {
		t.Errorf("what is kept after the undo is not what was kept (-before +after):\n%s", diff)
	}

	if again := writeJSON(t, routes, http.MethodPut, mark.Paths.Mark+"/"+kept.ID, removed.Body.String()); again.Code != http.StatusConflict {
		t.Errorf("a second PUT = %d, want 409", again.Code)
	}
	if held := listed(t, store); len(held) != 1 {
		t.Errorf("%d marks are kept after a second PUT, want 1", len(held))
	}
}

func TestMarkRoutePutsBackAMarkWhoseWordsHaveChanged(t *testing.T) {
	t.Parallel()

	store, notes, routes := newMarkRoutes(t)
	kept := decodeAnswer(t, writeJSON(t, routes, http.MethodPost, mark.Paths.Mark, asJSON(t, aHighlightBody())))
	removed := writeJSON(t, routes, http.MethodDelete, mark.Paths.Mark+"/"+kept.ID, "")
	// The author rewrote the sentence while the reader had the undo in view.
	notes["Notes/source.md"] = "A different note entirely."

	back := writeJSON(t, routes, http.MethodPut, mark.Paths.Mark+"/"+kept.ID, removed.Body.String())
	if back.Code != http.StatusOK {
		t.Fatalf("PUT = %d, %q; want 200: the mark is still the reader's", back.Code, back.Body.String())
	}
	if got := decodeAnswer(t, back); got.Placement != "changed" {
		t.Errorf("placement = %q, want changed", got.Placement)
	}
	if held := listed(t, store); len(held) != 1 {
		t.Errorf("%d marks are kept, want the one put back", len(held))
	}
}

func TestMarkRouteRefusesToPutBackWhatIsNotAMarkOfThisVault(t *testing.T) {
	t.Parallel()

	id, other := strings.Repeat("c", 32), strings.Repeat("d", 32)
	good := func() map[string]any {
		return map[string]any{
			"id": id, "kind": "highlight", "path": "Notes/source.md", "anchor": "section-one",
			"quote": "the topic particle", "identity": anIdentity, "at": "2026-09-22T11:00:00Z",
		}
	}
	_, _, control := newMarkRoutes(t)
	if got := writeJSON(t, control, http.MethodPut, mark.Paths.Mark+"/"+id, asJSON(t, good())); got.Code != http.StatusOK {
		t.Fatalf("the control mark answered %d, %q", got.Code, got.Body.String())
	}
	for _, tt := range []struct {
		name, pathID string
		edit         func(map[string]any)
	}{
		{"another id in the body", id, func(f map[string]any) { f["id"] = other }},
		{"another id in the path", other, func(map[string]any) {}},
		{"no time", id, func(f map[string]any) { delete(f, "at") }},
		{"a time to come", id, func(f map[string]any) { f["at"] = time.Now().Add(time.Hour).Format(time.RFC3339) }},
		{"a note nobody holds", id, func(f map[string]any) { f["path"] = "Notes/gone.md" }},
		{"an anchor that is not a place", id, func(f map[string]any) { f["anchor"] = "elsewhere" }},
		{"an identity of nothing", id, func(f map[string]any) { f["identity"] = "abc" }},
		{"a kind nobody has", id, func(f map[string]any) { f["kind"] = "bookmark" }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			store, _, routes := newMarkRoutes(t)
			fields := good()
			tt.edit(fields)
			if got := writeJSON(t, routes, http.MethodPut, mark.Paths.Mark+"/"+tt.pathID, asJSON(t, fields)); got.Code != http.StatusUnprocessableEntity {
				t.Errorf("status = %d, %q; want 422", got.Code, got.Body.String())
			}
			if store.Stored() {
				t.Error("a refused mark left a file")
			}
		})
	}
}

func TestMarkRouteRevisesAQuestionAndAThought(t *testing.T) {
	t.Parallel()

	store, _, routes := newMarkRoutes(t)
	question := decodeAnswer(t, writeJSON(t, routes, http.MethodPost, mark.Paths.Mark, asJSON(t, map[string]any{
		"kind": "question", "path": "Notes/source.md", "anchor": "section-one", "quote": "A second sentence",
	})))
	thought := decodeAnswer(t, writeJSON(t, routes, http.MethodPost, mark.Paths.Mark, asJSON(t, map[string]any{
		"kind": "thought", "path": "Notes/source.md", "body": "first words",
	})))

	resolved := writeJSON(t, routes, http.MethodPatch, mark.Paths.Mark+"/"+question.ID, `{"resolved":true,"reason":"now I see"}`)
	if resolved.Code != http.StatusOK {
		t.Fatalf("PATCH = %d, %q", resolved.Code, resolved.Body.String())
	}
	got := decodeAnswer(t, resolved)
	if got.ResolvedAt.IsZero() || got.Reason != "now I see" || got.Placement != "found" {
		t.Errorf("answer = %+v, want a resolved question with its new reason, still found", got)
	}
	reopened := decodeAnswer(t, writeJSON(t, routes, http.MethodPatch, mark.Paths.Mark+"/"+question.ID, `{"resolved":false}`))
	if !reopened.ResolvedAt.IsZero() || reopened.Reason != "now I see" {
		t.Errorf("reopened = %+v, want it open with its reason left alone", reopened)
	}
	reworded := decodeAnswer(t, writeJSON(t, routes, http.MethodPatch, mark.Paths.Mark+"/"+thought.ID, `{"body":"second words"}`))
	if reworded.Body != "second words" {
		t.Errorf("body = %q", reworded.Body)
	}
	held := listed(t, store)
	if len(held) != 2 || held[0].Reason != "now I see" || held[1].Body != "second words" {
		t.Errorf("what is kept is %+v", held)
	}
}

func TestMarkRouteRefusesAChangeTheMarkCannotTake(t *testing.T) {
	t.Parallel()

	store, _, routes := newMarkRoutes(t)
	highlight := decodeAnswer(t, writeJSON(t, routes, http.MethodPost, mark.Paths.Mark, asJSON(t, aHighlightBody())))
	before := contents(t, marksPath(store))
	for name, body := range map[string]string{
		"a reason on a highlight": `{"reason":"x"}`,
		"a resolution":            `{"resolved":true}`,
		"a body":                  `{"body":"x"}`,
		"nothing to change":       `{}`,
	} {
		if got := writeJSON(t, routes, http.MethodPatch, mark.Paths.Mark+"/"+highlight.ID, body); got.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s = %d, %q; want 422", name, got.Code, got.Body.String())
		}
	}
	if after := contents(t, marksPath(store)); after != before {
		t.Error("a refused change rewrote the file")
	}
}

func TestMarkRouteFindsOnlyIdsTheStoreIssues(t *testing.T) {
	t.Parallel()

	store, _, routes := newMarkRoutes(t)
	kept := decodeAnswer(t, writeJSON(t, routes, http.MethodPost, mark.Paths.Mark, asJSON(t, aHighlightBody())))
	put := asJSON(t, answer{Mark: kept.Mark, Placement: "found"})
	bodies := map[string]string{http.MethodPut: put, http.MethodPatch: `{"resolved":true}`, http.MethodDelete: ""}
	// Well-formed ids nothing holds are PUT's own business: that is the undo.
	for name, id := range map[string]string{
		"the id in capitals":   strings.ToUpper(kept.ID),
		"the id cut short":     kept.ID[:31],
		"the id with a suffix": kept.ID + "0",
		"a path":               "..%2Fmarks",
		"a wildcard":           "%25",
	} {
		for method, body := range bodies {
			if response := writeJSON(t, routes, method, mark.Paths.Mark+"/"+id, body); response.Code != http.StatusNotFound {
				t.Errorf("%s with %s = %d, %q; want 404", method, name, response.Code, response.Body.String())
			}
		}
	}
	nothing := strings.Repeat("0", 32)
	for _, method := range []string{http.MethodPatch, http.MethodDelete} {
		if response := writeJSON(t, routes, method, mark.Paths.Mark+"/"+nothing, bodies[method]); response.Code != http.StatusNotFound {
			t.Errorf("%s with an id nothing holds = %d, want 404", method, response.Code)
		}
	}
	if held := listed(t, store); len(held) != 1 || held[0].ID != kept.ID || !held[0].ResolvedAt.IsZero() {
		t.Errorf("a request for an id that is not the mark's changed %+v", held)
	}
	if response := writeJSON(t, routes, http.MethodDelete, mark.Paths.Mark+"/"+kept.ID, ""); response.Code != http.StatusOK {
		t.Errorf("DELETE of the real id = %d, want 200", response.Code)
	}
	if response := writeJSON(t, routes, http.MethodDelete, mark.Paths.Mark+"/"+kept.ID, ""); response.Code != http.StatusNotFound {
		t.Errorf("DELETE of what is removed = %d, want 404", response.Code)
	}
}

func TestMarkRouteTakesOneJSONObjectAndNothingElse(t *testing.T) {
	t.Parallel()

	store, _, routes := newMarkRoutes(t)
	valid := asJSON(t, aHighlightBody())
	withExtra := func(extra string) string { return strings.TrimSuffix(valid, "}") + extra + "}" }
	for name, tt := range map[string]struct {
		contentType, body string
		want              int
	}{
		"a form":                    {"application/x-www-form-urlencoded", "kind=highlight&path=Notes%2Fsource.md", http.StatusUnsupportedMediaType},
		"plain text":                {"text/plain", valid, http.StatusUnsupportedMediaType},
		"no type":                   {"", valid, http.StatusUnsupportedMediaType},
		"JSON by another name":      {"application/jsonp", valid, http.StatusUnsupportedMediaType},
		"a field nobody knows":      {"application/json", withExtra(`,"admin":true`), http.StatusBadRequest},
		"a second value":            {"application/json", valid + valid, http.StatusBadRequest},
		"words after the value":     {"application/json", valid + " x", http.StatusBadRequest},
		"half a value":              {"application/json", valid[:len(valid)-3], http.StatusBadRequest},
		"nothing":                   {"application/json", "", http.StatusBadRequest},
		"a number":                  {"application/json", "7", http.StatusBadRequest},
		"a field of the wrong type": {"application/json", `{"kind":7}`, http.StatusBadRequest},
		"a body past the cap":       {"application/json", `{"body":"` + strings.Repeat("a", 65<<10) + `"}`, http.StatusRequestEntityTooLarge},
	} {
		if got := write(t, routes, http.MethodPost, mark.Paths.Mark, tt.contentType, tt.body); got.Code != tt.want {
			t.Errorf("%s = %d, %q; want %d", name, got.Code, got.Body.String(), tt.want)
		}
	}
	if store.Stored() {
		t.Error("a request that was not one JSON object left a file")
	}
	// A type with a parameter is still JSON.
	if got := write(t, routes, http.MethodPost, mark.Paths.Mark, "application/json; charset=utf-8", valid); got.Code != http.StatusCreated {
		t.Errorf("JSON with a charset = %d, want 201", got.Code)
	}
}

func TestMarkRouteSpeaksTheReadersLanguageWhenItRefuses(t *testing.T) {
	t.Parallel()

	_, _, routes := newMarkRoutes(t)
	for lang, want := range map[string]string{"en": "That mark could not be kept.", "zh-Hant": "這則標記無法保留。"} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, mark.Paths.Mark, strings.NewReader(`{"kind":"nothing"}`))
		request.Header.Set("Content-Type", "application/json")
		request.AddCookie(&http.Cookie{Name: "yomihon_lang", Value: lang, Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode})
		response := httptest.NewRecorder()
		routes.ServeHTTP(response, request)
		if got := strings.TrimSpace(response.Body.String()); got != want {
			t.Errorf("%s: answered %q, want %q", lang, got, want)
		}
	}
}

func TestMarkRouteLeavesAFileItCannotReadAsItIs(t *testing.T) {
	t.Parallel()

	store, _, routes := newMarkRoutes(t)
	writeFile(t, marksPath(store), "{")
	id := strings.Repeat("a", 32)
	for name, response := range map[string]*httptest.ResponseRecorder{
		"POST":   writeJSON(t, routes, http.MethodPost, mark.Paths.Mark, asJSON(t, aHighlightBody())),
		"PATCH":  writeJSON(t, routes, http.MethodPatch, mark.Paths.Mark+"/"+id, `{"resolved":true}`),
		"DELETE": writeJSON(t, routes, http.MethodDelete, mark.Paths.Mark+"/"+id, ""),
	} {
		if response.Code != http.StatusInternalServerError {
			t.Errorf("%s over a damaged file = %d, %q; want 500", name, response.Code, response.Body.String())
		}
	}
	if data := contents(t, marksPath(store)); data != "{" {
		t.Errorf("the damaged file became %q", data)
	}
}

// treeOf is every file and directory under dir, with each file's bytes.
func treeOf(t *testing.T, dir string) map[string]string {
	t.Helper()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	})
	tree := map[string]string{}
	err = fs.WalkDir(root.FS(), ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			tree[path+"/"] = ""
			return nil
		}
		data, err := fs.ReadFile(root.FS(), path)
		tree[path] = string(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

// TestMarkRouteWritesNothingIntoTheFolderBeingRead is promise 1 for these
// routes: a keep, a change, a removal and a putting back leave every byte of
// the vault as it was, and the marks are under the configuration directory.
func TestMarkRouteWritesNothingIntoTheFolderBeingRead(t *testing.T) {
	t.Parallel()

	vaultRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(vaultRoot, "source.md"), []byte(sourceText), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(vaultRoot, "Notes"), 0o700); err != nil {
		t.Fatal(err)
	}
	configDir := t.TempDir()
	store := newStoreFor(t, configDir, vaultRoot)
	mux := http.NewServeMux()
	mark.NewHandler(store, aVault{"Notes/source.md": sourceText}, slog.New(slog.DiscardHandler)).Register(mux)
	before := treeOf(t, vaultRoot)

	kept := decodeAnswer(t, writeJSON(t, mux, http.MethodPost, mark.Paths.Mark, asJSON(t, aHighlightBody())))
	writeJSON(t, mux, http.MethodPost, mark.Paths.Mark, asJSON(t, map[string]any{"kind": "question", "path": "Notes/source.md", "anchor": "section-one"}))
	writeJSON(t, mux, http.MethodPatch, mark.Paths.Mark+"/"+kept.ID, `{}`)
	removed := writeJSON(t, mux, http.MethodDelete, mark.Paths.Mark+"/"+kept.ID, "")
	writeJSON(t, mux, http.MethodPut, mark.Paths.Mark+"/"+kept.ID, removed.Body.String())

	if diff := cmp.Diff(before, treeOf(t, vaultRoot)); diff != "" {
		t.Errorf("the routes changed the folder being read (-before +after):\n%s", diff)
	}
	if held, err := store.List(); err != nil || len(held) != 2 {
		t.Errorf("%d marks are kept, %v; want the two the routes made, kept under %s", len(held), err, configDir)
	}
	if !strings.HasPrefix(store.Dir(), configDir) {
		t.Errorf("the marks are kept in %s, want under %s", store.Dir(), configDir)
	}
}

func TestOnlyTheMethodsOfTheMarkRoutesChangeAMark(t *testing.T) {
	t.Parallel()

	store, _, routes := newMarkRoutes(t)
	kept := decodeAnswer(t, writeJSON(t, routes, http.MethodPost, mark.Paths.Mark, asJSON(t, aHighlightBody())))
	for _, tt := range []struct{ method, target string }{
		{http.MethodGet, mark.Paths.Mark},
		{http.MethodPut, mark.Paths.Mark},
		{http.MethodDelete, mark.Paths.Mark},
		{http.MethodPost, mark.Paths.Mark + "/" + kept.ID},
		{http.MethodGet, mark.Paths.Mark + "/" + kept.ID},
	} {
		if got := writeJSON(t, routes, tt.method, tt.target, asJSON(t, aHighlightBody())); got.Code != http.StatusMethodNotAllowed && got.Code != http.StatusNotFound {
			t.Errorf("%s %s = %d, want it refused", tt.method, tt.target, got.Code)
		}
	}
	if held := listed(t, store); len(held) != 1 {
		t.Errorf("%d marks are kept, want the one", len(held))
	}
}
