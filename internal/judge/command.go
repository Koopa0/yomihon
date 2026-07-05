package judge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
)

// The three subcommands — check, coverage, exists — run as stateless scans: a
// process walks the vault fresh, prints, and exits. There is no server, no
// daemon, and no persistent store behind them. Each runner takes already-parsed
// options and returns the bytes to print and the process exit code, so the
// binary's own main only wires arguments in and the exit code out, and every
// exit-code decision is exercised by a test.
//
// Exit codes match the frozen contract the pipelines depend on: 0 clean, 1 a
// gate hit or a "does not exist", 2 a tool error. A tool error is returned as a
// non-nil error, which the binary reports and turns into exit 2.

// ruleIDs is every rule a finding can carry. A --deny value is validated
// against it so a typo fails loudly instead of silently disabling the gate.
var ruleIDs = []string{
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

// RunCheck scans the vault and renders the findings. It returns the bytes to
// print and the exit code: 1 when a finding gates, 0 otherwise. An unknown
// --deny token, an unreadable baseline, or a scan failure is returned as an
// error, which the caller turns into a tool-error exit.
func RunCheck(o *CheckOptions) (stdout []byte, exit int, err error) {
	for _, d := range o.Deny {
		if !isSeverityKeyword(d) && !slices.Contains(ruleIDs, d) {
			return nil, 0, fmt.Errorf("unknown --deny %q; use a severity (error|warn|info) or a rule id", d)
		}
	}
	findings, err := check(o.Root, o.Paths, o.All)
	if err != nil {
		return nil, 0, err
	}
	if o.Baseline != "" {
		data, err := os.ReadFile(o.Baseline) // #nosec G304 -- the baseline path is an operator-supplied CLI argument, not untrusted input
		if err != nil {
			return nil, 0, fmt.Errorf("read baseline %s: %w", o.Baseline, err)
		}
		findings = retainNew(findings, parseBaseline(string(data)))
	}
	switch o.Format {
	case FormatJSON:
		var buf bytes.Buffer
		if err := WriteJSONL(&buf, findings); err != nil {
			return nil, 0, fmt.Errorf("serialize findings: %w", err)
		}
		stdout = buf.Bytes()
	case FormatHuman:
		stdout = []byte(humanReport(findings))
	case FormatMarkdown:
		stdout = []byte(markdownReport(findings))
	default:
		panic("judge: unknown Format: " + strconv.Itoa(int(o.Format)))
	}
	if gated(findings, o.Deny) {
		return stdout, 1, nil
	}
	return stdout, 0, nil
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
	notes, resources, err := collectVault(o.Root)
	if err != nil {
		return nil, 0, err
	}
	cov := computeCoverage(notes, buildIndex(notes, resources))
	if o.Format == FormatJSON {
		out, err := marshalWire(cov)
		if err != nil {
			return nil, 0, fmt.Errorf("serialize coverage: %w", err)
		}
		return out, 0, nil
	}
	// md is a check-only format; coverage falls back to the human view.
	return []byte(renderCoverage(&cov)), 0, nil
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
	notes, err := collectNotes(o.Root)
	if err != nil {
		return nil, 0, err
	}
	report := existsLookup(notes, o.Name)
	if o.Format == FormatJSON {
		out, err := marshalWire(report)
		if err != nil {
			return nil, 0, fmt.Errorf("serialize exists: %w", err)
		}
		stdout = out
	} else {
		// md is a check-only format; exists falls back to the human view.
		stdout = []byte(renderExists(report))
	}
	if report.found() {
		return stdout, 0, nil
	}
	return stdout, 1, nil
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
	out := findings[:0]
	for i := range findings {
		if !baseline[findings[i].Fingerprint] {
			out = append(out, findings[i])
		}
	}
	return out
}

// marshalWire encodes v as a compact JSON object and a trailing newline, the
// on-wire form for coverage and exists. It matches the JSONL encoder's two
// departures from the encoder's defaults: HTML characters are left unescaped,
// and the two line-separator code points are carried as raw UTF-8.
func marshalWire(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return unescapeLineSeparators(buf.Bytes()), nil
}
