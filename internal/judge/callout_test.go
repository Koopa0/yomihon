package judge

import (
	"strings"
	"testing"
)

// TestACalloutTitleWithMarkupDrawsAFinding is the lock for #359: markup on a
// recognised callout's opening line used to reach the reader as escaped
// characters and yomihon check said nothing. The renderer must not rewrite
// the note; the check reports, with the same suggested action the authoring
// table names.
func TestACalloutTitleWithMarkupDrawsAFinding(t *testing.T) {
	t.Parallel()

	root := writeLeftoverLockVault(t, leftoverLockContract, map[string]string{
		"Notes/Source.md": "---\ntitle: Source\ntype: note\nstatus: draft\n---\nThe source.\n",
		"Notes/Titles.md": "---\ntitle: Titles\ntype: note\nstatus: draft\n---\n" +
			"> [!quote] <ruby>芭蕉<rt>ばしょう</rt></ruby>\n" +
			"> [!quote] [[Source]]\n" +
			"> [!quote] **bold** and `code`\n" +
			"> [!tip]- **folded**\n" +
			"> [!quote] 松尾芭蕉\n" +
			"> [!nonesuch] **not a callout**\n" +
			"> [!quote] Plain title\n" +
			"> A citation inside a callout: [[Source]].\n" +
			"```\n> [!quote] **fenced**\n```\n" +
			"%%\n> [!quote] **commented**\n%%\n\n" +
			"    > [!quote] **indented**\n",
	})

	findings, err := Check(t.Context(), root)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	t.Log("producer-hit: actual callout title findings through Check")

	var got []Finding
	for _, f := range findings {
		if f.RuleID == calloutTitleMarkupRule {
			got = append(got, f)
		}
	}
	if len(got) != 5 {
		t.Fatalf("caught: callout.title_markup findings = %d (%v), want 5 (ruby, wikilink, emphasis+code, folded, unknown)", len(got), findingTargets(got))
	}
	for _, f := range got {
		if f.Severity != SeverityInfo {
			t.Errorf("%q Severity = %v, want info", deref(f.Target), f.Severity)
		}
		if f.SourceRule != sourceYomihon {
			t.Errorf("%q SourceRule = %q, want %q", deref(f.Target), f.SourceRule, sourceYomihon)
		}
		if f.Path != "Notes/Titles.md" {
			t.Errorf("%q Path = %q, want Notes/Titles.md", deref(f.Target), f.Path)
		}
		if f.Line == nil {
			t.Errorf("%q Line is nil", deref(f.Target))
		}
		if !strings.Contains(f.SuggestedAction, "move the markup into the callout body") {
			t.Errorf("%q SuggestedAction = %q, want it to name moving the markup", deref(f.Target), f.SuggestedAction)
		}
		if f.Fingerprint == "" || !strings.HasPrefix(f.Fingerprint, fingerprintVersion) {
			t.Errorf("%q Fingerprint = %q, want a versioned value", deref(f.Target), f.Fingerprint)
		}
	}
	wantTargets := []string{
		"<ruby>芭蕉<rt>ばしょう</rt></ruby>",
		"[[Source]]",
		"**bold** and `code`",
		"**folded**",
		"**not a callout**",
	}
	if diff := strings.Join(findingTargets(got), "\n"); diff != strings.Join(wantTargets, "\n") {
		t.Errorf("targets mismatch\ngot:\n%s\nwant:\n%s", diff, strings.Join(wantTargets, "\n"))
	}
}

func TestCalloutTitleUsesEveryValidOpeningType(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name  string
		line  string
		title string
		ok    bool
	}{
		{name: "known", line: "> [!note] **known**", title: "**known**", ok: true},
		{name: "unknown", line: "> [!nonesuch] **unknown** ^address", title: "**unknown** ^address", ok: true},
		{name: "unknown open fold", line: "> [!CUSTOM_TYPE2]+ **open**  ", title: "**open**", ok: true},
		{name: "unknown closed fold", line: "> [!my-callout]- **closed**", title: "**closed**", ok: true},
		{name: "unknown plain", line: "> [!nonesuch] Plain title", title: "Plain title", ok: true},
		{name: "empty type", line: "> [!] **literal**"},
		{name: "dot in type", line: "> [!my.callout] **literal**"},
		{name: "table", line: "| > [!note] **literal** |"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			title, ok := recognisedCalloutTitle(tt.line)
			if title != tt.title || ok != tt.ok {
				t.Errorf("recognisedCalloutTitle(%q) = (%q, %v), want (%q, %v)", tt.line, title, ok, tt.title, tt.ok)
			}
		})
	}
}

func findingTargets(findings []Finding) []string {
	out := make([]string, len(findings))
	for i := range findings {
		out[i] = deref(findings[i].Target)
	}
	return out
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
