//go:build freebsd || netbsd || openbsd || windows

package semantic

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSemanticStoreEntryPointsFailBeforeCreatingFilesOnUnsupportedPlatform(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "absent")
	path := filepath.Join(parent, "generation.sqlite")
	openers := []struct {
		name string
		open func() error
	}{
		{name: "reader", open: func() error { _, err := openStore(t.Context(), path); return err }},
		{name: "writer", open: func() error { _, err := openWriter(t.Context(), path); return err }},
		{name: "rebuild writer", open: func() error { _, err := openRebuildWriter(t.Context(), path); return err }},
	}
	for _, opener := range openers {
		t.Run(opener.name, func(t *testing.T) {
			if err := opener.open(); !errors.Is(err, ErrStoreUnsupportedPlatform) {
				t.Fatalf("open error = %v, want ErrStoreUnsupportedPlatform", err)
			}
			if _, err := os.Stat(parent); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("semantic store entry point created %q: %v", parent, err)
			}
		})
	}
}
