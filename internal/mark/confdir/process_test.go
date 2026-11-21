package confdir_test

import (
	"os"
	"os/exec"
	"testing"

	"github.com/koopa0/yomihon/internal/mark/confdir"
)

// deadPID is the id of a process that has ended.
func deadPID(t *testing.T) int {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^$") // #nosec G204 G702 -- this test binary, run again so that it ends at once
	if err := cmd.Run(); err != nil {
		t.Fatalf("run a process that ends at once: %v", err)
	}
	return cmd.ProcessState.Pid()
}

func TestProcessAliveKnowsThisProcessAndOneThatEnded(t *testing.T) {
	t.Parallel()
	if !confdir.ProcessAlive(os.Getpid()) {
		t.Error("this process was called gone")
	}
	if confdir.ProcessAlive(deadPID(t)) {
		t.Error("a process that has ended was called alive")
	}
	for _, pid := range []int{0, -1} {
		if confdir.ProcessAlive(pid) {
			t.Errorf("ProcessAlive(%d) = true", pid)
		}
	}
}
