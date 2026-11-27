package vaults

import (
	"errors"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/mark/confdir"
)

// runtimeHasModes says whether file modes mean anything on this system.
func runtimeHasModes() bool { return runtime.GOOS != "windows" }

func TestARunningInstanceIsOneWhoseProcessIsStillThere(t *testing.T) {
	t.Parallel()
	s, _ := storeIn(t, t.TempDir(), nil)
	if _, ok := s.Running(); ok {
		t.Fatal("an instance was running with no file")
	}
	if _, err := s.ReadInstance(); !errors.Is(err, ErrNoInstance) {
		t.Fatalf("ReadInstance with no file = %v, want ErrNoInstance", err)
	}

	mine := Instance{PID: os.Getpid(), Port: 8800, Started: time.Now().UTC()}
	if err := s.WriteInstance(mine); err != nil {
		t.Fatal(err)
	}
	got, ok := s.Running()
	if !ok || got.PID != mine.PID || got.Port != 8800 {
		t.Fatalf("Running = %+v, %v; want %+v", got, ok, mine)
	}
}

func TestAnInstanceLeftByACrashIsNotRunningAndIsOverwritten(t *testing.T) {
	t.Parallel()
	s, _ := storeIn(t, t.TempDir(), nil)
	if err := s.WriteInstance(Instance{PID: deadPID(t), Port: 8800, Started: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	if in, ok := s.Running(); ok {
		t.Fatalf("a crashed process's instance reads as running: %+v", in)
	}

	if err := s.WriteInstance(Instance{PID: os.Getpid(), Port: 8801, Started: time.Now().UTC()}); err != nil {
		t.Fatalf("the leftover could not be overwritten: %v", err)
	}
	if in, ok := s.Running(); !ok || in.Port != 8801 {
		t.Fatalf("Running after the overwrite = %+v, %v", in, ok)
	}
}

func TestRemoveInstanceLeavesAnotherProcessesRecordAlone(t *testing.T) {
	t.Parallel()
	s, _ := storeIn(t, t.TempDir(), nil)
	if err := s.WriteInstance(Instance{PID: os.Getpid(), Port: 8800, Started: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveInstance(os.Getpid() + 1); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Running(); !ok {
		t.Fatal("a process removed another's record")
	}
	if err := s.RemoveInstance(os.Getpid()); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReadInstance(); !errors.Is(err, ErrNoInstance) {
		t.Fatalf("the process's own record is still there: %v", err)
	}
	if err := s.RemoveInstance(os.Getpid()); err != nil {
		t.Fatalf("RemoveInstance with no record = %v, want nil", err)
	}
}

// A record that cannot be read is not taken for none: removing it says why it
// could not, rather than leaving it behind as though nothing were there.
func TestARecordThatCannotBeReadIsNotRemovedAsIfNoneWereThere(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not refuse a file by its permission bits")
	}
	s, _ := storeIn(t, t.TempDir(), nil)
	if err := s.WriteInstance(Instance{PID: os.Getpid(), Port: 8800, Started: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	path := s.InstanceFile()
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(path, 0o600); err != nil {
			t.Errorf("restore the file's mode: %v", err)
		}
	})
	if _, err := os.ReadFile(path); err == nil { // #nosec G304 -- the test's own temporary directory
		t.Skip("this user reads a file it may not read (a privileged user), so the fixture shows nothing")
	}
	if err := s.RemoveInstance(os.Getpid()); err == nil {
		t.Fatal("RemoveInstance of a record that cannot be read = nil, want why it could not")
	}
}

func TestAnInstanceFileNothingCouldHaveWrittenIsNoInstance(t *testing.T) {
	t.Parallel()
	for name, content := range map[string]string{
		"not json":     "yomihon is running",
		"no process":   `{"pid":0,"port":8800}`,
		"no port":      `{"pid":1,"port":0}`,
		"a port above": `{"pid":1,"port":70000}`,
	} {
		config := t.TempDir()
		s, _ := storeIn(t, config, nil)
		if err := confdir.Replace(homeAt(t, config), instanceFile, []byte(content)); err != nil {
			t.Fatal(err)
		}
		if _, err := s.ReadInstance(); !errors.Is(err, ErrNoInstance) {
			t.Errorf("%s: ReadInstance = %v, want ErrNoInstance", name, err)
		}
		if _, ok := s.Running(); ok {
			t.Errorf("%s: read as a running instance", name)
		}
	}
}

func TestAnInstanceNeedsAProcessAndAPort(t *testing.T) {
	t.Parallel()
	s, _ := storeIn(t, t.TempDir(), nil)
	for _, in := range []Instance{{PID: 0, Port: 8800}, {PID: 1, Port: 0}, {PID: 1, Port: 65536}} {
		if err := s.WriteInstance(in); err == nil {
			t.Errorf("WriteInstance accepted %+v", in)
		}
	}
}
