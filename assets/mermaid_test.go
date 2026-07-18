package assets

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestMermaidFilesMatchManifest(t *testing.T) {
	t.Parallel()

	const root = "js/mermaid"
	manifest, err := Files.ReadFile(root + "/SHA256SUMS")
	if err != nil {
		t.Fatalf("read Mermaid manifest: %v", err)
	}

	var got strings.Builder
	err = fs.WalkDir(Files, root, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}

		relative := strings.TrimPrefix(name, root+"/")
		switch {
		case relative == "SHA256SUMS":
			return nil
		case relative == "LICENSE", strings.HasSuffix(relative, ".mjs"):
		default:
			return fmt.Errorf("unlocked Mermaid file %q", relative)
		}

		data, readErr := Files.ReadFile(name)
		if readErr != nil {
			return readErr
		}
		sum := sha256.Sum256(data)
		fmt.Fprintf(&got, "%x  %s\n", sum, relative)
		return nil
	})
	if err != nil {
		t.Fatalf("inventory Mermaid files: %v", err)
	}

	if diff := cmp.Diff(string(manifest), got.String()); diff != "" {
		t.Errorf("Mermaid SHA256SUMS mismatch (-want +got):\n%s", diff)
	}
}
