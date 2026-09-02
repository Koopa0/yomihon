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

// TestAnAbsentContractIsNotAnUnusableOne is the difference between a folder
// that declared nothing and a folder whose declaration could not be honoured.
// Both refusals stand — these three commands judge notes against a vocabulary,
// and there is none here — but the first is the ordinary shape of any directory
// yomihon is pointed at, and telling its owner that an authority is
// "unavailable" names a fault that does not exist and a role that is not
// theirs.
//
// The refusal for a contract that exists still refuses to say why, which the
// redaction test below pins. This one runs either side of that line: the branch
// that answers for an absent contract must not turn a contract yomihon could
// not decode into a claim that the folder carries none.
func TestAnAbsentContractIsNotAnUnusableOne(t *testing.T) {
	t.Parallel()

	base, err := os.ReadFile("testdata/vault-supersession/System/schemas/vault-schema.toml")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	folders := []struct {
		name  string
		build func(*testing.T) string
		want  error
	}{
		{
			name:  "no contract at all",
			build: func(t *testing.T) string { t.Helper(); return t.TempDir() },
			want:  ErrNoVaultContract,
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
			contract, err := os.ReadFile("testdata/vault-supersession/System/schemas/vault-schema.toml")
			if err != nil {
				t.Fatalf("ReadFile() error = %v", err)
			}
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
