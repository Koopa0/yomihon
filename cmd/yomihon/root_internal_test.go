package main

import (
	"os"
	"strings"
	"testing"
)

// TestServeRootPrecedence locks how the served folder is chosen. The first
// version of this answered the complaint halfway: it added a flag but kept a
// path compiled into the binary as the default, so a reader who named nothing
// still got one particular person's vault instead of the folder they were
// standing in — while this program's own check and search commands had always
// read the current directory. Naming a folder needs no flag now, and naming
// nothing means here.
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
		{name: "a bare directory is the folder to read", args: []string{"/tmp/flag"}, want: "/tmp/flag"},
		{name: "a bare directory wins over environment", args: []string{"/tmp/flag"}, env: "/tmp/env", want: "/tmp/flag"},
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
	for _, want := range []string{"<dir>", "YOMIHON_ROOT", "current"} {
		if !strings.Contains(text, want) {
			t.Errorf("serve help does not name %q; text = %q", want, text)
		}
	}
}

// Naming no folder means the one the reader is standing in. It used to mean a
// path compiled into the binary, which is the author's own setup wearing the
// costume of a default: anyone else's first run read a directory they had
// never mentioned.
func TestServeRootDefaultsToHere(t *testing.T) {
	t.Setenv("YOMIHON_ROOT", "")
	here, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	got, err := serveRoot(nil)
	if err != nil {
		t.Fatalf("serveRoot(nil) error = %v", err)
	}
	if got != here {
		t.Errorf("serveRoot(nil) = %q, want the current directory %q", got, here)
	}
	if strings.Contains(got, "obsidian") {
		t.Errorf("serveRoot(nil) = %q, which still reaches for one particular vault", got)
	}
}

// Reading a folder is what this program is for, so it is what the bare command
// does — in the folder you are standing in, the way tools that serve a
// directory behave and the way this program's own check and search commands
// already behaved while serve alone insisted on a subcommand and a flag.
func TestDispatch(t *testing.T) {
	t.Parallel()

	here := t.TempDir()
	tests := []struct {
		name     string
		argv     []string
		wantCmd  string
		wantArgs []string
	}{
		{name: "nothing means serve here", wantCmd: "serve"},
		{name: "a directory means serve it", argv: []string{here}, wantCmd: "serve", wantArgs: []string{here}},
		{name: "serve stays explicit", argv: []string{"serve", here}, wantCmd: "serve", wantArgs: []string{here}},
		{name: "a command is a command", argv: []string{"check", "--all"}, wantCmd: "check", wantArgs: []string{"--all"}},
		// A command name wins over a directory of the same name, so a folder
		// called check cannot capture the command; naming it needs the
		// explicit form, which is why that form stays.
		{name: "search keeps its own arguments", argv: []string{"search", "kafka"}, wantCmd: "search", wantArgs: []string{"kafka"}},
		// Neither a command nor a directory: saying so beats serving the
		// current folder under a name the reader plainly did not mean.
		{name: "a typo is neither", argv: []string{"serv"}, wantCmd: "serv"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotCmd, gotArgs := dispatch(tt.argv)
			if gotCmd != tt.wantCmd {
				t.Errorf("dispatch(%q) command = %q, want %q", tt.argv, gotCmd, tt.wantCmd)
			}
			if len(gotArgs) != len(tt.wantArgs) {
				t.Fatalf("dispatch(%q) args = %q, want %q", tt.argv, gotArgs, tt.wantArgs)
			}
			for i := range gotArgs {
				if gotArgs[i] != tt.wantArgs[i] {
					t.Errorf("dispatch(%q) args = %q, want %q", tt.argv, gotArgs, tt.wantArgs)
					break
				}
			}
		})
	}
}
