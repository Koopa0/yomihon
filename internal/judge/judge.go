// Package judge implements the vault diagnostics behind the check,
// exists, and coverage commands, and their JSONL wire format.
//
// The wire format is frozen. External pipelines parse the emitted lines
// and exit codes byte for byte, so every shape in this package — field
// order, omitted fields, escaping, hashing — is a compatibility
// contract rather than a design choice. The golden files under
// testdata/golden/ pin the exact bytes, and the tests assert that this
// package reproduces them.
//
// Diagnostic strings, including mixed Chinese and English text, are
// part of the frozen format. Rewording, translating, or reformatting
// them changes bytes that consumers match against, so they stay exactly
// as they are even where yomihon's own text would otherwise be English
// only.
//
// The commands are not the only consumer. The reading server builds its
// pages from six things here: Finding and LintFrontmatter for the
// notices printed beside a note, LinkTargets for the reverse of the link
// graph, and Planned with NewPlanned and its Has for telling a link the
// corpus has promised to write from one that is simply broken. That
// second face is why those readings live here rather than in the
// commands: a page and a finding that disagreed about which links exist,
// or about which broken link is a fault, would teach the reader to
// distrust whichever one cried wolf. It is also why they are the
// package's exported surface while the engines' own options are not.
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
// constants must remain in ascending order of weight.
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

// String returns the wire name of the severity: "info", "warn", or "error". It
// is the single source for that spelling, shared by the JSONL encoder and the
// human and markdown reports. A value outside the three constants is a
// programming error, so it panics rather than yielding bytes consumers cannot
// parse.
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

// Finding is one diagnostic, serialized as one JSONL line. The struct
// layout is the wire contract: fields serialize in declaration order,
// and only the four pointer fields and the slice are omitted when
// empty. Reordering fields, renaming a JSON key, or adding or removing
// an omitempty option changes frozen bytes.
type Finding struct {
	RuleID   string   `json:"rule_id"`
	Severity Severity `json:"severity"`
	// Path is the file that carries the finding, relative to the vault
	// root.
	Path string `json:"path"`
	// Line is the 1-based body line the finding points at, or nil when
	// the finding is not tied to a line.
	Line *int `json:"line,omitempty"`
	// Field names the frontmatter field at fault, when one is.
	Field *string `json:"field,omitempty"`
	// Message, Evidence, SuggestedAction, and SourceRule are frozen
	// diagnostic strings; see the package comment.
	Message         string `json:"message"`
	Evidence        string `json:"evidence"`
	SuggestedAction string `json:"suggested_action"`
	SourceRule      string `json:"source_rule"`
	// Target is the original link or value text, kept structured so
	// consumers need no prose parsing.
	Target *string `json:"target,omitempty"`
	// ResolvedTo is the path the target resolved to; nil means it did
	// not resolve.
	ResolvedTo *string `json:"resolved_to,omitempty"`
	// CollisionMembers lists every path involved in a name collision,
	// so a single finding describes the whole collision.
	CollisionMembers []string `json:"collision_members,omitempty"`
	// Fingerprint identifies the finding across runs; see
	// fingerprint.go.
	Fingerprint string `json:"fingerprint"`
}

// The complete set of values SourceRule may carry. A finding points a reader
// at where its rule's authority is written down, so each of these has to name
// a thing that holds it: a vault artifact is spelled the way the vault spells
// it, an anchor is a table the contract really declares and whose keys the
// rule really reads, and a rule no vault artifact declares names the product
// itself, whose golden files pin the behaviour. Anchors were once invented —
// a heading the vault's note schema does not have, a contract table nothing
// declares — and rules were hung on artifacts that never state them, which
// left the field reading as authority while resolving to nothing. Declaring
// the set in one place is what makes adding another an edit somebody has to
// make deliberately.
const (
	// sourceContract is the vault's machine contract, for the frontmatter
	// rules that read its type, field, and status declarations across
	// several of its tables.
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
	// dialect — link resolution, name and alias collisions, reference and
	// path liveness, and the syllabus-versus-disk reconciliation. No vault
	// artifact declares them; this repository's golden files pin them.
	sourceYomihon = "yomihon"
	// sourceAuthoring is this repository's authoring contract, which ships
	// with the parser that reads it.
	sourceAuthoring = "AUTHORING.md"
)

// WriteJSONL writes findings to w, one compact JSON object and a
// trailing newline per finding. It is the only serialization path for
// findings in this repository, because the frozen format differs from
// what encoding/json produces by default in two ways: it leaves HTML
// characters unescaped, which SetEscapeHTML(false) covers, and it
// carries U+2028 and U+2029 as raw UTF-8, which the encoder offers no
// switch for, so those two escape sequences are rewritten after
// encoding. A plain json.Marshal elsewhere would reintroduce both
// divergences and corrupt, for example, every message containing "->".
// The coverage and exists payloads are not findings and go out through
// marshalWire below, which makes the same two departures.
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
// on-wire form for the coverage and exists payloads. It sits here because it
// makes the same two departures from the encoder's defaults that WriteJSONL
// documents above — HTML characters left unescaped, the two line-separator code
// points carried as raw UTF-8 — and those two departures are the whole reason
// this package encodes anything by hand. A third payload type reaches for this
// rather than growing a third copy of them in a third file.
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
// line zero, before line one of the same path), then by rule id. Comparison
// is bytewise on the UTF-8 of path and rule id, and the sort is stable, so
// findings that tie on all three keep the order the checks produced them in.
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

// unescapeLineSeparators rewrites the escape sequences the encoder
// produces for U+2028 and U+2029 into the raw UTF-8 bytes the frozen
// format carries. It steps over escape sequences pairwise, so a literal
// backslash in a value — which the encoder renders as a doubled
// backslash — cannot be misread as the start of one of the two
// sequences.
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
