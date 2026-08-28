package note_test

import (
	"strings"
	"testing"
)

// transitionEffectSentence is what both status faces say about the buttons
// beside it, before any of them is pressed.
const transitionEffectSentence = "這個按鈕只會改寫這篇筆記 frontmatter 的 status 欄位。"

// TestBothStatusFacesStateWhatTheirButtonsDo holds the sentence to the moment
// it is useful: a reader deciding whether to press. Both faces carry it,
// because a reader at a narrow width sees only one of them and is owed the same
// account of the control in front of them.
func TestBothStatusFacesStateWhatTheirButtonsDo(t *testing.T) {
	t.Parallel()
	page := terminalLessonPage(t, "draft")

	surfaces := statusSurfaces(t, page)
	if len(surfaces) == 0 {
		t.Fatal("the page rendered no status face, so this test asserts nothing")
	}
	for _, surface := range surfaces {
		if !strings.Contains(surface.body, `action="/status"`) {
			t.Fatalf("%s offers no transition, so there is no button for the sentence to describe", surface.name)
		}
		if !strings.Contains(surface.body, transitionEffectSentence) {
			t.Errorf("%s carries buttons without saying what they do; body = %q", surface.name, surface.body)
		}
	}
}

// TestNoButtonsMeansNoSentenceAboutThem is the control. A face that printed the
// line unconditionally would pass the test above and would describe, on a note
// the write face offers nothing onward from, a button that is not there.
func TestNoButtonsMeansNoSentenceAboutThem(t *testing.T) {
	t.Parallel()
	page := terminalLessonPage(t, "archived")

	if !strings.Contains(page, "目前沒有合法的狀態轉換。") {
		t.Fatal("the fixture no longer reaches the empty-transitions state, so this test asserts nothing")
	}
	if strings.Contains(page, transitionEffectSentence) {
		t.Error("a page with no legal transition still describes what pressing a button would do")
	}
}
