package mark

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
	"unicode/utf8"

	"github.com/koopa0/yomihon/internal/vault"
)

// ErrUnreadableUncertainties means an existing uncertainty file cannot be
// read safely. A toggle leaves that file untouched so the reader can recover it.
var ErrUnreadableUncertainties = errors.New("the uncertainty marks cannot be read")

// MaxUncertainties bounds how many places are kept at once. A reader marks a
// handful of spots to revisit; the bound keeps an unbounded stream of requests
// from growing a file that every later request reads whole.
const MaxUncertainties = 500

// Uncertainty is a place the reader wants to revisit without claiming an
// answer. Its path and anchor identify it; it carries no source-content copy.
type Uncertainty struct {
	RelPath string    `json:"path"`
	Anchor  string    `json:"anchor"`
	At      time.Time `json:"time"`
}

// UncertaintyPath names a separate file. Older versions replacing reader.json
// cannot erase these accumulated marks.
func (f *File) UncertaintyPath() string {
	return filepath.Join(f.dir, "uncertainty.json")
}

// Uncertainties returns marks in the order they were added. Missing storage is
// an empty list; an unreadable or malformed file is an error, not an empty list.
// Each call reads the file again so deleting it clears the marks.
func (f *File) Uncertainties() ([]Uncertainty, error) {
	data, err := os.ReadFile(f.UncertaintyPath())
	if errors.Is(err, os.ErrNotExist) {
		return []Uncertainty{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnreadableUncertainties, err)
	}
	var held []Uncertainty
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&held); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnreadableUncertainties, err)
	}
	// Valid checks the entire document, including trailing bytes. A null value
	// is not the array written here and must not silently become an empty file.
	if !json.Valid(data) || held == nil {
		return nil, fmt.Errorf("%w: expected an array of marks", ErrUnreadableUncertainties)
	}
	seen := make(map[[2]string]bool, len(held))
	for i := range held {
		if err = validateUncertainty(&held[i]); err != nil {
			// The stored entry's fault stays text: wrapping it would let the
			// route read a corrupt file as a refused request.
			return nil, fmt.Errorf("%w: %s", ErrUnreadableUncertainties, err.Error())
		}
		key := [2]string{held[i].RelPath, held[i].Anchor}
		if seen[key] {
			return nil, fmt.Errorf("%w: duplicate path and anchor", ErrUnreadableUncertainties)
		}
		seen[key] = true
	}
	return held, nil
}

// ToggleUncertainty adds a new path and anchor, or removes the existing pair.
// The result says whether the pair is now kept. Removing is always allowed;
// adding needs admit to accept the pair (checked while the file is locked, only
// when the pair is new) and room under MaxUncertainties. A refusal wraps
// ErrInvalid. Validation and read failures leave the existing file unchanged,
// including a corrupt file's original bytes.
func (f *File) ToggleUncertainty(kept *Uncertainty, admit func(*Uncertainty) bool) (bool, error) {
	if err := validateUncertainty(kept); err != nil {
		return false, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	held, err := f.Uncertainties()
	if err != nil {
		return false, err
	}
	added := true
	for i := range held {
		if held[i].RelPath == kept.RelPath && held[i].Anchor == kept.Anchor {
			held = append(held[:i], held[i+1:]...)
			added = false
			break
		}
	}
	if added {
		if admit == nil || !admit(kept) {
			return false, fmt.Errorf("%w: the place is not one this vault renders", ErrInvalid)
		}
		if len(held) >= MaxUncertainties {
			return false, fmt.Errorf("%w: %d places are already kept", ErrInvalid, len(held))
		}
		held = append(held, *kept)
	}
	data, err := json.MarshalIndent(held, "", "  ")
	if err != nil {
		return false, fmt.Errorf("encode the uncertainty marks: %w", err)
	}
	if err = os.MkdirAll(f.dir, 0o700); err != nil {
		return false, fmt.Errorf("create the marks directory: %w", err)
	}
	err = f.replaceUncertainties(append(data, '\n'))
	if err != nil {
		return false, err
	}
	return added, nil
}

func validateUncertainty(kept *Uncertainty) error {
	switch {
	case kept == nil:
		return fmt.Errorf("%w: no uncertainty mark", ErrInvalid)
	case !utf8.ValidString(kept.RelPath):
		return fmt.Errorf("%w: the path or anchor is not valid UTF-8", ErrInvalid)
	case kept.RelPath == "" || len(kept.RelPath) > maxRelPathBytes:
		return fmt.Errorf("%w: the note path is empty or too long", ErrInvalid)
	case !fs.ValidPath(kept.RelPath) || kept.RelPath == ".":
		return fmt.Errorf("%w: the note path is not vault-relative", ErrInvalid)
	case kept.RelPath != vault.NormalizeNFC(kept.RelPath):
		return fmt.Errorf("%w: the note path is not written in NFC", ErrInvalid)
	}
	if err := ValidateAnchor(kept.Anchor); err != nil {
		return err
	}
	if kept.At.IsZero() {
		return fmt.Errorf("%w: the mark has no time", ErrInvalid)
	}
	return nil
}

// replaceUncertainties installs a complete array by rename, never partially
// rewriting the existing file. Like continuation marks, these are not synced
// to disk against a power loss; the reader can set a lost mark again.
func (f *File) replaceUncertainties(data []byte) error {
	tmp, err := os.CreateTemp(f.dir, ".uncertainty-*.json")
	if err != nil {
		return fmt.Errorf("create the temporary uncertainty file: %w", err)
	}
	name := tmp.Name()
	if _, err = tmp.Write(data); err != nil {
		return abandon(tmp, name, fmt.Errorf("write the temporary uncertainty file: %w", err))
	}
	if err = tmp.Close(); err != nil {
		return abandon(nil, name, fmt.Errorf("close the temporary uncertainty file: %w", err))
	}
	if err = os.Rename(name, f.UncertaintyPath()); err != nil {
		return abandon(nil, name, fmt.Errorf("install the uncertainty file: %w", err))
	}
	return nil
}
