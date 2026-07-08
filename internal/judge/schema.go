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
func checkSchema(notes []note, s *schema.Schema) ([]Finding, error) {
	slug, err := regexp.Compile(s.Rules.SlugPattern)
	if err != nil {
		return nil, fmt.Errorf("compile slug pattern %q: %w", s.Rules.SlugPattern, err)
	}
	var out []Finding
	for i := range notes {
		n := &notes[i]
		seg := strings.Split(n.path, "/")
		inScope := slices.Contains(s.Scan.KnowledgeDirs, seg[0])
		skipped := slices.Contains(s.Scan.SkipBasenames, seg[len(seg)-1])
		if inScope && !skipped {
			out = append(out, lintNote(n, s, seg, slug)...)
		}
	}
	return out, nil
}

// lintNote returns the frontmatter findings for one in-scope note, in the
// contract's reading order: the type enum, unknown keys, the lesson-only
// rules, then either the light system-document rules or the full
// knowledge-note rules. That order is the tiebreak the stable sort preserves
// among findings that share a path and rule id.
func lintNote(n *note, s *schema.Schema, seg []string, slug *regexp.Regexp) []Finding {
	if n.noFrontmatter {
		return nil
	}
	if n.badFrontmatter {
		return []Finding{schemaFinding(n, "schema.frontmatter", "", false, "", "is not valid YAML")}
	}

	var out []Finding
	ty, hasType := fmScalar(n.frontmatter, "type")
	if hasType && !slices.Contains(s.Enums.Type, ty) {
		out = append(out, schemaFinding(n, "schema.enum", "type", true, ty, "is not an allowed type"))
	}

	isLesson := hasType && ty == "lesson"
	out = append(out, lintUnknownKeys(n, s, isLesson)...)
	if isLesson {
		out = append(out, lintLesson(n, s, slug)...)
	}

	// System documents (system / template / guide) carry only the light
	// status rule; the full knowledge-note rules below do not apply to them.
	if hasType && slices.Contains(s.Fields.StatusGroup["system"], ty) {
		return append(out, lintSystemStatus(n, s)...)
	}
	return append(out, lintKnowledge(n, s, seg)...)
}

// lintUnknownKeys reports every frontmatter key the contract does not list as
// known, in sorted key order. A lesson may additionally use the lesson-only
// keys.
func lintUnknownKeys(n *note, s *schema.Schema, isLesson bool) []Finding {
	var out []Finding
	for _, key := range slices.Sorted(maps.Keys(n.frontmatter)) {
		known := slices.Contains(s.Fields.Known, key) ||
			(isLesson && slices.Contains(s.Fields.LessonOnly, key))
		if !known {
			out = append(out, schemaFinding(n, "schema.unknown_key", "", false, key, "is not a known field"))
		}
	}
	return out
}

// lintLesson reports the lesson-only faults: a missing or malformed slug, and
// a status outside the lesson status set.
func lintLesson(n *note, s *schema.Schema, slug *regexp.Regexp) []Finding {
	var out []Finding
	switch sl, ok := fmScalar(n.frontmatter, "slug"); {
	case !ok:
		out = append(out, schemaFinding(n, "schema.required", "slug", true, "", "is required for a lesson"))
	case !slug.MatchString(sl):
		out = append(out, schemaFinding(n, "schema.slug", "slug", true, sl, "is not a valid slug"))
	}
	if st, ok := fmScalar(n.frontmatter, "status"); ok && !slices.Contains(s.Enums.Status["lesson"], st) {
		out = append(out, schemaFinding(n, "schema.enum", "status", true, st, "is not a valid lesson status"))
	}
	return out
}

// lintSystemStatus reports a system document's status outside the system
// status set.
func lintSystemStatus(n *note, s *schema.Schema) []Finding {
	if st, ok := fmScalar(n.frontmatter, "status"); ok && !slices.Contains(s.Enums.Status["system"], st) {
		return []Finding{schemaFinding(n, "schema.enum", "status", true, st, "is not a valid system status")}
	}
	return nil
}

// lintKnowledge reports the full knowledge-note rules, in reading order:
// required fields, the note status enum, the remaining value enums, then the
// structural rules.
func lintKnowledge(n *note, s *schema.Schema, seg []string) []Finding {
	var out []Finding
	out = append(out, lintRequired(n, s)...)
	if st, ok := fmScalar(n.frontmatter, "status"); ok && !slices.Contains(s.Enums.Status["note"], st) {
		out = append(out, schemaFinding(n, "schema.enum", "status", true, st, "is not a valid status"))
	}
	out = append(out, lintEnumFields(n, s)...)
	out = append(out, lintStructural(n, s, seg)...)
	return out
}

// lintRequired reports each required field that is absent or blank. The domain
// requirement is waived for undefined-shape inbox notes and for the types the
// contract exempts (system docs, research briefs), which are not classified by
// knowledge domain.
func lintRequired(n *note, s *schema.Schema) []Finding {
	ty, hasType := fmScalar(n.frontmatter, "type")
	noDomain := hasType && (ty == "inbox" || slices.Contains(s.Fields.DomainExempt, ty))
	var out []Finding
	for _, key := range s.Fields.Required {
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
func lintEnumFields(n *note, s *schema.Schema) []Finding {
	var out []Finding
	for _, ef := range []struct {
		field   string
		allowed []string
	}{
		{"domain", s.Enums.Domain},
		{"source_kind", s.Enums.SourceKind},
		{"source_provider", s.Enums.SourceProvider},
		{"level", s.Enums.Level},
		{"map_kind", s.Enums.MapKind},
	} {
		if v, ok := fmScalar(n.frontmatter, ef.field); ok && !slices.Contains(ef.allowed, v) {
			out = append(out, schemaFinding(n, "schema.enum", ef.field, true, v, "is not an allowed value"))
		}
	}
	return out
}

// lintStructural reports the structural rules: a domain that does not match
// its folder, a slash-bearing legacy tag, and a concept missing provenance.
func lintStructural(n *note, s *schema.Schema, seg []string) []Finding {
	var out []Finding
	// A domain must equal the first folder under the configured roots, e.g.
	// Concepts/<domain>/….
	if d, ok := fmScalar(n.frontmatter, "domain"); ok &&
		slices.Contains(s.Rules.DomainEqualsFolderUnder, seg[0]) && len(seg) >= 3 && d != seg[1] {
		out = append(out, schemaFinding(n, "schema.domain_folder", "domain", true, d, "does not match its folder "+seg[1]))
	}
	if s.Rules.ForbidTagWithSlash {
		if v, ok := n.frontmatter["tags"]; ok && v.isList {
			for _, tag := range v.list {
				if strings.Contains(tag, "/") {
					out = append(out, schemaFinding(n, "schema.legacy_tag", "tags", true, tag, "is a legacy tag (use a property)"))
				}
			}
		}
	}
	if ty, ok := fmScalar(n.frontmatter, "type"); ok && ty == "concept" &&
		!hasProvenance(n.frontmatter, s.Rules.ConceptRequiresProvenance) {
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
