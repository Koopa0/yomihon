package note_test

import (
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/koopa0/yomihon/internal/mark"
)

// blockedDeskTransport makes an accidental use of the process transport fail
// before it can open a connection. The successful desk request must use the
// real test server's client instead.
type blockedDeskTransport struct {
	calls atomic.Int64
}

func (b *blockedDeskTransport) RoundTrip(*http.Request) (*http.Response, error) {
	b.calls.Add(1)
	return nil, errors.New("continuation desk used the default transport")
}

func TestContinuationDeskOwnsItsHTTPTransport(t *testing.T) {
	// Parallel tests start only after serial tests finish, so this temporary
	// process transport cannot interfere with their requests.
	original := http.DefaultTransport
	blocked := &blockedDeskTransport{}
	http.DefaultTransport = blocked
	t.Cleanup(func() { http.DefaultTransport = original })

	page := deskWithMark(t, aNote, &mark.Continuation{}, false)
	if !strings.HasPrefix(page, "<!doctype html>") {
		t.Errorf("the real desk response is not an HTML document: prefix %q", page[:min(160, len(page))])
	}
	if got := blocked.calls.Load(); got != 0 {
		t.Errorf("desk requests used the default transport %d times, want 0", got)
	}
}
