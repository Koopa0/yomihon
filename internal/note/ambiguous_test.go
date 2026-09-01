package note_test

import (
	"net/http"
	"slices"
	"strings"
	"testing"
)

// ambiguousVault holds one name two files answer to, and a citation written
// against it.
func ambiguousVault(t *testing.T) string {
	t.Helper()
	body := "---\ntitle: %s\ntype: concept\ndomain: golang\nstatus: draft\ncreated: 2026-06-01\nupdated: 2026-06-01\nbased_on: \"[[x]]\"\n---\n\nbody\n"
	return writeNotes(t, map[string]string{
		"Concepts/golang/Citing.md":     "---\ntitle: Citing\ntype: concept\ndomain: golang\nstatus: draft\ncreated: 2026-06-01\nupdated: 2026-06-01\nbased_on: \"[[x]]\"\n---\n\n見 [[Repeated]]。\n",
		"Concepts/golang/A/Repeated.md": strings.Replace(body, "%s", "One", 1),
		"Concepts/golang/B/Repeated.md": strings.Replace(body, "%s", "Two", 1),
	})
}

// TestAnAmbiguousLinkIsAudible holds the ambiguous link to what a broken one
// already does: carry its explanation twice, once for a pointer and once for
// anyone listening. Only the pointer was served, so a reader who does not use
// one was told nothing at all about a link the page had visibly degraded.
func TestAnAmbiguousLinkIsAudible(t *testing.T) {
	t.Parallel()

	srv := newServerWithContract(t, ambiguousVault(t), loadHomeContract(t))
	code, page := get(t, srv.URL+"/notes/Concepts/golang/Citing.md")
	if code != http.StatusOK {
		t.Fatalf("note page status = %d, want %d", code, http.StatusOK)
	}
	if !strings.Contains(page, `class="wikilink-ambiguous"`) {
		t.Fatal("the page carries no ambiguous link, so this proves nothing about how one reads")
	}
	span := ambiguousSpan(t, page)
	if !strings.Contains(span, "y-offscreen") {
		t.Error("the explanation is carried only where a pointer can reach it")
	}
	// The sentence, not a bare list of paths: a title attribute holding only
	// "A/Repeated.md, B/Repeated.md" says nothing about what went wrong.
	if !strings.Contains(span, "指向不只一個檔案") {
		t.Error("the link does not say what is wrong with it, only where it might have gone")
	}
	// A span is not a control. A tab stop that opens nothing would be an
	// affordance in name only.
	if strings.Contains(span, "tabindex") {
		t.Error("the ambiguous span takes a tab stop while doing nothing when focused")
	}
}

// TestAnAmbiguousLinkIsNotToldItIsATitle keeps the two vocabularies apart. A
// name placing several files is followed perfectly well and arrives in more
// than one place; a name matching a note's title is not followed at all. The
// repairs differ — rename one file, or add an alias — so the sentences must.
func TestAnAmbiguousLinkIsNotToldItIsATitle(t *testing.T) {
	t.Parallel()

	srv := newServerWithContract(t, ambiguousVault(t), loadHomeContract(t))
	_, page := get(t, srv.URL+"/notes/Concepts/golang/Citing.md")
	if strings.Contains(page, "是〈") || strings.Contains(page, "篇筆記共同的 title") {
		t.Error("an ambiguous name is described in the words used for a name that matched a title")
	}
	if strings.Contains(page, "還沒有") {
		t.Error("an ambiguous name is described as a note that does not exist")
	}
}

// TestHealthTellsTwoCollidingFilesApart is the one list in the product whose
// rows are guaranteed to share a label: a collision is several files answering
// to one name. Naming each by that name prints the same word twice and leaves
// the reader to hover the links to find out which is which.
func TestHealthTellsTwoCollidingFilesApart(t *testing.T) {
	t.Parallel()

	srv := newServerWithContract(t, ambiguousVault(t), loadHomeContract(t))
	_, page := get(t, srv.URL+"/health")
	section := healthSectionBody(t, page, "兩個檔案共用的名字")
	shown := linkTexts(t, section)
	if len(shown) < 2 {
		t.Fatalf("the section holds %d links, so there is no pair to tell apart: %q", len(shown), section)
	}
	// The words a reader sees, not the href they cannot. Two rows reading the
	// same thing are two rows nobody can choose between.
	if shown[0] == shown[1] {
		t.Errorf("both rows read %q, so the list a reader looks at does not distinguish them", shown[0])
	}
	for _, want := range []string{"Concepts/golang/A/Repeated.md", "Concepts/golang/B/Repeated.md"} {
		if !slices.Contains(shown, want) {
			t.Errorf("no row reads %q; rows read %v", want, shown)
		}
	}
}

// ambiguousSpan cuts out exactly the ambiguous link's own markup. A window
// measured in characters runs past it into whatever the page puts next, and
// this page carries offscreen text in several places — so a looser slice
// answers about the wrong element.
func ambiguousSpan(t *testing.T, page string) string {
	t.Helper()
	at := strings.Index(page, `<span class="wikilink-ambiguous"`)
	if at < 0 {
		t.Fatal("the page carries no ambiguous link, so this proves nothing about how one reads")
	}
	rest := page[at:]
	depth := 0
	for i := range len(rest) {
		switch {
		case strings.HasPrefix(rest[i:], "<span"):
			depth++
		case strings.HasPrefix(rest[i:], "</span>"):
			depth--
			if depth == 0 {
				return rest[:i+len("</span>")]
			}
		}
	}
	t.Fatal("the ambiguous link's markup does not close")
	return ""
}

// linkTexts reads the words each link in a fragment shows, which is what a
// reader has to choose between — an href carries the path whether or not
// anything on screen does.
func linkTexts(t *testing.T, fragment string) []string {
	t.Helper()
	var out []string
	for _, chunk := range strings.Split(fragment, "<a ")[1:] {
		open := strings.IndexByte(chunk, '>')
		shut := strings.Index(chunk, "</a>")
		if open < 0 || shut < 0 || shut < open {
			t.Fatalf("a link in the fragment does not close: %q", chunk[:min(len(chunk), 120)])
		}
		out = append(out, chunk[open+1:shut])
	}
	return out
}
