package mark_test

import (
	"context"
	"encoding/json"
	"io/fs"
	"log/slog"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/mark"
	"github.com/koopa0/yomihon/internal/mark/confdir"
	"github.com/koopa0/yomihon/internal/mark/machinemarks"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

const (
	mkNote   = "Notes/source.md"
	mkSecret = "Notes/secret.md"
	mkText   = "In Japanese, the topic particle marks what the sentence is about. A second sentence follows. The topic particle again."
	mkHash   = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
)

// mkNotes are the notes a route can place a mark in, each with its one place
// "section-one" and the note as a whole.
type mkNotes map[string]string

func (n mkNotes) HasPlace(rel, anchor string) bool {
	_, ok := n[rel]
	return ok && (anchor == "" || anchor == "section-one")
}

func (n mkNotes) Reading(rel string, _ wording.Lang) (mark.Page, bool) {
	text, ok := n[rel]
	return mark.Page{Text: text, Identity: mkHash}, ok
}

// mkRigNotes is the notes of a rig, which can make something happen as one is read.
type mkRigNotes struct {
	mkNotes

	rig *mkRig
}

func (n *mkRigNotes) Reading(rel string, lang wording.Lang) (mark.Page, bool) {
	n.rig.mu.Lock()
	hook := n.rig.onReading
	n.rig.onReading = nil
	n.rig.mu.Unlock()
	if hook != nil {
		hook()
	}
	return n.mkNotes.Reading(rel, lang)
}

// Note is the margin note a page would draw for a mark: here only enough to tell
// a test that the route asked for one, in the language it was asked in.
func (n *mkRigNotes) Note(_ context.Context, m *machinemarks.Mark, placement string, lang wording.Lang) string {
	return "<aside data-kind=\"" + string(m.Kind) + "\" data-placement=\"" + placement + "\" lang=\"" + lang.Tag() + "\"></aside>"
}

func (n mkNotes) Units(string) (mainLine, branches []string, ok bool) { return nil, nil, false }

func (n mkNotes) Rows(string) (planned int, rows []string, ok bool) { return 0, nil, false }

// mkRig is one computer: a vault folder, its own configuration directory, and the
// routes over them. hidden is the path a gate that refuses it would hide, if any;
// the reading server passes no such gate, and the tests that set one hold the
// engine's.
type mkRig struct {
	t      *testing.T
	root   string
	conf   string
	store  *mark.Store
	kept   *mark.Kept
	mux    *http.ServeMux
	mu     sync.Mutex
	hidden string
	// onReading, when set, is called once, the next time a note is read to place a
	// mark: a write reads it after it has been let in and before it records
	// anything, which is the moment a test makes the vault refuse the file.
	onReading func()
}

func newMkRig(t *testing.T) *mkRig {
	t.Helper()
	r := &mkRig{t: t, root: t.TempDir(), conf: t.TempDir()}
	if err := os.MkdirAll(filepath.Join(r.root, "Notes"), 0o750); err != nil {
		t.Fatal(err)
	}
	r.open()
	return r
}

// open is a new process over the same folders.
func (r *mkRig) open() {
	r.t.Helper()
	reader, err := vault.Open(r.root)
	if err != nil {
		r.t.Fatal(err)
	}
	r.t.Cleanup(func() { _ = reader.Close() }) //nolint:errcheck // the test is over
	store, err := mark.Open(confdir.At(r.conf), reader.Name())
	if err != nil {
		r.t.Fatal(err)
	}
	gate := func(m *machinemarks.Mark) bool {
		r.mu.Lock()
		defer r.mu.Unlock()
		return r.hidden == "" || m.Path != r.hidden
	}
	r.store = store
	r.kept = mark.OpenKept(store, reader, slog.New(slog.DiscardHandler), gate)
	r.t.Cleanup(func() { _ = r.kept.Close() }) //nolint:errcheck // the test is over
	r.mux = http.NewServeMux()
	notes := &mkRigNotes{mkNotes: mkNotes{mkNote: mkText, mkSecret: mkText}, rig: r}
	mark.NewHandler(store, r.kept, notes, slog.New(slog.DiscardHandler)).Register(r.mux)
}

func (r *mkRig) withhold(path string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hidden = path
}

// do sends one JSON request, in the language asked for when there is one.
func (r *mkRig) do(method, target, body string, lang ...string) *httptest.ResponseRecorder {
	r.t.Helper()
	request := httptest.NewRequestWithContext(r.t.Context(), method, target, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if len(lang) > 0 {
		request.Header.Set("Accept-Language", lang[0])
	}
	recorder := httptest.NewRecorder()
	r.mux.ServeHTTP(recorder, request)
	return recorder
}

// press sends the form the question button posts.
func (r *mkRig) press(anchor string) *httptest.ResponseRecorder {
	r.t.Helper()
	request := httptest.NewRequestWithContext(r.t.Context(), http.MethodPost, "/questions", strings.NewReader("path="+mkNote+"&anchor="+anchor))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()
	r.mux.ServeHTTP(recorder, request)
	return recorder
}

// writeEarlier leaves marks.json as the earlier version kept it.
func (r *mkRig) writeEarlier(content string) {
	r.t.Helper()
	if err := os.MkdirAll(r.store.Dir(), 0o750); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r.store.Dir(), "marks.json"), []byte(content), 0o600); err != nil {
		r.t.Fatal(err)
	}
}

// mkAnswer is what the routes answer with for one mark.
type mkAnswer struct {
	machinemarks.Mark

	Placement string            `json:"placement"`
	Seen      machinemarks.Seen `json:"seen"`
}

// mkConflict is what a 409 says.
type mkConflict struct {
	Reason string            `json:"reason"`
	Seen   machinemarks.Seen `json:"seen"`
}

func mkDecode[T any](t *testing.T, response *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(response.Body.Bytes(), &out); err != nil {
		t.Fatalf("the answer %q is not what was expected: %v", response.Body.String(), err)
	}
	return out
}

func mkStatus(t *testing.T, response *httptest.ResponseRecorder, want int) {
	t.Helper()
	if response.Code != want {
		t.Fatalf("status = %d, want %d; body %q", response.Code, want, response.Body.String())
	}
}

// mkSeenJSON is a seen as a request carries it.
func mkSeenJSON(t *testing.T, seen machinemarks.Seen) string {
	t.Helper()
	b, err := json.Marshal(seen)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// mkThought keeps a thought and answers it.
func (r *mkRig) mkThought(body string) mkAnswer {
	r.t.Helper()
	response := r.do(http.MethodPost, "/mark", `{"kind":"thought","path":"`+mkNote+`","anchor":"section-one","body":`+mkQuote(body)+`}`)
	mkStatus(r.t, response, http.StatusCreated)
	return mkDecode[mkAnswer](r.t, response)
}

func mkQuote(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// mkLines is the lines of every machine file of the vault.
func (r *mkRig) mkLines() []string {
	r.t.Helper()
	var out []string
	files, err := filepath.Glob(filepath.Join(r.root, "reader-notes", ".yomihon", "marks", "*.jsonl"))
	if err != nil {
		r.t.Fatal(err)
	}
	for _, f := range files {
		data, readErr := os.ReadFile(f) // #nosec G304 -- a test's own path
		if readErr != nil {
			r.t.Fatal(readErr)
		}
		for line := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
			if line != "" {
				out = append(out, line)
			}
		}
	}
	return out
}

// mkTree is every file under dir with what it holds.
func mkTree(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p) //nolint:errcheck // p is under dir
		if d.Type()&fs.ModeSymlink != 0 {
			target, linkErr := os.Readlink(p)
			out[rel] = "-> " + target
			return linkErr
		}
		if d.IsDir() {
			out[rel+"/"] = ""
			return nil
		}
		b, readErr := os.ReadFile(p) // #nosec G304 G122 -- a test's own path
		out[rel] = string(b)
		return readErr
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// POST /mark keeps each kind, answers it with where it stands and what the page
// has read, keeps a highlight once, and every thought as new.
func TestMkPostKeepsEachKindAndAnswersWithWhatThePageHasRead(t *testing.T) {
	t.Parallel()
	r := newMkRig(t)

	highlight := `{"kind":"highlight","path":"` + mkNote + `","anchor":"section-one","quote":"the topic particle marks","before":"In Japanese, ","after":" what the sentence"}`
	created := r.do(http.MethodPost, "/mark", highlight)
	mkStatus(t, created, http.StatusCreated)
	first := mkDecode[mkAnswer](t, created)
	if len(first.ID) != 32 || first.Kind != machinemarks.Highlight || first.Quote != "the topic particle marks" || first.Identity != mkHash {
		t.Fatalf("a highlight was answered %+v", first)
	}
	if first.Placement != "found" || len(first.Seen) != 1 {
		t.Fatalf("placement %q, seen %v; want found and the one line the page has read", first.Placement, first.Seen)
	}
	if got := len(r.mkLines()); got != 1 {
		t.Fatalf("the vault holds %d lines after one mark, want 1", got)
	}

	again := r.do(http.MethodPost, "/mark", highlight)
	mkStatus(t, again, http.StatusOK)
	if second := mkDecode[mkAnswer](t, again); second.ID != first.ID || !cmp.Equal(second.Seen, first.Seen) {
		t.Fatalf("the same highlight kept again was answered %+v, want the first %+v", second, first)
	}
	if got := len(r.mkLines()); got != 1 {
		t.Fatalf("a highlight kept twice wrote %d lines, want 1", got)
	}

	thought := `{"kind":"thought","path":"` + mkNote + `","anchor":"section-one","body":"why <b>this</b> particle?"}`
	one, two := r.do(http.MethodPost, "/mark", thought), r.do(http.MethodPost, "/mark", thought)
	mkStatus(t, one, http.StatusCreated)
	mkStatus(t, two, http.StatusCreated)
	a, b := mkDecode[mkAnswer](t, one), mkDecode[mkAnswer](t, two)
	if a.ID == b.ID || a.Body != "why <b>this</b> particle?" || a.Placement != "section" {
		t.Fatalf("two thoughts were %+v and %+v, want two marks with the words as sent", a, b)
	}

	// Asking again with other words is a new question, not an answer with the
	// earlier one and the new words dropped; asking again with the same ones is
	// the same question.
	asked := `{"kind":"question","path":"` + mkNote + `","anchor":"section-one","body":"why が?"}`
	mkStatus(t, r.do(http.MethodPost, "/mark", asked), http.StatusCreated)
	mkStatus(t, r.do(http.MethodPost, "/mark", asked), http.StatusOK)
	reasked := r.do(http.MethodPost, "/mark", `{"kind":"question","path":"`+mkNote+`","anchor":"section-one","body":"why not は?"}`)
	mkStatus(t, reasked, http.StatusCreated)
	if got := mkDecode[mkAnswer](t, reasked); got.Body != "why not は?" {
		t.Fatalf("a question asked again with other words was answered %+v, want the words sent", got)
	}
	section := r.do(http.MethodPost, "/mark", `{"kind":"question","path":"`+mkNote+`","anchor":"section-one"}`)
	mkStatus(t, section, http.StatusCreated)
	withWords := r.do(http.MethodPost, "/mark", `{"kind":"question","path":"`+mkNote+`","anchor":"section-one","quote":"the topic particle marks","body":"is it always が?"}`)
	mkStatus(t, withWords, http.StatusCreated)
	if got := len(r.mkLines()); got != 7 {
		t.Fatalf("the vault holds %d lines, want 7", got)
	}
}

// What the route will not keep, it says, in the reader's language, and writes
// nothing into the vault.
func TestMkPostRefusesWhatItCannotKeepAndWritesNothing(t *testing.T) {
	t.Parallel()
	r := newMkRig(t)
	for _, tt := range []struct {
		name, body string
		want       int
	}{
		{"a thought that says nothing", `{"kind":"thought","path":"` + mkNote + `","body":""}`, http.StatusUnprocessableEntity},
		{"a highlight with words of its own", `{"kind":"highlight","path":"` + mkNote + `","quote":"the topic particle","body":"mine"}`, http.StatusUnprocessableEntity},
		{"a highlight of nothing", `{"kind":"highlight","path":"` + mkNote + `"}`, http.StatusUnprocessableEntity},
		{"words the note never had", `{"kind":"highlight","path":"` + mkNote + `","quote":"not in the note"}`, http.StatusUnprocessableEntity},
		{"words at several places nothing tells apart", `{"kind":"highlight","path":"` + mkNote + `","quote":"topic particle"}`, http.StatusUnprocessableEntity},
		{"a note this vault does not hold", `{"kind":"question","path":"Notes/missing.md"}`, http.StatusUnprocessableEntity},
		{"an anchor the note does not render", `{"kind":"question","path":"` + mkNote + `","anchor":"nowhere"}`, http.StatusUnprocessableEntity},
		{"a kind nobody has", `{"kind":"bookmark","path":"` + mkNote + `"}`, http.StatusUnprocessableEntity},
		{"a path that leaves the vault", `{"kind":"question","path":"../x.md"}`, http.StatusUnprocessableEntity},
		{"a field nobody knows", `{"kind":"question","path":"` + mkNote + `","reason":"why"}`, http.StatusBadRequest},
		{"two values", `{"kind":"question","path":"` + mkNote + `"}{}`, http.StatusBadRequest},
		{"no JSON at all", `not json`, http.StatusBadRequest},
		{"a body past its cap", `{"kind":"thought","path":"` + mkNote + `","body":"` + strings.Repeat("x", 70<<10) + `"}`, http.StatusRequestEntityTooLarge},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			own := newMkRig(t)
			vaultBefore := mkTree(t, own.root)
			response := own.do(http.MethodPost, "/mark", tt.body)
			mkStatus(t, response, tt.want)
			if diff := cmp.Diff(vaultBefore, mkTree(t, own.root)); diff != "" {
				t.Errorf("a refused mark changed the vault (-before +after):\n%s", diff)
			}
		})
	}

	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mark", strings.NewReader("kind=question&path="+mkNote))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()
	r.mux.ServeHTTP(recorder, request)
	mkStatus(t, recorder, http.StatusUnsupportedMediaType)

	zh := r.do(http.MethodPost, "/mark", `{"kind":"thought","path":"`+mkNote+`","body":""}`, "zh-TW")
	en := r.do(http.MethodPost, "/mark", `{"kind":"thought","path":"`+mkNote+`","body":""}`, "en")
	if !strings.Contains(zh.Body.String(), wording.MarkNotKept.In(wording.ZhHant)) || !strings.Contains(en.Body.String(), wording.MarkNotKept.In(wording.En)) {
		t.Errorf("the refusals say %q and %q, want the reader's own language", zh.Body.String(), en.Body.String())
	}
}

// Where the reader folder cannot be written to, a request made by hand is told
// why and nothing is written.
func TestMkAWriteWhereTheReaderFolderCannotTakeItIsRefusedWithTheReason(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, want string
		build      func(t *testing.T, root string)
	}{
		{"a link", mark.WritableLink, func(t *testing.T, root string) {
			t.Helper()
			if err := os.Symlink("Notes", filepath.Join(root, "reader-notes")); err != nil {
				t.Skipf("no links here: %v", err)
			}
		}},
		{"a file", mark.WritableNotAFolder, func(t *testing.T, root string) {
			t.Helper()
			if err := os.WriteFile(filepath.Join(root, "reader-notes"), []byte("x"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newMkRig(t)
			tt.build(t, r.root)
			if got := r.kept.Writable(); got != tt.want {
				t.Fatalf("Writable() = %q, want %q", got, tt.want)
			}
			before := mkTree(t, r.root)
			for _, request := range []struct{ method, target, body string }{
				{http.MethodPost, "/mark", `{"kind":"question","path":"` + mkNote + `"}`},
				{http.MethodPatch, "/mark/" + strings.Repeat("a", 32), `{"seen":{},"body":"x"}`},
				{http.MethodDelete, "/mark/" + strings.Repeat("a", 32), `{"seen":{}}`},
				{http.MethodPut, "/mark/" + strings.Repeat("a", 32), `{"seen":{}}`},
			} {
				response := r.do(request.method, request.target, request.body, "en")
				mkStatus(t, response, http.StatusUnprocessableEntity)
				// The reader is told only that the mark could not be kept; why is the log's.
				if want := wording.MarkNotKept.In(wording.En); !strings.Contains(response.Body.String(), want) || strings.Contains(response.Body.String(), "reader-notes") {
					t.Errorf("%s %s said %q, want %q and no word of the folder", request.method, request.target, response.Body.String(), want)
				}
			}
			form := r.press("section-one")
			mkStatus(t, form, http.StatusUnprocessableEntity)
			if diff := cmp.Diff(before, mkTree(t, r.root)); diff != "" {
				t.Errorf("a refused write changed the vault:\n%s", diff)
			}
		})
	}
}

// GET /mark/{id} answers what a page needs to change the mark: the value, which is
// the newest live one where computers changed it differently, and the seen. A deleted mark is answered with
// deleted set, which is what undoing it starts from.
func TestMkGetAnswersTheMarkAndWhatThePageHasRead(t *testing.T) {
	t.Parallel()
	r := newMkRig(t)
	kept := r.mkThought("first words")
	got := r.do(http.MethodGet, "/mark/"+kept.ID, "")
	mkStatus(t, got, http.StatusOK)
	read := mkDecode[mkAnswer](t, got)
	if read.Body != "first words" || read.Placement != "section" || !cmp.Equal(read.Seen, kept.Seen) {
		t.Fatalf("GET answered %+v, want the mark, its placement, and the seen the keep answered", read)
	}
	if control := got.Header().Get("Cache-Control"); control != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store: a seen that was kept is a stale base", control)
	}

	deleted := r.do(http.MethodDelete, "/mark/"+kept.ID, `{"seen":`+mkSeenJSON(t, read.Seen)+`}`)
	mkStatus(t, deleted, http.StatusOK)
	after := mkDecode[mkAnswer](t, r.do(http.MethodGet, "/mark/"+kept.ID, ""))
	if !after.Deleted || after.Body != "first words" {
		t.Fatalf("a deleted mark was answered %+v, want deleted, with its words", after)
	}
}

// The reading server's own gate lets every mark through (the reader is the owner,
// on the loopback, reading their own marks), so this holds the engine's gate as
// a caller that is not the reader would set it: a mark the gate refuses is
// answered exactly as one that was never there, by
// every route that takes an id: the same status and the same bytes.
func TestMkAMarkAGateRefusesAnswersExactlyLikeAMissingOne(t *testing.T) {
	t.Parallel()
	r := newMkRig(t)
	response := r.do(http.MethodPost, "/mark", `{"kind":"thought","path":"`+mkSecret+`","body":"a private thought"}`)
	mkStatus(t, response, http.StatusCreated)
	secret := mkDecode[mkAnswer](t, response)
	r.withhold(mkSecret)

	missing := strings.Repeat("e", 32)
	body := `{"seen":` + mkSeenJSON(t, secret.Seen) + `,"body":"changed"}`
	for _, request := range []struct{ method, body string }{
		{http.MethodGet, ""},
		{http.MethodPatch, body},
		{http.MethodDelete, `{"seen":` + mkSeenJSON(t, secret.Seen) + `}`},
		{http.MethodPut, `{"seen":` + mkSeenJSON(t, secret.Seen) + `}`},
	} {
		for _, lang := range []string{"zh-TW", "en"} {
			held := r.do(request.method, "/mark/"+secret.ID, request.body, lang)
			absent := r.do(request.method, "/mark/"+missing, request.body, lang)
			if held.Code != http.StatusNotFound || held.Code != absent.Code || held.Body.String() != absent.Body.String() {
				t.Errorf("%s of a withheld mark answered %d %q, of a missing one %d %q; want the same", request.method, held.Code, held.Body.String(), absent.Code, absent.Body.String())
			}
			if strings.Contains(held.Body.String(), "private") || strings.Contains(held.Body.String(), secret.ID) {
				t.Errorf("%s of a withheld mark told something of it: %q", request.method, held.Body.String())
			}
		}
	}
	r.withhold("")
	if got := mkDecode[mkAnswer](t, r.do(http.MethodGet, "/mark/"+secret.ID, "")); got.Body != "a private thought" {
		t.Fatalf("a withheld mark was changed through the gate: it reads %q", got.Body)
	}
	if n := len(r.mkLines()); n != 1 {
		t.Fatalf("the vault holds %d lines, want the one thought and nothing written for the refused requests", n)
	}
}

// A malformed id is a missing mark, and nothing is read for it.
func TestMkAnIdThatIsNotOneIsAMissingMark(t *testing.T) {
	t.Parallel()
	r := newMkRig(t)
	for _, id := range []string{"x", strings.Repeat("A", 32), strings.Repeat("a", 31), strings.Repeat("g", 32), "..%2f..", strings.Repeat("a", 33)} {
		for _, method := range []string{http.MethodGet, http.MethodPatch, http.MethodDelete, http.MethodPut} {
			response := r.do(method, "/mark/"+id, `{"seen":{},"body":"x"}`)
			if response.Code != http.StatusNotFound {
				t.Errorf("%s /mark/%s = %d, want 404", method, id, response.Code)
			}
		}
	}
	if _, err := os.Stat(filepath.Join(r.root, "reader-notes")); err == nil {
		t.Error("asking after marks that are not there made the reader folder")
	}
}

// PATCH changes the words of a thought or a question, resolves or reopens a
// question, and answers with the new seen, which the next change is made on top of.
func TestMkPatchChangesWordsAndResolvesAndAnswersTheNextBase(t *testing.T) {
	t.Parallel()
	r := newMkRig(t)
	thought := r.mkThought("first")
	patched := r.do(http.MethodPatch, "/mark/"+thought.ID, `{"seen":`+mkSeenJSON(t, thought.Seen)+`,"body":"second"}`)
	mkStatus(t, patched, http.StatusOK)
	second := mkDecode[mkAnswer](t, patched)
	if second.Body != "second" || second.ID != thought.ID || !second.At.Equal(thought.At) || second.Seen[firstKey(second.Seen)] <= thought.Seen[firstKey(thought.Seen)] {
		t.Fatalf("PATCH answered %+v, want the same mark with new words, made at the same time", second)
	}
	third := r.do(http.MethodPatch, "/mark/"+thought.ID, `{"seen":`+mkSeenJSON(t, second.Seen)+`,"body":"third"}`)
	mkStatus(t, third, http.StatusOK)

	question := mkDecode[mkAnswer](t, r.do(http.MethodPost, "/mark", `{"kind":"question","path":"`+mkNote+`","anchor":"section-one"}`))
	resolved := r.do(http.MethodPatch, "/mark/"+question.ID, `{"seen":`+mkSeenJSON(t, question.Seen)+`,"resolved":true,"body":"it was が"}`)
	mkStatus(t, resolved, http.StatusOK)
	done := mkDecode[mkAnswer](t, resolved)
	if done.Resolved.IsZero() || done.Body != "it was が" {
		t.Fatalf("a resolved question was answered %+v", done)
	}
	again := mkDecode[mkAnswer](t, r.do(http.MethodPatch, "/mark/"+question.ID, `{"seen":`+mkSeenJSON(t, done.Seen)+`,"resolved":true}`))
	if !again.Resolved.Equal(done.Resolved) {
		t.Fatalf("resolving again moved the time from %v to %v", done.Resolved, again.Resolved)
	}
	reopened := mkDecode[mkAnswer](t, r.do(http.MethodPatch, "/mark/"+question.ID, `{"seen":`+mkSeenJSON(t, again.Seen)+`,"resolved":false}`))
	if !reopened.Resolved.IsZero() || reopened.Body != "it was が" {
		t.Fatalf("a reopened question was answered %+v, want it open with its words", reopened)
	}
}

// firstKey is a key of a seen, for comparing the one writer a test has.
func firstKey(seen machinemarks.Seen) string {
	for k := range seen {
		return k
	}
	return ""
}

// What a change may not be, it is refused as 422 and writes nothing.
func TestMkPatchRefusesWhatTheKindDoesNotHaveAndWritesNothing(t *testing.T) {
	t.Parallel()
	r := newMkRig(t)
	thought := r.mkThought("words")
	highlight := mkDecode[mkAnswer](t, r.do(http.MethodPost, "/mark", `{"kind":"highlight","path":"`+mkNote+`","quote":"the topic particle marks"}`))
	for _, tt := range []struct {
		name string
		id   string
		seen machinemarks.Seen
		body string
	}{
		{"a thought resolved", thought.ID, thought.Seen, `"resolved":true`},
		{"a thought with no words", thought.ID, thought.Seen, `"body":""`},
		{"a highlight with words", highlight.ID, highlight.Seen, `"body":"mine"`},
		{"a highlight resolved", highlight.ID, highlight.Seen, `"resolved":true`},
		{"nothing to change", thought.ID, thought.Seen, `"edit":null`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel() // each of these refuses and writes nothing, so they may share one vault
			before := mkTree(t, r.root)
			response := r.do(http.MethodPatch, "/mark/"+tt.id, `{"seen":`+mkSeenJSON(t, tt.seen)+","+tt.body+`}`)
			mkStatus(t, response, http.StatusUnprocessableEntity)
			if diff := cmp.Diff(before, mkTree(t, r.root)); diff != "" {
				t.Errorf("a refused change wrote:\n%s", diff)
			}
		})
	}
	// A body with fields nobody knows, and no seen at all.
	mkStatus(t, r.do(http.MethodPatch, "/mark/"+thought.ID, `{"seen":{},"reason":"x"}`), http.StatusBadRequest)
}

// Every refusal that has a reason says it, and the page tells them apart by it.
func TestMkEveryRefusalHasItsReason(t *testing.T) {
	t.Parallel()
	r := newMkRig(t)
	thought := r.mkThought("first")
	seen0 := thought.Seen

	// A change on a page that read nothing of the mark is no change at all.
	noBase := r.do(http.MethodPatch, "/mark/"+thought.ID, `{"body":"x"}`)
	mkStatus(t, noBase, http.StatusBadRequest)
	nullBase := r.do(http.MethodPatch, "/mark/"+thought.ID, `{"seen":null,"body":"x"}`)
	mkStatus(t, nullBase, http.StatusBadRequest)
	mkStatus(t, r.do(http.MethodDelete, "/mark/"+thought.ID, `{}`), http.StatusBadRequest)
	mkStatus(t, r.do(http.MethodPut, "/mark/"+thought.ID, `{}`), http.StatusBadRequest)

	// Another tab changes the mark; this page, which is old, is stale.
	changed := mkDecode[mkAnswer](t, r.do(http.MethodPatch, "/mark/"+thought.ID, `{"seen":`+mkSeenJSON(t, seen0)+`,"body":"another tab"}`))
	for _, request := range []struct{ method, body string }{
		{http.MethodPatch, `{"seen":` + mkSeenJSON(t, seen0) + `,"body":"old page"}`},
		{http.MethodDelete, `{"seen":` + mkSeenJSON(t, seen0) + `}`},
	} {
		response := r.do(request.method, "/mark/"+thought.ID, request.body)
		mkStatus(t, response, http.StatusConflict)
		got := mkDecode[mkConflict](t, response)
		if got.Reason != "stale" || !cmp.Equal(got.Seen, changed.Seen) {
			t.Fatalf("%s from an old page said %+v, want stale with what is read now %v", request.method, got, changed.Seen)
		}
	}
	if got := mkDecode[mkAnswer](t, r.do(http.MethodGet, "/mark/"+thought.ID, "")); got.Body != "another tab" {
		t.Fatalf("a stale page changed the mark: it reads %q", got.Body)
	}

	// A late send of an edit that a newer send of the same edit is written over.
	mkStatus(t, r.do(http.MethodPatch, "/mark/"+thought.ID, `{"seen":`+mkSeenJSON(t, changed.Seen)+`,"edit":{"id":"tab-1","seq":2},"body":"newest words"}`), http.StatusOK)
	late := r.do(http.MethodPatch, "/mark/"+thought.ID, `{"seen":`+mkSeenJSON(t, changed.Seen)+`,"edit":{"id":"tab-1","seq":1},"body":"older words"}`)
	mkStatus(t, late, http.StatusConflict)
	if got := mkDecode[mkConflict](t, late); got.Reason != "superseded" || len(got.Seen) != 0 {
		t.Fatalf("a late send said %+v, want superseded and nothing else", got)
	}
	if got := mkDecode[mkAnswer](t, r.do(http.MethodGet, "/mark/"+thought.ID, "")); got.Body != "newest words" {
		t.Fatalf("a late send wrote: the mark reads %q", got.Body)
	}
	// The last words of an edit, sent after the page was left, go through.
	last := r.do(http.MethodPatch, "/mark/"+thought.ID, `{"seen":`+mkSeenJSON(t, changed.Seen)+`,"edit":{"id":"tab-1","seq":3},"body":"last words"}`)
	mkStatus(t, last, http.StatusOK)

	// Deleting what is deleted already, and changing it.
	now := mkDecode[mkAnswer](t, last)
	deleted := r.do(http.MethodDelete, "/mark/"+thought.ID, `{"seen":`+mkSeenJSON(t, now.Seen)+`}`)
	mkStatus(t, deleted, http.StatusOK)
	for _, request := range []struct{ method, body string }{
		{http.MethodDelete, `{"seen":` + mkSeenJSON(t, now.Seen) + `}`},
		{http.MethodPatch, `{"seen":` + mkSeenJSON(t, now.Seen) + `,"body":"into a deleted mark"}`},
	} {
		response := r.do(request.method, "/mark/"+thought.ID, request.body)
		mkStatus(t, response, http.StatusConflict)
		if got := mkDecode[mkConflict](t, response); got.Reason != "gone" {
			t.Fatalf("%s of a deleted mark said %+v, want gone", request.method, got)
		}
	}
	mkStatus(t, r.do(http.MethodPatch, "/mark/"+strings.Repeat("c", 32), `{"seen":{},"body":"x"}`), http.StatusNotFound)
}

// "Use my words": a page that was told it is stale writes on top of what it was
// told, which replaces the other tab's words and nothing else.
func TestMkAStalePageMayWriteOnTopOfWhatItWasTold(t *testing.T) {
	t.Parallel()
	r := newMkRig(t)
	thought := r.mkThought("first")
	r.do(http.MethodPatch, "/mark/"+thought.ID, `{"seen":`+mkSeenJSON(t, thought.Seen)+`,"body":"other tab"}`)
	old := r.do(http.MethodPatch, "/mark/"+thought.ID, `{"seen":`+mkSeenJSON(t, thought.Seen)+`,"body":"mine"}`)
	mkStatus(t, old, http.StatusConflict)
	told := mkDecode[mkConflict](t, old)
	mine := r.do(http.MethodPatch, "/mark/"+thought.ID, `{"seen":`+mkSeenJSON(t, told.Seen)+`,"body":"mine"}`)
	mkStatus(t, mine, http.StatusOK)
	got := mkDecode[mkAnswer](t, r.do(http.MethodGet, "/mark/"+thought.ID, ""))
	if got.Body != "mine" {
		t.Fatalf("after use my words the mark reads %+v", got)
	}
}

// DELETE keeps the words in the mark's line, and PUT puts the mark back from them.
func TestMkDeleteAndPutBackRoundTripTheWords(t *testing.T) {
	t.Parallel()
	r := newMkRig(t)
	thought := r.mkThought("words that matter")
	deleted := r.do(http.MethodDelete, "/mark/"+thought.ID, `{"seen":`+mkSeenJSON(t, thought.Seen)+`}`)
	mkStatus(t, deleted, http.StatusOK)
	gone := mkDecode[mkAnswer](t, deleted)
	if !gone.Deleted || gone.Body != "words that matter" {
		t.Fatalf("a deleted mark was answered %+v, want deleted with its words", gone)
	}
	// An old page cannot put it back over the delete it had not read.
	stale := r.do(http.MethodPut, "/mark/"+thought.ID, `{"seen":`+mkSeenJSON(t, thought.Seen)+`}`)
	mkStatus(t, stale, http.StatusConflict)
	if got := mkDecode[mkConflict](t, stale); got.Reason != "stale" {
		t.Fatalf("an old page's undo said %+v, want stale", got)
	}
	back := r.do(http.MethodPut, "/mark/"+thought.ID, `{"seen":`+mkSeenJSON(t, gone.Seen)+`}`)
	mkStatus(t, back, http.StatusOK)
	restored := mkDecode[mkAnswer](t, back)
	if restored.Deleted || restored.Body != "words that matter" || !restored.At.Equal(thought.At) || restored.ID != thought.ID {
		t.Fatalf("a mark put back was answered %+v, want the mark as it was", restored)
	}
	// Putting back what is not deleted answers it and writes nothing.
	lines := len(r.mkLines())
	again := r.do(http.MethodPut, "/mark/"+thought.ID, `{"seen":`+mkSeenJSON(t, restored.Seen)+`}`)
	mkStatus(t, again, http.StatusOK)
	if got := len(r.mkLines()); got != lines {
		t.Fatalf("putting back a mark that was not deleted wrote %d lines", got-lines)
	}
}

// A highlight is deleted and put back too, and a mark deleted by one writer is
// never brought back by a page that had not read the deletion.
func TestMkADeletedMarkStaysDeletedForAPageThatHadNotReadTheDeletion(t *testing.T) {
	t.Parallel()
	r := newMkRig(t)
	kept := mkDecode[mkAnswer](t, r.do(http.MethodPost, "/mark", `{"kind":"highlight","path":"`+mkNote+`","quote":"the topic particle marks"}`))
	mkStatus(t, r.do(http.MethodDelete, "/mark/"+kept.ID, `{"seen":`+mkSeenJSON(t, kept.Seen)+`}`), http.StatusOK)
	if got := mkDecode[mkAnswer](t, r.do(http.MethodGet, "/mark/"+kept.ID, "")); !got.Deleted {
		t.Fatal("a deleted highlight reads as there")
	}
	// Keeping the same highlight again is a new mark: the deleted one is not "the same".
	again := r.do(http.MethodPost, "/mark", `{"kind":"highlight","path":"`+mkNote+`","quote":"the topic particle marks"}`)
	mkStatus(t, again, http.StatusCreated)
	if got := mkDecode[mkAnswer](t, again); got.ID == kept.ID {
		t.Fatal("a highlight kept after its delete was answered as the deleted one")
	}
}

// The two-tab scenario, over HTTP: a tab edits, another computer's edit arrives
// through git, and the tab sends again. The reader is shown the newest live value
// and is told nothing of any other: no answer of a route has a field for one. The
// other computer's words are not dropped: no send of a page that has not read a
// line replaces it, and both values stay in the files of the vault.
func TestMkAForeignEditThatGitBringsInSurvivesTwoSendsAndShowsTheNewest(t *testing.T) {
	t.Parallel()
	a, b := newMkRig(t), newMkRig(t)

	thought := a.mkThought("the first words")
	mkPull(t, a, b)
	// B's page reads the mark and edits it: B has read A's line, so its edit
	// replaces it.
	read := mkDecode[mkAnswer](t, b.do(http.MethodGet, "/mark/"+thought.ID, ""))
	mkStatus(t, b.do(http.MethodPatch, "/mark/"+thought.ID, `{"seen":`+mkSeenJSON(t, read.Seen)+`,"body":"B's words, written elsewhere"}`), http.StatusOK)

	// A's tab, drawn before B wrote, sends its edit, and gets what it has read.
	sent := a.do(http.MethodPatch, "/mark/"+thought.ID, `{"seen":`+mkSeenJSON(t, thought.Seen)+`,"edit":{"id":"tab","seq":1},"body":"A's words"}`)
	mkStatus(t, sent, http.StatusOK)
	first := mkDecode[mkAnswer](t, sent)

	// git pull brings B's file in.
	mkPull(t, b, a)

	// The same tab sends again, on top of what the first send answered.
	second := a.do(http.MethodPatch, "/mark/"+thought.ID, `{"seen":`+mkSeenJSON(t, first.Seen)+`,"edit":{"id":"tab","seq":2},"body":"A's words, a little longer"}`)
	mkStatus(t, second, http.StatusOK)

	// Every answer says the one value and no word of another.
	fields := func(response *httptest.ResponseRecorder) map[string]json.RawMessage {
		t.Helper()
		var got map[string]json.RawMessage
		if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
			t.Fatalf("the answer %q is not an object: %v", response.Body.String(), err)
		}
		return got
	}
	want := slices.Sorted(maps.Keys(fields(a.do(http.MethodGet, "/mark/"+mkOther(t, a).ID, ""))))
	for name, response := range map[string]*httptest.ResponseRecorder{
		"the send that left B's words beside A's": second,
		"A's GET": a.do(http.MethodGet, "/mark/"+thought.ID, ""),
	} {
		if got := slices.Sorted(maps.Keys(fields(response))); !slices.Equal(got, want) || strings.Contains(strings.ToLower(response.Body.String()), "conflict") {
			t.Errorf("%s answered %s, want the fields of a mark that has no other value (%v) and no word of one", name, response.Body.String(), want)
		}
	}

	// The value shown is the newest live one: A's second send is the later write.
	if got := mkDecode[mkAnswer](t, a.do(http.MethodGet, "/mark/"+thought.ID, "")); got.Body != "A's words, a little longer" {
		t.Fatalf("A reads %q, want the newest live value, A's second send", got.Body)
	}
	// B reads the same value once A's file comes the other way.
	mkPull(t, a, b)
	if got := mkDecode[mkAnswer](t, b.do(http.MethodGet, "/mark/"+thought.ID, "")); got.Body != "A's words, a little longer" {
		t.Fatalf("B reads %q, want the same newest live value", got.Body)
	}

	// Neither send replaced the other computer's value: both are still values of
	// the mark in the data, though only the newest is shown.
	view, err := a.kept.View()
	if err != nil {
		t.Fatal(err)
	}
	if entry, ok := view.Entry(thought.ID, mark.AllMarks); !ok || len(entry.Values) != 2 {
		t.Fatalf("the mark holds %+v, want both computers' values kept in the data", entry)
	}
	// Both values are still in the files of the vault, each whole.
	lines := strings.Join(a.mkLines(), "\n")
	for _, words := range []string{"A's words, a little longer", "B's words, written elsewhere"} {
		if !strings.Contains(lines, words) {
			t.Errorf("the machine files of the vault no longer hold %q:\n%s", words, lines)
		}
	}
}

// mkOther keeps a mark that nothing else touches, to know what a mark's answer
// has in it when there is nothing but the one value.
func mkOther(t *testing.T, r *mkRig) mkAnswer {
	t.Helper()
	return r.mkThought("a thought with one value")
}

// mkPull stands for git: the machine file of one computer is copied over the
// copy the other has.
func mkPull(t *testing.T, from, to *mkRig) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(from.root, "reader-notes", ".yomihon", "marks", "*.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(filepath.Join(to.root, "reader-notes", ".yomihon", "marks"), 0o750); err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		data, readErr := os.ReadFile(f) // #nosec G304 -- a test's own path
		if readErr != nil {
			t.Fatal(readErr)
		}
		if err = os.WriteFile(filepath.Join(to.root, "reader-notes", ".yomihon", "marks", filepath.Base(f)), data, 0o600); err != nil { // #nosec G703 -- a test's own path
			t.Fatal(err)
		}
	}
}

// The question button's route keeps its wire: the same list, the same toggle,
// and a section that two marked is not taken away blind.
func TestMkQuestionsListAndToggleAsTheButtonExpects(t *testing.T) {
	t.Parallel()
	r := newMkRig(t)
	list := func() []map[string]any {
		t.Helper()
		response := r.do(http.MethodGet, "/questions", "")
		mkStatus(t, response, http.StatusOK)
		return mkDecode[[]map[string]any](t, response)
	}
	if got := list(); got == nil || len(got) != 0 {
		t.Fatalf("no questions listed as %v, want an empty list and not null", got)
	}
	marked := r.press("section-one")
	mkStatus(t, marked, http.StatusOK)
	if got := mkDecode[map[string]bool](t, marked); !got["marked"] {
		t.Fatalf("the first press answered %v, want marked", got)
	}
	got := list()
	if len(got) != 1 || got[0]["path"] != mkNote || got[0]["anchor"] != "section-one" || got[0]["time"] == nil {
		t.Fatalf("the list is %v, want the section with its time", got)
	}
	// A question about a passage is not a mark of the section.
	mkStatus(t, r.do(http.MethodPost, "/mark", `{"kind":"question","path":"`+mkNote+`","anchor":"section-one","quote":"the topic particle marks"}`), http.StatusCreated)
	if len(list()) != 1 {
		t.Fatalf("a question about a passage is listed as a section's: %v", list())
	}
	unmarked := r.press("section-one")
	mkStatus(t, unmarked, http.StatusOK)
	if got := mkDecode[map[string]bool](t, unmarked); got["marked"] {
		t.Fatalf("the second press answered %v, want unmarked", got)
	}
	if len(list()) != 0 {
		t.Fatalf("a question taken away is still listed: %v", list())
	}
	// The note as a whole is a place too, kept and taken away by the same press.
	whole := r.press("")
	mkStatus(t, whole, http.StatusOK)
	if got := mkDecode[map[string]bool](t, whole); !got["marked"] || len(list()) != 1 || list()[0]["anchor"] != "" {
		t.Fatalf("the note as a whole was answered %v and listed as %v, want it marked", got, list())
	}
	mkStatus(t, r.press(""), http.StatusOK)
	if r.do(http.MethodPost, "/questions", "{}").Code != http.StatusBadRequest && r.do(http.MethodPost, "/questions", "{}").Code != http.StatusUnprocessableEntity {
		t.Error("a JSON body to the form route was taken")
	}
}

// Taking a question away is allowed where the note is gone; making one is not.
func TestMkAQuestionOfANoteThatIsGoneCanStillBeTakenAway(t *testing.T) {
	t.Parallel()
	r := newMkRig(t)
	mkStatus(t, r.press("section-one"), http.StatusOK)
	// A new process whose vault has lost the note.
	reader, err := vault.Open(r.root)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close() //nolint:errcheck // the test is over
	store, err := mark.Open(confdir.At(r.conf), reader.Name())
	if err != nil {
		t.Fatal(err)
	}
	kept := mark.OpenKept(store, reader, slog.New(slog.DiscardHandler), mark.AllMarks)
	defer kept.Close() //nolint:errcheck // the test is over
	mux := http.NewServeMux()
	mark.NewHandler(store, kept, mkNotes{}, slog.New(slog.DiscardHandler)).Register(mux)
	send := func(body string) *httptest.ResponseRecorder {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/questions", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, request)
		return recorder
	}
	gone := send("path=" + mkNote + "&anchor=section-one")
	mkStatus(t, gone, http.StatusOK)
	if got := mkDecode[map[string]bool](t, gone); got["marked"] {
		t.Fatalf("pressing again on a question of a vanished note answered %v, want it taken away", got)
	}
	mkStatus(t, send("path=Notes/other.md&anchor="), http.StatusUnprocessableEntity)
}

// A question two computers each marked before either had read the other's is two
// marks; pressing takes one away, and the answer still says the section is marked.
func TestMkAQuestionTwoComputersMarkedStaysMarkedUntilBothAreTakenAway(t *testing.T) {
	t.Parallel()
	a, b := newMkRig(t), newMkRig(t)
	mkStatus(t, a.press("section-one"), http.StatusOK)
	mkStatus(t, b.press("section-one"), http.StatusOK)
	mkPull(t, b, a)
	first := a.press("section-one")
	mkStatus(t, first, http.StatusOK)
	if got := mkDecode[map[string]bool](t, first); !got["marked"] {
		t.Fatalf("taking one of two away answered %v, want the section still marked", got)
	}
	second := a.press("section-one")
	if got := mkDecode[map[string]bool](t, second); got["marked"] {
		t.Fatalf("taking the second away answered %v, want unmarked", got)
	}
}

// A question the earlier file kept is listed before the reader has written
// anything, and the first press of the button takes it away: nothing the reader
// marked before the upgrade is hidden until a write, and none is lost by one.
func TestMkAnEarlierQuestionIsListedBeforeAnyWriteAndPressedAway(t *testing.T) {
	t.Parallel()
	r := newMkRig(t)
	earlier := `{"version":1,"vault":"/somewhere","marks":[{"id":"` + strings.Repeat("7", 32) + `","kind":"question","path":"` + mkNote + `","anchor":"section-one","at":"2026-09-22T10:00:00Z"}]}`
	r.writeEarlier(earlier)
	before := mkTree(t, r.root)
	listed := mkDecode[[]map[string]any](t, r.do(http.MethodGet, "/questions", ""))
	if len(listed) != 1 || listed[0]["path"] != mkNote {
		t.Fatalf("the list is %v, want the earlier file's question", listed)
	}
	if diff := cmp.Diff(before, mkTree(t, r.root)); diff != "" {
		t.Fatalf("listing wrote into the vault:\n%s", diff)
	}
	pressed := r.press("section-one")
	mkStatus(t, pressed, http.StatusOK)
	if got := mkDecode[map[string]bool](t, pressed); got["marked"] {
		t.Fatalf("pressing a question of the earlier file answered %v, want it taken away", got)
	}
	if got := mkDecode[[]map[string]any](t, r.do(http.MethodGet, "/questions", "")); len(got) != 0 {
		t.Fatalf("the list after the press is %v, want none", got)
	}
	if _, err := os.Stat(filepath.Join(r.store.Dir(), "marks.json")); err != nil {
		t.Fatalf("the earlier file is gone: %v", err)
	}
}

// A change addressed to a mark that only the earlier file holds takes the
// earlier marks in first, so that it is not told the mark is not there. A page
// that had read nothing of it is told what is read now, and writes on top of
// that.
func TestMkAChangeToAnEarlierMarkTakesTheEarlierMarksIn(t *testing.T) {
	t.Parallel()
	r := newMkRig(t)
	id := strings.Repeat("5", 32)
	earlier := `{"version":1,"vault":"/somewhere","marks":[{"id":"` + id + `","kind":"thought","path":"` + mkNote + `","anchor":"section-one","body":"from before","at":"2026-09-22T10:00:00Z"}]}`
	r.writeEarlier(earlier)
	first := r.do(http.MethodPatch, "/mark/"+id, `{"seen":{},"body":"now"}`)
	mkStatus(t, first, http.StatusConflict)
	told := mkDecode[mkConflict](t, first)
	if told.Reason != "stale" || len(told.Seen) == 0 {
		t.Fatalf("the first change said %+v, want stale with what the earlier marks became", told)
	}
	mkStatus(t, r.do(http.MethodPatch, "/mark/"+id, `{"seen":`+mkSeenJSON(t, told.Seen)+`,"body":"now"}`), http.StatusOK)
	if got := mkDecode[mkAnswer](t, r.do(http.MethodGet, "/mark/"+id, "")); got.Body != "now" || got.At.Year() != 2026 || got.At.Month() != 9 {
		t.Fatalf("the earlier mark reads %+v after a change, want its new words and the day it was made", got)
	}
	if _, err := os.Stat(filepath.Join(r.store.Dir(), "marks.json")); err != nil {
		t.Fatalf("the earlier file is gone: %v", err)
	}
}
