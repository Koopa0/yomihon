package judge

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/vault"
)

// The pair every test below collides: one name as a Linux keyboard writes it,
// and the same name as a macOS-to-Linux copy or a git checkout leaves it.
const (
	collidingComposed   = "\u304c.md"
	collidingDecomposed = "\u304b\u3099.md"
)

// TestScanStoppedNamesBothCollidingFilesWhereTheContractAllowsIt is the
// refusal's whole decision. Before it, a folder holding two names with one
// canonical path stopped every command at the bare sentence "vault scan
// failed", which sends the person to look for a permission. The repair is to
// rename one of two files, so the refusal names both — unless the contract
// withholds either, in which case it says what kind of failure it was and
// nothing a withheld directory could be recognised by.
func TestScanStoppedNamesBothCollidingFilesWhereTheContractAllowsIt(t *testing.T) {
	t.Parallel()

	authority := testScanAuthority(t, "Diary")
	tests := []struct {
		name      string
		pair      [2]string
		withheld  bool
		neverSays []string
	}{
		{
			name: "two describable files",
			pair: [2]string{"Notes/" + collidingDecomposed, "Notes/" + collidingComposed},
		},
		{
			name:      "two files under a withheld directory",
			pair:      [2]string{"Diary/" + collidingDecomposed, "Diary/" + collidingComposed},
			withheld:  true,
			neverSays: []string{"Diary", collidingComposed, "u304c"},
		},
		{
			name:      "the withheld name sorting first",
			pair:      [2]string{"Diary/entry.md", "Notes/entry.md"},
			withheld:  true,
			neverSays: []string{"Diary", "Notes/entry.md"},
		},
		{
			name:      "the withheld name sorting second",
			pair:      [2]string{"Notes/entry.md", "Diary/entry.md"},
			withheld:  true,
			neverSays: []string{"Diary", "Notes/entry.md"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cause := wrapScanFailure(&vault.CollisionError{Paths: tt.pair})
			got := scanStopped(cause, authority)
			if tt.withheld {
				if !errors.Is(got, errWithheldCollision) {
					t.Fatalf("scanStopped(%q) = %v, want the withheld refusal", tt.pair, got)
				}
				for _, leaked := range tt.neverSays {
					if strings.Contains(got.Error(), leaked) {
						t.Errorf("scanStopped(%q) = %q, which describes withheld ground with %q", tt.pair, got, leaked)
					}
				}
				return
			}
			if errors.Is(got, errWithheldCollision) || errors.Is(got, errVaultScan) {
				t.Fatalf("scanStopped(%q) = %v, want both files named", tt.pair, got)
			}
			if !strings.HasPrefix(got.Error(), "vault scan failed: ") {
				t.Errorf("scanStopped(%q) = %q, want it to open the way every scan failure does", tt.pair, got)
			}
			for _, raw := range tt.pair {
				if !strings.Contains(got.Error(), vault.Spelled(raw)) {
					t.Errorf("scanStopped(%q) = %q, want it to name %q", tt.pair, got, raw)
				}
			}
			if !errors.Is(got, vault.ErrCanonicalCollision) {
				t.Errorf("scanStopped(%q) = %v, want it to stay a canonical collision", tt.pair, got)
			}
		})
	}
}

// wrapScanFailure is the cause as the reader hands it back: the collision
// under the sentence every scan failure carries.
func wrapScanFailure(collision *vault.CollisionError) error {
	return fmt.Errorf("list pinned vault: %w", collision)
}

// TestEveryCommandNamesTheFilesOfACollidingPair runs the three commands over a
// real folder holding the pair. They classify a folder they cannot judge alike,
// so one case is stated once and driven through each.
func TestEveryCommandNamesTheFilesOfACollidingPair(t *testing.T) {
	t.Parallel()

	for _, command := range []string{"check", "coverage", "exists"} {
		t.Run(command, func(t *testing.T) {
			t.Parallel()
			root := collidingFolder(t, "Notes")
			got := refuse(t.Context(), t, command, root).Error()
			for _, raw := range []string{"Notes/" + collidingDecomposed, "Notes/" + collidingComposed} {
				if !strings.Contains(got, vault.Spelled(raw)) {
					t.Errorf("%s error = %q, want it to name %q", command, got, raw)
				}
			}
		})
	}
}

// TestEveryCommandWithholdsACollidingPairUnderAPrivateDirectory is the other
// half: the same pair inside a directory the contract keeps out of
// agent-facing output is refused with the sentence for it, and the command
// does not echo a name that would let the caller recognise the directory.
func TestEveryCommandWithholdsACollidingPairUnderAPrivateDirectory(t *testing.T) {
	t.Parallel()

	for _, command := range []string{"check", "coverage", "exists"} {
		t.Run(command, func(t *testing.T) {
			t.Parallel()
			root := collidingFolder(t, "Diary")
			got := refuse(t.Context(), t, command, root).Error()
			if want := errWithheldCollision.Error(); got != want {
				t.Errorf("%s error = %q, want %q", command, got, want)
			}
			for _, leaked := range []string{"Diary", collidingComposed, "u304c", "u304b"} {
				if strings.Contains(got, leaked) {
					t.Errorf("%s error = %q, which describes withheld ground with %q", command, got, leaked)
				}
			}
		})
	}
}

// collidingFolder writes a vault whose contract withholds Diary and which holds
// the pair under dir, skipping the test where the filesystem folds the two
// spellings into one file and there is no pair.
func collidingFolder(t *testing.T, dir string) string {
	t.Helper()
	root := t.TempDir()
	writeTestContract(t, root, []string{"Diary"})
	write(t, root, "Notes/ok.md", "---\ntitle: Readable\n---\n")
	write(t, root, dir+"/"+collidingComposed, "---\ntitle: Composed\n---\n")
	write(t, root, dir+"/"+collidingDecomposed, "---\ntitle: Decomposed\n---\n")
	entries, err := os.ReadDir(filepath.Join(root, dir))
	if err != nil {
		t.Fatalf("ReadDir(%s) error = %v", dir, err)
	}
	var listed []string
	for _, entry := range entries {
		listed = append(listed, entry.Name())
	}
	if !slices.Contains(listed, path.Base(collidingComposed)) || !slices.Contains(listed, path.Base(collidingDecomposed)) {
		t.Skip("this filesystem folds the two spellings into one file, so there is no pair to refuse")
	}
	return root
}
