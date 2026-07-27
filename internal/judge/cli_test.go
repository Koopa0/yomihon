package judge

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
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

// unresolvedContractGuidance and absentContractGuidance are the paragraphs a
// person gets after a refusal a program only needs the first line of. They are
// written out here rather than imported from the command so an edit to either
// side has to be made on both, which is the only way this assertion can fail.
const unresolvedContractGuidance = "  The contract is at System/schemas/vault-schema.toml and yomihon could not use it.\n" +
	"  The reason is not printed here: this command's output is written for a program to\n" +
	"  read, and stating the reason would quote the contract back out under exactly the\n" +
	"  policy that is missing. Read it where reading is the point: yomihon serve\n" +
	"  --root <dir> states the cause on the page, and the server logs it at startup.\n"

const absentContractGuidance = "  yomihon reads System/schemas/vault-schema.toml for the note types, fields and\n" +
	"  lifecycle that check, coverage and exists judge against, and for the directories\n" +
	"  whose contents must never leave this machine. A folder carrying no such file has\n" +
	"  declared nothing, and these three commands have no vocabulary to answer in.\n" +
	"  Reading and search need none of it: yomihon serve --root <dir>\n"

// TestRunCommandSeparatesAnAbsentContractFromAnUnusableOne is the difference
// between a folder that declared nothing and a folder whose declaration could
// not be honoured. Both refusals stand — these three commands judge notes
// against a vocabulary, and there is none here — but the first is the ordinary
// shape of any directory yomihon is pointed at, and telling its owner that an
// authority is "unavailable" names a fault that does not exist and a role that
// is not theirs.
//
// The refusal for a contract that exists still refuses to say why. That is the
// redaction the sibling test pins, and this test's fixtures run either side of
// it: the branch added here must not turn a contract yomihon could not decode
// into a claim that the folder carries none.
func TestRunCommandSeparatesAnAbsentContractFromAnUnusableOne(t *testing.T) {
	t.Parallel()

	base, err := os.ReadFile("testdata/vault-supersession/System/schemas/vault-schema.toml")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	folders := []struct {
		name  string
		build func(*testing.T) string
		want  string
	}{
		{
			name:  "no contract at all",
			build: func(t *testing.T) string { t.Helper(); return t.TempDir() },
			want: "yomihon: this folder has no vault contract, so there is nothing to judge notes against\n" +
				absentContractGuidance,
		},
		{
			name: "contract without a privacy policy",
			build: func(t *testing.T) string {
				t.Helper()
				root := t.TempDir()
				contract := bytes.Replace(bytes.Clone(base), []byte("[privacy]\nnever_egress_dirs = []\n\n"), nil, 1)
				write(t, root, schema.ContractRelPath, string(contract))
				return root
			},
			want: "yomihon: privacy authority unavailable; agent-facing output disabled\n" +
				unresolvedContractGuidance,
		},
		{
			name: "contract that cannot be decoded",
			build: func(t *testing.T) string {
				t.Helper()
				root := t.TempDir()
				write(t, root, schema.ContractRelPath, string(append(bytes.Clone(base), []byte("\n[malformed")...)))
				return root
			},
			want: "yomihon: privacy authority unavailable; agent-facing output disabled\n" +
				unresolvedContractGuidance,
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
	for _, folder := range folders {
		t.Run(folder.name, func(t *testing.T) {
			t.Parallel()
			for _, command := range commands {
				t.Run(command.name, func(t *testing.T) {
					t.Parallel()

					root := folder.build(t)
					var stdout, stderr bytes.Buffer
					exit := RunCommand(command.command, command.args(root), &stdout, &stderr, false)
					if exit != 2 {
						t.Errorf("RunCommand(%q) exit = %d, want 2", command.command, exit)
					}
					if stdout.Len() != 0 {
						t.Errorf("RunCommand(%q) stdout = %q, want empty", command.command, stdout.String())
					}
					if diff := cmp.Diff(folder.want, stderr.String()); diff != "" {
						t.Errorf("RunCommand(%q) stderr mismatch (-want +got):\n%s", command.command, diff)
					}
				})
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
			want := "yomihon: privacy authority unavailable; agent-facing output disabled\n" + unresolvedContractGuidance
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
					want := "yomihon: privacy authority unavailable; agent-facing output disabled\n" + unresolvedContractGuidance
					if diff := cmp.Diff(want, stderr.String()); diff != "" {
						t.Errorf("RunCommand(%q) stderr mismatch (-want +got):\n%s", command.command, diff)
					}
					// The literal above pins today's words. This pins the property
					// those words exist for, so a later edit cannot satisfy the
					// comparison by moving contract material into both sides of it.
					for _, leak := range []string{"never_egress_dirs", "[malformed", "expected", root} {
						if strings.Contains(stderr.String(), leak) {
							t.Errorf("RunCommand(%q) stderr leaked contract material %q: %q",
								command.command, leak, stderr.String())
						}
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
			want := "yomihon: privacy authority unavailable; agent-facing output disabled\n" + unresolvedContractGuidance
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
