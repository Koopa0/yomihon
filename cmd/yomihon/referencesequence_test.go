package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestCheckCLIRecognizesReferenceSequenceDeny(t *testing.T) {
	t.Parallel()
	const root = "../../internal/judge/testdata/vault-reference-sequence"
	for _, deny := range []string{"error", "schema.reference_nested_sequence"} {
		t.Run(deny, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			exit := runCommand(t.Context(), "check", []string{"--root", root, "--format=json", "--all", "--deny", deny}, &stdout, &stderr, false)
			t.Log("hit: reference sequence CLI reached")
			if exit != 1 || stderr.Len() != 0 || strings.Count(stdout.String(), `"rule_id":"schema.reference_nested_sequence"`) != 2 {
				t.Errorf("caught: reference CLI deny %q = exit %d, stdout %q, stderr %q; want exit 1, two findings and no stderr", deny, exit, stdout.String(), stderr.String())
			}
		})
	}
}
