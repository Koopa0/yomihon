package judge

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/koopa0/yomihon/internal/vault"
)

// The diagnostics parse untrusted vault bytes by hand at the boundaries where a
// malformed file historically slipped a wrong line number, a dropped fence, or
// a corrupted separator into the frozen output. These fuzz targets keep that
// class of fault a machine's job: each hunts crashes and asserts the invariants
// the corresponding function actually promises, and the seeds double as
// regression cases that run on every ordinary test pass.

// FuzzSplitFrontmatter checks that the frontmatter split never panics, agrees
// with itself across calls, and — when it finds a block — reports a body start
// line that matches the newline count of the bytes it consumed and returns a
// block, closing fence, and body that reconstruct the input exactly.
func FuzzSplitFrontmatter(f *testing.F) {
	f.Add([]byte(""))
	f.Add([]byte("no frontmatter body\n"))
	f.Add([]byte("---\ntitle: x\n---\nbody\n"))
	f.Add([]byte("---\r\ntitle: x\r\n---\r\nbody\r\n"))
	f.Add([]byte("---\ntitle: x\n...\nbody\n"))
	f.Add([]byte("---\n---\nbody\n"))
	f.Add([]byte("---\n---"))
	f.Add([]byte("---\na: b\n---"))
	f.Add([]byte("---\n..."))
	f.Add([]byte("---\ntitle: x\n"))
	f.Add([]byte("---\nkey: |\n  line1\n  line2\n---\nbody\n"))
	f.Add([]byte("---\nkey: >-\n  folded\n---\nbody\n"))
	f.Add([]byte("---\nkey: |+\n  keep\n\n---\nbody\n"))
	f.Add([]byte("---\n"))
	f.Add([]byte("---\nno close and ... not a fence line\n"))
	f.Add([]byte("text\n---\nnot at start\n---\n"))

	f.Fuzz(func(t *testing.T, data []byte) {
		block, found := vault.SplitFrontmatter(data)
		fm, body, bodyLine := block.Content, block.Body, block.BodyStartLine

		block2, found2 := vault.SplitFrontmatter(data)
		if found != found2 || bodyLine != block2.BodyStartLine || !bytes.Equal(fm, block2.Content) || !bytes.Equal(body, block2.Body) {
			t.Fatalf("vault.SplitFrontmatter is not deterministic for %q", data)
		}

		if !found {
			if bodyLine != 1 {
				t.Errorf("vault.SplitFrontmatter(%q) not found: bodyLine = %d, want 1", data, bodyLine)
			}
			if fm != nil {
				t.Errorf("vault.SplitFrontmatter(%q) not found: fm = %q, want nil", data, fm)
			}
			if !bytes.Equal(body, data) {
				t.Errorf("vault.SplitFrontmatter(%q) not found: body = %q, want the whole input", data, body)
			}
			return
		}

		// A found body is a suffix of the input, so the bytes before it are the
		// consumed prefix: the opening fence, the block, and the closing fence
		// line. The body begins on the line after the closing fence, so the
		// reported line is one past the number of lines in that prefix. A closing
		// fence with no trailing newline is still its own line — the body is
		// empty in that case, but the line count still advances past it.
		if !bytes.HasSuffix(data, body) {
			t.Fatalf("vault.SplitFrontmatter(%q) found: body %q is not a suffix of the input", data, body)
		}
		prefix := data[:len(data)-len(body)]
		prefixLines := bytes.Count(prefix, []byte("\n"))
		if len(prefix) > 0 && prefix[len(prefix)-1] != '\n' {
			prefixLines++
		}
		if want := 1 + prefixLines; bodyLine != want {
			t.Errorf("vault.SplitFrontmatter(%q) bodyLine = %d, want %d (one past the %d lines in %q)", data, bodyLine, want, prefixLines, prefix)
		}

		// The prefix opens with a fence, carries the returned block right after
		// it, and ends with a line that trims to a closing fence.
		openLen := openingFenceLen(data)
		if openLen == 0 {
			t.Fatalf("vault.SplitFrontmatter(%q) found without a leading fence", data)
		}
		if len(fm) > len(prefix)-openLen || !bytes.Equal(data[openLen:openLen+len(fm)], fm) {
			t.Fatalf("vault.SplitFrontmatter(%q) block %q does not sit right after the opening fence", data, fm)
		}
		closing := bytes.TrimRight(prefix[openLen+len(fm):], "\r\n")
		if string(closing) != "---" && string(closing) != "..." {
			t.Errorf("vault.SplitFrontmatter(%q) closing fence line = %q, want \"---\" or \"...\"", data, closing)
		}
	})
}

// openingFenceLen is the length of the opening frontmatter fence at the very
// start of data, or 0 when there is none.
func openingFenceLen(data []byte) int {
	switch {
	case bytes.HasPrefix(data, []byte("---\n")):
		return len("---\n")
	case bytes.HasPrefix(data, []byte("---\r\n")):
		return len("---\r\n")
	default:
		return 0
	}
}

// FuzzParseNote checks that parsing a note never panics on any frontmatter
// shape — malformed YAML, duplicate keys nested or shallow, merge keys, tagged
// scalars, and aliases — classifies every input into exactly one state, and
// yields the same note when parsing the same bytes twice.
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

		n2 := parseNote(rel, data)
		if diff := cmp.Diff(n, n2, cmp.AllowUnexported(note{}, fmValue{}, wikiLink{}, pathRef{})); diff != "" {
			t.Errorf("parseNote(%q) is not deterministic (-first +second):\n%s", data, diff)
		}
	})
}

// FuzzExtractLinks checks that link extraction never panics, that every
// reported line falls within the body's line span, that every wikilink target
// is a literal substring of the body it was scanned from, and that both
// extractors agree with themselves across calls.
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

		if diff := cmp.Diff(links, extractWikilinks(body, bodyLine), cmp.AllowUnexported(wikiLink{})); diff != "" {
			t.Errorf("extractWikilinks(%q, %d) is not deterministic (-first +second):\n%s", body, bodyLine, diff)
		}
		if diff := cmp.Diff(refs, extractPathRefs(body, bodyLine), cmp.AllowUnexported(pathRef{})); diff != "" {
			t.Errorf("extractPathRefs(%q, %d) is not deterministic (-first +second):\n%s", body, bodyLine, diff)
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
		if err := WriteJSONL(&buf, findings); err != nil {
			t.Fatalf("WriteJSONL(%d findings) = %v", len(findings), err)
		}
		out := buf.Bytes()

		// Every control character, including a newline in any field, is escaped
		// inside the JSON, so the only raw newlines are the line terminators.
		if got := bytes.Count(out, []byte("\n")); got != len(findings) {
			t.Errorf("WriteJSONL wrote %d newlines for %d findings:\n%q", got, len(findings), out)
		}
		if len(out) > 0 && !bytes.HasSuffix(out, []byte("\n")) {
			t.Errorf("WriteJSONL output does not end in a newline:\n%q", out)
		}

		lines := bytes.Split(out, []byte("\n"))
		if len(lines) != len(findings)+1 || len(lines[len(lines)-1]) != 0 {
			t.Fatalf("WriteJSONL(%d findings) did not split into one line each:\n%q", len(findings), out)
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
		t.Fatalf("WriteJSONL produced invalid JSON:\n%q", line)
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
		t.Errorf("WriteJSONL round-trip mismatch (-want +got):\n%s", diff)
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

// FuzzStripTarget checks that reducing a wikilink's inner text to its
// resolution target never panics, agrees its found flag with a non-empty
// result, keeps the target a substring of the input, and drops the delimiters
// it splits on. The reduction is not idempotent — a backslash left trailing by
// the heading or block cut is trimmed only on a later pass — so idempotence is
// deliberately not asserted; the delimiter and substring invariants are what
// the function guarantees.
func FuzzStripTarget(f *testing.F) {
	f.Add("")
	f.Add("Target")
	f.Add("Target|Display")
	f.Add("Target#heading")
	f.Add("Target^block")
	f.Add("Target|Disp#h^b")
	f.Add("#anchor-only")
	f.Add("  spaced  ")
	f.Add("a\\#b")
	f.Add("a\\|b")
	f.Add("だ")
	f.Add("name with 内部 space")

	f.Fuzz(func(t *testing.T, s string) {
		target, ok := stripTarget(s)

		if ok != (target != "") {
			t.Errorf("stripTarget(%q) = (%q, %v); ok must equal target != \"\"", s, target, ok)
		}
		if !strings.Contains(s, target) {
			t.Errorf("stripTarget(%q) = %q, which is not a substring of the input", s, target)
		}
		if strings.ContainsAny(target, "|#^") {
			t.Errorf("stripTarget(%q) = %q, which still carries a delimiter it splits on", s, target)
		}
		if trimmed := strings.TrimSpace(target); trimmed != target {
			t.Errorf("stripTarget(%q) = %q, which is not trimmed of surrounding space", s, target)
		}
	})
}
