//go:build linux || darwin

package status

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
// filesystem that answers EPERM (or ENOTSUP) to Flistxattr must yield nothing
// to copy and return nil, so a volume without xattrs never refuses a flip. The
// same reader answers the comparison before the install, so such a volume
// reads an empty set both times and cannot refuse on a difference. EIO is
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
			_, err := readXattrs(-1, func(int) ([]string, error) {
				return nil, tt.err
			})
			if tt.wantNil {
				if err != nil {
					t.Fatalf("readXattrs() with Flistxattr %v = %v, want nil", tt.err, err)
				}
				return
			}
			if !errors.Is(err, tt.err) {
				t.Fatalf("readXattrs() with Flistxattr %v = %v, want that error", tt.err, err)
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

// TestFlipRefusesWhenAttributesMoveWhileReplacing is the lock that the
// replacement never reinstates an attribute value another program has already
// moved on. The copy captures the source's attributes while the replacement
// is prepared; only the source's bytes, mode, mtime and identity were checked
// again before the install, and an attribute-only update moves none of those.
// The row that moves the tag after the copy refuses; the rows that move it
// earlier or not at all install what the source held when the copy ran.
func TestFlipRefusesWhenAttributesMoveWhileReplacing(t *testing.T) {
	t.Parallel()
	const (
		attrName = "user.auditTag"
		oldTag   = "old-tag"
		newTag   = "new-tag"
	)
	for _, tt := range []struct {
		name    string
		hooks   func(moveTag func()) flipHooks
		wantErr error
		wantTag string
	}{
		{
			name:    "no other writer",
			hooks:   func(func()) flipHooks { return flipHooks{} },
			wantTag: oldTag,
		},
		{
			name:    "tag moves before the copy",
			hooks:   func(moveTag func()) flipHooks { return flipHooks{beforeLock: moveTag} },
			wantTag: newTag,
		},
		{
			name:    "tag moves after the copy",
			hooks:   func(moveTag func()) flipHooks { return flipHooks{beforeAuthority: moveTag} },
			wantErr: ErrConcurrentWrite,
			wantTag: newTag,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
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
			setXattr(t, path, attrName, oldTag)
			before := noteModTime(t, path)

			moved := false
			err := writer.flip(t.Context(), rel, "draft", schema.SealStatus, internalLessonIdentity(), tt.hooks(func() {
				moved = true
				setXattr(t, path, attrName, newTag)
				// A tag move that also moved the mtime would be refused by the
				// existing content check, and this test would prove nothing.
				if after := noteModTime(t, path); !after.Equal(before) {
					t.Errorf("mtime after moving %q = %v, want the unchanged %v", attrName, after, before)
				}
			}))

			if moved != (tt.wantTag == newTag) {
				t.Fatalf("the other writer ran = %v, want %v", moved, tt.wantTag == newTag)
			}
			if got := xattrValue(t, path, attrName); got != tt.wantTag {
				t.Errorf("%s after flip = %q, want %q", attrName, got, tt.wantTag)
			}
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("flip() with the tag moved after the copy = %v, want %v", err, tt.wantErr)
				}
			} else if err != nil {
				t.Fatalf("flip() = %v, want nil", err)
			}
			want := original
			if tt.wantErr == nil {
				want = strings.Replace(original, "status: draft", "status: "+schema.SealStatus, 1)
			}
			got, readErr := os.ReadFile(path) // #nosec G304 -- path is a fixed name under t.TempDir
			if readErr != nil {
				t.Fatalf("read note: %v", readErr)
			}
			if string(got) != want {
				t.Errorf("note after flip = %q, want %q", got, want)
			}
			assertNoStatusTemps(t, filepath.Dir(path))
		})
	}
}

func setXattr(t *testing.T, path, name, value string) {
	t.Helper()
	if err := unix.Setxattr(path, name, []byte(value), 0); err != nil {
		t.Fatalf("Setxattr(%q, %q) = %v", path, name, err)
	}
}

func xattrValue(t *testing.T, path, name string) string {
	t.Helper()
	value := make([]byte, 64)
	n, err := unix.Getxattr(path, name, value)
	if err != nil {
		t.Fatalf("Getxattr(%q, %q) = %v", path, name, err)
	}
	return string(value[:n])
}

func noteModTime(t *testing.T, path string) time.Time {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %q = %v", path, err)
	}
	return info.ModTime()
}
