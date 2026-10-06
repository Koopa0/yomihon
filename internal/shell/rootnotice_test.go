package shell

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/status"
	"github.com/koopa0/yomihon/internal/vault"
)

func TestRootNoticeReachesTheShellWithoutChangingFindingCounts(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "vault")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	reader, openErr := vault.Open(root)
	if openErr != nil {
		t.Fatal(openErr)
	}
	t.Cleanup(func() {
		if err := reader.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := os.Rename(root, root+".old"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	store, err := snapshot.New(t.Context(), reader, slog.New(slog.DiscardHandler), nil, schema.Governance{})
	if err != nil {
		t.Fatal(err)
	}
	view := store.Current().Capture()
	found := GatherFindings(status.Authority{}, view)
	want := &snapshot.RootNotice{SelectedPath: root, OpenedName: reader.Name()}
	if diff := cmp.Diff(want, found.Root); diff != "" {
		t.Fatalf("shell root projection (-want +got):\n%s", diff)
	}
	if found.Total() != 0 {
		t.Fatalf("root notice counted as %d finding rows", found.Total())
	}
	if diff := cmp.Diff(nav.Vault{Name: "vault", Noticed: true}, Project("vault", status.Authority{}, view).Vault); diff != "" {
		t.Fatalf("standing root cue (-want +got):\n%s", diff)
	}
	found.Root.SelectedPath = "caller mutation"
	if diff := cmp.Diff(want, GatherFindings(status.Authority{}, view).Root); diff != "" {
		t.Fatalf("gathered root notice changed the live fact (-want +got):\n%s", diff)
	}
}
