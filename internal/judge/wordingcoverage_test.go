package judge

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/wording"
)

// TestEverySchemaRuleHasWordsForAReader holds every registered schema rule a
// note page can meet to a sentence of its own in each language, so a rule
// landed later than its words does not reach the page with nothing to say. The
// two rules left out are about the folder and the contract rather than a note,
// so no note page is asked to say them.
func TestEverySchemaRuleHasWordsForAReader(t *testing.T) {
	t.Parallel()

	// The rules no note page is asked to say.
	saidElsewhere := map[string]string{
		// Reached from the scan rather than from any note's frontmatter, so no
		// note page can be asked to say it.
		"schema.unmatched_knowledge_dir": "it is about the folder rather than a note",
		// Reached from the contract's type roles, not from any note.
		"schema.navigation_unavailable": "it is about the contract rather than a note",
	}

	for _, ruleID := range ruleIDs {
		id := string(ruleID)
		if !strings.HasPrefix(id, "schema.") {
			continue
		}
		if _, elsewhere := saidElsewhere[id]; elsewhere {
			continue
		}
		t.Run(id, func(t *testing.T) {
			t.Parallel()
			for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
				parts := wording.SchemaSentence(lang, id, "domain", "golang")
				var said strings.Builder
				for _, part := range parts {
					said.WriteString(part.Text)
				}
				if said.Len() == 0 {
					t.Errorf("rule %q in %s says nothing", id, lang)
				}
				if strings.Contains(said.String(), id) {
					t.Errorf("rule %q in %s fell through to the words for a rule nobody wrote: %q", id, lang, said.String())
				}
			}
		})
	}
}
