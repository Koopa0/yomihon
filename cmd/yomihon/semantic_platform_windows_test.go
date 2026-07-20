//go:build windows

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/search/agent"
)

func TestSemanticRenewalReportsUnsupportedPlatformOnWindows(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeRecoverySiteFixture(t, root)
	contractPath := filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))
	contract, err := os.ReadFile(contractPath) // #nosec G304 -- fixed fixture path under t.TempDir
	if err != nil {
		t.Fatalf("read schema fixture: %v", err)
	}
	contract = append(contract, []byte("\n[privacy]\nnever_egress_dirs = []\n")...)
	if err := os.WriteFile(contractPath, contract, 0o600); err != nil { // #nosec G703 -- fixed fixture path under t.TempDir
		t.Fatalf("write schema fixture: %v", err)
	}
	var stdout, stderr bytes.Buffer
	exit := (agent.CLI{
		Root:   root,
		Stdout: &stdout,
		Stderr: &stderr,
		ReadEmbeddingKey: func() string {
			t.Error("unsupported platform read the embedding key")
			return "sentinel-key"
		},
	}).Run(t.Context(), "search-index", []string{
		"build",
		"--json",
		"--renew-attempt-budget",
	})
	if exit != 3 {
		t.Fatalf("search-index renewal exit = %d, want 3; stderr = %q", exit, stderr.Bytes())
	}
	wantStdout := "{\"error\":{\"reason\":\"unsupported-platform\",\"active_generation\":\"not-inspected\",\"staging_generation\":\"not-inspected\",\"retry_safe\":false,\"next_action\":\"use-supported-platform\"}}\n"
	if got := stdout.String(); got != wantStdout {
		t.Errorf("search-index renewal stdout = %q, want %q", got, wantStdout)
	}
	wantStderr := "yomihon search-index: unsupported-platform: the semantic generation store is not supported on this platform\n"
	if got := stderr.String(); got != wantStderr {
		t.Errorf("search-index renewal stderr = %q, want %q", got, wantStderr)
	}
}
