package render_test

import (
	"strings"
	"testing"

	"github.com/koopa0/kurodo/internal/render"
)

// conceptLookup is a tiny hand-written stand-in for the concept index: the set
// of concept paths and their slugs. Asserted on outputs only.
func conceptLookup(paths map[string]string) func(string) (string, bool) {
	return func(rel string) (string, bool) {
		s, ok := paths[rel]
		return s, ok
	}
}

func TestInjectConceptTriggersMarksConceptLink(t *testing.T) {
	t.Parallel()
	// A resolved wikilink to a concept note, as renderWikilink emits it (the
	// href is url.PathEscape'd per segment — note the %20 for the space).
	in := `read <a href="/notes/Concepts/japanese/%E3%81%AF%20%28%E4%B8%BB%E9%A1%8C%29.md" class="wikilink">は</a> here`
	lookup := conceptLookup(map[string]string{"Concepts/japanese/は (主題).md": "topic-ha"})
	out, refs := render.InjectConceptTriggers(in, lookup)

	if !strings.Contains(out, `class="wikilink concept-link" data-concept="topic-ha"`) {
		t.Errorf("concept link not marked with data-concept; got:\n%s", out)
	}
	// The href and display survive — it is still a real navigable link.
	if !strings.Contains(out, `href="/notes/Concepts/japanese/%E3%81%AF%20%28%E4%B8%BB%E9%A1%8C%29.md"`) || !strings.Contains(out, `>は</a>`) {
		t.Errorf("concept trigger lost its href or display (must stay navigable); got:\n%s", out)
	}
	if len(refs) != 1 || refs[0] != "Concepts/japanese/は (主題).md" {
		t.Errorf("refs = %v, want [Concepts/japanese/は (主題).md]", refs)
	}
}

func TestInjectConceptTriggersLeavesNonConceptLink(t *testing.T) {
	t.Parallel()
	// A resolved wikilink to a NON-concept note must stay exactly as rendered.
	in := `see <a href="/notes/Writing/lessons/japanese/L02.md" class="wikilink">L02</a>`
	out, refs := render.InjectConceptTriggers(in, conceptLookup(map[string]string{"Concepts/japanese/は.md": "ha"}))
	if out != in {
		t.Errorf("non-concept wikilink was altered:\nwant %q\ngot  %q", in, out)
	}
	if len(refs) != 0 {
		t.Errorf("refs = %v, want none for a non-concept link", refs)
	}
}

func TestInjectConceptTriggersDedupesRepeats(t *testing.T) {
	t.Parallel()
	// The same concept linked twice in a body yields one ref (one <template>).
	link := `<a href="/notes/Concepts/japanese/%E3%81%AF.md" class="wikilink">は</a>`
	in := link + " and again " + link
	out, refs := render.InjectConceptTriggers(in, conceptLookup(map[string]string{"Concepts/japanese/は.md": "ha"}))
	if n := strings.Count(out, "data-concept"); n != 2 {
		t.Errorf("both occurrences should be marked; got %d data-concept, want 2", n)
	}
	if len(refs) != 1 {
		t.Errorf("refs = %v, want a single deduped entry", refs)
	}
}

func TestInjectConceptTriggersIgnoresAmbiguousAndBroken(t *testing.T) {
	t.Parallel()
	// Ambiguous/broken wikilinks are <span>, never <a class="wikilink"> — they
	// can never become triggers. The lookup returns true for everything to prove
	// the shape, not the path, is what gates it.
	in := `<span class="wikilink-ambiguous" title="a, b">X</span><span class="wikilink-broken">Y</span>`
	out, refs := render.InjectConceptTriggers(in, func(string) (string, bool) { return "x", true })
	if out != in || len(refs) != 0 {
		t.Errorf("ambiguous/broken wikilinks must not become triggers:\ngot %q refs %v", out, refs)
	}
}
