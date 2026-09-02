package main_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/schema"
)

// TestTheBinaryPrintsTheEnginesPayloadUnaltered holds the last stretch of the
// frozen adjudication wire: the engine's bytes are compared to golden files
// inside its own package, but between that comparison and the pipe a consumer
// reads there is a parser, a format decision and a write, and none of them was
// asserted. A newline appended on the way out satisfies every other test in the
// repository and breaks every consumer that reads a fixed number of bytes.
//
// The oracle is the engine itself rather than a second copy of the golden
// files. A copy of frozen bytes drifts from the original the first time one of
// them is regenerated, and then two answers exist for a question that has one.
//
// Each command is run twice: once naming the machine format outright, and once
// naming no format at all. The second is the shape an agent actually invokes —
// stdout is a pipe, and the resolution that turns a pipe into machine output is
// made by the binary, so nothing inside the engine can assert it.
func TestTheBinaryPrintsTheEnginesPayloadUnaltered(t *testing.T) {
	t.Parallel()

	binary := buildYomihonBinary(t)
	root := adjudicableVault(t)
	env := isolatedUserEnv(t.TempDir())

	for _, tt := range []struct {
		command  string
		trailing []string
		payload  func(t *testing.T) []byte
	}{
		{
			command: "check",
			payload: func(t *testing.T) []byte {
				t.Helper()
				out, _, err := judge.RunCheck(t.Context(), &judge.CheckOptions{Root: root, Format: judge.FormatJSON})
				if err != nil {
					t.Fatalf("RunCheck: %v", err)
				}
				return out
			},
		},
		{
			command: "coverage",
			payload: func(t *testing.T) []byte {
				t.Helper()
				out, _, err := judge.RunCoverage(t.Context(), &judge.CoverageOptions{Root: root, Format: judge.FormatJSON})
				if err != nil {
					t.Fatalf("RunCoverage: %v", err)
				}
				return out
			},
		},
		{
			command:  "exists",
			trailing: []string{"Present"},
			payload: func(t *testing.T) []byte {
				t.Helper()
				out, _, err := judge.RunExists(t.Context(), &judge.ExistsOptions{Root: root, Name: "Present", Format: judge.FormatJSON})
				if err != nil {
					t.Fatalf("RunExists: %v", err)
				}
				return out
			},
		},
	} {
		t.Run(tt.command, func(t *testing.T) {
			t.Parallel()

			want := tt.payload(t)
			if len(want) == 0 {
				t.Fatalf("%s answers with nothing on this folder, so comparing the binary against it proves nothing", tt.command)
			}

			for _, asked := range []struct {
				name   string
				format []string
			}{
				{name: "the machine format named outright", format: []string{"--format=json"}},
				{name: "no format named, so a pipe decides", format: nil},
			} {
				t.Run(asked.name, func(t *testing.T) {
					t.Parallel()

					args := append([]string{tt.command, "--root=" + root}, asked.format...)
					args = append(args, tt.trailing...)
					exit, stdout, stderr := runYomihonBinary(t, binary, args, env)
					if exit != 0 {
						t.Fatalf("exit = %d, want 0; stderr:\n%s", exit, stderr)
					}
					if !bytes.Equal([]byte(stdout), want) {
						t.Errorf("the binary's stdout is not the engine's payload\n got %q\nwant %q", stdout, want)
					}
				})
			}
		})
	}
}

// adjudicableVault writes a folder the three commands can judge: the loader's
// own contract fixture, which is the module's one valid contract, and one note
// that gives check something to say and exists something to find.
func adjudicableVault(t *testing.T) string {
	t.Helper()

	contract, err := os.ReadFile(filepath.Join("..", "..", "internal", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("read the loader's contract fixture: %v", err)
	}
	root := t.TempDir()
	write := func(rel, content string) {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil { // #nosec G703 -- a path under this test's own temporary root
			t.Fatalf("WriteFile(%q) error = %v", full, err)
		}
	}
	write(schema.ContractRelPath, string(contract)+"\n[privacy]\nnever_egress_dirs = []\n")
	write("Writing/Present.md",
		"---\ntitle: Present\ntype: lesson\ndomain: meta\nstatus: draft\n"+
			"created: 2026-01-01\nupdated: 2026-01-01\nslug: present\n---\n[[Nobody Wrote This]]\n")
	return root
}
