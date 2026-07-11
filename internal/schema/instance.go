package schema

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/koopa0/yomihon/internal/graph"
)

const (
	missingNavigationDiagnostic = "contract declares no navigation roles; Paths and Maps disabled until it does"
	missingArtifactDiagnostic   = "contract declares no artifact policy; instance projections disabled until it does"
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
	nonInstanceDirs []string
	available       bool
	diagnostic      string
}

// Available reports whether the contract declared a valid artifact policy.
func (p ArtifactPolicy) Available() bool {
	return p.available
}

// Diagnostic explains why the artifact policy is unavailable.
func (p ArtifactPolicy) Diagnostic() string {
	if p.available {
		return ""
	}
	if p.diagnostic != "" {
		return p.diagnostic
	}
	return missingArtifactDiagnostic
}

// IsNonInstance reports whether rel is equal to or below a declared artifact
// directory. Component boundaries prevent a sibling with the same prefix from
// matching.
func (p ArtifactPolicy) IsNonInstance(rel string) bool {
	if !p.available {
		return false
	}
	rel = graph.NormalizeNFC(rel)
	for _, dir := range p.nonInstanceDirs {
		if rel == dir || strings.HasPrefix(rel, dir+"/") {
			return true
		}
	}
	return false
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

func deriveArtifactPolicy(section *artifactSection, nonInstanceDirsDefined bool) ArtifactPolicy {
	if section == nil {
		return ArtifactPolicy{}
	}
	if !nonInstanceDirsDefined {
		return ArtifactPolicy{diagnostic: `invalid artifact policy: missing required key "non_instance_dirs"`}
	}
	dirs := make([]string, 0, len(section.NonInstanceDirs))
	for _, original := range section.NonInstanceDirs {
		if original == "" || strings.Contains(original, `\`) {
			return invalidArtifactPolicy(original)
		}
		normalized := path.Clean(graph.NormalizeNFC(original))
		if normalized == "." || normalized == ".." || strings.HasPrefix(normalized, "../") || path.IsAbs(normalized) {
			return invalidArtifactPolicy(original)
		}
		dirs = append(dirs, normalized)
	}
	return ArtifactPolicy{nonInstanceDirs: slices.Clone(dirs), available: true}
}

func invalidArtifactPolicy(value string) ArtifactPolicy {
	return ArtifactPolicy{diagnostic: fmt.Sprintf("invalid artifact policy: non_instance_dirs contains %q", value)}
}
