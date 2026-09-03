package origin_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"syscall"
	"testing"

	"github.com/koopa0/yomihon/internal/origin"
)

// TestAReaderWhoLeftIsNotAFaultYomihonMade holds the log's loud level for the
// faults an operator can act on. Every other answer here is one of them, so
// the two that are not have to be told apart by what they are rather than by
// which handler happened to meet them.
func TestAReaderWhoLeftIsNotAFaultYomihonMade(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		cancelled bool
		err       error
		want      slog.Level
	}{
		{name: "the connection closed while the page was going out", cancelled: true, err: errors.New("write: broken pipe"), want: slog.LevelDebug},
		{name: "the write itself met a broken pipe", err: fmt.Errorf("write tcp: %w", syscall.EPIPE), want: slog.LevelDebug},
		{name: "the write itself met a reset connection", err: fmt.Errorf("write tcp: %w", syscall.ECONNRESET), want: slog.LevelDebug},
		{name: "the template could not render", err: errors.New("render: bad template"), want: slog.LevelError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/favicon.ico", http.NoBody)
			if tc.cancelled {
				ctx, cancel := context.WithCancel(r.Context())
				cancel()
				r = r.WithContext(ctx)
			}
			if got := origin.WriteFailureLevel(r, tc.err); got != tc.want {
				t.Errorf("WriteFailureLevel() = %v, want %v", got, tc.want)
			}
		})
	}
}
