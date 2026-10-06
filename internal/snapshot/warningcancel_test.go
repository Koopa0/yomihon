package snapshot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

type finalCancelSource struct {
	reader        *vault.Reader
	refuse        bool
	cancel        context.CancelFunc
	last          string
	prefix        bool
	hit           bool
	refusalHits   int
	recoveryHits  int
	lastSucceeded bool
	lastErr       error
	ctxErr        error
}

func (s *finalCancelSource) ObserveRoot() (vault.RootIdentity, error) {
	return s.reader.ObserveRoot()
}

func (s *finalCancelSource) ScanAvailable(ctx context.Context) (vault.Scan, error) {
	return s.reader.ScanAvailable(ctx)
}

func (s *finalCancelSource) ReadFile(ctx context.Context, entry vault.Entry) ([]byte, error) {
	if entry.Path() == "A-fault.txt" && s.refuse {
		s.refusalHits++
		return nil, errors.New("unchanged refusal")
	}
	data, err := s.reader.ReadFile(ctx, entry)
	if entry.Path() == "A-fault.txt" && !s.refuse && err == nil {
		s.recoveryHits++
	}
	if !s.prefix && s.cancel != nil && entry.Path() == s.last && err == nil {
		s.hit = true
		s.lastSucceeded = true
		s.cancel()
		s.ctxErr = ctx.Err()
	}
	return data, err
}

func (s *finalCancelSource) ReadPrefix(ctx context.Context, entry vault.Entry, limit int64) ([]byte, error) {
	if s.prefix && s.cancel != nil && entry.Path() == s.last {
		s.hit = true
		s.cancel()
	}
	data, err := s.reader.ReadPrefix(ctx, entry, limit)
	if s.prefix && s.cancel != nil && entry.Path() == s.last {
		s.lastErr, s.ctxErr = err, ctx.Err()
	}
	return data, err
}

func finalRefusalWarnings(t *testing.T, data []byte) int {
	t.Helper()
	count := 0
	for _, line := range bytes.Split(bytes.TrimSpace(data), []byte{'\n'}) {
		var record map[string]any
		if err := json.Unmarshal(line, &record); err != nil {
			t.Fatal(err)
		}
		if record["level"] == "WARN" && record["msg"] == "vault source unavailable in snapshot generation" && record["path"] == "A-fault.txt" {
			count++
		}
	}
	return count
}

func TestSnapshotWarningsFinalCancellation(t *testing.T) {
	for _, prefix := range []bool{false, true} {
		name, last, body := "successful final read", "zz.md", "last body\n"
		if prefix {
			name, last, body = "unread oversized briefing", "System/reports/daily-briefing/2026-10-06.html", strings.Repeat("x", render.MaxSourceBytes+1)
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			for path, data := range map[string]string{"A-fault.txt": "refused body\n", last: body} {
				full := filepath.Join(root, filepath.FromSlash(path))
				if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(full, []byte(data), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			reader, err := vault.Open(root)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := reader.Close(); err != nil {
					t.Error(err)
				}
			})
			source := &finalCancelSource{reader: reader, refuse: true, last: last, prefix: prefix}
			var logs bytes.Buffer
			store, err := New(t.Context(), source, slog.New(slog.NewJSONHandler(&logs, nil)), nil, schema.Ungoverned())
			if err != nil {
				t.Fatal(err)
			}
			initial := store.Current()
			if !initial.Freshness().Complete || source.refusalHits != 1 || finalRefusalWarnings(t, logs.Bytes()) != 1 {
				t.Fatal("not-applied: initial real source refusal did not emit exactly one WARN in a complete generation")
			}
			source.refusalHits = 0
			now := initial.Freshness().BuiltAt.Add(2 * time.Hour)
			store.now = func() time.Time { return now }
			source.refuse = false
			ctx, cancel := context.WithCancel(t.Context())
			t.Cleanup(cancel)
			source.cancel = cancel
			store.rescan(ctx)
			if !source.hit || ctx.Err() == nil {
				t.Fatal("not-applied: cancellation boundary was not reached")
			}
			if source.refusalHits != 0 || source.recoveryHits != 1 || !errors.Is(source.ctxErr, context.Canceled) {
				t.Fatal("not-applied: canceled attempted recovery did not reach successful fault-file read before final cancellation")
			}
			if prefix && !errors.Is(source.lastErr, context.Canceled) {
				t.Fatal("not-applied: actual ReadPrefix did not observe cancellation")
			}
			if !prefix && !source.lastSucceeded {
				t.Fatal("not-applied: final ReadFile did not succeed before cancellation")
			}
			t.Log("producer-hit: actual canceled final source boundary")
			publishedCanceled := store.Current() != initial
			if !publishedCanceled {
				t.Fatal("pre-existing late-cancel candidate publication changed")
			}
			source.cancel, source.refuse = nil, true
			now = now.Add(2 * time.Hour)
			store.rescan(t.Context())
			if store.Current() == initial || !store.Current().Freshness().Complete {
				t.Fatal("noncanceled source-refusal control did not publish")
			}
			if source.refusalHits != 1 {
				t.Fatal("not-applied: identical noncanceled refusal was not offered to the real logger boundary exactly once")
			}
			count := finalRefusalWarnings(t, logs.Bytes())
			t.Logf("traced pre-existing late-cancel publication: %t", publishedCanceled)
			if count != 1 {
				t.Fatalf("caught: canceled final observation pruned warning history: refusal warnings=%d, want 1", count)
			}
		})
	}
}
