package pages

import (
	"bytes"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/wording"
)

func TestDeclaredByBlockKeepsDeclaringNoteAndSourcePlaceDistinct(t *testing.T) {
	t.Parallel()
	v := NoteView{DeclaredBy: []DeclaringNoteView{{
		Note:      nav.NoteRef{Name: "My thought", RelPath: "Writing/My thought.md"},
		Locations: []DeclaredPlaceView{{Label: "Chapter", Href: "/notes/Sources/Book.md#chapter"}},
	}}}
	var html bytes.Buffer
	if err := declaredByBlock(v, wording.En).Render(t.Context(), &html); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`data-declared-by`,
		`href="/notes/Writing/My%20thought.md"`,
		`href="/notes/Sources/Book.md#chapter"`,
	} {
		if !strings.Contains(html.String(), want) {
			t.Errorf("declared source block lacks %q: %s", want, html.String())
		}
	}
}
