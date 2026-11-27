package judge

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/koopa0/yomihon/internal/vault"
)

// The check and the page read a note's frontmatter through one decode, so the
// names a note answers to cannot be one thing to `exists` and another to the
// link on the page. This holds the two readings to each other over the
// spellings of the field an author is likely to write.
func TestTheCheckAndThePageReadTheSameAliases(t *testing.T) {
	t.Parallel()
	for name, frontmatter := range map[string]string{
		"one piece of text":     "aliases: Solo Name",
		"quoted text":           `aliases: "Solo Name"`,
		"single quoted":         "aliases: 'Solo Name'",
		"a list of one":         "aliases: [Solo Name]",
		"a block list":          "aliases:\n  - First\n  - Second",
		"a list with a number":  "aliases: [First, 7, Second]",
		"a list with a bool":    "aliases: [First, true]",
		"a list of lists":       "aliases:\n  - [Inner]",
		"an empty list":         "aliases: []",
		"empty":                 "aliases:",
		"null":                  "aliases: null",
		"Null":                  "aliases: Null",
		"NULL":                  "aliases: NULL",
		"a bare date":           "aliases: 2026-11-27",
		"underscored digits":    "aliases: 1_000",
		"a 0b integer":          "aliases: 0b101",
		"a signed hex":          "aliases: 0x-1",
		"a number":              "aliases: 7",
		"a bool":                "aliases: true",
		"yes":                   "aliases: yes",
		"a mapping":             "aliases:\n  key: value",
		"a folded scalar":       "aliases: >\n  Folded name",
		"a literal scalar":      "aliases: |\n  Literal name",
		"a plain scalar of two": "aliases: First, Second",
		"another key only":      "title: Plain",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			data := []byte("---\n" + frontmatter + "\n---\nbody\n")
			page := vault.Parse("Notes/a.md", data).Aliases()
			check := readNote("Notes/a.md", data, nil).aliases
			if diff := cmp.Diff(check, page, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("the check and the page read different aliases (-check +page):\n%s", diff)
			}
		})
	}
}
