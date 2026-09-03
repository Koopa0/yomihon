package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func FuzzParseCommandArgs(f *testing.F) {
	for _, seed := range []struct {
		raw    string
		noArgs bool
	}{
		{"", true},
		{"", false},
		{"--root\x00vault\x00--deny=warn\x00note", false},
		{"--all=value", false},
		{"--root", false},
		{"--root=", false},
		{"--format\x00yaml", false},
		{"--mystery", false},
		{"--\x00note", false},
		{"--deny=warn\x00--deny\x00link.broken", false},
		{"--root\x00日本語の保管庫\x00資料", false},
		{"-position", false},
	} {
		f.Add(seed.raw, seed.noArgs)
	}

	f.Fuzz(func(t *testing.T, raw string, noArgs bool) {
		const maxInput = 8 << 10
		if len(raw) > maxInput {
			return
		}

		var args []string
		if !noArgs {
			args = strings.Split(raw, "\x00")
		}
		first, firstErr := parseCommandArgs(slices.Clone(args))
		second, secondErr := parseCommandArgs(slices.Clone(args))
		if diff := cmp.Diff(first, second, cmp.AllowUnexported(commandArgs{})); diff != "" {
			t.Fatalf("parseCommandArgs() is not deterministic (-first +second):\n%s", diff)
		}
		if commandErrorText(firstErr) != commandErrorText(secondErr) {
			t.Fatalf("parseCommandArgs() error is not deterministic: first %q, second %q", firstErr, secondErr)
		}
		if firstErr != nil && commandArgErrorClass(firstErr.Error()) == "" {
			t.Fatalf("parseCommandArgs() returned an unstable error class: %q", firstErr)
		}

		stored := len(first.deny) + len(first.positionals)
		if first.root != "" {
			stored++
		}
		if first.baseline != "" {
			stored++
		}
		if first.format != nil {
			stored++
		}
		if first.all {
			stored++
		}
		if stored > len(args) {
			t.Fatalf("parseCommandArgs() retained %d values from %d arguments", stored, len(args))
		}
	})
}

func commandErrorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func commandArgErrorClass(message string) string {
	switch {
	case strings.HasPrefix(message, "unknown flag "):
		return "unknown-flag"
	case strings.Contains(message, " takes no value"):
		return "unexpected-value"
	case strings.Contains(message, " needs a value"), strings.Contains(message, " needs a non-empty value"):
		return "missing-value"
	case strings.HasPrefix(message, "invalid --format "):
		return "invalid-format"
	default:
		return ""
	}
}
