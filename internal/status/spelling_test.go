package status

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
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
