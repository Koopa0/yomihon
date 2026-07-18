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
	missingNavigationDiagnostic = "contract declares no navigation roles; Paths and Maps disabled until it does"
	missingArtifactDiagnostic   = "contract declares no artifact policy; instance projections disabled until it does"
	staleArtifactDiagnostic     = "vault artifact policy source changed after startup; instance projections disabled until restart"
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
type NavigationRoles struct {
	pathTypes  map[string]struct{}
	mapTypes   map[string]struct{}
	available  bool
	diagnostic string
}

// Available reports whether the contract declared a valid navigation role set.
func (r NavigationRoles) Available() bool {
	return r.available
}

// Diagnostic explains why navigation roles are unavailable.
func (r NavigationRoles) Diagnostic() string {
	if r.available {
		return ""
	}
	if r.diagnostic != "" {
		return r.diagnostic
	}
	return missingNavigationDiagnostic
}

// IsPathType reports whether noteType is declared as an ordered study path.
func (r NavigationRoles) IsPathType(noteType string) bool {
	_, ok := r.pathTypes[noteType]
	return r.available && ok
}

// IsMapType reports whether noteType is declared as a general map.
func (r NavigationRoles) IsMapType(noteType string) bool {
	_, ok := r.mapTypes[noteType]
	return r.available && ok
}

// ArtifactPolicy identifies vault directories whose files are readable
// artifacts but are not governed note instances.
type ArtifactPolicy struct {
	state *artifactPolicyState
}

type artifactPolicyState struct {
	nonInstanceDirs []string
	source          policySource
	available       bool
	diagnostic      string
	frozen          bool
	stale           atomic.Bool
}

// Available reports whether the contract declared a valid artifact policy.
func (p ArtifactPolicy) Available() bool {
	return p.state != nil && p.state.available && !p.state.stale.Load()
}

// Diagnostic explains why the artifact policy is unavailable.
func (p ArtifactPolicy) Diagnostic() string {
	if p.state == nil {
		return missingArtifactDiagnostic
	}
	if p.state.stale.Load() {
		return staleArtifactDiagnostic
	}
	if p.state.available {
		return ""
	}
	if p.state.diagnostic != "" {
		return p.state.diagnostic
	}
	return missingArtifactDiagnostic
}

// IsNonInstance reports whether rel is equal to or below a declared artifact
// directory. Component boundaries prevent a sibling with the same prefix from
// matching.
func (p ArtifactPolicy) IsNonInstance(rel string) bool {
	if !p.Available() {
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
	if p.state == nil || !p.state.available || p.state.frozen || p.state.stale.Load() {
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
		available:       true,
		frozen:          true,
	}}
}

func deriveNavigationRoles(
	section *navigationSection,
	enumTypes []string,
	pathTypesDefined bool,
	mapTypesDefined bool,
) NavigationRoles {
	if section == nil {
		return NavigationRoles{}
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
	return NavigationRoles{pathTypes: paths, mapTypes: maps, available: true}
}

func invalidNavigationRoles(format string, args ...any) NavigationRoles {
	return NavigationRoles{diagnostic: fmt.Sprintf("invalid navigation roles: "+format+"; Paths and Maps disabled", args...)}
}

func deriveArtifactPolicy(section *artifactSection, nonInstanceDirsDefined bool, source policySource) ArtifactPolicy {
	if section == nil {
		return ArtifactPolicy{}
	}
	if !nonInstanceDirsDefined {
		return ArtifactPolicy{state: &artifactPolicyState{diagnostic: `invalid artifact policy: missing required key "non_instance_dirs"`}}
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
		available:       true,
	}}
}

func invalidArtifactPolicy(value string) ArtifactPolicy {
	return ArtifactPolicy{state: &artifactPolicyState{
		diagnostic: fmt.Sprintf("invalid artifact policy: non_instance_dirs contains %q", value),
	}}
}
