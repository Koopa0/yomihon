package judge

import (
	"maps"
	"slices"
	"strings"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

// checkSchema validates every knowledge note's frontmatter against the
// contract, all findings at error severity. Scope is the contract's own
// knowledge layer: an undeclared layer excludes nothing, so every note is
// linted.
func checkSchema(notes []note, contract *schema.Contract) []Finding {
	return newLintRun(contract).check(notes)
}

// checkKnowledgeScope reports each knowledge directory the contract declares
// that nothing in this vault answers to. Such an entry turns the frontmatter
// rules off for the ground its author meant to govern while every gate stays
// green. A declaration is answered by the directory itself or by any file
// below it, so an emptied inbox is still the folder the contract named.
func checkKnowledgeScope(scan vault.Scan, contract *schema.Contract) []Finding {
	declared := contract.KnowledgeScope().Declared()
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
		Evidence:        "the vault contract declares this directory under folders.checked and the scan observed neither that directory nor any file below one of that name",
		SuggestedAction: "correct the spelling in the vault contract, or drop the entry if the directory is gone",
		SourceRule:      sourceContractFolders,
		Target:          new(dir),
		Fingerprint:     fingerprint("schema.unmatched_knowledge_dir", "", dir),
	}
}

// lintRun is one contract resolved into everything the frontmatter rules read,
// held together for the length of one scan. Every field is derived from the
// contract exactly once, and they travel together because they have to agree:
// passed separately, a caller could mix them from two different contracts.
type lintRun struct {
	contract            *schema.Contract
	scope               schema.KnowledgeScope
	skip                []string
	requiresFrontmatter bool
	conceptType         string
	conceptDeclared     bool
	lessonType          string
	provenance          []string
	enums               []schema.EnumDefinition
	domains             schema.Domains
}

// newLintRun resolves a contract into the run the rules read from.
func newLintRun(contract *schema.Contract) *lintRun {
	definition := contract.Definition()
	run := &lintRun{
		contract:            contract,
		scope:               contract.KnowledgeScope(),
		skip:                contract.SkipBasenames(),
		requiresFrontmatter: contract.RequiresFrontmatter(),
		provenance:          definition.Rules.ConceptRequiresProvenance,
		enums:               definition.Enums,
		domains:             contract.Domains(),
	}
	run.conceptType, run.conceptDeclared = contract.ConceptType()
	run.lessonType, _ = contract.LessonType()
	return run
}

// check returns the frontmatter findings of every note the run's contract
// governs: those inside its knowledge scope whose file name its scan does not
// skip.
func (r *lintRun) check(notes []note) []Finding {
	var out []Finding
	for i := range notes {
		n := &notes[i]
		seg := strings.Split(n.path, "/")
		skipped := slices.Contains(r.skip, seg[len(seg)-1])
		if r.scope.Includes(n.path) && !skipped {
			out = append(out, r.note(n)...)
		}
	}
	return out
}

// note returns the frontmatter findings for one in-scope note, in the
// contract's reading order: the type enum, unknown keys, the article language,
// the draft flag, the lesson slug, reference shapes, then the required fields,
// the value enums and the structural rules. That order is the tiebreak the
// stable sort preserves.
func (r *lintRun) note(n *note) []Finding {
	if n.noFrontmatter {
		// The fence that never closes is said whatever the contract thinks of
		// a note with no block, and in place of "is missing": the block is
		// there and only its closing line is not, so "is missing" would send
		// the author to the wrong fault.
		if n.unclosedFrontmatter {
			return []Finding{unclosedFrontmatterFinding(n)}
		}
		if r.requiresFrontmatter {
			return []Finding{schemaFinding(n, "schema.frontmatter", "", "", "is missing")}
		}
		return nil
	}
	if n.badFrontmatter {
		return []Finding{schemaFinding(n, "schema.frontmatter", "", "", "is not valid YAML")}
	}

	var out []Finding
	ty, hasType := fmScalar(n.frontmatter, "type")
	if hasType && !slices.Contains(r.contract.EnumValues("type"), schema.NormalizeWord(ty)) {
		out = append(out, schemaFinding(n, "schema.enum", "type", ty, "is not an allowed type"))
	}

	isLesson := hasType && ty == r.lessonType
	out = append(out, r.unknownKeys(n, ty)...)
	out = append(out, r.articleLanguage(n)...)
	out = append(out, r.draft(n)...)
	if isLesson {
		out = append(out, r.lessonSlug(n, ty)...)
	}
	out = append(out, r.referenceNestedSequences(n)...)
	out = append(out, r.required(n, ty, isLesson)...)
	out = append(out, r.enumFields(n)...)
	out = append(out, r.structural(n)...)
	return out
}

// referenceNestedSequences reports once per reference field, preserving the
// parser's list values while explaining items YAML did not read as references.
func (r *lintRun) referenceNestedSequences(n *note) []Finding {
	var out []Finding
	for _, field := range noteReferenceFields(n) {
		if !n.frontmatter[field.name].hasNestedSequence {
			continue
		}
		out = append(out, Finding{
			RuleID:          "schema.reference_nested_sequence",
			Severity:        SeverityError,
			Path:            n.path,
			Field:           new(field.name),
			Message:         field.name + " contains a nested YAML list instead of a reference",
			Evidence:        "a reference item was read as a YAML sequence",
			SuggestedAction: "quote the reference, for example " + field.name + ": [\"[[Note]]\"] or " + field.name + ": [\"Note\"]",
			SourceRule:      field.sourceRule,
			Fingerprint:     fingerprint("schema.reference_nested_sequence", n.path, field.name+"\x1f"),
		})
	}
	return out
}

// articleLanguage reports a language tag the reader's browser cannot act on,
// for the vaults whose contract knows the field at all.
func (r *lintRun) articleLanguage(n *note) []Finding {
	if !r.contract.ArticleLanguage().Declared() {
		return nil
	}
	value, ok := n.frontmatter["lang"]
	if !ok {
		return nil
	}
	if value.isList || !value.scalarIsString || value.scalar == "" {
		return []Finding{schemaFinding(n, "schema.language", "lang", value.scalar, "must be a non-empty BCP 47 language tag")}
	}
	if _, err := schema.ParseLanguageTag(value.scalar); err != nil {
		return []Finding{schemaFinding(n, "schema.language", "lang", value.scalar, "is not a valid BCP 47 language tag")}
	}
	return nil
}

// unknownKeys reports every frontmatter key the contract does not let this
// note carry, in sorted key order. The list is closed: a key is one yomihon
// defines and the contract gave this type, or the name Obsidian reserves, or it
// is reported. A key only some other type lists leaves a note of an undeclared
// type unjudged and reports that dependency once, without hiding genuinely
// unknown keys.
func (r *lintRun) unknownKeys(n *note, noteType string) []Finding {
	var out []Finding
	deferred := false
	for _, key := range slices.Sorted(maps.Keys(n.frontmatter)) {
		switch r.contract.FieldStanding(noteType, key) {
		case schema.FieldAllowed:
			continue
		case schema.FieldTypeDependent:
			deferred = true
			continue
		case schema.FieldUnknown:
			out = append(out, schemaFinding(n, "schema.unknown_key", "", key, "is not a known field"))
		}
	}
	if deferred {
		finding := schemaFinding(n, "schema.type_dependent", "type", noteType, "is not valid, so type-only fields cannot be judged until type is valid")
		finding.SuggestedAction = "fix type before judging type-only fields"
		out = append(out, finding)
	}
	return out
}

// draft reports a draft flag that is not the boolean true or false. The key
// is the product's own, so the check holds in every vault whether or not the
// contract lists it, and a flag written as text would otherwise read as a
// finished note.
func (r *lintRun) draft(n *note) []Finding {
	v, ok := n.frontmatter[schema.DraftField]
	if !ok {
		return nil
	}
	if !v.isList && !v.scalarIsString && (v.scalar == "true" || v.scalar == "false") {
		return nil
	}
	return []Finding{schemaFinding(n, "schema.draft", schema.DraftField, v.scalar, "must be true or false")}
}

// lessonSlug reports the slug faults of a note holding the lesson role.
func (r *lintRun) lessonSlug(n *note, noteType string) []Finding {
	var out []Finding
	switch sl, ok := fmScalar(n.frontmatter, "slug"); {
	case !ok:
		out = append(out, schemaFinding(n, "schema.required", "slug", "", "is required for a lesson"))
	default:
		if pattern, declared := r.contract.SlugPattern(noteType); declared && !pattern.MatchString(sl) {
			out = append(out, schemaFinding(n, "schema.slug", "slug", sl, "is not a valid slug"))
		}
	}
	return out
}

// required reports each required field that is absent or blank: the fields
// every note must carry, then those its type adds. A lesson's slug has its own
// finding and is not asked twice.
func (r *lintRun) required(n *note, noteType string, isLesson bool) []Finding {
	var out []Finding
	for _, key := range r.contract.RequiredFields(noteType) {
		if isLesson && key == schema.FieldSlug {
			continue
		}
		if v, ok := n.frontmatter[key]; !ok || !v.present() {
			out = append(out, schemaFinding(n, "schema.required", key, "", "is required"))
		}
	}
	return out
}

// enumFields reports each field whose value is outside the vocabulary the
// contract states for it. The domain is the one value no note writes: it is
// read off the folder, and a folder the vocabulary does not name is reported
// against the note in it.
func (r *lintRun) enumFields(n *note) []Finding {
	var out []Finding
	for _, enum := range r.enums {
		if len(enum.Values) == 0 {
			continue
		}
		// The contract holds its own values in one spelling already, so only
		// the note's value is folded here; the finding still carries the word
		// the file wrote, so a reader is shown their own bytes.
		if enum.Field == "domain" {
			if domain := r.domains.Of(n.path); domain != "" && !slices.Contains(enum.Values, schema.NormalizeWord(domain)) {
				out = append(out, unlistedDomainFinding(n, domain))
			}
			continue
		}
		if v, ok := fmScalar(n.frontmatter, enum.Field); ok && !slices.Contains(enum.Values, schema.NormalizeWord(v)) {
			out = append(out, schemaFinding(n, "schema.enum", enum.Field, v, "is not an allowed value"))
		}
	}
	return out
}

// structural reports the one structural rule left: a distilled idea missing
// provenance.
func (r *lintRun) structural(n *note) []Finding {
	// Which type holds the corpus of distilled ideas, and whether this vault
	// keeps one at all, is the contract's to say; a vault declaring none has
	// nothing for this rule to be about.
	if ty, ok := fmScalar(n.frontmatter, "type"); ok && r.conceptDeclared && ty == r.conceptType &&
		!hasProvenance(n.frontmatter, r.provenance) {
		return []Finding{schemaFinding(n, "schema.provenance", "", "", "concept has neither based_on nor source_locator")}
	}
	return nil
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
// authority. The slug rule enforces the pattern a type's own table states, and
// the provenance rule a key the [rules] table declares, so each anchors there;
// every other rule reads several tables and cites the file without an anchor.
func schemaRuleSource(ruleID RuleID) string {
	switch ruleID {
	case "schema.slug":
		return sourceContractTypes
	case "schema.provenance":
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
func schemaFinding(n *note, ruleID RuleID, field, value, reason string) Finding {
	// Every caller names the field it is reporting on, or names none at all;
	// there is no finding about an unnamed field of a known name.
	hasField := field != ""
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

// unlistedDomainFinding builds the finding for a note whose folder names a
// domain the contract's vocabulary does not. The note wrote no domain, so
// there is no frontmatter to fix: the evidence says where the value came from
// and the action moves the note or widens the vocabulary.
func unlistedDomainFinding(n *note, domain string) Finding {
	f := schemaFinding(n, "schema.enum", "domain", domain, "is not an allowed value")
	f.Evidence = "the note's domain is the name of the folder it sits in below a folders.domain_from root, and the vault contract's enums.domain does not list that name"
	f.SuggestedAction = "move the note into a folder named for a domain the vault contract declares, or add the folder's name to enums.domain in the vault contract"
	return f
}

// unclosedFrontmatterFinding builds the finding for a note whose opening fence
// nothing closes. It is a reason of schema.frontmatter, whose other two reasons
// are a block that cannot be read and a note with no block where the contract
// wants one, and it keeps the identity those two share: one per path. The
// action is its own, because nothing here is a mismatch with the schema to fix.
func unclosedFrontmatterFinding(n *note) Finding {
	f := schemaFinding(n, "schema.frontmatter", "", "", "opens on line 1 and never closes")
	f.SuggestedAction = "add the closing --- line, or remove the opening --- if the note has no frontmatter"
	return f
}

// LintFrontmatter reports what the check command would say about one note's
// frontmatter, so a reading page and the command cannot disagree about a file.
// It takes the note's own bytes rather than a frontmatter parsed elsewhere,
// because this parse walks the YAML nodes to see a key written twice; linting
// the reading parse would be a second, quieter set of rules. The findings come
// back in the order the command puts them in.
func LintFrontmatter(relPath string, data []byte, contract *schema.Contract) []Finding {
	return NewFrontmatterLinter(contract).Lint(relPath, data)
}

// FrontmatterLinter is LintFrontmatter for a caller with many notes to judge
// against one contract: the contract is resolved once, rather than once per
// note, and a note is read for its frontmatter alone, because the frontmatter
// rules read nothing of the body. The zero value, which is what a folder no
// contract governs gets, says nothing about any note.
type FrontmatterLinter struct {
	run *lintRun
}

// NewFrontmatterLinter resolves the contract. A nil contract yields the zero
// linter.
func NewFrontmatterLinter(contract *schema.Contract) FrontmatterLinter {
	if contract == nil {
		return FrontmatterLinter{}
	}
	return FrontmatterLinter{run: newLintRun(contract)}
}

// Lint reports what the check command would say about one note's frontmatter,
// in the order the command puts the findings in. Like LintFrontmatter it takes
// the note's own bytes.
func (l FrontmatterLinter) Lint(relPath string, data []byte) []Finding {
	if l.run == nil {
		return nil
	}
	findings := l.run.check([]note{parseFrontmatter(relPath, data)})
	sortFindings(findings)
	return findings
}
