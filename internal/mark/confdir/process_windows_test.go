package confdir_test

import (
	"os"
	"os/exec"
	"testing"

	"github.com/koopa0/yomihon/internal/mark/confdir"
)

// TestAProcessThatEndedIsGoneWhileAHandleToItIsOpen holds what Windows makes
// easy to get wrong: a process object outlives its process for as long as any
// handle to it is open, a scanner's or another program's, so a process that can
// be opened by its id may have ended. The test holds such a handle itself, from
// before each process ends until after it is asked about, so the case is not
// left to whoever else happens to have one.
func TestAProcessThatEndedIsGoneWhileAHandleToItIsOpen(t *testing.T) {
	t.Parallel()
	const ended = 3
	calledAlive := 0
	for range ended {
		cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^$") // #nosec G204 G702 -- this test binary, run again so that it ends at once
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		held, err := os.FindProcess(cmd.Process.Pid)
		if err != nil {
			t.Fatalf("open a handle to the process: %v", err)
		}
		if err := cmd.Wait(); err != nil {
			t.Fatalf("run a process that ends at once: %v", err)
		}
		if confdir.ProcessAlive(cmd.Process.Pid) {
			calledAlive++
		}
		if err := held.Release(); err != nil {
			t.Fatalf("close the handle: %v", err)
		}
	}
	if calledAlive > 0 {
		t.Errorf("%d of %d processes that had ended, each with a handle to it still open, were called alive", calledAlive, ended)
	}
}
