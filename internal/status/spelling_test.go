package status

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

// TestFlipRefusesWhenStoredDirOpenFails locks the spelling walk's open
// failure as a refusal. Swallowing that error used to hand Flip the raw
// requested spelling, which a case-insensitive volume then wrote.
func TestFlipRefusesWhenStoredDirOpenFails(t *testing.T) {
	t.Parallel()
	root, writer := internalVault(t)
	path := seedInstallNote(t, root)
	original, err := os.ReadFile(path) // #nosec G304 -- seedInstallNote's path under this test's TempDir
	if err != nil {
		t.Fatalf("read seed: %v", err)
	}

	err = writer.flip(t.Context(), installRel, "draft", schema.SealStatus, internalLessonIdentity(), flipHooks{
		descend: func(*os.Root, string, string) (*os.Root, error) {
			return nil, fmt.Errorf("%w: %s", errPathNotRegular, installRel)
		},
	})
	if !errors.Is(err, errPathNotRegular) {
		t.Fatalf("Flip with forced descend failure = %v, want %v", err, errPathNotRegular)
	}
	got, readErr := os.ReadFile(path) // #nosec G304 -- seedInstallNote's path under this test's TempDir
	if readErr != nil {
		t.Fatalf("read note: %v", readErr)
	}
	if diff := cmp.Diff(string(original), string(got)); diff != "" {
		t.Errorf("note rewritten after a stored-dir open failure (-want +got):\n%s", diff)
	}
}

// TestUniqueNFCNameRefusesADifferentlyCasedNFCSpelling asks the walk itself
// — one directory, one stored name, the NFC request the reading page types —
// for a note whose on-disk spelling differs only in case. The folder holds
// nothing else, so deleting the case-sensitive NFC comparison cannot hide
// behind a uniqueness refusal: the listed name becomes the match and the
// answer is a status. The case-sensitive volume's answer is missing.
func TestUniqueNFCNameRefusesADifferentlyCasedNFCSpelling(t *testing.T) {
	t.Parallel()

	const nfcLeaf = "käln.md"
	const casedLeaf = "Käln.md"
	if vault.NormalizeNFC(casedLeaf) == vault.NormalizeNFC(nfcLeaf) {
		t.Fatal("NFC folded the two cases together; this lock would not bind")
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, casedLeaf), []byte("draft"), 0o600); err != nil {
		t.Fatalf("write %s: %v", casedLeaf, err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatalf("OpenRoot: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Errorf("Root.Close() = %v", closeErr)
		}
	})

	got, err := uniqueNFCName(root, nfcLeaf, nfcLeaf)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("uniqueNFCName(%q) against on-disk %q = (%q, %v), want %v", nfcLeaf, casedLeaf, got, err, fs.ErrNotExist)
	}
}
