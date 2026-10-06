package vault

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestRootIdentityFollowsTheSelectedAliasAndPinnedObject(t *testing.T) {
	parent := t.TempDir()
	original := filepath.Join(parent, "original")
	replacement := filepath.Join(parent, "replacement")
	alias := filepath.Join(parent, "selected")
	for _, name := range []string{original, replacement} {
		if err := os.Mkdir(name, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(original, alias); err != nil {
		t.Fatal(err)
	}
	reader, openErr := Open(alias)
	if openErr != nil {
		t.Fatal(openErr)
	}
	t.Cleanup(func() {
		if err := reader.Close(); err != nil {
			t.Error(err)
		}
	})
	opened := reader.Name()
	for _, tt := range []struct {
		name     string
		target   string
		wantSame bool
	}{
		{name: "same object", target: original, wantSame: true},
		{name: "retargeted alias", target: replacement},
		{name: "alternate same object spelling", target: filepath.Join(original, "."), wantSame: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.Remove(alias); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(tt.target, alias); err != nil {
				t.Fatal(err)
			}
			got, err := reader.ObserveRoot()
			if err != nil {
				t.Fatal(err)
			}
			want := RootIdentity{SelectedPath: alias, OpenedName: opened, Same: tt.wantSame}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Fatalf("selected alias identity (-want +got):\n%s", diff)
			}
			if reader.Name() != opened {
				t.Fatal("startup root name changed")
			}
		})
	}
	if err := os.Rename(original, original+".old"); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(alias); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(original+".old", alias); err != nil {
		t.Fatal(err)
	}
	got, err := reader.ObserveRoot()
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(RootIdentity{SelectedPath: alias, OpenedName: opened, Same: true}, got); diff != "" {
		t.Fatalf("alias to renamed pinned object (-want +got):\n%s", diff)
	}
	pinned, err := os.OpenRoot(original + ".old")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := pinned.Close(); err != nil {
			t.Error(err)
		}
	})
	if same, err := reader.SameRoot(pinned); err != nil || !same {
		t.Fatalf("pinned writer identity changed: same=%t error=%v", same, err)
	}
}

func TestRootIdentityDistinguishesMissingFileReplacementAndClosedReader(t *testing.T) {
	t.Parallel()
	selected := filepath.Join(t.TempDir(), "selected")
	if err := os.Mkdir(selected, 0o700); err != nil {
		t.Fatal(err)
	}
	reader, openErr := Open(selected)
	if openErr != nil {
		t.Fatal(openErr)
	}
	t.Cleanup(func() {
		if err := reader.Close(); err != nil {
			t.Error(err)
		}
	})
	opened := reader.Name()
	if err := os.Rename(selected, selected+".old"); err != nil {
		t.Fatal(err)
	}
	want := RootIdentity{SelectedPath: selected, OpenedName: opened}
	got, observeErr := reader.ObserveRoot()
	if !errors.Is(observeErr, fs.ErrNotExist) {
		t.Fatalf("missing selected path error = %v", observeErr)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("unconfirmed identity (-want +got):\n%s", diff)
	}
	if err := os.WriteFile(selected, []byte("replacement"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, observeErr = reader.ObserveRoot()
	if observeErr != nil {
		t.Fatal(observeErr)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("file replacement identity (-want +got):\n%s", diff)
	}
	if err := os.Remove(selected); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(selected+".old", selected); err != nil {
		t.Fatal(err)
	}
	got, observeErr = reader.ObserveRoot()
	if observeErr != nil {
		t.Fatal(observeErr)
	}
	want.Same = true
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("restored identity (-want +got):\n%s", diff)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	got, observeErr = reader.ObserveRoot()
	if !errors.Is(observeErr, fs.ErrClosed) {
		t.Fatalf("closed root error = %v", observeErr)
	}
	want.Same = false
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("closed root labels (-want +got):\n%s", diff)
	}
}
