package semantic

import (
	"io"
	"testing"
)

// closeOnCleanup makes resource teardown part of the test verdict. A failed
// SQLite or HTTP close can carry the durable write error that the test exists
// to detect, so test-wide ignored Close errors are not acceptable here.
func closeOnCleanup(tb testing.TB, closer io.Closer) {
	tb.Helper()
	tb.Cleanup(func() {
		closeNow(tb, closer)
	})
}

func closeNow(tb testing.TB, closer io.Closer) {
	tb.Helper()
	if err := closer.Close(); err != nil {
		tb.Errorf("close test resource: %v", err)
	}
}
