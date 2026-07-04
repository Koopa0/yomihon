package judge

import (
	"bytes"
	"encoding/hex"
	"os"
	"testing"
)

// The golden files under testdata/golden/ hold the frozen wire bytes
// for the fixture vaults under testdata/. Every Finding below is a hand
// transcription of one golden line, and every expected value in this
// file is a literal, never computed by the code under test.
func TestWriteJSONLGolden(t *testing.T) {
	tests := []struct {
		name string
		// golden is the file holding the expected bytes for this case.
		golden   string
		findings []Finding
	}{
		{
			// Three notes exercising the link resolver: a duplicated
			// alias, a broken link, a link written against a note's
			// title, and an unresolved provenance reference. Covers CJK
			// as raw UTF-8, "->" left unescaped, escaped quotes, and the
			// presence and absence of line, field, target, and
			// collision_members.
			name:   "conformance",
			golden: "testdata/golden/check.jsonl",
			findings: []Finding{
				{
					RuleID:           "collision.alias",
					Severity:         SeverityWarn,
					Path:             "Concepts/golang/A.md",
					Field:            new("aliases"),
					Message:          `alias "shared" is declared by 2 notes, so [[shared]] cannot resolve deterministically`,
					Evidence:         "shared alias across: Concepts/golang/A.md, Concepts/golang/B.md",
					SuggestedAction:  "give the alias a single owner note, or qualify the duplicates",
					SourceRule:       "vault-schema.toml#rules",
					Target:           new("shared"),
					CollisionMembers: []string{"Concepts/golang/A.md", "Concepts/golang/B.md"},
					Fingerprint:      "c6a289a5d2524c77",
				},
				{
					RuleID:          "link.broken",
					Severity:        SeverityWarn,
					Path:            "Concepts/golang/A.md",
					Line:            new(6),
					Message:         "[[Ghost]] resolves to no note",
					Evidence:        "no filename or alias matches the target",
					SuggestedAction: "create the target note, or change the link to an existing filename/alias",
					SourceRule:      "Note-Schema.md#aliases",
					Target:          new("Ghost"),
					Fingerprint:     "1e6dec2ff85a905f",
				},
				{
					RuleID:          "link.title_not_alias",
					Severity:        SeverityWarn,
					Path:            "Concepts/golang/A.md",
					Line:            new(6),
					Message:         "[[Go Slice 內部結構]] resolves to no filename or alias",
					Evidence:        "the target is the title of Concepts/golang/Go Slice.md but not one of its aliases",
					SuggestedAction: "add the title to Concepts/golang/Go Slice.md's aliases, or link an existing filename/alias",
					SourceRule:      "Note-Schema.md#aliases",
					Target:          new("Go Slice 內部結構"),
					Fingerprint:     "f4980b309f407345",
				},
				{
					RuleID:          "provenance.unresolved",
					Severity:        SeverityWarn,
					Path:            "Concepts/golang/B.md",
					Field:           new("based_on"),
					Message:         "based_on -> [[Missing]] resolves to nothing",
					Evidence:        "no note, alias, or lesson slug matches the reference",
					SuggestedAction: "fix the reference, or create the target note",
					SourceRule:      "vault-schema.toml#provenance",
					Target:          new("[[Missing]]"),
					Fingerprint:     "9c0a72924642edb4",
				},
			},
		},
		{
			// One note violating three schema rules. Pins the schema
			// finding shape: error severity, the fixed evidence, action,
			// and source strings, and target omitted when the violating
			// value is empty.
			name:   "schema",
			golden: "testdata/golden/schema.jsonl",
			findings: []Finding{
				{
					RuleID:          "schema.enum",
					Severity:        SeverityError,
					Path:            "Concepts/golang/Bad.md",
					Field:           new("status"),
					Message:         `status "bogus" is not a valid status`,
					Evidence:        "frontmatter validated against vault-schema.toml",
					SuggestedAction: "fix the frontmatter to match the schema",
					SourceRule:      "vault-schema.toml",
					Target:          new("bogus"),
					Fingerprint:     "c86353e56e0bc094",
				},
				{
					RuleID:          "schema.required",
					Severity:        SeverityError,
					Path:            "Concepts/golang/Bad.md",
					Field:           new("domain"),
					Message:         "domain is required",
					Evidence:        "frontmatter validated against vault-schema.toml",
					SuggestedAction: "fix the frontmatter to match the schema",
					SourceRule:      "vault-schema.toml",
					Fingerprint:     "2856d81293513cdc",
				},
				{
					RuleID:          "schema.unknown_key",
					Severity:        SeverityError,
					Path:            "Concepts/golang/Bad.md",
					Message:         `frontmatter "extra" is not a known field`,
					Evidence:        "frontmatter validated against vault-schema.toml",
					SuggestedAction: "fix the frontmatter to match the schema",
					SourceRule:      "vault-schema.toml",
					Target:          new("extra"),
					Fingerprint:     "78fe6de614452605",
				},
			},
		},
		{
			// One note with six broken links whose targets carry control
			// characters and the two line-separator code points. Pins the
			// escape surface of the wire format: 0x08 and 0x0C as their
			// two-character escapes, other control characters as
			// four-digit lowercase escapes, U+2028 and U+2029 as raw
			// UTF-8 bytes, and a fingerprint whose leading nibble is
			// zero, locking the sixteen-digit zero-padded rendering.
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
					SourceRule:      "Note-Schema.md#aliases",
					Target:          new("Ba\bck"),
					Fingerprint:     "626d4de5ae0f4e0f",
				},
				{
					RuleID:          "link.broken",
					Severity:        SeverityWarn,
					Path:            "note.md",
					Line:            new(1),
					Message:         "[[Gho\fst]] resolves to no note",
					Evidence:        "no filename or alias matches the target",
					SuggestedAction: "create the target note, or change the link to an existing filename/alias",
					SourceRule:      "Note-Schema.md#aliases",
					Target:          new("Gho\fst"),
					Fingerprint:     "7492bcd08f83d54b",
				},
				{
					RuleID:          "link.broken",
					Severity:        SeverityWarn,
					Path:            "note.md",
					Line:            new(1),
					Message:         "[[A\x1bZ]] resolves to no note",
					Evidence:        "no filename or alias matches the target",
					SuggestedAction: "create the target note, or change the link to an existing filename/alias",
					SourceRule:      "Note-Schema.md#aliases",
					Target:          new("A\x1bZ"),
					Fingerprint:     "5f582abb3bae406e",
				},
				{
					RuleID:          "link.broken",
					Severity:        SeverityWarn,
					Path:            "note.md",
					Line:            new(1),
					Message:         "[[L\u2028S]] resolves to no note",
					Evidence:        "no filename or alias matches the target",
					SuggestedAction: "create the target note, or change the link to an existing filename/alias",
					SourceRule:      "Note-Schema.md#aliases",
					Target:          new("L\u2028S"),
					Fingerprint:     "e657c404685dae8d",
				},
				{
					RuleID:          "link.broken",
					Severity:        SeverityWarn,
					Path:            "note.md",
					Line:            new(1),
					Message:         "[[P\u2029S]] resolves to no note",
					Evidence:        "no filename or alias matches the target",
					SuggestedAction: "create the target note, or change the link to an existing filename/alias",
					SourceRule:      "Note-Schema.md#aliases",
					Target:          new("P\u2029S"),
					Fingerprint:     "966f74ea2b04b126",
				},
				{
					RuleID:          "link.broken",
					Severity:        SeverityWarn,
					Path:            "note.md",
					Line:            new(1),
					Message:         "[[Zero 2 Padding]] resolves to no note",
					Evidence:        "no filename or alias matches the target",
					SuggestedAction: "create the target note, or change the link to an existing filename/alias",
					SourceRule:      "Note-Schema.md#aliases",
					Target:          new("Zero 2 Padding"),
					Fingerprint:     "0fa417727bc33bfd",
				},
			},
		},
		{
			// A study path plus a lesson it does not list. The resulting
			// finding is the only shape that sets resolved_to, which no
			// other fixture exercises.
			name:   "maps",
			golden: "testdata/golden/maps.jsonl",
			findings: []Finding{
				{
					RuleID:          "map.disk_unlisted",
					Severity:        SeverityWarn,
					Path:            "Writing/lessons/golang/L1.md",
					Message:         "lesson is on disk but not listed in syllabus Maps/paths/Go Path.md",
					Evidence:        "the lesson exists but the study-path for its domain does not list it",
					SuggestedAction: "add the lesson to the syllabus, or confirm it is intentionally excluded",
					SourceRule:      "vault-schema.toml#rules",
					Target:          new("Writing/lessons/golang/L1.md"),
					ResolvedTo:      new("Maps/paths/Go Path.md"),
					Fingerprint:     "184c0b31f8c4aaeb",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
	tests := []struct {
		name                 string
		ruleID, path, target string
		want                 string
	}{
		{
			// An alias collision feeds an empty path and the normalized
			// (trimmed, composed, lowercased) alias — check.jsonl line 1.
			name:   "empty path (collision.alias)",
			ruleID: "collision.alias", path: "", target: "shared",
			want: "c6a289a5d2524c77",
		},
		{
			// check.jsonl line 2.
			name:   "plain link target (link.broken)",
			ruleID: "link.broken", path: "Concepts/golang/A.md", target: "Ghost",
			want: "1e6dec2ff85a905f",
		},
		{
			// A CJK target hashes as its raw UTF-8 bytes — check.jsonl
			// line 3.
			name:   "CJK target (link.title_not_alias)",
			ruleID: "link.title_not_alias", path: "Concepts/golang/A.md", target: "Go Slice 內部結構",
			want: "f4980b309f407345",
		},
		{
			// A provenance reference keeps its [[...]] wrapper —
			// check.jsonl line 4.
			name:   "raw wikilink target (provenance.unresolved)",
			ruleID: "provenance.unresolved", path: "Concepts/golang/B.md", target: "[[Missing]]",
			want: "9c0a72924642edb4",
		},
		{
			// A schema finding feeds the field name and violating value
			// joined by the separator, so the separator byte appears
			// inside a part — schema.jsonl line 1.
			name:   "embedded separator (schema.enum)",
			ruleID: "schema.enum", path: "Concepts/golang/Bad.md", target: "status\x1fbogus",
			want: "c86353e56e0bc094",
		},
		{
			// An empty violating value leaves the separator trailing —
			// schema.jsonl line 2.
			name:   "trailing separator (schema.required)",
			ruleID: "schema.required", path: "Concepts/golang/Bad.md", target: "domain\x1f",
			want: "2856d81293513cdc",
		},
		{
			// A finding with no field feeds an empty field name, so the
			// target begins with the separator — schema.jsonl line 3.
			name:   "leading separator, no field (schema.unknown_key)",
			ruleID: "schema.unknown_key", path: "Concepts/golang/Bad.md", target: "\x1fextra",
			want: "78fe6de614452605",
		},
		{
			// An unlisted lesson feeds the lesson path and the syllabus
			// path — maps.jsonl line 1.
			name:   "two paths (map.disk_unlisted)",
			ruleID: "map.disk_unlisted", path: "Writing/lessons/golang/L1.md", target: "Maps/paths/Go Path.md",
			want: "184c0b31f8c4aaeb",
		},
		{
			// A raw line-separator code point hashes as its UTF-8 bytes —
			// escapes.jsonl line 4.
			name:   "line separator in target (link.broken)",
			ruleID: "link.broken", path: "note.md", target: "L\u2028S",
			want: "e657c404685dae8d",
		},
		{
			// A hash whose top nibble is zero must still render sixteen
			// digits — escapes.jsonl line 6.
			name:   "leading zero is padded (link.broken)",
			ruleID: "link.broken", path: "note.md", target: "Zero 2 Padding",
			want: "0fa417727bc33bfd",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fingerprint(tt.ruleID, tt.path, tt.target); got != tt.want {
				t.Errorf("fingerprint(%q, %q, %q) = %q, want %q",
					tt.ruleID, tt.path, tt.target, got, tt.want)
			}
		})
	}
}

func TestSeverityOrdering(t *testing.T) {
	if SeverityInfo >= SeverityWarn || SeverityWarn >= SeverityError {
		t.Errorf("severity gating order broken: Info=%d Warn=%d Error=%d",
			SeverityInfo, SeverityWarn, SeverityError)
	}
}

func TestSeverityMarshalText(t *testing.T) {
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
		defer func() {
			if recover() == nil {
				t.Error("MarshalText on an out-of-range severity must panic")
			}
		}()
		_, _ = Severity(3).MarshalText()
	})
}
