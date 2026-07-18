package semantic

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

// Egress errors name the local authority or source condition that prevented a
// document or query from reaching the embedding provider.
var (
	ErrArtifactPolicyUnavailable = errors.New("artifact policy unavailable for semantic corpus")
	ErrPrivacyPolicyUnavailable  = errors.New("privacy policy unavailable for semantic corpus")
	ErrChunkEgressDenied         = errors.New("chunk embedding denied at final egress check")
	ErrSourceNoteChanged         = errors.New("source note changed before final egress check")
	ErrSourceNoteUnavailable     = errors.New("source note unavailable at final egress check")
	ErrQueryNotApplicable        = errors.New("query embedding is not applicable")
)

// SourceNote is the pure collector's local input. NoteHash binds the
// complete current note bytes, while Body has already had frontmatter removed.
type SourceNote struct {
	RelPath  string
	Title    string
	Status   string
	Body     string
	NoteHash [sha256.Size]byte
	Source   vault.Entry
}

// CorpusChunk is one exact provider submission and the identity needed
// to re-authorize it immediately before network egress and publication.
type CorpusChunk struct {
	RelPath       string
	Title         string
	Status        string
	NoteHash      [sha256.Size]byte
	Ordinal       uint32
	Submitted     []byte
	SubmittedHash [sha256.Size]byte
	Headings      []string
	Body          string
	ProxyTokens   int
	source        vault.Entry
}

// ChunkIdentity is the content-free minimum needed to re-authorize one
// chunk immediately before egress. The callback never receives submitted
// note bytes merely to answer an eligibility question.
type ChunkIdentity struct {
	RelPath       string
	NoteHash      [sha256.Size]byte
	Ordinal       uint32
	SubmittedHash [sha256.Size]byte
	source        vault.Entry
}

// EgressIdentity returns d's content-free final-egress identity.
func (d *CorpusChunk) EgressIdentity() ChunkIdentity {
	return ChunkIdentity{
		RelPath:       d.RelPath,
		NoteHash:      d.NoteHash,
		Ordinal:       d.Ordinal,
		SubmittedHash: d.SubmittedHash,
		source:        d.source,
	}
}

// chunkAuthorizer rechecks current local eligibility at the final network
// choke point. A typed error preserves policy drift, cancellation, and local
// read failures instead of collapsing every denial into one boolean.
type chunkAuthorizer func(context.Context, *ChunkIdentity) error

type chunkAuthority struct {
	reader        *vault.Reader
	corpusPolicy  [sha256.Size]byte
	policySource  [sha256.Size]byte
	chunkTokenCap int
}

// newChunkAuthorizer returns the final egress capability for one CLI action.
// NewIndexer alone binds it to the production transport.
func newChunkAuthorizer(
	reader *vault.Reader,
	artifact schema.ArtifactPolicy,
	privacy schema.PrivacyPolicy,
	chunkTokenCap int,
) (chunkAuthorizer, error) {
	authority, err := newChunkAuthority(reader, artifact, privacy, chunkTokenCap)
	if err != nil {
		return nil, err
	}
	authorize := func(ctx context.Context, identity *ChunkIdentity) error {
		return authorizeChunk(ctx, &authority, artifact, privacy, identity)
	}
	return authorize, nil
}

func newChunkAuthority(
	reader *vault.Reader,
	artifact schema.ArtifactPolicy,
	privacy schema.PrivacyPolicy,
	chunkTokenCap int,
) (chunkAuthority, error) {
	if !artifact.Available() {
		return chunkAuthority{}, ErrArtifactPolicyUnavailable
	}
	if !privacy.Available() {
		return chunkAuthority{}, ErrPrivacyPolicyUnavailable
	}
	if reader == nil || reader.Name() == "" || chunkTokenCap <= 0 {
		return chunkAuthority{}, ErrChunkEgressDenied
	}
	corpusPolicy, ok := schema.CorpusPolicyFingerprint(artifact, privacy)
	if !ok {
		return chunkAuthority{}, ErrPolicySourceChanged
	}
	policySource, ok := schema.PolicySourceFingerprint(reader, artifact, privacy)
	if !ok {
		return chunkAuthority{}, ErrPolicySourceChanged
	}
	return chunkAuthority{reader: reader, corpusPolicy: corpusPolicy, policySource: policySource, chunkTokenCap: chunkTokenCap}, nil
}

func authorizeChunk(
	ctx context.Context,
	authority *chunkAuthority,
	artifact schema.ArtifactPolicy,
	privacy schema.PrivacyPolicy,
	identity *ChunkIdentity,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if authority == nil || identity == nil {
		return ErrChunkEgressDenied
	}
	if err := authorizeChunkPath(artifact, privacy, identity.RelPath); err != nil {
		return err
	}
	if identity.source.Path() != identity.RelPath {
		return ErrChunkEgressDenied
	}
	data, err := authority.reader.ReadFile(ctx, identity.source)
	if errors.Is(err, vault.ErrSourceChanged) {
		return ErrSourceNoteChanged
	}
	if err != nil || !utf8.Valid(data) {
		return ErrSourceNoteUnavailable
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if sha256.Sum256(data) != identity.NoteHash {
		return ErrSourceNoteChanged
	}
	return matchAuthorizedChunk(data, authority.chunkTokenCap, identity)
}

func authorizeChunkPath(
	artifact schema.ArtifactPolicy,
	privacy schema.PrivacyPolicy,
	relPath string,
) error {
	if !artifact.ValidateSource().Available() || !privacy.ValidateSource().Available() {
		return ErrPolicySourceChanged
	}
	if !validRelPath(relPath) || artifact.IsNonInstance(relPath) || !privacy.EgressAllowed(relPath) {
		return ErrChunkEgressDenied
	}
	return nil
}

func matchAuthorizedChunk(data []byte, chunkTokenCap int, identity *ChunkIdentity) error {
	if identity == nil {
		return ErrChunkEgressDenied
	}
	note := vault.Parse(identity.RelPath, data)
	chunked := ChunkNote(note.Title(), note.Body, chunkTokenCap)
	if len(chunked.Failures) != 0 {
		return ErrSourceNoteChanged
	}
	for position := range chunked.Chunks {
		chunk := &chunked.Chunks[position]
		if chunk.Ordinal < 0 || uint32(chunk.Ordinal) != identity.Ordinal { // #nosec G115 -- non-negative chunk ordinal is bounded by the in-memory slice length
			continue
		}
		if sha256.Sum256([]byte(chunk.Submitted)) == identity.SubmittedHash {
			return nil
		}
	}
	return ErrSourceNoteChanged
}

// collectionFailure is content-free evidence that an eligible source could
// not be represented under the chunk cap. Any failure leaves the epoch partial.
type collectionFailure struct {
	RelPath string
	Section int
	Reason  ChunkFailureKind
}

// chunkCollection is the deterministic result of applying both schema
// capabilities and the chunker. Excluded source bytes are never retained.
type chunkCollection struct {
	Chunks   []CorpusChunk
	Failures []collectionFailure
}

// EmbeddingResult is the provider response data semantic retrieval may retain.
// It deliberately excludes raw response bytes and provider diagnostics.
type EmbeddingResult struct {
	Vector []float32
}

// collectCorpusChunks is pure: it performs no filesystem or network I/O.
// Capability availability is checked before any source is inspected, and both
// instance and privacy authority must positively admit a path.
func collectCorpusChunks(
	sources []SourceNote,
	artifact schema.ArtifactPolicy,
	privacy schema.PrivacyPolicy,
	tokenCap int,
) (chunkCollection, error) {
	if !privacy.Available() {
		return chunkCollection{}, ErrPrivacyPolicyUnavailable
	}
	if !artifact.Available() {
		return chunkCollection{}, ErrArtifactPolicyUnavailable
	}

	ordered := slices.Clone(sources)
	slices.SortFunc(ordered, func(a, b SourceNote) int {
		return strings.Compare(a.RelPath, b.RelPath)
	})
	result := chunkCollection{}
	var previous string
	for i := range ordered {
		source := &ordered[i]
		if !validRelPath(source.RelPath) {
			return chunkCollection{}, fmt.Errorf("invalid semantic source path %q", source.RelPath)
		}
		if source.RelPath == previous {
			return chunkCollection{}, fmt.Errorf("duplicate semantic source path %q", source.RelPath)
		}
		previous = source.RelPath
		if artifact.IsNonInstance(source.RelPath) || !privacy.EgressAllowed(source.RelPath) {
			continue
		}
		chunked := ChunkNote(source.Title, source.Body, tokenCap)
		for _, failure := range chunked.Failures {
			result.Failures = append(result.Failures, collectionFailure{
				RelPath: source.RelPath,
				Section: failure.Section,
				Reason:  failure.Reason,
			})
		}
		for _, chunk := range chunked.Chunks {
			submitted := []byte(chunk.Submitted)
			result.Chunks = append(result.Chunks, CorpusChunk{
				RelPath:       source.RelPath,
				Title:         source.Title,
				Status:        source.Status,
				NoteHash:      source.NoteHash,
				Ordinal:       uint32(chunk.Ordinal), // #nosec G115 -- one note cannot hold 2^32 in-memory chunks
				Submitted:     slices.Clone(submitted),
				SubmittedHash: sha256.Sum256(submitted),
				Headings:      slices.Clone(chunk.Headings),
				Body:          chunk.Body,
				ProxyTokens:   chunk.ProxyTokens,
				source:        source.Source,
			})
		}
	}
	return result, nil
}
