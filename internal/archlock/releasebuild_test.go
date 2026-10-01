package archlock

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTheReleaseBuildTrimsPathsAndStripsSymbols holds every `go build` in the
// release workflow to -trimpath and -ldflags="-s -w". -trimpath keeps the
// runner's checkout path out of each binary, so a rebuild of the tagged commit
// from a clean checkout yields the published SHA-256; -s -w drops the symbol
// table and DWARF data a downloaded binary never uses. A workflow with no
// `go build` line fails too, so a moved or emptied file cannot pass.
func TestTheReleaseBuildTrimsPathsAndStripsSymbols(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(filepath.Join(repoRoot, ".github", "workflows", "release.yml")) // #nosec G304 -- a fixed path under the repository root
	if err != nil {
		t.Fatal(err)
	}
	builds := 0
	for i, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") || !strings.Contains(line, "go build") {
			continue
		}
		builds++
		for _, flag := range []string{"-trimpath", `-ldflags="-s -w"`} {
			if !strings.Contains(line, flag) {
				t.Errorf("release.yml:%d: this go build lacks %s: %s", i+1, flag, strings.TrimSpace(line))
			}
		}
	}
	if builds == 0 {
		t.Fatal("release.yml holds no go build line, so there is no release build to hold")
	}
}
