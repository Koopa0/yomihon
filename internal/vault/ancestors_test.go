package vault

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

const rulesPath = "System/schemas/vault-schema.toml"

func touch(t *testing.T, dir, rel string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", full, err)
	}
}

func openBelow(t *testing.T, dir, rel string) *Reader {
	t.Helper()
	below := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(below, 0o750); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", below, err)
	}
	reader, err := Open(below)
	if err != nil {
		t.Fatalf("Open(%q) error = %v", below, err)
	}
	t.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Errorf("Close() error = %v", closeErr)
		}
	})
	return reader
}

func TestAncestorHoldsFindsWhatGovernsAFolderFromAbove(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		file string
		open string
		want bool
	}{
		{name: "the vault's own folder is the parent", file: "v/" + rulesPath, open: "v/Diary", want: true},
		{name: "two folders below the vault", file: "v/" + rulesPath, open: "v/Diary/2026", want: true},
		{name: "the same path at other letter cases", file: "v/system/Schemas/VAULT-SCHEMA.TOML", open: "v/Diary", want: true},
		{name: "a directory where the file would be", file: "v/" + rulesPath + "/inner.md", open: "v/Diary", want: true},
		{name: "a rules file beside the folder and not above it", file: "v/" + rulesPath, open: "w", want: false},
		{name: "an unrelated System folder above", file: "v/System/notes.md", open: "v/Diary", want: false},
		{name: "nothing above", file: "v/Notes/a.md", open: "v/Diary", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			touch(t, dir, tt.file)
			got, err := openBelow(t, dir, tt.open).AncestorHolds(rulesPath)
			if err != nil {
				t.Fatalf("AncestorHolds() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("AncestorHolds() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestAncestorHoldsFollowsASymlinkedSystemFolder(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	touch(t, dir, "elsewhere/schemas/vault-schema.toml")
	if err := os.MkdirAll(filepath.Join(dir, "v"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "elsewhere"), filepath.Join(dir, "v", "System")); err != nil {
		t.Skipf("symlinks are not available here: %v", err)
	}
	got, err := openBelow(t, dir, "v/Diary").AncestorHolds(rulesPath)
	if err != nil || !got {
		t.Errorf("AncestorHolds() = %t, %v, want true: a rules file reached through a symlink still governs", got, err)
	}
}

// A parent that can be walked through and not listed can still be asked for
// the one name the loader reads, and can never be answered "no" by guessing.
func TestAncestorHoldsAsksAnUnlistableParentForTheExactName(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("directory permissions do not hold here")
	}
	for _, held := range []bool{true, false} {
		dir := t.TempDir()
		if held {
			touch(t, dir, "v/"+rulesPath)
		}
		reader := openBelow(t, dir, "v/Diary")
		parent := filepath.Join(dir, "v")
		if err := os.Chmod(parent, 0o111); err != nil { // #nosec G302 -- a directory needs its execute bit; this is the test's own temp dir
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := os.Chmod(parent, 0o750); err != nil { // #nosec G302 -- a directory needs its execute bit; restoring it lets cleanup remove the temp dir
				t.Errorf("restore %q: %v", parent, err)
			}
		})
		got, err := reader.AncestorHolds(rulesPath)
		if err != nil || got != held {
			t.Errorf("held=%t: AncestorHolds() = %t, %v, want %t", held, got, err, held)
		}
	}
}
