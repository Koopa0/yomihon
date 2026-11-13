// Package schema loads the vault's machine-readable contract
// (System/schemas/vault-schema.toml). It is the only package allowed to read
// that contract: nowhere else holds a second copy of an enum, a field list, or
// the role a note type plays.
package schema

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/BurntSushi/toml"

	"github.com/koopa0/yomihon/internal/vault"
)

const (
	// ContractRelPath is where the contract lives inside the vault.
	ContractRelPath  = "System/schemas/vault-schema.toml"
	supportedVersion = "2"

	// conceptType is the note type name yomihon reserves for distilled ideas,
	// spelled once so no face keeps a second copy. It is a name, not a role: a
	// contract that wants the concept sheets and the coverage report declares
	// a type of that name.
	conceptType = "concept"

	// lessonType is the type name a note is read as when no contract names a
	// type with the lesson role, so a note that calls itself a lesson keeps its
	// reading aids in a folder that declared nothing.
	lessonType = "lesson"
)

// Role is what a note type does in the reader, which a contract states beside
// the type. A type with no role is a plain note.
type Role string

const (
	// RoleLesson marks the notes a course is made of: each carries a slug.
	RoleLesson Role = "lesson"
	// RoleCourse marks an ordered study path over lessons.
	RoleCourse Role = "course"
	// RoleMap marks a general map of notes.
	RoleMap Role = "map"
	// RoleThought marks a reader's own source-linked thoughts, the notes the
	// shelf of what is left open is made of.
	RoleThought Role = "thought"
)

// The heading and inline marks a contract that omits the keys is loaded with.
// They are this dialect's gap vocabulary, not a vault's: writing the key
// replaces them, and writing an empty list tracks no mark.
var (
	defaultPlannedGapMarks    = []string{"缺口", "待補", "待寫", "待整理", "待建"}
	defaultPlannedInlineMarks = []string{"待整理", "待建", "下一課"}
)

// DefaultPlannedGapMarks is the heading-mark list a contract that omits
// rules.planned_gap_marks is loaded with, cloned so a caller cannot change
// the loader's own copy.
func DefaultPlannedGapMarks() []string {
	return slices.Clone(defaultPlannedGapMarks)
}

// DefaultPlannedInlineMarks is the inline-mark list a contract that omits
// rules.planned_inline_marks is loaded with, cloned so a caller cannot change
// the loader's own copy.
func DefaultPlannedInlineMarks() []string {
	return slices.Clone(defaultPlannedInlineMarks)
}

// Contract is the validated, immutable vault authority. Its zero value carries
// no authority; load one with [Load], [LoadFile], or [LoadReader]. A nil
// *Contract answers as an ungoverned vault: every method is safe to call and
// returns what a folder that declared nothing declares — no version, no
// vocabulary, no capability.
type Contract struct {
	version    string
	types      []typeDefinition
	typeByName map[string]int
	enums      []enumDefinition
	common     []string
	required   []string
	rules      Rules
	folders    Folders

	navigationRoles NavigationRoles
	knowledgeScope  KnowledgeScope
	artifactPolicy  ArtifactPolicy
	privacyPolicy   PrivacyPolicy
	journalDir      JournalDir
	written         writtenKeys

	// provenanceFields are the fields the concept rule names, which that type
	// may carry beside the fields it lists.
	provenanceFields []string
}

type typeDefinition struct {
	name        string
	role        Role
	required    []string
	fields      []string
	slugPattern *regexp.Regexp
	slugText    string
}

type enumDefinition struct {
	field  string
	values []string
}

// Definition is a detached copy of the contract's declarative vocabulary.
// Mutating it never changes the loaded [Contract].
type Definition struct {
	Types   []TypeDefinition
	Enums   []EnumDefinition
	Fields  FieldDefinition
	Rules   Rules
	Folders Folders
}

// TypeDefinition is one note type as the contract declares it.
type TypeDefinition struct {
	Name        string
	Role        Role
	Required    []string
	Fields      []string
	SlugPattern string
}

// EnumDefinition is the vocabulary a contract gives one frontmatter field, or
// the derived domain.
type EnumDefinition struct {
	Field  string
	Values []string
}

// FieldDefinition holds the fields every type may carry and the ones every
// note must carry.
type FieldDefinition struct {
	Common   []string
	Required []string
}

// Rules holds the structural rules a contract still states beside its types.
type Rules struct {
	ConceptRequiresProvenance []string
	PlannedGapMarks           []string
	PlannedInlineMarks        []string
}

// Folders are the directories and file names the contract names by what they
// are for. Which of them the contract wrote is not recoverable from here: a
// field left out and a field written empty read the same.
type Folders struct {
	Checked             []string
	Ignored             []string
	Templates           []string
	Journal             string
	DomainFrom          []string
	SkippedFiles        []string
	FrontmatterOptional bool
}

type contractFile struct {
	Version string              `toml:"schema_version"`
	Folders folderSection       `toml:"folders"`
	Enums   map[string][]string `toml:"enums"`
	Fields  fieldSection        `toml:"fields"`
	Types   map[string]typeFile `toml:"types"`
	Rules   ruleSection         `toml:"rules"`
	Privacy toml.Primitive      `toml:"privacy"`
}

type folderSection struct {
	Checked             []string `toml:"checked"`
	Ignored             []string `toml:"ignored"`
	Templates           []string `toml:"templates"`
	Journal             string   `toml:"journal"`
	DomainFrom          []string `toml:"domain_from"`
	SkippedFiles        []string `toml:"skipped_files"`
	FrontmatterOptional bool     `toml:"frontmatter_optional"`
}

type fieldSection struct {
	Common   []string `toml:"common"`
	Required []string `toml:"required"`
}

type typeFile struct {
	Role        string   `toml:"role"`
	Required    []string `toml:"required"`
	Fields      []string `toml:"fields"`
	SlugPattern string   `toml:"slug_pattern"`
}

type ruleSection struct {
	ConceptRequiresProvenance []string `toml:"concept_requires_provenance"`
	PlannedGapMarks           []string `toml:"planned_gap_marks"`
	PlannedInlineMarks        []string `toml:"planned_inline_marks"`
}

// writtenKeys records which optional keys the contract wrote down. Each holds
// a value that is also its Go zero value, so absence has to be recorded
// separately from a key written as false or as an empty list.
type writtenKeys struct {
	frontmatterOptional bool
	plannedGapMarks     bool
	plannedInlineMarks  bool
}

type privacyPrimitives struct {
	NeverEgressDirs toml.Primitive `toml:"never_egress_dirs"`
}

type contractUnknownKeys struct {
	core    []string
	privacy []string
}

// ContractAbsent reports whether err from Load, LoadFile, or LoadReader means
// the folder carries no contract at all, as distinct from one that exists and
// could not be read. A folder that declared nothing is not one in trouble.
func ContractAbsent(err error) bool {
	return errors.Is(err, fs.ErrNotExist)
}

// Load reads the contract from the vault rooted at root, and LoadFile from an
// explicit path. Neither is on the served path, which loads through LoadReader
// so a capability binds to the exact file it came from; these two serve a
// caller that holds only a path.
func Load(root string) (*Contract, error) {
	return LoadFile(filepath.Join(root, filepath.FromSlash(ContractRelPath)))
}

func LoadFile(path string) (*Contract, error) {
	sourcePath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve vault contract %s: %w", path, err)
	}
	data, err := os.ReadFile(sourcePath) // #nosec G304 -- path is the operator-selected vault contract
	if err != nil {
		return nil, fmt.Errorf("read vault contract %s: %w", path, err)
	}
	source := policySource{path: filepath.Clean(sourcePath), digest: sha256.Sum256(data)}
	s, err := decodeContract(data, source)
	if err != nil {
		return nil, fmt.Errorf("decode vault contract %s: %w", path, err)
	}
	return s, nil
}

// LoadReader reads the contract through the same pinned vault capability that
// an agent-facing action uses for note collection and source revalidation.
func LoadReader(ctx context.Context, reader *vault.Reader) (*Contract, error) {
	if reader == nil {
		return nil, errors.New("read vault contract: vault reader is nil")
	}
	entry, err := reader.Lookup(ContractRelPath)
	if err != nil {
		return nil, fmt.Errorf("resolve vault contract: %w", err)
	}
	data, err := reader.ReadFile(ctx, entry)
	if err != nil {
		return nil, fmt.Errorf("read vault contract: %w", err)
	}
	source := policySource{
		path:   filepath.Join(reader.Name(), filepath.FromSlash(ContractRelPath)),
		digest: sha256.Sum256(data),
		pinned: &pinnedPolicySource{reader: reader, entry: entry},
	}
	contract, err := decodeContract(data, source)
	if err != nil {
		return nil, fmt.Errorf("decode vault contract: %w", err)
	}
	return contract, nil
}

// checkVersion reads the contract's schema_version before anything else is
// decoded, so a contract written for another version is refused with the one
// sentence that names the mismatch. Decoded further, it would be reported as
// whichever of its tables this version no longer has.
func checkVersion(data []byte) error {
	var probe struct {
		Version string `toml:"schema_version"`
	}
	meta, err := toml.Decode(string(data), &probe)
	if err != nil {
		return err
	}
	if !meta.IsDefined("schema_version") {
		return errors.New(`missing required key "schema_version"`)
	}
	if probe.Version != supportedVersion {
		return fmt.Errorf("unsupported schema_version %q; supported version is %q", probe.Version, supportedVersion)
	}
	return nil
}

func decodeContract(data []byte, source policySource) (*Contract, error) {
	if err := checkContractDepth(data); err != nil {
		return nil, err
	}
	if err := validateDistinctKeys(data); err != nil {
		return nil, err
	}
	if err := checkVersion(data); err != nil {
		return nil, err
	}
	var decoded contractFile
	tomlMeta, err := toml.Decode(string(data), &decoded)
	if err != nil {
		return nil, err
	}
	privacy, privacyTypeErrorKey := decodePrivacySection(&tomlMeta, decoded.Privacy)
	unknown := classifyUnknownKeys(tomlMeta.Undecoded())
	if len(unknown.core) > 0 {
		slices.Sort(unknown.core)
		return nil, fmt.Errorf("unknown keys: %s", formatContractKeys(unknown.core))
	}

	contract := &Contract{
		version: decoded.Version,
		common:  slices.Clone(decoded.Fields.Common),
		written: writtenKeys{
			frontmatterOptional: tomlMeta.IsDefined("folders", "frontmatter_optional"),
			plannedGapMarks:     tomlMeta.IsDefined("rules", "planned_gap_marks"),
			plannedInlineMarks:  tomlMeta.IsDefined("rules", "planned_inline_marks"),
		},
		required: slices.Clone(decoded.Fields.Required),
		rules: Rules{
			ConceptRequiresProvenance: slices.Clone(decoded.Rules.ConceptRequiresProvenance),
			PlannedGapMarks:           slices.Clone(decoded.Rules.PlannedGapMarks),
			PlannedInlineMarks:        slices.Clone(decoded.Rules.PlannedInlineMarks),
		},
		folders: Folders{
			Checked:             slices.Clone(decoded.Folders.Checked),
			Ignored:             slices.Clone(decoded.Folders.Ignored),
			Templates:           slices.Clone(decoded.Folders.Templates),
			Journal:             decoded.Folders.Journal,
			DomainFrom:          slices.Clone(decoded.Folders.DomainFrom),
			SkippedFiles:        slices.Clone(decoded.Folders.SkippedFiles),
			FrontmatterOptional: decoded.Folders.FrontmatterOptional,
		},
	}
	applyPlannedMarkDefaults(&contract.rules, contract.written)
	if duplicate, found := firstRepeated(decoded.Enums["type"]); found {
		return nil, fmt.Errorf("enums.type: duplicate value %q", duplicate)
	}
	contract.declareTypes(decoded.Types, decoded.Enums, tomlMeta.Keys())
	contract.declareEnums(decoded.Enums)
	foldDeclaredWords(contract)
	if err := validateContractSemantics(contract); err != nil {
		return nil, err
	}

	contract.navigationRoles = deriveNavigationRoles(contract.types)
	contract.knowledgeScope = deriveKnowledgeScope(contract.folders.Checked)
	contract.journalDir = deriveJournalDir(contract.folders.Journal, tomlMeta.IsDefined("folders", "journal"))
	contract.artifactPolicy = deriveArtifactPolicy(contract.folders.Templates, source)
	contract.privacyPolicy = resolvePrivacyPolicy(
		privacy,
		privacyTypeErrorKey,
		unknown.privacy,
		source,
		&tomlMeta,
	)
	return contract, nil
}

// declareTypes lists the contract's note types in the order they were written.
// A type is declared by its own [types.<name>] table or by a place in
// enums.type, so a plain type with nothing to say needs no table of its own.
func (c *Contract) declareTypes(tables map[string]typeFile, enums map[string][]string, keys []toml.Key) {
	var names []string
	add := func(name string) {
		if !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	for _, name := range enums["type"] {
		add(name)
	}
	for _, key := range keys {
		if len(key) >= 2 && key[0] == "types" {
			add(key[1])
		}
	}
	c.types = make([]typeDefinition, 0, len(names))
	c.typeByName = make(map[string]int, len(names))
	for _, name := range names {
		table := tables[name]
		def := typeDefinition{
			name:     name,
			role:     Role(table.Role),
			required: slices.Clone(table.Required),
			fields:   slices.Clone(table.Fields),
			slugText: table.SlugPattern,
		}
		c.typeByName[name] = len(c.types)
		c.types = append(c.types, def)
	}
}

// declareEnums keeps every vocabulary in the order its contract wrote it,
// leaving out the type list, which the types themselves carry.
func (c *Contract) declareEnums(enums map[string][]string) {
	fields := make([]string, 0, len(enums))
	for field := range enums {
		if field != "type" {
			fields = append(fields, field)
		}
	}
	slices.Sort(fields)
	c.enums = make([]enumDefinition, 0, len(fields))
	for _, field := range fields {
		c.enums = append(c.enums, enumDefinition{field: field, values: slices.Clone(enums[field])})
	}
}

// foldDeclaredWords folds every word this contract declares to one spelling, so
// a note's own value is the only side left to fold when something compares the
// two. A contract is written in an editor and a note's frontmatter is read back
// off a filesystem, and the two hand over different byte strings for the same
// word; a lookup keyed on the bytes then loses that note's whole type. This
// runs before the contract is validated, so a contract declaring one word
// twice is refused as the duplicate it is. Case is left alone: two words
// differing only in case are two declarations the contract meant.
func foldDeclaredWords(contract *Contract) {
	for i := range contract.types {
		contract.types[i].name = NormalizeWord(contract.types[i].name)
	}
	for i := range contract.enums {
		foldWords(contract.enums[i].values)
	}
	contract.typeByName = make(map[string]int, len(contract.types))
	for i := range contract.types {
		if _, duplicate := contract.typeByName[contract.types[i].name]; !duplicate {
			contract.typeByName[contract.types[i].name] = i
		}
	}
}

func foldWords(words []string) {
	for i, word := range words {
		words[i] = NormalizeWord(word)
	}
}

func decodePrivacySection(
	metadata *toml.MetaData,
	primitive toml.Primitive,
) (section *privacySection, typeErrorKey string) {
	if !metadata.IsDefined("privacy") {
		return nil, ""
	}
	var fields privacyPrimitives
	if err := metadata.PrimitiveDecode(primitive, &fields); err != nil {
		return nil, "privacy"
	}
	section = &privacySection{}
	if metadata.IsDefined("privacy", "never_egress_dirs") {
		if err := metadata.PrimitiveDecode(fields.NeverEgressDirs, &section.NeverEgressDirs); err != nil {
			return nil, "privacy.never_egress_dirs"
		}
	}
	return section, ""
}

func classifyUnknownKeys(keys []toml.Key) contractUnknownKeys {
	var unknown contractUnknownKeys
	for _, key := range keys {
		if key[0] == "privacy" {
			unknown.privacy = append(unknown.privacy, key.String())
			continue
		}
		unknown.core = append(unknown.core, key.String())
	}
	return unknown
}

func resolvePrivacyPolicy(
	section *privacySection,
	typeErrorKey string,
	unknownKeys []string,
	source policySource,
	metadata *toml.MetaData,
) PrivacyPolicy {
	if typeErrorKey != "" {
		return PrivacyPolicy{state: &privacyPolicyState{
			claim: Rejected(fmt.Sprintf("invalid privacy policy: key %q has incompatible TOML type", typeErrorKey)),
		}}
	}
	if len(unknownKeys) > 0 {
		slices.Sort(unknownKeys)
		return PrivacyPolicy{state: &privacyPolicyState{
			claim: Rejected("invalid privacy policy: unknown keys " + formatContractKeys(unknownKeys)),
		}}
	}
	return derivePrivacyPolicy(
		section,
		metadata.IsDefined("privacy", "never_egress_dirs"),
		source,
	)
}

// validateDistinctKeys refuses a contract that spells one key two ways.
//
// A table's key reaches a Go field by its exact name and, failing that, by one
// differing only in case; the table is walked in map order, so two spellings
// that fold together have no fixed winner and whichever loses is dropped
// without a word. Sibling names follow strings.EqualFold identity, the
// comparison the decoder makes. Keys read into a map keep both spellings and
// are refused anyway: the vault's vocabulary has one place to be written down.
func validateDistinctKeys(data []byte) error {
	var tables map[string]any
	if _, err := toml.Decode(string(data), &tables); err != nil {
		return err
	}
	pairs := foldedKeyPairs("", tables)
	if len(pairs) == 0 {
		return nil
	}
	return fmt.Errorf("contract keys differ only by letter case: %s", strings.Join(pairs, "; "))
}

// foldedKeyPairs names every pair of sibling keys that fold together, in each
// table reachable from table.
func foldedKeyPairs(prefix string, table map[string]any) []string {
	names := make([]string, 0, len(table))
	for name := range table {
		names = append(names, name)
	}
	slices.Sort(names)

	pairs := siblingsThatFold(prefix, names)
	for _, name := range names {
		pairs = append(pairs, foldedKeyPairsUnder(prefix+name+".", table[name])...)
	}
	return pairs
}

// siblingsThatFold names the pairs, among one table's own sorted keys, that
// the decoder resolves to the same field.
func siblingsThatFold(prefix string, names []string) []string {
	groups := make(map[string][]string, len(names))
	folded := make([]string, len(names))
	for i, name := range names {
		key := simpleFoldKey(name)
		folded[i] = key
		groups[key] = append(groups[key], name)
	}

	var pairs []string
	for i, name := range names {
		// Keep the original name order even when different fold groups
		// interleave; each name reports only its remaining group members.
		others := groups[folded[i]][1:]
		groups[folded[i]] = others
		for _, other := range others {
			pairs = append(pairs, fmt.Sprintf("%s and %s",
				strconv.Quote(prefix+name), strconv.Quote(prefix+other)))
		}
	}
	return pairs
}

// simpleFoldKey chooses one rune from each Unicode simple-fold orbit. It
// preserves strings.EqualFold identity: full folding would conflate ß and ss.
func simpleFoldKey(name string) string {
	return strings.Map(func(r rune) rune {
		least := r
		for folded := unicode.SimpleFold(r); folded != r; folded = unicode.SimpleFold(folded) {
			least = min(least, folded)
		}
		return least
	}, name)
}

// foldedKeyPairsUnder walks the tables a value holds. Rows of a list are walked
// one at a time: each binds on its own, so two rows spelling one field
// differently is not an ambiguity.
func foldedKeyPairsUnder(prefix string, value any) []string {
	switch value := value.(type) {
	case map[string]any:
		return foldedKeyPairs(prefix, value)
	case []map[string]any:
		var pairs []string
		for _, row := range value {
			pairs = append(pairs, foldedKeyPairs(prefix, row)...)
		}
		return pairs
	case []any:
		var pairs []string
		for _, element := range value {
			if row, ok := element.(map[string]any); ok {
				pairs = append(pairs, foldedKeyPairs(prefix, row)...)
			}
		}
		return pairs
	}
	return nil
}

func formatContractKeys(keys []string) string {
	quoted := make([]string, len(keys))
	for i, key := range keys {
		quoted[i] = strconv.Quote(key)
	}
	return strings.Join(quoted, ", ")
}

func validateContractSemantics(contract *Contract) error {
	if err := validateTypes(contract); err != nil {
		return err
	}
	if err := validateEnums(contract); err != nil {
		return err
	}
	if err := validateFieldLists(contract); err != nil {
		return err
	}
	if err := validateRules(contract); err != nil {
		return err
	}
	return validateFolders(&contract.folders)
}

func validateTypes(contract *Contract) error {
	seen := make(map[string]struct{}, len(contract.types))
	for _, def := range contract.types {
		if def.name == "" {
			return errors.New("types: empty type name")
		}
		if _, duplicate := seen[def.name]; duplicate {
			return fmt.Errorf("types: duplicate type %q", def.name)
		}
		seen[def.name] = struct{}{}
	}
	return nil
}

func validateEnums(contract *Contract) error {
	for _, enum := range contract.enums {
		name := "enums." + enum.field
		if enum.field != domainField && !slices.Contains(Fields(), enum.field) {
			return fmt.Errorf("%s: %q is not a field yomihon defines or derives", name, enum.field)
		}
		if err := validateUniqueStrings(name, enum.values); err != nil {
			return err
		}
	}
	return nil
}

func validateUniqueStrings(name string, values []string) error {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value == "" {
			return fmt.Errorf("%s: empty value", name)
		}
		if _, duplicate := seen[value]; duplicate {
			return fmt.Errorf("%s: duplicate value %q", name, value)
		}
		seen[value] = struct{}{}
	}
	return nil
}

// firstRepeated returns the first non-empty value a list writes twice.
func firstRepeated(values []string) (string, bool) {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, duplicate := seen[value]; duplicate && value != "" {
			return value, true
		}
		seen[value] = struct{}{}
	}
	return "", false
}

func applyPlannedMarkDefaults(rules *Rules, written writtenKeys) {
	if !written.plannedGapMarks {
		rules.PlannedGapMarks = slices.Clone(defaultPlannedGapMarks)
	}
	if !written.plannedInlineMarks {
		rules.PlannedInlineMarks = slices.Clone(defaultPlannedInlineMarks)
	}
}

func validateRules(contract *Contract) error {
	rules := &contract.rules
	if err := validateUniqueStrings("rules.planned_gap_marks", rules.PlannedGapMarks); err != nil {
		return err
	}
	if err := validateUniqueStrings("rules.planned_inline_marks", rules.PlannedInlineMarks); err != nil {
		return err
	}
	if err := validateUniqueStrings("rules.concept_requires_provenance", rules.ConceptRequiresProvenance); err != nil {
		return err
	}
	if _, hasConcept := contract.typeByName[conceptType]; hasConcept && len(rules.ConceptRequiresProvenance) == 0 {
		return errors.New(`rules.concept_requires_provenance must not be empty when the types include "concept"`)
	}
	if _, hasConcept := contract.typeByName[conceptType]; hasConcept {
		contract.provenanceFields = slices.Clone(rules.ConceptRequiresProvenance)
	}
	return nil
}

func validateFolders(folders *Folders) error {
	if err := validateDomainRoots(folders.DomainFrom); err != nil {
		return err
	}
	if err := validateUniqueStrings("folders.checked", folders.Checked); err != nil {
		return err
	}
	for _, dir := range folders.Checked {
		if !validTopLevelComponent(dir) {
			return fmt.Errorf("folders.checked: unsafe top-level component %q", dir)
		}
	}

	if err := validateUniqueStrings("folders.skipped_files", folders.SkippedFiles); err != nil {
		return err
	}
	for _, basename := range folders.SkippedFiles {
		if !validBasename(basename) {
			return fmt.Errorf("folders.skipped_files: invalid basename %q", basename)
		}
	}

	if err := validateUniqueStrings("folders.ignored", folders.Ignored); err != nil {
		return err
	}
	for _, pattern := range folders.Ignored {
		if fault := vault.ValidExcludePattern(pattern); fault != "" {
			return fmt.Errorf("folders.ignored: pattern %q %s", pattern, fault)
		}
	}
	return nil
}

func validateDomainRoots(roots []string) error {
	if err := validateUniqueStrings("folders.domain_from", roots); err != nil {
		return err
	}
	for i, dir := range roots {
		if !validDomainRoot(dir) {
			return fmt.Errorf("folders.domain_from: unsafe relative directory path %q", dir)
		}
		for _, previous := range roots[:i] {
			if pathHasFoldedPrefix(dir, previous) || pathHasFoldedPrefix(previous, dir) {
				return fmt.Errorf("folders.domain_from: overlapping roots %q and %q", previous, dir)
			}
		}
	}
	return nil
}

func validDomainRoot(value string) bool {
	if filepath.IsAbs(value) {
		return false
	}
	for component := range strings.SplitSeq(value, "/") {
		if !validTopLevelComponent(component) {
			return false
		}
	}
	return true
}

// DomainFolder returns the first directory after a matching declared root.
// Roots must be validated vault-relative directory paths with no overlap under
// [SameDirName]. relPath must be a canonical vault-relative file path; matching
// is byte-exact.
// A file directly under a root, or outside every root, has no domain folder.
func DomainFolder(roots []string, relPath string) (string, bool) {
	for _, root := range roots {
		if rest, ok := strings.CutPrefix(relPath, root+"/"); ok {
			folder, _, found := strings.Cut(rest, "/")
			if found {
				return folder, true
			}
			return "", false
		}
	}
	return "", false
}

func validTopLevelComponent(value string) bool {
	return value != "" &&
		value != "." &&
		value != ".." &&
		!filepath.IsAbs(value) &&
		!strings.ContainsRune(value, 0) &&
		!strings.ContainsAny(value, `/\`)
}

func validBasename(value string) bool {
	return value != "" &&
		value != "." &&
		value != ".." &&
		!strings.ContainsRune(value, 0) &&
		!strings.ContainsAny(value, `/\`)
}

// Version returns the contract format version, empty for a vault no contract
// governs.
func (c *Contract) Version() string {
	if c == nil {
		return ""
	}
	return c.version
}

// Definition returns a detached copy of the contract's declarative
// vocabulary. Knowledge-layer membership is [KnowledgeScope], skipped files
// are [Contract.SkipBasenames], and ignored directories are
// [Contract.ScanBoundary]; the folders carry what was written. A vault no
// contract governs declares an empty vocabulary.
func (c *Contract) Definition() Definition {
	if c == nil {
		return Definition{}
	}
	out := Definition{
		Types:  make([]TypeDefinition, len(c.types)),
		Enums:  make([]EnumDefinition, len(c.enums)),
		Fields: FieldDefinition{Common: slices.Clone(c.common), Required: slices.Clone(c.required)},
		Rules: Rules{
			ConceptRequiresProvenance: slices.Clone(c.rules.ConceptRequiresProvenance),
			PlannedGapMarks:           slices.Clone(c.rules.PlannedGapMarks),
			PlannedInlineMarks:        slices.Clone(c.rules.PlannedInlineMarks),
		},
		Folders: Folders{
			Checked:             slices.Clone(c.folders.Checked),
			Ignored:             slices.Clone(c.folders.Ignored),
			Templates:           slices.Clone(c.folders.Templates),
			Journal:             c.folders.Journal,
			DomainFrom:          slices.Clone(c.folders.DomainFrom),
			SkippedFiles:        slices.Clone(c.folders.SkippedFiles),
			FrontmatterOptional: c.folders.FrontmatterOptional,
		},
	}
	for i, def := range c.types {
		out.Types[i] = TypeDefinition{
			Name:        def.name,
			Role:        def.role,
			Required:    slices.Clone(def.required),
			Fields:      slices.Clone(def.fields),
			SlugPattern: def.slugText,
		}
	}
	for i, enum := range c.enums {
		out.Enums[i] = EnumDefinition{Field: enum.field, Values: slices.Clone(enum.values)}
	}
	return out
}

// PlannedMarks returns detached copies of the heading and inline marks this
// contract uses to decide which broken links are tracked forward-references.
// A vault no contract governs returns nil lists: it declared no marks. A
// loaded contract that omitted the keys is filled with today's dialect
// defaults by the loader, and those are what this returns.
func (c *Contract) PlannedMarks() (heading, inline []string) {
	if c == nil {
		return nil, nil
	}
	return slices.Clone(c.rules.PlannedGapMarks),
		slices.Clone(c.rules.PlannedInlineMarks)
}

// ConceptType returns the note type a vault files as its distilled ideas, and
// whether this contract declares it at all. The name comes back either way, so
// a caller can say which type it looked for.
func (c *Contract) ConceptType() (name string, declared bool) {
	if c == nil {
		return conceptType, false
	}
	_, declared = c.typeByName[conceptType]
	return conceptType, declared
}

// LessonType returns the note type a vault files its course members as, and
// whether this contract declares it at all: the type that holds the lesson
// role. Without one the name comes back as the one a note calls itself by, so
// a caller can say which type it looked for.
func (c *Contract) LessonType() (name string, declared bool) {
	if c == nil {
		return lessonType, false
	}
	for _, def := range c.types {
		if def.role == RoleLesson {
			return def.name, true
		}
	}
	return lessonType, false
}

// RequiresFrontmatter reports whether this vault treats a note carrying no
// frontmatter block as a fault. Only a contract that wrote
// folders.frontmatter_optional down and set it against legality asks for that;
// silence is not a prohibition.
func (c *Contract) RequiresFrontmatter() bool {
	if c == nil {
		return false
	}
	return c.written.frontmatterOptional && !c.folders.FrontmatterOptional
}

// DeclaresType reports whether noteType is listed in the contract's own type
// vocabulary, so a face can decline a rule about a type this vault never named.
func (c *Contract) DeclaresType(noteType string) bool {
	if c == nil {
		return false
	}
	_, ok := c.typeByName[NormalizeWord(noteType)]
	return ok
}

// NavigationRoles returns the contract-derived navigation role capability.
func (c *Contract) NavigationRoles() NavigationRoles {
	if c == nil {
		return NavigationRoles{}
	}
	return c.navigationRoles
}

// KnowledgeScope returns the top-level directories this vault calls its
// knowledge layer.
func (c *Contract) KnowledgeScope() KnowledgeScope {
	if c == nil {
		return KnowledgeScope{}
	}
	return c.knowledgeScope
}

// SkipBasenames returns the note filenames this vault's scan skips, cloned so
// a caller cannot change the loaded contract. An ungoverned vault skips none.
func (c *Contract) SkipBasenames() []string {
	if c == nil {
		return nil
	}
	return slices.Clone(c.folders.SkippedFiles)
}

// SkipsBasename reports whether relPath's final segment is one of the
// filenames this vault's scan skips. The path is vault-relative and
// slash-form. An ungoverned vault skips none.
func (c *Contract) SkipsBasename(relPath string) bool {
	if c == nil || relPath == "" {
		return false
	}
	base := relPath
	if _, after, found := strings.CutLast(relPath, "/"); found {
		base = after
	}
	return slices.Contains(c.folders.SkippedFiles, base)
}

// ScanBoundary returns the directories this vault's scan keeps out, as the
// scan reads them. An ungoverned vault, or one that declares none, excludes
// nothing beyond the dot-prefixed names every scan skips.
func (c *Contract) ScanBoundary() vault.Boundary {
	if c == nil {
		return vault.Boundary{}
	}
	return vault.NewBoundary(c.folders.Ignored)
}

// ArtifactPolicy returns the contract-derived artifact policy capability.
func (c *Contract) ArtifactPolicy() ArtifactPolicy {
	if c == nil {
		return ArtifactPolicy{}
	}
	return c.artifactPolicy
}

// JournalDir returns the contract-derived journal directory capability.
func (c *Contract) JournalDir() JournalDir {
	if c == nil {
		return JournalDir{}
	}
	return c.journalDir
}

// PrivacyPolicy returns the contract-derived fail-closed egress capability.
func (c *Contract) PrivacyPolicy() PrivacyPolicy {
	if c == nil {
		return PrivacyPolicy{}
	}
	return c.privacyPolicy
}

// NormalizeWord is the one spelling rule for a word a contract declares —
// a type, any enum value. Every comparison against a declared word goes
// through it: the contract's own values once as they are loaded, and a note's
// value as it is judged. A word arrives decomposed from the filesystem and
// composed from an editor and the search index, so the bytes are folded before
// anything compares them. Case is deliberately not folded — two words
// differing only in case are two declarations the contract meant.
func NormalizeWord(s string) string {
	return vault.NormalizeNFC(s)
}
