// Package schema loads the vault's machine-readable contract
// (System/schemas/vault-schema.toml) and answers status state-machine
// questions.
//
// It is the only package in this repo allowed to read the contract (wall 3):
// no other package may hardcode a second copy of any enum, field list, or
// lifecycle rule.
package schema

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/BurntSushi/toml"
)

// ContractRelPath is where the contract lives inside the vault.
const ContractRelPath = "System/schemas/vault-schema.toml"

// Sentinel errors for state-machine answers. Callers match with errors.Is.
var (
	ErrUnknownStatus     = errors.New("status not defined for this type")
	ErrIllegalTransition = errors.New("transition not allowed by lifecycle")
	ErrOwnerForbidden    = errors.New("actor may not set this status")
)

// Schema is the decoded vault contract.
type Schema struct {
	Version   string  `toml:"schema_version"`
	Enums     Enums   `toml:"enums"`
	Fields    Fields  `toml:"fields"`
	Rules     Rules   `toml:"rules"`
	Scan      Scan    `toml:"scan"`
	Lifecycle []Stage `toml:"lifecycle"`
}

// Enums holds the closed value sets for frontmatter fields.
type Enums struct {
	Type           []string            `toml:"type"`
	Domain         []string            `toml:"domain"`
	SourceKind     []string            `toml:"source_kind"`
	SourceProvider []string            `toml:"source_provider"`
	Level          []string            `toml:"level"`
	MapKind        []string            `toml:"map_kind"`
	Status         map[string][]string `toml:"status"`
}

// Fields describes required and known frontmatter keys.
type Fields struct {
	Required      []string            `toml:"required"`
	RequiredInbox []string            `toml:"required_inbox"`
	DomainExempt  []string            `toml:"domain_exempt_types"`
	Known         []string            `toml:"known"`
	LessonOnly    []string            `toml:"lesson_only"`
	StatusGroup   map[string][]string `toml:"status_group"`
}

// Rules holds structural rules beyond plain enums.
type Rules struct {
	DomainEqualsFolderUnder   []string `toml:"domain_equals_folder_under"`
	ConceptRequiresProvenance []string `toml:"concept_requires_provenance"`
	SlugPattern               string   `toml:"slug_pattern"`
	ForbidTagWithSlash        bool     `toml:"forbid_tag_with_slash"`
}

// Scan is the checker's default scan policy (tool policy, not schema fact).
type Scan struct {
	KnowledgeDirs        []string `toml:"knowledge_dirs"`
	SkipBasenames        []string `toml:"skip_basenames"`
	NoFrontmatterIsLegal bool     `toml:"no_frontmatter_is_legal"`
}

// Stage is one lifecycle entry: a status, the types it applies to, its legal
// predecessor states, and who may set it. An empty From means the status is
// only legal as an initial state; "*" in From or AppliesTo means any.
type Stage struct {
	Status    string   `toml:"status"`
	AppliesTo []string `toml:"applies_to"`
	From      []string `toml:"from"`
	Owner     []string `toml:"owner"`
}

// Load reads the contract from the vault rooted at root.
func Load(root string) (*Schema, error) {
	return LoadFile(filepath.Join(root, filepath.FromSlash(ContractRelPath)))
}

// LoadFile reads the contract from an explicit path.
func LoadFile(path string) (*Schema, error) {
	var s Schema
	if _, err := toml.DecodeFile(path, &s); err != nil {
		return nil, fmt.Errorf("decode vault contract %s: %w", path, err)
	}
	if len(s.Lifecycle) == 0 {
		return nil, fmt.Errorf("vault contract %s: no lifecycle stages", path)
	}
	return &s, nil
}

// StatusGroup returns which status enum group applies to a note type.
// Types not listed in the contract's status_group table use the "note" group.
func (s *Schema) StatusGroup(noteType string) string {
	for group, types := range s.Fields.StatusGroup {
		if slices.Contains(types, noteType) {
			return group
		}
	}
	return "note"
}

// Statuses returns the legal status values for a note type.
func (s *Schema) Statuses(noteType string) []string {
	return s.Enums.Status[s.StatusGroup(noteType)]
}

// Stage returns the lifecycle entry for setting status on a note of the
// given type, or false when the contract defines none.
func (s *Schema) Stage(noteType, status string) (Stage, bool) {
	for _, st := range s.Lifecycle {
		if st.Status != status {
			continue
		}
		if slices.Contains(st.AppliesTo, noteType) || slices.Contains(st.AppliesTo, "*") {
			return st, true
		}
	}
	return Stage{}, false
}

// Transition reports whether actor may move a note of the given type from
// one status to another. An empty from means the note is being given its
// initial status. The returned error wraps one of the package sentinels.
func (s *Schema) Transition(noteType, from, to, actor string) error {
	st, ok := s.Stage(noteType, to)
	if !ok {
		return fmt.Errorf("%w: %q for type %q", ErrUnknownStatus, to, noteType)
	}
	switch {
	case from == "":
		if len(st.From) != 0 && !slices.Contains(st.From, "*") {
			return fmt.Errorf("%w: %q is not an initial status for type %q", ErrIllegalTransition, to, noteType)
		}
	case slices.Contains(st.From, from) || slices.Contains(st.From, "*"):
		// legal predecessor
	default:
		return fmt.Errorf("%w: %s → %s for type %q", ErrIllegalTransition, from, to, noteType)
	}
	if !slices.Contains(st.Owner, actor) {
		return fmt.Errorf("%w: %q may not set %q (owners: %v)", ErrOwnerForbidden, actor, to, st.Owner)
	}
	return nil
}
