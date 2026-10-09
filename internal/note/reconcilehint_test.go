package note_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/note"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/schema"
)

func TestFreshnessReconcileHintAnswers(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, answer string
		hints        int
	}{
		{"unchanged", "unchanged", 0}, {"stale", "stale", 0},
		{"content mismatch", "preparing", 1}, {"unpublished", "preparing", 0},
		{"gone", "gone", 0}, {"unreadable", "unreadable", 0},
		{"over-bound", "preparing", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			const body = "# Watched\n\nOriginal body.\n"
			if tc.name != "unpublished" {
				writeFreshNote(t, root, body)
			}
			log := slog.New(slog.DiscardHandler)
			store, source := newSnapshotStore(t, root, log, nil, schema.Ungoverned())
			writer := openStatusWriter(t, source, nil, schema.Ungoverned())
			calls := 0
			mux := http.NewServeMux()
			note.New(&note.Sources{
				Source: source, Snapshot: store.Current, Status: writer.Authority,
				ObservedStatus: writer.ObservedStatus, ConsumeReceipt: writer.ConsumeReceipt,
				Continuation: noMark, Log: log, RequestReconcile: func() { calls++ },
			}).Register(mux)
			identity := identityOf(body)
			ctx := t.Context()
			switch tc.name {
			case "stale":
				identity = strings.Repeat("0", 64)
			case "content mismatch":
				writeFreshNote(t, root, "# Watched\n\nChanged body.\n")
			case "unpublished":
				writeFreshNote(t, root, body)
			case "gone":
				if err := os.Remove(filepath.Join(root, filepath.FromSlash(freshRel))); err != nil {
					t.Fatal(err)
				}
			case "unreadable":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "over-bound":
				writeFreshNote(t, root, strings.Repeat("x", render.MaxSourceBytes+1))
			}
			for request := 1; request <= 2; request++ {
				w := httptest.NewRecorder()
				r := httptest.NewRequestWithContext(ctx, http.MethodGet, "/freshness/"+freshRel+"?identity="+identity, http.NoBody)
				mux.ServeHTTP(w, r)
				if w.Code != 200 || w.Body.String() != tc.answer {
					t.Fatalf("answer = (%d, %q), want (200, %q)", w.Code, w.Body.String(), tc.answer)
				}
				if calls != request*tc.hints {
					t.Errorf("hint calls after request %d = %d, want %d", request, calls, request*tc.hints)
				}
			}
		})
	}
}
