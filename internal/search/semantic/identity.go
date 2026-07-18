package semantic

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	requestCardinalityDescriptor = "one-request-one-content-one-text-part"
	requestEncodingDescriptor    = "compact-json:auto-truncate=false:explicit-dimension"
	chunkSplitDescriptor         = "heading-bounded:block-line-rune"
	proxyCountingDescriptor      = "proxy:han-hiragana-katakana-u3000-303f-uff00-ffef-else-four-runes"
	dropRuleDescriptor           = "drop-empty-or-markup-only"
)

// IdentityConfig contains the corpus-specific inputs to the current semantic
// implementation. Provider, prompt, response, chunker, and vector-format
// facts are deliberately absent: semantic derives those from the code that
// owns them, so a caller cannot label new vectors with a hand-written epoch.
type IdentityConfig struct {
	Dimension               int
	ChunkTokenCap           int
	VaultRoot               string
	CorpusPolicyFingerprint [sha256.Size]byte
}

// generationIdentity names one numerically compatible corpus generation. Its
// fields are immutable outside package semantic. Stored generations are
// decoded internally so an incompatible identity can be classified without
// letting a producer forge a current one.
type generationIdentity struct {
	model                   string
	dimension               int
	protocolEpoch           [sha256.Size]byte
	chunkerEpoch            [sha256.Size]byte
	vectorFormatVersion     int
	vaultRoot               string
	corpusPolicyFingerprint [sha256.Size]byte
}

// newGenerationIdentity derives the current provider/chunker epochs and validates the
// corpus-specific inputs supplied by the CLI action.
func newGenerationIdentity(config IdentityConfig) (generationIdentity, error) {
	identity := generationIdentity{
		model:                   geminiEmbeddingModel,
		dimension:               config.Dimension,
		protocolEpoch:           currentProtocolEpoch(),
		chunkerEpoch:            currentChunkerEpoch(config.ChunkTokenCap),
		vectorFormatVersion:     vectorFormatVersion,
		vaultRoot:               config.VaultRoot,
		corpusPolicyFingerprint: config.CorpusPolicyFingerprint,
	}
	if config.Dimension < minEmbeddingDimension || config.Dimension > maxEmbeddingDimension {
		return generationIdentity{}, fmt.Errorf("%w: dimension is outside the model range", ErrInvalidIdentity)
	}
	if config.ChunkTokenCap <= 0 {
		return generationIdentity{}, fmt.Errorf("%w: chunk token cap", ErrInvalidIdentity)
	}
	if err := validateIdentity(&identity); err != nil {
		return generationIdentity{}, err
	}
	return identity, nil
}

// Model returns the provider model whose vector space owns the generation.
func (i *generationIdentity) Model() string { return i.model }

// Dimension returns the number of float32 components in every vector.
func (i *generationIdentity) Dimension() int { return i.dimension }

// ProtocolEpoch returns the request/response protocol identity.
func (i *generationIdentity) ProtocolEpoch() [sha256.Size]byte { return i.protocolEpoch }

// ChunkerEpoch returns the preprocessing/chunk-cap identity.
func (i *generationIdentity) ChunkerEpoch() [sha256.Size]byte { return i.chunkerEpoch }

// VectorFormatVersion returns the durable vector representation version.
func (i *generationIdentity) VectorFormatVersion() int { return i.vectorFormatVersion }

// VaultRoot returns the canonical absolute corpus root.
func (i *generationIdentity) VaultRoot() string { return i.vaultRoot }

// CorpusPolicyFingerprint returns the normalized eligibility-policy identity.
func (i *generationIdentity) CorpusPolicyFingerprint() [sha256.Size]byte {
	return i.corpusPolicyFingerprint
}

// QueryVectorCompatibilityIdentity hashes exactly the raw current components
// that determine whether a recorded query vector is numerically comparable to
// a current corpus vector. It deliberately excludes corpus-specific inputs.
func QueryVectorCompatibilityIdentity(dimension int) ([sha256.Size]byte, error) {
	if dimension < minEmbeddingDimension || dimension > maxEmbeddingDimension {
		return [sha256.Size]byte{}, fmt.Errorf("%w: dimension is outside the model range", ErrInvalidIdentity)
	}
	return hashDescriptor(
		"yomihon-query-vector-compatibility-v1",
		geminiEmbeddingModel,
		strconv.Itoa(dimension),
		queryPromptPrefix,
		responseHandlingVersion,
	), nil
}

// FullCacheIdentity hashes the raw current provider, prompt, response,
// preprocessing, vector-format, and corpus components. Unlike a projection
// from ProtocolEpoch, this remains able to select the query-only subset above.
func FullCacheIdentity(config IdentityConfig) ([sha256.Size]byte, error) {
	if _, err := newGenerationIdentity(config); err != nil {
		return [sha256.Size]byte{}, err
	}
	return hashDescriptor(
		"yomihon-full-cache-identity-v1",
		geminiEmbeddingModel,
		strconv.Itoa(config.Dimension),
		geminiEmbeddingEndpoint,
		queryPromptPrefix,
		documentPromptTitlePrefix,
		documentTitleFallback,
		documentHeadingSeparator,
		documentContinuationPrefix,
		documentContinuationJoin,
		documentPromptTextJoin,
		requestCardinalityDescriptor,
		requestEncodingDescriptor,
		responseHandlingVersion,
		chunkerDescriptorVersion,
		strconv.Itoa(config.ChunkTokenCap),
		chunkSplitDescriptor,
		proxyCountingDescriptor,
		dropRuleDescriptor,
		strconv.Itoa(vectorFormatVersion),
		config.VaultRoot,
		hex.EncodeToString(config.CorpusPolicyFingerprint[:]),
	), nil
}

func currentProtocolEpoch() [sha256.Size]byte {
	return hashDescriptor(
		protocolDescriptorVersion,
		geminiEmbeddingModel,
		geminiEmbeddingEndpoint,
		requestCardinalityDescriptor,
		queryPromptPrefix,
		documentPromptTitlePrefix,
		documentTitleFallback,
		documentHeadingSeparator,
		documentContinuationPrefix,
		documentContinuationJoin,
		documentPromptTextJoin,
		requestEncodingDescriptor,
		responseHandlingVersion,
	)
}

func currentChunkerEpoch(chunkTokenCap int) [sha256.Size]byte {
	return hashDescriptor(
		chunkerDescriptorVersion,
		strconv.Itoa(chunkTokenCap),
		documentPromptTitlePrefix,
		documentTitleFallback,
		documentHeadingSeparator,
		documentContinuationPrefix,
		documentContinuationJoin,
		documentPromptTextJoin,
		chunkSplitDescriptor,
		proxyCountingDescriptor,
		dropRuleDescriptor,
	)
}

func hashDescriptor(parts ...string) [sha256.Size]byte {
	h := sha256.New()
	var size [8]byte
	for _, part := range parts {
		binary.BigEndian.PutUint64(size[:], uint64(len(part))) // #nosec G115 -- string length is non-negative
		_, _ = h.Write(size[:])
		_, _ = h.Write([]byte(part))
	}
	var result [sha256.Size]byte
	copy(result[:], h.Sum(nil))
	return result
}

func validateIdentity(identity *generationIdentity) error {
	if identity == nil {
		return fmt.Errorf("%w: identity is nil", ErrInvalidIdentity)
	}
	if !validIdentityModel(identity.model) {
		return fmt.Errorf("%w: model", ErrInvalidIdentity)
	}
	maxInt := int(^uint(0) >> 1)
	if identity.dimension <= 0 || identity.dimension > maxInt/4 {
		return fmt.Errorf("%w: dimension", ErrInvalidIdentity)
	}
	if identity.vectorFormatVersion <= 0 {
		return fmt.Errorf("%w: vector format", ErrInvalidIdentity)
	}
	if identity.protocolEpoch == ([sha256.Size]byte{}) || identity.chunkerEpoch == ([sha256.Size]byte{}) ||
		identity.corpusPolicyFingerprint == ([sha256.Size]byte{}) {
		return fmt.Errorf("%w: identity digest", ErrInvalidIdentity)
	}
	if !validIdentityVaultRoot(identity.vaultRoot) {
		return fmt.Errorf("%w: vault root", ErrInvalidIdentity)
	}
	return nil
}

func validIdentityModel(model string) bool {
	trimmed := strings.TrimSpace(model)
	return utf8.ValidString(model) && trimmed != "" && model == trimmed && !containsControl(model)
}

func validIdentityVaultRoot(root string) bool {
	return utf8.ValidString(root) && filepath.IsAbs(root) && filepath.Clean(root) == root && !containsControl(root)
}
