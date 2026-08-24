package status_test

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

// changedContentSummary is the recovery page's statement for a flip refused
// because the note's bytes are no longer the ones the page rendered. It is
// asserted verbatim so the copy stays distinct from the stale-status page.
const changedContentSummary = "筆記內容在這個頁面載入之後被改過；這次操作綁定的是當時讀到的版本。"

// staleStatusSummary is the recovery page's statement for a submitted "from"
// that no longer matches the status on disk.
const staleStatusSummary = "這個頁面已過期；磁碟上的狀態已經不同。"

// renderedIdentity is the content identity the reading page embeds for a
// note, spelled out independently of the product: the SHA-256 of the note's
// bytes with the status line's text removed (its newline stays). Pinning the
// definition here keeps the test an oracle rather than an echo of the
// implementation.
func renderedIdentity(content, statusLine string) string {
	spliced := strings.Replace(content, statusLine, "", 1)
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
	lifecycle := newLifecycle(t, root, loadContract(t))
	rendered := lessonContent("draft")
	writeNote(t, root, rendered)
	srv := newHandlerServer(t, lifecycle)

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
// everything but that line.
func TestFlipStaleStatusOutranksChangedContent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	lifecycle := newLifecycle(t, root, loadContract(t))
	rendered := lessonContent("draft")
	writeNote(t, root, rendered)
	srv := newHandlerServer(t, lifecycle)

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
	lifecycle := newLifecycle(t, root, loadContract(t))
	rendered := lessonContent("draft")
	writeNote(t, root, rendered)
	srv := newHandlerServer(t, lifecycle)

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
	lifecycle := newLifecycle(t, root, loadContract(t))
	rendered := lessonContent("draft")
	writeNote(t, root, rendered)
	srv := newHandlerServer(t, lifecycle)

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
	lifecycle := newLifecycle(t, root, loadContract(t))
	rendered := lessonContent("draft")
	writeNote(t, root, rendered)
	srv := newHandlerServer(t, lifecycle)

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
