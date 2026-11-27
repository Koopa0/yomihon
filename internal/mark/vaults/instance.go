package vaults

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/koopa0/yomihon/internal/mark/confdir"
)

// ErrNoInstance means no running yomihon has said that it is one.
var ErrNoInstance = errors.New("no running yomihon is recorded")

// maxInstanceBytes bounds what a read takes in; the file is three small fields.
const maxInstanceBytes = 4 << 10

// Instance is a running yomihon's account of itself. It exists so that the
// command line can tell a yomihon that is already serving from another program
// holding the port, without connecting to anything: a yomihon writes it when it
// starts and removes it when it ends, and one left by a crash names a process
// that is gone.
type Instance struct {
	PID int `json:"pid"`
	// Port is the starting port the process holds. A process that holds it is
	// the one a second `yomihon <folder>` has to hand the folder to.
	Port    int       `json:"port"`
	Started time.Time `json:"started"`
}

// WriteInstance records that this process is a running yomihon.
func (s *Store) WriteInstance(in Instance) error {
	if in.PID < 1 || in.Port < MinPort || in.Port > maxPort {
		return fmt.Errorf("an instance needs a process id and a port, not %d and %d", in.PID, in.Port)
	}
	data, err := json.MarshalIndent(in, "", "  ")
	if err != nil {
		return fmt.Errorf("encode the running instance: %w", err)
	}
	if err := confdir.Replace(s.dir, instanceFile, append(data, '\n')); err != nil {
		return fmt.Errorf("record the running instance: %w", err)
	}
	return nil
}

// ReadInstance is what the instance file says. A file that is not there, or
// that says nothing a process could have written, is ErrNoInstance: the same
// answer, because the command line does the same in both. A file that cannot be
// read is that error and not ErrNoInstance, since nothing is known of it.
func (s *Store) ReadInstance() (Instance, error) {
	raw, err := confdir.Read(s.dir, instanceFile, maxInstanceBytes)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return Instance{}, ErrNoInstance
	case errors.Is(err, confdir.ErrTooLarge), errors.Is(err, confdir.ErrNotRegular):
		return Instance{}, fmt.Errorf("%w: %w", ErrNoInstance, err)
	case err != nil:
		return Instance{}, fmt.Errorf("read the running instance: %w", err)
	}
	var in Instance
	if err := json.Unmarshal(raw, &in); err != nil || in.PID < 1 || in.Port < MinPort || in.Port > maxPort {
		return Instance{}, fmt.Errorf("%w: the file is not one this wrote", ErrNoInstance)
	}
	return in, nil
}

// Running is the recorded instance when its process is still there. A file a
// crash left names a process that is gone, which is how the command line learns
// that nothing is running without asking the network.
func (s *Store) Running() (Instance, bool) {
	in, err := s.ReadInstance()
	if err != nil || !confdir.ProcessAlive(in.PID) {
		return Instance{}, false
	}
	return in, true
}

// RemoveInstance removes the instance file if it is pid's. A process that ends
// leaves nothing that says it is running, and one that was replaced by another
// does not take the other's record with it. A file that cannot be read is not
// removed, and the error says why.
func (s *Store) RemoveInstance(pid int) error {
	in, err := s.ReadInstance()
	if err != nil {
		if errors.Is(err, ErrNoInstance) {
			return nil
		}
		return err
	}
	if in.PID != pid {
		return nil
	}
	if err := confdir.Remove(s.dir, instanceFile); err != nil {
		return fmt.Errorf("remove the running instance: %w", err)
	}
	return nil
}
