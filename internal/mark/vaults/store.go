package vaults

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/koopa0/yomihon/internal/mark/confdir"
	"github.com/koopa0/yomihon/internal/vault"
)

const (
	// lockWait is how long a change waits for another process to finish its own.
	lockWait = 5 * time.Second
	// lockStaleAfter is how old a lock has to be before it is taken whoever
	// holds it. A change takes milliseconds; this covers a holder whose process
	// id has since been given to another process.
	lockStaleAfter = 30 * time.Second
)

// holderSeq tells two claimants in one process apart.
var holderSeq atomic.Uint64

// Store is the list of vaults and the file that says a yomihon is running, both
// in the reader's own directory. One value serves one process: the command line
// and the running server are two processes with two Stores over the same files,
// and the lock file is what orders their changes.
type Store struct {
	dir  string
	free func(port int) bool
	now  func() time.Time
	wait time.Duration

	mu           sync.Mutex
	stamp        stamp
	touched      map[string]touch
	removed      map[string]Entry
	removedOrder []string
}

// stamp is how the list looked the last time it was read.
type stamp struct {
	present  bool
	size     int64
	modified time.Time
}

func (a stamp) equal(b stamp) bool {
	return a.present == b.present && a.size == b.size && a.modified.Equal(b.modified)
}

// touch is what this process last wrote of a vault's last-read time and name.
type touch struct {
	at   time.Time
	name string
}

// Open is the Store of the reader's directory under configDir. free says
// whether a port can be bound right now; it is how a new vault is kept off a
// port something else holds, and nil says every port is free. Nothing is read or
// written until the Store is used.
func Open(configDir string, free func(port int) bool) (*Store, error) {
	dir, err := confdir.Home(configDir)
	if err != nil {
		return nil, err
	}
	return &Store{
		dir:     dir,
		free:    free,
		now:     time.Now,
		wait:    lockWait,
		touched: map[string]touch{},
		removed: map[string]Entry{},
	}, nil
}

// List is every vault, the one read most recently first. A list that cannot be
// read answers ErrDamaged and no vaults; the next change sets it aside.
func (s *Store) List() ([]Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, _, err := s.read()
	if err != nil {
		return nil, err
	}
	return recentFirst(list), nil
}

// Changed says whether the list is other than it was when it was last read. It
// looks at the file's length and modification time and at nothing inside it,
// which is what makes it cheap enough to ask twice a second.
func (s *Store) Changed() (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now, err := s.statList()
	if err != nil {
		return false, err
	}
	return !s.stamp.equal(now), nil
}

// Add puts the folder root in the list if it is not there, and answers its
// entry and whether it was new. A new vault is given the first port from start
// up that no other vault holds and that free allows, and the port is written
// down: it is the vault's from then on. A vault already listed keeps its port.
// Adding is the command line's: it is given a path because the reader typed one.
func (s *Store) Add(root string, start int) (entry Entry, added bool, err error) {
	if start < 1 || start > maxPort {
		return Entry{}, false, ErrStartPort
	}
	_, resolved, err := vault.ResolveRoot(root)
	if err != nil {
		return Entry{}, false, err
	}
	id := confdir.VaultID(resolved)

	s.mu.Lock()
	defer s.mu.Unlock()
	at := s.now()
	err = s.update(func(list []Entry) ([]Entry, bool, error) {
		if i := indexOf(list, id); i >= 0 {
			// Typed again: the reader is about to read it, which is a read.
			if at.Sub(list[i].LastRead) >= touchEvery {
				list[i].LastRead = at
				entry = list[i]
				return list, true, nil
			}
			entry = list[i]
			return list, false, nil
		}
		if len(list) >= maxVaults {
			return nil, false, ErrFull
		}
		port, portErr := s.choosePort(list, start)
		if portErr != nil {
			return nil, false, portErr
		}
		entry = Entry{ID: id, Path: resolved, Name: cleanName("", resolved), LastRead: at, Port: port}
		added = true
		return append(list, entry), true, nil
	})
	if err != nil {
		return Entry{}, false, err
	}
	s.forget(id)
	// The page the reader is about to open counts as read now, so it is not a
	// second write.
	s.touched[id] = touch{at: entry.LastRead, name: entry.Name}
	return entry, added, nil
}

// Touch says that the reader loaded a page of the vault, and what the vault is
// called now. It writes only when the last-read time is a minute stale or the
// name changed, and it is answered from memory when this process wrote both
// less than a minute ago, so a page load costs no file access. An empty name
// leaves the name as it is. It reports whether anything was written.
func (s *Store) Touch(id, name string, at time.Time) (written bool, err error) {
	name = cleanName(name, "")

	s.mu.Lock()
	defer s.mu.Unlock()
	if last, ok := s.touched[id]; ok && at.Sub(last.at) < touchEvery && (name == "" || name == last.name) {
		return false, nil
	}
	err = s.update(func(list []Entry) ([]Entry, bool, error) {
		i := indexOf(list, id)
		if i < 0 {
			return nil, false, ErrUnknown
		}
		stale := at.Sub(list[i].LastRead) >= touchEvery
		renamed := name != "" && name != list[i].Name
		if stale {
			list[i].LastRead = at
		}
		if renamed {
			list[i].Name = name
		}
		s.touched[id] = touch{at: list[i].LastRead, name: list[i].Name}
		written = stale || renamed
		return list, written, nil
	})
	return written, err
}

// Remove takes the vault out of the list and remembers it so that Restore can
// put it back. Only the list entry goes: the vault's marks, bookmarks and
// progress are in directories this does not touch, and they are there when the
// folder is added again.
func (s *Store) Remove(id string) (Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var gone Entry
	err := s.update(func(list []Entry) ([]Entry, bool, error) {
		i := indexOf(list, id)
		if i < 0 {
			return nil, false, ErrUnknown
		}
		gone = list[i]
		return slices.Delete(list, i, i+1), true, nil
	})
	if err != nil {
		return Entry{}, err
	}
	delete(s.touched, id)
	s.remember(&gone)
	return gone, nil
}

// Restore puts back a vault this Store removed and still remembers, with the
// id and port it had. It takes an id and nothing else, and only one that
// Remove answered: a path, an id it never held or one removed by another
// process is ErrNotRemoved, so a page cannot use it to add a folder. If the
// port was given to another vault in the meantime the vault takes the next free
// one.
func (s *Store) Restore(id string) (Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	was, ok := s.removed[id]
	if !ok {
		return Entry{}, ErrNotRemoved
	}
	var back Entry
	err := s.update(func(list []Entry) ([]Entry, bool, error) {
		if i := indexOf(list, id); i >= 0 {
			back = list[i]
			return list, false, nil
		}
		if len(list) >= maxVaults {
			return nil, false, ErrFull
		}
		if slices.ContainsFunc(list, func(e Entry) bool { return e.Port == was.Port }) {
			port, portErr := s.choosePort(list, was.Port)
			if portErr != nil {
				return nil, false, portErr
			}
			was.Port = port
		}
		back = was
		return append(list, was), true, nil
	})
	if err != nil {
		return Entry{}, err
	}
	s.forget(id)
	return back, nil
}

// Anchor makes sure some vault answers on the starting port. The running
// process always holds that port, because it is how the command line tells a
// running yomihon from another program, so if no vault is on it the first vault
// in the list moves there. It answers the vault it moved, and false when none
// had to move.
func (s *Store) Anchor(start int) (moved Entry, ok bool, err error) {
	if start < 1 || start > maxPort {
		return Entry{}, false, ErrStartPort
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	err = s.update(func(list []Entry) ([]Entry, bool, error) {
		if len(list) == 0 || slices.ContainsFunc(list, func(e Entry) bool { return e.Port == start }) {
			return list, false, nil
		}
		list[0].Port = start
		moved, ok = list[0], true
		return list, true, nil
	})
	return moved, ok, err
}

// SetPort says that the vault is served on port. The process that serves a
// vault the reader removed and added again keeps its port, and the list is made
// to say so. Another vault's port is refused.
func (s *Store) SetPort(id string, port int) error {
	if port < 1 || port > maxPort {
		return ErrStartPort
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.update(func(list []Entry) ([]Entry, bool, error) {
		i := indexOf(list, id)
		if i < 0 {
			return nil, false, ErrUnknown
		}
		if list[i].Port == port {
			return list, false, nil
		}
		if slices.ContainsFunc(list, func(e Entry) bool { return e.Port == port }) {
			return nil, false, ErrPortTaken
		}
		list[i].Port = port
		return list, true, nil
	})
}

// Rebind moves a vault whose port something else has taken to the first free
// port above it, and writes the new port down. It answers the vault as it is
// now.
func (s *Store) Rebind(id string) (Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var moved Entry
	err := s.update(func(list []Entry) ([]Entry, bool, error) {
		i := indexOf(list, id)
		if i < 0 {
			return nil, false, ErrUnknown
		}
		port, portErr := s.choosePort(list, list[i].Port+1)
		if portErr != nil {
			return nil, false, portErr
		}
		list[i].Port = port
		moved = list[i]
		return list, true, nil
	})
	return moved, err
}

// choosePort is the first port from `from` up that no listed vault holds and
// that free allows.
func (s *Store) choosePort(list []Entry, from int) (int, error) {
	taken := make(map[int]bool, len(list))
	for _, e := range list {
		taken[e.Port] = true
	}
	for port := from; port <= maxPort; port++ {
		if taken[port] || (s.free != nil && !s.free(port)) {
			continue
		}
		return port, nil
	}
	return 0, ErrNoPort
}

func (s *Store) remember(e *Entry) {
	s.forget(e.ID)
	s.removed[e.ID] = *e
	s.removedOrder = append(s.removedOrder, e.ID)
	if len(s.removedOrder) > rememberRemoved {
		delete(s.removed, s.removedOrder[0])
		s.removedOrder = s.removedOrder[1:]
	}
}

func (s *Store) forget(id string) {
	if _, ok := s.removed[id]; !ok {
		return
	}
	delete(s.removed, id)
	s.removedOrder = slices.DeleteFunc(s.removedOrder, func(other string) bool { return other == id })
}

// statList is how the list looks on disk, without reading it.
func (s *Store) statList() (stamp, error) {
	size, modified, err := confdir.Stat(s.dir, listFile)
	if errors.Is(err, os.ErrNotExist) {
		return stamp{}, nil
	}
	if err != nil {
		return stamp{}, fmt.Errorf("look at the list of vaults: %w", err)
	}
	return stamp{present: true, size: size, modified: modified}, nil
}

// read is the list as it is on disk. The raw bytes come back with ErrDamaged
// when there are any, so a change can set them aside. A list that is not there
// is an empty one.
func (s *Store) read() (list []Entry, raw []byte, err error) {
	// The stamp is taken before the read, so a change between the two is seen
	// by the next Changed rather than lost.
	if s.stamp, err = s.statList(); err != nil {
		return nil, nil, err
	}
	raw, err = confdir.Read(s.dir, listFile, maxListBytes)
	switch {
	case errors.Is(err, os.ErrNotExist):
		s.stamp = stamp{}
		return nil, nil, nil
	case errors.Is(err, confdir.ErrTooLarge):
		return nil, nil, fmt.Errorf("%w: %w", ErrDamaged, err)
	case err != nil:
		return nil, nil, fmt.Errorf("read the list of vaults: %w", err)
	}
	if list, err = decode(raw); err != nil {
		return nil, raw, fmt.Errorf("%w: %w", ErrDamaged, err)
	}
	return list, raw, nil
}

// update is every change to the list: under the lock, from the list as it is
// now, so that a vault another process added a moment ago is in it. change
// answers the list to write and whether it differs; a list that cannot be read
// is set aside first and the change is made to an empty one, which keeps what
// the reader had in a file they can open and never deletes it.
func (s *Store) update(change func(list []Entry) ([]Entry, bool, error)) (err error) {
	lock, err := confdir.Acquire(s.dir, lockFile, newHolder(), abandoned, s.wait)
	if err != nil {
		return fmt.Errorf("lock the list of vaults: %w", err)
	}
	defer func() { err = errors.Join(err, lock.Release()) }()

	list, raw, err := s.read()
	if err != nil {
		if !errors.Is(err, ErrDamaged) || raw == nil {
			return err
		}
		if asideErr := confdir.Replace(s.dir, damagedFile, raw); asideErr != nil {
			return fmt.Errorf("set the damaged list of vaults aside: %w", asideErr)
		}
		list = nil
	}
	next, changed, err := change(list)
	if err != nil || !changed {
		return err
	}
	data, err := encode(next)
	if err != nil {
		return err
	}
	if err := confdir.Replace(s.dir, listFile, data); err != nil {
		return fmt.Errorf("write the list of vaults: %w", err)
	}
	// Taking the stamp of what was just written keeps this process's own change
	// from reading as someone else's.
	if stamped, statErr := s.statList(); statErr == nil {
		s.stamp = stamped
	}
	return nil
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
	return err == nil && !processAlive(n)
}
