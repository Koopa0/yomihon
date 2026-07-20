package agent

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseBuildArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want buildArgs
	}{
		{name: "flags", args: []string{"build", "--json", "--renew-attempt-budget", "--root=/vault"}, want: buildArgs{root: "/vault", json: true, renewAttemptBudget: true}},
		{name: "idempotent JSON and empty end marker", args: []string{"build", "--json", "--json", "--"}, want: buildArgs{json: true}},
		{name: "idempotent renewal", args: []string{"build", "--renew-attempt-budget", "--renew-attempt-budget"}, want: buildArgs{renewAttemptBudget: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseBuildArgs(tt.args)
			if err != nil {
				t.Fatalf("parseBuildArgs() error: %v", err)
			}
			if diff := cmp.Diff(tt.want, got, cmp.AllowUnexported(buildArgs{})); diff != "" {
				t.Errorf("parseBuildArgs() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestParseBuildArgsErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing subcommand", args: nil, want: "subcommand is required; use build"},
		{name: "unknown subcommand", args: []string{"YOMIHON_EMBED_KEY"}, want: "unknown subcommand; use build"},
		{name: "positional", args: []string{"build", "extra"}, want: "build takes no arguments"},
		{name: "positional after end marker", args: []string{"build", "--", "--json"}, want: "build takes no arguments"},
		{name: "semantic flag", args: []string{"build", "--semantic"}, want: "unknown flag"},
		{name: "limit flag", args: []string{"build", "--limit=YOMIHON_EMBED_KEY"}, want: "unknown flag"},
		{name: "duplicate root", args: []string{"build", "--root", "/a", "--root=/b"}, want: "flag --root specified more than once"},
		{name: "root too long", args: []string{"build", "--root", strings.Repeat("x", 4097)}, want: "root exceeds 4096 bytes"},
		{name: "json value", args: []string{"build", "--json=true"}, want: "flag --json takes no value"},
		{name: "renewal value", args: []string{"build", "--renew-attempt-budget=true"}, want: "flag --renew-attempt-budget takes no value"},
		{name: "invalid UTF-8", args: []string{"build", string([]byte{0xff})}, want: "arguments must be valid UTF-8"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseBuildArgs(tt.args)
			if err == nil || err.Error() != tt.want {
				t.Errorf("parseBuildArgs() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func FuzzParseBuildArgs(f *testing.F) {
	f.Add("--json", "--root=/vault")
	f.Add("--renew-attempt-budget", "--renew-attempt-budget")
	f.Add("--", "--json")
	f.Add("--root=bad\npath", "--json")
	f.Fuzz(func(t *testing.T, a, b string) {
		args := []string{"build", a, b}
		first, firstErr := parseBuildArgs(args)
		second, secondErr := parseBuildArgs(args)
		if diff := cmp.Diff(first, second, cmp.AllowUnexported(buildArgs{})); diff != "" {
			t.Errorf("parseBuildArgs(%q) is not deterministic (-first +second):\n%s", args, diff)
		}
		if errorText(firstErr) != errorText(secondErr) {
			t.Errorf("parseBuildArgs(%q) errors = %q then %q", args, errorText(firstErr), errorText(secondErr))
		}
		if firstErr != nil || first.root == "" {
			return
		}
		if err := validateCLIRoot(first.root); err != nil {
			t.Errorf("parseBuildArgs(%q) accepted invalid root %q: %v", args, first.root, err)
		}
	})
}
