package judge

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

// TestAnUnclosedFenceKeepsTheIdentityEveryReasonOfItsRuleShares holds the
// fingerprint of the new reason to the one the rule already gives. The three
// reasons of schema.frontmatter name no field and no value, so they share one
// identity per path, and a baseline that silenced a note while its block was
// unreadable or missing keeps silencing it when the note is changed into any of
// the other two. A reason that hashed on its own would bring that note back as
// new.
func TestAnUnclosedFenceKeepsTheIdentityEveryReasonOfItsRuleShares(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	text := contractFixture(t, nil,
		[2]string{`checked = ["Concepts", "Sources", "Maps", "Writing", "Synthesis", "Inbox"]`, `checked = ["Writing"]`},
	)
	text = strings.Replace(text, "frontmatter_optional = true", "frontmatter_optional = false", 1)
	write(t, root, schema.ContractRelPath, text)
	contract, err := schema.Load(root)
	if err != nil {
		t.Fatalf("schema.Load() error = %v", err)
	}

	const path = "Writing/lessons/golang/L1.md"
	fingerprints := map[string]string{}
	for reason, body := range map[string]string{
		"opens on line 1 and never closes": "---\ntitle: L1\ntype: lesson\nstatus: draft\n--\n\nbody\n",
		"is missing":                       "body\n",
		"is not valid YAML":                "---\ntitle: [\n---\n",
	} {
		findings := LintFrontmatter(path, []byte(body), contract)
		if len(findings) != 1 || !strings.HasSuffix(findings[0].Message, reason) {
			t.Fatalf("LintFrontmatter(%q) = %+v, want exactly one finding ending %q", body, findings, reason)
		}
		fingerprints[reason] = findings[0].Fingerprint
	}
	for reason, got := range fingerprints {
		if want := fingerprints["is missing"]; got != want {
			t.Errorf("the finding %q has fingerprint %s, want the %s every reason of schema.frontmatter shares", reason, got, want)
		}
	}
}
