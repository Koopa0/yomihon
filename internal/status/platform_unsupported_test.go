//go:build windows || yomihon_nodurable

package status_test

import (
	"crypto/sha256"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/status"
)

func TestUnsupportedPlatformClosesWriteFaceBeforeFilesystem(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writer := newWriter(t, root, loadContract(t))

	view := writer.View()
	if view.Closed() {
		t.Fatalf("View().Closed() = true, want read authority preserved: %s", view.Diagnostic())
	}
	if got := view.Diagnostic(); got != "" {
		t.Errorf("View().Diagnostic() = %q, want empty read-authority diagnostic", got)
	}
	if got := view.WriteDiagnostic(); got != status.DurableInstallUnavailableDiagnostic {
		t.Errorf("View().WriteDiagnostic() = %q, want %q", got, status.DurableInstallUnavailableDiagnostic)
	}
	if got := view.Order(); len(got) == 0 {
		t.Error("View().Order() is empty, want read-only lifecycle projection preserved")
	}
	if got := view.Transitions(testRel, "lesson", "draft"); got != nil {
		t.Errorf("View().Transitions() = %v, want none on a platform without a durable install", got)
	}

	err := writer.Flip(t.Context(), testRel, "draft", schema.SealStatus, [sha256.Size]byte{})
	if !errors.Is(err, status.ErrDurabilityUnsupported) {
		t.Fatalf("Flip() = %v, want %v before opening the target", err, status.ErrDurabilityUnsupported)
	}
	entries, readErr := os.ReadDir(root)
	if readErr != nil {
		t.Fatalf("ReadDir(%q) = %v", root, readErr)
	}
	if len(entries) != 0 {
		t.Errorf("Flip() created %d filesystem entries, want none", len(entries))
	}
}

func TestUnsupportedPlatformPOSTIsAnUnchangedServiceRefusal(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	srv := newHandlerServer(t, newWriter(t, root, loadContract(t)))

	code, location, body := postStatus(t, srv, url.Values{
		"path":             {testRel},
		"from":             {"draft"},
		"to":               {schema.SealStatus},
		"content_identity": {wellFormedIdentity},
	})
	if code != http.StatusServiceUnavailable {
		t.Errorf("POST /status status = %d, want %d", code, http.StatusServiceUnavailable)
	}
	if location != "" {
		t.Errorf("POST /status Location = %q, want empty", location)
	}
	if !containsAll(body, "此平台", "狀態尚未變更") {
		t.Errorf("POST /status body = %q, want platform refusal and unchanged guarantee", body)
	}
}

func containsAll(s string, substrings ...string) bool {
	for _, substring := range substrings {
		if !strings.Contains(s, substring) {
			return false
		}
	}
	return true
}
