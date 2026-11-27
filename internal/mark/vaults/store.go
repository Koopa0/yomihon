package vaults

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/koopa0/yomihon/internal/mark/confdir"
	"github.com/koopa0/yomihon/internal/vault"
)

// Store is the list of vaults and the file that says a yomihon is running, both
// in the reader's own directory. One value serves one process: the command line
// and the running server are two processes with two Stores over the same files,
// and the lock file is what orders their changes.
type Store struct {
	// configDir is what a folder added to the list must be apart from.
	configDir confdir.ConfigDir
	dir       confdir.Dir
	free      func(port int) bool
	now       func() time.Time
	wait      time.Duration

	// own is the ports this process holds itself. They are free for the vault
	// this process is about to give one to, which a probe of them cannot say,
	// because the probe finds the process in the way.
	own map[int]bool

	// say is told, in a line, what was wrong with the list and what was done
	// about it. Nil says nothing.
	say func(line string)

	mu           sync.Mutex
	stamp        stamp
	told         stamp
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
func Open(configDir confdir.ConfigDir, free func(port int) bool, options ...Option) (*Store, error) {
	dir, err := confdir.Home(configDir)
	if err != nil {
		return nil, err
	}
	s := &Store{
		configDir: configDir,
		dir:       dir,
		free:      free,
		now:       time.Now,
		wait:      confdir.LockWait,
		touched:   map[string]touch{},
		removed:   map[string]Entry{},
		own:       map[int]bool{},
	}
	for _, option := range options {
		option(s)
	}
	return s, nil
}

// Option is a way a Store is set up beyond where it is kept.
type Option func(*Store)

// Report sends each line the Store has to say about a list it could not use
// whole to say: which entries were left out and why, and where a list it
// replaced is kept. The Store prints nothing itself.
func Report(say func(line string)) Option {
	return func(s *Store) { s.say = say }
}

// Hold says that this process has bound port, so that a new vault can be given
// it. The running yomihon holds the starting port before it records the folder
// it was given, which is how a second yomihon finds it, and the folder is then
// the first vault of that port rather than the next.
func (s *Store) Hold(port int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.own[port] = true
}

// InstanceFile is where the record that a yomihon is running is kept, for a
// message that tells a reader which file to remove.
func (s *Store) InstanceFile() string { return s.dir.Join(instanceFile) }

// List is every vault, the one read most recently first. A list that cannot be
// read answers ErrDamaged and no vaults; the next change saves it as damaged. An
// entry that cannot be used is left out of the answer, and said once for each
// state of the file.
func (s *Store) List() ([]Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, _, skipped, err := s.read()
	if err != nil {
		return nil, err
	}
	s.sayLeftOut(skipped)
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
// down: it is the vault's from then on, and no later start moves it. A vault
// already listed keeps its port. Adding is the command line's: it is given a
// path because the reader typed one.
func (s *Store) Add(root string, start int) (entry Entry, added bool, err error) {
	if start < MinPort || start > maxPort {
		return Entry{}, false, ErrStartPort
	}
	_, resolved, err := vault.ResolveRoot(root)
	if err != nil {
		return Entry{}, false, err
	}
	// A folder joins the list only when it is known to be apart from the
	// configuration directory: one that overlaps it is not one yomihon reads,
	// and one that cannot be compared with it is not added on the chance. One
	// listed before this was refused stays listed, and is not served.
	if notApart := s.configDir.Classify(root); notApart != nil {
		return Entry{}, false, notApart
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

// SetPort says that the vault is served on port. The process that serves a
// vault the reader removed and added again keeps its port, and the list is made
// to say so. Another vault's port is refused.
func (s *Store) SetPort(id string, port int) error {
	if port < MinPort || port > maxPort {
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

// choosePort is the first port from `from` up that no listed vault holds and
// that free allows.
func (s *Store) choosePort(list []Entry, from int) (int, error) {
	taken := make(map[int]bool, len(list))
	for _, e := range list {
		taken[e.Port] = true
	}
	for port := from; port <= maxPort; port++ {
		if taken[port] || (!s.own[port] && s.free != nil && !s.free(port)) {
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
	info, err := confdir.Stat(s.dir, listFile)
	if errors.Is(err, os.ErrNotExist) {
		return stamp{}, nil
	}
	if err != nil {
		return stamp{}, fmt.Errorf("look at the list of vaults: %w", err)
	}
	return stamp{present: true, size: info.Size(), modified: info.ModTime()}, nil
}

// read is the list as it is on disk. The raw bytes come back with the list
// whenever there are any, and with ErrDamaged, so a change can save them;
// skipped names the entries that were left out. A list that is not there is an
// empty one.
func (s *Store) read() (list []Entry, raw []byte, skipped []string, err error) {
	// The stamp is taken before the read, so a change between the two is seen
	// by the next Changed rather than lost.
	if s.stamp, err = s.statList(); err != nil {
		return nil, nil, nil, err
	}
	raw, err = confdir.Read(s.dir, listFile, maxListBytes)
	switch {
	case errors.Is(err, os.ErrNotExist):
		s.stamp = stamp{}
		return nil, nil, nil, nil
	case errors.Is(err, confdir.ErrTooLarge), errors.Is(err, confdir.ErrNotRegular):
		return nil, nil, nil, fmt.Errorf("%w: %w", ErrDamaged, err)
	case err != nil:
		return nil, nil, nil, fmt.Errorf("read the list of vaults: %w", err)
	}
	if list, skipped, err = decode(raw); err != nil {
		return nil, raw, nil, fmt.Errorf("%w: %w", ErrDamaged, err)
	}
	return list, raw, skipped, nil
}

// update is every change to the list: under the lock, from the list as it is
// now, so that a vault another process added a moment ago is in it. change
// answers the list to write and whether it differs; a list that cannot be read
// is saved as damaged first and the change is made to an empty one, which keeps what
// the reader had in a file they can open and never deletes it. The store's
// methods take no context, so the wait for the lock is bounded by s.wait alone.
func (s *Store) update(change func(list []Entry) ([]Entry, bool, error)) error {
	return confdir.Exclusive(context.Background(), s.dir, lockFile, s.wait, func() error { return s.updateHeld(change) })
}

// updateHeld is update with the lock already taken.
func (s *Store) updateHeld(change func(list []Entry) ([]Entry, bool, error)) error {
	list, raw, skipped, err := s.read()
	damaged := errors.Is(err, ErrDamaged)
	if err != nil && !damaged {
		return err
	}
	if damaged {
		list = nil
	}
	next, changed, err := change(list)
	if err != nil {
		return err
	}
	if !changed {
		s.sayLeftOut(skipped)
		return nil
	}
	data, err := encode(next)
	if err != nil {
		return err
	}
	// The list is about to be replaced, so what it was is kept first: a list that
	// could not be read, or one with entries that could not be used.
	if damaged || len(skipped) > 0 {
		if err := s.saveDamaged(raw, damaged, skipped); err != nil {
			return err
		}
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

// saveDamaged keeps the list as it is before it is replaced, for the reader to
// open: the bytes it had, or, when it was never read (raw is nil), the file
// itself, renamed. It says what was wrong and where the list is kept.
func (s *Store) saveDamaged(raw []byte, damaged bool, skipped []string) error {
	name, err := s.freeDamagedName()
	if err != nil {
		return fmt.Errorf("save the damaged list of vaults: %w", err)
	}
	if raw == nil {
		err = confdir.Rename(s.dir, listFile, name)
	} else {
		err = confdir.Replace(s.dir, name, raw)
	}
	if err != nil {
		return fmt.Errorf("save the damaged list of vaults: %w", err)
	}
	if s.say != nil {
		where := s.dir.Join(name)
		if damaged {
			s.say("the list of vaults could not be read and is kept as " + where + "; a new list was started")
		} else {
			s.say(describeLeftOut(skipped) + "; the list as it was is kept as " + where)
		}
	}
	return nil
}

// freeDamagedName is the first name for a damaged list that is not taken. The
// first name is never given again, so an earlier list is not written over; when
// every name is taken, the last is.
func (s *Store) freeDamagedName() (string, error) {
	name := damagedName(damagedCopies)
	for n := 1; n <= damagedCopies; n++ {
		_, err := confdir.Stat(s.dir, damagedName(n))
		if errors.Is(err, os.ErrNotExist) {
			return damagedName(n), nil
		}
		if err != nil {
			return "", err
		}
	}
	return name, nil
}

// damagedName is the nth name a damaged list is kept under.
func damagedName(n int) string {
	if n <= 1 {
		return damagedFile
	}
	return strings.TrimSuffix(damagedFile, ".json") + "-" + strconv.Itoa(n) + ".json"
}

// sayLeftOut says what was left out of the list, once for each state the file is
// in, so that a poll of it does not say it again.
func (s *Store) sayLeftOut(skipped []string) {
	if len(skipped) == 0 || s.say == nil || s.told.present && s.told.equal(s.stamp) {
		return
	}
	s.told = s.stamp
	s.say(describeLeftOut(skipped))
}

// describeLeftOut is a line naming the entries that could not be used.
func describeLeftOut(skipped []string) string {
	const shown = 3
	line := "the list of vaults has " + strconv.Itoa(len(skipped)) + " entries that could not be used and are left out"
	if len(skipped) == 1 {
		line = "the list of vaults has 1 entry that could not be used and is left out"
	}
	if len(skipped) > shown {
		return line + " (" + strings.Join(skipped[:shown], "; ") + "; and " + strconv.Itoa(len(skipped)-shown) + " more)"
	}
	return line + " (" + strings.Join(skipped, "; ") + ")"
}
