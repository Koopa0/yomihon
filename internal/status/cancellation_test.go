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

// The writer's lock exists to serialize the read-check-replace window of a
// flip. Reading a liveness flag is not that window, and neither is re-reading
// the whole contract and hashing it — work that runs on every page draw, which
// under the lock made every draw a queue a flip had to wait behind.
//
// The observation is taken with the capture parked: the lock has to be free
// while the contract source is being read, or the read is inside it.
func TestAuthorityLeavesTheLockFreeWhileItReadsTheContract(t *testing.T) {
	t.Parallel()

	_, writer := internalVault(t)
	reading := make(chan struct{})
	release := make(chan struct{})
	free := make(chan bool, 1)

	var capturing sync.WaitGroup
	capturing.Go(func() {
		authority := writer.authority(authorityHooks{beforeCapture: func() {
			close(reading)
			<-release
		}})
		if !authority.Governed() {
			t.Error("the parked capture returned an ungoverned authority")
		}
	})

	<-reading
	if writer.mu.TryLock() {
		writer.mu.Unlock()
		free <- true
	} else {
		free <- false
	}
	close(release)
	capturing.Wait()

	if !<-free {
		t.Error("Authority held the writer's lock across the contract read, so every page draw queued a flip behind a file read and a hash")
	}
}

// Reading the contract outside the lock means two page draws can read it at
// once, and one can read it while a flip installs or while the face is being
// closed. The liveness flag Close writes is the one piece of the writer's own
// state a reader touches, so the copy that carries it out of the lock is what
// keeps this quiet: taking the flag without the lock races Close, and the
// detector says so.
func TestAuthorityAnswersBesideAFlipAndACloseWithoutRacing(t *testing.T) {
	t.Parallel()

	root, writer := internalVault(t)
	path := filepath.Join(root, filepath.FromSlash(heldRel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir note parent: %v", err)
	}
	if err := os.WriteFile(path, []byte(internalLesson()), 0o600); err != nil {
		t.Fatalf("write %s: %v", heldRel, err)
	}

	var busy sync.WaitGroup
	for range 8 {
		busy.Go(func() {
			for range 50 {
				// A released face still answers governed; that is the point of
				// the flag. Either answer is fine here, an unsynchronized read
				// of it is not.
				if !writer.Authority().Governed() {
					t.Error("a concurrent Authority read an ungoverned vault")
					return
				}
			}
		})
	}
	busy.Go(func() {
		// Which of the flip and the close reaches the lock first is not this
		// test's business, so a flip that finds the face already gone is one
		// of the two right answers.
		if err := writer.Flip(t.Context(), heldRel, "draft", schema.SealStatus, internalLessonIdentity()); err != nil &&
			!errors.Is(err, ErrClosed) {
			t.Errorf("Flip() beside concurrent Authority reads = %v", err)
		}
	})
	busy.Go(func() {
		if err := writer.Close(); err != nil {
			t.Errorf("Close() beside concurrent Authority reads = %v", err)
		}
	})
	busy.Wait()
}
