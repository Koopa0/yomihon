package shell

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/status"
	"github.com/koopa0/yomihon/internal/vaultfs"
)

func lifecycleView(t *testing.T, contract *schema.Contract) status.Authority {
	t.Helper()
	return governedLifecycleView(t, contract, contract.Governance())
}

func governedLifecycleView(t *testing.T, contract *schema.Contract, governance schema.Governance) status.Authority {
	t.Helper()
	root := t.TempDir()
	reader, err := vaultfs.Open(root)
	if err != nil {
		t.Fatalf("vaultfs.Open(%q) error = %v", root, err)
	}
	t.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Errorf("Reader.Close() error = %v", closeErr)
		}
	})
	lifecycle, err := status.Open(reader, contract, governance, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("status.Open(%q) error = %v", root, err)
	}
	t.Cleanup(func() {
		if closeErr := lifecycle.Close(); closeErr != nil {
			t.Errorf("Lifecycle.Close() error = %v", closeErr)
		}
	})
	return lifecycle.Authority()
}

func snapshotView(t *testing.T, contract *schema.Contract, notes map[string]string) *snapshot.Generation {
	t.Helper()
	return governedSnapshotView(t, contract, contract.Governance(), notes)
}

func governedSnapshotView(
	t *testing.T,
	contract *schema.Contract,
	governance schema.Governance,
	notes map[string]string,
) *snapshot.Generation {
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
	reader, err := vaultfs.Open(root)
	if err != nil {
		t.Fatalf("vaultfs.Open(%q) error = %v", root, err)
	}
	t.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Errorf("Reader.Close() error = %v", closeErr)
		}
	})
	store, err := snapshot.New(t.Context(), reader, slog.New(slog.DiscardHandler), contract, governance)
	if err != nil {
		t.Fatalf("snapshot.New() error = %v", err)
	}
	return store.Current()
}

// TestProjectOpensInstanceStateOnAGovernedLifecycle is the positive case the
// two closure tests below are measured against: a folder whose contract loads
// and whose notes are ordinary instances keeps both its governed answer and
// its instance projections. The notes are written across several statuses so
// the projection is exercised on a populated vault rather than an empty one.
func TestProjectOpensInstanceStateOnAGovernedLifecycle(t *testing.T) {
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

	got := Project(lifecycleView(t, contract), snap)
	if got.Nav.ArtifactClosure().Closed() {
		t.Error("Project() closed instance projections on an open lifecycle")
	}
	if !got.Governed {
		t.Error("Project() reports an open governed lifecycle as ungoverned")
	}
}

func TestProjectClosesInstanceStateWithEitherUnavailableAuthority(t *testing.T) {
	t.Parallel()

	contract, err := schema.LoadFile(filepath.Join("testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}
	rejected, err := schema.LoadFile(filepath.Join("testdata", "rejected-artifacts.toml"))
	if err != nil {
		t.Fatalf("LoadFile(rejected-artifacts.toml) error = %v", err)
	}
	notes := map[string]string{"Writing/A.md": "---\ntitle: A\ntype: lesson\nstatus: draft\n---\n"}
	open := lifecycleView(t, contract)
	// Two ways an authority refuses, sampled independently: a contract that
	// exists and cannot be read, and one that loaded with an artifact policy
	// yomihon had to reject.
	unreadable := governedLifecycleView(t, nil, schema.Unreadable(errors.New("toml: line 42: expected a key separator")))
	openSnapshot := snapshotView(t, contract, notes)
	rejectedSnapshot := snapshotView(t, rejected, notes)

	tests := []struct {
		name      string
		lifecycle status.Authority
		snap      *snapshot.Generation
	}{
		{name: "lifecycle", lifecycle: unreadable, snap: openSnapshot},
		{name: "artifact policy", lifecycle: open, snap: rejectedSnapshot},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := Project(tt.lifecycle, tt.snap)
			// The recent-notes summary is plain reading and survives the
			// refusal; what the refusal removes is the knowledge-layer
			// citation, which is the refused contract's own claim.
			if len(got.Nav.KnowledgeNotes()) != 1 {
				t.Errorf("Project() KnowledgeNotes = %d, want 1: plain reading survives a refusing authority", len(got.Nav.KnowledgeNotes()))
			}
			if got.Nav.KnowledgeScoped() {
				t.Error("Project() kept the knowledge-layer citation under a refusing authority")
			}
			if len(got.Nav.Paths()) != 0 {
				t.Errorf("Project() retained %d study paths under a refusing authority", len(got.Nav.Paths()))
			}
			if !got.Nav.ArtifactClosure().Closed() {
				t.Error("Project() left instance projections open under a refusing authority")
			}
			// The closure carries the refusing authority's own sentence, so a
			// surface reading only this model still has something true to say.
			if got.Nav.ArtifactClosure().Diagnostic() == "" {
				t.Error("Project() closed the projection and gave no reason")
			}
		})
	}
}

// TestProjectKeepsProjectionsOpenForAnUngovernedFolder is the case the old
// closure conflated with a failure. Nothing declared an exclusion here, so the
// instance projections are answerable and stay open; what is absent is the
// lifecycle vocabulary, so nothing is offered from it and no fault is
// reported.
func TestProjectKeepsProjectionsOpenForAnUngovernedFolder(t *testing.T) {
	t.Parallel()

	notes := map[string]string{"Writing/A.md": "---\ntitle: A\ntype: lesson\nstatus: draft\n---\n"}
	view := governedLifecycleView(t, nil, schema.Ungoverned())
	snap := governedSnapshotView(t, nil, schema.Ungoverned(), notes)

	got := Project(view, snap)
	if got.Governed {
		t.Error("Project() reports an ungoverned folder as governed")
	}
	if got.Nav.ArtifactClosure().Closed() {
		t.Error("Project() closed instance projections for a folder that declared no exclusions")
	}
	if got.Nav.ArtifactClosure().Diagnostic() != "" || got.Nav.NavigationClosure().Diagnostic() != "" {
		t.Errorf("Project() reported a fault on an ungoverned folder: artifact %q navigation %q",
			got.Nav.ArtifactClosure().Diagnostic(), got.Nav.NavigationClosure().Diagnostic())
	}
	if len(got.Nav.KnowledgeNotes()) != 1 {
		t.Errorf("Project() KnowledgeNotes = %d, want 1: nothing was excluded", len(got.Nav.KnowledgeNotes()))
	}
}
