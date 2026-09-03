package judge

import (
	"cmp"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vaultfs"
)

// checkSchema validates every knowledge note's frontmatter against the
// contract, all findings at error severity. Scope follows the contract's own
// scan policy, and a contract declaring no knowledge directory lints nothing:
// linting every file would hold notes to rules their author never claimed. The
// only failure is a slug pattern that is not a valid regular expression.
func checkSchema(notes []note, contract *schema.Contract) ([]Finding, error) {
	run, err := newLintRun(contract)
	if err != nil {
		return nil, err
	}
	var out []Finding
	for i := range notes {
		n := &notes[i]
		seg := strings.Split(n.path, "/")
		inScope := slices.ContainsFunc(run.definition.Scan.KnowledgeDirs, func(dir string) bool {
			return schema.SameDirName(seg[0], dir)
		})
		skipped := slices.Contains(run.definition.Scan.SkipBasenames, seg[len(seg)-1])
		if inScope && !skipped {
			out = append(out, run.note(n, seg)...)
		}
	}
	return out, nil
}

// checkKnowledgeScope reports each knowledge directory the contract declares
// that nothing in this vault answers to. Such an entry turns the frontmatter
// rules off for the ground its author meant to govern while every gate stays
// green. A declaration is answered by the directory itself or by any file
// below it, so an emptied inbox is still the folder the contract named.
func checkKnowledgeScope(scan vaultfs.Scan, contract *schema.Contract) []Finding {
	declared := contract.Definition().Scan.KnowledgeDirs
	if len(declared) == 0 {
		return nil
	}
	files := scan.Files()
	occupied := make([]string, 0, len(files))
	for _, entry := range files {
		top, _, _ := strings.Cut(entry.Path(), "/")
		if !slices.Contains(occupied, top) {
			occupied = append(occupied, top)
		}
	}
	var out []Finding
	for _, dir := range declared {
		if scan.Contains(dir) ||
			slices.ContainsFunc(occupied, func(top string) bool { return schema.SameDirName(top, dir) }) {
			continue
		}
		out = append(out, unmatchedKnowledgeDir(dir))
	}
	return out
}

// unmatchedKnowledgeDir builds one finding for a declared knowledge directory
// nothing on disk answers to. The finding carries the declared value as its
// path, so the privacy policy is asked about that same value; the fingerprint
// keys on it alone, the fault being one line of the contract.
func unmatchedKnowledgeDir(dir string) Finding {
	return Finding{
		RuleID:          "schema.unmatched_knowledge_dir",
		Severity:        SeverityError,
		Path:            dir,
		Message:         "knowledge directory \"" + dir + "\" matches nothing in this vault, so the frontmatter rules reach nothing there",
		Evidence:        "the vault contract declares this directory under scan.knowledge_dirs and the scan observed neither that directory nor any file below one of that name",
		SuggestedAction: "correct the spelling in the vault contract, or drop the entry if the directory is gone",
		SourceRule:      sourceContractScan,
		Target:          new(dir),
		Fingerprint:     fingerprint("schema.unmatched_knowledge_dir", "", dir),
	}
}

// systemDocumentGroup is the status group holding a vault's own working
// documents rather than knowledge it wrote. Membership is the contract's
// answer; only the group's name is this face's, because nothing in the
// contract marks a group as holding documents. A vault filing its documents
// under another name has the full knowledge-note rules applied to them.
const systemDocumentGroup = "system"

// lintRun is one contract resolved into everything the frontmatter rules read,
// held together for the length of one scan. Every field is derived from the
// contract exactly once, and they travel together because they have to agree:
// passed separately, a caller could mix them from two different contracts.
type lintRun struct {
	contract            *schema.Contract
	definition          schema.Definition
	slug                *regexp.Regexp
	requiresFrontmatter bool
	inboxType           string
	inboxRequired       []string
	inboxDeclared       bool
	conceptType         string
	conceptDeclared     bool
}

// newLintRun resolves a contract into the run the rules read from. The only
// failure is a slug pattern the contract declares that is not a valid regular
// expression, which is a fault in the contract file rather than in any note.
func newLintRun(contract *schema.Contract) (*lintRun, error) {
	run := &lintRun{
		contract:            contract,
		definition:          contract.Definition(),
		requiresFrontmatter: contract.RequiresFrontmatter(),
	}
	slug, err := regexp.Compile(run.definition.Rules.SlugPattern)
	if err != nil {
		return nil, fmt.Errorf("compile slug pattern %q: %w", run.definition.Rules.SlugPattern, err)
	}
	run.slug = slug
	run.inboxType, run.inboxRequired, run.inboxDeclared = contract.InboxRequiredFields()
	run.conceptType, run.conceptDeclared = contract.ConceptType()
	return run, nil
}

// note returns the frontmatter findings for one in-scope note, in the
// contract's reading order: the type enum, unknown keys, the article language,
// the lesson-only rules, then either the light document rules or the full
// knowledge-note rules. That order is the tiebreak the stable sort preserves.
func (r *lintRun) note(n *note, seg []string) []Finding {
	if n.noFrontmatter {
		if r.requiresFrontmatter {
			return []Finding{schemaFinding(n, "schema.frontmatter", "", false, "", "is missing")}
		}
		return nil
	}
	if n.badFrontmatter {
		return []Finding{schemaFinding(n, "schema.frontmatter", "", false, "", "is not valid YAML")}
	}

	var out []Finding
	ty, hasType := fmScalar(n.frontmatter, "type")
	if hasType && !slices.Contains(r.definition.Enums.Type, ty) {
		out = append(out, schemaFinding(n, "schema.enum", "type", true, ty, "is not an allowed type"))
	}

	isLesson := hasType && ty == "lesson"
	out = append(out, r.unknownKeys(n, isLesson)...)
	out = append(out, r.articleLanguage(n)...)
	if isLesson {
		out = append(out, r.lessonSlug(n)...)
	}

	// The group is resolved once and travels to the rule, so the enum the rule
	// reads is the group it was routed by.
	if hasType && r.contract.StatusGroup(ty) == systemDocumentGroup {
		return append(out, r.documentStatus(n, systemDocumentGroup)...)
	}
	return append(out, r.knowledge(n, seg)...)
}

// articleLanguage reports a language tag the reader's browser cannot act on,
// for the vaults whose contract knows the field at all.
func (r *lintRun) articleLanguage(n *note) []Finding {
	if !slices.Contains(r.definition.Fields.Known, "lang") {
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

// unknownKeys reports every frontmatter key the contract does not list as
// known, in sorted key order. A lesson may additionally use the lesson-only
// keys.
func (r *lintRun) unknownKeys(n *note, isLesson bool) []Finding {
	var out []Finding
	for _, key := range slices.Sorted(maps.Keys(n.frontmatter)) {
		known := slices.Contains(r.definition.Fields.Known, key) ||
			(isLesson && slices.Contains(r.definition.Fields.LessonOnly, key))
		if !known {
			out = append(out, schemaFinding(n, "schema.unknown_key", "", false, key, "is not a known field"))
		}
	}
	return out
}

// lessonSlug reports the lesson-only slug faults. Status validation belongs to
// the knowledge rules, which select the configured status group exactly once.
func (r *lintRun) lessonSlug(n *note) []Finding {
	var out []Finding
	switch sl, ok := fmScalar(n.frontmatter, "slug"); {
	case !ok:
		out = append(out, schemaFinding(n, "schema.required", "slug", true, "", "is required for a lesson"))
	case !r.slug.MatchString(sl):
		out = append(out, schemaFinding(n, "schema.slug", "slug", true, sl, "is not a valid slug"))
	}
	return out
}

// documentStatus reports a document's status outside the status set its own
// group declares. The group is the one the caller routed by, so the enum
// checked here is the enum that decided this note is a document.
func (r *lintRun) documentStatus(n *note, group string) []Finding {
	if st, ok := fmScalar(n.frontmatter, "status"); ok && !slices.Contains(r.definition.Enums.Status[group], st) {
		return []Finding{schemaFinding(n, "schema.enum", "status", true, st, "is not a valid system status")}
	}
	return nil
}

// knowledge reports the full knowledge-note rules, in reading order: required
// fields, the note status enum, the remaining value enums, then the structural
// rules.
func (r *lintRun) knowledge(n *note, seg []string) []Finding {
	var out []Finding
	out = append(out, r.required(n)...)
	// A type outside the contract resolves to no group and reads against the
	// general note group; it already carries its own finding.
	group := cmp.Or(r.contract.StatusGroup(n.noteType), "note")
	if st, ok := fmScalar(n.frontmatter, "status"); ok && !slices.Contains(r.definition.Enums.Status[group], st) {
		reason := "is not a valid status"
		if group != "note" {
			reason = "is not a valid " + group + " status"
		}
		out = append(out, schemaFinding(n, "schema.enum", "status", true, st, reason))
	}
	out = append(out, r.enumFields(n)...)
	out = append(out, r.structural(n, seg)...)
	return out
}

// required reports each required field that is absent or blank. A capture of
// undecided shape answers to the field set the contract declares for one; where
// the contract declares none it answers to the general set with the domain
// requirement waived, as do the types the contract exempts.
func (r *lintRun) required(n *note) []Finding {
	ty, hasType := fmScalar(n.frontmatter, "type")
	isInbox := hasType && ty == r.inboxType
	noDomain := isInbox || (hasType && slices.Contains(r.definition.Fields.DomainExempt, ty))
	required := r.definition.Fields.Required
	if isInbox && r.inboxDeclared {
		// The declared set is the whole answer for a capture, so nothing is
		// waived out of it: a contract that lists domain there wants it.
		required = r.inboxRequired
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

// enumFields reports each of the remaining enum-valued fields whose value is
// outside its allowed set.
func (r *lintRun) enumFields(n *note) []Finding {
	var out []Finding
	for _, ef := range []struct {
		field   string
		allowed []string
	}{
		{"domain", r.definition.Enums.Domain},
		{"source_kind", r.definition.Enums.SourceKind},
		{"source_provider", r.definition.Enums.SourceProvider},
		{"level", r.definition.Enums.Level},
		{"map_kind", r.definition.Enums.MapKind},
	} {
		if v, ok := fmScalar(n.frontmatter, ef.field); ok && !slices.Contains(ef.allowed, v) {
			out = append(out, schemaFinding(n, "schema.enum", ef.field, true, v, "is not an allowed value"))
		}
	}
	return out
}

// conceptDomainRoot and lessonDomainRoot are the two folders the human and
// markdown reports read a finding's knowledge domain out of. The concept root
// is a second spelling of what the contract declares under
// domain_equals_folder_under, so a vault that renames it files everything under
// the no-domain heading; the lesson root no contract key names at all.
const (
	conceptDomainRoot = "Concepts/"
	lessonDomainRoot  = "Writing/lessons/"
)

// structural reports the structural rules: a domain that does not match its
// folder, a slash-bearing legacy tag, and a distilled idea missing provenance.
func (r *lintRun) structural(n *note, seg []string) []Finding {
	var out []Finding
	// A domain must equal the first folder under the configured roots, e.g.
	// Concepts/<domain>/….
	if d, ok := fmScalar(n.frontmatter, "domain"); ok &&
		slices.Contains(r.definition.Rules.DomainEqualsFolderUnder, seg[0]) && len(seg) >= 3 && d != seg[1] {
		out = append(out, schemaFinding(n, "schema.domain_folder", "domain", true, d, "does not match its folder "+seg[1]))
	}
	if r.definition.Rules.ForbidTagWithSlash {
		if v, ok := n.frontmatter["tags"]; ok && v.isList {
			for _, tag := range v.list {
				if strings.Contains(tag, "/") {
					out = append(out, schemaFinding(n, "schema.legacy_tag", "tags", true, tag, "is a legacy tag (use a property)"))
				}
			}
		}
	}
	// Which type holds the corpus of distilled ideas, and whether this vault
	// keeps one at all, is the contract's to say; a vault declaring none has
	// nothing for this rule to be about.
	if ty, ok := fmScalar(n.frontmatter, "type"); ok && r.conceptDeclared && ty == r.conceptType &&
		!hasProvenance(n.frontmatter, r.definition.Rules.ConceptRequiresProvenance) {
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

// schemaRuleSource names the artifact holding one frontmatter rule's
// authority. The four structural rules each enforce a single key the
// contract's [rules] table declares, so they anchor there; every other rule
// reads several tables and cites the file without an anchor.
func schemaRuleSource(ruleID string) string {
	switch ruleID {
	case "schema.slug", "schema.domain_folder", "schema.legacy_tag", "schema.provenance":
		return sourceContractRules
	default:
		return sourceContract
	}
}

// schemaFinding builds one frontmatter finding. The message reads "<field>
// <reason>" for a blank value and `<field> "<value>" <reason>` otherwise, with
// the field name falling back to "frontmatter". The fingerprint folds the field
// name and the violating value together, so two blank-valued findings on one
// note stay distinct.
func schemaFinding(n *note, ruleID, field string, hasField bool, value, reason string) Finding {
	what := "frontmatter"
	if hasField {
		what = field
	}
	// The value is wrapped in literal double quotes and otherwise unchanged;
	// the encoder escapes those quotes, so its own bytes reach the message.
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
		SourceRule:      schemaRuleSource(ruleID),
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

// LintFrontmatter reports what the check command would say about one note's
// frontmatter, so a reading page and the command cannot disagree about a file.
// It takes the note's own bytes rather than a frontmatter parsed elsewhere,
// because this parse walks the YAML nodes to see a key written twice; linting
// the reading parse would be a second, quieter set of rules. The findings come
// back in the order the command puts them in.
func LintFrontmatter(relPath string, data []byte, contract *schema.Contract) ([]Finding, error) {
	if contract == nil {
		return nil, nil
	}
	findings, err := checkSchema([]note{parseNote(relPath, data)}, contract)
	if err != nil {
		return nil, err
	}
	sortFindings(findings)
	return findings, nil
}
