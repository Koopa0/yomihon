package pages

import (
	"bytes"
	"strings"
	"testing"

	"github.com/a-h/templ"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/wording"
)

// unnamedKind is a resolution outcome this package has no words for. It is
// deliberately far past the four navigation declares, so it stands for the
// member somebody adds there later rather than for an off-by-one.
const unnamedKind = nav.EntryKind(200)

// TestARailRowReportsAResolutionItHasNoWordsFor holds the policy for a value
// arriving from another package's enum. EntryKind belongs to navigation and is
// eight bits wide: a member added there compiles here without a word, and the
// three answers a warning row needs used to abort on it — so a single new
// outcome would have taken down the reading page, the folder pages, search,
// reports, Home and the course page together, every one of which draws a rail.
//
// The row reports the value it does not recognise and keeps drawing. Yomihon's
// whole job on this surface is saying what it found.
func TestARailRowReportsAResolutionItHasNoWordsFor(t *testing.T) {
	t.Parallel()

	if unnamedKind == nav.EntryResolved || unnamedKind == nav.EntryUnresolved ||
		unnamedKind == nav.EntryAmbiguous || unnamedKind == nav.EntryNonInstance {
		t.Fatalf("the fixture kind %d is one navigation declares, so this test asserts nothing", unnamedKind)
	}

	t.Run("the three answers report rather than abort", func(t *testing.T) {
		t.Parallel()
		if got := entryResolutionCode(unnamedKind); got != "200" {
			t.Errorf("entryResolutionCode(%d) = %q, want the value itself", unnamedKind, got)
		}
		if got := entryResolutionLabel(unnamedKind, wording.ZhHant); got != "200" {
			t.Errorf("entryResolutionLabel(%d) = %q, want the value itself", unnamedKind, got)
		}
		if got := entryResolutionTitle(unnamedKind, wording.ZhHant); got != "" {
			t.Errorf("entryResolutionTitle(%d) = %q, want nothing to explain", unnamedKind, got)
		}
	})

	sb := NewSidebar(nil, "", wording.ZhHant)
	for name, component := range map[string]templ.Component{
		"a course row": pathEntryLink(sb, &nav.PathEntry{Text: "Lesson", Kind: unnamedKind}),
		"a map row":    entryLink(sb, nav.MapEntry{Text: "Entry", Kind: unnamedKind}),
	} {
		t.Run(name+" still draws", func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			if err := component.Render(t.Context(), &buf); err != nil {
				t.Fatalf("render %s: %v", name, err)
			}
			html := buf.String()
			for _, want := range []string{`data-resolution="200"`, `>200</span>`} {
				if !strings.Contains(html, want) {
					t.Errorf("%s does not report the resolution it has no words for — %q is missing; html = %q", name, want, html)
				}
			}
		})
	}
}
