// Package vaults is the reader's list of the vaults they have opened: which
// folders, what each is called, when each was last read, and the loopback port
// each is served on. It is a record about the reader, not about any vault,
// which is why it lives under the configuration directory and never in a vault.
//
// It holds the list's shape and rules. Every read and write of its files goes
// through internal/mark/confdir, which is where the promise that yomihon writes
// only under the configuration directory is kept.
//
// A folder is added only by the command line, by the folder's own path. What a
// page may ask of the list is an id, and only for the two things that act on an
// entry already in it.
package vaults

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/koopa0/yomihon/internal/mark/confdir"
)

// ErrDamaged means the list exists and cannot be read as a list at all: it is
// not the file this writes, or it is too long, or it is not a file. Nothing it
// holds is honoured, and the next change sets it aside rather than deleting it.
// An entry that is wrong on its own is not this: it is left out and the rest are
// read.
var ErrDamaged = errors.New("the list of vaults cannot be read")

// ErrUnknown means no vault in the list has the id.
var ErrUnknown = errors.New("no vault in the list has that id")

// ErrNotRemoved means this process did not remove a vault with that id, or has
// forgotten it. A page asking to bring a vault back is never taken at its word.
var ErrNotRemoved = errors.New("no vault removed here is remembered under that id")

// MinPort is the lowest port a vault is given: the ports below it are the
// system's, which a reader cannot bind without more than they were given.
const MinPort = 1024

// ErrStartPort means the starting port is not one a vault can be given.
var ErrStartPort = errors.New("the starting port must be an integer from 1024 to 65535")

// ErrNoPort means every port from the starting one up is taken.
var ErrNoPort = errors.New("no free port from the starting port up")

// ErrPortTaken means another vault in the list is served on that port.
var ErrPortTaken = errors.New("another vault is on that port")

// ErrFull means the list holds as many vaults as it will.
var ErrFull = errors.New("the list of vaults is full")

const (
	// version is the only shape of the file this reads back.
	version = 1

	listFile     = "vaults.json"
	damagedFile  = "vaults.damaged.json"
	lockFile     = "vaults.lock"
	instanceFile = "instance.json"

	// maxListBytes bounds what a read takes in. An entry is a path, a name, a
	// time and two numbers; a file this long holds far more vaults than a
	// reader has, and a longer one was not written here.
	// keptAside is how many lists that could not be used are kept side by side.
	// The first is never replaced, so what the reader first lost stays; when all
	// are taken the last one is.
	keptAside = 8

	maxListBytes = 1 << 20
	maxVaults    = 4096
	maxPathBytes = 4096
	maxNameBytes = 256
	maxPort      = 65535

	// touchEvery is how often a vault's last-read time is written. The list
	// shows a date, and every page load would otherwise be a write.
	touchEvery = time.Minute

	// rememberRemoved is how many removed vaults can be brought back. It is the
	// undo window's room, not a history.
	rememberRemoved = 32
)

// Entry is one vault the reader has opened.
type Entry struct {
	// ID names the vault to a page. It is the name of the vault's directory of
	// marks, so a vault added again after it was removed finds its marks where
	// it left them, and it spells no path.
	ID string `json:"id"`
	// Path is the vault's folder as vault.ResolveRoot spells it. It is the
	// vault's identity: another path is another vault.
	Path string `json:"path"`
	// Name is the vault as it was last called, kept so that a vault whose folder
	// cannot be found still has one.
	Name string `json:"name"`
	// LastRead is when the reader last loaded a page of the vault, written at
	// most once a minute.
	LastRead time.Time `json:"last_read"`
	// Port is the loopback port the vault is served on. It stays the vault's
	// across restarts, because the browser's tabs and history follow the address.
	Port int `json:"port"`
}

// valid says whether e is something this list can hold. The id is checked
// against the path rather than trusted, so a hand-edited file cannot give a
// vault another vault's marks.
func (e *Entry) valid() error {
	switch {
	case e.Path == "" || !filepath.IsAbs(e.Path) || filepath.Clean(e.Path) != e.Path:
		return fmt.Errorf("the path %q is not a clean absolute path", e.Path)
	case filepath.Dir(e.Path) == e.Path:
		return fmt.Errorf("the path %q is the top of a filesystem, which is not a vault", e.Path)
	case len(e.Path) > maxPathBytes || !utf8.ValidString(e.Path):
		return fmt.Errorf("the path of %q is not one this list holds", e.ID)
	case e.ID != confdir.VaultID(e.Path):
		return fmt.Errorf("the id %q is not the id of %q", e.ID, e.Path)
	case e.Name == "" || cleanName(e.Name, "") != e.Name:
		return fmt.Errorf("the name %q of %q is not one this list holds", e.Name, e.ID)
	case e.Port < MinPort || e.Port > maxPort:
		return fmt.Errorf("the port %d of %q is not a port", e.Port, e.ID)
	}
	return nil
}

// cleanName is a name as the list keeps it: one line, single spaces, valid
// text, not longer than the list holds. An empty name is the folder's own name
// when the path is given and empty when it is not, which is how a caller tells
// that there was nothing to keep.
func cleanName(name, path string) string {
	name = strings.ToValidUTF8(name, "�")
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, name)
	name = strings.Join(strings.Fields(name), " ")
	if name == "" && path != "" {
		name = filepath.Base(path)
	}
	for len(name) > maxNameBytes {
		_, size := utf8.DecodeLastRuneInString(name)
		name = name[:len(name)-size]
	}
	return name
}

// decode reads a list, refusing anything it would not have written. A file that
// is not a list is an error. An entry that is not one this list holds is left
// out and named in skipped, and the rest are kept, because the list is the port
// of every vault in it: dropping the whole list for one bad entry would move
// every other vault to another address.
func decode(raw []byte) (list []Entry, skipped []string, err error) {
	var file struct {
		Version int               `json:"version"`
		Vaults  []json.RawMessage `json:"vaults"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if decodeErr := dec.Decode(&file); decodeErr != nil {
		return nil, nil, decodeErr
	}
	if _, tokenErr := dec.Token(); !errors.Is(tokenErr, io.EOF) {
		return nil, nil, errors.New("there is more after the list")
	}
	if file.Version != version {
		return nil, nil, fmt.Errorf("version %d is not the version %d this reads", file.Version, version)
	}
	if len(file.Vaults) > maxVaults {
		return nil, nil, fmt.Errorf("%d vaults is more than the %d this holds", len(file.Vaults), maxVaults)
	}
	ids, ports := map[string]bool{}, map[int]bool{}
	for i, one := range file.Vaults {
		e, entryErr := decodeEntry(one)
		if entryErr == nil && (ids[e.ID] || ports[e.Port]) {
			entryErr = fmt.Errorf("the id or the port of %q is another vault's", e.ID)
		}
		if entryErr != nil {
			skipped = append(skipped, fmt.Sprintf("entry %d: %v", i+1, entryErr))
			continue
		}
		ids[e.ID], ports[e.Port] = true, true
		list = append(list, e)
	}
	return list, skipped, nil
}

// decodeEntry reads one entry and checks it.
func decodeEntry(raw json.RawMessage) (Entry, error) {
	var e Entry
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&e); err != nil {
		return Entry{}, err
	}
	if err := e.valid(); err != nil {
		return Entry{}, err
	}
	return e, nil
}

// encode is what a list is written as.
func encode(list []Entry) ([]byte, error) {
	if list == nil {
		list = []Entry{}
	}
	data, err := json.MarshalIndent(struct {
		Version int     `json:"version"`
		Vaults  []Entry `json:"vaults"`
	}{version, list}, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode the list of vaults: %w", err)
	}
	return append(data, '\n'), nil
}

// recentFirst orders a list by when each vault was last read, latest first, and
// keeps the order it was written in among vaults read at the same moment.
func recentFirst(list []Entry) []Entry {
	ordered := slices.Clone(list)
	slices.SortStableFunc(ordered, func(a, b Entry) int { return cmp.Compare(b.LastRead.UnixNano(), a.LastRead.UnixNano()) })
	return ordered
}

func indexOf(list []Entry, id string) int {
	return slices.IndexFunc(list, func(e Entry) bool { return e.ID == id })
}
