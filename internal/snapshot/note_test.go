package snapshot

import (
	"crypto/sha256"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

func TestCaptureNoteDetachesPublishedReadingData(t *testing.T) {
	t.Parallel()

	data := []byte("---\ntype: concept\ndraft: true\nslug: original\ncreated: 2026-07-12\n---\n# Original heading\n\nbody\n")
	wantIdentity := sha256.Sum256(data)
	parsed := vault.Parse("Concepts/Original.md", data)
	got := newReading(parsed, data, schema.ArticleLanguage{})

	parsed.RelPath = "Concepts/Changed.md"
	parsed.Body = "changed"
	parsed.Frontmatter["type"] = "lesson"
	parsed.Frontmatter["created"] = "2020-01-01"
	parsed.Frontmatter["draft"] = false
	parsed.Frontmatter["slug"] = "changed"
	data[0] = 'x'

	if got.RelPath != "Concepts/Original.md" || got.Title != "Original heading" || got.Body != "# Original heading\n\nbody\n" {
		t.Fatalf("captured note identity/body changed with its inputs: %+v", got)
	}
	if got.Type != "concept" || !got.Draft || got.Slug != "original" || !got.Created.Equal(time.Date(2026, time.July, 12, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("captured note metadata changed with its inputs: %+v", got)
	}
	if !got.HasFrontmatter || got.Language != "" || got.LanguageDiagnostic != "" {
		t.Fatalf("captured note authority = frontmatter %t language %q diagnostic %q", got.HasFrontmatter, got.Language, got.LanguageDiagnostic)
	}
	if got.ContentIdentity != wantIdentity {
		t.Errorf("ContentIdentity = %x, want %x", got.ContentIdentity, wantIdentity)
	}
}

func TestCaptureNoteRetainsFrontmatterDiagnosticWithoutAuthority(t *testing.T) {
	t.Parallel()

	data := []byte("---\ntype: [broken\n---\nbody\n")
	got := newReading(vault.Parse("Broken.md", data), data, schema.ArticleLanguage{})
	if !got.HasFrontmatter {
		t.Fatal("malformed frontmatter dropped the block that produced the diagnostic")
	}
	if got.FMDiagnostic == "" {
		t.Fatal("malformed frontmatter diagnostic was dropped")
	}
}

// TestCaptureNoteKeepsEmptyBlockApartFromAbsent holds the projection to the
// split's own answer: an empty fence pair is a present block, and a file with
// no delimiters is not. Deriving the flag from a nil field map collapses them.
func TestCaptureNoteKeepsEmptyBlockApartFromAbsent(t *testing.T) {
	t.Parallel()

	empty := []byte("---\n---\nbody\n")
	if got := newReading(vault.Parse("Empty.md", empty), empty, schema.ArticleLanguage{}); !got.HasFrontmatter {
		t.Fatal("empty frontmatter block was projected as absent")
	}

	absent := []byte("body\n")
	if got := newReading(vault.Parse("Absent.md", absent), absent, schema.ArticleLanguage{}); got.HasFrontmatter {
		t.Fatal("a file with no delimiters was projected as a frontmatter block")
	}
}

// TestCaptureNoteKeepsAFenceNothingClosedApartFromAbsent holds the second
// projection of a file the split read no block from. A note whose closing fence
// lost a character and a note that never wrote frontmatter both have no block,
// and a page that cannot tell them apart calls the first one legal. Neither is a
// frontmatter block, and only the first is a fence nothing closed.
func TestCaptureNoteKeepsAFenceNothingClosedApartFromAbsent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		content      string
		wantBlock    bool
		wantUnclosed bool
	}{
		{name: "the closing fence lost a dash", content: "---\ntype: note\n--\n\n# Body\n", wantUnclosed: true},
		{name: "a block that closes", content: "---\n---\nbody\n", wantBlock: true},
		{name: "an empty fence pair", content: "---\n---\nbody\n", wantBlock: true},
		{name: "a thematic break and prose", content: "---\n\nbody\n"},
		{name: "no delimiters", content: "body\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data := []byte(tt.content)
			got := newReading(vault.Parse("Note.md", data), data, schema.ArticleLanguage{})
			if got.HasFrontmatter != tt.wantBlock || got.FrontmatterUnclosed != tt.wantUnclosed {
				t.Errorf("newReading(%q) = block %v, unclosed %v; want block %v, unclosed %v",
					tt.content, got.HasFrontmatter, got.FrontmatterUnclosed, tt.wantBlock, tt.wantUnclosed)
			}
		})
	}
}

// TestAReadingIsTitledByItsFirstHeadingElseItsFile pins where a reading's title
// comes from: the first level-one heading the body opens on, and the file name
// when it has none. A title the frontmatter writes is no field and names
// nothing.
func TestAReadingIsTitledByItsFirstHeadingElseItsFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		rel  string
		data string
		want string
	}{
		{name: "the first heading", rel: "Notes/file name.md", data: "# The heading\n\nbody\n", want: "The heading"},
		{name: "a frontmatter title is not read", rel: "Notes/file name.md", data: "---\ntitle: Written title\n---\n# The heading\n", want: "The heading"},
		{name: "no heading is the file name", rel: "Notes/file name.md", data: "---\ntitle: Written title\n---\n## Lesser\n\nbody\n", want: "file name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data := []byte(tt.data)
			got := newReading(vault.Parse(tt.rel, data), data, schema.ArticleLanguage{})
			if got.Title != tt.want {
				t.Errorf("newReading(%q).Title = %q, want %q", tt.rel, got.Title, tt.want)
			}
		})
	}
}

// TestAReadingCarriesItsDeclaredCreationDate pins which date a reading keeps:
// the one created declares. An update date is no field, and a note that
// declares no creation date carries the zero time.
func TestAReadingCarriesItsDeclaredCreationDate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data string
		want time.Time
	}{
		{name: "created is read", data: "---\ncreated: 2026-07-12\nupdated: 2026-08-01\n---\nbody\n", want: time.Date(2026, time.July, 12, 0, 0, 0, 0, time.UTC)},
		{name: "an update date alone is no date", data: "---\nupdated: 2026-08-01\n---\nbody\n"},
		{name: "no frontmatter is no date", data: "body\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data := []byte(tt.data)
			got := newReading(vault.Parse("Notes/n.md", data), data, schema.ArticleLanguage{})
			if !got.Created.Equal(tt.want) {
				t.Errorf("newReading().Created = %v, want %v", got.Created, tt.want)
			}
		})
	}
}

// TestDeclaredTitleIsTheHeadingAlone pins that a generation's declared title is
// the note's own first heading and nothing else: a note with no heading
// declares none, even though its reading is still called by its file.
func TestDeclaredTitleIsTheHeadingAlone(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeNote(t, root, "Notes/headed.md", "# A heading\n\nbody\n")
	writeNote(t, root, "Notes/plain.md", "---\ntitle: Written title\n---\nbody\n")
	store, _ := newTestStore(t, root, testContract(t, root))
	gen := store.Current()

	if got := gen.DeclaredTitle("Notes/headed.md"); got != "A heading" {
		t.Errorf("DeclaredTitle(headed) = %q, want %q", got, "A heading")
	}
	if got := gen.DeclaredTitle("Notes/plain.md"); got != "" {
		t.Errorf("DeclaredTitle(plain) = %q, want empty: no heading, no declared title", got)
	}
	if reading, ok := gen.Note("Notes/plain.md"); !ok || reading.Title != "plain" {
		t.Errorf("Note(plain).Title = %q, want the file name", reading.Title)
	}
}

// TestTitlesAreKeyedByTheDerivedTitle pins the key the title table files a
// note under: its first heading, else its file name, and never a title its
// frontmatter writes.
func TestTitlesAreKeyedByTheDerivedTitle(t *testing.T) {
	t.Parallel()

	notes := []*vault.Note{
		parse(t, "A/headed.md", "# Shared Name\n\nbody\n"),
		parse(t, "B/Shared Name.md", "body\n"),
		parse(t, "C/written.md", "---\ntitle: Shared Name\n---\nbody\n"),
	}
	got := titlesByName(notes)
	want := []nav.NoteRef{
		{Name: "headed", RelPath: "A/headed.md"},
		{Name: "Shared Name", RelPath: "B/Shared Name.md"},
	}
	if diff := cmp.Diff(want, got[graph.NormalizeKey("Shared Name")]); diff != "" {
		t.Errorf("titlesByName()[shared name] mismatch (-want +got):\n%s", diff)
	}
	if _, ok := got[graph.NormalizeKey("written")]; !ok {
		t.Error("titlesByName() has no entry for a note with no heading under its file name")
	}
}
