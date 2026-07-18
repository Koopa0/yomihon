package shell

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/status"
	"github.com/koopa0/yomihon/internal/vault"
)

func lifecycleView(t *testing.T, contract *schema.Contract) status.View {
	t.Helper()
	root := t.TempDir()
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open(%q) error = %v", root, err)
	}
	t.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Errorf("Reader.Close() error = %v", closeErr)
		}
	})
	lifecycle, err := status.Open(reader, contract)
	if err != nil {
		t.Fatalf("status.Open(%q) error = %v", root, err)
	}
	t.Cleanup(func() {
		if closeErr := lifecycle.Close(); closeErr != nil {
			t.Errorf("Lifecycle.Close() error = %v", closeErr)
		}
	})
	return lifecycle.View()
}

func snapshotView(t *testing.T, contract *schema.Contract, notes map[string]string) *snapshot.View {
	t.Helper()
	root := t.TempDir()
	for relPath, body := range notes {
		full := filepath.Join(root, filepath.FromSlash(relPath))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("creating %q: %v", relPath, err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatalf("writing %q: %v", relPath, err)
		}
	}
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open(%q) error = %v", root, err)
	}
	t.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Errorf("Reader.Close() error = %v", closeErr)
		}
	})
	store, err := snapshot.New(t.Context(), reader, slog.New(slog.DiscardHandler), contract)
	if err != nil {
		t.Fatalf("snapshot.New() error = %v", err)
	}
	return store.Current()
}

func TestProjectCountsOwnerHeldNamedTransitions(t *testing.T) {
	t.Parallel()

	contract, err := schema.LoadFile(filepath.Join("testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}
	counts := map[string]int{
		"imported":        2,
		"draft":           4,
		schema.SealStatus: 3,
		"beyond":          9,
	}
	notes := make(map[string]string)
	for noteStatus, n := range counts {
		for i := range n {
			relPath := fmt.Sprintf("Writing/%s-%d.md", noteStatus, i)
			notes[relPath] = fmt.Sprintf("---\ntitle: %s\ntype: lesson\nstatus: %s\n---\n", relPath, noteStatus)
		}
	}
	snap := snapshotView(t, contract, notes)

	got := Project(lifecycleView(t, contract), contract.ArtifactPolicy().Capture(), snap)
	if !got.AdvanceableKnown {
		t.Fatal("Project() advanceable count is unknown on an open lifecycle")
	}
	if got.Advanceable != 9 {
		t.Errorf("Project() advanceable = %d, want 9", got.Advanceable)
	}
}

func TestProjectClosesInstanceStateWithEitherUnavailableAuthority(t *testing.T) {
	t.Parallel()

	contract, err := schema.LoadFile(filepath.Join("testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}
	open := lifecycleView(t, contract)
	closed := lifecycleView(t, nil)
	notes := map[string]string{"Writing/A.md": "---\ntitle: A\ntype: lesson\nstatus: draft\n---\n"}
	openSnapshot := snapshotView(t, contract, notes)
	closedSnapshot := snapshotView(t, nil, notes)

	tests := []struct {
		name      string
		lifecycle Lifecycle
		snap      *snapshot.View
	}{
		{name: "lifecycle", lifecycle: closed, snap: openSnapshot},
		{name: "artifact policy", lifecycle: open, snap: closedSnapshot},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := Project(tt.lifecycle, tt.snap.ArtifactPolicy(), tt.snap)
			if got.Advanceable != 0 || got.AdvanceableKnown {
				t.Errorf("Project() advanceable = (%d, %t), want (0, false)", got.Advanceable, got.AdvanceableKnown)
			}
			if len(got.Nav.KnowledgeNotes()) != 0 {
				t.Errorf("Project() retained %d instance notes", len(got.Nav.KnowledgeNotes()))
			}
			if got.Nav.ArtifactDiagnostic() == "" {
				t.Error("Project() has no closure diagnostic")
			}
		})
	}
}
