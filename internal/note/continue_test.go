package note_test

import (
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/mark"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/wording"
)

// deskWithMark serves a desk over a one-note vault, with the place the reader
// kept already in hand. The mark never reaches a file here: what the row is
// built from is the closure, which is the seam this exercises.
func deskWithMark(t *testing.T, body string, kept *mark.Continuation, has bool) string {
	t.Helper()
	return deskOverNote(t, "alpha.md", body, kept, has)
}

// deskOverNote is deskWithMark with the note's own filename, for the one case
// that is about what a name does to an address.
func deskOverNote(t *testing.T, name, body string, kept *mark.Continuation, has bool) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "Notes"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Notes", name), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	srv := newServerWithMark(t, root, nil, schema.Ungoverned(),
		func() (mark.Continuation, bool) { return *kept, has })

	response, err := http.Get(srv.URL + "/") //nolint:noctx // the test server is torn down by the helper
	if err != nil {
		t.Fatalf("GET the desk: %v", err)
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	}()
	page, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read the desk: %v", err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET the desk = %d, want 200", response.StatusCode)
	}
	return string(page)
}

const aNote = "---\ntitle: Alpha\n---\n\n# Alpha\n\nSome words.\n"

func keptIn(path, anchor string, offset int, identity string) *mark.Continuation {
	return &mark.Continuation{
		RelPath:  path,
		Anchor:   anchor,
		Offset:   offset,
		Identity: identity,
		At:       time.Now(),
	}
}

// TestADeskWithNoMarkSaysNothingAboutOne holds the quiet case. A reader who
// has kept no place is not shown an empty row advertising a control that lives
// on another page.
func TestADeskWithNoMarkSaysNothingAboutOne(t *testing.T) {
	t.Parallel()

	page := deskWithMark(t, aNote, &mark.Continuation{}, false)
	if strings.Contains(page, "data-home-continue") {
		t.Error("the desk draws a way back when the reader has kept no place")
	}
	if strings.Contains(page, wording.ContinueReading.In(wording.ZhHant)) {
		t.Error("the desk names the way back when there is none")
	}
}

// TestTheDeskOffersTheKeptPlaceBack is the round trip's server half: the row
// names the note, leads to it, carries the anchor and the distance, and says
// where the mark is kept.
func TestTheDeskOffersTheKeptPlaceBack(t *testing.T) {
	t.Parallel()

	page := deskWithMark(t, aNote,
		keptIn("Notes/alpha.md", "alpha", 320, identityOf(aNote)), true)

	if !strings.Contains(page, "data-home-continue") {
		t.Fatal("the desk offers no way back to the place that was kept")
	}
	if !strings.Contains(page, "Alpha") {
		t.Error("the row does not name the note it leads to")
	}
	if !strings.Contains(page, "/notes/Notes/alpha.md?at=320#alpha") {
		t.Errorf("the row does not lead to the anchor and the distance that were kept; page:\n%s", rowOf(page))
	}
	if !strings.Contains(page, wording.MarkPerDevice.In(wording.ZhHant)) {
		t.Error("the row does not say that the place is kept on this device only")
	}
	if strings.Contains(page, wording.MarkNoteChanged.In(wording.ZhHant)) {
		t.Error("the row says the note changed when its bytes are the ones the mark was taken against")
	}
}

// TestAChangedNoteIsSaidAndStillOffered holds the ruled behaviour: an identity
// that no longer matches still goes to the mark, because it is the reader's
// own best pointer, and the row says why the place may have moved.
func TestAChangedNoteIsSaidAndStillOffered(t *testing.T) {
	t.Parallel()

	page := deskWithMark(t, aNote,
		keptIn("Notes/alpha.md", "alpha", 320, strings.Repeat("f", 64)), true)

	if !strings.Contains(page, wording.MarkNoteChanged.In(wording.ZhHant)) {
		t.Error("the note changed since the place was kept and the row says nothing about it")
	}
	if !strings.Contains(page, "data-continue-link") {
		t.Error("a changed note withheld the way back, which is the reader's own pointer")
	}
}

// TestTheStatusMovingDoesNotAgeAMark holds the half of the identity decision
// that makes it usable: the note's own status is spliced out of the identity,
// so an author walking a note through its lifecycle does not make every mark
// look stale.
func TestTheStatusMovingDoesNotAgeAMark(t *testing.T) {
	t.Parallel()

	const draft = "---\ntitle: Alpha\nstatus: draft\n---\n\n# Alpha\n\nSome words.\n"
	const ready = "---\ntitle: Alpha\nstatus: ready\n---\n\n# Alpha\n\nSome words.\n"
	if draft == ready {
		t.Fatal("the two versions are the same bytes, so this proves nothing")
	}
	// The mark is taken against the draft and the desk is served over the
	// version whose status has moved on.
	page := deskWithMark(t, ready,
		keptIn("Notes/alpha.md", "alpha", 320, identityOf(draft)), true)

	if strings.Contains(page, wording.MarkNoteChanged.In(wording.ZhHant)) {
		t.Errorf("a status flip aged the mark; the row says the note changed:\n%s", rowOf(page))
	}
}

// TestAMarkedNoteThatIsGoneSaysSo covers the other end: the note the place was
// kept in is no longer in the vault. Nothing is cleared — the reader did not
// ask for that — and the row stops being a link.
func TestAMarkedNoteThatIsGoneSaysSo(t *testing.T) {
	t.Parallel()

	page := deskWithMark(t, aNote,
		keptIn("Notes/vanished.md", "somewhere", 10, strings.Repeat("a", 64)), true)

	if !strings.Contains(page, wording.MarkNoteGone.In(wording.ZhHant)) {
		t.Error("the marked note is gone and the row does not say so")
	}
	if strings.Contains(page, "data-continue-link") {
		t.Error("the row still offers a way into a note that is not there")
	}
	if !strings.Contains(page, "Notes/vanished.md") {
		t.Error("the row does not name what the reader had kept")
	}
}

// TestTheAddressTheRowCarriesIsEscaped holds the one place a vault-authored
// name reaches an address this package builds.
func TestTheAddressTheRowCarriesIsEscaped(t *testing.T) {
	t.Parallel()

	page := deskOverNote(t, "a b.md", aNote,
		keptIn("Notes/a b.md", "one two", 5, identityOf(aNote)), true)

	if !strings.Contains(page, "data-continue-link") {
		t.Fatalf("the note is in the vault and the row offers no way into it; row:\n%s", rowOf(page))
	}
	if strings.Contains(page, "/notes/Notes/a b.md") {
		t.Error("a name carrying a space reached the address unescaped")
	}
	if !strings.Contains(page, "/notes/Notes/"+url.PathEscape("a b.md")) {
		t.Errorf("the row does not carry the escaped name; row:\n%s", rowOf(page))
	}
	if !strings.Contains(page, "#"+url.PathEscape("one two")) {
		t.Errorf("the row does not carry the escaped anchor; row:\n%s", rowOf(page))
	}
}

// rowOf cuts the desk down to the row, so a failure prints what it is about
// rather than a whole page.
func rowOf(page string) string {
	_, after, found := strings.Cut(page, "data-home-continue")
	if !found {
		return "(the page draws no way back)"
	}
	before, _, _ := strings.Cut(after, "</section>")
	return before
}
