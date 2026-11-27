package mark

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/koopa0/yomihon/internal/mark/machinemarks"
)

// The files this version writes. Each is one JSON document with its version
// first, so a reader that does not recognise it stops at the first field, and
// the vault's absolute path second, because the directory holding it is named
// after a digest and a person who has to find the right one needs something to
// read.
const (
	marksFile    = "marks.json"
	progressFile = "progress.json"
	lockFile     = "marks.lock"

	// documentVersion is the shape both files are written in and the only one
	// read back. A file carrying any other number is kept as it is and reported
	// as unreadable; it is never read as empty and never replaced.
	documentVersion = 1
)

// The most a read takes in, which is also the most a write installs: a file
// this program writes is always one it can read back.
const (
	maxMarksFileBytes    = 16 << 20
	maxProgressFileBytes = 4 << 20
)

// marksDocument is marks.json, which this version only reads.
type marksDocument struct {
	Version int          `json:"version"`
	Vault   string       `json:"vault"`
	Marks   []legacyMark `json:"marks"`
}

// legacyMark is a mark as marks.json holds it. A question's words are its
// reason there, and a thought's its body; the machine files have one field for
// the reader's words in every state.
type legacyMark struct {
	ID         string            `json:"id"`
	Kind       machinemarks.Kind `json:"kind"`
	RelPath    string            `json:"path"`
	Anchor     string            `json:"anchor,omitempty"`
	Quote      string            `json:"quote,omitempty"`
	Before     string            `json:"before,omitempty"`
	After      string            `json:"after,omitempty"`
	Identity   string            `json:"identity,omitempty"`
	Reason     string            `json:"reason,omitempty"`
	Body       string            `json:"body,omitempty"`
	ResolvedAt time.Time         `json:"resolved,omitzero"`
	At         time.Time         `json:"at"`
}

// machine is m as a line of the machine files holds it, or why it cannot be one
// without losing a word: the earlier version wrote a reason only on a question
// and a body only on another kind, so a mark that has the other is not one it
// wrote.
func (m *legacyMark) machine() (machinemarks.Mark, error) {
	words := m.Body
	switch {
	case m.Kind == machinemarks.Question && m.Body != "":
		return machinemarks.Mark{}, fmt.Errorf("the question %s has a body", m.ID)
	case m.Kind == machinemarks.Question:
		words = m.Reason
	case m.Reason != "":
		return machinemarks.Mark{}, fmt.Errorf("the %s %s has a reason", m.Kind, m.ID)
	}
	return machinemarks.Mark{
		ID: m.ID, Kind: m.Kind, Path: m.RelPath, Anchor: m.Anchor,
		Quote: m.Quote, Before: m.Before, After: m.After, Identity: m.Identity,
		Body: words, Resolved: m.ResolvedAt, At: m.At,
	}, nil
}

// progressDocument is progress.json.
type progressDocument struct {
	Version   int              `json:"version"`
	Vault     string           `json:"vault"`
	Bookmarks []courseBookmark `json:"bookmarks"`
	// Finished is left out while nothing is finished, so a file that holds
	// bookmarks alone is the file an earlier version wrote.
	Finished []Finished `json:"finished,omitempty"`
}

// progress is what progress.json holds, read whole and written whole, and the
// vault it records, which is empty for a file an earlier version wrote.
type progress struct {
	bookmarks []courseBookmark
	finished  []Finished
	vault     string
}

// courseBookmark is one bookmark as it sits in progress.json, with the course
// it belongs to. An empty course is the bookmark for reading outside any
// course.
type courseBookmark struct {
	Course   string   `json:"course,omitempty"`
	Bookmark Bookmark `json:"bookmark"`
}

// decodeMarks reads a marks.json as the marks of the machine files. A file that
// is not one the earlier version wrote is ErrUnreadable: another version, an
// unknown field, two marks under one id, more marks than are kept, or a mark
// with a word the machine files have no place for. A mark is not held to its
// shape here: that is machinemarks' Validate, which the take-in asks, and a mark
// it refuses stays in the file and holds nothing else back.
func decodeMarks(data []byte) (marks []machinemarks.Mark, recorded string, err error) {
	var doc marksDocument
	if decodeErr := decodeStrict(data, &doc); decodeErr != nil {
		return nil, "", decodeErr
	}
	if doc.Version != documentVersion || doc.Marks == nil {
		return nil, "", fmt.Errorf("%w: not a version %d marks file", ErrUnreadable, documentVersion)
	}
	if len(doc.Marks) > machinemarks.MaxLive {
		return nil, "", fmt.Errorf("%w: more than %d marks", ErrUnreadable, machinemarks.MaxLive)
	}
	out := make([]machinemarks.Mark, len(doc.Marks))
	seen := make(map[string]bool, len(doc.Marks))
	for i := range doc.Marks {
		m, convErr := doc.Marks[i].machine()
		if convErr != nil {
			return nil, "", fmt.Errorf("%w: %w", ErrUnreadable, convErr)
		}
		if seen[m.ID] {
			return nil, "", fmt.Errorf("%w: two marks carry the id %s", ErrUnreadable, m.ID)
		}
		seen[m.ID] = true
		out[i] = m
	}
	return out, doc.Vault, nil
}

// decodeProgress reads a progress.json and holds every bookmark and every
// finished note in it to the shape a write would have produced.
func decodeProgress(data []byte) (progress, error) {
	var doc progressDocument
	if err := decodeStrict(data, &doc); err != nil {
		return progress{}, err
	}
	if doc.Version != documentVersion || doc.Bookmarks == nil {
		return progress{}, fmt.Errorf("%w: not a version %d progress file", ErrUnreadable, documentVersion)
	}
	if err := checkBookmarks(doc.Bookmarks); err != nil {
		return progress{}, err
	}
	if err := checkFinished(doc.Finished); err != nil {
		return progress{}, err
	}
	return progress{bookmarks: doc.Bookmarks, finished: doc.Finished, vault: doc.Vault}, nil
}

func checkFinished(held []Finished) error {
	if len(held) > maxFinished {
		return fmt.Errorf("%w: more than %d finished notes", ErrUnreadable, maxFinished)
	}
	seen := make(map[string]bool, len(held))
	for i := range held {
		if err := validateFinished(&held[i]); err != nil {
			return fmt.Errorf("%w: %v", ErrUnreadable, err) //nolint:errorlint // ErrInvalid stays unwrapped: a damaged file is not a refused request
		}
		if seen[held[i].RelPath] {
			return fmt.Errorf("%w: one note finished twice", ErrUnreadable)
		}
		seen[held[i].RelPath] = true
	}
	return nil
}

func checkBookmarks(held []courseBookmark) error {
	if len(held) > maxBookmarks {
		return fmt.Errorf("%w: more than %d bookmarks", ErrUnreadable, maxBookmarks)
	}
	seen := make(map[string]bool, len(held))
	for i := range held {
		if err := validateBookmark(held[i].Course, &held[i].Bookmark); err != nil {
			return fmt.Errorf("%w: %v", ErrUnreadable, err) //nolint:errorlint // ErrInvalid stays unwrapped: a damaged file is not a refused request
		}
		if seen[held[i].Course] {
			return fmt.Errorf("%w: two bookmarks for one course", ErrUnreadable)
		}
		seen[held[i].Course] = true
	}
	return nil
}

// decodeStrict decodes one JSON value into v, refusing an unknown field and
// anything after the value. Both are signs of a file this program did not
// write.
func decodeStrict(data []byte, v any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(v); err != nil {
		return fmt.Errorf("%w: %w", ErrUnreadable, err)
	}
	if !json.Valid(data) {
		return fmt.Errorf("%w: more than one value", ErrUnreadable)
	}
	return nil
}

// encode writes a document the way a person opening the file expects to find
// it, refusing one the next read would refuse for its length.
func encode(doc any, limit int) ([]byte, error) {
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode the marks: %w", err)
	}
	data = append(data, '\n')
	if len(data) > limit {
		return nil, fmt.Errorf("%w: more than this file holds", ErrInvalid)
	}
	return data, nil
}
