package judge

import (
	"cmp"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

// checkSchema validates every knowledge note's frontmatter against the
// contract and returns the resulting findings, all at error severity. Scope
// follows the contract's own scan policy: only notes whose first path segment
// is a knowledge directory, and never a skipped basename. The findings are in
// per-note emission order; the deterministic total order is applied before
// they are written.
//
// A contract that declares no knowledge directory lints nothing here. That is
// the opposite of what the reading surfaces answer for the same silence, and
// deliberately so: a surface that hid every file would show an empty vault,
// while a judge that linted every file would hold notes to rules their author
// never claimed covered them. Which spelling of a directory counts is not
// deliberate in the same way, so the membership test folds case the way the
// privacy and artifact scopes do.
//
// The only failure is a slug pattern in the contract that is not a valid
// regular expression, which is a fault in the contract file rather than in
// any note.
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
// rules off for the ground its author meant to govern — a misspelling, a folder
// that was renamed, a contract copied from another vault — and every gate stays
// green while it does, because the notes the rules were written for are simply
// never examined. It is the configuration that speaks here rather than each
// note, and at error severity: a whole set of rules reaching nothing outweighs
// any single note breaking one.
//
// A directory the scan saw answers whether or not anything is in it: an inbox
// its owner has emptied is the folder the contract named, waiting for the next
// capture, and telling him to fix his contract over it would be advice to
// break a vault that is correct. So a declaration is answered either by the
// directory itself or by a file below it — the scan's files, not its notes,
// since a folder holding only pictures is governed ground with no frontmatter
// to judge yet, and since the directory is only observable under the exact
// spelling that was declared while a file below it is matched under any.
func checkKnowledgeScope(scan vault.Scan, contract *schema.Contract) []Finding {
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
// nothing on disk answers to. The declared value is the path the finding
// carries: it is the ground the contract claims, it is a single safe component
// by the contract's own validation, and it is what the contract's privacy
// policy is asked about, so a directory declared both private and knowledge
// drops out of agent-facing output through the ordinary filter. The fingerprint
// keys on the declared value alone, since the fault is one line of the contract
// rather than anything in the vault's files.
func unmatchedKnowledgeDir(dir string) Finding {
	return Finding{
		RuleID:          "schema.unmatched_knowledge_dir",
		Severity:        SeverityError,
		Path:            dir,
		Message:         "knowledge directory \"" + dir + "\" matches nothing in this vault, so the frontmatter rules reach nothing there",
		Evidence:        "the vault contract declares this directory under scan.knowledge_dirs and the scan observed neither that directory nor any file below one of that name",
		SuggestedAction: "correct the spelling in the vault contract, or drop the entry if the directory is gone",
		SourceRule:      sourceContract,
		Target:          new(dir),
		Fingerprint:     fingerprint("schema.unmatched_knowledge_dir", "", dir),
	}
}

// systemDocumentGroup is the status group whose members are a vault's own
// working documents — its templates, guides and system notes — rather than
// knowledge it wrote. Membership is the contract's answer, one type at a time;
// only the name of the group is this face's, and it is a word rather than a
// derivation because nothing in the contract marks a group as holding
// documents. That is a gap in the contract's vocabulary, and until it is
// closed, a vault filing its documents under a group of another name has the
// full knowledge-note rules applied to its templates. Naming it once at least
// keeps the routing and the enum the routed rule reads from drifting apart.
const systemDocumentGroup = "system"

// lintRun is one contract resolved into everything the frontmatter rules read,
// held together for the length of one scan. Every field is derived from the
// contract and derived exactly once: the definition it publishes, the slug
// pattern compiled from that definition, whether a note has to carry a
// frontmatter block at all, what a capture of undecided shape is required to
// carry, and which type this vault files its distilled ideas under — together
// with whether it keeps such a corpus at all, since a vault that keeps none has
// nothing for the provenance rule to be about.
//
// They travel together because they have to agree. Passed separately they were
// six arguments a caller could mix from two different contracts with nothing
// objecting, and each new rule added a seventh.
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
// contract's reading order: the type enum, unknown keys, the optional article
// language, the lesson-only rules, then either the light document rules or the
// full knowledge-note rules. That order is the tiebreak the stable sort
// preserves among findings that share a path and rule id.
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

	// A vault's templates, guides and system documents carry only the light
	// status rule; the full knowledge-note rules below do not apply to them.
	// The group the type belongs to is resolved once and travels to the rule,
	// so the enum the rule reads is the group it was routed by.
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
// group declares. The group is the one the caller routed by rather than a
// second spelling of it, so the enum checked here is always the enum that
// decided this note is a document.
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
	// The contract resolves a declared type's group; a type outside the
	// contract resolves to none, and its status still reads against the
	// general note group — the type already carries its own finding, and a
	// group that does not exist could neither name the enum to check nor
	// the group to blame in the reason.
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

// required reports each required field that is absent or blank. A capture
// of undecided shape answers to the field set the contract declares for one, in
// place of the general set; where the contract declares none it answers to the
// general set with the domain requirement waived, since such a capture is not
// classified by knowledge domain. The same waiver covers the types the contract
// exempts (system docs, research briefs).
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
	// Asking a distilled idea where it came from presupposes a corpus of them,
	// and which type holds that corpus — and whether this vault keeps one at
	// all — is the contract's to say. A vault that declares none has nothing
	// for this rule to be about, and its notes already carry the finding that
	// says the type is not one the contract lists.
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
		SourceRule:      sourceContract,
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
// frontmatter, given the note's vault-relative path and its bytes.
//
// It exists so the reading pages can show a reader the same verdict the
// command reports. They had no way to reach it and went nearly silent about
// faults the command calls errors, which left the two faces of one program
// disagreeing about the same file.
//
// The note's own bytes are what this takes, rather than a frontmatter already
// parsed elsewhere, because the two parses are deliberately different: the
// reading parse decodes into a map and is content with what it can read, while
// this one walks the YAML nodes so it can see a key written twice and refuse a
// name the schema would resolve to a number. Linting the looser parse would be
// a second, quieter set of rules — the thing that must not exist twice.
//
// The findings come back in the order the command puts them in, so a page and
// a command listing the same note's faults list them the same way round.
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
