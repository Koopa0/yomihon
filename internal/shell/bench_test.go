package shell

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/status"
	"github.com/koopa0/yomihon/internal/vault"
)

// benchNoteCount is the folder this is measured against: larger than the
// low-thousands the reading server is built for, so a number taken here is an
// upper bound on what a real folder costs rather than a hopeful one.
const benchNoteCount = 5000

// BenchmarkGatherFindings measures one whole-folder gathering, which is what
// every page pays now that the foot of the rail states how many findings stand
// against the folder. It is separate from the projection below so the two
// questions — what the gathering costs, and what a page costs with it — are
// answered by different numbers.
func BenchmarkGatherFindings(b *testing.B) {
	lifecycle, snap := benchFolder(b)
	b.ReportAllocs()
	for b.Loop() {
		found := GatherFindings(lifecycle, snap)
		if found.Total() < 0 {
			b.Fatal("negative total")
		}
	}
}

// BenchmarkProject measures the whole shared projection a full page takes,
// gathering included, beside the same projection without it. The two run in
// one process so the comparison survives a machine doing other work: what the
// foot costs a page is the difference between them, not either number alone.
func BenchmarkProject(b *testing.B) {
	lifecycle, snap := benchFolder(b)
	b.Run("with-the-foot", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			projected := Project("bench-vault", lifecycle, snap)
			if projected.Vault.Notes == 0 {
				b.Fatal("the projection counted no notes")
			}
			if projected.Nav.ArtifactClosure().Closed() {
				b.Fatal("the projection took the degraded path, so this measures the wrong one")
			}
		}
	})
	// Everything Project reads except the gathering, so the pair differs by
	// exactly the work the foot's three lines added.
	b.Run("without-the-foot", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			policy := snap.ArtifactPolicy()
			claim := lifecycle.Claim()
			projected := nav.Shell{Nav: snap.Navigation(), Governed: lifecycle.Governed()}
			if !claim.Trustworthy() || !policy.Trustworthy() || projected.Nav == nil {
				b.Fatal("the stand-in took a different path from the projection it stands in for")
			}
		}
	})
}

// benchFolder builds one folder of benchNoteCount notes and the lifecycle view
// a request would hold beside it. Every note declares a type, a domain and a
// status the contract knows, which is the ordinary shape of a folder somebody
// keeps; every fourth cites a name nothing answers to, so the gathering reports
// something rather than walking an empty folder.
func benchFolder(b *testing.B) (status.Authority, *snapshot.Generation) {
	b.Helper()
	root := b.TempDir()
	for i := range benchNoteCount {
		rel := fmt.Sprintf("Writing/lessons/L%04d.md", i)
		body := fmt.Sprintf(
			"---\ntitle: L%04d\ntype: lesson\ndomain: japanese\nstatus: draft\n"+
				"created: 2026-06-01\nupdated: 2026-06-01\n---\n\nbody\n",
			i,
		)
		if i%4 == 0 {
			body += fmt.Sprintf("cites [[Nobody wrote %04d]]\n", i)
		}
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			b.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			b.Fatalf("write %s: %v", rel, err)
		}
	}
	contract, err := schema.LoadFile(filepath.Join("testdata", "contract.toml"))
	if err != nil {
		b.Fatalf("schema.LoadFile() error = %v", err)
	}
	reader, err := vault.Open(root)
	if err != nil {
		b.Fatalf("vault.Open() error = %v", err)
	}
	b.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			b.Errorf("Reader.Close() error = %v", closeErr)
		}
	})
	lifecycle, err := status.Open(reader, contract, contract.Governance(), slog.New(slog.DiscardHandler))
	if err != nil {
		b.Fatalf("status.Open() error = %v", err)
	}
	b.Cleanup(func() {
		if closeErr := lifecycle.Close(); closeErr != nil {
			b.Errorf("Lifecycle.Close() error = %v", closeErr)
		}
	})
	store, err := snapshot.New(b.Context(), reader, slog.New(slog.DiscardHandler), contract, contract.Governance())
	if err != nil {
		b.Fatalf("snapshot.New() error = %v", err)
	}
	snap := store.Current().Capture()
	if snap.NoteCount() != benchNoteCount {
		b.Fatalf("the folder built for this benchmark holds %d notes, want %d", snap.NoteCount(), benchNoteCount)
	}
	return lifecycle.Authority(), snap
}
