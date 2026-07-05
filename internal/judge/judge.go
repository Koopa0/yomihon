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
// as they are even where kurodo's own text would otherwise be English
// only.
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

// name returns the wire name of the severity: "info", "warn", or "error". It
// is the single source for that spelling, shared by the JSONL encoder and the
// human and markdown reports. A value outside the three constants is a
// programming error, so it panics rather than yielding bytes consumers cannot
// parse.
func (s Severity) name() string {
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
	return []byte(s.name()), nil
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

// WriteJSONL writes findings to w, one compact JSON object and a
// trailing newline per finding. It is the only serialization path for
// findings in this repository, because the frozen format differs from
// what encoding/json produces by default in two ways: it leaves HTML
// characters unescaped, which SetEscapeHTML(false) covers, and it
// carries U+2028 and U+2029 as raw UTF-8, which the encoder offers no
// switch for, so those two escape sequences are rewritten after
// encoding. A plain json.Marshal elsewhere would reintroduce both
// divergences and corrupt, for example, every message containing "->".
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
