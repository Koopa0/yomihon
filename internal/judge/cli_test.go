package judge

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
)

func TestRunCommandWritesExistsResult(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	exit := RunCommand(
		"exists",
		[]string{"--root=testdata/vault-report", "--format=json", "shared"},
		&stdout,
		&stderr,
		false,
	)
	if exit != 0 {
		t.Errorf("RunCommand exit = %d, want 0", exit)
	}
	wantGolden(t, stdout.Bytes(), "testdata/golden/exists-shared.golden")
	if stderr.Len() != 0 {
		t.Errorf("RunCommand stderr = %q, want empty", stderr.String())
	}
}

func TestRunCommandRejectsInvalidInvocationWithoutPayload(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	exit := RunCommand("exists", []string{"--root=testdata/vault-report"}, &stdout, &stderr, false)
	if exit != 2 {
		t.Errorf("RunCommand exit = %d, want 2", exit)
	}
	if stdout.Len() != 0 {
		t.Errorf("RunCommand stdout = %q, want empty", stdout.String())
	}
	want := "yomihon: exists takes exactly one name argument\n"
	if diff := cmp.Diff(want, stderr.String()); diff != "" {
		t.Errorf("RunCommand stderr mismatch (-want +got):\n%s", diff)
	}
}

func TestRunCommandRejectsFlagsAndPositionalsOwnedByAnotherCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		command string
		args    []string
		want    string
	}{
		{
			name:    "coverage positional",
			command: "coverage",
			args:    []string{"unexpected"},
			want:    "yomihon: coverage takes no positional arguments\n",
		},
		{
			name:    "coverage all",
			command: "coverage",
			args:    []string{"--all"},
			want:    "yomihon: coverage does not accept --all\n",
		},
		{
			name:    "coverage deny",
			command: "coverage",
			args:    []string{"--deny", "warn"},
			want:    "yomihon: coverage does not accept --deny\n",
		},
		{
			name:    "coverage baseline",
			command: "coverage",
			args:    []string{"--baseline", "old.jsonl"},
			want:    "yomihon: coverage does not accept --baseline\n",
		},
		{
			name:    "exists all",
			command: "exists",
			args:    []string{"--all", "candidate"},
			want:    "yomihon: exists does not accept --all\n",
		},
		{
			name:    "exists deny",
			command: "exists",
			args:    []string{"--deny", "warn", "candidate"},
			want:    "yomihon: exists does not accept --deny\n",
		},
		{
			name:    "exists baseline",
			command: "exists",
			args:    []string{"--baseline", "old.jsonl", "candidate"},
			want:    "yomihon: exists does not accept --baseline\n",
		},
		{
			name:    "exists multiple names",
			command: "exists",
			args:    []string{"first", "second"},
			want:    "yomihon: exists takes exactly one name argument\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			exit := RunCommand(tt.command, tt.args, &stdout, &stderr, false)
			if exit != 2 {
				t.Errorf("RunCommand(%q) exit = %d, want 2", tt.command, exit)
			}
			if stdout.Len() != 0 {
				t.Errorf("RunCommand(%q) stdout = %q, want empty", tt.command, stdout.String())
			}
			if diff := cmp.Diff(tt.want, stderr.String()); diff != "" {
				t.Errorf("RunCommand(%q) stderr mismatch (-want +got):\n%s", tt.command, diff)
			}
		})
	}
}

func TestRunCommandRejectsUnavailablePrivacyWithoutPayload(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	contract, err := os.ReadFile("testdata/vault-supersession/System/schemas/vault-schema.toml")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	contract = bytes.Replace(contract, []byte("[privacy]\nnever_egress_dirs = []\n\n"), nil, 1)
	write(t, root, schema.ContractRelPath, string(contract))

	tests := []struct {
		name    string
		command string
		args    []string
	}{
		{name: "check", command: "check", args: []string{"--root=" + root, "--format=json"}},
		{name: "coverage", command: "coverage", args: []string{"--root=" + root, "--format=json"}},
		{name: "exists", command: "exists", args: []string{"--root=" + root, "--format=json", "candidate"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			exit := RunCommand(tt.command, tt.args, &stdout, &stderr, false)
			if exit != 2 {
				t.Errorf("RunCommand(%q) exit = %d, want 2", tt.command, exit)
			}
			if stdout.Len() != 0 {
				t.Errorf("RunCommand(%q) stdout = %q, want empty", tt.command, stdout.String())
			}
			want := "yomihon: privacy authority unavailable; agent-facing output disabled\n"
			if diff := cmp.Diff(want, stderr.String()); diff != "" {
				t.Errorf("RunCommand(%q) stderr mismatch (-want +got):\n%s", tt.command, diff)
			}
		})
	}
}

func TestRunCommandFailsClosedWhenNestedVaultEntryCannotBeRead(t *testing.T) {
	root := t.TempDir()
	writeTestContract(t, root, nil)
	blocked := filepath.Join(root, "Concepts", "blocked")
	if err := os.MkdirAll(blocked, 0o750); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	write(t, root, "Concepts/blocked/note.md", "---\ntitle: hidden by scan failure\n---\n")
	if err := os.Chmod(blocked, 0); err != nil {
		t.Fatalf("Chmod(0) error = %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(blocked, 0o700); err != nil && !os.IsNotExist(err) { // #nosec G302 -- a directory needs owner execute permission so TempDir cleanup can traverse it
			t.Errorf("restore blocked directory mode: %v", err)
		}
	})
	if _, err := os.ReadDir(blocked); err == nil {
		t.Skip("filesystem permissions do not make the nested directory unreadable for this process")
	}

	tests := []struct {
		command string
		args    []string
	}{
		{command: "check", args: []string{"--root=" + root, "--format=json"}},
		{command: "coverage", args: []string{"--root=" + root, "--format=json"}},
		{command: "exists", args: []string{"--root=" + root, "--format=json", "candidate"}},
	}
	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			exit := RunCommand(tt.command, tt.args, &stdout, &stderr, false)
			if exit != 2 {
				t.Errorf("RunCommand(%q) exit = %d, want 2", tt.command, exit)
			}
			if stdout.Len() != 0 {
				t.Errorf("RunCommand(%q) stdout = %q, want empty", tt.command, stdout.String())
			}
			want := "yomihon: vault scan failed\n"
			if diff := cmp.Diff(want, stderr.String()); diff != "" {
				t.Errorf("RunCommand(%q) stderr mismatch (-want +got):\n%s", tt.command, diff)
			}
		})
	}
}

func TestRunCommandRedactsInvalidPrivacyAuthority(t *testing.T) {
	t.Parallel()

	base, err := os.ReadFile("testdata/vault-supersession/System/schemas/vault-schema.toml")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	states := []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{
			name: "invalid capability",
			mutate: func(data []byte) []byte {
				return bytes.Replace(data, []byte("never_egress_dirs = []"), []byte(`never_egress_dirs = ["."]`), 1)
			},
		},
		{
			name: "malformed contract",
			mutate: func(data []byte) []byte {
				return append(data, []byte("\n[malformed")...)
			},
		},
	}
	commands := []struct {
		name    string
		command string
		args    func(string) []string
	}{
		{name: "check", command: "check", args: func(root string) []string { return []string{"--root=" + root, "--format=json"} }},
		{name: "coverage", command: "coverage", args: func(root string) []string { return []string{"--root=" + root, "--format=json"} }},
		{name: "exists", command: "exists", args: func(root string) []string { return []string{"--root=" + root, "--format=json", "candidate"} }},
	}
	for _, state := range states {
		t.Run(state.name, func(t *testing.T) {
			t.Parallel()
			for _, command := range commands {
				t.Run(command.name, func(t *testing.T) {
					t.Parallel()

					root := t.TempDir()
					write(t, root, schema.ContractRelPath, string(state.mutate(bytes.Clone(base))))
					var stdout, stderr bytes.Buffer
					exit := RunCommand(command.command, command.args(root), &stdout, &stderr, false)
					if exit != 2 {
						t.Errorf("RunCommand(%q) exit = %d, want 2", command.command, exit)
					}
					if stdout.Len() != 0 {
						t.Errorf("RunCommand(%q) stdout = %q, want empty", command.command, stdout.String())
					}
					want := "yomihon: privacy authority unavailable; agent-facing output disabled\n"
					if diff := cmp.Diff(want, stderr.String()); diff != "" {
						t.Errorf("RunCommand(%q) stderr mismatch (-want +got):\n%s", command.command, diff)
					}
				})
			}
		})
	}
}

func TestRunCommandRechecksPrivacyImmediatelyBeforeEmission(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		command string
		args    func(string) []string
	}{
		{
			name:    "check",
			command: "check",
			args:    func(root string) []string { return []string{"--root=" + root, "--format=json"} },
		},
		{
			name:    "coverage",
			command: "coverage",
			args:    func(root string) []string { return []string{"--root=" + root, "--format=json"} },
		},
		{
			name:    "exists",
			command: "exists",
			args:    func(root string) []string { return []string{"--root=" + root, "--format=json", "candidate"} },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			contract, err := os.ReadFile("testdata/vault-supersession/System/schemas/vault-schema.toml")
			if err != nil {
				t.Fatalf("ReadFile() error = %v", err)
			}
			write(t, root, schema.ContractRelPath, string(contract))
			contractPath := filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))

			var stdout, stderr bytes.Buffer
			exit := runCommand(
				tt.command,
				tt.args(root),
				&stdout,
				&stderr,
				false,
				commandHooks{beforeEmission: func() {
					if writeErr := os.WriteFile(contractPath, append(contract, '\n'), 0o600); writeErr != nil { // #nosec G703 -- path is rooted in t.TempDir
						t.Fatalf("WriteFile(changed contract) error = %v", writeErr)
					}
				}},
			)
			if exit != 2 {
				t.Errorf("runCommand(%q) exit = %d, want 2", tt.command, exit)
			}
			if stdout.Len() != 0 {
				t.Errorf("runCommand(%q) stdout = %q, want empty", tt.command, stdout.String())
			}
			want := "yomihon: privacy authority unavailable; agent-facing output disabled\n"
			if diff := cmp.Diff(want, stderr.String()); diff != "" {
				t.Errorf("runCommand(%q) stderr mismatch (-want +got):\n%s", tt.command, diff)
			}
		})
	}
}

func TestParseCommandArgsOwnsValues(t *testing.T) {
	t.Parallel()

	input := []string{"--root", "vault", "--deny=warn", "note"}
	got, err := parseCommandArgs(input)
	if err != nil {
		t.Fatal(err)
	}
	input[1] = "changed"
	want := commandArgs{root: "vault", deny: []string{"warn"}, positionals: []string{"note"}}
	if diff := cmp.Diff(want, got, cmp.AllowUnexported(commandArgs{})); diff != "" {
		t.Errorf("parseCommandArgs mismatch (-want +got):\n%s", diff)
	}
}
