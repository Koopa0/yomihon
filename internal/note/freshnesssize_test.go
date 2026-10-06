package note_test

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/note"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/snapshot"
)

func TestFreshnessSourceSizeBoundary(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name           string
		size           int
		fileInfo       bool
		canceledBefore string
		canceledAfter  string
	}{
		{name: "at the limit", size: render.MaxSourceBytes, canceledBefore: "unreadable", canceledAfter: "unreadable"},
		{name: "above the limit", size: render.MaxSourceBytes + 1, fileInfo: true, canceledBefore: "preparing", canceledAfter: "stale"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			const original = "# Watched\n\nSmall rendered body.\n"
			writeFreshNote(t, root, original)
			log := slog.New(slog.DiscardHandler)
			governance := schema.Ungoverned()
			store, source := newSnapshotStore(t, root, log, nil, governance)
			published := store.Current()
			writer := openStatusWriter(t, source, nil, governance)
			mux := http.NewServeMux()
			note.New(&note.Sources{
				Source: source, Snapshot: func() *snapshot.Generation { return published },
				Status: writer.Authority, ObservedStatus: writer.ObservedStatus,
				ConsumeReceipt: writer.ConsumeReceipt, Continuation: noMark, Log: log,
			}).Register(mux)

			page := httptest.NewRecorder()
			mux.ServeHTTP(page, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/notes/"+freshRel, http.NoBody))
			identity := identityOf(original)
			if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), `data-freshness-identity="`+identity+`"`) {
				t.Fatal("the small note did not render with its freshness identity")
			}

			writeFreshNote(t, root, original+strings.Repeat("x", tc.size-len(original)))
			canceled, cancel := context.WithCancel(t.Context())
			cancel()
			entry, err := source.Lookup(freshRel)
			if err != nil {
				t.Fatalf("Lookup: %v", err)
			}
			if _, readErr := source.ReadFile(canceled, entry); !errors.Is(readErr, context.Canceled) {
				t.Fatalf("ReadFile(canceled) error = %v, want context.Canceled", readErr)
			}
			check := func(ctx context.Context, want string) {
				t.Helper()
				response := httptest.NewRecorder()
				request := httptest.NewRequestWithContext(ctx, http.MethodGet, "/freshness/"+freshRel+"?identity="+identity, http.NoBody)
				mux.ServeHTTP(response, request)
				if response.Code != http.StatusOK || response.Body.String() != want {
					t.Errorf("freshness(size=%d, canceled=%t) = (%d, %q), want (200, %q)", tc.size, ctx.Err() != nil, response.Code, response.Body.String(), want)
				}
			}
			check(t.Context(), "preparing")
			check(canceled, tc.canceledBefore)

			rebuilt, err := snapshot.New(t.Context(), source, log, nil, governance)
			if err != nil {
				t.Fatalf("rebuild generation: %v", err)
			}
			published = rebuilt.Current()
			check(t.Context(), "stale")
			check(canceled, tc.canceledAfter)

			reloaded := httptest.NewRecorder()
			mux.ServeHTTP(reloaded, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/notes/"+freshRel, http.NoBody))
			if reloaded.Code != http.StatusOK {
				t.Fatalf("reload status = %d, want 200", reloaded.Code)
			}
			if got := strings.Contains(reloaded.Body.String(), `class="y-fileinfo"`); got != tc.fileInfo {
				t.Errorf("reload file information page = %t, want %t", got, tc.fileInfo)
			}
		})
	}
}
