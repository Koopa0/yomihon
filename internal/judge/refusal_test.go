package judge

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

// The three adjudicating engines answer a folder they cannot judge with one of
// a small set of refusals, and which one they pick is the whole of what the
// caller can act on: the paragraph a person reads is written from the class,
// not from the words. These tests hold the classification. What each class
// renders as belongs to the binary and is held there.

// everyCommand is the three engines, run over one folder. A refusal is a
// property of the folder rather than of the command, so a test states its case
// once and each engine has to reach the same verdict about it.
var everyCommand = []string{"check", "coverage", "exists"}

// TestOnlyAFolderWithNoRulesFileIsRead is the difference between a folder that
// declared nothing and a folder whose declaration could not be honoured. The
// first is read and withholds nothing; the second is refused, because what it
// withholds is ground this process never learned.
//
// The refusal for a rules file that exists still refuses to say why, which the
// redaction test below pins. This one runs either side of that line: the branch
// that answers for an absent rules file must not turn one yomihon could not
// decode into a claim that the folder carries none.
func TestOnlyAFolderWithNoRulesFileIsRead(t *testing.T) {
	t.Parallel()

	base := []byte(contractFixture(t, nil))
	folders := []struct {
		name  string
		build func(*testing.T) string
		want  error
	}{
		{
			name: "contract without a privacy policy",
			build: func(t *testing.T) string {
				t.Helper()
				root := t.TempDir()
				contract := bytes.Replace(bytes.Clone(base), []byte("[privacy]\nnever_egress_dirs = []\n"), nil, 1)
				if bytes.Equal(contract, base) {
					t.Fatal("the contract fixture carries no privacy table to remove")
				}
				write(t, root, schema.ContractRelPath, string(contract))
				return root
			},
			want: ErrPrivacyAuthorityUnavailable,
		},
		{
			name: "contract that cannot be decoded",
			build: func(t *testing.T) string {
				t.Helper()
				root := t.TempDir()
				write(t, root, schema.ContractRelPath, string(append(bytes.Clone(base), []byte("\n[malformed")...)))
				return root
			},
			want: ErrPrivacyAuthorityUnavailable,
		},
		{
			name: "a folder that is not there",
			build: func(t *testing.T) string {
				t.Helper()
				return filepath.Join(t.TempDir(), "no-folder-of-this-name")
			},
			// Nothing has been read at this point, so there is no policy state
			// to report and nothing about a vault to withhold. Answering with
			// the privacy refusal named a fault in a contract file that is not
			// there to be at fault, and carried a paragraph saying where it
			// lives. The three cases above are contract states, which is why
			// none of them reached this one: an absent folder is not a way for
			// a contract to be wrong.
			want: errVaultScan,
		},
	}
	for _, folder := range folders {
		t.Run(folder.name, func(t *testing.T) {
			t.Parallel()
			for _, command := range everyCommand {
				t.Run(command, func(t *testing.T) {
					t.Parallel()

					err := refuse(t.Context(), t, command, folder.build(t))
					if !errors.Is(err, folder.want) {
						t.Errorf("%s error = %v, want %v", command, err, folder.want)
					}
				})
			}
		})
	}
}

// TestAnUnusableContractIsRedactedWhateverIsWrongWithIt holds the redaction. A
// contract that exists and cannot be honoured is refused without saying why:
// the decoder's account of its keys is vault content, and these commands write
// for a program to read, so explaining the fault would send that content out
// under exactly the policy that is missing.
func TestAnUnusableContractIsRedactedWhateverIsWrongWithIt(t *testing.T) {
	t.Parallel()

	base := []byte(contractFixture(t, nil))
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
	for _, state := range states {
		t.Run(state.name, func(t *testing.T) {
			t.Parallel()
			for _, command := range everyCommand {
				t.Run(command, func(t *testing.T) {
					t.Parallel()

					root := t.TempDir()
					write(t, root, schema.ContractRelPath, string(state.mutate(bytes.Clone(base))))

					err := refuse(t.Context(), t, command, root)
					if !errors.Is(err, ErrPrivacyAuthorityUnavailable) {
						t.Errorf("%s error = %v, want %v", command, err, ErrPrivacyAuthorityUnavailable)
					}
					// The class above is what the binary renders from. This is
					// the property the class exists for, so a later edit cannot
					// satisfy the comparison by carrying contract material
					// alongside it.
					for _, leak := range []string{"never_egress_dirs", "[malformed", "expected", root} {
						if err != nil && strings.Contains(err.Error(), leak) {
							t.Errorf("%s leaked contract material %q: %v", command, leak, err)
						}
					}
				})
			}
		})
	}
}

// TestAScanThatCannotEnterADirectoryFailsClosed holds that a folder the walk
// cannot get into ends the judgement rather than producing a verdict over the
// part it could read. It also names the directory: an operator told only that a
// scan failed, on a vault of any size, has nowhere to start looking.
func TestAScanThatCannotEnterADirectoryFailsClosed(t *testing.T) {
	t.Parallel()

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

	for _, command := range everyCommand {
		t.Run(command, func(t *testing.T) {
			t.Parallel()

			err := refuse(t.Context(), t, command, root)
			if !strings.HasPrefix(err.Error(), "vault scan failed: ") {
				t.Errorf("%s error = %v, want a scan refusal naming what it stopped on", command, err)
			}
			// The operating system's word for the operation varies by platform,
			// so the assertion holds the two parts that carry the meaning: the
			// path to look at, and why it could not be read.
			for _, part := range []string{"Concepts/blocked", "permission denied"} {
				if !strings.Contains(err.Error(), part) {
					t.Errorf("%s error = %v, want it to name %q", command, err, part)
				}
			}
		})
	}
}

// TestTheAuthorityIsRecheckedAfterThePayloadExists holds the publication
// boundary. A filesystem and an arbitrary writer cannot form one transaction,
// so the contract is asked a second time at the last moment a refusal is still
// possible: the payload is rendered and nothing has been written yet.
//
// It drives the moment between those two steps directly, which is the one thing
// the binary's front end cannot do — the moment is inside this package, and no
// caller outside it can reach in. The payload existing and never being returned
// is the whole assertion.
func TestTheAuthorityIsRecheckedAfterThePayloadExists(t *testing.T) {
	t.Parallel()

	for _, command := range everyCommand {
		t.Run(command, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			contract := []byte(contractFixture(t, nil))
			write(t, root, schema.ContractRelPath, string(contract))
			contractPath := filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))

			payload, err := runPrepared(t.Context(), t, command, root, "candidate", actionHooks{}, func() {
				if writeErr := os.WriteFile(contractPath, append(contract, '\n'), 0o600); writeErr != nil { // #nosec G703 -- path is rooted in t.TempDir
					t.Fatalf("WriteFile(changed contract) error = %v", writeErr)
				}
			})
			if err == nil {
				t.Fatalf("%s published %d bytes rendered under an authority that was replaced before it printed", command, len(payload))
			}
			if payload != nil {
				t.Errorf("%s returned a payload alongside its refusal: %q", command, payload)
			}
			if !errors.Is(err, ErrPrivacyAuthorityUnavailable) {
				t.Errorf("%s error = %v, want %v", command, err, ErrPrivacyAuthorityUnavailable)
			}
		})
	}
}

// TestAFolderWithNoRulesFileWithholdsNothing is the other half of the promise:
// what a rules file does not withhold, a folder with no rules file does not
// either, so every command answers for every note, including the ones under a
// directory a rules file would plausibly have named. Each command also says so,
// first, in the form a person reads.
func TestAFolderWithNoRulesFileWithholdsNothing(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	write(t, root, "Private/Secret.md", "---\n---\n# Secret\n\nSee [[Missing]].\n")
	write(t, root, "Notes/Open.md", "---\n---\n# Open\n")

	exists, err := runExistsText(t, root, "Secret")
	if err != nil {
		t.Fatalf("exists over a folder with no rules file: %v", err)
	}
	for _, want := range []string{NoRulesFile, "Private/Secret.md"} {
		if !strings.Contains(exists, want) {
			t.Errorf("exists output is missing %q:\n%s", want, exists)
		}
	}
	if strings.Contains(exists, "withholds") {
		t.Errorf("exists says something is withheld in a folder that declared nothing:\n%s", exists)
	}

	out, exit, err := RunCheck(t.Context(), &CheckOptions{Root: root, Format: FormatHuman})
	if err != nil {
		t.Fatalf("check over a folder with no rules file: %v", err)
	}
	if exit != 1 && exit != 0 {
		t.Errorf("check exit = %d", exit)
	}
	if first, _, _ := strings.Cut(string(out), "\n"); first != noRulesChecks {
		t.Errorf("check opens with %q, want %q", first, noRulesChecks)
	}
	if !strings.Contains(string(out), "Private/Secret.md") {
		t.Errorf("check does not reach a note under Private/ in a folder with no rules file:\n%s", out)
	}

	coverage, _, err := RunCoverage(t.Context(), &CoverageOptions{Root: root, Format: FormatHuman})
	if err != nil {
		t.Fatalf("coverage over a folder with no rules file: %v", err)
	}
	if first, _, _ := strings.Cut(string(coverage), "\n"); !strings.HasPrefix(first, NoRulesFile) {
		t.Errorf("coverage opens with %q, want it to open with %q", first, NoRulesFile)
	}
}

// runExistsText runs exists in the human form over root and returns what it
// printed.
func runExistsText(t *testing.T, root, name string) (string, error) {
	t.Helper()
	out, _, err := RunExists(t.Context(), &ExistsOptions{Root: root, Name: name, Format: FormatHuman})
	return string(out), err
}

// A command started from a folder inside a vault, or above one, is not started
// from a folder with no rules file. Every shape of that is refused before any
// byte is produced, for each command that speaks to a program, because the
// withheld folder the rules name would otherwise be read as an ordinary one.
func TestAFolderInsideOrAboveAVaultIsRefusedByEveryCommand(t *testing.T) {
	t.Parallel()

	rules := contractFixture(t, []string{"Diary"})
	shapes := []struct {
		name  string
		build func(t *testing.T) string
	}{
		{"started inside the folder the rules withhold", func(t *testing.T) string {
			t.Helper()
			wrap := t.TempDir()
			write(t, wrap, "v/"+schema.ContractRelPath, rules)
			write(t, wrap, "v/Diary/Secret Plan.md", "# Secret Plan\n\nthe plan\n")
			return filepath.Join(wrap, "v", "Diary")
		}},
		{"started two folders inside the vault", func(t *testing.T) string {
			t.Helper()
			wrap := t.TempDir()
			write(t, wrap, "v/"+schema.ContractRelPath, rules)
			write(t, wrap, "v/Diary/2026/Secret Plan.md", "# Secret Plan\n\nthe plan\n")
			return filepath.Join(wrap, "v", "Diary", "2026")
		}},
		{"started one folder above a vault", func(t *testing.T) string {
			t.Helper()
			wrap := t.TempDir()
			write(t, wrap, "v/"+schema.ContractRelPath, rules)
			write(t, wrap, "v/Diary/Secret Plan.md", "# Secret Plan\n\nthe plan\n")
			return wrap
		}},
		{"started above a vault nested below the root", func(t *testing.T) string {
			t.Helper()
			wrap := t.TempDir()
			write(t, wrap, "a/b/v/"+schema.ContractRelPath, rules)
			write(t, wrap, "a/b/v/Diary/Secret Plan.md", "# Secret Plan\n\nthe plan\n")
			return wrap
		}},
	}
	commands := []struct {
		name string
		run  func(root string) ([]byte, int, error)
	}{
		{"check", func(root string) ([]byte, int, error) {
			return RunCheck(t.Context(), &CheckOptions{Root: root, Format: FormatJSON})
		}},
		{"coverage", func(root string) ([]byte, int, error) {
			return RunCoverage(t.Context(), &CoverageOptions{Root: root, Format: FormatJSON})
		}},
		{"exists", func(root string) ([]byte, int, error) {
			return RunExists(t.Context(), &ExistsOptions{Root: root, Name: "Secret Plan", Format: FormatJSON})
		}},
	}
	for _, shape := range shapes {
		for _, command := range commands {
			t.Run(shape.name+"/"+command.name, func(t *testing.T) {
				t.Parallel()
				root := shape.build(t)
				stdout, _, err := command.run(root)
				if !errors.Is(err, ErrRulesFileNearby) {
					t.Errorf("%s error = %v, want ErrRulesFileNearby", command.name, err)
				}
				if len(stdout) != 0 {
					t.Errorf("caught: %s wrote %d bytes before refusing a folder inside or above a vault: %q", command.name, len(stdout), stdout)
				}
			})
		}
	}
}
