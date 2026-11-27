//go:build unix

package vaults

import (
	"errors"
	"syscall"
	"testing"
	"time"
)

// A named pipe where the list should be is not read: opening one waits for a
// writer that never comes, and that hung every start and ignored the interrupt
// that should end it. It is saved as damaged and the list starts fresh.
func TestAListThatIsANamedPipeIsNotOpenedAndIsSavedAsDamaged(t *testing.T) {
	t.Parallel()
	s, _ := storeIn(t, t.TempDir(), nil)
	if err := mkdirHome(s); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(s.dir.Join(listFile), 0o600); err != nil {
		t.Skipf("no named pipes here: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := s.List()
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, ErrDamaged) {
			t.Fatalf("List of a named pipe = %v, want ErrDamaged", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("reading the list waited on a named pipe")
	}

	added := make(chan error, 1)
	go func() {
		_, _, err := s.Add(folder(t, "a"), startPort, firstRead)
		added <- err
	}()
	select {
	case err := <-added:
		if err != nil {
			t.Fatalf("Add over a named pipe = %v, want the pipe saved as damaged and the vault taken in", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("adding a vault waited on a named pipe")
	}
}
