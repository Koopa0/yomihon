package judge

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// A private copy with stale JSONL files proves the maintenance command reaches
// every frozen finding, including consumers outside the check engine's table.
// Running the test binary separately keeps its working directory and opt-in
// guard away from the ordinary tests and the checkout's frozen bytes.
func TestRegenerateGoldensReproducesJSONL(t *testing.T) {
	t.Parallel()

	goldens, err := filepath.Glob(filepath.Join("testdata", "golden", "*.jsonl"))
	if err != nil || len(goldens) == 0 {
		t.Fatalf("inventory JSONL goldens: want frozen finding files (glob: %v)", err)
	}
	workspace := t.TempDir()
	root := filepath.Join(workspace, "judge")
	if copyErr := os.CopyFS(filepath.Join(root, "testdata"), os.DirFS("testdata")); copyErr != nil {
		t.Fatalf("copy judge fixtures: %v", copyErr)
	}
	// Contractless fixtures receive the same declared authority as their
	// ordinary consumers, from the schema fixture beside the judge package.
	if copyErr := os.CopyFS(filepath.Join(workspace, "schema", "testdata"), os.DirFS("../schema/testdata")); copyErr != nil {
		t.Fatalf("copy schema fixtures: %v", copyErr)
	}
	want := make(map[string][]byte, len(goldens))
	stale := []byte("stale JSONL golden\n")
	for _, name := range goldens {
		data, readErr := os.ReadFile(name) // #nosec G304 -- paths inventoried under the fixed testdata/golden directory
		if readErr != nil {
			t.Fatalf("read %s: %v", name, readErr)
		}
		want[name] = data
		if writeErr := os.WriteFile(filepath.Join(root, name), stale, 0o600); writeErr != nil {
			t.Fatalf("make %s stale in private copy: %v", name, writeErr)
		}
	}

	binary, err := os.Executable()
	if err != nil {
		t.Fatalf("locate test binary: %v", err)
	}
	cmd := exec.CommandContext(t.Context(), binary, "-test.run=^TestRegenerateGoldens$", "-test.v") // #nosec G204 -- this test's own binary and fixed arguments, in a private fixture copy
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "YOMIHON_REGEN_GOLDENS=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("regeneration failed: %v\n%s", err, output)
	}
	for _, name := range goldens {
		got, readErr := os.ReadFile(filepath.Join(root, name)) // #nosec G304 -- inventoried golden in the test-owned private copy
		if readErr != nil {
			t.Fatalf("read regenerated %s: %v", name, readErr)
		}
		if bytes.Equal(got, stale) {
			t.Errorf("caught: JSONL golden was not regenerated: %s", filepath.ToSlash(name))
		} else if !bytes.Equal(got, want[name]) {
			t.Errorf("caught: regeneration changed frozen bytes: %s\ngot:\n%s\nwant:\n%s", filepath.ToSlash(name), got, want[name])
		}
	}
}
