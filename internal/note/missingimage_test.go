package note_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAMissingPictureIsSaidOnTheNotePage holds the two halves of one
// inconsistency the reading page used to carry. A note citing a name nobody has
// written says so where the citation stands and again in the conditions block
// beside the article; a note showing a picture the vault does not hold said
// nothing anywhere, and the image quietly asked for an address that answers 404.
//
// The fixture carries both faults for that reason: the broken citation is the
// control, and an assertion about the picture means little unless the page is
// shown doing for it what it already does for the link.
func TestAMissingPictureIsSaidOnTheNotePage(t *testing.T) {
	t.Parallel()

	const rel = "Concepts/golang/Shows.md"
	body := "---\ntitle: Shows\ntype: concept\ndomain: golang\nstatus: draft\ncreated: 2026-06-01\nupdated: 2026-06-01\nbased_on: \"[[Shows]]\"\n---\n\n" +
		"See [[No such note]].\n\n![a missing picture](./missing.png)\n\n![one that is there](./there.png)\n"

	root := t.TempDir()
	dir := filepath.Join(root, "Concepts", "golang")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Shows.md"), []byte(body), 0o600); err != nil {
		t.Fatalf("write note: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "there.png"), []byte("not really a picture"), 0o600); err != nil {
		t.Fatalf("write picture: %v", err)
	}

	srv := newServerWithContract(t, root, loadHomeContract(t))
	_, page := get(t, srv.Client(), srv.URL+"/notes/"+rel)

	// The control: the citation half of the same absence still speaks.
	if !strings.Contains(page, "wikilink-broken") {
		t.Fatal("the page does not mark the broken citation, so it is not the page this test means to check")
	}
	if !strings.Contains(page, `class="image-missing"`) {
		t.Error("the missing picture is not marked where the author put it")
	}
	if !strings.Contains(page, "還沒有「Concepts/golang/missing.png」這個檔案") {
		t.Error("the mark does not name the file the vault does not hold")
	}
	if !strings.Contains(page, "圖片的檔案不在書庫裡") {
		t.Error("the conditions block beside the article does not list the missing picture")
	}
	// The picture that is there stays an ordinary image: one mark on the page,
	// not one per image.
	if got := strings.Count(page, `class="image-missing"`); got != 1 {
		t.Errorf("the page carries %d missing-picture marks, want exactly the one", got)
	}
	if !strings.Contains(page, `src="/raw/Concepts/golang/there.png"`) {
		t.Error("the picture the vault holds was not rewritten to its bytes")
	}

	// The mark has to go around a whole element. Wrapping from the source's
	// closing quote instead put the opening span inside the tag: a browser then
	// read the marker as one of the image's own attributes, lost the alt text,
	// and printed the rest of the tag as words in the article. Asserting that
	// the mark is present says nothing about any of that, which is how it
	// shipped, so the bytes are pinned here whole.
	const marked = `<span class="image-missing" title="還沒有「Concepts/golang/missing.png」這個檔案">` +
		`<img src="/raw/Concepts/golang/missing.png" alt="a missing picture">` +
		`<span class="y-offscreen">（還沒有「Concepts/golang/missing.png」這個檔案）</span></span>`
	if !strings.Contains(page, marked) {
		t.Errorf("the marked picture is not the element it should be; the page carries:\n%s", markedFragment(page))
	}
	// And no attribute is left standing outside a tag. This is written as the
	// shape the failure actually produced, measured rather than guessed: the
	// mark wrapped from the source's closing quote left
	// `</span> alt="a missing picture">` in the body text, and an attribute
	// can never legitimately follow a ">".
	if strings.Contains(page, "> alt=") {
		t.Error("an image attribute stands outside its tag, so part of the markup is being shown as text")
	}
}

// markedFragment cuts the region around the missing-picture mark out of a page,
// so a failure shows the markup that was produced rather than the whole page.
func markedFragment(page string) string {
	i := strings.Index(page, `class="image-missing"`)
	if i < 0 {
		return "(no mark at all)"
	}
	start := max(i-120, 0)
	end := min(i+400, len(page))
	return page[start:end]
}

// TestAPictureNamedInAnotherNormalisationIsNotCalledMissing covers the spelling
// half. A vault holds its names composed, a note's own text can carry either
// spelling of the same letter, and the route that serves the bytes composes
// before it looks — so a decomposed address answers 200. A page marking it
// missing would be reporting its own reading of the name as a fault in the
// author's note.
func TestAPictureNamedInAnotherNormalisationIsNotCalledMissing(t *testing.T) {
	t.Parallel()

	// The same name twice: composed on disk, decomposed in the note's text.
	const composed = "é.png"
	const decomposed = "e\u0301.png"

	root := t.TempDir()
	dir := filepath.Join(root, "Concepts", "golang")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := "---\ntitle: Spelled\ntype: concept\ndomain: golang\nstatus: draft\ncreated: 2026-06-01\nupdated: 2026-06-01\nbased_on: \"[[Spelled]]\"\n---\n\n![a picture](./" + decomposed + ")\n"
	if err := os.WriteFile(filepath.Join(dir, "Spelled.md"), []byte(body), 0o600); err != nil {
		t.Fatalf("write note: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, composed), []byte("not really a picture"), 0o600); err != nil {
		t.Fatalf("write picture: %v", err)
	}

	srv := newServerWithContract(t, root, loadHomeContract(t))
	_, page := get(t, srv.Client(), srv.URL+"/notes/Concepts/golang/Spelled.md")
	if strings.Contains(page, `class="image-missing"`) {
		t.Error("a picture the vault holds under the composed spelling is reported missing")
	}
}
