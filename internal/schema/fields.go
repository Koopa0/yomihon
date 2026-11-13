package schema

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
)

// The frontmatter fields yomihon defines for a note. The list is closed: an
// author who follows yomihon's rules writes these and nothing else, and a key
// outside it is reported. title and domain are not among them because they are
// derived, the title from the note's first heading and the domain from the
// folder it sits in.
const (
	FieldType          = "type"
	FieldAliases       = "aliases"
	FieldBasedOn       = "based_on"
	FieldLang          = "lang"
	FieldCreated       = "created"
	FieldSlug          = "slug"
	FieldLevel         = "level"
	FieldPrerequisites = "prerequisites"
	FieldVerifiedOn    = "verified_on"
	FieldVerifiedWith  = "verified_with"
)

// domainField names the enum a contract may state for the derived domain. It is
// not a frontmatter field: a note that writes it is reported like any other
// unknown key.
const domainField = "domain"

// titleField names the other derived value. A contract may state no enum for
// it and list it nowhere; it is read from the note's first heading.
const titleField = "title"

// definedFields is the vocabulary a contract chooses from: DraftField, which
// is built in, and the fields above.
var definedFields = []string{
	FieldType, FieldAliases, DraftField, FieldBasedOn, FieldLang, FieldCreated,
	FieldSlug, FieldLevel, FieldPrerequisites, FieldVerifiedOn, FieldVerifiedWith,
}

// obsidianReserved are the property names Obsidian itself owns. A note may
// carry them anywhere; yomihon neither lists them in a contract nor judges them.
var obsidianReserved = []string{"tags", "aliases", "cssclasses"}

// builtInFields are the keys every note may carry whether or not a contract
// lists them: the note's type, the draft flag the product itself defines, and
// the names Obsidian reserves.
var builtInFields = append([]string{FieldType, DraftField}, obsidianReserved...)

// removedFields are the names earlier contracts let a note write that yomihon
// no longer defines. Like the derived names they stay out of the closed list.
var removedFields = []string{
	"status", "replaces", "supersedes", "superseded_by", "archived", "published",
	"topics", "updated", "source_kind", "source_provider", "map_kind",
}

// shutProvenanceNames reports whether the concept provenance list may not name
// field: it is built in, derived or removed, so a concept would be let write it
// with no finding.
func shutProvenanceNames(field string) bool {
	return slices.Contains(builtInFields, field) || field == domainField || field == titleField ||
		slices.Contains(removedFields, field)
}

// Fields returns the frontmatter fields yomihon defines, in the order the
// documentation names them. The slice is the caller's own.
func Fields() []string {
	return slices.Clone(definedFields)
}

// FieldStanding says how one frontmatter key stands against the type of the
// note that wrote it.
type FieldStanding int

const (
	// FieldUnknown is a key no part of the contract lets this note carry.
	FieldUnknown FieldStanding = iota
	// FieldAllowed is a key the note's type, or every type, lets it carry.
	FieldAllowed
	// FieldTypeDependent is a key some type lists and this note's type is not
	// one of the declared ones, so it cannot be judged until the type is.
	FieldTypeDependent
)

// String names a standing for a diagnostic. A value outside the three
// constants is a programming error and panics.
func (s FieldStanding) String() string {
	switch s {
	case FieldUnknown:
		return "unknown"
	case FieldAllowed:
		return "allowed"
	case FieldTypeDependent:
		return "type-dependent"
	default:
		panic("schema: unknown FieldStanding: " + strconv.Itoa(int(s)))
	}
}

// FieldStanding reports whether a note of noteType may carry key. noteType is
// the note's own type word as written, empty for a note with none.
func (c *Contract) FieldStanding(noteType, key string) FieldStanding {
	if c == nil {
		return FieldUnknown
	}
	if slices.Contains(builtInFields, key) || slices.Contains(c.common, key) {
		return FieldAllowed
	}
	index, declared := c.typeByName[NormalizeWord(noteType)]
	if declared {
		def := &c.types[index]
		if slices.Contains(def.fields, key) || c.provenanceAdmits(def.name, key) {
			return FieldAllowed
		}
		return FieldUnknown
	}
	if noteType == "" {
		return FieldUnknown
	}
	for i := range c.types {
		if slices.Contains(c.types[i].fields, key) || c.provenanceAdmits(c.types[i].name, key) {
			return FieldTypeDependent
		}
	}
	return FieldUnknown
}

// provenanceAdmits reports whether key is a field the concept rule names, which
// the concept type may carry beside the fields it lists.
func (c *Contract) provenanceAdmits(typeName, key string) bool {
	return typeName == conceptType && slices.Contains(c.provenanceFields, key)
}

// RequiredFields returns the fields every note of noteType must carry: those
// every note must, then those the type adds. The slice is the caller's own.
func (c *Contract) RequiredFields(noteType string) []string {
	if c == nil {
		return nil
	}
	out := slices.Clone(c.required)
	if index, ok := c.typeByName[NormalizeWord(noteType)]; ok {
		for _, field := range c.types[index].required {
			if !slices.Contains(out, field) {
				out = append(out, field)
			}
		}
	}
	return out
}

// SlugPattern returns the pattern a note of noteType must write its slug to,
// and false where the type states none. The expression is shared and safe for
// concurrent use.
func (c *Contract) SlugPattern(noteType string) (*regexp.Regexp, bool) {
	if c == nil {
		return nil, false
	}
	index, ok := c.typeByName[NormalizeWord(noteType)]
	if !ok || c.types[index].slugPattern == nil {
		return nil, false
	}
	return c.types[index].slugPattern, true
}

// TypeRole returns the role the contract gives noteType, empty for a plain
// type and for one the contract does not declare.
func (c *Contract) TypeRole(noteType string) Role {
	if c == nil {
		return ""
	}
	if index, ok := c.typeByName[NormalizeWord(noteType)]; ok {
		return c.types[index].role
	}
	return ""
}

func validateFieldLists(contract *Contract) error {
	if err := checkFieldNames("fields.common", contract.common, nil); err != nil {
		return err
	}
	if err := validateUniqueStrings("fields.required", contract.required); err != nil {
		return err
	}
	allowedEverywhere := slices.Concat(builtInFields, contract.common)
	for _, field := range contract.required {
		if !slices.Contains(allowedEverywhere, field) {
			return fmt.Errorf("fields.required: %q is not listed in fields.common", field)
		}
	}
	_, hasConcept := contract.typeByName[conceptType]
	for i := range contract.types {
		var extra []string
		if hasConcept && contract.types[i].name == conceptType {
			extra = contract.rules.ConceptRequiresProvenance
		}
		if err := validateTypeFields(&contract.types[i], allowedEverywhere, extra); err != nil {
			return err
		}
	}
	return nil
}

// checkFieldNames refuses a list that names a field yomihon does not define,
// and one that names a field twice. extra holds the names the contract's own
// rules declare for this list's type.
func checkFieldNames(name string, values, extra []string) error {
	if err := validateUniqueStrings(name, values); err != nil {
		return err
	}
	for _, value := range values {
		defined := slices.Contains(definedFields, value) || slices.Contains(obsidianReserved, value)
		if !defined && !slices.Contains(extra, value) {
			return fmt.Errorf("%s: %q is not a field yomihon defines (the fields are %s; title and domain are derived)",
				name, value, formatContractKeys(definedFields))
		}
	}
	return nil
}

// validateTypeFields holds one type's own lists to the closed field list and
// to what its role needs, and compiles its slug pattern. provenance is the
// fields the concept rule names, which that type carries beside its own.
func validateTypeFields(def *typeDefinition, allowedEverywhere, provenance []string) error {
	name := "types." + def.name
	if err := checkFieldNames(name+".fields", def.fields, provenance); err != nil {
		return err
	}
	if err := validateUniqueStrings(name+".required", def.required); err != nil {
		return err
	}
	allowed := slices.Concat(allowedEverywhere, def.fields, provenance)
	for _, field := range def.required {
		if !slices.Contains(allowed, field) {
			return fmt.Errorf("%s.required: %q is not listed in %s.fields", name, field, name)
		}
	}
	if def.role == RoleLesson && !slices.Contains(allowed, FieldSlug) {
		return fmt.Errorf("%s: the lesson role needs slug in %s.fields", name, name)
	}
	if def.role == RoleThought && !slices.Contains(allowed, FieldBasedOn) {
		return fmt.Errorf("%s: the thought role needs based_on in %s.fields", name, name)
	}
	return compileSlugPattern(def, allowed)
}

func compileSlugPattern(def *typeDefinition, allowed []string) error {
	if def.slugText == "" {
		return nil
	}
	if !slices.Contains(allowed, FieldSlug) {
		return fmt.Errorf("types.%s.slug_pattern: the type does not list slug in fields", def.name)
	}
	pattern, err := regexp.Compile(def.slugText)
	if err != nil {
		return fmt.Errorf("types.%s.slug_pattern: invalid regular expression: %w", def.name, err)
	}
	def.slugPattern = pattern
	return nil
}
