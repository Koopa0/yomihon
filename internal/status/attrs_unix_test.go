//go:build linux || darwin

package status

import (
	"errors"
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
// and return nil, so a volume without xattrs never refuses a flip. EIO is
// the other side of that branch: swallowing every list error would pass the
// tolerated rows and fail this one.
func TestCopyXattrsIgnoresAForbiddenList(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name    string
		err     error
		wantNil bool
	}{
		{name: "EPERM", err: unix.EPERM, wantNil: true},
		{name: "ENOTSUP", err: unix.ENOTSUP, wantNil: true},
		{name: "EIO", err: unix.EIO, wantNil: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := copyXattrs(-1, -1, func(int) ([]string, error) {
				return nil, tt.err
			})
			if tt.wantNil {
				if err != nil {
					t.Fatalf("copyXattrs() with Flistxattr %v = %v, want nil", tt.err, err)
				}
				return
			}
			if !errors.Is(err, tt.err) {
				t.Fatalf("copyXattrs() with Flistxattr %v = %v, want that error", tt.err, err)
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

// TestFlipRefusesWhenListingXattrsFailsUnexpectedly is the other side of the
// list-stage lock: an unexpected errno must refuse the flip and leave the
// note untouched. Swallowing every list error would pass the EPERM row and
// fail this.
func TestFlipRefusesWhenListingXattrsFailsUnexpectedly(t *testing.T) {
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
			return nil, unix.EIO
		},
	})
	if !listed {
		t.Fatal("Flistxattr was not consulted")
	}
	if !errors.Is(err, unix.EIO) {
		t.Fatalf("Flip() when Flistxattr returns EIO = %v, want %v", err, unix.EIO)
	}
	got, readErr := os.ReadFile(path) // #nosec G304 -- path is a fixed name under t.TempDir
	if readErr != nil {
		t.Fatalf("read note: %v", readErr)
	}
	if string(got) != original {
		t.Errorf("note after refusal = %q, want untouched %q", got, original)
	}
	assertNoStatusTemps(t, filepath.Dir(path))
}
