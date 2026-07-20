package semantic

import (
	"crypto/sha256"
	"errors"
	"path/filepath"
	"testing"
)

func TestIdentityDerivesProtocolChunkerAndVectorFormat(t *testing.T) {
	t.Parallel()

	root := filepath.Join(string(filepath.Separator), "vault")
	policy := sha256.Sum256([]byte("policy"))
	base, err := newGenerationIdentity(IdentityConfig{
		Dimension:               1536,
		ChunkTokenCap:           7372,
		VaultRoot:               root,
		CorpusPolicyFingerprint: policy,
	})
	if err != nil {
		t.Fatal(err)
	}
	again, err := newGenerationIdentity(IdentityConfig{
		Dimension:               1536,
		ChunkTokenCap:           7372,
		VaultRoot:               root,
		CorpusPolicyFingerprint: policy,
	})
	if err != nil {
		t.Fatal(err)
	}
	if base != again {
		t.Fatalf("same current identity inputs produced different values: %#v != %#v", base, again)
	}
	if base.model != geminiEmbeddingModel || base.protocolEpoch != currentProtocolEpoch() ||
		base.vectorFormatVersion != vectorFormatVersion {
		t.Fatalf("identity protocol ownership = %#v, want current provider/vector descriptors", base)
	}

	differentCap, err := newGenerationIdentity(IdentityConfig{
		Dimension:               1536,
		ChunkTokenCap:           7000,
		VaultRoot:               root,
		CorpusPolicyFingerprint: policy,
	})
	if err != nil {
		t.Fatal(err)
	}
	if differentCap.chunkerEpoch == base.chunkerEpoch || differentCap.protocolEpoch != base.protocolEpoch {
		t.Fatal("chunk cap must change only the chunker epoch")
	}
}

func TestIdentityRejectsUnruledOrUnboundInputs(t *testing.T) {
	t.Parallel()

	valid := IdentityConfig{
		Dimension:               1536,
		ChunkTokenCap:           7372,
		VaultRoot:               filepath.Join(string(filepath.Separator), "vault"),
		CorpusPolicyFingerprint: sha256.Sum256([]byte("policy")),
	}
	tests := map[string]func(*IdentityConfig){
		"dimension":     func(config *IdentityConfig) { config.Dimension = 0 },
		"chunk cap":     func(config *IdentityConfig) { config.ChunkTokenCap = 0 },
		"relative root": func(config *IdentityConfig) { config.VaultRoot = "vault" },
		"invalid utf8 root": func(config *IdentityConfig) {
			config.VaultRoot = filepath.Join(string(filepath.Separator), string([]byte{0xff}))
		},
		"policy": func(config *IdentityConfig) { config.CorpusPolicyFingerprint = [sha256.Size]byte{} },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			config := valid
			mutate(&config)
			if _, err := newGenerationIdentity(config); !errors.Is(err, ErrInvalidIdentity) {
				t.Fatalf("NewIdentity() error = %v, want ErrInvalidIdentity", err)
			}
		})
	}
}

func TestEvalIdentitiesProjectRawComponentsAtTheCorrectBoundary(t *testing.T) {
	t.Parallel()

	base := IdentityConfig{
		Dimension:               1536,
		ChunkTokenCap:           7372,
		VaultRoot:               filepath.Join(string(filepath.Separator), "vault-a"),
		CorpusPolicyFingerprint: sha256.Sum256([]byte("policy-a")),
	}
	full, err := FullCacheIdentity(base)
	if err != nil {
		t.Fatal(err)
	}
	query, err := QueryVectorCompatibilityIdentity(base.Dimension)
	if err != nil {
		t.Fatal(err)
	}

	corpusChange := base
	corpusChange.ChunkTokenCap++
	corpusChange.VaultRoot = filepath.Join(string(filepath.Separator), "vault-b")
	corpusChange.CorpusPolicyFingerprint = sha256.Sum256([]byte("policy-b"))
	changedFull, err := FullCacheIdentity(corpusChange)
	if err != nil {
		t.Fatal(err)
	}
	unchangedQuery, err := QueryVectorCompatibilityIdentity(corpusChange.Dimension)
	if err != nil {
		t.Fatal(err)
	}
	if changedFull == full {
		t.Fatal("corpus-specific raw components did not change the full cache identity")
	}
	if unchangedQuery != query {
		t.Fatal("corpus-specific components leaked into query-vector compatibility")
	}

	dimensionChange := base
	dimensionChange.Dimension = 3072
	dimensionFull, err := FullCacheIdentity(dimensionChange)
	if err != nil {
		t.Fatal(err)
	}
	dimensionQuery, err := QueryVectorCompatibilityIdentity(dimensionChange.Dimension)
	if err != nil {
		t.Fatal(err)
	}
	if dimensionFull == full || dimensionQuery == query {
		t.Fatal("dimension must change both full and query-vector identities")
	}
}
