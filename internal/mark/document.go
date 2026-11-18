package mark

import (
	"bytes"
	"encoding/json"
	"fmt"
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

// marksDocument is marks.json.
type marksDocument struct {
	Version int    `json:"version"`
	Vault   string `json:"vault"`
	Marks   []Mark `json:"marks"`
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

// progress is what progress.json holds, read whole and written whole.
type progress struct {
	bookmarks []courseBookmark
	finished  []Finished
}

// courseBookmark is one bookmark as it sits in progress.json, with the course
// it belongs to. An empty course is the bookmark for reading outside any
// course.
type courseBookmark struct {
	Course   string   `json:"course,omitempty"`
	Bookmark Bookmark `json:"bookmark"`
}

// decodeMarks reads a marks.json and holds every mark in it to the shape a
// write would have produced. Anything else is ErrUnreadable.
func decodeMarks(data []byte) ([]Mark, error) {
	var doc marksDocument
	if err := decodeStrict(data, &doc); err != nil {
		return nil, err
	}
	if doc.Version != documentVersion || doc.Marks == nil {
		return nil, fmt.Errorf("%w: not a version %d marks file", ErrUnreadable, documentVersion)
	}
	if len(doc.Marks) > MaxMarks {
		return nil, fmt.Errorf("%w: more than %d marks", ErrUnreadable, MaxMarks)
	}
	if err := checkMarks(doc.Marks); err != nil {
		return nil, err
	}
	return doc.Marks, nil
}

// checkMarks holds stored marks to the bounds a submission is. The file is the
// reader's own and they may edit it, and a value that arrived by hand reaches
// the same page that a posted one does. The fault stays text: wrapping
// ErrInvalid would let a route read a corrupt file as a refused request.
func checkMarks(marks []Mark) error {
	seen := make(map[string]bool, len(marks))
	for i := range marks {
		if err := validateMark(&marks[i]); err != nil {
			return fmt.Errorf("%w: %s", ErrUnreadable, err.Error())
		}
		if seen[marks[i].ID] {
			return fmt.Errorf("%w: two marks carry the id %s", ErrUnreadable, marks[i].ID)
		}
		seen[marks[i].ID] = true
	}
	return nil
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
	return progress{bookmarks: doc.Bookmarks, finished: doc.Finished}, nil
}

func checkFinished(held []Finished) error {
	if len(held) > maxFinished {
		return fmt.Errorf("%w: more than %d finished notes", ErrUnreadable, maxFinished)
	}
	seen := make(map[string]bool, len(held))
	for i := range held {
		if err := validateFinished(&held[i]); err != nil {
			return fmt.Errorf("%w: %s", ErrUnreadable, err.Error())
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
			return fmt.Errorf("%w: %s", ErrUnreadable, err.Error())
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
