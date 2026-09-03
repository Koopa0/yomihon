package note_test

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"syscall"
	"testing"

	"github.com/koopa0/yomihon/internal/note"
	"github.com/koopa0/yomihon/internal/schema"
)

// goneReader is a connection whose other end has already closed. Every write
// meets the operating system's answer for that, which is what a browser asking
// for an icon leaves behind when it sees a page arriving instead.
type goneReader struct{ header http.Header }

func (g *goneReader) Header() http.Header {
	if g.header == nil {
		g.header = make(http.Header)
	}
	return g.header
}

func (g *goneReader) Write([]byte) (int, error) { return 0, fmt.Errorf("write tcp: %w", syscall.EPIPE) }

func (g *goneReader) WriteHeader(int) {}

// TestAProbeThatLeavesIsNotLoggedAsAFault holds the log's loud level for the
// faults an operator can act on. A browser asks for an icon at a fixed address
// on nearly every fresh page, is handed a whole page, and drops the connection
// the moment it sees what it is — which made this the most frequent loud line
// in the log, on a site where the loud lines are the only instrument there is.
func TestAProbeThatLeavesIsNotLoggedAsAFault(t *testing.T) {
	t.Parallel()

	var written bytes.Buffer
	log := slog.New(slog.NewTextHandler(&written, &slog.HandlerOptions{Level: slog.LevelDebug}))
	store, source := newSnapshotStore(t, t.TempDir(), log, nil, schema.Ungoverned())
	writer := openStatusWriter(t, source, nil, schema.Ungoverned())
	mux := http.NewServeMux()
	note.New(&note.Sources{
		Source:         source,
		Status:         writer.Authority,
		Snapshot:       store.Current,
		ObservedStatus: writer.ObservedStatus,
		ConsumeReceipt: writer.ConsumeReceipt,
		Log:            log,
	}).Register(mux)

	mux.ServeHTTP(&goneReader{}, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/favicon.ico", http.NoBody))

	logged := written.String()
	if !strings.Contains(logged, "write not-found page") {
		t.Fatalf("the failed response was not reported at all; log = %q", logged)
	}
	if strings.Contains(logged, "level=ERROR") {
		t.Errorf("a reader who left was reported as a fault yomihon made; log = %q", logged)
	}
	if !strings.Contains(logged, "level=DEBUG") {
		t.Errorf("the failed response was not reported quietly; log = %q", logged)
	}
}
