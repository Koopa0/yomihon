package note_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/wording"
)

// KOO-40 lock: a declared based_on source is walkable on the reading page, and
// an unresolved or ambiguous value stays the author's own text. The two scope
// labels — declared source versus cited in the text — render as different
// words on the note page and on the health list.
func TestDeclaredSourcesOnTheReadingPage(t *testing.T) {
	t.Parallel()

	root := writeNotes(t, map[string]string{
		"Scratch.md":    "see [[Book notes]]\n",
		"Book notes.md": concept("Book notes", "based_on: \"[[Book notes]]\"", "the source notebook"),
		"Concepts/yomihon/Source model.md": concept(
			"Source model",
			`based_on: "[[Book notes]]"`,
			"derived from the notebook, with no body wikilink",
		),
		"Concepts/yomihon/Derived model.md": concept(
			"Derived model",
			`based_on: "[[Source model]]"`,
			"derived from the source model, with no body wikilink",
		),
		"Concepts/yomihon/Bare name.md": concept(
			"Bare name",
			"based_on:\n  - Book notes",
			"a list whose member is a bare name, not a wikilink",
		),
		"Concepts/yomihon/Ambiguous.md": concept(
			"Ambiguous",
			`based_on: "[[twin]]"`,
			"the name two files answer to",
		),
		"A/twin.md": "one\n",
		"B/twin.md": "two\n",
	})
	srv := newServerWithContract(t, root, loadHomeContract(t))

	t.Run("a resolved wikilink is a title and a link", func(t *testing.T) {
		t.Parallel()
		code, body := get(t, srv.Client(), srv.URL+"/notes/Concepts/yomihon/Derived%20model.md")
		if code != http.StatusOK {
			t.Fatalf("status = %d, want 200", code)
		}
		block := basedOnBlock(t, body)
		if !strings.Contains(block, wording.BasedOn.In(wording.ZhHant)) {
			t.Errorf("the declaring note does not label its sources; block = %q", block)
		}
		if !strings.Contains(block, `href="/notes/Concepts/yomihon/Source`) {
			t.Errorf("the resolved source is not a link; block = %q", block)
		}
		if !strings.Contains(block, "Source model") {
			t.Errorf("the resolved source is not named; block = %q", block)
		}
	})

	t.Run("a bare name that resolves is a title and a link", func(t *testing.T) {
		t.Parallel()
		code, body := get(t, srv.Client(), srv.URL+"/notes/Concepts/yomihon/Bare%20name.md")
		if code != http.StatusOK {
			t.Fatalf("status = %d, want 200", code)
		}
		block := basedOnBlock(t, body)
		if !strings.Contains(block, `href="/notes/Book`) {
			t.Errorf("the bare name did not become a link; block = %q", block)
		}
	})

	t.Run("an ambiguous value stays the author's text, unlinked", func(t *testing.T) {
		t.Parallel()
		code, body := get(t, srv.Client(), srv.URL+"/notes/Concepts/yomihon/Ambiguous.md")
		if code != http.StatusOK {
			t.Fatalf("status = %d, want 200", code)
		}
		block := basedOnBlock(t, body)
		if !strings.Contains(block, "twin") {
			t.Errorf("the author's ambiguous value is missing; block = %q", block)
		}
		if strings.Contains(block, `href="/notes/A/twin.md"`) || strings.Contains(block, `href="/notes/B/twin.md"`) {
			t.Errorf("an ambiguous source was guessed into a link; block = %q", block)
		}
	})
}

func TestCitationScopeLabelsDifferOnNoteAndHealth(t *testing.T) {
	t.Parallel()

	root := writeNotes(t, map[string]string{
		"Scratch.md":    "see [[Book notes]]\n",
		"Book notes.md": concept("Book notes", `based_on: "[[Book notes]]"`, "the source notebook"),
		"Concepts/yomihon/Source model.md": concept(
			"Source model",
			`based_on: "[[Book notes]]"`,
			"no body wikilink, so backlinks count nothing",
		),
		"Concepts/yomihon/Derived model.md": concept(
			"Derived model",
			`based_on: "[[Source model]]"`,
			"declares Source model, still with no body wikilink",
		),
	})
	srv := newServerWithContract(t, root, loadHomeContract(t))

	declared := wording.BasedOn.In(wording.ZhHant)
	cited := wording.CitedBy.In(wording.ZhHant)
	islands := wording.IslandsTitle.In(wording.ZhHant)
	if declared == cited {
		t.Fatal("the declared-source label and the text-citation label are the same words")
	}
	if declared == islands {
		t.Fatal("the declared-source label and the health island label are the same words")
	}
	if cited == islands {
		// They name the same scope from two faces; they still must not be one
		// string, or a reader cannot tell which surface they are on.
		t.Fatal("the note-page text-citation label and the health island label are the same words")
	}

	code, derived := get(t, srv.Client(), srv.URL+"/notes/Concepts/yomihon/Derived%20model.md")
	if code != http.StatusOK {
		t.Fatalf("derived status = %d, want 200", code)
	}
	if !strings.Contains(derived, declared) {
		t.Errorf("the declaring note does not show the declared-source label %q", declared)
	}
	if !strings.Contains(derived, cited) {
		t.Errorf("the declaring note does not show the text-citation label %q", cited)
	}

	code, source := get(t, srv.Client(), srv.URL+"/notes/Concepts/yomihon/Source%20model.md")
	if code != http.StatusOK {
		t.Fatalf("source status = %d, want 200", code)
	}
	citedBlock := citedByBlock(t, source)
	if !strings.Contains(citedBlock, wording.CitedByNone.In(wording.ZhHant)) {
		t.Errorf("the source note's backlinks do not say they count text citations; block = %q", citedBlock)
	}
	if strings.Contains(citedBlock, `href="/notes/Concepts/yomihon/Derived`) {
		t.Errorf("backlinks mixed in a based_on declaration; block = %q", citedBlock)
	}

	code, health := get(t, srv.Client(), srv.URL+"/health")
	if code != http.StatusOK {
		t.Fatalf("health status = %d, want 200", code)
	}
	if !strings.Contains(health, islands) {
		t.Errorf("the health list does not show the text-citation island label %q", islands)
	}
	if strings.Contains(health, "沒有人連過來的筆記") {
		t.Error("the health list still uses the old unscoped island heading")
	}
	section := healthSectionBody(t, health, islands)
	if !strings.Contains(section, "Source model") {
		t.Errorf("a note declared as a source is missing from the text-citation island list; section = %q", section)
	}
}

func concept(title, basedOn, body string) string {
	return "---\ntitle: " + title + "\ntype: concept\ndomain: golang\nstatus: draft\n" + basedOn + "\n---\n\n" + body + "\n"
}

func basedOnBlock(t *testing.T, body string) string {
	t.Helper()
	const open = `<nav class="y-basedon"`
	start := strings.Index(body, open)
	if start < 0 {
		t.Fatal("the reading page has no declared-source block at all")
	}
	end := strings.Index(body[start:], "</nav>")
	if end < 0 {
		t.Fatal("the declared-source block is not closed")
	}
	return body[start : start+end]
}
