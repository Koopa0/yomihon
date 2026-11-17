package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestNoContractRefusalNamesStarterAndHealth(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, command := range []string{"check", "coverage", "exists"} {
		for _, format := range []string{"json", "human", "md"} {
			t.Run(command+" "+format, func(t *testing.T) {
				t.Parallel()
				args := []string{"--root", root, "--format", format}
				if command == "exists" {
					args = append(args, "Start")
				}
				var stdout, stderr bytes.Buffer
				exit := runCommand(t.Context(), command, args, &stdout, &stderr, false)
				if exit != 2 || stdout.Len() != 0 {
					t.Fatalf("runCommand(%q, %q) = exit %d, stdout %q; want 2 and empty stdout", command, args, exit, stdout.String())
				}
				line, _, _ := strings.Cut(stderr.String(), "\n")
				if line != "yomihon: this folder has no rules file, so there is nothing to judge notes against" {
					t.Fatalf("refusal first line = %q, want unchanged no-contract refusal", line)
				}
				t.Log("invoked: actual no-contract command refusal")
				for _, phrase := range []string{
					"examples/vault/System/schemas/vault-schema.toml",
					"https://github.com/koopa0/yomihon/releases",
					"source archive",
					"/health",
					"link findings",
				} {
					if !strings.Contains(stderr.String(), phrase) {
						t.Errorf("caught: no-contract refusal omits %q; stderr = %q", phrase, stderr.String())
					}
				}
			})
		}
	}
}
