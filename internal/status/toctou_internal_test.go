package status

// This file is a deliberate, narrow exception to the repo's usual black-box
// (package status_test) testing convention: checkUnmodifiedSince guards a
// TOCTOU window inside a single synchronous Flip call (between its initial
// os.Stat and its pre-write recheck) that no exported API can trigger from
// outside the package without a real, inherently flaky filesystem race.
// Extracting it and testing it directly here is the only way to exercise
// docs/spec.md §4's "mtime changed" row deterministically.

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCheckUnmodifiedSinceDetectsConcurrentWrite(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "note.md")
	if err := os.WriteFile(path, []byte("v1"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	// Simulate an external process (Obsidian, another cron script) touching
	// the file in the narrow window between Flip's initial read and its
	// pre-write recheck. os.Chtimes with a deliberately displaced mtime
	// avoids depending on filesystem mtime granularity or wall-clock timing.
	touched := before.ModTime().Add(time.Hour)
	if err = os.Chtimes(path, touched, touched); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	err = checkUnmodifiedSince(path, "note.md", before)
	if !errors.Is(err, ErrConcurrentWrite) {
		t.Fatalf("checkUnmodifiedSince() = %v, want an error wrapping %v", err, ErrConcurrentWrite)
	}
	// The two error-table rows (docs/spec.md §4) are spec-distinguished:
	// this must NOT also satisfy the unrelated "stale form" sentinel, or
	// handler.go's switch collapses back to one indistinguishable message.
	if errors.Is(err, ErrStale) {
		t.Errorf("checkUnmodifiedSince() = %v, must not also satisfy ErrStale", err)
	}
}

func TestCheckUnmodifiedSinceNoChange(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "note.md")
	if err := os.WriteFile(path, []byte("v1"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	if err := checkUnmodifiedSince(path, "note.md", before); err != nil {
		t.Errorf("checkUnmodifiedSince() = %v, want nil when nothing touched the file", err)
	}
}
