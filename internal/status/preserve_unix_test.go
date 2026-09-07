//go:build linux || darwin

package status_test

import (
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/koopa0/yomihon/internal/schema"
)

const probeXattrName = "user.probeTag"
const probeXattrValue = "hello-tag"

// TestFlipPreservesExtendedAttributes is the lock that a tag (or any other
// extended attribute) on the note survives the install. writeTemp used to
// create a fresh inode carrying only the permission bits; the replacement
// must copy the source's attributes before it takes the note's name.
func TestFlipPreservesExtendedAttributes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writer := newWriter(t, root, loadContract(t))
	original := lessonContent("draft")
	writeNote(t, root, original)
	path := filepath.Join(root, filepath.FromSlash(testRel))
	if err := unix.Setxattr(path, probeXattrName, []byte(probeXattrValue), 0); err != nil {
		t.Fatalf("Setxattr(%q): %v", probeXattrName, err)
	}

	if err := writer.Flip(t.Context(), testRel, "draft", schema.SealStatus, diskIdentity(original)); err != nil {
		t.Fatalf("Flip() = %v, want nil", err)
	}

	got := make([]byte, len(probeXattrValue)+16)
	n, err := unix.Getxattr(path, probeXattrName, got)
	if err != nil {
		t.Fatalf("Getxattr(%q) after flip = %v, want the attribute to survive", probeXattrName, err)
	}
	if string(got[:n]) != probeXattrValue {
		t.Errorf("Getxattr(%q) = %q, want %q", probeXattrName, got[:n], probeXattrValue)
	}
	want := strings.Replace(original, "status: draft", "status: "+schema.SealStatus, 1)
	if read := readNote(t, root); read != want {
		t.Errorf("note after flip = %q, want %q", read, want)
	}
}
