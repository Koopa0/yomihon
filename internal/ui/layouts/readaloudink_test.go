package layouts

import (
	"strings"
	"testing"
)

// TestTheVoiceIsNeverDrawnInTheReadersVermilion holds the vermilion to the
// reader's own marks: the bookmark where they left off and the diamond on what
// they are not sure of yet. The paragraph the voice is on and the button that
// is speaking are the interface answering a press, so they take a wash of the
// ink like any other state; drawn in the vermilion, a reader would take the
// line being read aloud for a place they had marked.
//
// The set is every rule whose selector names a read-aloud state, so a state
// added later is held too.
func TestTheVoiceIsNeverDrawnInTheReadersVermilion(t *testing.T) {
	t.Parallel()
	read := 0
	for _, rule := range componentRules(t) {
		if !strings.Contains(rule.selector, "[data-reading]") && !strings.Contains(rule.selector, "[data-speaking]") {
			continue
		}
		read++
		if strings.Contains(rule.body, "--accent") {
			t.Errorf("%q draws the voice in the reader's vermilion, which belongs to the bookmark and the diamond alone", rule.selector)
		}
	}
	// The paragraph being read, the sentence button and the table cell button.
	if read < 3 {
		t.Fatalf("only %d rules name a read-aloud state, so the scan above read almost nothing", read)
	}
}
