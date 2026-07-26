package main

import (
	"strings"
	"testing"
)

// TestServeRootPrecedence locks how the served folder is chosen. Every other
// command already takes --root; serve did not, and on a machine that happens to
// have a ~/obsidian it silently read that instead of the folder the operator
// was standing in, with nothing in the tool naming the environment variable
// that decided it.
func TestServeRootPrecedence(t *testing.T) {
	// Not parallel: the precedence under test is partly environmental, and
	// t.Setenv cannot run inside a parallel test.
	tests := []struct {
		name    string
		args    []string
		env     string
		want    string
		wantErr bool
	}{
		{name: "flag wins over environment", args: []string{"--root", "/tmp/flag"}, env: "/tmp/env", want: "/tmp/flag"},
		{name: "environment when no flag", args: nil, env: "/tmp/env", want: "/tmp/env"},
		{name: "flag alone", args: []string{"--root", "/tmp/flag"}, want: "/tmp/flag"},
		{name: "empty flag value is refused", args: []string{"--root", ""}, wantErr: true},
		{name: "bare directory is not a root", args: []string{"/tmp/flag"}, wantErr: true},
		{name: "unknown flag is refused", args: []string{"--vault", "/tmp/flag"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("YOMIHON_ROOT", tt.env)
			got, err := serveRoot(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("serveRoot(%q) = %q, want an error", tt.args, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("serveRoot(%q) error = %v", tt.args, err)
			}
			if got != tt.want {
				t.Errorf("serveRoot(%q) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}

// TestServeHelpNamesEveryWayToChooseTheFolder locks the help text against the
// thing it exists to prevent: a first run where the reader cannot discover how
// to point the tool at their own folder.
func TestServeHelpNamesEveryWayToChooseTheFolder(t *testing.T) {
	t.Parallel()

	text, handled, err := helpRequest([]string{"serve", "--help"})
	if err != nil || !handled {
		t.Fatalf("helpRequest(serve --help) = handled %t, err %v", handled, err)
	}
	for _, want := range []string{"--root", "YOMIHON_ROOT", "~/obsidian"} {
		if !strings.Contains(text, want) {
			t.Errorf("serve help does not name %q; text = %q", want, text)
		}
	}
}
