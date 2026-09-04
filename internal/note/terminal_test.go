package note_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
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
	code, body := get(t, srv.Client(), srv.URL+"/notes/Writing/lessons/japanese/L01.md")
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

// loadContractWithReturnPath is the testdata contract with one edge added:
// draft may also be entered from archived. Every status then has a walk back
// to draft, which is what separates the two conditions a confirm gate can
// encode — a target with no way onward at all, and a target with no way back
// to where the reader stands now.
func loadContractWithReturnPath(t *testing.T) *schema.Contract {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("read test contract: %v", err)
	}
	const draftFrom = `from = ["imported"]`
	modified := strings.Replace(string(data), draftFrom, `from = ["imported", "archived"]`, 1)
	if modified == string(data) {
		t.Fatal("return-path edge replacement did not apply")
	}
	path := filepath.Join(t.TempDir(), "vault-schema.toml")
	if writeErr := os.WriteFile(path, []byte(modified), 0o600); writeErr != nil { // #nosec G703 -- fixed basename under t.TempDir
		t.Fatalf("write modified contract: %v", writeErr)
	}
	contract, err := schema.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile(%q) = %v", path, err)
	}
	return contract
}

// lessonPage renders the reading page for one lesson fixture at the given
// status under the given contract and returns its body.
func lessonPage(t *testing.T, noteStatus string, contract *schema.Contract) string {
	t.Helper()
	root := t.TempDir()
	writeLesson(t, root, lessonWithStatus(noteStatus))
	srv := newServerWithContract(t, root, contract)
	code, body := get(t, srv.Client(), srv.URL+"/notes/Writing/lessons/japanese/L01.md")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	return body
}

// TestShowNoReturnTargetNeedsASecondInteraction locks what the two-step
// confirm is for: a press the reader cannot walk back from. The condition is
// derived from the contract's transitions, never configured — a target is
// gated when no chain of offered transitions leads from it back to the
// note's current status.
//
// Under the testdata contract nothing returns to draft, so from draft both
// offers are one-way doors: ready — the press most readers reach first — and
// archived alike get the closed native disclosure, and the note beside it
// says what is lost. The gate used to hang on "the destination offers
// nothing onward", which armed archived and left ready — just as
// irreversible — a quiet single press.
func TestShowNoReturnTargetNeedsASecondInteraction(t *testing.T) {
	t.Parallel()
	body := lessonPage(t, "draft", loadContract(t))

	for _, target := range []string{schema.SealStatus, "archived"} {
		form := transitionFormMarkup(t, body, target)
		if !strings.Contains(form, "<details") {
			t.Fatalf("no-return target %q renders without a disclosure; one press submits it:\n%s", target, form)
		}
		if strings.Contains(form, "<details open") {
			t.Errorf("the confirm for %q renders already open, so one interaction still reaches submit:\n%s", target, form)
		}
		summaryAt := strings.Index(form, "<summary")
		submitAt := strings.Index(form, `type="submit"`)
		if summaryAt < 0 || submitAt < 0 || submitAt < summaryAt {
			t.Errorf("the submit for %q is not behind the disclosure control (summary at %d, submit at %d):\n%s", target, summaryAt, submitAt, form)
		}
		for _, want := range []string{"回到目前狀態", "確認設為 " + target} {
			if !strings.Contains(form, want) {
				t.Errorf("the confirm for %q is missing %q:\n%s", target, want, form)
			}
		}
	}
}

// TestShowReturnableTargetKeepsTheQuietPress is the other half of the gate:
// give archived a way back to draft and every offer from draft becomes a
// walk the reader can reverse — ready through the two-step chain past
// archived — so no confirm belongs on any of them. A gate keyed on the
// destination's own dead-endedness cannot make this distinction; this
// fixture is what holds it to reachability.
func TestShowReturnableTargetKeepsTheQuietPress(t *testing.T) {
	t.Parallel()
	body := lessonPage(t, "draft", loadContractWithReturnPath(t))

	for _, target := range []string{schema.SealStatus, "archived"} {
		form := transitionFormMarkup(t, body, target)
		if strings.Contains(form, "<details") {
			t.Errorf("returnable target %q grew a confirm step:\n%s", target, form)
		}
		if !strings.Contains(form, `type="submit"`) {
			t.Errorf("returnable target %q lost its immediate submit:\n%s", target, form)
		}
	}
}

// TestFlipReceiptNamesTheRecoveryForAOneWayDoor holds the moment after the
// press the confirm warned about: the receipt that states the change also
// says how to get back, in the words the zero-transition state already uses —
// a hand edit of the frontmatter, through the editor link the page carries.
// A flip the reader can reverse through the offered controls keeps the plain
// receipt; naming the hand edit there would send them to an editor for a
// press the page in front of them can undo.
func TestFlipReceiptNamesTheRecoveryForAOneWayDoor(t *testing.T) {
	t.Parallel()
	const recoveryDoor = "要恢復請直接編輯 frontmatter"

	receiptParagraph := func(t *testing.T, landing string) string {
		t.Helper()
		start := strings.Index(landing, `class="y-flipreceipt"`)
		if start < 0 {
			t.Fatalf("the landing carries no receipt at all")
		}
		end := strings.Index(landing[start:], "</p>")
		if end < 0 {
			t.Fatalf("the receipt paragraph is unterminated")
		}
		return landing[start : start+end]
	}

	t.Run("a no-return flip's receipt carries the door", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeLesson(t, root, lessonWithStatus("draft"))
		srv := newServerWithContract(t, root, loadContract(t))

		_, page := get(t, srv.Client(), srv.URL+"/notes/Writing/lessons/japanese/L01.md")
		location := flipViaPage(t, srv, page, schema.SealStatus)
		_, landing := get(t, srv.Client(), srv.URL+location)
		receipt := receiptParagraph(t, landing)
		if !strings.Contains(receipt, recoveryDoor) {
			t.Errorf("the receipt for a flip nothing walks back from does not say how recovery happens:\n%s", receipt)
		}
		if !strings.Contains(receipt, "「在 Obsidian 開啟」") {
			t.Errorf("the receipt does not point at the editor door the page already carries:\n%s", receipt)
		}
	})

	t.Run("a reversible flip keeps the plain receipt", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeLesson(t, root, lessonWithStatus("draft"))
		srv := newServerWithContract(t, root, loadContractWithReturnPath(t))

		_, page := get(t, srv.Client(), srv.URL+"/notes/Writing/lessons/japanese/L01.md")
		location := flipViaPage(t, srv, page, schema.SealStatus)
		_, landing := get(t, srv.Client(), srv.URL+location)
		receipt := receiptParagraph(t, landing)
		if strings.Contains(receipt, recoveryDoor) {
			t.Errorf("the receipt for a reversible flip names the hand edit anyway:\n%s", receipt)
		}
	})
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
