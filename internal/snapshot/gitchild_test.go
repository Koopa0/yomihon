package snapshot_test

import (
	"os"
	"testing"

	"github.com/koopa0/yomihon/internal/status"
)

// TestMain answers the write face's own re-execution of this binary. A
// degraded generation is read back here through the reading pages, which carry
// the write face, and that face runs git by re-executing the running program
// with a marker argument. Without this hook the marker means nothing and the
// child runs the whole suite again, each run spawning more.
func TestMain(m *testing.M) {
	if handled, exit := status.RunGitChild(os.Args[1:], os.Stderr); handled {
		os.Exit(exit)
	}
	os.Exit(m.Run())
}
