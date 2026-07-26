package schema

import (
	"fmt"
	"path"
	"slices"
	"strings"
	"sync/atomic"

	"github.com/koopa0/yomihon/internal/vault"
)

const (
	// undeclaredNavigationDiagnostic is said only about a contract that exists
	// and left the section out. A folder carrying no contract at all declares
	// nothing, which is not a fault and is reported as nothing.
	undeclaredNavigationDiagnostic = "contract declares no navigation roles; Paths and Maps disabled until it does"
	undeclaredArtifactDiagnostic   = "contract declares no artifact policy; instance projections disabled until it does"
	staleArtifactDiagnostic        = "vault artifact policy source changed after startup; instance projections disabled until restart"
)

type navigationSection struct {
	PathTypes []string `toml:"path_types"`
	MapTypes  []string `toml:"map_types"`
}

type artifactSection struct {
	NonInstanceDirs []string `toml:"non_instance_dirs"`
}

// NavigationRoles classifies note types used for ordered study paths and
// general maps. Its derived membership sets cannot be changed after loading.
// The zero value is unclaimed: no contract ever named a path or map type, so
// both sets are empty and no note is either.
type NavigationRoles struct {
	pathTypes map[string]struct{}
	mapTypes  map[string]struct{}
	claim     Claim
}

// Claim reports how far the navigation declaration got.
func (r NavigationRoles) Claim() Claim {
	return r.claim
}

// Available reports whether the contract declared a valid navigation role set.
func (r NavigationRoles) Available() bool {
	return r.claim.Held()
}

// Trustworthy reports whether the role sets may be projected over: true when
// they were read cleanly and true when nothing ever declared them, false only
// when a declaration was made and could not be honoured.
func (r NavigationRoles) Trustworthy() bool {
	return r.claim.Trustworthy()
}

// Diagnostic explains why navigation roles could not be honoured. It is empty
// when they were read cleanly and empty when nothing declared them.
func (r NavigationRoles) Diagnostic() string {
	return r.claim.Diagnostic()
}

// IsPathType reports whether noteType is declared as an ordered study path.
func (r NavigationRoles) IsPathType(noteType string) bool {
	if !r.Trustworthy() {
		return false
	}
	_, ok := r.pathTypes[noteType]
	return ok
}

// IsMapType reports whether noteType is declared as a general map.
func (r NavigationRoles) IsMapType(noteType string) bool {
	if !r.Trustworthy() {
		return false
	}
	_, ok := r.mapTypes[noteType]
	return ok
}

// ArtifactPolicy identifies vault directories whose files are readable
// artifacts but are not governed note instances. The zero value is unclaimed:
// no contract ever excluded a directory, so the excluded set is empty and every
// readable file is an ordinary instance.
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
// bytes changed after startup is treated as an assertion that can no longer be
// honoured, whatever it said when it was read.
func (p ArtifactPolicy) Claim() Claim {
	if p.state == nil {
		return Claim{}
	}
	if p.state.stale.Load() {
		return Rejected(staleArtifactDiagnostic)
	}
	return p.state.claim
}

// Available reports whether the contract declared a valid artifact policy.
func (p ArtifactPolicy) Available() bool {
	return p.Claim().Held()
}

// Trustworthy reports whether the excluded set may be projected over: true when
// it was read cleanly and true when nothing ever declared one, false only when
// a declaration was made and could not be honoured.
func (p ArtifactPolicy) Trustworthy() bool {
	return p.Claim().Trustworthy()
}

// Diagnostic explains why the artifact policy could not be honoured. It is
// empty when the policy was read cleanly and empty when nothing declared one.
func (p ArtifactPolicy) Diagnostic() string {
	return p.Claim().Diagnostic()
}

// IsNonInstance reports whether rel is equal to or below a declared artifact
// directory. Component boundaries prevent a sibling with the same prefix from
// matching. An unclaimed policy excludes nothing, which is the true answer for
// a vault that never declared an exclusion; an unresolved one also answers
// false, so callers that act on the classification gate on Trustworthy first.
func (p ArtifactPolicy) IsNonInstance(rel string) bool {
	if !p.Trustworthy() || p.state == nil {
		return false
	}
	rel = vault.NormalizeNFC(rel)
	for _, dir := range p.state.nonInstanceDirs {
		if rel == dir || strings.HasPrefix(rel, dir+"/") {
			return true
		}
	}
	return false
}

// ValidateSource returns p only while the exact contract source from which it
// was derived is unchanged. Every copy derived from one Contract shares the
// same one-way stale latch, so once any consumer observes drift, all instance
// projections remain unavailable until a freshly loaded Contract replaces it.
func (p ArtifactPolicy) ValidateSource() ArtifactPolicy {
	if p.state == nil || !p.state.claim.Held() || p.state.frozen || p.state.stale.Load() {
		return p
	}
	if !p.state.source.unchanged() {
		p.state.stale.Store(true)
	}
	return p
}

// Capture validates the source once and returns an immutable point-in-time
// policy for one request. Source-bound policies keep their shared one-way stale
// latch; a successful capture owns a copy of the classification and does not
// change underneath the response that is already using it.
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

func deriveNavigationRoles(
	section *navigationSection,
	enumTypes []string,
	pathTypesDefined bool,
	mapTypesDefined bool,
) NavigationRoles {
	// A contract that exists and omits the section left an input out: it
	// claimed authority over this vault and then did not say which types order
	// a curriculum. That is the operator's news, and it is why this answer is
	// produced here rather than as a fallback on the zero value — the zero
	// value belongs to a folder that carries no contract, which asserted
	// nothing and has nothing to report.
	if section == nil {
		return NavigationRoles{claim: Rejected(undeclaredNavigationDiagnostic)}
	}
	switch {
	case !pathTypesDefined && !mapTypesDefined:
		return invalidNavigationRoles(`missing required keys "path_types", "map_types"`)
	case !pathTypesDefined:
		return invalidNavigationRoles(`missing required key "path_types"`)
	case !mapTypesDefined:
		return invalidNavigationRoles(`missing required key "map_types"`)
	}
	known := make(map[string]struct{}, len(enumTypes))
	for _, noteType := range enumTypes {
		known[noteType] = struct{}{}
	}
	paths := make(map[string]struct{}, len(section.PathTypes))
	for _, noteType := range section.PathTypes {
		if _, ok := known[noteType]; !ok {
			return invalidNavigationRoles("path type %q is not declared in enums.type", noteType)
		}
		if _, duplicate := paths[noteType]; duplicate {
			return invalidNavigationRoles("path type %q is declared more than once", noteType)
		}
		paths[noteType] = struct{}{}
	}
	maps := make(map[string]struct{}, len(section.MapTypes))
	for _, noteType := range section.MapTypes {
		if _, ok := known[noteType]; !ok {
			return invalidNavigationRoles("map type %q is not declared in enums.type", noteType)
		}
		if _, duplicate := maps[noteType]; duplicate {
			return invalidNavigationRoles("map type %q is declared more than once", noteType)
		}
		if _, pathType := paths[noteType]; pathType {
			return invalidNavigationRoles("type %q is declared as both a path and a map", noteType)
		}
		maps[noteType] = struct{}{}
	}
	return NavigationRoles{pathTypes: paths, mapTypes: maps, claim: heldClaim()}
}

func invalidNavigationRoles(format string, args ...any) NavigationRoles {
	return NavigationRoles{claim: Rejected(fmt.Sprintf("invalid navigation roles: "+format+"; Paths and Maps disabled", args...))}
}

func deriveArtifactPolicy(section *artifactSection, nonInstanceDirsDefined bool, source policySource) ArtifactPolicy {
	// As with navigation roles: an existing contract that omits the section
	// left an input out, while a folder with no contract at all yields the
	// silent zero value.
	if section == nil {
		return ArtifactPolicy{state: &artifactPolicyState{claim: Rejected(undeclaredArtifactDiagnostic)}}
	}
	if !nonInstanceDirsDefined {
		return ArtifactPolicy{state: &artifactPolicyState{claim: Rejected(`invalid artifact policy: missing required key "non_instance_dirs"`)}}
	}
	dirs := make([]string, 0, len(section.NonInstanceDirs))
	for _, original := range section.NonInstanceDirs {
		if original == "" || strings.Contains(original, `\`) {
			return invalidArtifactPolicy(original)
		}
		normalized := path.Clean(vault.NormalizeNFC(original))
		if normalized == "." || normalized == ".." || strings.HasPrefix(normalized, "../") || path.IsAbs(normalized) {
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
		claim: Rejected(fmt.Sprintf("invalid artifact policy: non_instance_dirs contains %q", value)),
	}}
}
