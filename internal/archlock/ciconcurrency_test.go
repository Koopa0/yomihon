package archlock

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"go.yaml.in/yaml/v3"
)

// TestCIKeepsEachMainCommitAndSupersedesPullRequests pins the complete
// concurrency declaration, not a comment mentioning a commit or a fallback
// that a nonempty ref would keep unreachable. Pull requests share their ref;
// every other event takes the exact commit whose jobs are being scheduled.
func TestCIKeepsEachMainCommitAndSupersedesPullRequests(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join(repoRoot, ".github", "workflows", "ci.yml")) // #nosec G304 -- a fixed workflow path under the repository root
	if err != nil {
		t.Fatalf("read CI workflow: %v", err)
	}
	var workflow struct {
		Concurrency map[string]string `yaml:"concurrency"`
	}
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		t.Fatalf("decode CI workflow: %v", err)
	}
	want := map[string]string{
		"group":              "${{ github.workflow }}-${{ github.event_name == 'pull_request' && github.ref || github.sha }}",
		"cancel-in-progress": "${{ github.event_name == 'pull_request' }}",
	}
	if diff := cmp.Diff(want, workflow.Concurrency); diff != "" {
		t.Errorf("CI concurrency must preserve every main commit and supersede only pull requests (-want +got):\n%s", diff)
	}
}
