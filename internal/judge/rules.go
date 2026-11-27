package judge

import "github.com/koopa0/yomihon/internal/sequence"

// The rules this package decides for itself. The course grammar names its own
// rules, and ruleIDs holds both halves.
const (
	// RuleLinkBroken reports a wikilink that resolves to no note.
	RuleLinkBroken RuleID = "link.broken"
	// RuleLinkTitleNotAlias reports a wikilink that names a note's title but
	// not its filename or an alias.
	RuleLinkTitleNotAlias RuleID = "link.title_not_alias"
	// RuleLinkBrokenPath reports a path reference that names no file, or that
	// leaves the vault.
	RuleLinkBrokenPath RuleID = "link.broken.path"
	// RuleLinkBrokenImage reports an image or an embed that names no file.
	RuleLinkBrokenImage RuleID = "link.broken.image"
	// RuleLinkUnusedDefinition reports a link reference definition nothing in
	// its note uses.
	RuleLinkUnusedDefinition RuleID = "link.unused.definition"
	// RuleLinkSectionMissing reports a link whose note has no such section.
	RuleLinkSectionMissing RuleID = "link.section_missing"
	// RuleLinkBlockMissing reports a link whose note has no such block address.
	RuleLinkBlockMissing RuleID = "link.block_missing"
	// RuleEmbedSectionMissing reports an embed whose note has no such section.
	RuleEmbedSectionMissing RuleID = "embed.section_missing"
	// RuleEmbedBlockMissing reports an embed whose note has no such block
	// address.
	RuleEmbedBlockMissing RuleID = "embed.block_missing"
	// RuleCollisionAlias reports an alias more than one note declares.
	RuleCollisionAlias RuleID = "collision.alias"
	// RuleCollisionName reports a name more than one file answers to.
	RuleCollisionName RuleID = "collision.name"
	// RuleProvenanceUnresolved reports a based_on or related reference that
	// resolves to nothing.
	RuleProvenanceUnresolved RuleID = "provenance.unresolved"
	// RuleProvenanceAliasResolved reports a thought's based_on that reaches its
	// note only through an alias.
	RuleProvenanceAliasResolved RuleID = "provenance.alias_resolved"
	// RuleCourseDiskMismatch reports a course entry that resolves to nothing.
	RuleCourseDiskMismatch RuleID = "course.disk_mismatch"
	// RuleCourseDiskUnlisted reports a unit no course of its domain lists.
	RuleCourseDiskUnlisted RuleID = "course.disk_unlisted"
	// RuleCalloutTitleMarkup reports a callout title whose markup the page
	// shows as text.
	RuleCalloutTitleMarkup RuleID = "callout.title_markup"
	// RuleScanSkipped reports a path the scan saw that is not a regular file.
	RuleScanSkipped RuleID = "scan.skipped"
	// RuleScanUnreadable reports a file the scan listed and the read could not
	// open.
	RuleScanUnreadable RuleID = "scan.unreadable"
	// RuleSchemaFrontmatter reports frontmatter that is missing where the rules
	// file wants it, is not YAML, or never closes.
	RuleSchemaFrontmatter RuleID = "schema.frontmatter"
	// RuleSchemaEnum reports a value outside the vocabulary the rules file
	// states.
	RuleSchemaEnum RuleID = "schema.enum"
	// RuleSchemaRequired reports a required field that is absent or blank.
	RuleSchemaRequired RuleID = "schema.required"
	// RuleSchemaUnknownKey reports a field the rules file does not let the note
	// carry.
	RuleSchemaUnknownKey RuleID = "schema.unknown_key"
	// RuleSchemaTypeDependent reports fields only some type takes, on a note
	// whose type is not valid.
	RuleSchemaTypeDependent RuleID = "schema.type_dependent"
	// RuleSchemaSlug reports a unit slug its type's pattern does not match.
	RuleSchemaSlug RuleID = "schema.slug"
	// RuleSchemaProvenance reports a concept with no provenance.
	RuleSchemaProvenance RuleID = "schema.provenance"
	// RuleSchemaReferenceNestedSequence reports a reference field holding a
	// nested YAML list.
	RuleSchemaReferenceNestedSequence RuleID = "schema.reference_nested_sequence"
	// RuleSchemaLanguage reports a lang value that is not a BCP 47 tag.
	RuleSchemaLanguage RuleID = "schema.language"
	// RuleSchemaDraft reports a draft flag that is not true or false.
	RuleSchemaDraft RuleID = "schema.draft"
	// RuleSchemaUnmatchedKnowledgeDir reports a knowledge directory the rules
	// file declares and nothing in the vault answers to.
	RuleSchemaUnmatchedKnowledgeDir RuleID = "schema.unmatched_knowledge_dir"
	// RuleSchemaNavigationUnavailable reports a rules file whose type roles
	// could not be used.
	RuleSchemaNavigationUnavailable RuleID = "schema.navigation_unavailable"
)

// ruleIDs is every rule a finding can carry, the rules above and then the
// course grammar's, asked of the grammar so a rule it gains can be denied by
// name. A --deny value is validated against it, so a typo fails loudly instead
// of disabling the gate.
var ruleIDs = append([]RuleID{
	RuleLinkBroken,
	RuleLinkTitleNotAlias,
	RuleLinkBrokenPath,
	RuleLinkBrokenImage,
	RuleLinkUnusedDefinition,
	RuleLinkSectionMissing,
	RuleLinkBlockMissing,
	RuleEmbedSectionMissing,
	RuleEmbedBlockMissing,
	RuleCollisionAlias,
	RuleCollisionName,
	RuleProvenanceUnresolved,
	RuleProvenanceAliasResolved,
	RuleCourseDiskMismatch,
	RuleCourseDiskUnlisted,
	RuleCalloutTitleMarkup,
	RuleScanSkipped,
	RuleScanUnreadable,
	RuleSchemaFrontmatter,
	RuleSchemaEnum,
	RuleSchemaRequired,
	RuleSchemaUnknownKey,
	RuleSchemaTypeDependent,
	RuleSchemaSlug,
	RuleSchemaProvenance,
	RuleSchemaReferenceNestedSequence,
	RuleSchemaLanguage,
	RuleSchemaDraft,
	RuleSchemaUnmatchedKnowledgeDir,
	RuleSchemaNavigationUnavailable,
}, courseRules()...)

// courseRules are the course grammar's rules in this package's type.
func courseRules() []RuleID {
	rules := sequence.Rules()
	ids := make([]RuleID, len(rules))
	for i, rule := range rules {
		ids[i] = RuleID(rule)
	}
	return ids
}
