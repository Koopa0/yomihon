package mark_test

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/mark"
)

// mkMarksFolder is where the machine files of a computer are.
func (r *mkRig) mkMarksFolder() string {
	return filepath.Join(r.root, "reader-notes", ".yomihon", "marks")
}

// mkFiles is the machine files of the vault by name.
func (r *mkRig) mkFiles() map[string]string {
	r.t.Helper()
	out := map[string]string{}
	files, err := filepath.Glob(filepath.Join(r.mkMarksFolder(), "*.jsonl"))
	if err != nil {
		r.t.Fatal(err)
	}
	for _, f := range files {
		data, readErr := os.ReadFile(f) // #nosec G304 -- a test's own path
		if readErr != nil {
			r.t.Fatal(readErr)
		}
		out[filepath.Base(f)] = string(data)
	}
	return out
}

// mkBring stands for git bringing in what another computer wrote: its file, and
// not the copies of files that the receiving computer wrote itself, which git
// never replaces with an older copy of its own.
func mkBring(t *testing.T, from, to *mkRig, except ...string) {
	t.Helper()
	if err := os.MkdirAll(to.mkMarksFolder(), 0o750); err != nil {
		t.Fatal(err)
	}
	for name, data := range from.mkFiles() {
		if slices.Contains(except, name) {
			continue
		}
		if err := os.WriteFile(filepath.Join(to.mkMarksFolder(), name), []byte(data), 0o600); err != nil { // #nosec G703 -- a test's own path
			t.Fatal(err)
		}
	}
}

// mkOnlyFile is the name of the one machine file a computer has.
func (r *mkRig) mkOnlyFile() string {
	r.t.Helper()
	files := r.mkFiles()
	if len(files) != 1 {
		r.t.Fatalf("the vault holds %d machine files, want 1", len(files))
	}
	for name := range files {
		return name
	}
	return ""
}

// mkHolds says some machine file of the vault has a line with all these words.
func (r *mkRig) mkHolds(words ...string) bool {
	for _, data := range r.mkFiles() {
		for line := range strings.SplitSeq(data, "\n") {
			if line != "" && !slices.ContainsFunc(words, func(w string) bool { return !strings.Contains(line, w) }) {
				return true
			}
		}
	}
	return false
}

// mkDiverged is two computers that changed one question differently, and A's
// tab, which has not read B's change: A's file holds "X2 from A", B's holds
// "Y from B", the newer of the two on the clock but not one A's page covers.
// seen is what A's tab has read.
func mkDiverged(t *testing.T) (a *mkRig, id, seen string) {
	t.Helper()
	a, b := newMkRig(t), newMkRig(t)
	first := a.do(http.MethodPost, "/mark", `{"kind":"question","path":"`+mkNote+`","anchor":"section-one","body":"X from A"}`)
	mkStatus(t, first, http.StatusCreated)
	kept := mkDecode[mkAnswer](t, first)
	aFile := a.mkOnlyFile()
	mkBring(t, a, b)

	second := a.do(http.MethodPatch, "/mark/"+kept.ID, `{"seen":`+mkSeenJSON(t, kept.Seen)+`,"body":"X2 from A"}`)
	mkStatus(t, second, http.StatusOK)
	seenByA := mkDecode[mkAnswer](t, second).Seen

	read := mkDecode[mkAnswer](t, b.do(http.MethodGet, "/mark/"+kept.ID, ""))
	mkStatus(t, b.do(http.MethodPatch, "/mark/"+kept.ID, `{"seen":`+mkSeenJSON(t, read.Seen)+`,"body":"Y from B"}`), http.StatusOK)
	mkBring(t, b, a, aFile)
	return a, kept.ID, mkSeenJSON(t, seenByA)
}

// A change that is sent without words is made on the value the page read, not on
// the newest value there is: otherwise a resolve sent from a page that shows
// "X2 from A" would write B's words, which the page never showed, over it, and
// "X2 from A" would be in no file. The reader sees the newest live value; B's
// line is still a value of the mark; nothing is lost.
func TestMkAResolveBuildsOnWhatThePageReadAndLosesNoValue(t *testing.T) {
	t.Parallel()
	a, id, seen := mkDiverged(t)

	response := a.do(http.MethodPatch, "/mark/"+id, `{"seen":`+seen+`,"resolved":true}`)
	mkStatus(t, response, http.StatusOK)

	if !a.mkHolds("X2 from A", `"resolved"`) {
		t.Errorf("no machine file holds the page's own words, resolved, as the reader asked:\n%v", a.mkFiles())
	}
	if !a.mkHolds("Y from B") {
		t.Errorf("no machine file holds B's words any more:\n%v", a.mkFiles())
	}
	view, err := a.kept.View()
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := view.Entry(id, mark.AllMarks)
	if !ok || len(entry.Values) != 2 {
		t.Fatalf("the mark holds %+v, want both values kept in the data", entry)
	}
	// The answer is what the reader will now see: the merged newest live value.
	got := mkDecode[mkAnswer](t, response)
	if want := entry.Mark(); got.Body != want.Body || got.Resolved.IsZero() != want.Resolved.IsZero() {
		t.Errorf("the answer is %q resolved=%v, want what the reader will see: %q resolved=%v", got.Body, !got.Resolved.IsZero(), want.Body, !want.Resolved.IsZero())
	}
	if strings.Contains(response.Body.String(), "conflict") {
		t.Errorf("the answer tells of a conflict: %s", response.Body.String())
	}
}

// The same for a deletion: it deletes the value the page showed, and B's value,
// which the page never read, is still there and is what the reader sees now.
func TestMkADeleteRemovesWhatThePageReadAndLosesNoValue(t *testing.T) {
	t.Parallel()
	a, id, seen := mkDiverged(t)

	response := a.do(http.MethodDelete, "/mark/"+id, `{"seen":`+seen+`}`)
	mkStatus(t, response, http.StatusOK)

	if !a.mkHolds("X2 from A", `"deleted":true`) {
		t.Errorf("no machine file holds the words of the mark the page deleted, in the line that deletes it:\n%v", a.mkFiles())
	}
	if !a.mkHolds("Y from B") {
		t.Errorf("no machine file holds B's words any more:\n%v", a.mkFiles())
	}
	// B's words are live: the answer is the mark the reader still has.
	got := mkDecode[mkAnswer](t, response)
	if got.Deleted || got.Body != "Y from B" {
		t.Errorf("the answer is %q deleted=%v, want the live value the reader still has: %q", got.Body, got.Deleted, "Y from B")
	}
	read := mkDecode[mkAnswer](t, a.do(http.MethodGet, "/mark/"+id, ""))
	if read.Deleted || read.Body != "Y from B" {
		t.Errorf("GET reads %q deleted=%v after the delete, want B's live value", read.Body, read.Deleted)
	}
}

// Putting back puts back what the page showed deleted, even when another value
// of the mark is live: otherwise the put-back would answer the other value and
// write nothing, and the reader's own words would stay deleted.
func TestMkPutBackRestoresWhatThePageReadDeleted(t *testing.T) {
	t.Parallel()
	a, b := newMkRig(t), newMkRig(t)
	first := a.do(http.MethodPost, "/mark", `{"kind":"question","path":"`+mkNote+`","anchor":"section-one","body":"X from A"}`)
	mkStatus(t, first, http.StatusCreated)
	kept := mkDecode[mkAnswer](t, first)
	aFile := a.mkOnlyFile()
	mkBring(t, a, b)

	// A deletes it; the tab has read the deletion.
	deleted := a.do(http.MethodDelete, "/mark/"+kept.ID, `{"seen":`+mkSeenJSON(t, kept.Seen)+`}`)
	mkStatus(t, deleted, http.StatusOK)
	seen := mkDecode[mkAnswer](t, deleted).Seen

	// B, which had not read the deletion, changes the words: they are live.
	read := mkDecode[mkAnswer](t, b.do(http.MethodGet, "/mark/"+kept.ID, ""))
	mkStatus(t, b.do(http.MethodPatch, "/mark/"+kept.ID, `{"seen":`+mkSeenJSON(t, read.Seen)+`,"body":"Y from B"}`), http.StatusOK)
	mkBring(t, b, a, aFile)

	response := a.do(http.MethodPut, "/mark/"+kept.ID, `{"seen":`+mkSeenJSON(t, seen)+`}`)
	mkStatus(t, response, http.StatusOK)
	view, err := a.kept.View()
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := view.Entry(kept.ID, mark.AllMarks)
	if !ok {
		t.Fatal("the mark is gone")
	}
	var live []string
	for _, v := range entry.Values {
		if !v.Mark.Deleted {
			live = append(live, v.Mark.Body)
		}
	}
	slices.Sort(live)
	if !slices.Equal(live, []string{"X from A", "Y from B"}) {
		t.Errorf("the live values are %q, want the put-back words and B's", live)
	}
}

// Two highlights of the same words at two places in a note are two marks: the
// words around them tell the places apart, and the second is not answered with
// the first's id and nothing written.
func TestMkTheSameWordsAtTwoPlacesAreTwoMarks(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"highlight", "question"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			r := newMkRig(t)
			at := func(before, after string) string {
				return `{"kind":"` + kind + `","path":"` + mkNote + `","anchor":"section-one","quote":"topic particle","before":` + mkQuote(before) + `,"after":` + mkQuote(after) + `}`
			}
			first := r.do(http.MethodPost, "/mark", at("In Japanese, the ", " marks what the sentence"))
			mkStatus(t, first, http.StatusCreated)
			second := r.do(http.MethodPost, "/mark", at("follows. The ", " again."))
			mkStatus(t, second, http.StatusCreated)
			one, two := mkDecode[mkAnswer](t, first), mkDecode[mkAnswer](t, second)
			if one.ID == two.ID {
				t.Fatalf("the same words at two places were answered with one mark, %s", one.ID)
			}
			if again := r.do(http.MethodPost, "/mark", at("follows. The ", " again.")); again.Code != http.StatusOK || mkDecode[mkAnswer](t, again).ID != two.ID {
				t.Errorf("the second place pressed again = %d %s, want 200 and the second mark", again.Code, again.Body.String())
			}
			if n := len(r.mkLines()); n != 2 {
				t.Errorf("the vault holds %d lines, want 2", n)
			}
		})
	}
}

// A write whose vault file cannot be replaced after the ledger line is recorded
// is kept: the ledger is the commit point, so the answer is a success and the
// mark is not offered again to be kept twice. The next page that reads the marks
// puts it in the vault.
func TestMkAWriteTheVaultCannotTakeAfterTheLedgerLineIsKept(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("a folder's mode does not stop the owner on this system")
	}
	t.Parallel()
	r := newMkRig(t)
	r.mkThought("the first")
	// #nosec G302 -- a test restores its own temporary folder so that it can be removed
	t.Cleanup(func() { _ = os.Chmod(r.mkMarksFolder(), 0o700) }) //nolint:errcheck // so the folder can be removed
	// The folder is open to a write when the request starts, and refuses the
	// file once the write has read what is there: a disk that fills, a share that
	// goes away.
	r.mu.Lock()
	r.onReading = func() {
		if err := os.Chmod(r.mkMarksFolder(), 0o500); err != nil { // #nosec G302 -- a test makes its own folder unwritable
			t.Error(err)
		}
	}
	r.mu.Unlock()

	response := r.do(http.MethodPost, "/mark", `{"kind":"thought","path":"`+mkNote+`","anchor":"section-one","body":"the second"}`)
	if response.Code != http.StatusCreated {
		t.Fatalf("POST = %d %s, want 201: the ledger has the line, so it is kept", response.Code, response.Body.String())
	}
	if r.mkHolds("the second") {
		t.Fatal("the vault took the file: the test did not make the write fail after the ledger line")
	}
	kept := mkDecode[mkAnswer](t, response)
	if len(kept.Seen) == 0 {
		t.Errorf("the answer has no seen: %s", response.Body.String())
	}
	if read := r.do(http.MethodGet, "/mark/"+kept.ID, ""); read.Code != http.StatusOK {
		t.Errorf("GET of the kept mark = %d, want 200", read.Code)
	}
	if err := os.Chmod(r.mkMarksFolder(), 0o700); err != nil { // #nosec G302 -- a test restores its own folder
		t.Fatal(err)
	}
	r.kept.Mend()
	if !r.mkHolds("the second") {
		t.Errorf("after the folder can be written to, the next page does not put the mark in the vault:\n%v", r.mkFiles())
	}
}

// Where the reader folder cannot be used the reader still sees the marks they
// have: the earlier file's, read and never written, and the writes are refused.
func TestMkAReaderFolderThatCannotBeUsedStillShowsTheEarlierMarks(t *testing.T) {
	t.Parallel()
	for name, spoil := range map[string]func(*testing.T, string){
		"a link": func(t *testing.T, root string) {
			t.Helper()
			if err := os.Symlink(t.TempDir(), filepath.Join(root, "reader-notes")); err != nil {
				t.Skipf("links are not made here: %v", err)
			}
		},
		"a file": func(t *testing.T, root string) {
			t.Helper()
			if err := os.WriteFile(filepath.Join(root, "reader-notes"), []byte("x"), 0o600); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := newMkRig(t)
			r.writeEarlier(`{"version":1,"vault":"/somewhere","marks":[{"id":"` + strings.Repeat("a", 32) + `","kind":"question","path":"` + mkNote + `","anchor":"section-one","at":"2026-09-22T10:00:00Z"}]}`)
			spoil(t, r.root)
			list := r.do(http.MethodGet, "/questions", "")
			mkStatus(t, list, http.StatusOK)
			if !strings.Contains(list.Body.String(), mkNote) {
				t.Errorf("GET /questions = %s, want the earlier question", list.Body.String())
			}
			mkStatus(t, r.do(http.MethodPost, "/mark", `{"kind":"thought","path":"`+mkNote+`","anchor":"section-one","body":"x"}`), http.StatusUnprocessableEntity)
		})
	}
}

// mkSetAside is the words of the values this computer's ledger holds aside.
func (r *mkRig) mkSetAside() []string {
	r.t.Helper()
	view, err := r.kept.View()
	if err != nil {
		r.t.Fatal(err)
	}
	var words []string
	for _, e := range view.Entries(mark.AllMarks) {
		for i := range e.Aside {
			words = append(words, e.Aside[i].Mark.Body)
		}
	}
	return words
}

// mkRead is a GET of a mark: what a page that reads it has read, which is the
// newest line of every writer.
func (r *mkRig) mkRead(id string) mkAnswer {
	r.t.Helper()
	response := r.do(http.MethodGet, "/mark/"+id, "")
	mkStatus(r.t, response, http.StatusOK)
	return mkDecode[mkAnswer](r.t, response)
}

// A mark is in conflict: this computer's own "X2 from A" and B's newer "Y from
// B", and the reader is shown only Y. A page that read it says it read both, and
// a resolve sent from it is built on Y, as it should. The new line of this
// computer replaces its older one by the rule that a writer's newer line always
// does, and "X2 from A" would then be in no file: it is set aside in the ledger
// first, durably, so it is recoverable, and the answer is still 200.
func TestMkAResolveFromAPageThatReadTheConflictSetsThisComputersHiddenValueAside(t *testing.T) {
	t.Parallel()
	a, id, _ := mkDiverged(t)
	page := a.mkRead(id)
	if page.Body != "Y from B" {
		t.Fatalf("the page is shown %q, want B's newer value", page.Body)
	}

	response := a.do(http.MethodPatch, "/mark/"+id, `{"seen":`+mkSeenJSON(t, page.Seen)+`,"resolved":true}`)
	mkStatus(t, response, http.StatusOK)

	if a.mkHolds("X2 from A") {
		t.Fatal("the test did not reach the case: this computer's file still holds the hidden value")
	}
	if got := a.mkSetAside(); !slices.Contains(got, "X2 from A") {
		t.Errorf("the values set aside are %q, want this computer's hidden value among them", got)
	}
	if got := mkDecode[mkAnswer](t, response); got.Body != "Y from B" {
		t.Errorf("the answer shows %q, want what the reader is shown: %q", got.Body, "Y from B")
	}
}

// The same for a delete from a page that read the conflict.
func TestMkADeleteFromAPageThatReadTheConflictSetsThisComputersHiddenValueAside(t *testing.T) {
	t.Parallel()
	a, id, _ := mkDiverged(t)
	page := a.mkRead(id)

	mkStatus(t, a.do(http.MethodDelete, "/mark/"+id, `{"seen":`+mkSeenJSON(t, page.Seen)+`}`), http.StatusOK)

	if a.mkHolds("X2 from A", `"deleted":false`) || a.mkHolds("X2 from A", `"rev":2`) {
		t.Fatal("the test did not reach the case: this computer's file still holds the hidden value")
	}
	if got := a.mkSetAside(); !slices.Contains(got, "X2 from A") {
		t.Errorf("the values set aside are %q, want this computer's hidden value among them", got)
	}
}

// An ordinary change of a mark that is not in conflict sets nothing aside: the
// value replaced is the one the page was built on.
func TestMkAnOrdinaryChangeSetsNothingAside(t *testing.T) {
	t.Parallel()
	r := newMkRig(t)
	first := r.mkThought("first")
	second := r.do(http.MethodPatch, "/mark/"+first.ID, `{"seen":`+mkSeenJSON(t, first.Seen)+`,"body":"second"}`)
	mkStatus(t, second, http.StatusOK)
	next := mkDecode[mkAnswer](t, second)
	mkStatus(t, r.do(http.MethodPatch, "/mark/"+first.ID, `{"seen":`+mkSeenJSON(t, next.Seen)+`,"body":"third"}`), http.StatusOK)
	mkStatus(t, r.do(http.MethodDelete, "/mark/"+first.ID, `{"seen":`+mkSeenJSON(t, r.mkRead(first.ID).Seen)+`}`), http.StatusOK)
	if got := r.mkSetAside(); len(got) != 0 {
		t.Errorf("the values set aside are %q, want none", got)
	}
}

// The answer to a change carries the seen of the value it shows, so that the
// next change of the page is built on what the page is shown: after a delete
// that leaves B's value as the one the reader has, a change of those words is
// made on them, and not refused as a change of a deleted mark.
func TestMkTheAnswerToAChangeCarriesTheSeenOfTheValueItShows(t *testing.T) {
	t.Parallel()
	a, id, seen := mkDiverged(t)
	deleted := a.do(http.MethodDelete, "/mark/"+id, `{"seen":`+seen+`}`)
	mkStatus(t, deleted, http.StatusOK)
	shown := mkDecode[mkAnswer](t, deleted)
	if shown.Body != "Y from B" || shown.Deleted {
		t.Fatalf("the answer shows %q deleted=%v, want B's live value", shown.Body, shown.Deleted)
	}
	next := a.do(http.MethodPatch, "/mark/"+id, `{"seen":`+mkSeenJSON(t, shown.Seen)+`,"body":"Y, changed by the reader"}`)
	mkStatus(t, next, http.StatusOK)
	if got := a.mkRead(id); got.Body != "Y, changed by the reader" || got.Deleted {
		t.Errorf("the mark reads %q deleted=%v, want the words the reader changed", got.Body, got.Deleted)
	}
}
