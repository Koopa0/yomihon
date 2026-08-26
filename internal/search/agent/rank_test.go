package agent

import (
	"context"
	"crypto/sha256"
	"errors"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/search"
	"github.com/koopa0/yomihon/internal/search/semantic"
)

func TestRunValidatesLocallyBeforeOneQueryAndJoinsCurrentEvidence(t *testing.T) {
	t.Parallel()

	ready := fixtureSemanticSearch(t)
	lexical := []search.Result{{RelPath: "b.md", Title: "B", Snippet: "lexical b"}}
	ready.ranked = ready.ranked[:1]
	got, err := runTestSearch(t, lexical, ready, "related idea", 20)
	if err != nil {
		t.Fatal(err)
	}
	if ready.calls != 1 || ready.query != "related idea" {
		t.Fatalf("query sends = %d with %q, want exactly one", ready.calls, ready.query)
	}
	if len(got) != 2 || got[0].RelPath != "a.md" || got[1].RelPath != "b.md" {
		t.Fatalf("results = %+v, want semantic a then lexical b", got)
	}
	if got[0].Title != "Current A" || got[0].Status != "ready" || got[0].Snippet != "semantic evidence" || got[0].Heading != "Detail" {
		t.Errorf("semantic evidence = %+v", got[0])
	}
	if got[0].LexicalRank != 0 || got[0].SemanticRank != 1 {
		t.Errorf("semantic provenance = %+v", got[0])
	}
}

func TestRunRejectsMismatchedFingerprintBeforeQueryEgress(t *testing.T) {
	t.Parallel()

	ready := fixtureSemanticSearch(t)
	snapshot, _ := newTestSnapshot(t, nil, ready, "must stay local")
	ready.corpus.Fingerprint = [sha256.Size]byte{}
	_, err := NewSearch(snapshot, ready)
	if !errors.Is(err, ErrEvidenceMismatch) {
		t.Fatalf("NewSearch() error = %v, want ErrEvidenceMismatch", err)
	}
	if ready.calls != 0 {
		t.Fatalf("query sends = %d, want zero", ready.calls)
	}
}

func TestRunRejectsNonPositiveLimitBeforeQueryEgress(t *testing.T) {
	t.Parallel()

	ready := fixtureSemanticSearch(t)
	_, err := runTestSearch(t, nil, ready, "related idea", 0)
	if !errors.Is(err, ErrInvalidSearch) {
		t.Fatalf("Run(limit 0) error = %v, want ErrInvalidSearch", err)
	}
	if ready.calls != 0 {
		t.Fatalf("query sends = %d, want zero", ready.calls)
	}
}

func TestNewSearchRejectsMissingDependenciesAsInvalidSearch(t *testing.T) {
	t.Parallel()

	ready := fixtureSemanticSearch(t)
	snapshot, _ := newTestSnapshot(t, nil, ready, "related idea")
	for _, tt := range []struct {
		name     string
		snapshot *Snapshot
		semantic SemanticSearch
	}{
		{name: "snapshot", semantic: ready},
		{name: "semantic", snapshot: snapshot},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewSearch(tt.snapshot, tt.semantic)
			if !errors.Is(err, ErrInvalidSearch) {
				t.Errorf("NewSearch() error = %v, want ErrInvalidSearch", err)
			}
		})
	}
}

func TestRunUsesTheSemanticCapabilityPrevalidatedIndex(t *testing.T) {
	t.Parallel()

	ready := fixtureSemanticSearch(t)
	ready.ranked = ready.ranked[:1]
	got, err := runTestSearch(t, nil, ready, "unmatched query", 20)
	if err != nil {
		t.Fatal(err)
	}
	if ready.calls != 1 || len(got) != 1 || got[0].RelPath != "a.md" {
		t.Fatalf("Run() calls/results = %d/%+v, want one send and prevalidated a.md", ready.calls, got)
	}
}

func TestSemanticSnippetIsOneLineBoundedAndRuneSafe(t *testing.T) {
	t.Parallel()

	body := strings.Repeat("界", 68) + "\n tail"
	got := semanticSnippet(body)
	if !strings.HasSuffix(got, "…") || strings.Contains(got, "\n") || len(strings.TrimSuffix(got, "…")) > semanticSnippetBytes {
		t.Fatalf("semanticSnippet() = %q (%d bytes)", got, len(got))
	}
	if got == "" || strings.ContainsRune(got, '�') {
		t.Fatalf("semanticSnippet() broke UTF-8: %q", got)
	}
}

type hybridFakeSemanticSearch struct {
	corpus             semantic.Corpus
	notes              []SnapshotNote
	ranked             []semantic.Rank
	calls              int
	query              string
	ignoreAllowedPaths bool
}

func (f *hybridFakeSemanticSearch) CorpusFingerprint() [sha256.Size]byte {
	return f.corpus.Fingerprint
}

func (f *hybridFakeSemanticSearch) Search(
	_ context.Context,
	query string,
	allowedPaths map[string]struct{},
	depth int,
) ([]semantic.Rank, error) {
	f.calls++
	f.query = query
	result := make([]semantic.Rank, 0, len(f.ranked))
	for _, rank := range f.ranked {
		if allowedPaths != nil && !f.ignoreAllowedPaths {
			if _, allowed := allowedPaths[rank.RelPath]; !allowed {
				continue
			}
		}
		result = append(result, rank)
		if len(result) == depth {
			break
		}
	}
	return result, nil
}

func TestRunAppliesOneHardFilterToBothChannels(t *testing.T) {
	t.Parallel()

	ready := fixtureSemanticSearch(t)
	lexical := []search.Result{{RelPath: "Exports/lexical.md", Title: "Lexical"}}
	got, err := runTestSearch(t, lexical, ready, "related idea folder:missing", 20)
	if err != nil {
		t.Fatal(err)
	}
	if ready.calls != 1 || len(got) != 0 {
		t.Fatalf("Run() calls/results = %d/%+v, want one query and no paths outside the filter", ready.calls, got)
	}
}

func TestRunRejectsSemanticRanksOutsideHardFilter(t *testing.T) {
	t.Parallel()

	ready := fixtureSemanticSearch(t)
	ready.ignoreAllowedPaths = true
	_, err := runTestSearch(t, nil, ready, "related idea folder:a.md", 20)
	if !errors.Is(err, ErrEvidenceMismatch) {
		t.Fatalf("Run() error = %v, want hard-filter evidence rejection", err)
	}
}

func fixtureSemanticSearch(t *testing.T) *hybridFakeSemanticSearch {
	t.Helper()
	notes := []SnapshotNote{
		{RelPath: "a.md", Data: []byte("---\ntitle: Current A\nstatus: ready\n---\n## Section\n### Detail\nsemantic evidence\n")},
		{RelPath: "b.md", Data: []byte("---\ntitle: Current B\n---\nrelated idea other evidence\n")},
	}
	root := writeSearchTestVault(t, "\n[artifacts]\nnon_instance_dirs = [\"Exports\"]\n\n[privacy]\nnever_egress_dirs = []\n", nil)
	contract, err := schema.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := NewSnapshot(t.Context(), notes, contract.ArtifactPolicy(), contract.PrivacyPolicy(), semantic.ChunkTokenCap)
	if err != nil {
		t.Fatal(err)
	}
	return &hybridFakeSemanticSearch{
		corpus: snapshot.semantic,
		notes:  notes,
		ranked: []semantic.Rank{
			{RelPath: "a.md", ChunkOrdinal: 0, Score: 1},
			{RelPath: "b.md", ChunkOrdinal: 0, Score: 0},
		},
	}
}

func runTestSearch(
	t *testing.T,
	lexical []search.Result,
	semanticSearch *hybridFakeSemanticSearch,
	rawQuery string,
	limit int,
) ([]FusedHit, error) {
	t.Helper()
	snapshot, query := newTestSnapshot(t, lexical, semanticSearch, rawQuery)
	searchAction, err := NewSearch(snapshot, semanticSearch)
	if err != nil {
		return nil, err
	}
	return searchAction.Run(t.Context(), query, limit)
}

func newTestSnapshot(
	t *testing.T,
	lexical []search.Result,
	semanticSearch *hybridFakeSemanticSearch,
	rawQuery string,
) (*Snapshot, search.Query) {
	t.Helper()
	notes := make([]SnapshotNote, len(semanticSearch.notes), len(semanticSearch.notes)+len(lexical))
	byPath := make(map[string]struct{}, len(semanticSearch.notes)+len(lexical))
	for i := range semanticSearch.notes {
		notes[i] = SnapshotNote{
			RelPath: semanticSearch.notes[i].RelPath,
			Data:    append([]byte(nil), semanticSearch.notes[i].Data...),
		}
		byPath[notes[i].RelPath] = struct{}{}
	}
	query := search.Parse(rawQuery)
	for i := range lexical {
		result := &lexical[i]
		if _, ok := byPath[result.RelPath]; ok {
			continue
		}
		byPath[result.RelPath] = struct{}{}
		data := []byte("---\ntitle: " + result.Title + "\nstatus: " + result.Status + "\n---\n" + query.BareText() + " " + result.Snippet + "\n")
		notes = append(notes, SnapshotNote{RelPath: result.RelPath, Data: data})
	}
	root := writeSearchTestVault(t, "\n[artifacts]\nnon_instance_dirs = [\"Exports\"]\n\n[privacy]\nnever_egress_dirs = []\n", nil)
	contract, err := schema.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := NewSnapshot(t.Context(), notes, contract.ArtifactPolicy(), contract.PrivacyPolicy(), semantic.ChunkTokenCap)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot, query
}
