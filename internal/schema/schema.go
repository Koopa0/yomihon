// Package schema loads the vault's machine-readable contract
// (System/schemas/vault-schema.toml) and answers status state-machine
// questions.
//
// It is the only package in this repo allowed to read the contract:
// no other package may hardcode a second copy of any enum, field list, or
// lifecycle rule.
package schema

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/koopa0/yomihon/internal/vault"
)

const (
	// ContractRelPath is where the contract lives inside the vault.
	ContractRelPath  = "System/schemas/vault-schema.toml"
	supportedVersion = "1"

	// SealStatus is the status rendered as the koopa-only seal. It remains
	// pinned here until vault-schema.toml declares an explicit seal marker;
	// ownership alone is ambiguous because the published transition is also
	// koopa-only.
	SealStatus = "ready"
)

// Sentinel errors for state-machine answers. Callers match with errors.Is.
var (
	ErrUnknownStatus     = errors.New("status not defined for this type")
	ErrIllegalTransition = errors.New("transition not allowed by lifecycle")
	ErrOwnerForbidden    = errors.New("actor may not set this status")
)

// Contract is the validated, immutable vault authority. Its zero value carries
// no authority; load one with [Load], [LoadFile], or [LoadReader].
type Contract struct {
	version    string
	definition Definition
	stages     []Stage

	navigationRoles NavigationRoles
	artifactPolicy  ArtifactPolicy
	privacyPolicy   PrivacyPolicy
	metadata        contractMetadata

	statusGroupByType map[string]string
	statusesByGroup   map[string][]string
	stageByTypeStatus map[lifecycleKey]Stage
	lifecycleByType   map[string][]Stage
}

// Definition is a detached copy of the contract's declarative vocabulary and
// validation policy. Mutating it never changes the loaded [Contract].
type Definition struct {
	Enums  Enums      `toml:"enums"`
	Fields Fields     `toml:"fields"`
	Rules  Rules      `toml:"rules"`
	Scan   ScanPolicy `toml:"scan"`
}

type contractFile struct {
	Definition

	Version string `toml:"schema_version"`

	AlignedWith          string               `toml:"aligned_with"`
	GeneratedAtMustMatch bool                 `toml:"generated_at_must_match"`
	Navigation           toml.Primitive       `toml:"navigation"`
	Artifacts            toml.Primitive       `toml:"artifacts"`
	Privacy              toml.Primitive       `toml:"privacy"`
	Supersession         *supersessionSection `toml:"supersession"`
	Lifecycle            []rawLifecycleStage  `toml:"lifecycle"`
}

// contractMetadata retains machine-readable coordination and supersession
// vocabulary for its schema-owned validation and consumer work. Strict
// decoding must not silently discard authority that is not consumed yet.
type contractMetadata struct {
	alignedWith          string
	generatedAtMustMatch bool
	supersession         *supersessionSection
}

type navigationPrimitives struct {
	PathTypes toml.Primitive `toml:"path_types"`
	MapTypes  toml.Primitive `toml:"map_types"`
}

type artifactPrimitives struct {
	NonInstanceDirs toml.Primitive `toml:"non_instance_dirs"`
}

type privacyPrimitives struct {
	NeverEgressDirs toml.Primitive `toml:"never_egress_dirs"`
}

type contractUnknownKeys struct {
	core       []string
	navigation []string
	artifacts  []string
	privacy    []string
}

type supersessionSection struct {
	PredecessorField string `toml:"predecessor_field"`
	SuccessorField   string `toml:"successor_field"`
	GeneralLinkField string `toml:"general_link_field"`
	ArchivedStatus   string `toml:"archived_status"`
}

type rawLifecycleStage struct {
	Status    *string   `toml:"status"`
	AppliesTo *[]string `toml:"applies_to"`
	From      *[]string `toml:"from"`
	Owner     *[]string `toml:"owner"`
}

type lifecycleKey struct {
	noteType string
	status   string
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

// ScanPolicy is the checker's default scan policy (tool policy, not schema fact).
type ScanPolicy struct {
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

// Supersession names the frontmatter fields and archive status that form the
// vault's explicit replacement ledger.
type Supersession struct {
	PredecessorField string
	SuccessorField   string
	GeneralLinkField string
	ArchivedStatus   string
}

// Load reads the contract from the vault rooted at root.
func Load(root string) (*Contract, error) {
	return LoadFile(filepath.Join(root, filepath.FromSlash(ContractRelPath)))
}

// LoadFile reads the contract from an explicit path.
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

func decodeContract(data []byte, source policySource) (*Contract, error) {
	var decoded contractFile
	tomlMeta, err := toml.Decode(string(data), &decoded)
	if err != nil {
		return nil, err
	}
	contract := &Contract{
		version:    decoded.Version,
		definition: decoded.Definition,
		metadata: contractMetadata{
			alignedWith:          decoded.AlignedWith,
			generatedAtMustMatch: decoded.GeneratedAtMustMatch,
			supersession:         decoded.Supersession,
		},
	}
	if !tomlMeta.IsDefined("schema_version") {
		return nil, errors.New(`missing required key "schema_version"`)
	}
	if contract.version != supportedVersion {
		return nil, fmt.Errorf("unsupported schema_version %q; supported version is %q", contract.version, supportedVersion)
	}
	navigation, navigationTypeErrorKey := decodeNavigationSection(&tomlMeta, decoded.Navigation)
	artifacts, artifactTypeErrorKey := decodeArtifactSection(&tomlMeta, decoded.Artifacts)
	privacy, privacyTypeErrorKey := decodePrivacySection(&tomlMeta, decoded.Privacy)
	unknown := classifyUnknownKeys(tomlMeta.Undecoded())
	if len(unknown.core) > 0 {
		slices.Sort(unknown.core)
		return nil, fmt.Errorf("unknown core keys: %s", formatContractKeys(unknown.core))
	}
	if len(decoded.Lifecycle) == 0 {
		return nil, errors.New("no lifecycle stages")
	}
	contract.stages, err = decodeLifecycleStages(decoded.Lifecycle)
	if err != nil {
		return nil, err
	}
	if err := validateContractSemantics(contract); err != nil {
		return nil, err
	}
	contract.navigationRoles = resolveNavigationRoles(
		navigation,
		navigationTypeErrorKey,
		unknown.navigation,
		contract.definition.Enums.Type,
		&tomlMeta,
	)
	contract.artifactPolicy = resolveArtifactPolicy(
		artifacts,
		artifactTypeErrorKey,
		unknown.artifacts,
		source,
		&tomlMeta,
	)
	contract.privacyPolicy = resolvePrivacyPolicy(
		privacy,
		privacyTypeErrorKey,
		unknown.privacy,
		source,
		&tomlMeta,
	)
	return contract, nil
}

func decodeLifecycleStages(rows []rawLifecycleStage) ([]Stage, error) {
	stages := make([]Stage, len(rows))
	for i, row := range rows {
		ordinal := i + 1
		switch {
		case row.Status == nil:
			return nil, fmt.Errorf(`lifecycle row %d: missing required key "status"`, ordinal)
		case row.AppliesTo == nil:
			return nil, fmt.Errorf(`lifecycle row %d: missing required key "applies_to"`, ordinal)
		case row.From == nil:
			return nil, fmt.Errorf(`lifecycle row %d: missing required key "from"`, ordinal)
		case row.Owner == nil:
			return nil, fmt.Errorf(`lifecycle row %d: missing required key "owner"`, ordinal)
		}
		stages[i] = Stage{
			Status:    *row.Status,
			AppliesTo: slices.Clone(*row.AppliesTo),
			From:      slices.Clone(*row.From),
			Owner:     slices.Clone(*row.Owner),
		}
	}
	return stages, nil
}

func decodeNavigationSection(
	metadata *toml.MetaData,
	primitive toml.Primitive,
) (section *navigationSection, typeErrorKey string) {
	if !metadata.IsDefined("navigation") {
		return nil, ""
	}
	var fields navigationPrimitives
	if err := metadata.PrimitiveDecode(primitive, &fields); err != nil {
		return nil, "navigation"
	}
	section = &navigationSection{}
	if metadata.IsDefined("navigation", "path_types") {
		if err := metadata.PrimitiveDecode(fields.PathTypes, &section.PathTypes); err != nil {
			return nil, "navigation.path_types"
		}
	}
	if metadata.IsDefined("navigation", "map_types") {
		if err := metadata.PrimitiveDecode(fields.MapTypes, &section.MapTypes); err != nil {
			return nil, "navigation.map_types"
		}
	}
	return section, ""
}

func decodeArtifactSection(
	metadata *toml.MetaData,
	primitive toml.Primitive,
) (section *artifactSection, typeErrorKey string) {
	if !metadata.IsDefined("artifacts") {
		return nil, ""
	}
	var fields artifactPrimitives
	if err := metadata.PrimitiveDecode(primitive, &fields); err != nil {
		return nil, "artifacts"
	}
	section = &artifactSection{}
	if metadata.IsDefined("artifacts", "non_instance_dirs") {
		if err := metadata.PrimitiveDecode(fields.NonInstanceDirs, &section.NonInstanceDirs); err != nil {
			return nil, "artifacts.non_instance_dirs"
		}
	}
	return section, ""
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
		switch key[0] {
		case "navigation":
			unknown.navigation = append(unknown.navigation, key.String())
		case "artifacts":
			unknown.artifacts = append(unknown.artifacts, key.String())
		case "privacy":
			unknown.privacy = append(unknown.privacy, key.String())
		default:
			unknown.core = append(unknown.core, key.String())
		}
	}
	return unknown
}

func resolveNavigationRoles(
	section *navigationSection,
	typeErrorKey string,
	unknownKeys []string,
	enumTypes []string,
	metadata *toml.MetaData,
) NavigationRoles {
	if typeErrorKey != "" {
		return invalidNavigationRoles("key %q has incompatible TOML type", typeErrorKey)
	}
	if len(unknownKeys) > 0 {
		slices.Sort(unknownKeys)
		return invalidNavigationRoles("unknown keys %s", formatContractKeys(unknownKeys))
	}
	return deriveNavigationRoles(
		section,
		enumTypes,
		metadata.IsDefined("navigation", "path_types"),
		metadata.IsDefined("navigation", "map_types"),
	)
}

func resolveArtifactPolicy(
	section *artifactSection,
	typeErrorKey string,
	unknownKeys []string,
	source policySource,
	metadata *toml.MetaData,
) ArtifactPolicy {
	if typeErrorKey != "" {
		return ArtifactPolicy{state: &artifactPolicyState{
			diagnostic: fmt.Sprintf("invalid artifact policy: key %q has incompatible TOML type", typeErrorKey),
		}}
	}
	if len(unknownKeys) > 0 {
		slices.Sort(unknownKeys)
		return ArtifactPolicy{state: &artifactPolicyState{
			diagnostic: "invalid artifact policy: unknown keys " + formatContractKeys(unknownKeys),
		}}
	}
	return deriveArtifactPolicy(
		section,
		metadata.IsDefined("artifacts", "non_instance_dirs"),
		source,
	)
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
			diagnostic: fmt.Sprintf("invalid privacy policy: key %q has incompatible TOML type", typeErrorKey),
		}}
	}
	if len(unknownKeys) > 0 {
		slices.Sort(unknownKeys)
		return PrivacyPolicy{state: &privacyPolicyState{
			diagnostic: "invalid privacy policy: unknown keys " + formatContractKeys(unknownKeys),
		}}
	}
	return derivePrivacyPolicy(
		section,
		metadata.IsDefined("privacy", "never_egress_dirs"),
		source,
	)
}

func formatContractKeys(keys []string) string {
	quoted := make([]string, len(keys))
	for i, key := range keys {
		quoted[i] = strconv.Quote(key)
	}
	return strings.Join(quoted, ", ")
}

func validateContractSemantics(contract *Contract) error {
	if err := validateEnums(&contract.definition.Enums); err != nil {
		return err
	}
	if err := validateFields(&contract.definition.Enums, &contract.definition.Fields); err != nil {
		return err
	}
	if err := validateStatusGroups(&contract.definition.Enums, contract.definition.Fields.StatusGroup); err != nil {
		return err
	}
	if err := validateRules(&contract.definition.Enums, &contract.definition.Fields, &contract.definition.Rules); err != nil {
		return err
	}
	if err := validateScan(contract.definition.Scan); err != nil {
		return err
	}
	if err := compileLifecycle(contract); err != nil {
		return err
	}
	return validateSupersession(contract)
}

func validateEnums(enums *Enums) error {
	lists := []struct {
		name   string
		values []string
	}{
		{name: "enums.type", values: enums.Type},
		{name: "enums.domain", values: enums.Domain},
		{name: "enums.source_kind", values: enums.SourceKind},
		{name: "enums.source_provider", values: enums.SourceProvider},
		{name: "enums.level", values: enums.Level},
		{name: "enums.map_kind", values: enums.MapKind},
	}
	for _, list := range lists {
		if err := validateUniqueStrings(list.name, list.values); err != nil {
			return err
		}
	}

	groups := make([]string, 0, len(enums.Status))
	for group := range enums.Status {
		groups = append(groups, group)
	}
	slices.Sort(groups)
	for _, group := range groups {
		if group == "" {
			return errors.New("enums.status: empty group name")
		}
		if err := validateUniqueStrings(fmt.Sprintf("enums.status.%q", group), enums.Status[group]); err != nil {
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

func validateFields(enums *Enums, fields *Fields) error {
	lists := []struct {
		name   string
		values []string
	}{
		{name: "fields.required", values: fields.Required},
		{name: "fields.required_inbox", values: fields.RequiredInbox},
		{name: "fields.known", values: fields.Known},
		{name: "fields.lesson_only", values: fields.LessonOnly},
		{name: "fields.domain_exempt_types", values: fields.DomainExempt},
	}
	for _, list := range lists {
		if err := validateUniqueStrings(list.name, list.values); err != nil {
			return err
		}
	}

	known := stringSet(fields.Known)
	if err := validateSubset("fields.required", fields.Required, "fields.known", known); err != nil {
		return err
	}
	if err := validateSubset("fields.required_inbox", fields.RequiredInbox, "fields.known", known); err != nil {
		return err
	}

	lessonOnly := stringSet(fields.LessonOnly)
	for _, field := range fields.Known {
		if _, overlap := lessonOnly[field]; overlap {
			return fmt.Errorf("fields.known and fields.lesson_only overlap at %q", field)
		}
	}

	types := stringSet(enums.Type)
	if err := validateSubset("fields.domain_exempt_types", fields.DomainExempt, "enums.type", types); err != nil {
		return err
	}
	if len(fields.RequiredInbox) > 0 {
		if _, ok := types["inbox"]; !ok {
			return errors.New(`fields.required_inbox requires enums.type to contain "inbox"`)
		}
	}
	if len(fields.LessonOnly) > 0 {
		if _, ok := types["lesson"]; !ok {
			return errors.New(`fields.lesson_only requires enums.type to contain "lesson"`)
		}
	}
	return nil
}

func stringSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}

func validateSubset(name string, values []string, setName string, set map[string]struct{}) error {
	for _, value := range values {
		if _, ok := set[value]; !ok {
			return fmt.Errorf("%s: value %q is not listed in %s", name, value, setName)
		}
	}
	return nil
}

func validateStatusGroups(enums *Enums, statusGroups map[string][]string) error {
	if _, ok := enums.Status["note"]; !ok {
		return errors.New(`enums.status: missing required group "note"`)
	}

	groups := make([]string, 0, len(statusGroups))
	for group := range statusGroups {
		groups = append(groups, group)
	}
	slices.Sort(groups)

	types := stringSet(enums.Type)
	assigned := make(map[string]string)
	for _, group := range groups {
		if group == "" {
			return errors.New("fields.status_group: empty group name")
		}
		if _, ok := enums.Status[group]; !ok {
			return fmt.Errorf("fields.status_group: unknown status group %q", group)
		}
		name := fmt.Sprintf("fields.status_group.%q", group)
		mappedTypes := statusGroups[group]
		if err := validateUniqueStrings(name, mappedTypes); err != nil {
			return err
		}
		if err := validateSubset(name, mappedTypes, "enums.type", types); err != nil {
			return err
		}
		for _, noteType := range mappedTypes {
			if previous, exists := assigned[noteType]; exists {
				return fmt.Errorf(
					"fields.status_group: type %q is assigned to both %q and %q",
					noteType,
					previous,
					group,
				)
			}
			assigned[noteType] = group
		}
	}
	return nil
}

func validateRules(enums *Enums, fields *Fields, rules *Rules) error {
	if err := validateUniqueStrings("rules.domain_equals_folder_under", rules.DomainEqualsFolderUnder); err != nil {
		return err
	}
	for _, dir := range rules.DomainEqualsFolderUnder {
		if !validTopLevelComponent(dir) {
			return fmt.Errorf("rules.domain_equals_folder_under: unsafe top-level component %q", dir)
		}
	}

	if err := validateUniqueStrings("rules.concept_requires_provenance", rules.ConceptRequiresProvenance); err != nil {
		return err
	}
	types := stringSet(enums.Type)
	if _, hasConcept := types["concept"]; hasConcept && len(rules.ConceptRequiresProvenance) == 0 {
		return errors.New(`rules.concept_requires_provenance must not be empty when enums.type contains "concept"`)
	}
	known := stringSet(fields.Known)
	lessonOnly := stringSet(fields.LessonOnly)
	for _, field := range rules.ConceptRequiresProvenance {
		if _, excluded := lessonOnly[field]; excluded {
			return fmt.Errorf("rules.concept_requires_provenance: value %q is listed in fields.lesson_only", field)
		}
		if _, ok := known[field]; !ok {
			return fmt.Errorf("rules.concept_requires_provenance: value %q is not listed in fields.known", field)
		}
	}

	if _, hasLesson := types["lesson"]; hasLesson {
		if rules.SlugPattern == "" {
			return errors.New(`rules.slug_pattern must not be empty when enums.type contains "lesson"`)
		}
		if _, err := regexp.Compile(rules.SlugPattern); err != nil {
			return fmt.Errorf("rules.slug_pattern: invalid regular expression: %w", err)
		}
	}
	return nil
}

func validTopLevelComponent(value string) bool {
	return value != "" &&
		value != "." &&
		value != ".." &&
		!filepath.IsAbs(value) &&
		!strings.ContainsRune(value, 0) &&
		!strings.ContainsAny(value, `/\`)
}

func validateScan(scan ScanPolicy) error {
	if err := validateUniqueStrings("scan.knowledge_dirs", scan.KnowledgeDirs); err != nil {
		return err
	}
	for _, dir := range scan.KnowledgeDirs {
		if !validTopLevelComponent(dir) {
			return fmt.Errorf("scan.knowledge_dirs: unsafe top-level component %q", dir)
		}
	}

	if err := validateUniqueStrings("scan.skip_basenames", scan.SkipBasenames); err != nil {
		return err
	}
	for _, basename := range scan.SkipBasenames {
		if !validBasename(basename) {
			return fmt.Errorf("scan.skip_basenames: invalid basename %q", basename)
		}
	}
	return nil
}

func validBasename(value string) bool {
	return value != "" &&
		value != "." &&
		value != ".." &&
		!strings.ContainsRune(value, 0) &&
		!strings.ContainsAny(value, `/\`)
}

func compileLifecycle(contract *Contract) error {
	statusSets := compileLifecycleStatusLookups(contract)
	declaredTypes := stringSet(contract.definition.Enums.Type)
	contract.stageByTypeStatus = make(map[lifecycleKey]Stage)
	contract.lifecycleByType = make(map[string][]Stage, len(contract.definition.Enums.Type))
	rowByKey := make(map[lifecycleKey]int)
	for i := range contract.stages {
		row := i + 1
		stage := &contract.stages[i]
		applicableTypes, err := lifecycleApplicableTypes(contract, row, stage, declaredTypes)
		if err != nil {
			return err
		}
		for _, noteType := range applicableTypes {
			statusSet := statusSets[contract.statusGroupByType[noteType]]
			if err := validateLifecycleStageForType(row, stage, noteType, statusSet); err != nil {
				return err
			}
			if err := indexLifecycleStage(contract, rowByKey, row, noteType, stage); err != nil {
				return err
			}
		}
	}
	return nil
}

func compileLifecycleStatusLookups(contract *Contract) map[string]map[string]struct{} {
	contract.statusGroupByType = make(map[string]string, len(contract.definition.Enums.Type))
	for _, noteType := range contract.definition.Enums.Type {
		contract.statusGroupByType[noteType] = "note"
	}
	for group, noteTypes := range contract.definition.Fields.StatusGroup {
		for _, noteType := range noteTypes {
			contract.statusGroupByType[noteType] = group
		}
	}

	contract.statusesByGroup = make(map[string][]string, len(contract.definition.Enums.Status))
	statusSets := make(map[string]map[string]struct{}, len(contract.definition.Enums.Status))
	for group, statuses := range contract.definition.Enums.Status {
		contract.statusesByGroup[group] = slices.Clone(statuses)
		statusSets[group] = stringSet(statuses)
	}
	return statusSets
}

func lifecycleApplicableTypes(
	contract *Contract,
	row int,
	stage *Stage,
	declaredTypes map[string]struct{},
) ([]string, error) {
	if strings.TrimSpace(stage.Status) == "" {
		return nil, fmt.Errorf("lifecycle row %d: status must not be blank", row)
	}
	if len(stage.AppliesTo) == 0 {
		return nil, fmt.Errorf("lifecycle row %d: applies_to must not be empty", row)
	}
	if err := validateLifecycleValues(row, "applies_to", stage.AppliesTo, true); err != nil {
		return nil, err
	}
	if err := validateLifecycleValues(row, "from", stage.From, true); err != nil {
		return nil, err
	}
	if err := validateLifecycleValues(row, "owner", stage.Owner, false); err != nil {
		return nil, err
	}
	if len(stage.AppliesTo) == 1 && stage.AppliesTo[0] == "*" {
		return contract.definition.Enums.Type, nil
	}
	for _, noteType := range stage.AppliesTo {
		if _, ok := declaredTypes[noteType]; !ok {
			return nil, fmt.Errorf(
				"lifecycle row %d applies_to: type %q is not listed in enums.type",
				row,
				noteType,
			)
		}
	}
	return stage.AppliesTo, nil
}

func validateLifecycleStageForType(
	row int,
	stage *Stage,
	noteType string,
	statusSet map[string]struct{},
) error {
	if _, ok := statusSet[stage.Status]; !ok {
		return fmt.Errorf(
			"lifecycle row %d: status %q is not legal for type %q",
			row,
			stage.Status,
			noteType,
		)
	}
	if len(stage.From) == 1 && stage.From[0] == "*" {
		return nil
	}
	for _, predecessor := range stage.From {
		if _, ok := statusSet[predecessor]; !ok {
			return fmt.Errorf(
				"lifecycle row %d from: status %q is not legal for type %q",
				row,
				predecessor,
				noteType,
			)
		}
	}
	return nil
}

func indexLifecycleStage(
	contract *Contract,
	rowByKey map[lifecycleKey]int,
	row int,
	noteType string,
	stage *Stage,
) error {
	key := lifecycleKey{noteType: noteType, status: stage.Status}
	if previous, overlap := rowByKey[key]; overlap {
		return fmt.Errorf(
			"lifecycle rows %d and %d overlap for type %q and status %q",
			previous,
			row,
			noteType,
			stage.Status,
		)
	}
	rowByKey[key] = row
	compiled := cloneStage(stage)
	contract.stageByTypeStatus[key] = compiled
	contract.lifecycleByType[noteType] = append(contract.lifecycleByType[noteType], cloneStage(&compiled))
	return nil
}

func validateLifecycleValues(row int, field string, values []string, wildcard bool) error {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("lifecycle row %d %s: blank value", row, field)
		}
		if _, duplicate := seen[value]; duplicate {
			return fmt.Errorf("lifecycle row %d %s: duplicate value %q", row, field, value)
		}
		seen[value] = struct{}{}
	}
	if wildcard && len(values) > 1 && slices.Contains(values, "*") {
		return fmt.Errorf(`lifecycle row %d %s: wildcard "*" must be the only value`, row, field)
	}
	return nil
}

func cloneStage(stage *Stage) Stage {
	return Stage{
		Status:    stage.Status,
		AppliesTo: slices.Clone(stage.AppliesTo),
		From:      slices.Clone(stage.From),
		Owner:     slices.Clone(stage.Owner),
	}
}

func validateSupersession(contract *Contract) error {
	section := contract.metadata.supersession
	if section == nil {
		return nil
	}
	if err := validateSupersessionFields(contract, section); err != nil {
		return err
	}
	return validateSupersessionLifecycle(contract, section.ArchivedStatus)
}

func validateSupersessionFields(contract *Contract, section *supersessionSection) error {
	values := []struct {
		name  string
		value string
	}{
		{name: "predecessor_field", value: section.PredecessorField},
		{name: "successor_field", value: section.SuccessorField},
		{name: "general_link_field", value: section.GeneralLinkField},
		{name: "archived_status", value: section.ArchivedStatus},
	}
	for _, field := range values {
		if field.value == "" {
			return fmt.Errorf("supersession.%s must not be empty", field.name)
		}
	}

	switch {
	case section.PredecessorField == section.SuccessorField:
		return errors.New(`supersession fields "predecessor_field" and "successor_field" must be distinct`)
	case section.PredecessorField == section.GeneralLinkField:
		return errors.New(`supersession fields "predecessor_field" and "general_link_field" must be distinct`)
	case section.SuccessorField == section.GeneralLinkField:
		return errors.New(`supersession fields "successor_field" and "general_link_field" must be distinct`)
	}

	lessonOnly := stringSet(contract.definition.Fields.LessonOnly)
	if _, ok := lessonOnly[section.PredecessorField]; !ok {
		return fmt.Errorf(
			"supersession.predecessor_field: value %q is not listed in fields.lesson_only",
			section.PredecessorField,
		)
	}
	if _, ok := lessonOnly[section.SuccessorField]; !ok {
		return fmt.Errorf(
			"supersession.successor_field: value %q is not listed in fields.lesson_only",
			section.SuccessorField,
		)
	}
	known := stringSet(contract.definition.Fields.Known)
	if _, ok := known[section.GeneralLinkField]; !ok {
		return fmt.Errorf(
			"supersession.general_link_field: value %q is not listed in fields.known",
			section.GeneralLinkField,
		)
	}
	return nil
}

func validateSupersessionLifecycle(contract *Contract, archivedStatus string) error {
	archivedStatusKnown := false
	for _, statuses := range contract.definition.Enums.Status {
		if slices.Contains(statuses, archivedStatus) {
			archivedStatusKnown = true
			break
		}
	}
	if !archivedStatusKnown {
		return fmt.Errorf(
			"supersession.archived_status: value %q is not listed in enums.status",
			archivedStatus,
		)
	}
	for _, noteType := range contract.definition.Enums.Type {
		if err := validateSupersessionType(contract, noteType, archivedStatus); err != nil {
			return err
		}
	}
	return nil
}

func validateSupersessionType(contract *Contract, noteType, archivedStatus string) error {
	group := contract.statusGroupByType[noteType]
	statuses := contract.statusesByGroup[group]
	if !slices.Contains(statuses, archivedStatus) {
		return fmt.Errorf(
			"supersession.archived_status: value %q is not legal for type %q",
			archivedStatus,
			noteType,
		)
	}
	archived, ok := contract.stageByTypeStatus[lifecycleKey{
		noteType: noteType,
		status:   archivedStatus,
	}]
	if !ok {
		return fmt.Errorf(
			"supersession.archived_status: no lifecycle row applies to type %q and status %q",
			noteType,
			archivedStatus,
		)
	}
	if slices.Contains(archived.From, "*") {
		return nil
	}
	for _, predecessor := range statuses {
		if predecessor == archivedStatus {
			continue
		}
		if !slices.Contains(archived.From, predecessor) {
			return fmt.Errorf(
				"supersession archived stage for type %q does not accept predecessor %q",
				noteType,
				predecessor,
			)
		}
	}
	return nil
}

// Version returns the contract format version.
func (c *Contract) Version() string {
	return c.version
}

// Definition returns a detached copy of the contract's declarative
// vocabulary and validation policy.
func (c *Contract) Definition() Definition {
	return cloneDefinition(&c.definition)
}

// StageCount returns the number of lifecycle rows declared by the contract.
func (c *Contract) StageCount() int {
	return len(c.stages)
}

func cloneDefinition(source *Definition) Definition {
	return Definition{
		Enums: Enums{
			Type:           slices.Clone(source.Enums.Type),
			Domain:         slices.Clone(source.Enums.Domain),
			SourceKind:     slices.Clone(source.Enums.SourceKind),
			SourceProvider: slices.Clone(source.Enums.SourceProvider),
			Level:          slices.Clone(source.Enums.Level),
			MapKind:        slices.Clone(source.Enums.MapKind),
			Status:         cloneStringSliceMap(source.Enums.Status),
		},
		Fields: Fields{
			Required:      slices.Clone(source.Fields.Required),
			RequiredInbox: slices.Clone(source.Fields.RequiredInbox),
			DomainExempt:  slices.Clone(source.Fields.DomainExempt),
			Known:         slices.Clone(source.Fields.Known),
			LessonOnly:    slices.Clone(source.Fields.LessonOnly),
			StatusGroup:   cloneStringSliceMap(source.Fields.StatusGroup),
		},
		Rules: Rules{
			DomainEqualsFolderUnder:   slices.Clone(source.Rules.DomainEqualsFolderUnder),
			ConceptRequiresProvenance: slices.Clone(source.Rules.ConceptRequiresProvenance),
			SlugPattern:               source.Rules.SlugPattern,
			ForbidTagWithSlash:        source.Rules.ForbidTagWithSlash,
		},
		Scan: ScanPolicy{
			KnowledgeDirs:        slices.Clone(source.Scan.KnowledgeDirs),
			SkipBasenames:        slices.Clone(source.Scan.SkipBasenames),
			NoFrontmatterIsLegal: source.Scan.NoFrontmatterIsLegal,
		},
	}
}

func cloneStringSliceMap(source map[string][]string) map[string][]string {
	if source == nil {
		return nil
	}
	cloned := make(map[string][]string, len(source))
	for key, values := range source {
		cloned[key] = slices.Clone(values)
	}
	return cloned
}

// Supersession returns the configured replacement-ledger vocabulary, or false
// when the contract declares none.
func (c *Contract) Supersession() (Supersession, bool) {
	section := c.metadata.supersession
	if section == nil {
		return Supersession{}, false
	}
	return Supersession{
		PredecessorField: section.PredecessorField,
		SuccessorField:   section.SuccessorField,
		GeneralLinkField: section.GeneralLinkField,
		ArchivedStatus:   section.ArchivedStatus,
	}, true
}

// NavigationRoles returns the contract-derived navigation role capability.
func (c *Contract) NavigationRoles() NavigationRoles {
	return c.navigationRoles
}

// ArtifactPolicy returns the contract-derived artifact policy capability.
func (c *Contract) ArtifactPolicy() ArtifactPolicy {
	return c.artifactPolicy
}

// PrivacyPolicy returns the contract-derived fail-closed egress capability.
func (c *Contract) PrivacyPolicy() PrivacyPolicy {
	return c.privacyPolicy
}

// StatusGroup returns which status enum group applies to a declared note type.
// An empty note type selects the default "note" group for aggregate views.
// An undeclared non-empty type returns "".
func (c *Contract) StatusGroup(noteType string) string {
	if c.version == "" {
		return ""
	}
	if noteType == "" {
		return "note"
	}
	if group, declared := c.statusGroupByType[noteType]; declared {
		return group
	}
	return ""
}

// Statuses returns the legal status values for a declared note type. An empty
// note type selects the default "note" group; an undeclared type returns nil.
func (c *Contract) Statuses(noteType string) []string {
	return slices.Clone(c.statusesByGroup[c.StatusGroup(noteType)])
}

// Stage returns the lifecycle entry for setting status on a note of the
// given type, or false when the contract defines none.
func (c *Contract) Stage(noteType, status string) (Stage, bool) {
	stage, ok := c.stage(noteType, status)
	if !ok {
		return Stage{}, false
	}
	return cloneStage(&stage), true
}

func (c *Contract) stage(noteType, status string) (Stage, bool) {
	stage, ok := c.stageByTypeStatus[lifecycleKey{noteType: noteType, status: status}]
	return stage, ok
}

// Transition reports whether actor may move a note of the given type from
// one status to another. An empty from means the note is being given its
// initial status. The returned error wraps one of the package sentinels.
func (c *Contract) Transition(noteType, from, to, actor string) error {
	st, ok := c.stage(noteType, to)
	if !ok {
		return fmt.Errorf("%w: %q for type %q", ErrUnknownStatus, to, noteType)
	}
	if from != "" {
		group := c.statusGroupByType[noteType]
		if !slices.Contains(c.statusesByGroup[group], from) {
			return fmt.Errorf("%w: current status %q for type %q", ErrUnknownStatus, from, noteType)
		}
	}
	if from == to {
		return fmt.Errorf("%w: %s → %s for type %q is a no-op", ErrIllegalTransition, from, to, noteType)
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

// AdvanceableBy reports whether actor may move a note of the given type one
// concrete step onward from status: some lifecycle entry names status as a
// specific predecessor in its from list and grants actor the ownership to make
// that move.
//
// It differs from Transition in two deliberate ways, matching the question of
// whether a note still awaits a decision from actor. It ignores every edge
// whose predecessor list is the wildcard rather than a named predecessor, so a
// match requires status to be listed literally. And it reports only whether one
// such owned edge exists, not whether a specific target is reachable.
func (c *Contract) AdvanceableBy(noteType, status, actor string) bool {
	for _, st := range c.lifecycleByType[noteType] {
		if !slices.Contains(st.From, status) {
			continue
		}
		if slices.Contains(st.Owner, actor) {
			return true
		}
	}
	return false
}
