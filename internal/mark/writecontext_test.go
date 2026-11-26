package mark_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/mark"
	"github.com/koopa0/yomihon/internal/mark/confdir"
	"github.com/koopa0/yomihon/internal/vault"
)

// A mark the server has received is kept though the page that sent it goes away
// while the change waits for the vault's lock, held here as another process
// would hold it: what the reader wrote is theirs, and a page left as its words
// were sent is not a reason to drop them.
func TestAMarkIsKeptThoughItsPageLeftWhileItWaitedForTheLock(t *testing.T) {
	t.Parallel()
	r := newMkRig(t)
	_, resolved, err := vault.ResolveRoot(r.root)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := confdir.VaultDir(confdir.At(r.conf), resolved)
	if err != nil {
		t.Fatal(err)
	}
	other, err := confdir.Acquire(t.Context(), dir, "machine-file.lock", 0)
	if err != nil {
		t.Fatal(err)
	}
	ctx, leave := context.WithCancel(t.Context())
	request := httptest.NewRequestWithContext(ctx, http.MethodPost, "/mark",
		strings.NewReader(`{"kind":"thought","path":"`+mkNote+`","anchor":"section-one","body":"sent as the page was left"}`))
	request.Header.Set("Content-Type", "application/json")
	answered := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		recorder := httptest.NewRecorder()
		r.mux.ServeHTTP(recorder, request)
		answered <- recorder
	}()
	time.AfterFunc(50*time.Millisecond, leave)
	time.AfterFunc(150*time.Millisecond, func() {
		if releaseErr := other.Release(); releaseErr != nil {
			t.Errorf("Release: %v", releaseErr)
		}
	})
	var response *httptest.ResponseRecorder
	select {
	case response = <-answered:
	case <-time.After(30 * time.Second):
		t.Fatal("the change never answered once the lock came free")
	}
	mkStatus(t, response, http.StatusCreated)
	view, err := r.kept.View()
	if err != nil {
		t.Fatal(err)
	}
	var words []string
	for _, e := range view.Entries(mark.AllMarks) {
		words = append(words, e.Mark().Body)
	}
	if len(words) != 1 || words[0] != "sent as the page was left" {
		t.Errorf("the vault keeps the marks %q, want the one the page sent as it was left", words)
	}
}
