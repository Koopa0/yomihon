// Package judge implements the vault diagnostics behind the check, exists and
// coverage commands, and their JSONL wire format; the reading server consumes
// the same findings, so a page and a command cannot disagree. The format is
// frozen — external pipelines parse the lines and exit codes byte for byte,
// the goldens pin them, and the diagnostic strings are part of those bytes.
package judge

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
)

// Severity classifies one diagnostic. The order is significant: gating
// compares a finding's severity against a deny threshold, so the three
// constants ascend by weight.
type Severity int

const (
	// SeverityInfo marks listed gaps, tracked forward references, and
	// formatting hints. It never fails a run.
	SeverityInfo Severity = iota
	// SeverityWarn marks broken links, alias collisions, and similar
	// findings that fail a run only when denied by configuration.
	SeverityWarn
	// SeverityError marks schema violations, which a gating run must
	// fail on.
	SeverityError
)

// String returns the wire name of the severity: "info", "warn" or "error". It
// is the single source for that spelling, shared by the JSONL encoder and the
// human and markdown reports; a value outside the three constants panics.
func (s Severity) String() string {
	switch s {
	case SeverityInfo:
		return "info"
	case SeverityWarn:
		return "warn"
	case SeverityError:
		return "error"
	default:
		panic("judge: unknown Severity: " + strconv.Itoa(int(s)))
	}
}

// MarshalText returns the wire name of the severity for the JSONL encoder.
func (s Severity) MarshalText() ([]byte, error) {
	return []byte(s.String()), nil
}

// Finding is one diagnostic, serialized as one JSONL line. The struct layout
// is the wire contract: fields serialize in declaration order, and only the
// four pointer fields and the slice are omitted when empty. Reordering fields,
// renaming a key, or changing an omitempty option changes frozen bytes.
type Finding struct {
	RuleID   string   `json:"rule_id"`
	Severity Severity `json:"severity"`
	// Path is the file that carries the finding, relative to the vault root.
	Path string `json:"path"`
	// Line is the 1-based body line the finding points at, or nil when the
	// finding is not tied to a line.
	Line *int `json:"line,omitempty"`
	// Field names the frontmatter field at fault, when one is.
	Field *string `json:"field,omitempty"`
	// Message, Evidence, SuggestedAction and SourceRule are frozen diagnostic
	// strings.
	Message         string `json:"message"`
	Evidence        string `json:"evidence"`
	SuggestedAction string `json:"suggested_action"`
	SourceRule      string `json:"source_rule"`
	// Target is the original link or value text, kept structured so consumers
	// need no prose parsing.
	Target *string `json:"target,omitempty"`
	// ResolvedTo is the path the target resolved to; nil means it did not resolve.
	ResolvedTo *string `json:"resolved_to,omitempty"`
	// CollisionMembers lists every path involved in a name collision, so one
	// finding describes the whole collision.
	CollisionMembers []string `json:"collision_members,omitempty"`
	// Fingerprint identifies the finding across runs.
	Fingerprint string `json:"fingerprint"`
}

// The complete set of values SourceRule may carry. A finding points a reader
// at where its rule's authority is written down, so each of these names
// something that really holds it: an artifact spelled the way the vault spells
// it, an anchor a table really declares, or the product itself.
const (
	// sourceContract is the vault's machine contract, for the frontmatter
	// rules that read its type, field and status declarations.
	sourceContract = "vault-schema.toml"
	// sourceContractRules is its [rules] table, for a rule that enforces a
	// key that table declares.
	sourceContractRules = "vault-schema.toml#rules"
	// sourceContractScan is its [scan] table, which declares the knowledge
	// directories the frontmatter rules govern.
	sourceContractScan = "vault-schema.toml#scan"
	// sourceContractSupersession is its [supersession] table, which names
	// the replacement-ledger fields and the archived status.
	sourceContractSupersession = "vault-schema.toml#supersession"
	// sourceYomihon is the product itself, for the rules that are its own
	// dialect — link resolution, name and alias collisions, reference and path
	// liveness, and the study-path grammar. No vault artifact declares them;
	// this repository's goldens do.
	sourceYomihon = "yomihon"
)

// WriteJSONL writes findings to w, one compact JSON object and a trailing
// newline per finding. It is the only serialization path for findings here,
// because the frozen format departs from encoding/json's defaults twice: HTML
// characters are left unescaped, and U+2028 and U+2029 are carried as raw
// UTF-8, which the encoder offers no switch for and which is rewritten after
// encoding. A plain json.Marshal elsewhere would reintroduce both.
func WriteJSONL(w io.Writer, findings []Finding) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	for i := range findings {
		buf.Reset()
		if err := enc.Encode(&findings[i]); err != nil {
			return fmt.Errorf("encode finding %d: %w", i, err)
		}
		if _, err := w.Write(unescapeLineSeparators(buf.Bytes())); err != nil {
			return fmt.Errorf("write finding %d: %w", i, err)
		}
	}
	return nil
}

// marshalWire encodes v as a compact JSON object and a trailing newline, the
// on-wire form for the coverage and exists payloads. It makes the same two
// departures from the encoder's defaults that WriteJSONL names.
func marshalWire(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return unescapeLineSeparators(buf.Bytes()), nil
}

// sortFindings orders findings into the deterministic total order the wire
// format is emitted in: by path, then by line (a line-less finding sorts as
// line zero), then by rule id. Comparison is bytewise on the UTF-8 of path and
// rule id, and the sort is stable, so ties keep the order the checks produced.
func sortFindings(findings []Finding) {
	line := func(f *Finding) int {
		if f.Line != nil {
			return *f.Line
		}
		return 0
	}
	slices.SortStableFunc(findings, func(a, b Finding) int {
		if c := strings.Compare(a.Path, b.Path); c != 0 {
			return c
		}
		if c := cmp.Compare(line(&a), line(&b)); c != 0 {
			return c
		}
		return strings.Compare(a.RuleID, b.RuleID)
	})
}

// unescapeLineSeparators rewrites the escape sequences the encoder produces
// for U+2028 and U+2029 into the raw UTF-8 bytes the frozen format carries. It
// steps over escape sequences pairwise, so a doubled backslash in a value
// cannot be misread as the start of one of the two sequences.
func unescapeLineSeparators(line []byte) []byte {
	if !bytes.Contains(line, []byte(`\u202`)) {
		return line
	}
	out := make([]byte, 0, len(line))
	for i := 0; i < len(line); i++ {
		if line[i] != '\\' || i+1 == len(line) {
			out = append(out, line[i])
			continue
		}
		if line[i+1] == 'u' && i+6 <= len(line) {
			switch string(line[i+2 : i+6]) {
			case "2028":
				out = append(out, 0xe2, 0x80, 0xa8)
				i += 5
				continue
			case "2029":
				out = append(out, 0xe2, 0x80, 0xa9)
				i += 5
				continue
			}
		}
		out = append(out, line[i], line[i+1])
		i++
	}
	return out
}
