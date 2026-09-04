//go:build unix

package vaultfs

import (
	"path/filepath"
	"syscall"
	"testing"
)

// TestScanRecordsANonRegularFileByItsOwnKind covers the member of the skip set
// no fixture can carry: a repository can hold a symbolic link, and cannot hold
// a named pipe. Without this the second kind would exist only as a branch
// nothing ever took, while the judging commands emit frozen bytes for it.
func TestScanRecordsANonRegularFileByItsOwnKind(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeReaderFixture(t, root, "Notes/real.md", "real")
	pipe := filepath.Join(root, "Notes", "pipe.md")
	if err := syscall.Mkfifo(pipe, 0o600); err != nil {
		t.Skipf("this filesystem will not hold a named pipe: %v", err)
	}

	scan, err := openTestReader(t, root).ScanAvailable(t.Context())
	if err != nil {
		t.Fatalf("scan() error = %v", err)
	}
	if files := scan.Files(); len(files) != 1 || files[0].Path() != "Notes/real.md" {
		t.Errorf("scan().Files() = %#v, want only the real file", files)
	}
	skipped := scan.Skipped()
	if len(skipped) != 1 || skipped[0].Path() != "Notes/pipe.md" {
		t.Fatalf("scan().Skipped() = %#v, want the pipe named", skipped)
	}
	if got := skipped[0].Kind(); got != SkipNotRegular {
		t.Errorf("scan().Skipped()[0].Kind() = %v, want %v", got, SkipNotRegular)
	}
}
