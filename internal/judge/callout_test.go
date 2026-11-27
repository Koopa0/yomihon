package judge

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestACalloutTitleWithMarkupDrawsAFinding holds callout.title_markup: markup
// on a recognised callout's opening line, which the page shows as escaped
// characters, draws a finding, and a title in code, a comment or an indented
// block does not.
func TestACalloutTitleWithMarkupDrawsAFinding(t *testing.T) {
	t.Parallel()

	root := noteVault(t, map[string]string{
		"Notes/Source.md": "---\ntype: note\n---\nThe source.\n",
		"Notes/Titles.md": "---\ntype: note\n---\n" +
			"> [!quote] <ruby>芭蕉<rt>ばしょう</rt></ruby>\n" +
			"> [!quote] [[Source]]\n" +
			"> [!quote] **bold** and `code`\n" +
			"> [!tip]- **folded**\n" +
			"> [!quote] 松尾芭蕉\n" +
			"> [!nonesuch] **not a callout**\n" +
			"> [!quote] Plain title\n" +
			"> A citation inside a callout: [[Source]].\n" +
			"```\n> [!quote] **fenced**\n```\n" +
			"%%\n> [!quote] **commented**\n%%\n" +
			"    > [!quote] **indented**\n",
	})

	findings, err := runCheckAction(t.Context(), root, nil, false)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	var got []string
	for _, f := range findings {
		if f.RuleID == RuleCalloutTitleMarkup {
			got = append(got, f.Path+" "+*f.Target)
		}
	}
	want := []string{
		"Notes/Titles.md <ruby>芭蕉<rt>ばしょう</rt></ruby>",
		"Notes/Titles.md [[Source]]",
		"Notes/Titles.md **bold** and `code`",
		"Notes/Titles.md **folded**",
		"Notes/Titles.md **not a callout**",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("callout.title_markup findings (-want +got):\n%s", diff)
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

// TestACalloutTitleFindingKeepsItsIdentityWhenALineMovesIt asserts the
// fingerprint names the title, not the line, so a baseline still recognizes the
// finding after a line is written above it.
func TestACalloutTitleFindingKeepsItsIdentityWhenALineMovesIt(t *testing.T) {
	t.Parallel()
	const body = "> [!note] **Bold** title\n> text\n"
	before := calloutTitleFindings([]note{parseNote("Notes/a.md", []byte(body))})
	after := calloutTitleFindings([]note{parseNote("Notes/a.md", []byte("A new first line.\n\n"+body))})
	if len(before) != 1 || len(after) != 1 {
		t.Fatalf("calloutTitleFindings = %d and %d findings, want one each", len(before), len(after))
	}
	if before[0].Fingerprint != after[0].Fingerprint {
		t.Errorf("fingerprint moved with the line: %s before, %s after", before[0].Fingerprint, after[0].Fingerprint)
	}
}
