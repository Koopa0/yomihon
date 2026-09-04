package judge

import (
	"bytes"
	"encoding/hex"
	"os"
	"testing"
)

// TestWriteJSONLGolden pins the serializer's escape surface in isolation from
// the engine: every Finding below is a hand transcription, so the bytes come
// only from WriteJSONL, not from extraction. The full-engine conformance —
// which fixture produces which finding — lives in TestCheckGolden. Keeping a
// hand-authored escape case guards the serializer even if extraction is later
// reworked, since the custom serializer exists precisely to keep the two
// line-separator code points as raw UTF-8 and to leave HTML characters
// unescaped.
func TestWriteJSONLGolden(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// golden is the file holding the expected bytes for this case.
		golden   string
		findings []Finding
	}{
		{
			// Broken links whose targets carry control characters and the
			// two line-separator code points. Pins the escape surface of
			// the wire format: 0x08 and 0x0C as their two-character
			// escapes, other control characters as four-digit lowercase
			// escapes, U+2028 and U+2029 as raw UTF-8 bytes, and a
			// fingerprint whose leading nibble is zero, locking the
			// sixteen-digit zero-padded rendering. The last target holds
			// the literal characters backslash-u-2-0-2-8 instead of the
			// code point, which must survive as written and not be folded
			// back into the code point.
			name:   "escapes",
			golden: "testdata/golden/escapes.jsonl",
			findings: []Finding{
				{
					RuleID:          "link.broken",
					Severity:        SeverityWarn,
					Path:            "note.md",
					Line:            new(1),
					Message:         "[[Ba\bck]] resolves to no note",
					Evidence:        "no filename or alias matches the target",
					SuggestedAction: "create the target note, or change the link to an existing filename/alias",
					SourceRule:      sourceYomihon,
					Target:          new("Ba\bck"),
					Fingerprint:     "v1:626d4de5ae0f4e0f",
				},
				{
					RuleID:          "link.broken",
					Severity:        SeverityWarn,
					Path:            "note.md",
					Line:            new(1),
					Message:         "[[Gho\fst]] resolves to no note",
					Evidence:        "no filename or alias matches the target",
					SuggestedAction: "create the target note, or change the link to an existing filename/alias",
					SourceRule:      sourceYomihon,
					Target:          new("Gho\fst"),
					Fingerprint:     "v1:7492bcd08f83d54b",
				},
				{
					RuleID:          "link.broken",
					Severity:        SeverityWarn,
					Path:            "note.md",
					Line:            new(1),
					Message:         "[[A\x1bZ]] resolves to no note",
					Evidence:        "no filename or alias matches the target",
					SuggestedAction: "create the target note, or change the link to an existing filename/alias",
					SourceRule:      sourceYomihon,
					Target:          new("A\x1bZ"),
					Fingerprint:     "v1:5f582abb3bae406e",
				},
				{
					RuleID:          "link.broken",
					Severity:        SeverityWarn,
					Path:            "note.md",
					Line:            new(1),
					Message:         "[[L\u2028S]] resolves to no note",
					Evidence:        "no filename or alias matches the target",
					SuggestedAction: "create the target note, or change the link to an existing filename/alias",
					SourceRule:      sourceYomihon,
					Target:          new("L\u2028S"),
					Fingerprint:     "v1:e657c404685dae8d",
				},
				{
					RuleID:          "link.broken",
					Severity:        SeverityWarn,
					Path:            "note.md",
					Line:            new(1),
					Message:         "[[P\u2029S]] resolves to no note",
					Evidence:        "no filename or alias matches the target",
					SuggestedAction: "create the target note, or change the link to an existing filename/alias",
					SourceRule:      sourceYomihon,
					Target:          new("P\u2029S"),
					Fingerprint:     "v1:966f74ea2b04b126",
				},
				{
					RuleID:          "link.broken",
					Severity:        SeverityWarn,
					Path:            "note.md",
					Line:            new(1),
					Message:         "[[Zero 2 Padding]] resolves to no note",
					Evidence:        "no filename or alias matches the target",
					SuggestedAction: "create the target note, or change the link to an existing filename/alias",
					SourceRule:      sourceYomihon,
					Target:          new("Zero 2 Padding"),
					Fingerprint:     "v1:0fa417727bc33bfd",
				},
				{
					// A target holding the literal six characters
					// backslash-u-2-0-2-8 (not the code point) round-trips
					// unchanged: the encoder doubles the backslash to
					// \\u2028, and the line-separator rewrite steps over
					// that pair rather than mistake it for the code point.
					RuleID:          "link.broken",
					Severity:        SeverityWarn,
					Path:            "note.md",
					Line:            new(2),
					Message:         `[[Esc\u2028End]] resolves to no note`,
					Evidence:        "no filename or alias matches the target",
					SuggestedAction: "create the target note, or change the link to an existing filename/alias",
					SourceRule:      sourceYomihon,
					Target:          new(`Esc\u2028End`),
					Fingerprint:     "v1:58a23378d6cc4c43",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			want, err := os.ReadFile(tt.golden)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			var buf bytes.Buffer
			if err := WriteJSONL(&buf, tt.findings); err != nil {
				t.Fatalf("WriteJSONL: %v", err)
			}
			if got := buf.Bytes(); !bytes.Equal(got, want) {
				t.Errorf("output differs from golden %s\ngot:\n%s\nwant:\n%s\ngot hex:\n%s\nwant hex:\n%s",
					tt.golden, got, want, hex.Dump(got), hex.Dump(want))
			}
		})
	}
}

// TestWriteJSONLShape pins two wire shapes no golden line covers in
// one place: a finding with every field set, which locks the full
// thirteen-field order in a single line and leaves < & > unescaped,
// and a finding of nothing but empty always-present strings, which
// must all still serialize while the empty non-nil slice is omitted.
func TestWriteJSONLShape(t *testing.T) {
	t.Parallel()

	findings := []Finding{
		{
			RuleID:           "x.rule",
			Severity:         SeverityError,
			Path:             "p/a.md",
			Line:             new(7),
			Field:            new("f"),
			Message:          "m <&> -> ok",
			Evidence:         "e",
			SuggestedAction:  "s",
			SourceRule:       "r",
			Target:           new("t"),
			ResolvedTo:       new("q/b.md"),
			CollisionMembers: []string{"p/a.md", "q/b.md"},
			Fingerprint:      "0000000000000000",
		},
		{
			Severity:         SeverityInfo,
			CollisionMembers: []string{}, // an empty non-nil slice is also omitted
		},
	}
	want := `{"rule_id":"x.rule","severity":"error","path":"p/a.md","line":7,"field":"f","message":"m <&> -> ok","evidence":"e","suggested_action":"s","source_rule":"r","target":"t","resolved_to":"q/b.md","collision_members":["p/a.md","q/b.md"],"fingerprint":"0000000000000000"}` + "\n" +
		`{"rule_id":"","severity":"info","path":"","message":"","evidence":"","suggested_action":"","source_rule":"","fingerprint":""}` + "\n"
	var buf bytes.Buffer
	if err := WriteJSONL(&buf, findings); err != nil {
		t.Fatalf("WriteJSONL: %v", err)
	}
	if got := buf.String(); got != want {
		t.Errorf("shape bytes differ\ngot:\n%s\nwant:\n%s\ngot hex:\n%s\nwant hex:\n%s",
			got, want, hex.Dump([]byte(got)), hex.Dump([]byte(want)))
	}
}

// Every expected value is the fingerprint field of a golden line (see
// TestWriteJSONLGolden). Which path and target each rule feeds is part
// of the frozen contract, so the cases record one feed shape each,
// including the separator edge cases: an empty path, a separator
// embedded in the target, and a target that begins with it.
func TestFingerprint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		ruleID       RuleID
		path, target string
		want         string
	}{
		{
			// An alias collision feeds an empty path and the normalized
			// (trimmed, composed, lowercased) alias — check.jsonl line 1.
			name:   "empty path (collision.alias)",
			ruleID: "collision.alias", path: "", target: "shared",
			want: "v1:c6a289a5d2524c77",
		},
		{
			// check.jsonl line 2.
			name:   "plain link target (link.broken)",
			ruleID: "link.broken", path: "Concepts/golang/A.md", target: "Ghost",
			want: "v1:1e6dec2ff85a905f",
		},
		{
			// A CJK target hashes as its raw UTF-8 bytes — check.jsonl
			// line 3.
			name:   "CJK target (link.title_not_alias)",
			ruleID: "link.title_not_alias", path: "Concepts/golang/A.md", target: "Go Slice 內部結構",
			want: "v1:f4980b309f407345",
		},
		{
			// A provenance reference keeps its [[...]] wrapper —
			// check.jsonl line 4.
			name:   "raw wikilink target (provenance.unresolved)",
			ruleID: "provenance.unresolved", path: "Concepts/golang/B.md", target: "[[Missing]]",
			want: "v1:9c0a72924642edb4",
		},
		{
			// A schema finding feeds the field name and violating value
			// joined by the separator, so the separator byte appears
			// inside a part — schema.jsonl line 1.
			name:   "embedded separator (schema.enum)",
			ruleID: "schema.enum", path: "Concepts/golang/Bad.md", target: "status\x1fbogus",
			want: "v1:c86353e56e0bc094",
		},
		{
			// An empty violating value leaves the separator trailing —
			// schema.jsonl line 2.
			name:   "trailing separator (schema.required)",
			ruleID: "schema.required", path: "Concepts/golang/Bad.md", target: "domain\x1f",
			want: "v1:2856d81293513cdc",
		},
		{
			// A finding with no field feeds an empty field name, so the
			// target begins with the separator — schema.jsonl line 3.
			name:   "leading separator, no field (schema.unknown_key)",
			ruleID: "schema.unknown_key", path: "Concepts/golang/Bad.md", target: "\x1fextra",
			want: "v1:78fe6de614452605",
		},
		{
			// An unlisted lesson feeds the lesson path and the syllabus
			// path — maps.jsonl line 1.
			name:   "two paths (map.disk_unlisted)",
			ruleID: "map.disk_unlisted", path: "Writing/lessons/golang/L1.md", target: "Maps/paths/Go Path.md",
			want: "v1:184c0b31f8c4aaeb",
		},
		{
			// A raw line-separator code point hashes as its UTF-8 bytes —
			// escapes.jsonl line 4.
			name:   "line separator in target (link.broken)",
			ruleID: "link.broken", path: "note.md", target: "L\u2028S",
			want: "v1:e657c404685dae8d",
		},
		{
			// A hash whose top nibble is zero must still render sixteen
			// digits — escapes.jsonl line 6.
			name:   "leading zero is padded (link.broken)",
			ruleID: "link.broken", path: "note.md", target: "Zero 2 Padding",
			want: "v1:0fa417727bc33bfd",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := fingerprint(tt.ruleID, tt.path, tt.target); got != tt.want {
				t.Errorf("fingerprint(%q, %q, %q) = %q, want %q",
					tt.ruleID, tt.path, tt.target, got, tt.want)
			}
		})
	}
}

func TestSeverityOrdering(t *testing.T) {
	t.Parallel()

	if SeverityInfo >= SeverityWarn || SeverityWarn >= SeverityError {
		t.Errorf("severity gating order broken: Info=%d Warn=%d Error=%d",
			SeverityInfo, SeverityWarn, SeverityError)
	}
}

func TestSeverityMarshalText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		sev  Severity
		want string
	}{
		{SeverityInfo, "info"},
		{SeverityWarn, "warn"},
		{SeverityError, "error"},
	}
	for _, tt := range tests {
		got, err := tt.sev.MarshalText()
		if err != nil {
			t.Errorf("MarshalText(%d): %v", int(tt.sev), err)
			continue
		}
		if string(got) != tt.want {
			t.Errorf("MarshalText(%d) = %q, want %q", int(tt.sev), got, tt.want)
		}
	}
	t.Run("out of range panics", func(t *testing.T) {
		t.Parallel()

		defer func() {
			if recover() == nil {
				t.Error("MarshalText on an out-of-range severity must panic")
			}
		}()
		_, _ = Severity(3).MarshalText() //nolint:errcheck // this branch must panic before any return value exists
	})
}
