package render

import (
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v3"
)

// TestReshapeHighlighterFailureDropsTheInput holds the reason that reaches a
// page or a log: chroma's timeout text names the code that timed out, and that
// must not travel with the diagnostic.
func TestReshapeHighlighterFailureDropsTheInput(t *testing.T) {
	t.Parallel()
	const secret = "secret-fence-input-must-not-leak"
	err := fmt.Errorf("YomihonTimeoutLock: state root: matching %q: match timeout after 250ms on input `%s`", "(a+)+b", secret)
	got := reshapeHighlighterFailure(err)
	if got != highlightTimeoutReason {
		t.Errorf("reshapeHighlighterFailure() = %q, want %q", got, highlightTimeoutReason)
	}
	if strings.Contains(got, secret) || strings.Contains(got, "on input") {
		t.Errorf("reshaped reason leaked highlighter input: %q", got)
	}
}

// TestRecoveredHighlighterFailureLetsRuntimePanicsThrough is the other half of
// the recover: a timeout is decoration, a nil pointer is not.
func TestRecoveredHighlighterFailureLetsRuntimePanicsThrough(t *testing.T) {
	t.Parallel()
	if got := recoveredHighlighterFailure("unknown state root"); got != "" {
		t.Errorf("string panic was treated as a highlighter failure: %q", got)
	}
	rec := recoverRuntimeError(t)
	if got := recoveredHighlighterFailure(rec); got != "" {
		t.Errorf("runtime error was treated as a highlighter failure: %q", got)
	}
}

func recoverRuntimeError(t *testing.T) (rec any) {
	t.Helper()
	defer func() { rec = recover() }()
	var p *int
	_ = *p
	t.Fatal("nil pointer did not panic")
	return nil
}

// TestHighlightCodeReraisesRuntimePanics watches the recover around Format
// re-raise a runtime panic, so a bug in highlighting still fails the request
// rather than being dressed as a plain code block.
func TestHighlightCodeReraisesRuntimePanics(t *testing.T) {
	t.Parallel()
	defer func() {
		rec := recover()
		if rec == nil {
			t.Fatal("runtime panic was swallowed")
		}
		if _, ok := rec.(runtime.Error); !ok {
			t.Fatalf("re-raised %T %v, want runtime.Error", rec, rec)
		}
	}()
	_, _ = highlightCode(func(func(chroma.Token) bool) {
		var p *int
		_ = *p
	})
}
