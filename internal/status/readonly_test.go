package status

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

func requirePermissionBits(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Windows refuses status writes")
	}
	probe := filepath.Join(t.TempDir(), "permission-probe")
	if err := os.WriteFile(probe, []byte("permission probe"), 0o600); err != nil {
		t.Fatalf("write permission probe: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(probe, 0o600); err != nil && !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("restore permission probe: %v", err)
		}
	})
	if err := os.Chmod(probe, 0); err != nil {
		t.Fatalf("chmod permission probe: %v", err)
	}
	info, err := os.Stat(probe)
	if err != nil || info.Mode().Perm() != 0 {
		t.Fatalf("permission probe stimulus: info=%v error=%v, want mode 000", info, err)
	}
	if _, err := os.ReadFile(probe); err == nil { // #nosec G304 -- fixed permission probe inside this test's TempDir
		t.Skip("privileged process or filesystem ignores permission bits")
	} else if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("permission probe read failed for another reason: %v", err)
	}
}

func setNoteMode(t *testing.T, path string, mode fs.FileMode) {
	t.Helper()
	t.Cleanup(func() {
		if err := os.Chmod(path, 0o600); err != nil && !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("restore note permissions: %v", err)
		}
	})
	if err := os.Chmod(path, mode); err != nil {
		t.Fatalf("chmod note: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat permission stimulus: %v", err)
	}
	if info.Mode().Perm() != mode {
		t.Fatalf("permission stimulus = %04o, want %04o", info.Mode().Perm(), mode)
	}
}

type readOnlyEntry struct {
	Bytes    string
	Mode     fs.FileMode
	Modified time.Time
	Info     fs.FileInfo
}

func readOnlyInventory(t *testing.T, dir string) map[string]readOnlyEntry {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read note siblings: %v", err)
	}
	out := make(map[string]readOnlyEntry, len(entries))
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatalf("stat sibling %q: %v", entry.Name(), err)
		}
		if !info.Mode().IsRegular() {
			t.Fatalf("sibling %q is not a regular fixture file", entry.Name())
		}
		data, err := os.ReadFile(path) // #nosec G304 -- inventory entries belong to this test's private note directory
		if err != nil {
			t.Fatalf("read sibling %q: %v", entry.Name(), err)
		}
		out[entry.Name()] = readOnlyEntry{Bytes: string(data), Mode: info.Mode(), Modified: info.ModTime(), Info: info}
	}
	return out
}

func assertReadOnlyInventory(t *testing.T, dir string, before map[string]readOnlyEntry) {
	t.Helper()
	after := readOnlyInventory(t, dir)
	if diff := cmp.Diff(before, after, cmpopts.IgnoreFields(readOnlyEntry{}, "Info")); diff != "" {
		t.Errorf("caught: read-only sibling bytes, modes or inventory changed (-want +got):\n%s", diff)
	}
	for name, original := range before {
		if current, ok := after[name]; ok && !os.SameFile(original.Info, current.Info) {
			t.Errorf("caught: read-only sibling identity changed: %q", name)
		}
	}
}

func assertNoReadOnlyReceipt(t *testing.T, writer *Writer) {
	t.Helper()
	if writer.ConsumeReceipt(installRel, "draft") {
		t.Error("caught: read-only refusal minted a success receipt")
	}
}

func TestFlipRefusesOwnerReadOnly(t *testing.T) {
	t.Parallel()
	requirePermissionBits(t)
	for _, mode := range []fs.FileMode{0o444, 0o466} {
		t.Run(fmt.Sprintf("mode-%04o", mode), func(t *testing.T) {
			t.Parallel()
			root, writer := internalVault(t)
			path := seedInstallNote(t, root)
			setNoteMode(t, path, mode)
			before := readOnlyInventory(t, filepath.Dir(path))
			err := writer.Flip(t.Context(), installRel, "draft", "ready", internalLessonIdentity())
			if !errors.Is(err, ErrReadOnly) {
				t.Errorf("caught: owner-read-only note was accepted: Flip(mode=%04o) = %v, want ErrReadOnly", mode, err)
			}
			assertReadOnlyInventory(t, filepath.Dir(path), before)
			assertNoReadOnlyReceipt(t, writer)
		})
	}
}

func TestReadOnlyNoteRemainsReadable(t *testing.T) {
	t.Parallel()
	requirePermissionBits(t)
	root, writer := internalVault(t)
	path := seedInstallNote(t, root)
	setNoteMode(t, path, 0o444)
	before := readOnlyInventory(t, filepath.Dir(path))
	got, err := writer.ObservedStatus(t.Context(), installRel)
	if err != nil || got != "draft" {
		t.Errorf("caught: read-only status became unreadable: ObservedStatus() = %q, %v, want draft, nil", got, err)
	}
	assertReadOnlyInventory(t, filepath.Dir(path), before)
}

func TestFlipPreservesWritableNotes(t *testing.T) {
	t.Parallel()
	requirePermissionBits(t)
	for _, mode := range []fs.FileMode{0o600, 0o644} {
		t.Run(fmt.Sprintf("mode-%04o", mode), func(t *testing.T) {
			t.Parallel()
			root, writer := internalVault(t)
			path := seedInstallNote(t, root)
			setNoteMode(t, path, mode)
			if err := writer.Flip(t.Context(), installRel, "draft", "ready", internalLessonIdentity()); err != nil {
				t.Fatalf("Flip(writable mode=%04o) = %v", mode, err)
			}
			got := readOnlyInventory(t, filepath.Dir(path))
			want := "---\ntitle: L05\ntype: lesson\ndomain: japanese\nstatus: ready\ncreated: 2026-06-01\nupdated: 2026-06-01\n---\n\nbody\n"
			if len(got) != 1 || got["L05.md"].Bytes != want || got["L05.md"].Mode.Perm() != mode {
				t.Errorf("writable replacement = %#v, want only surgical ready note with mode %04o", got, mode)
			}
			if !writer.ConsumeReceipt(installRel, "draft") || writer.ConsumeReceipt(installRel, "draft") {
				t.Error("writable replacement did not mint exactly one success receipt")
			}
		})
	}
}

func TestReadOnlyRefusalDoesNotStageOrMint(t *testing.T) {
	t.Parallel()
	requirePermissionBits(t)
	root, writer := internalVault(t)
	path := seedInstallNote(t, root)
	setNoteMode(t, path, 0o444)
	// A real stale install sibling makes even a sweep followed by cleanup
	// observable; an empty directory would miss that early effect.
	stale := filepath.Join(filepath.Dir(path), ".yomihon-status-ABCDEFGHIJKLMNOPQRSTUVWXYZ.tmp")
	if err := os.WriteFile(stale, []byte("an earlier writer's preserved bytes\n"), 0o600); err != nil {
		t.Fatalf("seed stale sibling: %v", err)
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatalf("age stale sibling: %v", err)
	}
	before := readOnlyInventory(t, filepath.Dir(path))
	reached := false
	err := writer.flip(t.Context(), installRel, "draft", "ready", internalLessonIdentity(), flipHooks{
		beforeAuthority: func() { reached = true },
	})
	if !errors.Is(err, ErrReadOnly) || reached {
		t.Errorf("caught: read-only refusal reached staging: error=%v beforeAuthority=%v", err, reached)
	}
	assertReadOnlyInventory(t, filepath.Dir(path), before)
	assertNoReadOnlyReceipt(t, writer)
}

func TestReadOnlyRefusalIsRepeatable(t *testing.T) {
	t.Parallel()
	requirePermissionBits(t)
	root, writer := internalVault(t)
	path := seedInstallNote(t, root)
	setNoteMode(t, path, 0o444)
	before := readOnlyInventory(t, filepath.Dir(path))
	for attempt := range 2 {
		if err := writer.Flip(t.Context(), installRel, "draft", "ready", internalLessonIdentity()); !errors.Is(err, ErrReadOnly) {
			t.Errorf("read-only attempt %d = %v, want ErrReadOnly", attempt, err)
		}
		assertReadOnlyInventory(t, filepath.Dir(path), before)
		assertNoReadOnlyReceipt(t, writer)
	}
}

func TestWritableNoteMadeReadOnlyDuringInstall(t *testing.T) {
	t.Parallel()
	requirePermissionBits(t)
	root, writer := internalVault(t)
	path := seedInstallNote(t, root)
	setNoteMode(t, path, 0o600)
	err := writer.flip(t.Context(), installRel, "draft", "ready", internalLessonIdentity(), flipHooks{
		beforeAuthority: func() { setNoteMode(t, path, 0o444) },
	})
	if !errors.Is(err, ErrConcurrentWrite) || errors.Is(err, ErrReadOnly) {
		t.Errorf("Flip(chmod during install) = %v, want ErrConcurrentWrite", err)
	}
	got := readOnlyInventory(t, filepath.Dir(path))
	if len(got) != 1 || got["L05.md"].Bytes != internalLesson() || got["L05.md"].Mode.Perm() != 0o444 {
		t.Errorf("chmod race left %#v, want original note with externally changed permissions", got)
	}
	assertNoReadOnlyReceipt(t, writer)
}

func TestReadOnlyRefusalPreservesEarlierErrors(t *testing.T) {
	t.Parallel()
	requirePermissionBits(t)
	tests := []struct {
		name   string
		from   string
		to     string
		body   string
		err    error
		linked bool
	}{
		{name: "stale", from: "ready", to: "archived", err: ErrStale},
		{name: "content", from: "draft", to: "ready", body: strings.Replace(internalLesson(), "body", "edited body", 1), err: ErrContentChanged},
		{name: "hard linked", from: "draft", to: "ready", err: ErrHardLinked, linked: true},
		{name: "unknown transition", from: "draft", to: "not-declared", err: schema.ErrUnknownStatus},
		{name: "illegal transition", from: "draft", to: "imported", err: schema.ErrIllegalTransition},
		{name: "unsupported key", from: "draft", to: "ready", body: strings.Replace(internalLesson(), "status: draft", "\"status\": draft", 1), err: ErrStatusSyntaxUnsupported},
		{name: "duplicate status", from: "draft", to: "ready", body: strings.Replace(internalLesson(), "status: draft", "status: draft\nstatus: draft", 1), err: ErrStale},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root, writer := internalVault(t)
			path := seedInstallNote(t, root)
			body := internalLesson()
			if tt.body != "" {
				body = tt.body
				if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
					t.Fatalf("write earlier-refusal fixture: %v", err)
				}
			}
			if tt.linked {
				if err := os.Link(path, filepath.Join(filepath.Dir(path), "alias.md")); err != nil {
					t.Fatalf("link earlier-refusal fixture: %v", err)
				}
			}
			setNoteMode(t, path, 0o444)
			before := readOnlyInventory(t, filepath.Dir(path))
			identity := vault.ContentIdentity([]byte(body))
			if tt.name == "content" {
				identity = internalLessonIdentity()
			}
			err := writer.Flip(t.Context(), installRel, tt.from, tt.to, identity)
			if !errors.Is(err, tt.err) || errors.Is(err, ErrReadOnly) {
				t.Errorf("Flip(%s) = %v, want earlier %v", tt.name, err, tt.err)
			}
			assertReadOnlyInventory(t, filepath.Dir(path), before)
			assertNoReadOnlyReceipt(t, writer)
		})
	}
	t.Run("cancelled before lock", func(t *testing.T) {
		t.Parallel()
		root, writer := internalVault(t)
		path := seedInstallNote(t, root)
		setNoteMode(t, path, 0o444)
		before := readOnlyInventory(t, filepath.Dir(path))
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if err := writer.Flip(ctx, installRel, "draft", "ready", internalLessonIdentity()); !errors.Is(err, context.Canceled) {
			t.Errorf("Flip(cancelled read-only) = %v, want context.Canceled", err)
		}
		assertReadOnlyInventory(t, filepath.Dir(path), before)
		assertNoReadOnlyReceipt(t, writer)
	})
}
