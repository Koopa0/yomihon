package semantic

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"
)

func TestWaitForBuildRetry(t *testing.T) {
	tests := []struct {
		name string
		run  func(*testing.T)
	}{
		{
			name: "completion",
			run: func(t *testing.T) {
				t.Helper()
				const delay = 37 * time.Second
				started := time.Now()
				if err := waitForBuildRetry(t.Context(), delay); err != nil {
					t.Fatal(err)
				}
				if elapsed := time.Since(started); elapsed != delay {
					t.Fatalf("waitForBuildRetry() elapsed = %v, want %v", elapsed, delay)
				}
			},
		},
		{
			name: "cancellation",
			run: func(t *testing.T) {
				t.Helper()
				ctx, cancel := context.WithCancel(t.Context())
				result := make(chan error, 1)
				go func() {
					result <- waitForBuildRetry(ctx, time.Hour)
				}()
				synctest.Wait()
				cancel()
				if err := <-result; !errors.Is(err, context.Canceled) {
					t.Fatalf("waitForBuildRetry() error = %v, want context.Canceled", err)
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			synctest.Test(t, test.run)
		})
	}
}

func TestNewIndexerWithProviderDefaultsToBuildRetryWait(t *testing.T) {
	setup, _ := fixtureIndexerConfig(t, 1)
	indexer, err := newIndexerWithProvider(&setup, func() (*actionProvider, error) {
		return &actionProvider{
			embedChunk: func(context.Context, *CorpusChunk) (EmbeddingResult, error) {
				return EmbeddingResult{}, errors.New("unexpected provider send")
			},
			embedQuery: func(context.Context, string) (EmbeddingResult, error) {
				return EmbeddingResult{}, errors.New("unexpected provider send")
			},
		}, nil
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	synctest.Test(t, func(t *testing.T) {
		const delay = 23 * time.Second
		started := time.Now()
		if err := indexer.wait(t.Context(), delay); err != nil {
			t.Fatal(err)
		}
		if elapsed := time.Since(started); elapsed != delay {
			t.Fatalf("default build retry wait elapsed = %v, want %v", elapsed, delay)
		}
	})
}
