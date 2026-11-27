package mark_test

import (
	"context"
	"encoding/json"
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
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/mark"
	"github.com/koopa0/yomihon/internal/mark/confdir"
	"github.com/koopa0/yomihon/internal/mark/machinemarks"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

const (
	aNote     = "Notes/source.md"
	otherNote = "Notes/secret.md"
	aText     = "In Japanese, the topic particle marks what the sentence is about. A second sentence follows. The topic particle again."
)

// rigNotes is the notes of a rig, which can make something happen as one is read.
type rigNotes struct {
	notes

	rig *rig
}

func (n *rigNotes) Reading(rel string, lang wording.Lang) (mark.Page, bool) {
	n.rig.mu.Lock()
	hook := n.rig.onReading
	n.rig.onReading = nil
	n.rig.mu.Unlock()
	if hook != nil {
		hook()
	}
	return n.notes.Reading(rel, lang)
}

// Note is the margin note a page would draw for a mark: here only enough to tell
// a test that the route asked for one, in the language it was asked in.
func (n *rigNotes) Note(_ context.Context, m *machinemarks.Mark, placement string, lang wording.Lang) string {
	return "<aside data-kind=\"" + string(m.Kind) + "\" data-placement=\"" + placement + "\" lang=\"" + lang.Tag() + "\"></aside>"
}

// rig is one computer: a vault folder, its own configuration directory, and the
// routes over them.
type rig struct {
	t          *testing.T
	root       string
	conf       string
	store      *mark.Store
	vaultMarks *mark.VaultMarks
	mux        *http.ServeMux
	mu         sync.Mutex
	// onReading, when set, is called once, the next time a note is read to place a
	// mark: a write reads it after it has been let in and before it records
	// anything, which is the moment a test makes the vault refuse the file.
	onReading func()
}

func newRig(t *testing.T) *rig {
	t.Helper()
	r := &rig{t: t, root: t.TempDir(), conf: t.TempDir()}
	if err := os.MkdirAll(filepath.Join(r.root, "Notes"), 0o750); err != nil {
		t.Fatal(err)
	}
	r.open()
	return r
}

// open is a new process over the same folders.
func (r *rig) open() {
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
	r.store = store
	r.vaultMarks = mark.OpenVaultMarks(store, reader, slog.New(slog.DiscardHandler))
	r.t.Cleanup(func() { _ = r.vaultMarks.Close() }) //nolint:errcheck // the test is over
	r.mux = http.NewServeMux()
	notes := &rigNotes{notes: notes{aNote: aText, otherNote: aText}, rig: r}
	mark.NewHandler(store, r.vaultMarks, notes, slog.New(slog.DiscardHandler)).Register(r.mux)
}

// do sends one JSON request, in the language asked for when there is one.
func (r *rig) do(method, target, body string, lang ...string) *httptest.ResponseRecorder {
	r.t.Helper()
	return send(r.t, r.mux, method, target, jsonBody, body, lang...)
}

// press sends the form the question button of section-one posts.
func (r *rig) press() *httptest.ResponseRecorder {
	r.t.Helper()
	return send(r.t, r.mux, http.MethodPost, mark.Paths.Question, formBody, "path="+aNote+"&anchor=section-one")
}

// writeEarlier leaves marks.json as the earlier version kept it.
func (r *rig) writeEarlier(content string) {
	r.t.Helper()
	if err := os.MkdirAll(r.store.Dir(), 0o750); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r.store.Dir(), "marks.json"), []byte(content), 0o600); err != nil {
		r.t.Fatal(err)
	}
}

// answer is what the routes answer with for one mark.
type answer struct {
	machinemarks.Mark

	Placement string            `json:"placement"`
	Seen      machinemarks.Seen `json:"seen"`
}

// conflict is what a 409 says.
type conflict struct {
	Reason string            `json:"reason"`
	Seen   machinemarks.Seen `json:"seen"`
}

func decoded[T any](t *testing.T, response *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(response.Body.Bytes(), &out); err != nil {
		t.Fatalf("the answer %q is not what was expected: %v", response.Body.String(), err)
	}
	return out
}

func wantStatus(t *testing.T, response *httptest.ResponseRecorder, want int) {
	t.Helper()
	if response.Code != want {
		t.Fatalf("status = %d, want %d; body %q", response.Code, want, response.Body.String())
	}
}

// seenJSON is a seen as a request carries it.
func seenJSON(t *testing.T, seen machinemarks.Seen) string {
	t.Helper()
	b, err := json.Marshal(seen)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// thought keeps a thought and answers it.
func (r *rig) thought(body string) answer {
	r.t.Helper()
	response := r.do(http.MethodPost, "/mark", `{"kind":"thought","path":"`+aNote+`","anchor":"section-one","body":`+quoted(body)+`}`)
	wantStatus(r.t, response, http.StatusCreated)
	return decoded[answer](r.t, response)
}

func quoted(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// lines is the lines of every machine file of the vault.
func (r *rig) lines() []string {
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

// POST /mark keeps each kind, answers it with where it stands and what the page
// has read, keeps a highlight once, and every thought as new.
func TestPostKeepsEachKindAndAnswersWithWhatThePageHasRead(t *testing.T) {
	t.Parallel()
	r := newRig(t)

	highlight := `{"kind":"highlight","path":"` + aNote + `","anchor":"section-one","quote":"the topic particle marks","before":"In Japanese, ","after":" what the sentence"}`
	created := r.do(http.MethodPost, "/mark", highlight)
	wantStatus(t, created, http.StatusCreated)
	first := decoded[answer](t, created)
	if len(first.ID) != 32 || first.Kind != machinemarks.Highlight || first.Quote != "the topic particle marks" || first.Identity != anIdentity {
		t.Fatalf("a highlight was answered %+v", first)
	}
	if first.Placement != "found" || len(first.Seen) != 1 {
		t.Fatalf("placement %q, seen %v; want found and the one line the page has read", first.Placement, first.Seen)
	}
	if got := len(r.lines()); got != 1 {
		t.Fatalf("the vault holds %d lines after one mark, want 1", got)
	}

	again := r.do(http.MethodPost, "/mark", highlight)
	wantStatus(t, again, http.StatusOK)
	if second := decoded[answer](t, again); second.ID != first.ID || !cmp.Equal(second.Seen, first.Seen) {
		t.Fatalf("the same highlight kept again was answered %+v, want the first %+v", second, first)
	}
	if got := len(r.lines()); got != 1 {
		t.Fatalf("a highlight kept twice wrote %d lines, want 1", got)
	}

	thought := `{"kind":"thought","path":"` + aNote + `","anchor":"section-one","body":"why <b>this</b> particle?"}`
	one, two := r.do(http.MethodPost, "/mark", thought), r.do(http.MethodPost, "/mark", thought)
	wantStatus(t, one, http.StatusCreated)
	wantStatus(t, two, http.StatusCreated)
	a, b := decoded[answer](t, one), decoded[answer](t, two)
	if a.ID == b.ID || a.Body != "why <b>this</b> particle?" || a.Placement != "section" {
		t.Fatalf("two thoughts were %+v and %+v, want two marks with the words as sent", a, b)
	}

	// Asking again with other words is a new question, not an answer with the
	// earlier one and the new words dropped; asking again with the same ones is
	// the same question.
	asked := `{"kind":"question","path":"` + aNote + `","anchor":"section-one","body":"why が?"}`
	wantStatus(t, r.do(http.MethodPost, "/mark", asked), http.StatusCreated)
	wantStatus(t, r.do(http.MethodPost, "/mark", asked), http.StatusOK)
	reasked := r.do(http.MethodPost, "/mark", `{"kind":"question","path":"`+aNote+`","anchor":"section-one","body":"why not は?"}`)
	wantStatus(t, reasked, http.StatusCreated)
	if got := decoded[answer](t, reasked); got.Body != "why not は?" {
		t.Fatalf("a question asked again with other words was answered %+v, want the words sent", got)
	}
	section := r.do(http.MethodPost, "/mark", `{"kind":"question","path":"`+aNote+`","anchor":"section-one"}`)
	wantStatus(t, section, http.StatusCreated)
	withWords := r.do(http.MethodPost, "/mark", `{"kind":"question","path":"`+aNote+`","anchor":"section-one","quote":"the topic particle marks","body":"is it always が?"}`)
	wantStatus(t, withWords, http.StatusCreated)
	if got := len(r.lines()); got != 7 {
		t.Fatalf("the vault holds %d lines, want 7", got)
	}
}

// A mark's times are kept in UTC to the millisecond, as a bookmark's are: the
// machine file is carried to other computers by git, and a time that held this
// one's zone would say where the reader was.
func TestAMarkIsKeptAtATimeInUTCToTheMillisecond(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	question := r.do(http.MethodPost, "/mark", `{"kind":"question","path":"`+aNote+`","anchor":"section-one"}`)
	wantStatus(t, question, http.StatusCreated)
	asked := decoded[answer](t, question)
	resolved := r.do(http.MethodPatch, "/mark/"+asked.ID, `{"seen":`+seenJSON(t, asked.Seen)+`,"resolved":true}`)
	wantStatus(t, resolved, http.StatusOK)
	for _, line := range r.lines() {
		var l struct {
			Mark struct {
				At       time.Time `json:"at"`
				Resolved time.Time `json:"resolved"`
			} `json:"mark"`
		}
		if err := json.Unmarshal([]byte(line), &l); err != nil {
			t.Fatal(err)
		}
		for _, at := range []time.Time{l.Mark.At, l.Mark.Resolved} {
			if !at.IsZero() && (at.Location() != time.UTC || !at.Equal(at.Truncate(time.Millisecond))) {
				t.Errorf("a mark keeps the time %s, want UTC to the millisecond: %s", at.Format(time.RFC3339Nano), line)
			}
		}
	}
}

// A body that is past the limit after its first value is too large, not a
// second value: the answer says which.
func TestABodyPastTheLimitAfterItsValueIsTooLarge(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	body := `{"kind":"thought","path":"` + aNote + `","body":"x"}` + strings.Repeat(" ", 70<<10) + "x"
	wantStatus(t, r.do(http.MethodPost, "/mark", body), http.StatusRequestEntityTooLarge)
}

// What the route will not keep, it says, in the reader's language, and writes
// nothing into the vault.
func TestPostRefusesWhatItCannotKeepAndWritesNothing(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	for _, tt := range []struct {
		name, body string
		want       int
	}{
		{"a thought that says nothing", `{"kind":"thought","path":"` + aNote + `","body":""}`, http.StatusUnprocessableEntity},
		{"words the note never had", `{"kind":"highlight","path":"` + aNote + `","quote":"not in the note"}`, http.StatusUnprocessableEntity},
		{"words at several places nothing tells apart", `{"kind":"highlight","path":"` + aNote + `","quote":"topic particle"}`, http.StatusUnprocessableEntity},
		{"a note this vault does not hold", `{"kind":"question","path":"Notes/missing.md"}`, http.StatusUnprocessableEntity},
		{"an anchor the note does not render", `{"kind":"question","path":"` + aNote + `","anchor":"nowhere"}`, http.StatusUnprocessableEntity},
		{"a field nobody knows", `{"kind":"question","path":"` + aNote + `","reason":"why"}`, http.StatusBadRequest},
		{"two values", `{"kind":"question","path":"` + aNote + `"}{}`, http.StatusBadRequest},
		{"no JSON at all", `not json`, http.StatusBadRequest},
		{"a body past its cap", `{"kind":"thought","path":"` + aNote + `","body":"` + strings.Repeat("x", 70<<10) + `"}`, http.StatusRequestEntityTooLarge},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			own := newRig(t)
			vaultBefore := tree(t, own.root)
			response := own.do(http.MethodPost, "/mark", tt.body)
			wantStatus(t, response, tt.want)
			if diff := cmp.Diff(vaultBefore, tree(t, own.root)); diff != "" {
				t.Errorf("a refused mark changed the vault (-before +after):\n%s", diff)
			}
		})
	}

	wantStatus(t, send(t, r.mux, http.MethodPost, "/mark", formBody, "kind=question&path="+aNote), http.StatusUnsupportedMediaType)

	zh := r.do(http.MethodPost, "/mark", `{"kind":"thought","path":"`+aNote+`","body":""}`, "zh-TW")
	en := r.do(http.MethodPost, "/mark", `{"kind":"thought","path":"`+aNote+`","body":""}`, "en")
	if !strings.Contains(zh.Body.String(), wording.MarkNotKept.In(wording.ZhHant)) || !strings.Contains(en.Body.String(), wording.MarkNotKept.In(wording.En)) {
		t.Errorf("the refusals say %q and %q, want the reader's own language", zh.Body.String(), en.Body.String())
	}
}

// Where the reader folder cannot be written to, a request made by hand is told
// why and nothing is written.
func TestAWriteWhereTheReaderFolderCannotTakeItIsRefusedWithTheReason(t *testing.T) {
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
			r := newRig(t)
			tt.build(t, r.root)
			if got := r.vaultMarks.Writable(); got != tt.want {
				t.Fatalf("Writable() = %q, want %q", got, tt.want)
			}
			before := tree(t, r.root)
			for _, request := range []struct{ method, target, body string }{
				{http.MethodPost, "/mark", `{"kind":"question","path":"` + aNote + `"}`},
				{http.MethodPatch, "/mark/" + strings.Repeat("a", 32), `{"seen":{},"body":"x"}`},
				{http.MethodDelete, "/mark/" + strings.Repeat("a", 32), `{"seen":{}}`},
				{http.MethodPut, "/mark/" + strings.Repeat("a", 32), `{"seen":{}}`},
			} {
				response := r.do(request.method, request.target, request.body, "en")
				wantStatus(t, response, http.StatusUnprocessableEntity)
				// The reader is told only that the mark could not be kept; why is the log's.
				if want := wording.MarkNotKept.In(wording.En); !strings.Contains(response.Body.String(), want) || strings.Contains(response.Body.String(), "reader-notes") {
					t.Errorf("%s %s said %q, want %q and no word of the folder", request.method, request.target, response.Body.String(), want)
				}
			}
			if diff := cmp.Diff(before, tree(t, r.root)); diff != "" {
				t.Errorf("a refused write changed the vault:\n%s", diff)
			}
		})
	}
}

// GET /mark/{id} answers what a page needs to change the mark: the value, which is
// the newest live one where computers changed it differently, and the seen. A deleted mark is answered with
// deleted set, which is what undoing it starts from.
func TestGetAnswersTheMarkAndWhatThePageHasRead(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	kept := r.thought("first words")
	got := r.do(http.MethodGet, "/mark/"+kept.ID, "")
	wantStatus(t, got, http.StatusOK)
	read := decoded[answer](t, got)
	if read.Body != "first words" || read.Placement != "section" || !cmp.Equal(read.Seen, kept.Seen) {
		t.Fatalf("GET answered %+v, want the mark, its placement, and the seen the keep answered", read)
	}
	if control := got.Header().Get("Cache-Control"); control != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store: a seen that was kept is a stale base", control)
	}

	deleted := r.do(http.MethodDelete, "/mark/"+kept.ID, `{"seen":`+seenJSON(t, read.Seen)+`}`)
	wantStatus(t, deleted, http.StatusOK)
	after := decoded[answer](t, r.do(http.MethodGet, "/mark/"+kept.ID, ""))
	if !after.Deleted || after.Body != "first words" {
		t.Fatalf("a deleted mark was answered %+v, want deleted, with its words", after)
	}
}

// A malformed id is a missing mark, and nothing is read for it.
func TestAnIdThatIsNotOneIsAMissingMark(t *testing.T) {
	t.Parallel()
	r := newRig(t)
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
func TestPatchChangesWordsAndResolvesAndAnswersTheNextBase(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	thought := r.thought("first")
	patched := r.do(http.MethodPatch, "/mark/"+thought.ID, `{"seen":`+seenJSON(t, thought.Seen)+`,"body":"second"}`)
	wantStatus(t, patched, http.StatusOK)
	second := decoded[answer](t, patched)
	if second.Body != "second" || second.ID != thought.ID || !second.At.Equal(thought.At) || second.Seen[firstKey(second.Seen)] <= thought.Seen[firstKey(thought.Seen)] {
		t.Fatalf("PATCH answered %+v, want the same mark with new words, made at the same time", second)
	}
	third := r.do(http.MethodPatch, "/mark/"+thought.ID, `{"seen":`+seenJSON(t, second.Seen)+`,"body":"third"}`)
	wantStatus(t, third, http.StatusOK)

	question := decoded[answer](t, r.do(http.MethodPost, "/mark", `{"kind":"question","path":"`+aNote+`","anchor":"section-one"}`))
	resolved := r.do(http.MethodPatch, "/mark/"+question.ID, `{"seen":`+seenJSON(t, question.Seen)+`,"resolved":true,"body":"it was が"}`)
	wantStatus(t, resolved, http.StatusOK)
	done := decoded[answer](t, resolved)
	if done.Resolved.IsZero() || done.Body != "it was が" {
		t.Fatalf("a resolved question was answered %+v", done)
	}
	again := decoded[answer](t, r.do(http.MethodPatch, "/mark/"+question.ID, `{"seen":`+seenJSON(t, done.Seen)+`,"resolved":true}`))
	if !again.Resolved.Equal(done.Resolved) {
		t.Fatalf("resolving again moved the time from %v to %v", done.Resolved, again.Resolved)
	}
	reopened := decoded[answer](t, r.do(http.MethodPatch, "/mark/"+question.ID, `{"seen":`+seenJSON(t, again.Seen)+`,"resolved":false}`))
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
func TestPatchRefusesWhatTheKindDoesNotHaveAndWritesNothing(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	thought := r.thought("words")
	highlight := decoded[answer](t, r.do(http.MethodPost, "/mark", `{"kind":"highlight","path":"`+aNote+`","quote":"the topic particle marks"}`))
	for _, tt := range []struct {
		name string
		id   string
		seen machinemarks.Seen
		body string
	}{
		{"a thought resolved", thought.ID, thought.Seen, `"resolved":true`},
		{"a thought with no words", thought.ID, thought.Seen, `"body":""`},
		{"a highlight resolved", highlight.ID, highlight.Seen, `"resolved":true`},
		{"nothing to change", thought.ID, thought.Seen, `"edit":null`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel() // each of these refuses and writes nothing, so they may share one vault
			before := tree(t, r.root)
			response := r.do(http.MethodPatch, "/mark/"+tt.id, `{"seen":`+seenJSON(t, tt.seen)+","+tt.body+`}`)
			wantStatus(t, response, http.StatusUnprocessableEntity)
			if diff := cmp.Diff(before, tree(t, r.root)); diff != "" {
				t.Errorf("a refused change wrote:\n%s", diff)
			}
		})
	}
	// A body with fields nobody knows, and no seen at all.
	wantStatus(t, r.do(http.MethodPatch, "/mark/"+thought.ID, `{"seen":{},"reason":"x"}`), http.StatusBadRequest)
}

// Every refusal that has a reason says it, and the page tells them apart by it.
func TestEveryRefusalHasItsReason(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	thought := r.thought("first")
	seen0 := thought.Seen

	// A change on a page that read nothing of the mark is no change at all.
	noBase := r.do(http.MethodPatch, "/mark/"+thought.ID, `{"body":"x"}`)
	wantStatus(t, noBase, http.StatusBadRequest)
	nullBase := r.do(http.MethodPatch, "/mark/"+thought.ID, `{"seen":null,"body":"x"}`)
	wantStatus(t, nullBase, http.StatusBadRequest)
	wantStatus(t, r.do(http.MethodDelete, "/mark/"+thought.ID, `{}`), http.StatusBadRequest)
	wantStatus(t, r.do(http.MethodPut, "/mark/"+thought.ID, `{}`), http.StatusBadRequest)

	// Another tab changes the mark; this page, which is old, is stale.
	changed := decoded[answer](t, r.do(http.MethodPatch, "/mark/"+thought.ID, `{"seen":`+seenJSON(t, seen0)+`,"body":"another tab"}`))
	for _, request := range []struct{ method, body string }{
		{http.MethodPatch, `{"seen":` + seenJSON(t, seen0) + `,"body":"old page"}`},
		{http.MethodDelete, `{"seen":` + seenJSON(t, seen0) + `}`},
	} {
		response := r.do(request.method, "/mark/"+thought.ID, request.body)
		wantStatus(t, response, http.StatusConflict)
		got := decoded[conflict](t, response)
		if got.Reason != "stale" || !cmp.Equal(got.Seen, changed.Seen) {
			t.Fatalf("%s from an old page said %+v, want stale with what is read now %v", request.method, got, changed.Seen)
		}
	}
	if got := decoded[answer](t, r.do(http.MethodGet, "/mark/"+thought.ID, "")); got.Body != "another tab" {
		t.Fatalf("a stale page changed the mark: it reads %q", got.Body)
	}

	// A late send of an edit that a newer send of the same edit is written over.
	wantStatus(t, r.do(http.MethodPatch, "/mark/"+thought.ID, `{"seen":`+seenJSON(t, changed.Seen)+`,"edit":{"id":"tab-1","seq":2},"body":"newest words"}`), http.StatusOK)
	late := r.do(http.MethodPatch, "/mark/"+thought.ID, `{"seen":`+seenJSON(t, changed.Seen)+`,"edit":{"id":"tab-1","seq":1},"body":"older words"}`)
	wantStatus(t, late, http.StatusConflict)
	if got := decoded[conflict](t, late); got.Reason != "superseded" || len(got.Seen) != 0 {
		t.Fatalf("a late send said %+v, want superseded and nothing else", got)
	}
	if got := decoded[answer](t, r.do(http.MethodGet, "/mark/"+thought.ID, "")); got.Body != "newest words" {
		t.Fatalf("a late send wrote: the mark reads %q", got.Body)
	}
	// The last words of an edit, sent after the page was left, go through.
	last := r.do(http.MethodPatch, "/mark/"+thought.ID, `{"seen":`+seenJSON(t, changed.Seen)+`,"edit":{"id":"tab-1","seq":3},"body":"last words"}`)
	wantStatus(t, last, http.StatusOK)

	// Deleting what is deleted already, and changing it.
	now := decoded[answer](t, last)
	deleted := r.do(http.MethodDelete, "/mark/"+thought.ID, `{"seen":`+seenJSON(t, now.Seen)+`}`)
	wantStatus(t, deleted, http.StatusOK)
	for _, request := range []struct{ method, body string }{
		{http.MethodDelete, `{"seen":` + seenJSON(t, now.Seen) + `}`},
		{http.MethodPatch, `{"seen":` + seenJSON(t, now.Seen) + `,"body":"into a deleted mark"}`},
	} {
		response := r.do(request.method, "/mark/"+thought.ID, request.body)
		wantStatus(t, response, http.StatusConflict)
		if got := decoded[conflict](t, response); got.Reason != "gone" {
			t.Fatalf("%s of a deleted mark said %+v, want gone", request.method, got)
		}
	}
	wantStatus(t, r.do(http.MethodPatch, "/mark/"+strings.Repeat("c", 32), `{"seen":{},"body":"x"}`), http.StatusNotFound)
}

// "Use my words": a page that was told it is stale writes on top of what it was
// told, which replaces the other tab's words and nothing else.
func TestAStalePageMayWriteOnTopOfWhatItWasTold(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	thought := r.thought("first")
	r.do(http.MethodPatch, "/mark/"+thought.ID, `{"seen":`+seenJSON(t, thought.Seen)+`,"body":"other tab"}`)
	old := r.do(http.MethodPatch, "/mark/"+thought.ID, `{"seen":`+seenJSON(t, thought.Seen)+`,"body":"mine"}`)
	wantStatus(t, old, http.StatusConflict)
	told := decoded[conflict](t, old)
	mine := r.do(http.MethodPatch, "/mark/"+thought.ID, `{"seen":`+seenJSON(t, told.Seen)+`,"body":"mine"}`)
	wantStatus(t, mine, http.StatusOK)
	got := decoded[answer](t, r.do(http.MethodGet, "/mark/"+thought.ID, ""))
	if got.Body != "mine" {
		t.Fatalf("after use my words the mark reads %+v", got)
	}
}

// DELETE keeps the words in the mark's line, and PUT puts the mark back from them.
func TestDeleteAndPutBackRoundTripTheWords(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	thought := r.thought("words that matter")
	deleted := r.do(http.MethodDelete, "/mark/"+thought.ID, `{"seen":`+seenJSON(t, thought.Seen)+`}`)
	wantStatus(t, deleted, http.StatusOK)
	gone := decoded[answer](t, deleted)
	if !gone.Deleted || gone.Body != "words that matter" {
		t.Fatalf("a deleted mark was answered %+v, want deleted with its words", gone)
	}
	// An old page cannot put it back over the delete it had not read.
	stale := r.do(http.MethodPut, "/mark/"+thought.ID, `{"seen":`+seenJSON(t, thought.Seen)+`}`)
	wantStatus(t, stale, http.StatusConflict)
	if got := decoded[conflict](t, stale); got.Reason != "stale" {
		t.Fatalf("an old page's undo said %+v, want stale", got)
	}
	back := r.do(http.MethodPut, "/mark/"+thought.ID, `{"seen":`+seenJSON(t, gone.Seen)+`}`)
	wantStatus(t, back, http.StatusOK)
	restored := decoded[answer](t, back)
	if restored.Deleted || restored.Body != "words that matter" || !restored.At.Equal(thought.At) || restored.ID != thought.ID {
		t.Fatalf("a mark put back was answered %+v, want the mark as it was", restored)
	}
	// Putting back what is not deleted answers it and writes nothing.
	lines := len(r.lines())
	again := r.do(http.MethodPut, "/mark/"+thought.ID, `{"seen":`+seenJSON(t, restored.Seen)+`}`)
	wantStatus(t, again, http.StatusOK)
	if got := len(r.lines()); got != lines {
		t.Fatalf("putting back a mark that was not deleted wrote %d lines", got-lines)
	}
}

// A highlight is deleted and put back too, and a mark deleted by one writer is
// never brought back by a page that had not read the deletion.
func TestADeletedMarkStaysDeletedForAPageThatHadNotReadTheDeletion(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	kept := decoded[answer](t, r.do(http.MethodPost, "/mark", `{"kind":"highlight","path":"`+aNote+`","quote":"the topic particle marks"}`))
	wantStatus(t, r.do(http.MethodDelete, "/mark/"+kept.ID, `{"seen":`+seenJSON(t, kept.Seen)+`}`), http.StatusOK)
	if got := decoded[answer](t, r.do(http.MethodGet, "/mark/"+kept.ID, "")); !got.Deleted {
		t.Fatal("a deleted highlight reads as there")
	}
	// Keeping the same highlight again is a new mark: the deleted one is not "the same".
	again := r.do(http.MethodPost, "/mark", `{"kind":"highlight","path":"`+aNote+`","quote":"the topic particle marks"}`)
	wantStatus(t, again, http.StatusCreated)
	if got := decoded[answer](t, again); got.ID == kept.ID {
		t.Fatal("a highlight kept after its delete was answered as the deleted one")
	}
}

// The two-tab scenario, over HTTP: a tab edits, another computer's edit arrives
// through git, and the tab sends again. The reader is shown the newest live value
// and is told nothing of any other: no answer of a route has a field for one. The
// other computer's words are not dropped: no send of a page that has not read a
// line replaces it, and both values stay in the files of the vault.
func TestAForeignEditThatGitBringsInSurvivesTwoSendsAndShowsTheNewest(t *testing.T) {
	t.Parallel()
	a, b := newRig(t), newRig(t)

	thought := a.thought("the first words")
	bring(t, a, b)
	// B's page reads the mark and edits it: B has read A's line, so its edit
	// replaces it.
	read := decoded[answer](t, b.do(http.MethodGet, "/mark/"+thought.ID, ""))
	wantStatus(t, b.do(http.MethodPatch, "/mark/"+thought.ID, `{"seen":`+seenJSON(t, read.Seen)+`,"body":"B's words, written elsewhere"}`), http.StatusOK)

	// A's tab, drawn before B wrote, sends its edit, and gets what it has read.
	sent := a.do(http.MethodPatch, "/mark/"+thought.ID, `{"seen":`+seenJSON(t, thought.Seen)+`,"edit":{"id":"tab","seq":1},"body":"A's words"}`)
	wantStatus(t, sent, http.StatusOK)
	first := decoded[answer](t, sent)

	// git pull brings B's file in.
	bring(t, b, a)

	// The same tab sends again, on top of what the first send answered.
	second := a.do(http.MethodPatch, "/mark/"+thought.ID, `{"seen":`+seenJSON(t, first.Seen)+`,"edit":{"id":"tab","seq":2},"body":"A's words, a little longer"}`)
	wantStatus(t, second, http.StatusOK)

	// Every answer says the one value and no word of another.
	fields := func(response *httptest.ResponseRecorder) map[string]json.RawMessage {
		t.Helper()
		var got map[string]json.RawMessage
		if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
			t.Fatalf("the answer %q is not an object: %v", response.Body.String(), err)
		}
		return got
	}
	want := slices.Sorted(maps.Keys(fields(a.do(http.MethodGet, "/mark/"+oneValued(t, a).ID, ""))))
	for name, response := range map[string]*httptest.ResponseRecorder{
		"the send that left B's words beside A's": second,
		"A's GET": a.do(http.MethodGet, "/mark/"+thought.ID, ""),
	} {
		if got := slices.Sorted(maps.Keys(fields(response))); !slices.Equal(got, want) || strings.Contains(strings.ToLower(response.Body.String()), "conflict") {
			t.Errorf("%s answered %s, want the fields of a mark that has no other value (%v) and no word of one", name, response.Body.String(), want)
		}
	}

	// The value shown is the newest live one: A's second send is the later write.
	if got := decoded[answer](t, a.do(http.MethodGet, "/mark/"+thought.ID, "")); got.Body != "A's words, a little longer" {
		t.Fatalf("A reads %q, want the newest live value, A's second send", got.Body)
	}
	// B reads the same value once A's file comes the other way.
	bring(t, a, b)
	if got := decoded[answer](t, b.do(http.MethodGet, "/mark/"+thought.ID, "")); got.Body != "A's words, a little longer" {
		t.Fatalf("B reads %q, want the same newest live value", got.Body)
	}

	// Neither send replaced the other computer's value: both are still values of
	// the mark in the data, though only the newest is shown.
	view, err := a.vaultMarks.View()
	if err != nil {
		t.Fatal(err)
	}
	if entry, ok := view.Entry(thought.ID, mark.AllMarks); !ok || len(entry.Values) != 2 {
		t.Fatalf("the mark holds %+v, want both computers' values kept in the data", entry)
	}
	// Both values are still in the files of the vault, each whole.
	lines := strings.Join(a.lines(), "\n")
	for _, words := range []string{"A's words, a little longer", "B's words, written elsewhere"} {
		if !strings.Contains(lines, words) {
			t.Errorf("the machine files of the vault no longer hold %q:\n%s", words, lines)
		}
	}
}

// oneValued keeps a mark that nothing else touches, to know what a mark's answer
// has in it when there is nothing but the one value.
func oneValued(t *testing.T, r *rig) answer {
	t.Helper()
	return r.thought("a thought with one value")
}

// A question two computers each marked before either had read the other's is two
// marks; pressing takes one away, and the answer still says the section is marked.
func TestAQuestionTwoComputersMarkedStaysMarkedUntilBothAreTakenAway(t *testing.T) {
	t.Parallel()
	a, b := newRig(t), newRig(t)
	wantStatus(t, a.press(), http.StatusOK)
	wantStatus(t, b.press(), http.StatusOK)
	bring(t, b, a)
	if !marked(t, a.press()) {
		t.Fatal("taking one of two away answered unmarked, want the section still marked")
	}
	if marked(t, a.press()) {
		t.Fatal("taking the second away answered marked, want unmarked")
	}
}

// A change addressed to a mark that only the earlier file holds takes the
// earlier marks in first, so that it is not told the mark is not there. A page
// that had read nothing of it is told what is read now, and writes on top of
// that.
func TestAChangeToAnEarlierMarkTakesTheEarlierMarksIn(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	id := strings.Repeat("5", 32)
	earlier := `{"version":1,"vault":"/somewhere","marks":[{"id":"` + id + `","kind":"thought","path":"` + aNote + `","anchor":"section-one","body":"from before","at":"2026-09-22T10:00:00Z"}]}`
	r.writeEarlier(earlier)
	first := r.do(http.MethodPatch, "/mark/"+id, `{"seen":{},"body":"now"}`)
	wantStatus(t, first, http.StatusConflict)
	told := decoded[conflict](t, first)
	if told.Reason != "stale" || len(told.Seen) == 0 {
		t.Fatalf("the first change said %+v, want stale with what the earlier marks became", told)
	}
	wantStatus(t, r.do(http.MethodPatch, "/mark/"+id, `{"seen":`+seenJSON(t, told.Seen)+`,"body":"now"}`), http.StatusOK)
	if got := decoded[answer](t, r.do(http.MethodGet, "/mark/"+id, "")); got.Body != "now" || got.At.Year() != 2026 || got.At.Month() != 9 {
		t.Fatalf("the earlier mark reads %+v after a change, want its new words and the day it was made", got)
	}
	if _, err := os.Stat(filepath.Join(r.store.Dir(), "marks.json")); err != nil {
		t.Fatalf("the earlier file is gone: %v", err)
	}
}
