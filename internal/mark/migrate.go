package mark

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/koopa0/yomihon/internal/mark/confdir"
)

// The files an earlier version wrote. They are read when the file that replaces
// them is not there yet, and they are never written, renamed or removed: a
// reader who goes back to the earlier version finds them as they left them, and
// the new file existing is what says they have been converted. A mark the
// reader deletes after that stays deleted, because the old file is no longer
// asked.
//
// This is the only place their names are spelled.
const (
	legacyBookmarkFile = "reader.json"
	legacyQuestionFile = "uncertainty.json"

	// legacyBookmarkVersion is the one shape reader.json was written in.
	legacyBookmarkVersion = 1

	maxLegacyBookmarkBytes = 64 << 10
	// maxLegacyQuestionBytes is room for the 500 entries the earlier version
	// kept at most, at the longest path and anchor it admitted.
	maxLegacyQuestionBytes = 500 * 8 << 10
)

// legacyQuestions reads uncertainty.json as questions. stored says whether the
// file is there at all. Each entry becomes a question about the whole section
// its anchor names, with no reason and open, and an id derived from what it
// says so that converting the same file twice names the same marks.
func legacyQuestions(dir confdir.Dir) (marks []Mark, stored bool, err error) {
	data, err := confdir.Read(dir, legacyQuestionFile, maxLegacyQuestionBytes)
	if errors.Is(err, os.ErrNotExist) {
		return []Mark{}, false, nil
	}
	if err != nil {
		return nil, true, fmt.Errorf("%w: %s: %w", ErrUnreadable, legacyQuestionFile, err)
	}
	var entries []struct {
		RelPath string    `json:"path"`
		Anchor  string    `json:"anchor"`
		At      time.Time `json:"time"`
	}
	if err = decodeStrict(data, &entries); err != nil {
		return nil, true, fmt.Errorf("%s: %w", legacyQuestionFile, err)
	}
	if entries == nil {
		return nil, true, fmt.Errorf("%w: %s: expected an array of marks", ErrUnreadable, legacyQuestionFile)
	}
	if len(entries) > MaxMarks {
		return nil, true, fmt.Errorf("%w: %s: more than %d marks", ErrUnreadable, legacyQuestionFile, MaxMarks)
	}
	marks = make([]Mark, len(entries))
	for i, entry := range entries {
		marks[i] = Mark{
			ID:      legacyID(entry.RelPath, entry.Anchor),
			Kind:    Question,
			RelPath: entry.RelPath,
			Anchor:  entry.Anchor,
			At:      entry.At,
		}
	}
	if err = checkMarks(marks); err != nil {
		return nil, true, fmt.Errorf("%s: %w", legacyQuestionFile, err)
	}
	return marks, true, nil
}

// legacyID names a converted question by the place it marked. The earlier
// version kept one question for each note and anchor, so that pair is unique
// and the same in every conversion.
func legacyID(relPath, anchor string) string {
	sum := sha256.Sum256([]byte("question\x00" + relPath + "\x00" + anchor))
	return hex.EncodeToString(sum[:16])
}

// legacyBookmark reads reader.json as the bookmark for reading outside any
// course. found is false when the file is not there or holds none. A file in
// another version, or damaged, is ErrUnreadable and is left as it is: it is the
// reader's only record of where they were, and nothing here can say what it
// meant.
func legacyBookmark(dir confdir.Dir) (held Bookmark, found bool, err error) {
	data, err := confdir.Read(dir, legacyBookmarkFile, maxLegacyBookmarkBytes)
	if errors.Is(err, os.ErrNotExist) {
		return Bookmark{}, false, nil
	}
	if err != nil {
		return Bookmark{}, false, fmt.Errorf("%w: %s: %w", ErrUnreadable, legacyBookmarkFile, err)
	}
	var doc struct {
		Version      int       `json:"version"`
		Continuation *Bookmark `json:"continuation"`
	}
	if err = json.Unmarshal(data, &doc); err != nil {
		return Bookmark{}, false, fmt.Errorf("%w: %s: %w", ErrUnreadable, legacyBookmarkFile, err)
	}
	if doc.Version != legacyBookmarkVersion {
		return Bookmark{}, false, fmt.Errorf("%w: %s is version %d, and only version %d is converted",
			ErrUnreadable, legacyBookmarkFile, doc.Version, legacyBookmarkVersion)
	}
	if doc.Continuation == nil {
		return Bookmark{}, false, nil
	}
	if err = validateBookmark(NoCourse, doc.Continuation); err != nil {
		return Bookmark{}, false, fmt.Errorf("%w: %s: %s", ErrUnreadable, legacyBookmarkFile, err.Error())
	}
	return *doc.Continuation, true, nil
}
