package note_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The compiler overlay exposes the snapshot parser only to this isolated HTTP
// test process; the production package has no testing API.
func TestParseFailureReachesEveryReadingSurface(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	replacements := make(map[string]string)
	for _, item := range []struct{ source, target string }{
		{source: "parsefailure-http.go.txt", target: "internal/note/parsefailure_test.go"},
		{source: "parsefailure-snapshot.go.txt", target: "internal/snapshot/parsefailure_http_seam.go"},
	} {
		data, readErr := os.ReadFile(filepath.Join("testdata", item.source))
		if readErr != nil {
			t.Fatal(readErr)
		}
		backing := filepath.Join(dir, item.source)
		if writeErr := os.WriteFile(backing, data, 0o600); writeErr != nil { // #nosec G703 -- fixed fixture basename under this test's TempDir
			t.Fatal(writeErr)
		}
		replacements[filepath.Join(root, item.target)] = backing
	}
	data, err := json.Marshal(struct{ Replace map[string]string }{Replace: replacements})
	if err != nil {
		t.Fatal(err)
	}
	overlay := filepath.Join(dir, "overlay.json")
	if writeErr := os.WriteFile(overlay, data, 0o600); writeErr != nil {
		t.Fatal(writeErr)
	}
	args := []string{"test", "-overlay=" + overlay, "-count=1", "-timeout=45s", "-json", "-run", "^TestParseFailureReachesEveryReadingSurface$"}
	if parseFailureRace {
		args = append(args, "-race")
	}
	args = append(args, "./internal/note")
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", args...) // #nosec G204 -- fixed Go test arguments over this test-owned compiler overlay
	cmd.WaitDelay = 5 * time.Second
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	t.Logf("invoked: parse-failure HTTP child (race=%t)\n%s", parseFailureRace, output)
	if err != nil {
		t.Fatalf("caught: real HTTP child failed: %v", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(output))
	passed := make(map[string]bool)
	invoked := false
	for decoder.More() {
		var event struct{ Action, Test, Output string }
		if err := decoder.Decode(&event); err != nil {
			t.Fatalf("child test event: %v", err)
		}
		if event.Action == "pass" {
			passed[event.Test] = true
		}
		if strings.Contains(event.Output, "invoked: snapshot parser HTTP fixture") {
			invoked = true
		}
	}
	if !invoked || !passed[t.Name()] {
		t.Fatal("caught: HTTP child did not execute")
	}
	for _, lang := range []string{"zh-Hant", "en"} {
		for _, surface := range []string{"initial", "retained", "compare-a", "compare-b", "health", "recovery", "absent", "healthy"} {
			if !passed[t.Name()+"/"+lang+"/"+surface] {
				t.Errorf("caught: HTTP child omitted %s/%s", lang, surface)
			}
		}
	}
}
