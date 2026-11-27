package judge

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// The diagnostics parse untrusted vault bytes, so these fuzz targets hunt
// crashes and assert the invariants each function promises; the seeds run on
// every ordinary test pass.

// FuzzParseNote checks that parsing a note never panics on any frontmatter
// shape — malformed YAML, duplicate keys nested or shallow, merge keys, tagged
// scalars, and aliases — and classifies every input into exactly one state.
func FuzzParseNote(f *testing.F) {
	f.Add([]byte(""))
	f.Add([]byte("just a transcript\n"))
	f.Add([]byte("---\ntitle: X\ntype: concept\n---\nbody\n"))
	f.Add([]byte("---\ntitle: \"unclosed\n---\nbody\n"))
	f.Add([]byte("---\na: 1\na: 2\n---\nbody\n"))
	f.Add([]byte("---\nouter:\n  b: 1\n  b: 2\n---\n"))
	f.Add([]byte("---\nbase: &b {x: 1}\nmerged:\n  <<: *b\n  y: 2\n---\n"))
	f.Add([]byte("---\nanchored: &a hi\nref: *a\n---\n"))
	f.Add([]byte("---\nnested: &a [*a]\n---\n"))
	f.Add([]byte("---\ntype: !!str concept\nn: !!int 5\nb: !!bool true\n---\n"))
	f.Add([]byte("---\nx: yes\ny: no\nz: on\nw: off\n---\n"))
	f.Add([]byte("---\nnum: 007\nhex: 0xFF\n---\n"))
	f.Add([]byte("---\n---\nbody\n"))
	f.Add([]byte("---\n- a\n- b\n---\n"))
	f.Add([]byte("---\ntitle:\n---\n"))
	f.Add([]byte("> [!note] A titled callout\n> body\n"))

	f.Fuzz(func(t *testing.T, data []byte) {
		const rel = "Concepts/fuzz.md"
		n := parseNote(rel, data)

		// The three states are mutually exclusive: a note is one of no
		// frontmatter, bad frontmatter, or parsed. The frontmatter map is
		// populated in the parsed state alone.
		if n.noFrontmatter && n.badFrontmatter {
			t.Errorf("parseNote(%q) is flagged both no-frontmatter and bad-frontmatter", data)
		}
		parsed := !n.noFrontmatter && !n.badFrontmatter
		if parsed != (n.frontmatter != nil) {
			t.Errorf("parseNote(%q) parsed = %v but frontmatter != nil = %v; they must agree", data, parsed, n.frontmatter != nil)
		}
	})
}

// FuzzExtractLinks checks that link extraction never panics, that every
// reported line falls within the body's line span, and that every wikilink
// target is a literal substring of the body it was scanned from.
func FuzzExtractLinks(f *testing.F) {
	f.Add("", uint8(0))
	f.Add("see [[Note]] here\n", uint8(1))
	f.Add("[[Target|Display]] and [[A#heading]] and [[B^block]]\n", uint8(1))
	f.Add("[[#anchor-only]]\n", uint8(1))
	f.Add("%%[[hidden]]%% visible [[Shown]]\n", uint8(1))
	f.Add("```\n[[not a link]]\n```\n[[Real]]\n", uint8(1))
	f.Add("`code [[x]]` [[y]]\n", uint8(1))
	f.Add("[label](notes/file.md) and `dir/path.md`\n", uint8(1))
	f.Add("[[a\\|b]] table escape\n", uint8(1))
	f.Add("## 缺口\n- Concept A、Concept B\n[[Planned]]\n", uint8(1))
	f.Add("下一課 [[Next]]\n", uint8(1))
	f.Add("unterminated [[ open\n", uint8(1))
	f.Add("line1\nline2\n[[OnLine3]]\n", uint8(1))
	f.Add("\u2028[[X]]\n", uint8(1))

	f.Fuzz(func(t *testing.T, body string, start uint8) {
		// The body always begins on a positive file line in practice; a small
		// offset exercises the line arithmetic without risking an overflow that
		// the real caller never produces.
		bodyLine := 1 + int(start)
		maxLine := bodyLine + strings.Count(body, "\n")

		links := extractWikilinks(body, bodyLine)
		for _, wl := range links {
			if wl.line < bodyLine || wl.line > maxLine {
				t.Errorf("extractWikilinks(%q, %d) target %q line = %d, want within [%d, %d]", body, bodyLine, wl.target, wl.line, bodyLine, maxLine)
			}
			if !strings.Contains(body, wl.target) {
				t.Errorf("extractWikilinks(%q, %d) reported target %q that is not a substring of the body", body, bodyLine, wl.target)
			}
		}

		refs := extractPathRefs(body, bodyLine)
		for _, pr := range refs {
			if pr.line < bodyLine || pr.line > maxLine {
				t.Errorf("extractPathRefs(%q, %d) target %q line = %d, want within [%d, %d]", body, bodyLine, pr.target, pr.line, bodyLine, maxLine)
			}
		}
	})
}

// wireFinding mirrors Finding's JSON shape with the severity read back as its
// wire name, so a serialized finding can be decoded and compared even though
// the severity type carries no text-unmarshaler.
type wireFinding struct {
	RuleID           string   `json:"rule_id"`
	Severity         string   `json:"severity"`
	Path             string   `json:"path"`
	Line             *int     `json:"line,omitempty"`
	Field            *string  `json:"field,omitempty"`
	Message          string   `json:"message"`
	Evidence         string   `json:"evidence"`
	SuggestedAction  string   `json:"suggested_action"`
	SourceRule       string   `json:"source_rule"`
	Target           *string  `json:"target,omitempty"`
	ResolvedTo       *string  `json:"resolved_to,omitempty"`
	CollisionMembers []string `json:"collision_members,omitempty"`
	Fingerprint      string   `json:"fingerprint"`
}

// FuzzWriteJSONL checks that the finding encoder emits exactly one
// newline-terminated JSON line per finding, that each line decodes back to the
// value it was written from, and that the two line separators and the HTML
// characters ride as raw UTF-8 rather than as their JSON escapes — the two
// places the frozen format departs from the standard encoder.
func FuzzWriteJSONL(f *testing.F) {
	f.Add("rule.a", "Concepts/x.md", "message", "evidence", "fix it", "Target", uint8(0), false, 0, false)
	f.Add("rule.b", "Concepts/y.md", "arrow -> here", "ev", "act", "T", uint8(1), true, 5, true)
	f.Add("r", "p", "sep\u2028here\u2029end", "e", "a", "t", uint8(2), false, 0, false)
	f.Add("r", "p", "html <b> & </b>", "e>", "a<", "t&", uint8(1), true, 12, true)
	f.Add("r", "p", "newline\nin\tmsg", "e", "a", "t", uint8(0), true, 1, false)
	f.Add("r", "p", "literal \\u2028 text", "e", "a", "t", uint8(2), false, 0, true)

	f.Fuzz(func(t *testing.T, ruleID, pth, message, evidence, suggested, target string, sev uint8, hasLine bool, line int, two bool) {
		// The severity is one of the three defined weights; any other value is a
		// programming error the encoder is entitled to reject.
		one := Finding{
			RuleID:          RuleID(ruleID),
			Severity:        Severity(sev % 3),
			Path:            pth,
			Message:         message,
			Evidence:        evidence,
			SuggestedAction: suggested,
			SourceRule:      "fuzz",
			Fingerprint:     "0000000000000000",
		}
		if hasLine {
			one.Line = &line
		}
		if target != "" {
			one.Target = &target
		}
		findings := []Finding{one}
		if two {
			second := one
			second.RuleID = RuleID(ruleID + ".2")
			findings = append(findings, second)
		}

		var buf bytes.Buffer
		if err := writeJSONL(&buf, findings); err != nil {
			t.Fatalf("writeJSONL(%d findings) = %v", len(findings), err)
		}
		out := buf.Bytes()

		// Every control character, including a newline in any field, is escaped
		// inside the JSON, so the only raw newlines are the line terminators.
		if got := bytes.Count(out, []byte("\n")); got != len(findings) {
			t.Errorf("writeJSONL wrote %d newlines for %d findings:\n%q", got, len(findings), out)
		}
		if len(out) > 0 && !bytes.HasSuffix(out, []byte("\n")) {
			t.Errorf("writeJSONL output does not end in a newline:\n%q", out)
		}

		lines := bytes.Split(out, []byte("\n"))
		if len(lines) != len(findings)+1 || len(lines[len(lines)-1]) != 0 {
			t.Fatalf("writeJSONL(%d findings) did not split into one line each:\n%q", len(findings), out)
		}
		for i := range findings {
			assertLineRoundTrips(t, &findings[i], lines[i])
		}

		// The line separators and the HTML characters ride as raw bytes rather
		// than their JSON escapes. Counting each raw occurrence against the total
		// the fields carry catches a single escaped field even when another field
		// holds the same character raw, which a bare presence check would let
		// slip. Round-tripping alone would pass on either form, so this pins the
		// wire choice directly.
		assertSpecialsRideRaw(t, findings, out)
	})
}

// assertLineRoundTrips checks that one serialized finding decodes back to the
// value it was written from.
func assertLineRoundTrips(t *testing.T, want *Finding, line []byte) {
	t.Helper()
	if !json.Valid(line) {
		t.Fatalf("writeJSONL produced invalid JSON:\n%q", line)
	}
	// The encoder coerces invalid UTF-8 to the replacement rune, so an exact
	// round-trip is only defined for input that is already valid UTF-8.
	if !allValidUTF8(want) {
		return
	}
	var got wireFinding
	if err := json.Unmarshal(line, &got); err != nil {
		t.Fatalf("decoding %q: %v", line, err)
	}
	expected := wireFinding{
		RuleID:           string(want.RuleID),
		Severity:         want.Severity.String(),
		Path:             want.Path,
		Line:             want.Line,
		Field:            want.Field,
		Message:          want.Message,
		Evidence:         want.Evidence,
		SuggestedAction:  want.SuggestedAction,
		SourceRule:       want.SourceRule,
		Target:           want.Target,
		ResolvedTo:       want.ResolvedTo,
		CollisionMembers: want.CollisionMembers,
		Fingerprint:      want.Fingerprint,
	}
	if diff := cmp.Diff(expected, got, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("writeJSONL round-trip mismatch (-want +got):\n%s", diff)
	}
}

// assertSpecialsRideRaw checks that every line separator and HTML character the
// findings carry reaches the output as its raw bytes: the number of raw
// occurrences in the output equals the number the fields hold, so an escaped
// occurrence in any one field is caught rather than masked by a raw occurrence
// in another.
func assertSpecialsRideRaw(t *testing.T, findings []Finding, out []byte) {
	t.Helper()
	for _, c := range []struct {
		r    rune
		name string
	}{
		{'\u2028', "U+2028"},
		{'\u2029', "U+2029"},
		{'<', "less-than"},
		{'>', "greater-than"},
		{'&', "ampersand"},
	} {
		want := 0
		for i := range findings {
			for _, s := range findingStrings(&findings[i]) {
				want += strings.Count(s, string(c.r))
			}
		}
		if got := bytes.Count(out, []byte(string(c.r))); got != want {
			t.Errorf("output carries %s %d time(s), want %d across the findings' fields:\n%q", c.name, got, want, out)
		}
	}
}

// findingStrings returns every string field a finding serializes, so a check
// can range over exactly the text the encoder writes.
func findingStrings(fnd *Finding) []string {
	s := []string{string(fnd.RuleID), fnd.Path, fnd.Message, fnd.Evidence, fnd.SuggestedAction, fnd.SourceRule, fnd.Fingerprint}
	if fnd.Field != nil {
		s = append(s, *fnd.Field)
	}
	if fnd.Target != nil {
		s = append(s, *fnd.Target)
	}
	if fnd.ResolvedTo != nil {
		s = append(s, *fnd.ResolvedTo)
	}
	return append(s, fnd.CollisionMembers...)
}

// allValidUTF8 reports whether every string field of a finding is valid UTF-8.
func allValidUTF8(fnd *Finding) bool {
	for _, s := range findingStrings(fnd) {
		if !utf8.ValidString(s) {
			return false
		}
	}
	return true
}
