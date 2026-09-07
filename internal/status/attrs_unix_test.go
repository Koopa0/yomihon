//go:build linux || darwin

package status

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/koopa0/yomihon/internal/schema"
)

func TestXattrIgnorableCoversListFailures(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "ENOTSUP", err: unix.ENOTSUP, want: true},
		{name: "EOPNOTSUPP", err: unix.EOPNOTSUPP, want: true},
		{name: "ENOSYS", err: unix.ENOSYS, want: true},
		{name: "EPERM", err: unix.EPERM, want: true},
		{name: "EACCES", err: unix.EACCES, want: true},
		{name: "EIO", err: unix.EIO, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := xattrIgnorable(tt.err); got != tt.want {
				t.Errorf("xattrIgnorable(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// TestCopyXattrsIgnoresAForbiddenList locks the list-stage call site: a
// filesystem that answers EPERM (or ENOTSUP) to Flistxattr must copy nothing
// and return nil, so a volume without xattrs never refuses a flip. Reverting
// that branch to xattrUnsupported fails this for EPERM.
func TestCopyXattrsIgnoresAForbiddenList(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		err  error
	}{
		{name: "EPERM", err: unix.EPERM},
		{name: "ENOTSUP", err: unix.ENOTSUP},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := copyXattrs(-1, -1, func(int) ([]string, error) {
				return nil, tt.err
			})
			if err != nil {
				t.Fatalf("copyXattrs() with Flistxattr %v = %v, want nil", tt.err, err)
			}
		})
	}
}

// TestFlipProceedsWhenListingXattrsIsForbidden is the same lock on the write
// face: a flip whose Flistxattr answers EPERM still rewrites the status line
// and leaves every other byte identical.
func TestFlipProceedsWhenListingXattrsIsForbidden(t *testing.T) {
	t.Parallel()
	root, writer := internalVault(t)
	const rel = "Writing/lessons/japanese/L05.md"
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	original := internalLesson()
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	listed := false
	err := writer.flip(t.Context(), rel, "draft", schema.SealStatus, internalLessonIdentity(), flipHooks{
		listXattrs: func(int) ([]string, error) {
			listed = true
			return nil, unix.EPERM
		},
	})
	if err != nil {
		t.Fatalf("Flip() when Flistxattr returns EPERM = %v, want nil", err)
	}
	if !listed {
		t.Fatal("Flistxattr was not consulted")
	}
	want := strings.Replace(original, "status: draft", "status: "+schema.SealStatus, 1)
	got, readErr := os.ReadFile(path) // #nosec G304 -- path is a fixed name under t.TempDir
	if readErr != nil {
		t.Fatalf("read note: %v", readErr)
	}
	if string(got) != want {
		t.Errorf("note after flip = %q, want %q", got, want)
	}
}
