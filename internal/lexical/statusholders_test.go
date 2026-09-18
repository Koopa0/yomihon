package lexical

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
)

// statusHolderFixture is one index holding the three shapes the walk has to
// tell apart: a note carrying a status, a note carrying none, and a file, which
// has no frontmatter to carry one in.
func statusHolderFixture() *Index {
	return NewIndex([]Document{
		{RelPath: "Concepts/Alpha.md", Title: "Alpha", NoteType: "concept", Status: "draft"},
		{RelPath: "Concepts/Bravo.md", Title: "Bravo", NoteType: "concept"},
		{RelPath: "Writing/Charlie.md", Title: "Charlie", NoteType: "lesson", Status: "ready"},
		{RelPath: "Attachments/Delta.png", Title: "Delta", Status: "ready", File: true},
	}, schema.ArtifactPolicy{})
}

// TestTheStatusWalkAndTheStatusListAgree holds the two forms to each other. The
// walk exists because the whole-folder gathering gave up materialising a slice
// it only counts, and the list is still what the callers who want one ask for —
// so the list is built from the walk, and this says the two never drift apart
// in what they hold or the order they hold it in. It also pins what neither
// form reports: a note declaring no status, and a file, which has no
// frontmatter to declare one in.
func TestTheStatusWalkAndTheStatusListAgree(t *testing.T) {
	t.Parallel()

	idx := statusHolderFixture()
	list, err := idx.StatusHolders()
	if err != nil {
		t.Fatalf("StatusHolders() error = %v, want the holders", err)
	}
	walk, err := idx.EachStatusHolder()
	if err != nil {
		t.Fatalf("EachStatusHolder() error = %v, want the walk", err)
	}
	walked := make([]StatusHolder, 0, len(list))
	for holder := range walk {
		walked = append(walked, holder)
	}
	if diff := cmp.Diff(list, walked); diff != "" {
		t.Errorf("the walk and the list disagree (-list +walk):\n%s", diff)
	}
	want := []StatusHolder{
		{RelPath: "Concepts/Alpha.md", Type: "concept", Status: "draft"},
		{RelPath: "Writing/Charlie.md", Type: "lesson", Status: "ready"},
	}
	if diff := cmp.Diff(want, walked); diff != "" {
		t.Errorf("the walk reports the wrong holders (-want +got):\n%s", diff)
	}
}

// TestTheStatusWalkStopsWhenTheReaderStops holds the one branch a caller that
// runs to the end never reaches. A reader that has seen enough breaks out, and
// the walk has to stop there rather than carry on calling a reader that has
// gone: the gathering that counts every holder would never notice, and the next
// caller to want only the first one would walk the whole folder to get it.
func TestTheStatusWalkStopsWhenTheReaderStops(t *testing.T) {
	t.Parallel()

	walk, err := statusHolderFixture().EachStatusHolder()
	if err != nil {
		t.Fatalf("EachStatusHolder() error = %v, want the walk", err)
	}
	visited := 0
	for range walk {
		visited++
		break
	}
	if visited != 1 {
		t.Errorf("a reader that stopped after one holder saw %d, want 1", visited)
	}
}

// TestTheStatusWalkRefusesWhatTheListRefuses holds the refusal to the same
// line. A folder whose contract could not be read cannot say which files are
// artifacts, so no metadata answer it gives is worth anything — and the walk
// has to refuse for the reason the list refuses, not hand back an empty walk
// that reads as a folder where nobody set a status.
func TestTheStatusWalkRefusesWhatTheListRefuses(t *testing.T) {
	t.Parallel()

	policy := (*schema.Contract)(nil).Capabilities(schema.Unreadable(errors.New("toml: line 42: expected a key separator"))).Artifacts
	idx := NewIndex([]Document{
		{RelPath: "Concepts/Alpha.md", Title: "Alpha", NoteType: "concept", Status: "draft"},
	}, policy)

	walk, walkErr := idx.EachStatusHolder()
	if !errors.Is(walkErr, ErrMetadataUnavailable) {
		t.Errorf("EachStatusHolder() error = %v, want ErrMetadataUnavailable", walkErr)
	}
	if walk != nil {
		t.Error("the walk was refused and handed back anyway; a caller ranging over it reads an empty folder")
	}
	_, listErr := idx.StatusHolders()
	if !errors.Is(listErr, ErrMetadataUnavailable) {
		t.Errorf("StatusHolders() error = %v, want ErrMetadataUnavailable", listErr)
	}
}
