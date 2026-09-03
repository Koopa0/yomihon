package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
)

// TestTheParserOwnsTheValuesItReturns holds that a parsed invocation keeps no
// hold on the slice it was read from, so a caller reusing that memory cannot
// change what a command was asked to do after it was asked.
func TestTheParserOwnsTheValuesItReturns(t *testing.T) {
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

// TestACommandRefusesWhatItDoesNotOwn holds each command to the flags and
// positionals its usage string beside this file offers. The refusal happens
// before any folder is opened, so none of these cases needs a vault: an
// invocation that cannot be honoured is answered without reading anything.
func TestACommandRefusesWhatItDoesNotOwn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		command string
		args    []string
		want    string
	}{
		{
			name:    "exists with no name",
			command: "exists",
			args:    []string{"--root=somewhere"},
			want:    "yomihon: exists takes exactly one name argument\n",
		},
		{
			name:    "exists with two names",
			command: "exists",
			args:    []string{"first", "second"},
			want:    "yomihon: exists takes exactly one name argument\n",
		},
		{name: "coverage positional", command: "coverage", args: []string{"unexpected"}, want: "yomihon: coverage takes no positional arguments\n"},
		{name: "coverage all", command: "coverage", args: []string{"--all"}, want: "yomihon: coverage does not accept --all\n"},
		{name: "coverage deny", command: "coverage", args: []string{"--deny", "warn"}, want: "yomihon: coverage does not accept --deny\n"},
		{name: "coverage baseline", command: "coverage", args: []string{"--baseline", "old.jsonl"}, want: "yomihon: coverage does not accept --baseline\n"},
		{name: "exists all", command: "exists", args: []string{"--all", "candidate"}, want: "yomihon: exists does not accept --all\n"},
		{name: "exists deny", command: "exists", args: []string{"--deny", "warn", "candidate"}, want: "yomihon: exists does not accept --deny\n"},
		{name: "exists baseline", command: "exists", args: []string{"--baseline", "old.jsonl", "candidate"}, want: "yomihon: exists does not accept --baseline\n"},
		{name: "unknown flag", command: "check", args: []string{"--mystery"}, want: "yomihon: unknown flag \"--mystery\"\n"},
		{name: "flag with no value", command: "check", args: []string{"--root"}, want: "yomihon: flag --root needs a value\n"},
		{name: "flag with an empty value", command: "check", args: []string{"--root="}, want: "yomihon: flag --root needs a non-empty value\n"},
		{name: "valueless flag given a value", command: "check", args: []string{"--all=yes"}, want: "yomihon: flag --all takes no value\n"},
		{name: "unknown format", command: "check", args: []string{"--format=yaml"}, want: "yomihon: invalid --format \"yaml\"; use json, human, or md\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			exit := runCommand(t.Context(), tt.command, tt.args, &stdout, &stderr, false)
			if exit != 2 {
				t.Errorf("exit = %d, want 2", exit)
			}
			if stdout.Len() != 0 {
				t.Errorf("stdout = %q, want empty", stdout.String())
			}
			if diff := cmp.Diff(tt.want, stderr.String()); diff != "" {
				t.Errorf("stderr mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestTheVaultTypedAsAScopeNamesTheWordToMove holds the refusal a reader gets
// for the mistake this command invites. The vault and a scope filter are both
// directories typed after the command, and putting the vault in the scope
// position produced a message about vault-relative paths that never named the
// word to move.
//
// The engine refuses the same shape in its own words, but it cannot say this
// sentence: the word to move is a flag, and the engine takes no flags. Saying
// it only there would also mean opening a folder first, so a reader standing
// outside their vault would be told their folder carries no contract rather
// than which word they misplaced.
func TestTheVaultTypedAsAScopeNamesTheWordToMove(t *testing.T) {
	t.Parallel()

	vault := t.TempDir()
	var stdout, stderr bytes.Buffer
	exit := runCommand(t.Context(), "check", []string{vault}, &stdout, &stderr, false)

	if exit != 2 {
		t.Errorf("exit = %d, want 2", exit)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--root") {
		t.Errorf("the refusal never names the flag the vault belongs to:\n%s", stderr.String())
	}
	if !strings.Contains(stderr.String(), vault) {
		t.Errorf("the refusal never names the directory it is about:\n%s", stderr.String())
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

// TestARefusalCarriesTheParagraphItsClassEarns is where the engine's classes
// become sentences a person can act on. The engine answers a folder it cannot
// judge with one of a small set of refusals and says nothing else; deciding
// which paragraph each earns is this side's job, and getting it wrong is
// invisible from the other side.
//
// The third case is the reason the classes have to be told apart at all. A
// folder that is not there has no contract to be missing and none to be broken,
// so it earns neither paragraph — and telling its owner where a contract file
// belongs, in a directory that does not exist, withholds the one thing they
// need.
func TestARefusalCarriesTheParagraphItsClassEarns(t *testing.T) {
	t.Parallel()

	folders := []struct {
		name  string
		build func(*testing.T) string
		// want takes the folder because one of the three answers names it. A
		// fixed string could not: the folder is made by the test run.
		want func(root string) string
	}{
		{
			name:  "a folder carrying no contract",
			build: func(t *testing.T) string { t.Helper(); return t.TempDir() },
			want: func(string) string {
				return "yomihon: this folder has no vault contract, so there is nothing to judge notes against\n" +
					absentContractGuidance
			},
		},
		{
			name: "a contract that cannot be read",
			build: func(t *testing.T) string {
				t.Helper()
				root := t.TempDir()
				writeFile(t, root, schema.ContractRelPath, "[malformed\n")
				return root
			},
			want: func(string) string {
				return "yomihon: privacy authority unavailable; agent-facing output disabled\n" +
					unresolvedContractGuidance
			},
		},
		{
			name: "a folder that is not there",
			build: func(t *testing.T) string {
				t.Helper()
				return filepath.Join(t.TempDir(), "no-folder-of-this-name")
			},
			// The machine's own words for what it could not open, asked for
			// the same way the reader asks, so this pins the folder and the
			// reason rather than a guess at how they are spelled.
			want: func(root string) string {
				_, err := filepath.EvalSymlinks(root)
				if err == nil {
					t.Fatalf("EvalSymlinks(%q) succeeded; the folder this row needs absent is there", root)
				}
				return "yomihon: vault scan failed: open vault root: " + err.Error() + "\n"
			},
		},
	}
	commands := []struct {
		name string
		args func(string) []string
	}{
		{name: "check", args: func(root string) []string { return []string{"--root=" + root, "--format=json"} }},
		{name: "coverage", args: func(root string) []string { return []string{"--root=" + root, "--format=json"} }},
		{name: "exists", args: func(root string) []string {
			return []string{"--root=" + root, "--format=json", "candidate"}
		}},
	}
	for _, folder := range folders {
		t.Run(folder.name, func(t *testing.T) {
			t.Parallel()
			for _, command := range commands {
				t.Run(command.name, func(t *testing.T) {
					t.Parallel()

					var stdout, stderr bytes.Buffer
					root := folder.build(t)
					exit := runCommand(t.Context(), command.name, command.args(root), &stdout, &stderr, false)
					if exit != 2 {
						t.Errorf("exit = %d, want 2", exit)
					}
					if stdout.Len() != 0 {
						t.Errorf("stdout = %q, want empty", stdout.String())
					}
					if diff := cmp.Diff(folder.want(root), stderr.String()); diff != "" {
						t.Errorf("stderr mismatch (-want +got):\n%s", diff)
					}
				})
			}
		})
	}
}

// TestTheTypedWordsReachTheEngine holds the wiring between them: which folder,
// which output shape, which name, which gate. Each of these is a word the
// reader typed that has to arrive somewhere specific, and the engine's own
// tests cannot see any of them — they are handed already-resolved options.
func TestTheTypedWordsReachTheEngine(t *testing.T) {
	t.Parallel()

	root := judgeableVault(t)

	t.Run("the named folder and the machine format reach exists", func(t *testing.T) {
		t.Parallel()

		var stdout, stderr bytes.Buffer
		exit := runCommand(t.Context(), "exists",
			[]string{"--root=" + root, "--format=json", "Present"}, &stdout, &stderr, false)
		if exit != 0 {
			t.Fatalf("exit = %d, want 0; stderr:\n%s", exit, stderr.String())
		}
		var report struct {
			Query   string `json:"query"`
			Matches []struct {
				Path string `json:"path"`
			} `json:"matches"`
		}
		if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
			t.Fatalf("the payload is not the machine format: %v; stdout=%q", err, stdout.String())
		}
		if report.Query != "Present" {
			t.Errorf("query = %q, want the name typed after the command", report.Query)
		}
		if len(report.Matches) == 0 || report.Matches[0].Path != "Writing/Present.md" {
			t.Errorf("matches = %+v, want the note in the folder --root named", report.Matches)
		}
	})

	t.Run("a name nothing answers to exits one", func(t *testing.T) {
		t.Parallel()

		var stdout, stderr bytes.Buffer
		exit := runCommand(t.Context(), "exists",
			[]string{"--root=" + root, "--format=json", "Nothing Carries This Name"}, &stdout, &stderr, false)
		if exit != 1 {
			t.Errorf("exit = %d, want 1; stderr:\n%s", exit, stderr.String())
		}
		if stdout.Len() == 0 {
			t.Error("stdout is empty; a miss is an answer and is still printed")
		}
	})

	t.Run("the deny gate reaches check", func(t *testing.T) {
		t.Parallel()

		var ungated, stderr bytes.Buffer
		exit := runCommand(t.Context(), "check", []string{"--root=" + root, "--format=json"}, &ungated, &stderr, false)
		if exit != 0 {
			t.Fatalf("ungated exit = %d, want 0; stderr:\n%s", exit, stderr.String())
		}
		if !bytes.Contains(ungated.Bytes(), []byte(`"severity":"error"`)) {
			t.Fatalf("the fixture holds no error to gate on, so denying one would prove nothing:\n%s", ungated.String())
		}

		var gated bytes.Buffer
		stderr.Reset()
		if exit := runCommand(t.Context(), "check",
			[]string{"--root=" + root, "--format=json", "--deny=error"}, &gated, &stderr, false); exit != 1 {
			t.Errorf("exit = %d, want 1; --deny never reached the gate\nstderr:\n%s", exit, stderr.String())
		}
		if diff := cmp.Diff(ungated.String(), gated.String()); diff != "" {
			t.Errorf("the gate changed the payload (-ungated +gated):\n%s", diff)
		}
	})

	t.Run("a scope naming nothing is refused rather than answered", func(t *testing.T) {
		t.Parallel()

		var stdout, stderr bytes.Buffer
		exit := runCommand(t.Context(), "check",
			[]string{"--root=" + root, "--format=json", "Nowhere"}, &stdout, &stderr, false)
		if exit != 2 {
			t.Errorf("exit = %d, want 2; stdout:\n%s", exit, stdout.String())
		}
		if !strings.Contains(stderr.String(), "names nothing in this vault") {
			t.Errorf("the scope refusal did not reach stderr:\n%s", stderr.String())
		}
	})
}

// judgeableVault writes a folder these commands can actually judge: the
// loader's own contract fixture, which is the module's one valid contract, and
// one note that gives check something to say and exists something to find.
func judgeableVault(t *testing.T) string {
	t.Helper()
	contract, err := os.ReadFile(filepath.Join("..", "..", "internal", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("read the loader's contract fixture: %v", err)
	}
	root := t.TempDir()
	writeFile(t, root, schema.ContractRelPath, string(contract)+"\n[privacy]\nnever_egress_dirs = []\n")
	writeFile(t, root, "Writing/Present.md",
		"---\ntitle: Present\ntype: lesson\ndomain: meta\nstatus: draft\n"+
			"created: 2026-01-01\nupdated: 2026-01-01\nslug: present\n---\n[[Nobody Wrote This]]\n")
	return root
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil { // #nosec G703 -- test fixture paths are owned by each caller's temporary root
		t.Fatalf("WriteFile(%q) error = %v", full, err)
	}
}
