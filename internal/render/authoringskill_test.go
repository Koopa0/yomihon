package render

import (
	"maps"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// authoringSkillPath is the callout vocabulary's other reader: a person
// writing a note reaches for this guide instead of calloutVocabulary itself,
// so nothing but this test forces the two lists to move together. The path is
// relative to this package's own directory, the way go test always runs
// regardless of where the test command itself was invoked from.
const authoringSkillPath = "../../skills/yomihon-authoring/SKILL.md"

// calloutToken matches one inline-code span: the unit both the guide's list
// and this test operate on.
var calloutToken = regexp.MustCompile("`[^`\n]+`")

// TestAuthoringSkillListsExactlyTheKnownCalloutTypes pins the callout type
// list a person reads in the authoring guide to the vocabulary the renderer
// actually answers to. A type the guide names but calloutVocabulary drops
// sends an author toward a `[!that-type]` block expecting a tinted box and
// gets a plain blockquote with a diagnostic instead; a type calloutVocabulary
// carries but the guide never mentions is invisible to every author who only
// reads the guide. Reading the guide's own words here, rather than trusting a
// second copy of the count kept in this test, means an edit to either side is
// checked against the other automatically.
func TestAuthoringSkillListsExactlyTheKnownCalloutTypes(t *testing.T) {
	t.Parallel()

	documented := calloutTypesInAuthoringSkill(t)
	coded := map[string]bool{}
	for _, group := range calloutVocabulary {
		for _, typ := range group.types {
			coded[typ] = true
		}
	}

	want := slices.Sorted(maps.Keys(coded))
	got := slices.Sorted(maps.Keys(documented))
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("authoring skill's callout list disagrees with calloutVocabulary (-code +guide):\n%s", diff)
	}
}

// calloutTypesInAuthoringSkill extracts the closed set of callout types from
// the guide's own list, rather than trusting a hand-copied count kept in this
// test to track it.
//
// The list is a run of consecutive lines holding nothing but inline-code
// spans and the "·" that groups them — every other line in the guide mixes in
// prose, a table pipe, or a heading mark. That shape is not unique to this one
// list: a line of filter keys elsewhere in the guide has it too. So the
// extractor finds every such run and keeps the one whose lead-in paragraph
// names callouts, the way a reader would locate the right list — by what
// introduces it, not by a line number that moves every time the guide is
// edited somewhere else. Finding more or fewer than one such run fails the
// test outright instead of guessing which one is meant.
func calloutTypesInAuthoringSkill(t *testing.T) map[string]bool {
	t.Helper()

	data, err := os.ReadFile(authoringSkillPath) // #nosec G304 -- a fixed path to this repository's own guide, not untrusted input
	if err != nil {
		t.Fatalf("read %s: %v", authoringSkillPath, err)
	}
	lines := strings.Split(string(data), "\n")

	var run []string
	found := 0
	for i := 0; i < len(lines); {
		if !isTokenListLine(lines[i]) {
			i++
			continue
		}
		start := i
		for i < len(lines) && isTokenListLine(lines[i]) {
			i++
		}
		if !leadInNamesCallouts(lines[:start]) {
			continue
		}
		found++
		run = lines[start:i]
	}
	if found != 1 {
		t.Fatalf("found %d token-list blocks in the authoring skill introduced by a paragraph naming callouts, want exactly 1", found)
	}

	types := map[string]bool{}
	for _, line := range run {
		for _, tok := range calloutToken.FindAllString(line, -1) {
			types[strings.Trim(tok, "`")] = true
		}
	}
	return types
}

// isTokenListLine reports whether line holds nothing but inline-code spans,
// the "·" grouping mark, and whitespace: the shape of a values list rather
// than of prose.
func isTokenListLine(line string) bool {
	if strings.TrimSpace(line) == "" {
		return false
	}
	if calloutToken.FindAllString(line, -1) == nil {
		return false
	}
	rest := calloutToken.ReplaceAllString(line, "")
	rest = strings.ReplaceAll(rest, "·", "")
	return strings.TrimSpace(rest) == ""
}

// leadInNamesCallouts reports whether the non-blank paragraph immediately
// above a token-list run mentions callouts, skipping the single blank line
// the guide always leaves between a list and its lead-in.
func leadInNamesCallouts(before []string) bool {
	i := len(before) - 1
	for i >= 0 && strings.TrimSpace(before[i]) == "" {
		i--
	}
	end := i + 1
	for i >= 0 && strings.TrimSpace(before[i]) != "" {
		i--
	}
	paragraph := strings.Join(before[i+1:end], " ")
	return strings.Contains(strings.ToLower(paragraph), "callout")
}
