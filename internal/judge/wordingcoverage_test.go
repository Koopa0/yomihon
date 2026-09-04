package judge

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/wording"
)

// TestEverySchemaRuleHasWordsForAReader closes the gap this whole surface was
// built to close, in the one direction that reopens by itself: a rule landed
// later than the sentence that explains it. The page would then meet a finding
// it has nothing to say about, and a page with nothing to say is where this
// started.
//
// The enumeration is the registry a --deny value is validated against, which a
// sibling test holds to what the rules actually emit in both directions, so
// neither a new rule nor a dead entry can hide here. A list copied into this
// file would only pin what its author believed on the day.
//
// The one registered rule left out is about the folder rather than about any
// note: it is reached from the scan, never from a note's frontmatter, so no
// note page can ever be asked to say it.
func TestEverySchemaRuleHasWordsForAReader(t *testing.T) {
	t.Parallel()

	// Two rules are answered somewhere other than a note page's own words, so
	// a sentence here for either would be one nothing ever renders.
	saidElsewhere := map[string]string{
		// Reached from the scan rather than from any note's frontmatter, so no
		// note page can be asked to say it.
		"schema.unmatched_knowledge_dir": "it is about the folder rather than a note",
		// The panel that would carry it is not rendered at all when the
		// frontmatter cannot be read, and the page says so through the surface
		// that also carries the parser's own account of what failed.
		"schema.frontmatter": "the note's conditions face already says it, with more detail",
	}

	checked := 0
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
				parts := wording.SchemaSentence(lang, id, "domain", "golang", "japanese")
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
		checked++
	}
	if checked < 8 {
		t.Errorf("only %d schema rules were checked; the registry scan found too few to prove anything", checked)
	}
}

// TestAnUnknownSchemaRuleStillSaysSomething is the fallback's own lock: the
// test above proves no rule falls through today, and this proves that falling
// through is still answered rather than silent.
func TestAnUnknownSchemaRuleStillSaysSomething(t *testing.T) {
	t.Parallel()

	const invented = "schema.not_a_rule_anyone_wrote"
	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		var said strings.Builder
		for _, part := range wording.SchemaSentence(lang, invented, "", "", "") {
			said.WriteString(part.Text)
		}
		if !strings.Contains(said.String(), invented) {
			t.Errorf("an unknown rule in %s said %q, which does not name the rule a reader would have to ask about", lang, said.String())
		}
	}
}
