package agent

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/search"
	"github.com/koopa0/yomihon/internal/search/semantic"
	"github.com/koopa0/yomihon/internal/vault"
)

var errSnapshotRead = errors.New("agent search snapshot could not be read completely")

// ErrInvalidSnapshot reports malformed or ineligible caller-supplied notes.
var ErrInvalidSnapshot = errors.New("agent search snapshot input is invalid")

// ErrEvidenceMismatch reports a disagreement between local snapshot evidence
// and the immutable semantic generation selected for the action.
var ErrEvidenceMismatch = errors.New("agent search evidence does not match the semantic generation")

type evidenceKey struct {
	relPath string
	ordinal uint32
}

func semanticEvidence(corpus semantic.Corpus) (map[evidenceKey]*semantic.CorpusChunk, error) {
	targets := make([]semantic.ChunkTarget, 0, len(corpus.Chunks))
	evidence := make(map[evidenceKey]*semantic.CorpusChunk, len(corpus.Chunks))
	for i := range corpus.Chunks {
		chunk := &corpus.Chunks[i]
		target, err := chunk.Target()
		if err != nil {
			return nil, ErrEvidenceMismatch
		}
		key := evidenceKey{relPath: target.RelPath, ordinal: target.Ordinal}
		if _, duplicate := evidence[key]; duplicate {
			return nil, ErrEvidenceMismatch
		}
		targets = append(targets, target)
		evidence[key] = chunk
	}
	fingerprint, err := semantic.TargetFingerprint(targets)
	if err != nil || fingerprint != corpus.Fingerprint {
		return nil, ErrEvidenceMismatch
	}
	return evidence, nil
}

// Snapshot is the single local snapshot behind one agent-facing search.
// Both channels are derived from the same parsed bytes; private notes are
// rejected by path before their bytes are opened.
type Snapshot struct {
	lexical  *search.Index
	semantic semantic.Corpus
}

// SnapshotNote is one vault-relative note whose raw bytes are the sole source
// for both lexical and semantic projections.
type SnapshotNote struct {
	RelPath string
	Data    []byte
	Source  vault.Entry
}

type noteProjection struct {
	lexical  []search.Document
	semantic []semantic.SourceNote
}

type snapshotRead struct {
	snapshot    *Snapshot
	semanticErr error
}

// NewSnapshot builds a deterministic in-memory snapshot from raw note bytes.
// It rejects private paths before parsing, derives both channels from each
// note's single byte slice, and retains no caller-owned data. Production
// filesystem actions use readSnapshot.
func NewSnapshot(
	ctx context.Context,
	notes []SnapshotNote,
	artifact schema.ArtifactPolicy,
	privacy schema.PrivacyPolicy,
	chunkTokenCap int,
) (*Snapshot, error) {
	if !artifact.Available() {
		return nil, semantic.ErrArtifactPolicyUnavailable
	}
	if !privacy.Available() {
		return nil, semantic.ErrPrivacyPolicyUnavailable
	}
	if !artifact.ValidateSource().Available() || !privacy.ValidateSource().Available() {
		return nil, semantic.ErrPolicySourceChanged
	}
	projection, err := projectNotes(ctx, notes, artifact, privacy)
	if err != nil {
		return nil, err
	}
	semanticCorpus, err := semantic.NewCorpus(ctx, projection.semantic, artifact, privacy, chunkTokenCap)
	if err != nil {
		return nil, err
	}
	if _, err := semanticEvidence(semanticCorpus); err != nil {
		return nil, err
	}
	return &Snapshot{lexical: search.NewIndex(projection.lexical, artifact), semantic: semanticCorpus}, nil
}

func projectNotes(
	ctx context.Context,
	notes []SnapshotNote,
	artifact schema.ArtifactPolicy,
	privacy schema.PrivacyPolicy,
) (noteProjection, error) {
	if !privacy.Available() || !privacy.ValidateSource().Available() {
		return noteProjection{}, semantic.ErrPolicySourceChanged
	}
	projection := noteProjection{
		lexical:  make([]search.Document, 0, len(notes)),
		semantic: make([]semantic.SourceNote, 0, len(notes)),
	}
	seen := make(map[string]struct{}, len(notes))
	for i := range notes {
		if err := ctx.Err(); err != nil {
			return noteProjection{}, err
		}
		source := &notes[i]
		if source.RelPath == "" || !filepath.IsLocal(filepath.FromSlash(source.RelPath)) ||
			!privacy.EgressAllowed(source.RelPath) || !utf8.Valid(source.Data) {
			return noteProjection{}, ErrInvalidSnapshot
		}
		if _, duplicate := seen[source.RelPath]; duplicate {
			return noteProjection{}, ErrInvalidSnapshot
		}
		seen[source.RelPath] = struct{}{}
		note := vault.Parse(source.RelPath, source.Data)
		projection.lexical = append(projection.lexical, search.DocumentFromNote(note))
		if artifact.Available() && !artifact.IsNonInstance(source.RelPath) {
			projection.semantic = append(projection.semantic, semantic.SourceNote{
				RelPath:  source.RelPath,
				Title:    note.Title(),
				Status:   note.Status(),
				Body:     note.Body,
				NoteHash: sha256.Sum256(source.Data),
				Source:   source.Source,
			})
		}
	}
	return projection, nil
}

func readSnapshot(
	ctx context.Context,
	reader *vault.Reader,
	artifact schema.ArtifactPolicy,
	privacy schema.PrivacyPolicy,
	withSemantic bool,
) (snapshotRead, error) {
	if err := ctx.Err(); err != nil {
		return snapshotRead{}, err
	}
	if !privacy.Available() || !privacy.ValidateSource().Available() {
		return snapshotRead{}, semantic.ErrPolicySourceChanged
	}
	notes, err := readSnapshotNotes(ctx, reader, privacy)
	if err != nil {
		return snapshotRead{}, err
	}
	if !privacy.ValidateSource().Available() {
		return snapshotRead{}, semantic.ErrPolicySourceChanged
	}
	currentArtifact := artifact.ValidateSource()
	projection, err := projectNotes(ctx, notes, currentArtifact, privacy)
	if err != nil {
		return snapshotRead{}, err
	}
	result := snapshotRead{snapshot: &Snapshot{lexical: search.NewIndex(projection.lexical, currentArtifact)}}
	if !withSemantic {
		return result, nil
	}
	if !currentArtifact.Available() {
		result.semanticErr = semantic.ErrPolicySourceChanged
		return result, nil
	}
	result.snapshot.semantic, result.semanticErr = semantic.NewCorpus(
		ctx,
		projection.semantic,
		currentArtifact,
		privacy,
		semantic.ChunkTokenCap,
	)
	if result.semanticErr == nil {
		_, result.semanticErr = semanticEvidence(result.snapshot.semantic)
	}
	return result, nil
}

func readSnapshotNotes(
	ctx context.Context,
	reader *vault.Reader,
	privacy schema.PrivacyPolicy,
) ([]SnapshotNote, error) {
	if reader == nil {
		return nil, fmt.Errorf("%w: vault reader is nil", errSnapshotRead)
	}
	entries, err := reader.Entries(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: list vault", errSnapshotRead)
	}

	notes := make([]SnapshotNote, 0, len(entries))
	for _, entry := range entries {
		if contextErr := ctx.Err(); contextErr != nil {
			return nil, contextErr
		}
		relPath := entry.Path()
		if !strings.HasSuffix(relPath, ".md") || !privacy.EgressAllowed(relPath) {
			continue
		}
		if !filepath.IsLocal(filepath.FromSlash(relPath)) {
			return nil, fmt.Errorf("%w: invalid note path", errSnapshotRead)
		}
		data, readErr := reader.ReadFile(ctx, entry)
		if readErr != nil {
			return nil, fmt.Errorf("%w: read note", errSnapshotRead)
		}
		if !utf8.Valid(data) {
			return nil, fmt.Errorf("%w: note is not UTF-8", errSnapshotRead)
		}
		notes = append(notes, SnapshotNote{RelPath: relPath, Data: data, Source: entry})
	}
	return notes, nil
}
