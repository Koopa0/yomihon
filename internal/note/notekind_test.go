package note

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/ui/pages"
)

// TestTheHeadIsToldWhatANoteIsBesidesANote pins the one word the head ends on
// for a note that is more than a note. A template is a template whatever type
// it writes, because its fields are a note's to come; a report is one by where
// it stands; an index and a course are what the rules make of the type; and
// every other note is told nothing, including a type the rules do not know.
func TestTheHeadIsToldWhatANoteIsBesidesANote(t *testing.T) {
	t.Parallel()

	file := filepath.Join(t.TempDir(), "contract.toml")
	contract := `[folders]
checked = ["Notes", "Maps"]
templates = ["System/templates"]

[fields]
common = ["type"]

[types.note]

[types.topic-map]
role = "map"

[types.study-path]
role = "course"
`
	if err := os.WriteFile(file, []byte(contract), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := schema.LoadFile(file)
	if err != nil {
		t.Fatalf("schema.LoadFile: %v", err)
	}
	authority := schema.CaptureAuthority(loaded, loaded.Governance())
	roles := loaded.NavigationRoles()

	for _, tt := range []struct {
		relPath, noteType string
		want              pages.NoteKind
	}{
		{"Notes/a.md", "note", pages.KindNote},
		{"Notes/b.md", "field-note", pages.KindNote},
		{"Maps/reading.md", "topic-map", pages.KindIndex},
		{"Maps/study.md", "study-path", pages.KindCourse},
		{"System/reports/audit.md", "", pages.KindReport},
		// A file in a folder below the reports is not one, as the shelf has it.
		{"System/reports/old/audit.md", "", pages.KindNote},
		{"System/templates/map.md", "topic-map", pages.KindTemplate},
	} {
		state := governanceState{placement: classifyGovernance(authority, loaded.ArtifactPolicy(), tt.relPath)}
		if got := noteKind(&state, roles, &snapshot.Reading{RelPath: tt.relPath, Type: tt.noteType}); got != tt.want {
			t.Errorf("noteKind(%s, type %q) = %d, want %d", tt.relPath, tt.noteType, got, tt.want)
		}
	}
}
