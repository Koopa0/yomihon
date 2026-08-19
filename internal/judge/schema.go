package judge

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/koopa0/yomihon/internal/schema"
)

// checkSchema validates every knowledge note's frontmatter against the
// contract and returns the resulting findings, all at error severity. Scope
// follows the contract's own scan policy: only notes whose first path segment
// is a knowledge directory, and never a skipped basename. The findings are in
// per-note emission order; the deterministic total order is applied before
// they are written.
//
// The only failure is a slug pattern in the contract that is not a valid
// regular expression, which is a fault in the contract file rather than in
// any note.
func checkSchema(notes []note, contract *schema.Contract) ([]Finding, error) {
	definition := contract.Definition()
	slug, err := regexp.Compile(definition.Rules.SlugPattern)
	if err != nil {
		return nil, fmt.Errorf("compile slug pattern %q: %w", definition.Rules.SlugPattern, err)
	}
	policy := contractPolicy{requiresFrontmatter: contract.RequiresFrontmatter()}
	policy.inboxType, policy.inboxRequired, policy.inboxDeclared = contract.InboxRequiredFields()
	var out []Finding
	for i := range notes {
		n := &notes[i]
		seg := strings.Split(n.path, "/")
		inScope := slices.Contains(definition.Scan.KnowledgeDirs, seg[0])
		skipped := slices.Contains(definition.Scan.SkipBasenames, seg[len(seg)-1])
		if inScope && !skipped {
			out = append(out, lintNote(n, &definition, policy, seg, slug)...)
		}
	}
	return out, nil
}

// contractPolicy is the part of the contract the frontmatter rules ask for by
// name instead of spelling themselves, resolved once for a whole run: whether a
// note has to carry a frontmatter block at all, and what a capture of undecided
// shape is required to carry.
type contractPolicy struct {
	requiresFrontmatter bool
	inboxType           string
	inboxRequired       []string
	inboxDeclared       bool
}

// lintNote returns the frontmatter findings for one in-scope note, in the
// contract's reading order: the type enum, unknown keys, the optional article
// language, the lesson-only rules, then either the light system-document rules or the full
// knowledge-note rules. That order is the tiebreak the stable sort preserves
// among findings that share a path and rule id.
func lintNote(n *note, definition *schema.Definition, policy contractPolicy, seg []string, slug *regexp.Regexp) []Finding {
	if n.noFrontmatter {
		if policy.requiresFrontmatter {
			return []Finding{schemaFinding(n, "schema.frontmatter", "", false, "", "is missing")}
		}
		return nil
	}
	if n.badFrontmatter {
		return []Finding{schemaFinding(n, "schema.frontmatter", "", false, "", "is not valid YAML")}
	}

	var out []Finding
	ty, hasType := fmScalar(n.frontmatter, "type")
	if hasType && !slices.Contains(definition.Enums.Type, ty) {
		out = append(out, schemaFinding(n, "schema.enum", "type", true, ty, "is not an allowed type"))
	}

	isLesson := hasType && ty == "lesson"
	out = append(out, lintUnknownKeys(n, definition, isLesson)...)
	out = append(out, lintArticleLanguage(n, definition)...)
	if isLesson {
		out = append(out, lintLesson(n, slug)...)
	}

	// System documents (system / template / guide) carry only the light
	// status rule; the full knowledge-note rules below do not apply to them.
	if hasType && slices.Contains(definition.Fields.StatusGroup["system"], ty) {
		return append(out, lintSystemStatus(n, definition)...)
	}
	return append(out, lintKnowledge(n, definition, policy, seg)...)
}

func lintArticleLanguage(n *note, definition *schema.Definition) []Finding {
	if !slices.Contains(definition.Fields.Known, "lang") {
		return nil
	}
	value, ok := n.frontmatter["lang"]
	if !ok {
		return nil
	}
	if value.isList || !value.scalarIsString || value.scalar == "" {
		return []Finding{schemaFinding(n, "schema.language", "lang", true, value.scalar, "must be a non-empty BCP 47 language tag")}
	}
	if _, err := schema.ParseLanguageTag(value.scalar); err != nil {
		return []Finding{schemaFinding(n, "schema.language", "lang", true, value.scalar, "is not a valid BCP 47 language tag")}
	}
	return nil
}

// lintUnknownKeys reports every frontmatter key the contract does not list as
// known, in sorted key order. A lesson may additionally use the lesson-only
// keys.
func lintUnknownKeys(n *note, definition *schema.Definition, isLesson bool) []Finding {
	var out []Finding
	for _, key := range slices.Sorted(maps.Keys(n.frontmatter)) {
		known := slices.Contains(definition.Fields.Known, key) ||
			(isLesson && slices.Contains(definition.Fields.LessonOnly, key))
		if !known {
			out = append(out, schemaFinding(n, "schema.unknown_key", "", false, key, "is not a known field"))
		}
	}
	return out
}

// lintLesson reports the lesson-only slug faults. Status validation stays in
// lintKnowledge, which selects the configured status group exactly once.
func lintLesson(n *note, slug *regexp.Regexp) []Finding {
	var out []Finding
	switch sl, ok := fmScalar(n.frontmatter, "slug"); {
	case !ok:
		out = append(out, schemaFinding(n, "schema.required", "slug", true, "", "is required for a lesson"))
	case !slug.MatchString(sl):
		out = append(out, schemaFinding(n, "schema.slug", "slug", true, sl, "is not a valid slug"))
	}
	return out
}

// lintSystemStatus reports a system document's status outside the system
// status set.
func lintSystemStatus(n *note, definition *schema.Definition) []Finding {
	if st, ok := fmScalar(n.frontmatter, "status"); ok && !slices.Contains(definition.Enums.Status["system"], st) {
		return []Finding{schemaFinding(n, "schema.enum", "status", true, st, "is not a valid system status")}
	}
	return nil
}

// lintKnowledge reports the full knowledge-note rules, in reading order:
// required fields, the note status enum, the remaining value enums, then the
// structural rules.
func lintKnowledge(n *note, definition *schema.Definition, policy contractPolicy, seg []string) []Finding {
	var out []Finding
	out = append(out, lintRequired(n, definition, policy)...)
	group := configuredStatusGroup(definition, n.noteType)
	if st, ok := fmScalar(n.frontmatter, "status"); ok && !slices.Contains(definition.Enums.Status[group], st) {
		reason := "is not a valid status"
		if group != "note" {
			reason = "is not a valid " + group + " status"
		}
		out = append(out, schemaFinding(n, "schema.enum", "status", true, st, reason))
	}
	out = append(out, lintEnumFields(n, definition)...)
	out = append(out, lintStructural(n, definition, seg)...)
	return out
}

func configuredStatusGroup(definition *schema.Definition, noteType string) string {
	for group, noteTypes := range definition.Fields.StatusGroup {
		if slices.Contains(noteTypes, noteType) {
			return group
		}
	}
	return "note"
}

// lintRequired reports each required field that is absent or blank. A capture
// of undecided shape answers to the field set the contract declares for one, in
// place of the general set; where the contract declares none it answers to the
// general set with the domain requirement waived, since such a capture is not
// classified by knowledge domain. The same waiver covers the types the contract
// exempts (system docs, research briefs).
func lintRequired(n *note, definition *schema.Definition, policy contractPolicy) []Finding {
	ty, hasType := fmScalar(n.frontmatter, "type")
	isInbox := hasType && ty == policy.inboxType
	noDomain := isInbox || (hasType && slices.Contains(definition.Fields.DomainExempt, ty))
	required := definition.Fields.Required
	if isInbox && policy.inboxDeclared {
		// The declared set is the whole answer for a capture, so nothing is
		// waived out of it: a contract that lists domain there wants it.
		required = policy.inboxRequired
		noDomain = false
	}
	var out []Finding
	for _, key := range required {
		if key == "domain" && noDomain {
			continue
		}
		if v, ok := n.frontmatter[key]; !ok || !v.present() {
			out = append(out, schemaFinding(n, "schema.required", key, true, "", "is required"))
		}
	}
	return out
}

// lintEnumFields reports each of the remaining enum-valued fields whose value
// is outside its allowed set.
func lintEnumFields(n *note, definition *schema.Definition) []Finding {
	var out []Finding
	for _, ef := range []struct {
		field   string
		allowed []string
	}{
		{"domain", definition.Enums.Domain},
		{"source_kind", definition.Enums.SourceKind},
		{"source_provider", definition.Enums.SourceProvider},
		{"level", definition.Enums.Level},
		{"map_kind", definition.Enums.MapKind},
	} {
		if v, ok := fmScalar(n.frontmatter, ef.field); ok && !slices.Contains(ef.allowed, v) {
			out = append(out, schemaFinding(n, "schema.enum", ef.field, true, v, "is not an allowed value"))
		}
	}
	return out
}

// lintStructural reports the structural rules: a domain that does not match
// its folder, a slash-bearing legacy tag, and a concept missing provenance.
func lintStructural(n *note, definition *schema.Definition, seg []string) []Finding {
	var out []Finding
	// A domain must equal the first folder under the configured roots, e.g.
	// Concepts/<domain>/….
	if d, ok := fmScalar(n.frontmatter, "domain"); ok &&
		slices.Contains(definition.Rules.DomainEqualsFolderUnder, seg[0]) && len(seg) >= 3 && d != seg[1] {
		out = append(out, schemaFinding(n, "schema.domain_folder", "domain", true, d, "does not match its folder "+seg[1]))
	}
	if definition.Rules.ForbidTagWithSlash {
		if v, ok := n.frontmatter["tags"]; ok && v.isList {
			for _, tag := range v.list {
				if strings.Contains(tag, "/") {
					out = append(out, schemaFinding(n, "schema.legacy_tag", "tags", true, tag, "is a legacy tag (use a property)"))
				}
			}
		}
	}
	if ty, ok := fmScalar(n.frontmatter, "type"); ok && ty == "concept" &&
		!hasProvenance(n.frontmatter, definition.Rules.ConceptRequiresProvenance) {
		out = append(out, schemaFinding(n, "schema.provenance", "", false, "", "concept has neither based_on nor source_locator"))
	}
	return out
}

// fmScalar reports a field's value only when it is a non-empty scalar; a field
// that is absent, blank, or written as a list reads as unset.
func fmScalar(fm map[string]fmValue, key string) (string, bool) {
	v, ok := fm[key]
	if !ok {
		return "", false
	}
	text, ok := v.asScalar()
	if !ok || text == "" {
		return "", false
	}
	return text, true
}

// hasProvenance reports whether any of the provenance fields is filled in.
func hasProvenance(fm map[string]fmValue, fields []string) bool {
	for _, key := range fields {
		if v, ok := fm[key]; ok && v.present() {
			return true
		}
	}
	return false
}

// schemaFinding builds one frontmatter finding. The message reads "<field>
// <reason>" for a blank value and `<field> "<value>" <reason>` otherwise,
// where the field name falls back to "frontmatter" when the fault is not tied
// to a single field. The evidence, action, and source strings are fixed, and
// the fingerprint folds the field name and the violating value together so two
// blank-valued findings on one note stay distinct.
func schemaFinding(n *note, ruleID, field string, hasField bool, value, reason string) Finding {
	what := "frontmatter"
	if hasField {
		what = field
	}
	// The value is wrapped in literal double quotes and left otherwise
	// verbatim; the JSON encoder escapes those quotes when the line is
	// written, so the value's own bytes stay unchanged in the message.
	message := what + " " + reason
	if value != "" {
		message = what + ` "` + value + `" ` + reason
	}
	f := Finding{
		RuleID:          ruleID,
		Severity:        SeverityError,
		Path:            n.path,
		Message:         message,
		Evidence:        "frontmatter validated against vault-schema.toml",
		SuggestedAction: "fix the frontmatter to match the schema",
		SourceRule:      "vault-schema.toml",
		Fingerprint:     fingerprint(ruleID, n.path, field+"\x1f"+value),
	}
	if hasField {
		f.Field = new(field)
	}
	if value != "" {
		f.Target = new(value)
	}
	return f
}
