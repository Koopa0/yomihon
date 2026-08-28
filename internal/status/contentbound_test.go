package status_test

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/wording"
)

// changedContentSummary is the recovery page's statement for a flip refused
// because the note's bytes are no longer the ones the page rendered. It is
// asserted verbatim so the copy stays distinct from the stale-status page.
var changedContentSummary = wording.ContentMoved.In(wording.ZhHant)

// staleStatusSummary is the recovery page's statement for a submitted "from"
// that no longer matches the status on disk.
var staleStatusSummary = wording.PageStale.In(wording.ZhHant)

// renderedIdentity is the content identity the reading page embeds for a
// note, spelled out independently of the product: the SHA-256 of the note's
// bytes with the status value removed. Everything else on that line — the
// key, its separating space, a trailing comment, the newline — stays, because
// the write leaves all of it alone and a ruling is bound by it. Pinning the
// definition here keeps the test an oracle rather than an echo of the
// implementation.
func renderedIdentity(content, statusLine string) string {
	// The value is read off the line by hand: drop any trailing comment, then
	// the key and its separator, and what is left is the value's own text.
	value, _, _ := strings.Cut(statusLine, " #")
	value = strings.TrimSpace(strings.TrimPrefix(value, "status:"))
	if value == "" {
		panic("renderedIdentity: the status line carries no value")
	}
	withoutValue := strings.Replace(statusLine, value, "", 1)
	spliced := strings.Replace(content, statusLine, withoutValue, 1)
	if spliced == content {
		panic("renderedIdentity: the content does not carry that status line")
	}
	sum := sha256.Sum256([]byte(spliced))
	return hex.EncodeToString(sum[:])
}

// TestFlipRefusesRulingAgainstChangedContent replays the trial's failure
// whole: the page was rendered from one version of the note, another program
// edited the body — status line untouched, so the stale-status check cannot
// see it — and the reader pressed the transition afterwards. The POST carries
// the identity of the content the page rendered; the flip must refuse rather
// than install a ruling against bytes the reader never saw.
func TestFlipRefusesRulingAgainstChangedContent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writer := newWriter(t, root, loadContract(t))
	rendered := lessonContent("draft")
	writeNote(t, root, rendered)
	srv := newHandlerServer(t, writer)

	edited := strings.Replace(rendered, "body", "body rewritten after render", 1)
	if edited == rendered {
		t.Fatal("fixture edit did not change the body")
	}
	writeNote(t, root, edited)

	code, location, body := postStatus(t, srv, url.Values{
		"path":             {testRel},
		"from":             {"draft"},
		"to":               {schema.SealStatus},
		"content_identity": {renderedIdentity(rendered, "status: draft")},
	})
	if code != http.StatusConflict {
		t.Errorf("status = %d, want %d: the ruling was read against content no longer on disk", code, http.StatusConflict)
	}
	if location != "" {
		t.Errorf("Location = %q, want no redirect", location)
	}
	if !strings.Contains(body, changedContentSummary) {
		t.Errorf("body = %q, want the changed-content statement %q", body, changedContentSummary)
	}
	if strings.Contains(body, staleStatusSummary) {
		t.Errorf("body carries the stale-status copy; the two refusals must stay distinct")
	}
	if got := readNote(t, root); got != edited {
		t.Errorf("note after the refused POST = %q, want the externally edited bytes untouched", got)
	}
}

// TestFlipStaleStatusOutranksChangedContent pins the precedence between the
// two divergence refusals when an external edit moved both the status line
// and the body: the stale-status page answers, because the state having
// moved on is the fact that decides what the reader does next, and the
// content check cannot see a status move at all — its identity spans
// everything but that value.
func TestFlipStaleStatusOutranksChangedContent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writer := newWriter(t, root, loadContract(t))
	rendered := lessonContent("draft")
	writeNote(t, root, rendered)
	srv := newHandlerServer(t, writer)

	moved := strings.Replace(lessonContent("archived"), "body", "body rewritten too", 1)
	writeNote(t, root, moved)

	code, _, body := postStatus(t, srv, url.Values{
		"path":             {testRel},
		"from":             {"draft"},
		"to":               {schema.SealStatus},
		"content_identity": {renderedIdentity(rendered, "status: draft")},
	})
	if code != http.StatusConflict {
		t.Errorf("status = %d, want %d", code, http.StatusConflict)
	}
	if !strings.Contains(body, staleStatusSummary) {
		t.Errorf("body = %q, want the stale-status statement first when both axes diverged", body)
	}
	if strings.Contains(body, changedContentSummary) {
		t.Errorf("body carries the changed-content copy; the moved status is the deciding fact")
	}
}

// TestFlipKeepsTheRenderedIdentityValidForTheNextFlip locks the excision at
// the product seam: a successful flip rewrites the status line and nothing
// else, and the identity excludes exactly that line, so the identity a page
// rendered before the flip still names the bytes on disk after it. The next
// transition posted with that same identity — the reader pressing on from the
// page the redirect re-rendered, whose hidden field the rewrite left equal —
// must succeed, not be refused as a content change. The identity here is the
// product's own (formIdentity); the definitional oracle lives in the
// renderedIdentity tests above, while this lock is relational: whatever the
// page embeds, the flip must not invalidate it.
func TestFlipKeepsTheRenderedIdentityValidForTheNextFlip(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writer := newWriter(t, root, loadContract(t))
	rendered := lessonContent("draft")
	writeNote(t, root, rendered)
	srv := newHandlerServer(t, writer)

	identity := formIdentity(rendered)

	code, _, body := postStatus(t, srv, url.Values{
		"path":             {testRel},
		"from":             {"draft"},
		"to":               {schema.SealStatus},
		"content_identity": {identity},
	})
	if code != http.StatusSeeOther {
		t.Fatalf("first flip status = %d, want %d; body = %q", code, http.StatusSeeOther, body)
	}

	code, _, body = postStatus(t, srv, url.Values{
		"path":             {testRel},
		"from":             {schema.SealStatus},
		"to":               {"archived"},
		"content_identity": {identity},
	})
	if code != http.StatusSeeOther {
		t.Errorf("second flip with the pre-flip identity status = %d, want %d: the rewrite invalidated the identity the page rendered; body = %q", code, http.StatusSeeOther, body)
	}
	if got, want := readNote(t, root), lessonContent("archived"); got != want {
		t.Errorf("note after both flips = %q, want %q", got, want)
	}
}

// TestHandlerContentIdentityRequired locks the form contract: a caller that
// does not state which version of the note it read — the field absent, blank,
// or not a hex identity — is a malformed request, refused before any note is
// touched. Machine callers get no exemption; stating what was read is the
// point of the check.
func TestHandlerContentIdentityRequired(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writer := newWriter(t, root, loadContract(t))
	rendered := lessonContent("draft")
	writeNote(t, root, rendered)
	srv := newHandlerServer(t, writer)

	tests := []struct {
		name string
		form url.Values
	}{
		{"absent", url.Values{"path": {testRel}, "from": {"draft"}, "to": {schema.SealStatus}}},
		{"blank", url.Values{"path": {testRel}, "from": {"draft"}, "to": {schema.SealStatus}, "content_identity": {"  "}}},
		{"not hex", url.Values{"path": {testRel}, "from": {"draft"}, "to": {schema.SealStatus}, "content_identity": {strings.Repeat("zz", 32)}}},
		{"wrong length", url.Values{"path": {testRel}, "from": {"draft"}, "to": {schema.SealStatus}, "content_identity": {"abcdef"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			code, _, body := postStatus(t, srv, tt.form)
			if code != http.StatusUnprocessableEntity {
				t.Errorf("status = %d, want %d", code, http.StatusUnprocessableEntity)
			}
			if !strings.Contains(body, "content_identity 是必填欄位") {
				t.Errorf("body = %q, want the content_identity requirement stated", body)
			}
			if got := readNote(t, root); got != rendered {
				t.Errorf("note after the refused POST = %q, want untouched", got)
			}
		})
	}
}

// TestFlipStaleStatusKeepsItsOwnCopy holds the other half of the distinction:
// when the status line itself diverged, the refusal is the stale-status page,
// not the changed-content one — the reader's repair differs (the state moved
// on) and the copy has to say which happened.
func TestFlipStaleStatusKeepsItsOwnCopy(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writer := newWriter(t, root, loadContract(t))
	rendered := lessonContent("draft")
	writeNote(t, root, rendered)
	srv := newHandlerServer(t, writer)

	writeNote(t, root, lessonContent("archived"))

	code, _, body := postStatus(t, srv, url.Values{
		"path":             {testRel},
		"from":             {"draft"},
		"to":               {schema.SealStatus},
		"content_identity": {renderedIdentity(rendered, "status: draft")},
	})
	if code != http.StatusConflict {
		t.Errorf("status = %d, want %d", code, http.StatusConflict)
	}
	if !strings.Contains(body, staleStatusSummary) {
		t.Errorf("body = %q, want the stale-status statement %q", body, staleStatusSummary)
	}
	if strings.Contains(body, changedContentSummary) {
		t.Errorf("body carries the changed-content copy; the stale status is the fact to state")
	}
}

// commentedLesson is the fixture this file's last pair needs: a note whose
// author wrote a reason beside the status value. It is the ordinary shape of a
// note somebody is deciding about, and it is the shape the whole-line write and
// the whole-line identity both mishandled.
func commentedLesson(noteStatus, reason string) string {
	return strings.Replace(lessonContent(noteStatus), "status: "+noteStatus, "status: "+noteStatus+" # "+reason, 1)
}

// TestFlipKeepsTheReasonWrittenBesideTheStatus is the success control for the
// refusal below, and a lock in its own right. Without it the refusal proves
// nothing: an identity helper that disagreed with the product would refuse
// every flip, and both halves of the pair would pass while the write face was
// entirely broken.
func TestFlipKeepsTheReasonWrittenBesideTheStatus(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writer := newWriter(t, root, loadContract(t))
	rendered := commentedLesson("draft", "等原始資料")
	writeNote(t, root, rendered)
	srv := newHandlerServer(t, writer)

	code, location, body := postStatus(t, srv, url.Values{
		"path":             {testRel},
		"from":             {"draft"},
		"to":               {schema.SealStatus},
		"content_identity": {renderedIdentity(rendered, "status: draft # 等原始資料")},
	})
	if code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d: an unchanged note did not flip; body:\n%s", code, http.StatusSeeOther, body)
	}
	if location == "" {
		t.Error("a successful flip wrote no redirect")
	}
	want := commentedLesson(schema.SealStatus, "等原始資料")
	if got := readNote(t, root); got != want {
		t.Errorf("note after the flip =\n%q\nwant\n%q", got, want)
	}
}

// TestFlipRefusesWhenOnlyTheReasonBesideTheStatusChanged closes the lost
// update the whole-line identity allowed. The excised span was the whole
// status line, so a program that rewrote only the reason on it changed nothing
// the identity could see: a form rendered before that edit still validated,
// and installing it replaced the new reason with the old page's silence. The
// status value is the only thing a flip may move, so it is the only thing the
// identity may leave out.
func TestFlipRefusesWhenOnlyTheReasonBesideTheStatusChanged(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writer := newWriter(t, root, loadContract(t))
	rendered := commentedLesson("draft", "等原始資料")
	writeNote(t, root, rendered)
	srv := newHandlerServer(t, writer)

	edited := commentedLesson("draft", "來源已到，改等 Koopa 讀")
	if edited == rendered {
		t.Fatal("fixture edit did not change the reason")
	}
	writeNote(t, root, edited)

	code, _, body := postStatus(t, srv, url.Values{
		"path":             {testRel},
		"from":             {"draft"},
		"to":               {schema.SealStatus},
		"content_identity": {renderedIdentity(rendered, "status: draft # 等原始資料")},
	})
	if code != http.StatusConflict {
		t.Errorf("status = %d, want %d: a ruling installed over a reason the reader never saw", code, http.StatusConflict)
	}
	if !strings.Contains(body, changedContentSummary) {
		t.Errorf("body = %q, want the changed-content statement %q", body, changedContentSummary)
	}
	if got := readNote(t, root); got != edited {
		t.Errorf("note after the refused POST =\n%q\nwant the external edit untouched\n%q", got, edited)
	}
}
