package judge

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/egress"
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

// TestAFolderWhoseRulesCannotBeHonouredIsRefused holds the refusal for a rules
// file that cannot be honoured, since what it withholds is ground this process
// never learned, and for a folder that is not there, which has no rules to be
// at fault.
func TestAFolderWhoseRulesCannotBeHonouredIsRefused(t *testing.T) {
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
			want: egress.ErrUnavailable,
		},
		{
			name: "a folder that is not there",
			build: func(t *testing.T) string {
				t.Helper()
				return filepath.Join(t.TempDir(), "no-folder-of-this-name")
			},
			// Nothing has been read, so there is no policy state to report and
			// nothing about a vault to withhold: an absent folder is not a way
			// for a contract to be wrong.
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
					if !errors.Is(err, egress.ErrUnavailable) {
						t.Errorf("%s error = %v, want %v", command, err, egress.ErrUnavailable)
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
	write(t, root, "Concepts/blocked/note.md", "---\ntype: note\n---\n")
	unreadable(t, blocked)

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
			if !errors.Is(err, egress.ErrUnavailable) {
				t.Errorf("%s error = %v, want %v", command, err, egress.ErrUnavailable)
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
	for _, want := range []string{egress.NoRulesFile, "Private/Secret.md"} {
		if !strings.Contains(exists, want) {
			t.Errorf("exists output is missing %q:\n%s", want, exists)
		}
	}
	if strings.Contains(exists, "withholds") {
		t.Errorf("exists says something is withheld in a folder that declared nothing:\n%s", exists)
	}

	out, _, err := checkBytes(t.Context(), &CheckOptions{Root: root, Format: FormatHuman})
	if err != nil {
		t.Fatalf("check over a folder with no rules file: %v", err)
	}
	if first, _, _ := strings.Cut(string(out), "\n"); first != noRulesChecks {
		t.Errorf("check opens with %q, want %q", first, noRulesChecks)
	}
	if !strings.Contains(string(out), "Private/Secret.md") {
		t.Errorf("check does not reach a note under Private/ in a folder with no rules file:\n%s", out)
	}

	coverage, _, err := coverageBytes(t.Context(), &CoverageOptions{Root: root, Format: FormatHuman})
	if err != nil {
		t.Fatalf("coverage over a folder with no rules file: %v", err)
	}
	if first, _, _ := strings.Cut(string(coverage), "\n"); !strings.HasPrefix(first, egress.NoRulesFile) {
		t.Errorf("coverage opens with %q, want it to open with %q", first, egress.NoRulesFile)
	}
}

// runExistsText runs exists in the human form over root and returns what it
// printed.
func runExistsText(t *testing.T, root, name string) (string, error) {
	t.Helper()
	out, _, err := existsBytes(t.Context(), &ExistsOptions{Root: root, Name: name, Format: FormatHuman})
	return string(out), err
}

// A command started from a folder inside a vault, or above one, is not started
// from a folder with no rules file. Either is refused before any byte is
// produced, for each command that speaks to a program, because the withheld
// folder the rules name would otherwise be read as an ordinary one. The other
// places a rules file can hide from the scan are vault's to find.
func TestAFolderInsideOrAboveAVaultIsRefusedByEveryCommand(t *testing.T) {
	t.Parallel()

	rules := contractFixture(t, []string{"Diary"})
	shapes := []struct {
		name  string
		build func(t *testing.T) string
	}{
		{name: "started inside the folder the rules withhold", build: func(t *testing.T) string {
			t.Helper()
			wrap := t.TempDir()
			write(t, wrap, "v/"+schema.ContractRelPath, rules)
			write(t, wrap, "v/Diary/Secret Plan.md", "# Secret Plan\n\nthe plan\n")
			return filepath.Join(wrap, "v", "Diary")
		}},
		{name: "started one folder above a vault", build: func(t *testing.T) string {
			t.Helper()
			wrap := t.TempDir()
			write(t, wrap, "v/"+schema.ContractRelPath, rules)
			write(t, wrap, "v/Diary/Secret Plan.md", "# Secret Plan\n\nthe plan\n")
			return wrap
		}},
	}
	for _, shape := range shapes {
		for _, command := range everyCommand {
			t.Run(shape.name+"/"+command, func(t *testing.T) {
				t.Parallel()
				root := shape.build(t)
				stdout, err := runPrepared(t.Context(), t, command, root, "Secret Plan", actionHooks{}, nil)
				if !errors.Is(err, egress.ErrRulesFileNearby) {
					t.Errorf("%s error = %v, want %v", command, err, egress.ErrRulesFileNearby)
				}
				if len(stdout) != 0 {
					t.Errorf("%s wrote %d bytes before refusing a folder inside or above a vault: %q", command, len(stdout), stdout)
				}
				if err != nil && strings.Contains(err.Error(), root) {
					t.Errorf("%s refusal names a path: %q", command, err.Error())
				}
			})
		}
	}
}

// The first look at the folder ends before the notes are read, and a vault moved
// in between (a sync client, a checkout, a mv) would be read as ordinary ground.
// The hook puts one there at that moment, with no wait to hope for: after the
// rules are read and before the walk the output is read from. The refusal has to
// come from what that walk entered.
func TestAVaultMovedInAfterTheFirstLookIsRefusedByEveryCommand(t *testing.T) {
	t.Parallel()
	rules := contractFixture(t, []string{"Diary"})
	for _, command := range everyCommand {
		t.Run(command, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			write(t, root, "Notes/a.md", "# a\n")
			hooks := actionHooks{afterAuthority: func() {
				write(t, root, "Later/"+schema.ContractRelPath, rules)
				write(t, root, "Later/Diary/Secret Plan.md", "# Secret Plan\n\nthe plan\n")
			}}
			stdout, err := runPrepared(t.Context(), t, command, root, "Secret Plan", hooks, nil)
			if !errors.Is(err, egress.ErrRulesFileNearby) {
				t.Errorf("%s error = %v, want egress.ErrRulesFileNearby for a vault that appeared after the first look", command, err)
			}
			if len(stdout) != 0 {
				t.Errorf("%s wrote %d bytes read from a folder that holds a rules file: %q", command, len(stdout), stdout)
			}
		})
	}
}

// A stopped command is the reader's own doing and gets its own words, not the
// refusal about a rules file that never failed.
func TestACancelledCommandIsNotAPrivacyRefusal(t *testing.T) {
	t.Parallel()
	for _, command := range everyCommand {
		t.Run(command, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			write(t, root, "Notes/a.md", "# a\n")
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			_, err := runPrepared(ctx, t, command, root, "a", actionHooks{}, nil)
			if !errors.Is(err, context.Canceled) {
				t.Errorf("%s error = %v, want context.Canceled", command, err)
			}
			if errors.Is(err, egress.ErrUnavailable) {
				t.Errorf("%s reported a stopped command as an unavailable privacy authority", command)
			}
		})
	}
}
