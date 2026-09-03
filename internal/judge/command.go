package judge

import (
	"bytes"
	"context"
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
// prints, and exits. Each runner takes already-parsed options and returns the
// bytes to print and the process exit code, so the binary's main only wires
// arguments in and the exit code out. Those codes are the frozen contract the
// pipelines depend on: 0 clean, 1 a gate hit or a "does not exist", 2 a tool
// error, which a runner reports as a non-nil error.

// ruleIDs is every rule a finding can carry. A --deny value is validated
// against it so a typo fails loudly instead of silently disabling the gate.
var ruleIDs = allRuleIDs()

// allRuleIDs lists the rules this package decides for itself, then appends the
// study-path rules from the grammar that names them. The grammar's half is
// asked for rather than copied, so a rule it gains later can still be gated on
// by name instead of being silently undeniable.
func allRuleIDs() []string {
	ids := []string{
		"link.title_not_alias",
		"link.broken",
		"link.section_missing",
		"link.block_missing",
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

// String names a format with the spelling ParseFormat accepts, so a message
// and the flag a reader would type are the same word. A value outside the three
// constants has no spelling and panics rather than inventing one.
func (f Format) String() string {
	switch f {
	case FormatJSON:
		return "json"
	case FormatHuman:
		return "human"
	case FormatMarkdown:
		return "md"
	default:
		panic("judge: unknown Format: " + strconv.Itoa(int(f)))
	}
}

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

// finish re-validates the contract authority and closes the observation, once.
// The action field is the latch: a payload whose observation has already been
// finished cannot be published a second time on an authority nobody rechecked.
func (p *preparedCommand) finish() error {
	if p.action == nil {
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
func RunCheck(ctx context.Context, o *CheckOptions) (stdout []byte, exit int, err error) {
	prepared, err := prepareCheck(ctx, o)
	if err != nil {
		return nil, 0, err
	}
	if err := prepared.finish(); err != nil {
		return nil, 0, err
	}
	return prepared.stdout, prepared.exit, nil
}

func prepareCheck(ctx context.Context, o *CheckOptions) (preparedCommand, error) {
	return prepareCheckWithHooks(ctx, o, actionHooks{})
}

func prepareCheckWithHooks(ctx context.Context, o *CheckOptions, hooks actionHooks) (preparedCommand, error) {
	for _, d := range o.Deny {
		if !isSeverityKeyword(d) && !slices.Contains(ruleIDs, d) {
			return preparedCommand{}, fmt.Errorf("unknown --deny %q; use a severity (error|warn|info) or a rule id", d)
		}
	}
	a, err := openAction(ctx, o.Root, hooks)
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
		baseline, parseErr := parseBaseline(string(data))
		if parseErr != nil {
			return preparedCommand{}, a.abort(fmt.Errorf("baseline %s: %w", o.Baseline, parseErr))
		}
		findings = retainNew(findings, baseline)
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
		panic("judge: unknown Format: " + o.Format.String())
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
func RunCoverage(ctx context.Context, o *CoverageOptions) (stdout []byte, exit int, err error) {
	prepared, err := prepareCoverage(ctx, o)
	if err != nil {
		return nil, 0, err
	}
	if err := prepared.finish(); err != nil {
		return nil, 0, err
	}
	return prepared.stdout, prepared.exit, nil
}

func prepareCoverage(ctx context.Context, o *CoverageOptions) (preparedCommand, error) {
	return prepareCoverageWithHooks(ctx, o, actionHooks{})
}

func prepareCoverageWithHooks(ctx context.Context, o *CoverageOptions, hooks actionHooks) (preparedCommand, error) {
	a, err := openAction(ctx, o.Root, hooks)
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
func RunExists(ctx context.Context, o *ExistsOptions) (stdout []byte, exit int, err error) {
	prepared, err := prepareExists(ctx, o)
	if err != nil {
		return nil, 0, err
	}
	if err := prepared.finish(); err != nil {
		return nil, 0, err
	}
	return prepared.stdout, prepared.exit, nil
}

func prepareExists(ctx context.Context, o *ExistsOptions) (preparedCommand, error) {
	return prepareExistsWithHooks(ctx, o, actionHooks{})
}

func prepareExistsWithHooks(ctx context.Context, o *ExistsOptions, hooks actionHooks) (preparedCommand, error) {
	a, err := openAction(ctx, o.Root, hooks)
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

// gated reports whether any finding reaches the deny gate. A severity keyword
// gates any finding at or above the lowest denied severity; a rule id gates a
// finding of that rule, but only at warn or above, so an info-level tracked
// forward-reference never gates through its rule id.
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
// delta. Every non-blank line must be a JSON object carrying a string
// fingerprint starting with the current algorithm-version prefix; anything less
// stops the run with the offending line's number, because a skipped line would
// subtract less than the caller believes. An empty file is a valid baseline.
func parseBaseline(jsonl string) (map[string]bool, error) {
	set := make(map[string]bool)
	lineNo := 0
	for line := range strings.SplitSeq(jsonl, "\n") {
		lineNo++
		if strings.TrimSpace(line) == "" {
			continue
		}
		var obj map[string]json.RawMessage
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			return nil, fmt.Errorf("line %d is not a JSON object: %w", lineNo, err)
		}
		raw, ok := obj["fingerprint"]
		if !ok {
			return nil, fmt.Errorf("line %d carries no fingerprint", lineNo)
		}
		var fp string
		if err := json.Unmarshal(raw, &fp); err != nil {
			return nil, fmt.Errorf("line %d fingerprint is not a string: %w", lineNo, err)
		}
		if !strings.HasPrefix(fp, fingerprintVersion) {
			return nil, fmt.Errorf(
				"line %d fingerprint %q was written by a different fingerprint version; regenerate the baseline with this binary, whose values start with %q",
				lineNo, fp, fingerprintVersion)
		}
		set[fp] = true
	}
	return set, nil
}

// retainNew drops findings whose fingerprint is already in the baseline,
// leaving only what this run newly introduced.
func retainNew(findings []Finding, baseline map[string]bool) []Finding {
	return slices.DeleteFunc(findings, func(f Finding) bool {
		return baseline[f.Fingerprint]
	})
}
