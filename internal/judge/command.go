package judge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/koopa0/yomihon/internal/sequence"
)

// The three subcommands — check, coverage, exists — run as stateless actions:
// a process opens the selected vault once, captures its complete file domain,
// prints, and exits. There is no server, daemon, or persistent store behind
// them. Each runner takes already-parsed options and returns the bytes to print
// and the process exit code, so the binary's main only wires arguments in and
// the exit code out, and every exit-code decision is exercised by a test.
//
// Exit codes match the frozen contract the pipelines depend on: 0 clean, 1 a
// gate hit or a "does not exist", 2 a tool error. A tool error is returned as a
// non-nil error, which the binary reports and turns into exit 2.

// ruleIDs is every rule a finding can carry. A --deny value is validated
// against it so a typo fails loudly instead of silently disabling the gate.
var ruleIDs = allRuleIDs()

// allRuleIDs lists the rules this package decides for itself, then appends the
// study-path rules from the grammar that names them. The grammar's half is
// asked for rather than copied: a rule it gains later arrives here with it, so
// a vault that trips the new rule can still be gated on it by name. A hand copy
// would leave that rule undeniable and the omission would show up as a working
// gate that quietly lets one rule through.
func allRuleIDs() []string {
	ids := []string{
		"link.title_not_alias",
		"link.broken",
		"collision.alias",
		"provenance.unresolved",
		"map.disk_mismatch",
		"map.disk_unlisted",
		"link.broken.path",
		"schema.enum",
		"schema.required",
		"schema.unknown_key",
		"schema.slug",
		"schema.domain_folder",
		"schema.legacy_tag",
		"schema.provenance",
		"schema.frontmatter",
		"schema.unmatched_knowledge_dir",
		"collision.name",
		predecessorNotArchivedRule,
		archivedNavigationRule,
		// The authoring contract's own rules. schema.language is emitted but was
		// never listed here, so this list is kept honest by a test that compares it
		// against what the rules actually emit.
		"schema.language",
	}
	for _, rule := range sequence.Rules() {
		ids = append(ids, string(rule))
	}
	return ids
}

// Format is the output format of a subcommand.
type Format int

const (
	// FormatJSON is the machine format: JSONL for check, a compact JSON
	// object for coverage and exists.
	FormatJSON Format = iota
	// FormatHuman is the terminal format for a person.
	FormatHuman
	// FormatMarkdown is a fileable markdown report body. It is a check-only
	// format; coverage and exists fall back to the human view.
	FormatMarkdown
)

// ParseFormat maps a --format value to a Format, reporting false for a value
// that is none of json, human, or md.
func ParseFormat(s string) (Format, bool) {
	switch s {
	case "json":
		return FormatJSON, true
	case "human":
		return FormatHuman, true
	case "md":
		return FormatMarkdown, true
	default:
		return 0, false
	}
}

// ResolveFormat picks the output format: an explicit flag wins; otherwise the
// machine format for a pipe and the human format for a terminal, so an agent
// reading a pipe gets JSON and a person at a terminal gets the readable view.
func ResolveFormat(explicit *Format, isTTY bool) Format {
	if explicit != nil {
		return *explicit
	}
	if isTTY {
		return FormatHuman
	}
	return FormatJSON
}

// CheckOptions is the parsed check command. Paths only filter the output; the
// graph is always built from the whole root. All includes System/. Deny lists
// the severities or rule ids that gate the run. Baseline, when set, is a prior
// run's JSONL whose findings are subtracted so only what this run newly
// introduced is reported and gated.
type CheckOptions struct {
	Root     string
	Paths    []string
	All      bool
	Deny     []string
	Baseline string
	Format   Format
}

type preparedCommand struct {
	stdout []byte
	exit   int
	action *action
}

func (p *preparedCommand) finish() error {
	if p == nil || p.action == nil {
		return errVaultScan
	}
	a := p.action
	p.action = nil
	return a.finish()
}

// RunCheck scans the vault and renders the findings. It returns the bytes to
// print and the exit code: 1 when a finding gates, 0 otherwise. An unknown
// --deny token, an unreadable baseline, or a scan failure is returned as an
// error, which the caller turns into a tool-error exit.
func RunCheck(o *CheckOptions) (stdout []byte, exit int, err error) {
	prepared, err := prepareCheck(o)
	if err != nil {
		return nil, 0, err
	}
	if err := prepared.finish(); err != nil {
		return nil, 0, err
	}
	return prepared.stdout, prepared.exit, nil
}

func prepareCheck(o *CheckOptions) (preparedCommand, error) {
	return prepareCheckWithHooks(o, actionHooks{})
}

func prepareCheckWithHooks(o *CheckOptions, hooks actionHooks) (preparedCommand, error) {
	for _, d := range o.Deny {
		if !isSeverityKeyword(d) && !slices.Contains(ruleIDs, d) {
			return preparedCommand{}, fmt.Errorf("unknown --deny %q; use a severity (error|warn|info) or a rule id", d)
		}
	}
	a, err := openAction(o.Root, hooks)
	if err != nil {
		return preparedCommand{}, err
	}
	findings, err := checkAction(a, o.Paths, o.All)
	if err != nil {
		return preparedCommand{}, a.abort(err)
	}
	if o.Baseline != "" {
		data, err := os.ReadFile(o.Baseline) // #nosec G304 -- the baseline path is an operator-supplied CLI argument, not untrusted input
		if err != nil {
			return preparedCommand{}, a.abort(fmt.Errorf("read baseline %s: %w", o.Baseline, err))
		}
		findings = retainNew(findings, parseBaseline(string(data)))
	}
	var stdout []byte
	switch o.Format {
	case FormatJSON:
		var buf bytes.Buffer
		if err := WriteJSONL(&buf, findings); err != nil {
			return preparedCommand{}, a.abort(fmt.Errorf("serialize findings: %w", err))
		}
		stdout = buf.Bytes()
	case FormatHuman:
		stdout = []byte(humanReport(findings))
	case FormatMarkdown:
		stdout = []byte(markdownReport(findings))
	default:
		panic("judge: unknown Format: " + strconv.Itoa(int(o.Format)))
	}
	exit := 0
	if gated(findings, o.Deny) {
		exit = 1
	}
	return preparedCommand{stdout: stdout, exit: exit, action: a}, nil
}

// CoverageOptions is the parsed coverage command.
type CoverageOptions struct {
	Root   string
	Format Format
}

// RunCoverage computes and renders coverage. It always exits 0 — coverage
// reports state, it never gates. A scan or serialization failure is returned as
// an error.
func RunCoverage(o *CoverageOptions) (stdout []byte, exit int, err error) {
	prepared, err := prepareCoverage(o)
	if err != nil {
		return nil, 0, err
	}
	if err := prepared.finish(); err != nil {
		return nil, 0, err
	}
	return prepared.stdout, prepared.exit, nil
}

func prepareCoverage(o *CoverageOptions) (preparedCommand, error) {
	return prepareCoverageWithHooks(o, actionHooks{})
}

func prepareCoverageWithHooks(o *CoverageOptions, hooks actionHooks) (preparedCommand, error) {
	a, err := openAction(o.Root, hooks)
	if err != nil {
		return preparedCommand{}, err
	}
	cov := computeCoverage(a.notes, buildIndex(a.notes, a.resources), a.authority)
	var stdout []byte
	if o.Format == FormatJSON {
		out, err := marshalWire(cov)
		if err != nil {
			return preparedCommand{}, a.abort(fmt.Errorf("serialize coverage: %w", err))
		}
		stdout = out
	} else {
		// md is a check-only format; coverage falls back to the human view.
		stdout = []byte(renderCoverage(&cov))
	}
	return preparedCommand{stdout: stdout, action: a}, nil
}

// ExistsOptions is the parsed exists command.
type ExistsOptions struct {
	Root   string
	Name   string
	Format Format
}

// RunExists answers whether a note for the name already exists. It exits 0 when
// a match exists and 1 when none does, so a caller can gate a
// write-if-absent on the exit code alone. A scan or serialization failure is
// returned as an error.
func RunExists(o *ExistsOptions) (stdout []byte, exit int, err error) {
	prepared, err := prepareExists(o)
	if err != nil {
		return nil, 0, err
	}
	if err := prepared.finish(); err != nil {
		return nil, 0, err
	}
	return prepared.stdout, prepared.exit, nil
}

func prepareExists(o *ExistsOptions) (preparedCommand, error) {
	return prepareExistsWithHooks(o, actionHooks{})
}

func prepareExistsWithHooks(o *ExistsOptions, hooks actionHooks) (preparedCommand, error) {
	a, err := openAction(o.Root, hooks)
	if err != nil {
		return preparedCommand{}, err
	}
	report := existsLookup(a.notes, o.Name, a.authority)
	var stdout []byte
	if o.Format == FormatJSON {
		out, err := marshalWire(report)
		if err != nil {
			return preparedCommand{}, a.abort(fmt.Errorf("serialize exists: %w", err))
		}
		stdout = out
	} else {
		// md is a check-only format; exists falls back to the human view.
		stdout = []byte(renderExists(report))
	}
	exit := 1
	if report.found() {
		exit = 0
	}
	return preparedCommand{stdout: stdout, exit: exit, action: a}, nil
}

// gated reports whether any finding reaches the deny gate. A --deny token is
// either a severity keyword or a rule id, and the two coexist: a severity
// keyword gates any finding at or above the lowest denied severity — so
// denying error gates every error without naming each schema rule — while a
// rule id gates a finding of that rule, but only at warn or above, so an
// info-level tracked forward-reference never gates through its rule id.
func gated(findings []Finding, deny []string) bool {
	threshold, hasThreshold := minDeniedSeverity(deny)
	for i := range findings {
		f := &findings[i]
		if hasThreshold && f.Severity >= threshold {
			return true
		}
		if f.Severity >= SeverityWarn && slices.Contains(deny, f.RuleID) {
			return true
		}
	}
	return false
}

// minDeniedSeverity is the lowest severity among the deny tokens that are
// severity keywords, and whether any was found.
func minDeniedSeverity(deny []string) (Severity, bool) {
	found := false
	var lowest Severity
	for _, d := range deny {
		if s, ok := severityFromKeyword(d); ok {
			if !found || s < lowest {
				lowest = s
			}
			found = true
		}
	}
	return lowest, found
}

// severityFromKeyword maps a severity keyword to its Severity.
func severityFromKeyword(s string) (Severity, bool) {
	switch s {
	case "info":
		return SeverityInfo, true
	case "warn":
		return SeverityWarn, true
	case "error":
		return SeverityError, true
	default:
		return 0, false
	}
}

// isSeverityKeyword reports whether s is one of the severity keywords.
func isSeverityKeyword(s string) bool {
	_, ok := severityFromKeyword(s)
	return ok
}

// parseBaseline collects the fingerprints from a prior run's JSONL, for a
// delta. A line that does not parse or carries no string fingerprint is
// skipped: a baseline is advisory input, not a hard contract.
func parseBaseline(jsonl string) map[string]bool {
	set := make(map[string]bool)
	for line := range strings.SplitSeq(jsonl, "\n") {
		var obj map[string]json.RawMessage
		if json.Unmarshal([]byte(line), &obj) != nil {
			continue
		}
		raw, ok := obj["fingerprint"]
		if !ok {
			continue
		}
		var fp string
		if json.Unmarshal(raw, &fp) == nil {
			set[fp] = true
		}
	}
	return set
}

// retainNew drops findings whose fingerprint is already in the baseline,
// leaving only what this run newly introduced.
func retainNew(findings []Finding, baseline map[string]bool) []Finding {
	return slices.DeleteFunc(findings, func(f Finding) bool {
		return baseline[f.Fingerprint]
	})
}
