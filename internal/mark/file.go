package mark

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// File is the one file yomihon writes outside the vault: the marks kept for
// one vault, under the configuration directory of the person running it. It
// owns that path and the discipline of replacing the file whole.
//
// It holds no mark in memory. The file on disk is the record, so a reader who
// deletes it has deleted it, rather than arguing with a copy this process is
// still holding.
type File struct {
	// dir is this vault's own directory, created on the first write. A reader
	// who never sets a mark leaves nothing behind.
	dir string
	// path is the file itself.
	path string
	// vaultRoot is written inside the file, because the directory holding it
	// is named after a digest and a person who has to find the right one needs
	// something to read.
	vaultRoot string
	// mu serializes writers. Two marks set at once would otherwise race
	// between their temporary files and the rename that installs one, and the
	// loser's bytes could be the ones left under the name.
	mu sync.Mutex
}

// document is the file's own shape. Version comes first so a reader that does
// not recognise it stops at the first field.
type document struct {
	Version      int           `json:"version"`
	Vault        string        `json:"vault"`
	Continuation *continuation `json:"continuation,omitempty"`
}

// continuation is one mark as it sits in the file. The time is written in RFC
// 3339 by the encoder, which is the shape a person reading the file expects.
type continuation struct {
	Path     string    `json:"path"`
	Anchor   string    `json:"anchor,omitempty"`
	Offset   int       `json:"offset"`
	Identity string    `json:"identity"`
	At       time.Time `json:"at"`
}

// New names this vault's marks file. It creates nothing: the directory and the
// file appear when a reader first keeps a place.
//
// configDir is the platform's configuration directory, resolved once by the
// command that composes this — reading the environment is the process's own
// business, and a package handed a directory cannot widen the surface the
// command promises. vaultRoot is the resolved absolute root being served,
// which names which vault these marks belong to.
//
// Either one empty is refused rather than defaulted. A path built from an
// empty configuration directory is a relative one, which would put the
// reader's marks beside whatever folder the process happened to start in and
// look like it had worked.
func New(configDir, vaultRoot string) (*File, error) {
	if configDir == "" || vaultRoot == "" {
		return nil, fmt.Errorf("%w: configuration directory %q, vault root %q",
			ErrNotNamed, configDir, vaultRoot)
	}
	dir := Directory(configDir, vaultRoot)
	return &File{dir: dir, path: filepath.Join(dir, fileName), vaultRoot: vaultRoot}, nil
}

// Path is the file this keeps its marks in. It is what the interface names
// when it tells a reader where their marks live.
func (f *File) Path() string {
	return f.path
}

// Continuation reports the mark this file holds.
//
// A file that is not there, cannot be read, or carries a shape this version
// does not recognise answers false. A mark is a convenience the reader set in
// one press, and the honest answer to a damaged one is the same as to a
// missing one: there is nowhere to continue from, and the next mark replaces
// it. Nothing here repairs a file, and nothing reports one to the reader,
// because the reader did not ask this question — a page did, on their behalf.
func (f *File) Continuation() (Continuation, bool) {
	data, err := os.ReadFile(f.path)
	if err != nil {
		return Continuation{}, false
	}
	var doc document
	if json.Unmarshal(data, &doc) != nil || doc.Version != documentVersion || doc.Continuation == nil {
		return Continuation{}, false
	}
	held := Continuation{
		RelPath:  doc.Continuation.Path,
		Anchor:   doc.Continuation.Anchor,
		Offset:   doc.Continuation.Offset,
		Identity: doc.Continuation.Identity,
		At:       doc.Continuation.At,
	}
	// Held to the same bounds a submission is. The file is the reader's own and
	// they may edit it, and a value that arrived by hand reaches the same page
	// that a posted one does.
	if validate(&held) != nil {
		return Continuation{}, false
	}
	return held, true
}

// SetContinuation replaces this file's mark with c, or refuses it unchanged.
//
// The replacement is written beside the file and renamed over it, so a reader
// whose machine stops mid-write finds either the old mark or the new one and
// never half of either. It is not synchronized to the platter: the guarantee
// the status write buys there is for an author's own bytes surviving a power
// cut, and what is lost here is one mark a reader sets again by pressing the
// same control.
func (f *File) SetContinuation(c Continuation) error {
	if err := validate(&c); err != nil {
		return err
	}
	doc := document{
		Version: documentVersion,
		Vault:   f.vaultRoot,
		Continuation: &continuation{
			Path:     c.RelPath,
			Anchor:   c.Anchor,
			Offset:   c.Offset,
			Identity: c.Identity,
			At:       c.At,
		},
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("encode the marks file: %w", err)
	}
	data = append(data, '\n')

	f.mu.Lock()
	defer f.mu.Unlock()
	// Owner-only, like everything else this writes: what a reader is partway
	// through is theirs, and the directory is created here rather than at
	// startup so a reader who sets no mark leaves no directory behind.
	if err = os.MkdirAll(f.dir, 0o700); err != nil {
		return fmt.Errorf("create the marks directory: %w", err)
	}
	return f.replace(data)
}

// replace installs data under the file's name through a temporary sibling and
// one rename. The sibling is created in the same directory, so the rename
// stays inside one filesystem and is the atomic operation it looks like.
func (f *File) replace(data []byte) error {
	tmp, err := os.CreateTemp(f.dir, ".reader-*.json")
	if err != nil {
		return fmt.Errorf("create the temporary marks file: %w", err)
	}
	name := tmp.Name()
	if _, err = tmp.Write(data); err != nil {
		return abandon(tmp, name, fmt.Errorf("write the temporary marks file: %w", err))
	}
	if err = tmp.Close(); err != nil {
		return abandon(nil, name, fmt.Errorf("close the temporary marks file: %w", err))
	}
	if err = os.Rename(name, f.path); err != nil {
		return abandon(nil, name, fmt.Errorf("install the marks file: %w", err))
	}
	return nil
}

// abandon removes a temporary file whose bytes will not be installed and
// returns the failure that stopped it. The removal is best-effort: the cause
// is what the caller has to act on, and a leftover file under a dot name is
// not something to replace that cause with.
func abandon(open *os.File, name string, cause error) error {
	if open != nil {
		_ = open.Close() //nolint:errcheck // the cause is the actionable failure
	}
	if err := os.Remove(name); err != nil && !errors.Is(err, os.ErrNotExist) {
		return errors.Join(cause, fmt.Errorf("remove the temporary marks file: %w", err))
	}
	return cause
}
