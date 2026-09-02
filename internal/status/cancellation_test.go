package status

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/schema"
)

const heldRel = "Writing/lessons/japanese/Held.md"

// TestTheTwoRequestEntryPointsAnswerAGoneReaderRatherThanQueue is an ordering
// test, and only an ordering test: a refusal that arrives after the lock has
// been waited for is no refusal at all, and a check written on the wrong side
// of that line would still return the right error.
//
// So one flip is parked inside the lock, and the two entry points a reading
// request reaches are then called with a context that is already cancelled.
// Each has to answer well inside a deadline the parked flip cannot meet. The
// install itself is deliberately not cancellable — past the lock the note is
// being replaced under its own name — which is exactly why the only place a
// cancellation can be honoured is before the queue.
//
// The cases run in sequence rather than in parallel: the lock is released when
// this function returns, and a case that outlived it would be waiting on
// nothing and would pass whatever the code did.
func TestTheTwoRequestEntryPointsAnswerAGoneReaderRatherThanQueue(t *testing.T) {
	t.Parallel()

	root, writer := internalVault(t)
	path := filepath.Join(root, filepath.FromSlash(heldRel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir note parent: %v", err)
	}
	if err := os.WriteFile(path, []byte(internalLesson()), 0o600); err != nil {
		t.Fatalf("write %s: %v", heldRel, err)
	}

	held := make(chan struct{})
	release := make(chan struct{})
	parkedErr := make(chan error, 1)
	var parked sync.WaitGroup
	parked.Go(func() {
		parkedErr <- writer.flip(t.Context(), heldRel, "draft", schema.SealStatus, internalLessonIdentity(),
			flipHooks{afterLock: func() {
				close(held)
				<-release
			}})
	})
	<-held
	defer func() {
		close(release)
		parked.Wait()
		if err := <-parkedErr; err != nil {
			t.Errorf("the flip parked inside the lock did not finish once released: %v", err)
		}
	}()

	gone, cancel := context.WithCancel(t.Context())
	cancel()

	for _, tt := range []struct {
		name string
		call func(ctx context.Context) error
	}{
		{
			name: "a flip",
			call: func(ctx context.Context) error {
				return writer.Flip(ctx, heldRel, "draft", schema.SealStatus, internalLessonIdentity())
			},
		},
		{
			name: "a read of the note's own status",
			call: func(ctx context.Context) error {
				_, err := writer.ObservedStatus(ctx, heldRel)
				return err
			},
		},
	} {
		answered := make(chan error, 1)
		go func() { answered <- tt.call(gone) }()
		select {
		case err := <-answered:
			if !errors.Is(err, context.Canceled) {
				t.Errorf("%s with a cancelled request: error = %v, want %v", tt.name, err, context.Canceled)
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("%s queued for a lock another flip was holding instead of answering the cancelled request", tt.name)
		}
	}
}
