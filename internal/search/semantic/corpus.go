package semantic

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

// Corpus errors distinguish policy drift, incomplete reads, and chunking
// failures so callers never publish a partial generation.
var (
	ErrPolicySourceChanged = errors.New("semantic corpus policy source changed during the action")
	ErrCorpusRead          = errors.New("semantic corpus could not be read completely")
	ErrCorpusChunking      = errors.New("semantic corpus could not be chunked completely")
)

// Corpus is one locally collected eligible corpus snapshot. Chunks carry
// current local evidence for ranking output; Fingerprint binds only exact
// paths/content/submissions, and ProxyTokens is the pre-egress budget total.
type Corpus struct {
	Chunks      []CorpusChunk
	Fingerprint [sha256.Size]byte
	ProxyTokens int
}

// NewCorpus derives one semantic corpus from already-read local notes. It lets
// an agent-search action derive its lexical and semantic channels from the same
// filesystem snapshot instead of scanning the vault twice.
func NewCorpus(
	ctx context.Context,
	sources []SourceNote,
	artifact schema.ArtifactPolicy,
	privacy schema.PrivacyPolicy,
	chunkTokenCap int,
) (Corpus, error) {
	collector := corpusCollector{artifact: artifact, privacy: privacy, chunkTokenCap: chunkTokenCap}
	if err := collector.validateAuthority(ctx); err != nil {
		return Corpus{}, err
	}
	collection, err := collectCorpusChunks(sources, artifact, privacy, chunkTokenCap)
	if err != nil {
		return Corpus{}, err
	}
	if len(collection.Failures) != 0 {
		return Corpus{}, ErrCorpusChunking
	}
	return collector.finish(ctx, collection.Chunks)
}

// readCorpus performs a complete local read of every eligible Markdown note.
// It filters non-instance and never-egress paths before opening note bytes,
// fails rather than publishing a partial walk, and checks both policy sources
// before and after the scan.
func readCorpus(
	ctx context.Context,
	reader *vault.Reader,
	artifact schema.ArtifactPolicy,
	privacy schema.PrivacyPolicy,
	chunkTokenCap int,
) (Corpus, error) {
	if reader == nil {
		return Corpus{}, fmt.Errorf("%w: vault reader is nil", ErrCorpusRead)
	}
	collector := corpusCollector{
		reader:        reader,
		artifact:      artifact,
		privacy:       privacy,
		chunkTokenCap: chunkTokenCap,
	}
	if err := collector.validateAuthority(ctx); err != nil {
		return Corpus{}, err
	}
	chunks, err := collector.readChunks(ctx)
	if err != nil {
		return Corpus{}, err
	}
	return collector.finish(ctx, chunks)
}

type corpusCollector struct {
	reader        *vault.Reader
	artifact      schema.ArtifactPolicy
	privacy       schema.PrivacyPolicy
	chunkTokenCap int
}

func (c *corpusCollector) validateAuthority(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !c.artifact.Available() {
		return ErrArtifactPolicyUnavailable
	}
	if !c.privacy.Available() {
		return ErrPrivacyPolicyUnavailable
	}
	if !c.artifact.ValidateSource().Available() || !c.privacy.ValidateSource().Available() {
		return ErrPolicySourceChanged
	}
	return nil
}

func (c *corpusCollector) readChunks(ctx context.Context) ([]CorpusChunk, error) {
	entries, err := c.reader.Entries(ctx)
	if err != nil {
		if contextErr := ctx.Err(); contextErr != nil {
			return nil, contextErr
		}
		return nil, fmt.Errorf("%w: list vault", ErrCorpusRead)
	}
	chunks := make([]CorpusChunk, 0, len(entries))
	for _, entry := range entries {
		if contextErr := ctx.Err(); contextErr != nil {
			return nil, contextErr
		}
		relPath := entry.Path()
		if !vault.IsMarkdown(relPath) || c.artifact.IsNonInstance(relPath) || !c.privacy.EgressAllowed(relPath) {
			continue
		}
		collected, readErr := c.readNoteChunks(ctx, entry)
		if readErr != nil {
			return nil, readErr
		}
		chunks = append(chunks, collected...)
	}
	return chunks, nil
}

func (c *corpusCollector) readNoteChunks(ctx context.Context, entry vault.Entry) ([]CorpusChunk, error) {
	relPath := entry.Path()
	if !validRelPath(relPath) {
		return nil, fmt.Errorf("%w: invalid eligible path", ErrCorpusRead)
	}
	data, err := c.reader.ReadFile(ctx, entry)
	if err != nil {
		return nil, fmt.Errorf("%w: read eligible note", ErrCorpusRead)
	}
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("%w: eligible note is not UTF-8", ErrCorpusRead)
	}
	note := vault.Parse(relPath, data)
	collection, err := collectCorpusChunks([]SourceNote{{
		RelPath:  relPath,
		Title:    note.Title(),
		Status:   note.Status(),
		Body:     note.Body,
		NoteHash: sha256.Sum256(data),
		Source:   entry,
	}}, c.artifact, c.privacy, c.chunkTokenCap)
	if err != nil {
		return nil, err
	}
	if len(collection.Failures) != 0 {
		return nil, ErrCorpusChunking
	}
	return collection.Chunks, nil
}

func (c *corpusCollector) finish(ctx context.Context, chunks []CorpusChunk) (Corpus, error) {
	if err := ctx.Err(); err != nil {
		return Corpus{}, err
	}
	manifest := make([]ChunkVector, 0, len(chunks))
	proxyTokens := 0
	maxInt := int(^uint(0) >> 1)
	for position := range chunks {
		if err := ctx.Err(); err != nil {
			return Corpus{}, err
		}
		chunk := &chunks[position]
		if chunk.ProxyTokens < 0 || chunk.ProxyTokens > maxInt-proxyTokens {
			return Corpus{}, fmt.Errorf("%w: proxy-token total overflow", ErrCorpusChunking)
		}
		proxyTokens += chunk.ProxyTokens
		row := ChunkVector{
			RelPath:       chunk.RelPath,
			NoteHash:      chunk.NoteHash,
			Ordinal:       chunk.Ordinal,
			SubmittedHash: chunk.SubmittedHash,
		}
		manifest = append(manifest, bindChunkVector(&row))
	}
	fingerprint, err := CorpusFingerprint(manifest)
	if err != nil {
		return Corpus{}, err
	}
	if !c.artifact.ValidateSource().Available() || !c.privacy.ValidateSource().Available() {
		return Corpus{}, ErrPolicySourceChanged
	}
	return Corpus{
		Chunks:      chunks,
		Fingerprint: fingerprint,
		ProxyTokens: proxyTokens,
	}, nil
}
