package main

import (
	"slices"
	"strings"
	"testing"
)

// TestTheRetiredCommandNameStaysAnAliasOfMarks holds the compatibility of the
// command's old name for the scripts and agent instructions that still call it:
// it dispatches to the same command with the same arguments and prints the same
// help, however the help is asked for.
func TestTheRetiredCommandNameStaysAnAliasOfMarks(t *testing.T) {
	t.Parallel()

	command, args := dispatch([]string{"left-open", "--root", "vault"})
	if command != "marks" || !slices.Equal(args, []string{"--root", "vault"}) {
		t.Errorf("dispatch(left-open --root vault) = %q, %q, want marks, [--root vault]", command, args)
	}

	want, handled, err := helpRequest([]string{"marks", "--help"})
	if err != nil || !handled || !strings.Contains(want, "Usage: yomihon marks") {
		t.Fatalf("helpRequest(marks --help) = %q, %v, %v, want the marks usage", want, handled, err)
	}
	for _, asked := range [][]string{{"left-open", "--help"}, {"left-open", "-h"}, {"help", "left-open"}} {
		got, handled, err := helpRequest(asked)
		if err != nil || !handled {
			t.Errorf("helpRequest(%q) = handled %v, error %v, want it answered", asked, handled, err)
			continue
		}
		if got != want {
			t.Errorf("helpRequest(%q) differs from the help of marks:\n%s", asked, got)
		}
	}
}
