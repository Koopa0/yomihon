package schema

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"hash"
	"os"
	"path/filepath"
	"slices"

	"github.com/koopa0/yomihon/internal/vault"
)

const corpusPolicyFingerprintVersion = "yomihon-semantic-corpus-policy-v1"

// policySource binds a startup-derived capability to the exact contract file
// it came from. Keeping the source path with the digest avoids reconstructing
// provenance from a later consumer's vault root.
type policySource struct {
	path   string
	digest [sha256.Size]byte
	pinned *pinnedPolicySource
}

type pinnedPolicySource struct {
	reader *vault.Reader
	entry  vault.Entry
}

func (s policySource) unchanged() bool {
	if s.path == "" {
		return false
	}
	var (
		data []byte
		err  error
	)
	if s.pinned != nil {
		data, err = s.pinned.reader.ReadFile(context.Background(), s.pinned.entry)
	} else {
		data, err = os.ReadFile(s.path) // #nosec G304 -- LoadFile recorded the operator-selected contract path
	}
	return err == nil && sha256.Sum256(data) == s.digest
}

// CorpusPolicyFingerprint hashes only the normalized capability semantics
// that decide membership in the embedding corpus. Contract byte identity is a
// separate freshness gate: irrelevant TOML edits do not create a new corpus,
// while any artifact/privacy membership change does.
func CorpusPolicyFingerprint(artifact ArtifactPolicy, privacy PrivacyPolicy) ([sha256.Size]byte, bool) {
	if !artifact.Available() || !privacy.Available() {
		return [sha256.Size]byte{}, false
	}
	h := sha256.New()
	writeFingerprintString(h, corpusPolicyFingerprintVersion)
	writeFingerprintSet(h, "non-instance", artifact.state.nonInstanceDirs)
	writeFingerprintSet(h, "never-egress", privacy.state.neverEgressDirs)
	var result [sha256.Size]byte
	copy(result[:], h.Sum(nil))
	return result, true
}

// PolicySourceFingerprint returns the exact contract-byte digest shared by the
// artifact and privacy capabilities only when both were loaded through the
// exact selected vault Reader. Path-loaded capabilities are deliberately
// insufficient: their freshness could follow a replacement at the same root
// name while corpus bytes remain bound to the Reader's original root. It is
// deliberately separate from
// CorpusPolicyFingerprint: irrelevant contract edits keep active corpus
// compatibility, while an incomplete staging generation must not resume under
// authority derived from different source bytes or another vault's contract.
func PolicySourceFingerprint(reader *vault.Reader, artifact ArtifactPolicy, privacy PrivacyPolicy) ([sha256.Size]byte, bool) {
	if reader == nil || !artifact.Available() || !privacy.Available() || artifact.state.source.path == "" ||
		artifact.state.source != privacy.state.source {
		return [sha256.Size]byte{}, false
	}
	source := artifact.state.source
	if source.pinned == nil || source.pinned.reader != reader {
		return [sha256.Size]byte{}, false
	}
	expected := filepath.Clean(filepath.Join(reader.Name(), filepath.FromSlash(ContractRelPath)))
	if source.path != expected {
		return [sha256.Size]byte{}, false
	}
	return source.digest, true
}

func writeFingerprintSet(h hash.Hash, label string, values []string) {
	writeFingerprintString(h, label)
	canonical := slices.Clone(values)
	slices.Sort(canonical)
	canonical = slices.Compact(canonical)
	writeFingerprintUint64(h, uint64(len(canonical))) // #nosec G115 -- slice length is non-negative and encoded as uint64
	for _, value := range canonical {
		writeFingerprintString(h, value)
	}
}

func writeFingerprintString(h hash.Hash, value string) {
	writeFingerprintUint64(h, uint64(len(value))) // #nosec G115 -- string length is non-negative and encoded as uint64
	_, _ = h.Write([]byte(value))
}

func writeFingerprintUint64(h hash.Hash, value uint64) {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], value)
	_, _ = h.Write(encoded[:])
}
