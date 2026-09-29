package snapshot

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/nav"
)

func TestDeclaredByPreservesEachSourcesAuthoredLocations(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeNote(t, root, "Sources/Book.md", "---\naliases: [Notebook]\nbased_on: '[[Book#Self]]'\n---\n# Chapter one\n")
	writeNote(t, root, "Sources/Other.md", "other\n")
	writeNote(t, root, "Writing/Zebra.md", "---\nbased_on: '[[Book#Missing section]]'\n---\nmy thought\n")
	writeNote(t, root, "Writing/Apple.md", `---
based_on:
  - '[[Book#Chapter one|display text]]'
  - '[[Other#Not this source]]'
  - '[[Notebook#Chapter one|another label]]'
  - Book
  - '[[Book#^quote-1]]'
  - '[[Sources/Book.md^quote-1]]'
  - '[[Notebook#Chapter two]]'
  - '[[Book]]'
---
my thought
`)
	store, _ := newTestStore(t, root, testContract(t, root))
	got := store.Current().DeclaredBy("Sources/Book.md")
	want := []DeclaringNote{
		{
			Note: nav.NoteRef{Name: "Apple", RelPath: "Writing/Apple.md"},
			Locations: []DeclaredPlace{
				{Heading: "Chapter one"},
				{},
				{Block: "quote-1"},
				{Heading: "Chapter two"},
			},
		},
		{
			Note:      nav.NoteRef{Name: "Zebra", RelPath: "Writing/Zebra.md"},
			Locations: []DeclaredPlace{{Heading: "Missing section"}},
		},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("DeclaredBy(Book) mismatch (-want +got):\n%s", diff)
	}
	// The reverse place projection does not redefine file-only comparisons.
	if diff := cmp.Diff([]nav.NoteRef{want[0].Note, want[1].Note}, store.Current().BasedOnBy("Sources/Book.md")); diff != "" {
		t.Errorf("BasedOnBy(Book) changed (-want +got):\n%s", diff)
	}
}

func TestDeclaredByDeclinesAmbiguousMissingAndSelfTargets(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeNote(t, root, "A/twin.md", "one\n")
	writeNote(t, root, "B/twin.md", "two\n")
	writeNote(t, root, "Writing/Thought.md", `---
based_on:
  - '[[twin#section]]'
  - '[[missing#section]]'
  - '[[Thought#self]]'
  - '[[#section]]'
---
[[A/twin]]
`)
	store, _ := newTestStore(t, root, testContract(t, root))
	for _, rel := range []string{"A/twin.md", "B/twin.md", "Writing/Thought.md", "missing.md"} {
		if got := store.Current().DeclaredBy(rel); got != nil {
			t.Errorf("DeclaredBy(%q) = %+v, want no declaration", rel, got)
		}
	}
	if got := (*Generation)(nil).DeclaredBy("A/twin.md"); got != nil {
		t.Errorf("nil DeclaredBy() = %+v", got)
	}
}

func TestDeclaredByReturnsDetachedLocationsFromCapturedGeneration(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeNote(t, root, "Source.md", "source\n")
	writeNote(t, root, "Thought.md", "---\nbased_on: '[[Source#Earlier]]'\n---\nthought\n")
	store, _ := newTestStore(t, root, testContract(t, root))
	earlier := store.Current()
	wantEarlier := []DeclaringNote{{
		Note:      nav.NoteRef{Name: "Thought", RelPath: "Thought.md"},
		Locations: []DeclaredPlace{{Heading: "Earlier"}},
	}}
	returned := earlier.DeclaredBy("Source.md")
	if diff := cmp.Diff(wantEarlier, returned); diff != "" {
		t.Fatalf("initial DeclaredBy() mismatch (-want +got):\n%s", diff)
	}
	returned[0].Note.Name = "changed by caller"
	returned[0].Locations[0].Heading = "changed by caller"
	writeNote(t, root, "Thought.md", "---\nbased_on: '[[Source#Later and different]]'\n---\nnew thought\n")
	store.rescan(t.Context())
	if diff := cmp.Diff(wantEarlier, earlier.DeclaredBy("Source.md")); diff != "" {
		t.Errorf("retained generation changed (-want +got):\n%s", diff)
	}
	wantLater := []DeclaringNote{{
		Note:      nav.NoteRef{Name: "Thought", RelPath: "Thought.md"},
		Locations: []DeclaredPlace{{Heading: "Later and different"}},
	}}
	if diff := cmp.Diff(wantLater, store.Current().DeclaredBy("Source.md")); diff != "" {
		t.Errorf("new generation did not capture new place (-want +got):\n%s", diff)
	}
}
