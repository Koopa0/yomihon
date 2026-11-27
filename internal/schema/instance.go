package schema

import (
	"fmt"
	"slices"
	"sync/atomic"
)

const staleArtifactDiagnostic = "vault artifact policy source changed after startup; instance projections disabled until restart"

// ArtifactPolicy identifies vault directories whose files are readable
// artifacts but not governed note instances. The zero value is unclaimed, so
// the excluded set is empty and every readable file is an ordinary instance.
type ArtifactPolicy struct {
	state *artifactPolicyState
}

type artifactPolicyState struct {
	nonInstanceDirs []string
	source          policySource
	claim           Claim
	frozen          bool
	stale           atomic.Bool
}

// Claim reports how far the artifact declaration got. A policy whose source
// bytes changed after startup can no longer be honoured, whatever it said.
func (p ArtifactPolicy) Claim() Claim {
	if p.state == nil {
		return Claim{}
	}
	if p.state.stale.Load() {
		return changedClaim(staleArtifactDiagnostic)
	}
	return p.state.claim
}

// Available reports whether the contract declared a valid artifact policy.
func (p ArtifactPolicy) Available() bool {
	return p.Claim().held()
}

// Trustworthy reports whether the excluded set may be projected over: true when
// it was read cleanly or never declared, false when a declaration was made and
// could not be honoured.
func (p ArtifactPolicy) Trustworthy() bool {
	return p.Claim().Trustworthy()
}

// Diagnostic explains why the artifact policy could not be honoured. It is
// empty when the policy was read cleanly and empty when nothing declared one.
func (p ArtifactPolicy) Diagnostic() string {
	return p.Claim().Diagnostic()
}

// IsNonInstance reports whether rel is equal to or below a declared artifact
// directory, read the way every declared directory list is read. An unclaimed
// policy excludes nothing; an unresolved one also answers false, so callers
// gate on Trustworthy first.
func (p ArtifactPolicy) IsNonInstance(rel string) bool {
	if !p.Trustworthy() || p.state == nil {
		return false
	}
	in, _ := inDeclaredDir(rel, p.state.nonInstanceDirs)
	return in
}

// ValidateSource returns p only while the contract source it was derived from
// is unchanged. Copies of one Contract share a one-way stale latch, so once any
// consumer observes drift every instance projection stays unavailable. A source
// that cannot be read at all closes this caller's projection and nothing more:
// it is no evidence the declaration was withdrawn, and the next caller reads
// the file again.
func (p ArtifactPolicy) ValidateSource() ArtifactPolicy {
	if p.state == nil || !p.state.claim.held() || p.state.frozen || p.state.stale.Load() {
		return p
	}
	same, err := p.state.source.reread()
	if err != nil {
		return ArtifactPolicy{state: &artifactPolicyState{claim: unreadableClaim(err)}}
	}
	if !same {
		p.state.stale.Store(true)
	}
	return p
}

// Capture validates the source once and returns an immutable policy for one
// request, so the classification cannot change underneath a response already
// using it.
func (p ArtifactPolicy) Capture() ArtifactPolicy {
	p = p.ValidateSource()
	if !p.Available() || p.state.frozen {
		return p
	}
	return ArtifactPolicy{state: &artifactPolicyState{
		nonInstanceDirs: slices.Clone(p.state.nonInstanceDirs),
		claim:           heldClaim(),
		frozen:          true,
	}}
}

// noRulesArtifactPolicy is the artifact policy of a folder with no rules file:
// it declares no template directory, so every readable note is an ordinary
// instance. It is frozen because no file stands behind it to be re-read.
func noRulesArtifactPolicy() ArtifactPolicy {
	return ArtifactPolicy{state: &artifactPolicyState{claim: heldClaim(), frozen: true}}
}

func deriveArtifactPolicy(templates []string, source policySource) ArtifactPolicy {
	dirs := make([]string, 0, len(templates))
	for _, original := range templates {
		normalized, ok := normalizeDeclaredDir(original)
		if !ok {
			return invalidArtifactPolicy(original)
		}
		dirs = append(dirs, normalized)
	}
	return ArtifactPolicy{state: &artifactPolicyState{
		nonInstanceDirs: slices.Clone(dirs),
		source:          source,
		claim:           heldClaim(),
	}}
}

func invalidArtifactPolicy(value string) ArtifactPolicy {
	return ArtifactPolicy{state: &artifactPolicyState{
		claim: Rejected(fmt.Sprintf("invalid artifact policy: templates contains %q", value)),
	}}
}
