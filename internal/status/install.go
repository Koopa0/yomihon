package status

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// installRung names how far the filesystem under a note lets the write face go
// toward one guarantee: an edit another program makes while the flip is
// installing must still be on disk afterwards. The rungs promise different
// things, so the one in force is part of every install failure shown.
type installRung int

const (
	// rungUnknown is the zero value, so a rung nobody chose never reads as
	// the strongest one.
	rungUnknown installRung = iota
	// rungExchange swaps the prepared replacement with the note in one atomic
	// step, leaving the displaced version under the temporary name where it
	// can be read back and, if it is someone else's edit, put straight back.
	rungExchange
	// rungHardlink keeps a second name for the current version across a plain
	// rename. It catches an editor writing through the existing file; one that
	// replaces the name in the window between the two is still lost.
	rungHardlink
	// rungRename is a plain rename and promises nothing about the window: it
	// is the last resort on volumes that offer neither of the other two.
	rungRename
)

func (r installRung) String() string {
	switch r {
	case rungExchange:
		return "atomic exchange"
	case rungHardlink:
		return "retained hardlink"
	case rungRename:
		return "plain rename"
	case rungUnknown:
		return "unselected"
	}
	return "unselected"
}

// restoreAttempts bounds how many times the write face will try to put a
// racing edit back under the note's name. Each attempt preserves both
// versions, so exhausting them costs nothing but the refusal that follows.
const restoreAttempts = 2

// errNotSingleComponent guards the raw directory-relative syscalls: every name
// they receive has to be one entry inside the already-confined parent.
var errNotSingleComponent = errors.New("install name is not a single directory entry")

// installOps is the filesystem behind one install, named operation by
// operation so a test can reproduce a driver that reports success without
// swapping anything.
type installOps struct {
	swap   func(from, to string) error
	read   func(name string) ([]byte, error)
	link   func(oldname, newname string) error
	rename func(from, to string) error
	remove func(name string) error
}

// rootOps binds the install operations to one confined parent directory.
func rootOps(parent *os.Root) installOps {
	return installOps{
		swap:   func(from, to string) error { return exchangeAt(parent, from, to) },
		read:   parent.ReadFile,
		link:   parent.Link,
		rename: parent.Rename,
		remove: parent.Remove,
	}
}

// exchangeAt atomically swaps two directory entries inside parent. Both names
// are single entries inside the confined parent, so the swap cannot reach any
// path the walk that produced parent did not already validate.
func exchangeAt(parent *os.Root, from, to string) error {
	if err := singleComponent(from); err != nil {
		return err
	}
	if err := singleComponent(to); err != nil {
		return err
	}
	dir, err := parent.Open(".")
	if err != nil {
		return fmt.Errorf("open install directory: %w", err)
	}
	err = exchangeNames(int(dir.Fd()), from, to)
	// A close failure afterwards cannot change what the filesystem already did.
	_ = dir.Close() //nolint:errcheck // directory-descriptor cleanup is best-effort
	return err
}

func singleComponent(name string) error {
	if name == "" || name == "." || name == ".." ||
		strings.ContainsRune(name, '/') || strings.ContainsRune(name, filepath.Separator) {
		return fmt.Errorf("%w: %q", errNotSingleComponent, name)
	}
	return nil
}

// exchangeProbes remembers, per filesystem, which rung that filesystem earned.
// Keyed by device rather than directory: the answer is a property of the
// driver, and a vault spread over several mounts probes each one.
var exchangeProbes sync.Map

// selectRung reports the strongest install rung this directory's
// filesystem has been shown to support.
func selectRung(parent *os.Root, hooks installHooks) installRung {
	if hooks.rung != nil {
		return hooks.rung()
	}
	ops := installOpsFor(parent, hooks)
	key, ok := deviceKey(parent)
	if !ok {
		return probeRung(parent, ops)
	}
	if cached, hit := exchangeProbes.Load(key); hit {
		if rung, is := cached.(installRung); is {
			return rung
		}
	}
	rung := probeRung(parent, ops)
	// Only a rung the probe positively established is remembered. Every other
	// answer is reached by something going wrong — a throwaway file that could
	// not be written, a swap the driver refused — and a failure measures
	// nothing about what the driver supports, so it is forgotten and the next
	// flip asks again rather than pinning the process to a weaker install.
	if rung == rungExchange {
		exchangeProbes.Store(key, rung)
	}
	return rung
}

// installOpsFor is the set of operations an install performs on parent, with
// any caller-supplied wrapping applied. Selecting a rung and performing the
// install see the same operations, or the probe measures the wrong thing.
func installOpsFor(parent *os.Root, hooks installHooks) installOps {
	ops := rootOps(parent)
	if hooks.ops != nil {
		ops = hooks.ops(ops)
	}
	return ops
}

// deviceKey identifies the filesystem parent lives on.
func deviceKey(parent *os.Root) (any, bool) {
	info, err := parent.Stat(".")
	if err != nil {
		return nil, false
	}
	return deviceIdentity(info)
}

// probeRung decides what this filesystem may be trusted with by trying it on
// two throwaway entries of its own. The driver's own answer is not evidence:
// some volumes accept a swap, report success, and perform a destructive plain
// rename instead, so the probe believes the rung only when both entries are
// still there afterwards carrying each other's bytes.
func probeRung(parent *os.Root, ops installOps) installRung {
	const (
		firstBytes  = "yomihon install probe 1"
		secondBytes = "yomihon install probe 2"
	)
	// The probe's files are thrown away, so they skip the durability barrier.
	noSync := func(*os.File) error { return nil }
	first, err := writeTemp(parent, []byte(firstBytes), 0o600, noSync)
	if err != nil {
		return rungRename
	}
	defer func() { _ = ops.remove(first) }() //nolint:errcheck // probe cleanup is best-effort
	second, err := writeTemp(parent, []byte(secondBytes), 0o600, noSync)
	if err != nil {
		return rungRename
	}
	defer func() { _ = ops.remove(second) }() //nolint:errcheck // probe cleanup is best-effort

	if ops.swap(first, second) == nil {
		back, firstErr := ops.read(first)
		forth, secondErr := ops.read(second)
		if firstErr == nil && secondErr == nil &&
			string(back) == secondBytes && string(forth) == firstBytes {
			return rungExchange
		}
	}
	// A refused swap leaves both entries; one that lied destroyed the source,
	// so the surviving destination is the one to ask about a second name.
	link := tempName()
	if ops.link(second, link) != nil {
		return rungRename
	}
	_ = ops.remove(link) //nolint:errcheck // probe cleanup is best-effort
	return rungHardlink
}

// installRewritten puts the rewritten bytes under the note's own name and owns
// tmpName from the moment it is called: on every return path that entry has
// been consumed, removed after its bytes were recognized, or left in place and
// named in the error. Nothing here removes bytes it has not read and matched.
func installRewritten(
	ops installOps,
	relSlash, tmpName string,
	rung installRung,
	source *fileSnapshot,
	data []byte,
) error {
	switch rung {
	case rungExchange:
		return installByExchange(ops, relSlash, tmpName, source, data)
	case rungHardlink:
		return installByHardlink(ops, relSlash, tmpName, source)
	case rungRename, rungUnknown:
		return installByRename(ops, tmpName, source)
	}
	return installByRename(ops, tmpName, source)
}

// installByExchange swaps the prepared replacement with the note in one atomic
// step and reads what the swap displaced. Anything but the version the flip
// was computed from is an edit that landed inside the window, already
// preserved under the temporary name rather than overwritten.
func installByExchange(ops installOps, relSlash, tmpName string, source *fileSnapshot, data []byte) error {
	if err := ops.swap(tmpName, source.name); err != nil {
		_ = ops.remove(tmpName) //nolint:errcheck // best-effort cleanup after the primary swap error
		return fmt.Errorf("exchange temp file: %w", err)
	}
	displaced, err := ops.read(tmpName)
	if err != nil {
		return strandedError(relSlash, tmpName, rungExchange, fmt.Errorf("read the displaced version: %w", err))
	}
	if bytes.Equal(displaced, source.data) {
		// An editor holding the displaced file open still writes there, so the
		// entry is read again immediately before removal. No single step both
		// compares an entry and unlinks it, so this narrows the window without
		// closing it; a write inside what remains is restored, never removed.
		again, readErr := ops.read(tmpName)
		if errors.Is(readErr, fs.ErrNotExist) {
			// The entry is already gone, which is where removing it would
			// have arrived, so nothing is reported as being beside the note.
			return nil
		}
		if readErr != nil {
			return strandedError(relSlash, tmpName, rungExchange,
				fmt.Errorf("confirm the displaced version before removing it: %w", readErr))
		}
		if bytes.Equal(again, source.data) {
			_ = ops.remove(tmpName) //nolint:errcheck // the displaced entry holds the pre-flip bytes, just read again
			return nil
		}
	}
	// Another program reached the note inside the window; its bytes sit under
	// the temporary name, so one more exchange puts them back and the flip is
	// refused rather than installed over them.
	for range restoreAttempts {
		if err = ops.swap(tmpName, source.name); err != nil {
			return strandedError(relSlash, tmpName, rungExchange, fmt.Errorf("restore the external edit: %w", err))
		}
		back, readErr := ops.read(tmpName)
		if readErr != nil {
			return strandedError(relSlash, tmpName, rungExchange, fmt.Errorf("confirm the restored note: %w", readErr))
		}
		if bytes.Equal(back, data) {
			_ = ops.remove(tmpName) //nolint:errcheck // these are the bytes this flip wrote and never installed
			return concurrentWriteError(relSlash, rungExchange)
		}
	}
	return strandedError(relSlash, tmpName, rungExchange,
		errors.New("the note changed again while the external edit was being restored"))
}

// installByHardlink takes a second name for the current version, installs
// through a plain rename, then reads that second name back. An editor writing
// through the existing file is visible there; one that replaces the note's
// name after the second is taken is not, which is why this rung is a fallback.
func installByHardlink(ops installOps, relSlash, tmpName string, source *fileSnapshot) error {
	retained := tempName()
	if err := ops.link(source.name, retained); err != nil {
		_ = ops.remove(tmpName) //nolint:errcheck // best-effort cleanup after the primary link error
		return fmt.Errorf("retain the current version: %w", err)
	}
	if err := ops.rename(tmpName, source.name); err != nil {
		_ = ops.remove(tmpName)  //nolint:errcheck // best-effort cleanup after the primary rename error
		_ = ops.remove(retained) //nolint:errcheck // the retained name holds the unchanged current version
		return fmt.Errorf("rename temp file: %w", err)
	}
	kept, err := ops.read(retained)
	if err != nil {
		return strandedError(relSlash, retained, rungHardlink, fmt.Errorf("read the retained version: %w", err))
	}
	if bytes.Equal(kept, source.data) {
		_ = ops.remove(retained) //nolint:errcheck // the retained name holds the pre-flip bytes the caller already read
		return nil
	}
	if err = ops.rename(retained, source.name); err != nil {
		return strandedError(relSlash, retained, rungHardlink, fmt.Errorf("restore the external edit: %w", err))
	}
	return concurrentWriteError(relSlash, rungHardlink)
}

// installByRename installs through one atomic rename and can say nothing
// about the window it crosses.
func installByRename(ops installOps, tmpName string, source *fileSnapshot) error {
	if err := ops.rename(tmpName, source.name); err != nil {
		_ = ops.remove(tmpName) //nolint:errcheck // best-effort cleanup after the primary rename error
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}

func concurrentWriteError(relSlash string, rung installRung) error {
	return fmt.Errorf("%w: %s changed while flipping, and the other program's bytes were put back (install: %s)",
		ErrConcurrentWrite, relSlash, rung)
}

// strandedError reports an install that could neither finish nor tidy up,
// because the entry left beside the note may hold bytes yomihon did not write.
// It names that entry and removes nothing on the way out.
func strandedError(relSlash, kept string, rung installRung, cause error) error {
	return fmt.Errorf("%w: %s: %w; the other version is kept beside the note as %q (install: %s)",
		ErrInstallStranded, relSlash, cause, kept, rung)
}
