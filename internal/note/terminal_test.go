package note_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/wording"
)

// terminalLessonPage renders the reading page for one lesson fixture at the
// given status and returns its body.
func terminalLessonPage(t *testing.T, noteStatus string) string {
	t.Helper()
	root := t.TempDir()
	lessonMD := "---\ntitle: L01\ntype: lesson\ndomain: japanese\nstatus: " + noteStatus +
		"\ncreated: 2026-06-01\nupdated: 2026-06-01\n---\n\nbody\n"
	dir := filepath.Join(root, "Writing", "lessons", "japanese")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "L01.md"), []byte(lessonMD), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	srv := newServerWithContract(t, root, loadContract(t))
	code, body := get(t, srv.URL+"/notes/Writing/lessons/japanese/L01.md")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	return body
}

// transitionFormMarkup cuts the transition form whose hidden "to" field
// carries target out of a rendered page.
func transitionFormMarkup(t *testing.T, body, target string) string {
	t.Helper()
	before, _, found := strings.Cut(body, `name="to" value="`+target+`"`)
	if !found {
		t.Fatalf("page has no transition form for target %q", target)
	}
	start := strings.LastIndex(before, "<form")
	if start < 0 {
		t.Fatalf("transition target %q is not inside a form", target)
	}
	end := strings.Index(body[start:], "</form>")
	if end < 0 {
		t.Fatalf("transition form for %q is unterminated", target)
	}
	return body[start : start+end]
}

// TestShowTerminalTargetNeedsASecondInteraction locks the two-step shape for
// a terminal target. From archived the write face offers nothing onward —
// terminality is derived from the contract's transitions, never configured —
// so the single press that used to install it is replaced by a native
// disclosure: the first interaction opens the confirm, the second submits.
// A non-terminal target keeps its quiet single press.
func TestShowTerminalTargetNeedsASecondInteraction(t *testing.T) {
	t.Parallel()
	body := terminalLessonPage(t, "draft")

	// draft offers ready and archived; only archived is terminal (ready still
	// has the archived edge out).
	archivedForm := transitionFormMarkup(t, body, "archived")
	if !strings.Contains(archivedForm, "<details") {
		t.Fatalf("terminal target renders without a disclosure; one press submits it:\n%s", archivedForm)
	}
	if strings.Contains(archivedForm, "<details open") {
		t.Errorf("the terminal confirm renders already open, so one interaction still reaches submit:\n%s", archivedForm)
	}
	summaryAt := strings.Index(archivedForm, "<summary")
	submitAt := strings.Index(archivedForm, `type="submit"`)
	if summaryAt < 0 || submitAt < 0 || submitAt < summaryAt {
		t.Errorf("the terminal submit is not behind the disclosure control (summary at %d, submit at %d):\n%s", summaryAt, submitAt, archivedForm)
	}
	for _, want := range []string{"不會再提供任何狀態轉換", "確認設為 archived"} {
		if !strings.Contains(archivedForm, want) {
			t.Errorf("terminal confirm is missing %q:\n%s", want, archivedForm)
		}
	}

	readyForm := transitionFormMarkup(t, body, "ready")
	if strings.Contains(readyForm, "<details") {
		t.Errorf("non-terminal target grew a confirm step:\n%s", readyForm)
	}
	if !strings.Contains(readyForm, `type="submit"`) {
		t.Errorf("non-terminal target lost its immediate submit:\n%s", readyForm)
	}
}

// TestShowZeroTransitionsNamesTheFrontmatterDoor locks the empty state's
// honest escape hatch: a note the write face offers nothing onward from
// states how recovery actually happens — a hand edit of the frontmatter,
// through the editor link the page already carries.
func TestShowZeroTransitionsNamesTheFrontmatterDoor(t *testing.T) {
	t.Parallel()
	body := terminalLessonPage(t, "archived")

	if !strings.Contains(body, wording.NoLegalTransitions.In(wording.ZhHant)) {
		t.Fatalf("archived lesson lost its empty-transitions notice; body = %q", body)
	}
	if !strings.Contains(body, "要恢復請直接編輯 frontmatter") {
		t.Errorf("the empty state does not say how recovery happens (a hand edit of the frontmatter)")
	}
	if !strings.Contains(body, "「在 Obsidian 開啟」就在標題下方") {
		t.Errorf("the empty state does not point at the editor door the page already carries")
	}
}
