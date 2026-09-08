package pages

import (
	"bytes"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/wording"
)

func TestDeclaredSourceAndTextCitationLabelsDiffer(t *testing.T) {
	t.Parallel()

	declared := wording.BasedOn.In(wording.ZhHant)
	cited := wording.CitedBy.In(wording.ZhHant)
	islands := wording.IslandsTitle.In(wording.ZhHant)
	if declared == cited || declared == islands {
		t.Fatalf("declared-source %q must differ from cited-in-the-text labels %q and %q", declared, cited, islands)
	}

	var noteBuf bytes.Buffer
	noteView := NoteView{
		Title:         "Derived model",
		RelPath:       "Concepts/yomihon/Derived model.md",
		VaultHasLinks: true,
		BasedOn:       []nav.NoteRef{{Name: "Source model", RelPath: "Concepts/yomihon/Source model.md"}},
		CitedBy:       []nav.NoteRef{{Name: "Other", RelPath: "Concepts/yomihon/Other.md"}},
	}
	if err := Note(noteView, layouts.Chrome{}).Render(t.Context(), &noteBuf); err != nil {
		t.Fatalf("render note: %v", err)
	}
	noteHTML := noteBuf.String()
	if !strings.Contains(noteHTML, declared) {
		t.Errorf("the note page does not show the declared-source label %q", declared)
	}
	if !strings.Contains(noteHTML, cited) {
		t.Errorf("the note page does not show the text-citation label %q", cited)
	}
	if !strings.Contains(noteHTML, `class="y-basedon"`) {
		t.Error("the note page has no declared-source block")
	}
	if !strings.Contains(noteHTML, `href="/notes/Concepts/yomihon/Source`) {
		t.Error("a resolved source is not a link on the note page")
	}

	var healthBuf bytes.Buffer
	healthView := HealthView{
		Islands:     []HealthIslandGroup{{Dir: "Concepts/yomihon", Notes: []nav.NoteRef{{Name: "Source model", RelPath: "Concepts/yomihon/Source model.md"}}}},
		IslandCount: 1,
	}
	if err := Health(healthView, layouts.Chrome{}).Render(t.Context(), &healthBuf); err != nil {
		t.Fatalf("render health: %v", err)
	}
	healthHTML := healthBuf.String()
	if !strings.Contains(healthHTML, islands) {
		t.Errorf("the health list does not show the text-citation island label %q", islands)
	}
	if strings.Contains(healthHTML, "沒有人連過來的筆記") {
		t.Error("the health list still uses the old unscoped island heading")
	}
	if strings.Contains(healthHTML, declared) && !strings.Contains(healthHTML, wording.IslandsLede.In(wording.ZhHant)) {
		t.Error("the health list names declared sources without saying they are out of this list's scope")
	}
}

func TestDeclaredSourceRendersAuthorTextWhenUnlinked(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	view := NoteView{
		BasedOn: []nav.NoteRef{{Name: "[[twin]]"}, {Name: "Book notes", RelPath: "Book notes.md"}},
	}
	if err := Note(view, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, "[[twin]]") {
		t.Errorf("the author's ambiguous value is missing: %q", html)
	}
	if strings.Contains(html, `href="/notes/A/twin.md"`) || strings.Contains(html, `href="/notes/B/twin.md"`) {
		t.Error("an unlinked declared source was guessed into a href")
	}
	if !strings.Contains(html, `href="/notes/Book`) {
		t.Error("the resolved bare name is not a link")
	}
}
