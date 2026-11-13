package schema

import (
	"fmt"
	"maps"
	"slices"
	"sync/atomic"

	"github.com/koopa0/yomihon/internal/vault"
)

const staleArtifactDiagnostic = "vault artifact policy source changed after startup; instance projections disabled until restart"

// NavigationRoles classifies note types used for ordered study paths and
// general maps; its membership sets cannot be changed after loading. The zero
// value is unclaimed, so both sets are empty and no note is either.
type NavigationRoles struct {
	sets  *roleSets
	claim Claim
}

// roleSets holds what the contract declared. They sit behind one pointer so
// that NavigationRoles stays small enough to pass by value.
type roleSets struct {
	paths  map[string]struct{}
	maps   map[string]struct{}
	answer string
	lesson string
}

// Claim reports how far the navigation declaration got.
func (r NavigationRoles) Claim() Claim {
	return r.claim
}

// Available reports whether the contract declared a valid navigation role set.
func (r NavigationRoles) Available() bool {
	return r.claim.held()
}

// Trustworthy reports whether the role sets may be projected over: true when
// they were read cleanly or never declared, false when a declaration was made
// and could not be honoured.
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
	if r.sets == nil {
		return false
	}
	_, ok := r.sets.paths[NormalizeWord(noteType)]
	return ok
}

// IsMapType reports whether noteType is declared as a general map.
func (r NavigationRoles) IsMapType(noteType string) bool {
	if !r.Trustworthy() {
		return false
	}
	if r.sets == nil {
		return false
	}
	_, ok := r.sets.maps[NormalizeWord(noteType)]
	return ok
}

// PathTypes returns every type the contract declares for ordered study paths,
// and MapTypes the same for general maps. They are the words a surface names
// when it has to tell a reader what puts a note on one of those two shelves:
// nothing else can spell them, because a second copy of the vault's vocabulary
// goes on saying a word the vault has since renamed. The words are the folded
// spelling every comparison here uses.
//
// The answer is sorted rather than given in the order the contract listed it.
// What the contract declares is membership, so reordering the same words is no
// change at all, and a reader's sentence that rearranged itself for one would
// be reporting something that did not happen.
//
// A declaration nothing made, or one that could not be honoured, names none —
// the same answer IsPathType and IsMapType give about every note. The returned
// slice is the caller's own.
func (r NavigationRoles) PathTypes() []string {
	if !r.Trustworthy() {
		return nil
	}
	if r.sets == nil {
		return nil
	}
	return slices.Sorted(maps.Keys(r.sets.paths))
}

// MapTypes returns the types the contract declares as general maps. See
// PathTypes for what the answer means.
func (r NavigationRoles) MapTypes() []string {
	if !r.Trustworthy() {
		return nil
	}
	if r.sets == nil {
		return nil
	}
	return slices.Sorted(maps.Keys(r.sets.maps))
}

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
// directory, comparing whole components through the folded identity every vault
// directory scope shares. An unclaimed policy excludes nothing; an unresolved
// one also answers false, so callers gate on Trustworthy first.
func (p ArtifactPolicy) IsNonInstance(rel string) bool {
	if !p.Trustworthy() || p.state == nil {
		return false
	}
	rel = vault.NormalizeNFC(rel)
	for _, dir := range p.state.nonInstanceDirs {
		if pathHasFoldedPrefix(rel, dir) {
			return true
		}
	}
	return false
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

// deriveNavigationRoles reads the roles the contract gave its types. A role it
// does not know, or a second type holding a role only one may hold, closes
// navigation rather than refusing the contract: the check report says so, and
// the reader still opens everything it can read.
func deriveNavigationRoles(types []typeDefinition) NavigationRoles {
	sets := roleSets{paths: map[string]struct{}{}, maps: map[string]struct{}{}}
	holders := map[Role]string{}
	for _, def := range types {
		switch def.role {
		case "":
			continue
		case RoleCourse:
			sets.paths[def.name] = struct{}{}
		case RoleMap:
			sets.maps[def.name] = struct{}{}
		case RoleLesson, RoleThought:
			if holder, taken := holders[def.role]; taken {
				return invalidNavigationRoles("types %q and %q both hold the %s role, which one type holds", holder, def.name, def.role)
			}
			holders[def.role] = def.name
		default:
			return invalidNavigationRoles("type %q has the unknown role %q", def.name, string(def.role))
		}
	}
	sets.lesson = holders[RoleLesson]
	sets.answer = holders[RoleThought]
	return NavigationRoles{sets: &sets, claim: heldClaim()}
}

func invalidNavigationRoles(format string, args ...any) NavigationRoles {
	return NavigationRoles{claim: Rejected(fmt.Sprintf("invalid navigation roles: "+format+"; Paths and Maps disabled", args...))}
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
