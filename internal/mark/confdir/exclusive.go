package confdir

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	// LockWait is how long a change waits for another process to finish its own.
	LockWait = 5 * time.Second
	// lockStaleAfter is how old a lock has to be before it is taken whoever
	// holds it. A change takes milliseconds; this covers a holder whose process
	// id has since been given to another process.
	lockStaleAfter = 30 * time.Second
)

// holderSeq tells two claimants in one process apart.
var holderSeq atomic.Uint64

// Exclusive runs change holding the lock called name in dir, so that a file
// read inside change and replaced there is replaced from what the file holds
// now. The command line and the running server are two processes over the same
// directory, and without this the later of two changes writes over the earlier.
// It waits up to wait for the lock, and takes one whose holder is gone.
func Exclusive(dir, name string, wait time.Duration, change func() error) (err error) {
	lock, err := Acquire(dir, name, newHolder(), abandoned, wait)
	if err != nil {
		return fmt.Errorf("lock %s: %w", name, err)
	}
	defer func() { err = errors.Join(err, lock.Release()) }()
	return change()
}

// newHolder says who holds the lock: this process, and which claim of it.
func newHolder() []byte {
	return []byte(strconv.Itoa(os.Getpid()) + " " + strconv.FormatUint(holderSeq.Add(1), 10) + " " + strconv.FormatInt(time.Now().UnixNano(), 10))
}

// abandoned is the rule for a lock whose holder never released it: its process
// is gone, or it is older than any change takes. A lock not yet written reads
// as empty and is waited out.
func abandoned(holder []byte, age time.Duration) bool {
	if age > lockStaleAfter {
		return true
	}
	pid, _, _ := strings.Cut(string(holder), " ")
	n, err := strconv.Atoi(pid)
	return err == nil && !ProcessAlive(n)
}

// ProcessAlive says whether a process with the id exists. Signal 0 delivers
// nothing and reports only whether it could be delivered; a process that is not
// ours answers permission denied, which is still a process. Windows has no
// such signal, and there a process that cannot be opened by id is gone.
func ProcessAlive(pid int) bool {
	if pid < 1 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	if runtime.GOOS == "windows" {
		return process.Release() == nil
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil || errors.Is(err, syscall.EPERM)
}
