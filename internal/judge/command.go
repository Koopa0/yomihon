package judge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/koopa0/yomihon/internal/egress"
)

// The three subcommands — check, coverage, exists — run as stateless actions:
// a process opens the selected vault once, captures its complete file domain,
// prints, and exits. Each runner takes already-parsed options and returns the
// bytes to print and the process exit code, so the binary's main only wires
// arguments in and the exit code out: 0 clean, 1 a gate hit or a "does not
// exist", 2 a tool error, which a runner reports as a non-nil error.

// Format is the shape a command's answer is printed in.
type Format int

const (
	// FormatJSON is the machine format: JSONL for check, a compact JSON
	// object for coverage and exists.
	FormatJSON Format = iota
	// FormatHuman is the terminal format for a person.
	FormatHuman
	// FormatMarkdown is check's report as a markdown body. Coverage and exists
	// have no such view and refuse it.
	FormatMarkdown
)

// CheckOptions is the parsed check command. Paths only filter the output; the
// graph is always built from the whole root. All restores findings that touch
// nothing inside the declared knowledge layer. Deny lists the severities or
// rule ids that gate the run. Baseline, when set, is a prior run's JSONL
// whose findings are subtracted so only what this run newly introduced is
// reported and gated.
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

// seal looks at the rules file again, closes the observation and makes the
// payload, once. The action field is the latch: a payload whose observation has
// already been sealed cannot be published a second time on an authority nobody
// rechecked.
func (p *preparedCommand) seal() (egress.Payload, error) {
	if p.action == nil {
		return egress.Payload{}, errVaultScan
	}
	a := p.action
	p.action = nil
	return a.seal(p.stdout)
}

// sealPrepared seals what a command prepared, or passes on why it could not
// prepare it.
func sealPrepared(p preparedCommand, err error) (egress.Payload, int, error) {
	if err != nil {
		return egress.Payload{}, 0, err
	}
	payload, err := p.seal()
	if err != nil {
		return egress.Payload{}, 0, err
	}
	return payload, p.exit, nil
}

// RunCheck scans the vault and renders the findings. It returns the bytes to
// print and the exit code: 1 when a finding gates, 0 otherwise. An unknown
// --deny token, an unreadable baseline, or a scan failure is returned as an
// error, which the caller turns into a tool-error exit.
func RunCheck(ctx context.Context, o *CheckOptions) (stdout egress.Payload, exit int, err error) {
	return sealPrepared(prepareCheckWithHooks(ctx, o, actionHooks{}))
}

func prepareCheckWithHooks(ctx context.Context, o *CheckOptions, hooks actionHooks) (preparedCommand, error) {
	for _, d := range o.Deny {
		if !isSeverityKeyword(d) && !slices.Contains(ruleIDs, RuleID(d)) {
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
		if findings, err = subtractBaseline(findings, o.Baseline); err != nil {
			return preparedCommand{}, a.abort(err)
		}
	}
	stdout, err := renderCheck(findings, a.authority, o.Format)
	if err != nil {
		return preparedCommand{}, a.abort(err)
	}
	exit := 0
	if gated(findings, o.Deny) {
		exit = 1
	}
	return preparedCommand{stdout: stdout, exit: exit, action: a}, nil
}

// renderCheck prints the findings in the format asked for. A person is told
// first when the folder has no rules file, since the frontmatter rules did not
// run.
func renderCheck(findings []Finding, authority scanAuthority, format Format) ([]byte, error) {
	var out []byte
	switch format {
	case FormatJSON:
		var buf bytes.Buffer
		if err := writeJSONL(&buf, findings); err != nil {
			return nil, fmt.Errorf("serialize findings: %w", err)
		}
		return buf.Bytes(), nil
	case FormatHuman:
		out = []byte(humanReport(findings, authority.domainRoots()))
	case FormatMarkdown:
		out = []byte(markdownReport(findings, authority.domainRoots()))
	default:
		panic(fmt.Sprintf("judge: unknown Format: %d", format))
	}
	if authority.noRules() {
		lead := noRulesChecks + "\n"
		if format == FormatMarkdown {
			lead += "\n"
		}
		out = append([]byte(lead), out...)
	}
	return out, nil
}

// CoverageOptions is the parsed coverage command.
type CoverageOptions struct {
	Root   string
	Format Format
}

// RunCoverage computes and renders coverage. It always exits 0 — coverage
// reports state, it never gates. A scan or serialization failure is returned as
// an error.
func RunCoverage(ctx context.Context, o *CoverageOptions) (stdout egress.Payload, exit int, err error) {
	return sealPrepared(prepareCoverageWithHooks(ctx, o, actionHooks{}))
}

func prepareCoverageWithHooks(ctx context.Context, o *CoverageOptions, hooks actionHooks) (preparedCommand, error) {
	if o.Format == FormatMarkdown {
		return preparedCommand{}, errors.New("coverage has no markdown view; use json or human")
	}
	a, err := openAction(ctx, o.Root, hooks)
	if err != nil {
		return preparedCommand{}, err
	}
	// A coverage report is a census: how many concepts there are, and which of
	// them nothing points at. Both are answers about the whole corpus, and the
	// payload has no field that could hold "except for the part I could not
	// read" — so a folder with a hole in it gets the refusal rather than a
	// number nobody can read correctly.
	if a.partialCorpus() {
		return preparedCommand{}, a.abort(a.unreadableRefusal())
	}
	cov := computeCoverage(a.notes, buildIndex(a.notes, a.unreadable, a.resources), a.authority)
	var stdout []byte
	if o.Format == FormatJSON {
		out, err := marshalWire(cov)
		if err != nil {
			return preparedCommand{}, a.abort(fmt.Errorf("serialize coverage: %w", err))
		}
		stdout = out
	} else {
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
func RunExists(ctx context.Context, o *ExistsOptions) (stdout egress.Payload, exit int, err error) {
	return sealPrepared(prepareExistsWithHooks(ctx, o, actionHooks{}))
}

func prepareExistsWithHooks(ctx context.Context, o *ExistsOptions, hooks actionHooks) (preparedCommand, error) {
	if o.Format == FormatMarkdown {
		return preparedCommand{}, errors.New("exists has no markdown view; use json or human")
	}
	a, err := openAction(ctx, o.Root, hooks)
	if err != nil {
		return preparedCommand{}, err
	}
	report := existsLookup(a.notes, o.Name, a.authority)
	report.NoRules = a.authority.noRules()
	if strings.Contains(o.Name, "/") {
		appendExistsPaths(&report, a.notes, buildIndex(a.notes, a.unreadable, a.resources), a.authority)
	}
	// "A note answers to this name" survives a folder with a hole in it: the
	// note that answers was read. "No note does" is the same conclusion the
	// whole-vault rules make, and it is the one a caller writes a new note on
	// the strength of — so where a file could not be read, the absence is
	// refused rather than answered, and nothing gates a write on it.
	if !report.found() && a.partialCorpus() {
		return preparedCommand{}, a.abort(a.unreadableRefusal())
	}
	var stdout []byte
	if o.Format == FormatJSON {
		out, err := marshalWire(report)
		if err != nil {
			return preparedCommand{}, a.abort(fmt.Errorf("serialize exists: %w", err))
		}
		stdout = out
	} else {
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
// forward-reference never gates through its rule id. A finding that the
// navigation roles were unavailable also gates when a rule that reads those
// roles is denied, since that rule could not have fired.
func gated(findings []Finding, deny []string) bool {
	threshold, hasThreshold := minDeniedSeverity(deny)
	for i := range findings {
		f := &findings[i]
		if hasThreshold && f.Severity >= threshold {
			return true
		}
		if f.Severity >= SeverityWarn && slices.Contains(deny, string(f.RuleID)) {
			return true
		}
		if f.RuleID == RuleSchemaNavigationUnavailable && slices.ContainsFunc(deny, silencedWithoutNavigation) {
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

// subtractBaseline drops the findings a prior run's JSONL at path already
// carried.
func subtractBaseline(findings []Finding, path string) ([]Finding, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- the baseline path is an operator-supplied CLI argument, not untrusted input
	if err != nil {
		return nil, fmt.Errorf("read baseline %s: %w", path, err)
	}
	baseline, err := parseBaseline(string(data))
	if err != nil {
		return nil, fmt.Errorf("baseline %s: %w", path, err)
	}
	return retainNew(findings, baseline), nil
}

// retainNew drops findings whose fingerprint is already in the baseline,
// leaving only what this run newly introduced.
//
// A file this run could not read is not subtracted, however old it is. It is
// not one of the findings a baseline is for — it is this run's account of where
// its own judgement has a hole, and the rules it silenced are silent in this
// output whether or not the last run saw the same file. Subtracted, it would
// leave a report that is short of those rules and says nothing about why.
func retainNew(findings []Finding, baseline map[string]bool) []Finding {
	return slices.DeleteFunc(findings, func(f Finding) bool {
		return f.RuleID != RuleScanUnreadable && baseline[f.Fingerprint]
	})
}
