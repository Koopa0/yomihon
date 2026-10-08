package judge

import (
	"bytes"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestPageGrammarFenceInfo(t *testing.T) {
	pageGrammarControl(t, "fence-info", "link.broken", "ControlMissing", 0)
}

func TestPageGrammarFootnoteParagraph(t *testing.T) {
	pageGrammarControl(t, "footnote-paragraph", "link.broken", "ControlMissing", 13)
}

func TestPageGrammarTaskPath(t *testing.T) {
	pageGrammarControl(t, "task-path", "link.broken.path", "ControlMissing.md", 0)
}

func TestPageGrammarLinkifyCode(t *testing.T) {
	pageGrammarControl(t, "linkify-code", "link.broken.path", "Notes/ControlMissing.md", 0)
}

// Goldens are independently authored wire literals, not captured product output.
// Every selection compares the full finding set and complete serialized bytes.
func pageGrammarControl(t *testing.T, category string, rule RuleID, controlTarget string, subjectLine int) {
	t.Helper()
	root := judgeFixtureRoot(t, "testdata/vault-page-"+category)
	wire, err := os.ReadFile("testdata/golden/page-" + category + ".jsonl")
	if err != nil {
		t.Fatalf("not-applied: page grammar golden: %v", err)
	}
	type occurrence struct {
		Rule     RuleID
		Severity Severity
		Path     string
		Line     int
		Target   string
	}
	wantOccurrences := []occurrence{{Rule: rule, Severity: SeverityWarn, Path: "Notes/Control.md", Line: 9, Target: controlTarget}}
	if subjectLine != 0 {
		wantOccurrences = append(wantOccurrences, occurrence{Rule: "link.broken", Severity: SeverityWarn, Path: "Notes/Subject.md", Line: subjectLine, Target: "Missing"})
	}
	var want []Finding
	for _, expected := range wantOccurrences {
		finding := Finding{RuleID: expected.Rule, Severity: expected.Severity, Path: expected.Path, Line: new(expected.Line), Target: new(expected.Target), SourceRule: "yomihon"}
		if expected.Rule == "link.broken" {
			finding.Message = "[[" + expected.Target + "]] resolves to no note"
			finding.Evidence = "no filename or alias matches the target"
			finding.SuggestedAction = "create the target note, or change the link to an existing filename/alias"
		} else {
			finding.Message = "link to " + expected.Target + " resolves to no file"
			finding.Evidence = "Notes/ControlMissing.md does not exist in the vault"
			finding.SuggestedAction = "fix the path, restore the file, or remove the reference"
		}
		switch expected.Target {
		case "ControlMissing":
			finding.Fingerprint = "v1:9ea92c08613ad294"
		case "Missing":
			finding.Fingerprint = "v1:a1fa3e713737c040"
		case "ControlMissing.md":
			finding.Fingerprint = "v1:56c9668eefd5a0a6"
		case "Notes/ControlMissing.md":
			finding.Fingerprint = "v1:d9037b1917845114"
		default:
			t.Fatalf("not-applied: independent expected target missing: %q", expected.Target)
		}
		want = append(want, finding)
	}
	got, err := Check(t.Context(), root)
	if err != nil {
		t.Fatalf("not-applied: public Check category=%s: %v", category, err)
	}
	var occurrences []occurrence
	for _, finding := range got {
		if finding.Line == nil || finding.Target == nil {
			t.Errorf("caught: S4 %s incomplete finding=%+v", category, finding)
			continue
		}
		occurrences = append(occurrences, occurrence{Rule: finding.RuleID, Severity: finding.Severity, Path: finding.Path, Line: *finding.Line, Target: *finding.Target})
	}
	if diff := cmp.Diff(wantOccurrences, occurrences); diff != "" {
		t.Errorf("caught: S4 %s public Check targets/rules/lines (-want +got):\n%s", category, diff)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("caught: S4 %s complete findings (-want +got):\n%s", category, diff)
	}
	var encoded bytes.Buffer
	if err := WriteJSONL(&encoded, got); err != nil {
		t.Fatalf("not-applied: WriteJSONL category=%s: %v", category, err)
	}
	if !bytes.Equal(wire, encoded.Bytes()) {
		t.Errorf("caught: S4 %s WriteJSONL=%q want=%q", category, encoded.Bytes(), wire)
	}
	controlWire, _, found := bytes.Cut(wire, []byte("\n"))
	if !found || len(controlWire) == 0 {
		t.Fatalf("not-applied: page grammar golden has no complete control line: category=%s", category)
	}
	controlWire = append(bytes.Clone(controlWire), '\n')
	subjectWire := wire[len(controlWire):]
	for _, selection := range []struct {
		name  string
		paths []string
		wire  []byte
		warn  int
	}{
		{name: "subject", paths: []string{"Notes/Subject.md"}, wire: subjectWire, warn: min(subjectLine, 1)},
		{name: "control", paths: []string{"Notes/Control.md"}, wire: controlWire, warn: 1},
		{name: "whole vault", wire: wire, warn: 1},
	} {
		for _, deny := range []struct {
			name   string
			tokens []string
			exit   int
		}{
			{name: "report"},
			{name: "deny warn", tokens: []string{"warn"}, exit: selection.warn},
			{name: "deny error", tokens: []string{"error"}},
		} {
			stdout, exit, err := RunCheck(t.Context(), &CheckOptions{Root: root, Paths: selection.paths, Deny: deny.tokens, Format: FormatJSON})
			if err != nil {
				t.Fatalf("not-applied: RunCheck category=%s selection=%s deny=%s: %v", category, selection.name, deny.name, err)
			}
			if exit != deny.exit || !bytes.Equal(stdout, selection.wire) {
				t.Errorf("caught: S4 %s RunCheck selection=%s deny=%s exit=%d want=%d wire=%q want=%q", category, selection.name, deny.name, exit, deny.exit, stdout, selection.wire)
			}
		}
	}
}
