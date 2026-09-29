package note

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/wording"
)

func TestDeclaredByKeepsNotesAndSourceLocationsSeparate(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeNote(t, root, "Sources/Book.md", "---\ntitle: Book\n---\n# Book\n\n## First & second\n\nThe quotation. ^Q&\"\n\n## Later\n")
	writeNote(t, root, "Thoughts/Apple.md", `---
based_on:
  - '[[Book#First & second]]'
  - '[[Book#Later]]'
  - '[[Book#^Q&"]]'
  - '[[Book]]'
  - '[[Book#Book]]'
  - '[[Book#Missing]]'
  - '[[Book#^missing]]'
---
My thought.
`)
	writeNote(t, root, "Thoughts/Zebra.md", "---\nbased_on: '[[Book#Later]]'\n---\nAnother thought.\n")
	snap := plainVaultView(t, root)
	source, ok := snap.Note("Sources/Book.md")
	if !ok {
		t.Fatal("source absent from captured generation")
	}
	for _, prefix := range []string{"", "right-"} {
		for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
			t.Run(prefix+string(lang), func(t *testing.T) {
				t.Parallel()
				result := snap.Render(source.RelPath, source.Body, lang)
				render.Qualify(prefix, &result)
				got := declaredBy(snap, source.RelPath, &result, prefix, lang)
				want := []pages.DeclaringNoteView{
					{
						Note: nav.NoteRef{Name: "Apple", RelPath: "Thoughts/Apple.md"},
						Locations: []pages.DeclaredPlaceView{
							{Label: "#First & second", Href: "#" + prefix + "first-second"},
							{Label: "#Later", Href: "#" + prefix + "later"},
							{Label: "#^Q&\"", Href: "#" + prefix + "%5Eq&%22"},
							{Label: wording.WholeSource.In(lang), Href: "#" + prefix + "book"},
							{Label: "#Book", Href: "#" + prefix + "book"},
							{Label: "#Missing"},
							{Label: "#^missing"},
						},
					},
					{
						Note:      nav.NoteRef{Name: "Zebra", RelPath: "Thoughts/Zebra.md"},
						Locations: []pages.DeclaredPlaceView{{Label: "#Later", Href: "#" + prefix + "later"}},
					},
				}
				if diff := cmp.Diff(want, got); diff != "" {
					t.Errorf("declaredBy() mismatch (-want +got):\n%s", diff)
				}
			})
		}
	}
}

func TestDeclaredByUsesOnlyPlacesInTheSuppliedRender(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeNote(t, root, "Source.md", "# Existing\n\nActual block. ^existing\n")
	writeNote(t, root, "Thought.md", "---\nbased_on: ['[[Source#Existing]]', '[[Source#^existing]]', '[[Source]]']\n---\nThought.\n")
	snap := plainVaultView(t, root)
	// The generation has both locations, but this render does not. Reading or
	// rendering the captured body again would invent addresses on this page.
	result := render.Result{HTML: `<div id="existing">unrelated</div><span id="^existing">not a marker</span>`}
	got := declaredBy(snap, "Source.md", &result, "", wording.En)
	want := []pages.DeclaringNoteView{{
		Note: nav.NoteRef{Name: "Thought", RelPath: "Thought.md"},
		Locations: []pages.DeclaredPlaceView{
			{Label: "#Existing"},
			{Label: "#^existing"},
			{Label: wording.WholeSource.In(wording.En), Href: "/notes/Source.md"},
		},
	}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("declaredBy() used places outside this render (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(want, declaredBy(snap, "Source.md", nil, "", wording.En)); diff != "" {
		t.Errorf("declaredBy(nil render) mismatch (-want +got):\n%s", diff)
	}
	if got := declaredBy(nil, "Source.md", &result, "", wording.En); got != nil {
		t.Errorf("declaredBy(nil generation) = %+v, want nil", got)
	}
}

func TestDeclaredByRejectsCodeAndAuthoredHTMLBlockAnchors(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeNote(t, root, "Source.md", "````html\n<span id=\"^code\">^code</span>\n## Fenced\n````\n\n<span id=\"^authored\">^authored</span>\n\n`inline ^inline`\n")
	writeNote(t, root, "Thought.md", "---\nbased_on: ['[[Source#^code]]', '[[Source#^authored]]', '[[Source#^inline]]', '[[Source#Fenced]]']\n---\nThought.\n")
	snap := plainVaultView(t, root)
	source, ok := snap.Note("Source.md")
	if !ok {
		t.Fatal("source absent from captured generation")
	}
	result := snap.Render(source.RelPath, source.Body, wording.En)
	got := declaredBy(snap, source.RelPath, &result, "", wording.En)
	want := []pages.DeclaringNoteView{{
		Note: nav.NoteRef{Name: "Thought", RelPath: "Thought.md"},
		Locations: []pages.DeclaredPlaceView{
			{Label: "#^code"},
			{Label: "#^authored"},
			{Label: "#^inline"},
			{Label: "#Fenced"},
		},
	}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("declaredBy() trusted an unrendered anchor (-want +got):\n%s", diff)
	}
}
