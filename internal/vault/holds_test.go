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

// A parent that can be walked through and not listed cannot be shown to hold no
// rules file at any spelling, so it is refused and never answered "no", whether
// or not the exact name happens to be there.
func TestAncestorHoldsRefusesAParentItCannotList(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("directory permissions do not hold here")
	}
	for _, exact := range []bool{true, false} {
		dir := t.TempDir()
		if exact {
			touch(t, dir, "v/"+rulesPath)
		}
		reader := openBelow(t, dir, "v/Diary")
		restoreAfter(t, filepath.Join(dir, "v"), 0o111)
		if got, err := reader.AncestorHolds(rulesPath); err == nil {
			t.Errorf("exact spelling present=%t: AncestorHolds() = %t, nil, want an error: an unlistable parent is not a \"no\"", exact, got)
		}
	}
}

// restoreAfter sets a directory's mode for the test and puts back one that
// lets the temp dir be removed.
func restoreAfter(t *testing.T, dir string, mode os.FileMode) {
	t.Helper()
	if err := os.Chmod(dir, mode); err != nil { // #nosec G302 -- the test's own temp dir
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(dir, 0o750); err != nil { // #nosec G302 -- restoring the execute bit lets cleanup remove the temp dir
			t.Errorf("restore %q: %v", dir, err)
		}
	})
}

func openVault(t *testing.T, dir string) *Reader {
	t.Helper()
	reader, err := Open(dir)
	if err != nil {
		t.Fatalf("Open(%q) error = %v", dir, err)
	}
	t.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Errorf("Close() error = %v", closeErr)
		}
	})
	return reader
}

func symlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(link), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks are not available here: %v", err)
	}
}

// What a scan leaves out is exactly where a rules file hides, so the look below
// a folder reads each folder's own listing by name.
func TestHoldsBelowFindsARulesFileTheScanDoesNotList(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		// files are made in the vault; shared are made beside it, where a rules
		// file somebody shared lives; links point a name in the vault at one of
		// those; ignore is the vault's .gitignore.
		files, shared []string
		links         [][2]string
		ignore        string
		want          bool
	}{
		{name: "a rules file in a nested vault", files: []string{"v/" + rulesPath}, want: true},
		{name: "at other letter cases", files: []string{"v/system/Schemas/VAULT-SCHEMA.TOML"}, want: true},
		{name: "the rules file is a symlink", shared: []string{"vault-schema.toml"}, links: [][2]string{{"vault-schema.toml", "v/" + rulesPath}}, want: true},
		{name: "the schemas folder is a symlink", shared: []string{"vault-schema.toml"}, links: [][2]string{{"", "v/System/schemas"}}, want: true},
		{name: "the System folder is a symlink", shared: []string{"schemas/vault-schema.toml"}, links: [][2]string{{"", "v/System"}}, want: true},
		{name: "an ignore rule hides the schemas folder from the scan", files: []string{"v/" + rulesPath, "v/Diary/a.md"}, ignore: "schemas/\n", want: true},
		{name: "a folder deep in the tree", files: []string{"a/b/c/" + rulesPath}, want: true},
		{name: "no rules file anywhere", files: []string{"v/Notes/a.md"}, want: false},
		{name: "an unrelated System folder", files: []string{"v/System/notes.md"}, want: false},
		{name: "a dot-directory is not entered, so it is not looked in", files: []string{".hidden/" + rulesPath}, want: false},
		{name: "an ignored directory is not entered, so it is not looked in", files: []string{"Skipped/" + rulesPath}, ignore: "Skipped/\n", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir, shared := t.TempDir(), t.TempDir()
			for _, rel := range tt.files {
				touch(t, dir, rel)
			}
			for _, rel := range tt.shared {
				touch(t, shared, rel)
			}
			for _, link := range tt.links {
				symlink(t, filepath.Join(shared, filepath.FromSlash(link[0])), filepath.Join(dir, filepath.FromSlash(link[1])))
			}
			if tt.ignore != "" {
				if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(tt.ignore), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			got, err := openVault(t, dir).HoldsBelow(t.Context(), rulesPath)
			if err != nil {
				t.Fatalf("HoldsBelow() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("HoldsBelow() = %t, want %t", got, tt.want)
			}
		})
	}
}

// A folder the walk enters and cannot list could hold a rules file, so it is
// refused rather than counted as holding none, and the error names no path.
func TestHoldsBelowRefusesAFolderItCannotList(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("directory permissions do not hold here")
	}
	for _, unlistable := range []string{"v/System", "v/Diary", "v"} {
		t.Run(unlistable, func(t *testing.T) {
			dir := t.TempDir()
			touch(t, dir, "v/Notes/a.md")
			touch(t, dir, "v/System/notes.md")
			touch(t, dir, "v/Diary/b.md")
			reader := openVault(t, dir)
			restoreAfter(t, filepath.Join(dir, filepath.FromSlash(unlistable)), 0o000)
			got, err := reader.HoldsBelow(t.Context(), rulesPath)
			if err == nil {
				t.Fatalf("HoldsBelow() = %t, nil, want an error: a folder that cannot be listed is not a \"no\"", got)
			}
		})
	}
}
