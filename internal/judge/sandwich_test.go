package judge

import (
	"bytes"
	"errors"
	"os/exec"
	"testing"
)

// TestJudgeMatchesReferenceOnRealVault is the retirement gate's engineering
// evidence: over the live vault, every judge surface kurodo produces is
// asserted byte-identical to the reference tool's — the JSONL, the human and
// markdown reports, coverage, exists over several names, and the exit code
// under a gate. It is the quiescent sandwich: the reference is run twice per
// surface, and a difference between the two runs means the vault changed under
// the test, so that surface is skipped rather than compared against a moving
// target.
//
// Only the markdown body is expected to differ, and only in its tool-identity
// preamble, which the comparison accounts for. Any other difference is a real
// divergence between the two engines — the signal to stop and adjudicate, not
// to paper over — so the assertions compare directly and fail loudly. The live
// vault currently produces no finding that exercises a known divergence, so the
// surfaces match exactly today.
func TestJudgeMatchesReferenceOnRealVault(t *testing.T) {
	// The subtests run the reference binary sequentially rather than in
	// parallel, to avoid many concurrent scans of the same live vault.
	tool := referenceTool()
	if tool == "" {
		t.Skip("set KURODO_REFERENCE_BIN to the conformance reference binary to run this check")
	}
	vault := referenceVault()
	if vault == "" {
		t.Skip("vault not found; set KURODO_VAULT to a root holding the contract")
	}

	t.Run("check json", func(t *testing.T) {
		want := quiescentReference(t, tool, "check", "--root", vault, "--format", "json")
		got, _, err := RunCheck(&CheckOptions{Root: vault, Format: FormatJSON})
		if err != nil {
			t.Fatalf("RunCheck: %v", err)
		}
		assertMatchesReference(t, "check json", got, want)
	})

	t.Run("check human", func(t *testing.T) {
		want := quiescentReference(t, tool, "check", "--root", vault, "--format", "human")
		got, _, err := RunCheck(&CheckOptions{Root: vault, Format: FormatHuman})
		if err != nil {
			t.Fatalf("RunCheck: %v", err)
		}
		assertMatchesReference(t, "check human", got, want)
	})

	t.Run("check markdown body", func(t *testing.T) {
		// The two-line tool-identity preamble differs by design — kurodo names
		// itself rather than filing a note under the reference tool's name — so
		// the comparison is of the report body after the frontmatter and
		// heading, which is byte-identical. kurodo's exact preamble is pinned by
		// the report golden test instead.
		reference := quiescentReference(t, tool, "check", "--root", vault, "--format", "md")
		got, _, err := RunCheck(&CheckOptions{Root: vault, Format: FormatMarkdown})
		if err != nil {
			t.Fatalf("RunCheck: %v", err)
		}
		assertMatchesReference(t, "check markdown body", reportBody(got), reportBody(reference))
	})

	t.Run("coverage json", func(t *testing.T) {
		want := quiescentReference(t, tool, "coverage", "--root", vault, "--format", "json")
		got, _, err := RunCoverage(&CoverageOptions{Root: vault, Format: FormatJSON})
		if err != nil {
			t.Fatalf("RunCoverage: %v", err)
		}
		assertMatchesReference(t, "coverage json", got, want)
	})

	t.Run("coverage human", func(t *testing.T) {
		want := quiescentReference(t, tool, "coverage", "--root", vault, "--format", "human")
		got, _, err := RunCoverage(&CoverageOptions{Root: vault, Format: FormatHuman})
		if err != nil {
			t.Fatalf("RunCoverage: %v", err)
		}
		assertMatchesReference(t, "coverage human", got, want)
	})

	t.Run("exists", func(t *testing.T) {
		// A byte-for-byte match must hold whether or not the name resolves, so
		// the set spans a plausible hit and a certain miss.
		for _, name := range []string{"Go Slice", "concept", "definitely-not-a-real-note-xyz"} {
			want := quiescentReference(t, tool, "exists", "--root", vault, "--format", "json", name)
			got, _, err := RunExists(&ExistsOptions{Root: vault, Name: name, Format: FormatJSON})
			if err != nil {
				t.Fatalf("RunExists(%q): %v", name, err)
			}
			assertMatchesReference(t, "exists "+name, got, want)
		}
	})

	t.Run("deny error exit code", func(t *testing.T) {
		want := referenceExitCode(t, tool, "check", "--root", vault, "--format", "json", "--deny", "error")
		_, got, err := RunCheck(&CheckOptions{Root: vault, Format: FormatJSON, Deny: []string{"error"}})
		if err != nil {
			t.Fatalf("RunCheck: %v", err)
		}
		if got != want {
			t.Errorf("--deny error exit code = %d, want %d (the reference's)", got, want)
		}
	})
}

// quiescentReference runs the reference tool twice for one surface and returns
// its stdout, skipping the surface when the two runs differ — a signal that the
// vault changed under the test and the comparison would be against a moving
// target.
func quiescentReference(t *testing.T, tool string, args ...string) []byte {
	t.Helper()
	first := referenceStdout(t, tool, args...)
	second := referenceStdout(t, tool, args...)
	if !bytes.Equal(first, second) {
		t.Skipf("vault is not quiescent for %v; two reference runs differ", args)
	}
	return first
}

// referenceStdout runs the reference tool and returns its stdout, tolerating a
// nonzero exit (an exists miss or a gate hit still prints valid output). A
// failure to start the tool at all is fatal.
func referenceStdout(t *testing.T, tool string, args ...string) []byte {
	t.Helper()
	// #nosec G204 -- tool is resolved from a trusted environment variable set by the operator, not from user input
	out, err := exec.CommandContext(t.Context(), tool, args...).Output()
	if err != nil {
		if _, ok := errors.AsType[*exec.ExitError](err); !ok {
			t.Fatalf("run reference %v: %v", args, err)
		}
	}
	return out
}

// referenceExitCode runs the reference tool and returns its process exit code.
func referenceExitCode(t *testing.T, tool string, args ...string) int {
	t.Helper()
	// #nosec G204 -- tool is resolved from a trusted environment variable set by the operator, not from user input
	err := exec.CommandContext(t.Context(), tool, args...).Run()
	if err == nil {
		return 0
	}
	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		return exit.ExitCode()
	}
	t.Fatalf("run reference %v: %v", args, err)
	return -1
}

// reportBody returns a markdown report's body: everything after the
// frontmatter block and the heading, which the renderer separates from the body
// by a blank line each. Comparing the body sidesteps the deliberate difference
// in the tool-identity preamble.
func reportBody(md []byte) []byte {
	parts := bytes.SplitN(md, []byte("\n\n"), 3)
	if len(parts) < 3 {
		return md
	}
	return parts[2]
}

// assertMatchesReference fails when the bytes differ, framing the failure as the
// divergence signal it is.
func assertMatchesReference(t *testing.T, label string, got, want []byte) {
	t.Helper()
	if !bytes.Equal(got, want) {
		t.Errorf("%s differs from the reference on the live vault — a real divergence to bring to review\ngot:\n%s\nwant:\n%s",
			label, got, want)
	}
}
