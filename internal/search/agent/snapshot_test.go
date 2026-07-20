package agent

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/search"
	"github.com/koopa0/yomihon/internal/search/semantic"
	"github.com/koopa0/yomihon/internal/vault"
)

func TestReadSnapshotDerivesBothChannelsFromOnePublicSnapshot(t *testing.T) {
	root := writeSearchTestVault(t, `
[artifacts]
non_instance_dirs = ["Exports"]

[privacy]
never_egress_dirs = ["Private"]
`, map[string]string{
		"Writing/public.md":  "---\ntitle: Public\ntype: concept\nstatus: ready\n---\npublic token\n",
		"Exports/report.md":  "---\ntitle: Report\n---\nreport token\n",
		"Private/private.md": "---\ntitle: Private\ntype: concept\n---\nprivate token\n",
	})
	reader := openSearchVaultReader(t, root)
	contract, err := schema.LoadReader(t.Context(), reader)
	if err != nil {
		t.Fatal(err)
	}

	captured, err := readSnapshot(
		t.Context(),
		reader,
		contract.ArtifactPolicy(),
		contract.PrivacyPolicy(),
		true,
	)
	if err != nil {
		t.Fatal(err)
	}
	if captured.semanticErr != nil {
		t.Fatal(captured.semanticErr)
	}
	corpus := captured.snapshot
	lexical, err := corpus.lexical.Search(search.Parse("token"))
	if err != nil {
		t.Fatal(err)
	}
	if len(lexical) != 2 || lexical[0].RelPath != "Exports/report.md" || lexical[1].RelPath != "Writing/public.md" {
		t.Fatalf("lexical paths = %+v, want public instance plus public non-instance", lexical)
	}
	if got := corpus.semantic.Chunks; len(got) != 1 || got[0].RelPath != "Writing/public.md" || got[0].Body != "public token" {
		t.Fatalf("semantic documents = %+v, want exact public instance", got)
	}
}

func TestReadSnapshotNeverOpensPrivateNotes(t *testing.T) {
	root := writeSearchTestVault(t, `
[artifacts]
non_instance_dirs = []

[privacy]
never_egress_dirs = ["Private"]
`, map[string]string{
		"Writing/public.md":  "public token\n",
		"Private/private.md": "private token\n",
	})
	if err := os.Chmod(filepath.Join(root, "Private", "private.md"), 0); err != nil {
		t.Fatal(err)
	}
	reader := openSearchVaultReader(t, root)
	contract, err := schema.LoadReader(t.Context(), reader)
	if err != nil {
		t.Fatal(err)
	}

	captured, err := readSnapshot(t.Context(), reader, contract.ArtifactPolicy(), contract.PrivacyPolicy(), true)
	if err != nil {
		t.Fatal(err)
	}
	if captured.semanticErr != nil {
		t.Fatal(captured.semanticErr)
	}
	corpus := captured.snapshot
	if corpus.lexical.Len() != 1 || len(corpus.semantic.Chunks) != 1 {
		t.Fatalf("corpus sizes = lexical %d, semantic %d; want 1/1", corpus.lexical.Len(), len(corpus.semantic.Chunks))
	}
}

func openSearchVaultReader(t *testing.T, root string) *vault.Reader {
	t.Helper()
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Errorf("close vault reader: %v", closeErr)
		}
	})
	return reader
}

func TestNewSnapshotDerivesBothChannelsFromOneRawNote(t *testing.T) {
	root := writeSearchTestVault(t, `
[artifacts]
non_instance_dirs = []

[privacy]
never_egress_dirs = []
`, nil)
	contract, err := schema.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	notes := []SnapshotNote{{
		RelPath: "Writing/public.md",
		Data:    []byte("---\ntitle: Public\nstatus: ready\n---\nsemantic evidence\n"),
	}}
	snapshot, err := NewSnapshot(
		t.Context(),
		notes,
		contract.ArtifactPolicy(),
		contract.PrivacyPolicy(),
		semantic.ChunkTokenCap,
	)
	if err != nil {
		t.Fatal(err)
	}
	lexical, err := snapshot.lexical.Search(search.Parse("evidence"))
	if err != nil {
		t.Fatal(err)
	}
	if len(lexical) != 1 || lexical[0].Title != "Public" || len(snapshot.semantic.Chunks) != 1 ||
		snapshot.semantic.Chunks[0].Title != "Public" || snapshot.semantic.Chunks[0].Status != "ready" {
		t.Fatalf("snapshot projections disagree: lexical=%+v semantic=%+v", lexical, snapshot.semantic.Chunks)
	}
}

func TestNewSnapshotOwnsRawInputAndDropsNonInstanceFromSemantic(t *testing.T) {
	root := writeSearchTestVault(t, `
[artifacts]
non_instance_dirs = ["Exports"]

[privacy]
never_egress_dirs = ["Private"]
`, nil)
	contract, err := schema.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	notes := []SnapshotNote{{
		RelPath: "Writing/public.md",
		Data:    []byte("---\ntitle: Public\nstatus: ready\ntopics: [search]\n---\n## Evidence\nsemantic evidence\n"),
	}}
	snapshot, err := NewSnapshot(
		t.Context(),
		notes,
		contract.ArtifactPolicy(),
		contract.PrivacyPolicy(),
		semantic.ChunkTokenCap,
	)
	if err != nil {
		t.Fatal(err)
	}
	for i := range notes[0].Data {
		notes[0].Data[i] = 'X'
	}
	results, err := snapshot.lexical.Search(search.Parse("evidence"))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Title != "Public" ||
		snapshot.semantic.Chunks[0].Body != "semantic evidence" ||
		snapshot.semantic.Chunks[0].Headings[0] != "Evidence" {
		t.Fatalf("caller mutation changed snapshot: lexical=%+v semantic=%+v", results, snapshot.semantic.Chunks[0])
	}

	private := SnapshotNote{RelPath: "Private/private.md", Data: []byte("private")}
	if _, snapshotErr := NewSnapshot(t.Context(), []SnapshotNote{private}, contract.ArtifactPolicy(), contract.PrivacyPolicy(), semantic.ChunkTokenCap); !errors.Is(snapshotErr, ErrInvalidSnapshot) {
		t.Fatalf("NewSnapshot(private) error = %v, want ErrInvalidSnapshot", snapshotErr)
	}

	nonInstance, err := NewSnapshot(
		t.Context(),
		[]SnapshotNote{{RelPath: "Exports/report.md", Data: []byte("report token")}},
		contract.ArtifactPolicy(),
		contract.PrivacyPolicy(),
		semantic.ChunkTokenCap,
	)
	if err != nil {
		t.Fatal(err)
	}
	if nonInstance.lexical.Len() != 1 || len(nonInstance.semantic.Chunks) != 0 {
		t.Fatalf("non-instance snapshot = lexical %d, semantic %d; want 1/0", nonInstance.lexical.Len(), len(nonInstance.semantic.Chunks))
	}
}

func TestNewSnapshotDistinguishesUnavailablePoliciesFromInvalidEvidence(t *testing.T) {
	t.Parallel()

	root := writeSearchTestVault(t, `
[artifacts]
non_instance_dirs = []

[privacy]
never_egress_dirs = []
`, nil)
	contract, err := schema.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name     string
		artifact schema.ArtifactPolicy
		privacy  schema.PrivacyPolicy
		want     error
	}{
		{name: "artifact", privacy: contract.PrivacyPolicy(), want: semantic.ErrArtifactPolicyUnavailable},
		{name: "privacy", artifact: contract.ArtifactPolicy(), want: semantic.ErrPrivacyPolicyUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewSnapshot(t.Context(), nil, tt.artifact, tt.privacy, semantic.ChunkTokenCap)
			if !errors.Is(err, tt.want) {
				t.Errorf("NewSnapshot() error = %v, want %v", err, tt.want)
			}
		})
	}
}
