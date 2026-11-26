//go:build !windows

package machinemarks

import (
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// A named pipe where a machine file should be is not opened: a read of it would
// wait for ever for a writer.
func TestANamedPipeIsNotOpened(t *testing.T) {
	t.Parallel()
	a := newClone(t, "A")
	a.say(1, "from A")
	if err := syscall.Mkfifo(filepath.Join(a.marksDir(), otherFile), 0o600); err != nil {
		t.Skipf("no named pipes here: %v", err)
	}
	done := make(chan *View, 1)
	go func() {
		v, err := a.m.View()
		if err != nil {
			t.Error(err)
		}
		done <- v
	}()
	select {
	case v := <-done:
		if v == nil {
			return
		}
		if v.Unreadable() != 1 || a.words(1) != "from A" {
			t.Fatalf("%d unreadable, reads %q", v.Unreadable(), a.words(1))
		}
	case <-time.After(20 * time.Second):
		t.Fatal("reading a folder with a named pipe in it did not return: the pipe was opened")
	}
}
