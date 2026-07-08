package graph_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/text/unicode/norm"

	"github.com/koopa0/yomihon/internal/graph"
)

func TestResolveCaseInsensitive(t *testing.T) {
	t.Parallel()
	idx := graph.BuildFromNotes([]graph.NoteInput{{Path: "a/Go Slice.md"}}, nil)

	for _, name := range []string{"go slice", "GO SLICE", "Go Slice"} {
		got := idx.Resolve(name)
		if got.Kind != graph.Unique || got.Path != "a/Go Slice.md" {
			t.Errorf("Resolve(%q) = %+v, want Unique a/Go Slice.md", name, got)
		}
	}
}

func TestResolveNFCAndNFDAreEquivalent(t *testing.T) {
	t.Parallel()
	// The alias is stored decomposed (NFD: e + combining acute); a lookup
	// in composed form (NFC: a single é rune) must still resolve — macOS
	// itself stores filenames NFD on disk, independent of how a link was
	// typed.
	decomposed := "café" // café, NFD
	composed := "café"    // café, NFC

	idx := graph.BuildFromNotes([]graph.NoteInput{{Path: "x.md", Aliases: []string{decomposed}}}, nil)

	got := idx.Resolve(composed)
	if got.Kind != graph.Unique || got.Path != "x.md" {
		t.Errorf("Resolve(%q) = %+v, want Unique x.md", composed, got)
	}
}

// TestBuildResolvedPathIsNFCEvenWhenDiskFilenameIsNFD guards the property
// normalize()'s own doc comment claims but earlier only held for lookup
// keys, not the stored path value: a note whose filename arrived on disk
// as raw NFD bytes (macOS filesystems can hold either form regardless of
// how it was typed) must still resolve to an NFC Resolution.Path.
// normalize() alone cannot fix this — it only normalizes at lookup time —
// so this test exercises the real disk-reading path (graph.Build, which
// delegates to vault.List) rather than BuildFromNotes, to prove the fix
// lives where the bytes first enter the system.
func TestBuildResolvedPathIsNFCEvenWhenDiskFilenameIsNFD(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	const composed = "だ体.md" // だ = U+3060; decomposes to た (U+305F) + combining voiced sound mark (U+3099)
	decomposed := norm.NFD.String(composed)
	if decomposed == composed {
		t.Fatalf("test setup invalid: NFD form of %q did not change", composed)
	}
	if err := os.WriteFile(filepath.Join(root, decomposed), []byte("body\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	idx, err := graph.Build(root)
	if err != nil {
		t.Fatalf("Build(%q) = %v", root, err)
	}

	got := idx.Resolve("だ体") // an NFC-typed target, as any human keyboard/IME input would be
	if got.Kind != graph.Unique {
		t.Fatalf("Resolve(%q) = %+v, want Unique", "だ体", got)
	}
	if got.Path != composed {
		t.Errorf("Resolve(%q).Path = %q (% x), want NFC-normalized %q (% x)",
			"だ体", got.Path, []byte(got.Path), composed, []byte(composed))
	}
}

func TestResolveAliasSameAsFilename(t *testing.T) {
	t.Parallel()
	idx := graph.BuildFromNotes([]graph.NoteInput{
		{Path: "Concepts/golang/Go Slice.md", Aliases: []string{"Slice Header"}},
	}, nil)

	byFilename := idx.Resolve("Go Slice")
	byAlias := idx.Resolve("Slice Header")
	if byFilename.Kind != graph.Unique || byFilename.Path != "Concepts/golang/Go Slice.md" {
		t.Errorf("Resolve(filename) = %+v, want Unique Concepts/golang/Go Slice.md", byFilename)
	}
	if diff := cmp.Diff(byFilename, byAlias); diff != "" {
		t.Errorf("alias resolves differently than filename (-byFilename +byAlias):\n%s", diff)
	}
}

// TestTitleIsNotAResolutionKey is the single most important negative test
// in this package: a link written against a note's frontmatter title (not
// its filename or an alias) silently fails to resolve in real Obsidian,
// and the resolver must reproduce that failure mode. This goes through
// the real disk-reading path (graph.Build), not BuildFromNotes, because
// the property under test is that Build's frontmatter handling never
// promotes "title" into a key — BuildFromNotes has no title concept to
// even get wrong.
func TestTitleIsNotAResolutionKey(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, "Concepts", "golang")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := "---\ntitle: \"Go Slice 內部結構\"\naliases:\n  - Slice Header\n---\nbody\n"
	if err := os.WriteFile(filepath.Join(dir, "Go Slice.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	idx, err := graph.Build(root)
	if err != nil {
		t.Fatalf("Build(%q) = %v", root, err)
	}

	const path = "Concepts/golang/Go Slice.md"
	if got := idx.Resolve("Go Slice"); got.Kind != graph.Unique || got.Path != path {
		t.Errorf("Resolve(filename) = %+v, want Unique %s", got, path)
	}
	if got := idx.Resolve("Slice Header"); got.Kind != graph.Unique || got.Path != path {
		t.Errorf("Resolve(alias) = %+v, want Unique %s", got, path)
	}
	if got := idx.Resolve("Go Slice 內部結構"); got.Kind != graph.Unresolved {
		t.Errorf("Resolve(title) = %+v, want Unresolved — title must never be a resolution key", got)
	}
}

func TestResolveDuplicateAliasIsAmbiguous(t *testing.T) {
	t.Parallel()
	idx := graph.BuildFromNotes([]graph.NoteInput{
		{Path: "Concepts/golang/A.md", Aliases: []string{"Mechanical Sympathy"}},
		{Path: "Concepts/golang/B.md", Aliases: []string{"Mechanical Sympathy"}},
	}, nil)

	got := idx.Resolve("Mechanical Sympathy")
	want := graph.Resolution{
		Kind:       graph.Ambiguous,
		Candidates: []string{"Concepts/golang/A.md", "Concepts/golang/B.md"},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Resolve(duplicate alias) mismatch (-want +got):\n%s", diff)
	}
}

func TestResolveSameFilenameDifferentFolderIsAmbiguous(t *testing.T) {
	t.Parallel()
	idx := graph.BuildFromNotes([]graph.NoteInput{
		{Path: "golang/Foo.md"},
		{Path: "rust/Foo.md"},
	}, nil)

	got := idx.Resolve("Foo")
	if got.Kind != graph.Ambiguous {
		t.Fatalf("Resolve(Foo) = %+v, want Ambiguous", got)
	}
	want := []string{"golang/Foo.md", "rust/Foo.md"}
	if diff := cmp.Diff(want, got.Candidates); diff != "" {
		t.Errorf("Candidates mismatch (-want +got):\n%s", diff)
	}
}

func TestResolveNonMarkdownResourceNeedsExtension(t *testing.T) {
	t.Parallel()
	idx := graph.BuildFromNotes(
		[]graph.NoteInput{{Path: "Sources/DDIA.md"}},
		[]string{"Diagrams/canvas/DDIA-Ch1-Overview.canvas"},
	)

	got := idx.Resolve("DDIA-Ch1-Overview.canvas")
	if got.Kind != graph.Unique || got.Path != "Diagrams/canvas/DDIA-Ch1-Overview.canvas" {
		t.Errorf("Resolve(with extension) = %+v, want Unique Diagrams/canvas/DDIA-Ch1-Overview.canvas", got)
	}
	// Without the extension, a resource does not resolve — Obsidian
	// itself requires the extension to link a non-note file.
	if got := idx.Resolve("DDIA-Ch1-Overview"); got.Kind != graph.Unresolved {
		t.Errorf("Resolve(without extension) = %+v, want Unresolved", got)
	}
}

func TestSplitWikilink(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		inner       string
		wantTarget  string
		wantDisplay string
		wantOK      bool
	}{
		{name: "bare target", inner: "Go Slice", wantTarget: "Go Slice", wantDisplay: "Go Slice", wantOK: true},
		{name: "display text", inner: "Go Slice|see", wantTarget: "Go Slice", wantDisplay: "see", wantOK: true},
		{
			name: "heading fragment resolves on name alone", inner: "Go Slice#Internals",
			wantTarget: "Go Slice", wantDisplay: "Go Slice#Internals", wantOK: true,
		},
		{
			name: "block fragment strips correctly", inner: "Go Slice^abc",
			wantTarget: "Go Slice", wantDisplay: "Go Slice^abc", wantOK: true,
		},
		{
			name: "heading then explicit display", inner: "X#H|disp",
			wantTarget: "X", wantDisplay: "disp", wantOK: true,
		},
		{
			name: "escaped table-cell pipe is not the display separator's naive split point " +
				"(the escaped and unescaped pipe yield the same target)",
			inner: `Go Slice\|see`, wantTarget: "Go Slice", wantDisplay: "see", wantOK: true,
		},
		{
			name:  "bare anchor is a same-page jump, not a cross-file target",
			inner: "#Heading", wantTarget: "", wantDisplay: "#Heading", wantOK: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			target, display, ok := graph.SplitWikilink(tt.inner)
			if target != tt.wantTarget || display != tt.wantDisplay || ok != tt.wantOK {
				t.Errorf("SplitWikilink(%q) = (%q, %q, %v), want (%q, %q, %v)",
					tt.inner, target, display, ok, tt.wantTarget, tt.wantDisplay, tt.wantOK)
			}
		})
	}
}

func TestWikilinkFragmentsResolveOnNameAloneRegardlessOfHeadingExistence(t *testing.T) {
	t.Parallel()
	// Anchors are never verified: [[Go Slice#NoSuchHeading]] must resolve
	// exactly like [[Go Slice]] — only the target file's existence
	// matters.
	idx := graph.BuildFromNotes([]graph.NoteInput{{Path: "Go Slice.md"}}, nil)

	target, _, ok := graph.SplitWikilink("Go Slice#This Heading Does Not Exist Anywhere")
	if !ok {
		t.Fatalf("SplitWikilink() ok = false, want true")
	}
	got := idx.Resolve(target)
	if got.Kind != graph.Unique || got.Path != "Go Slice.md" {
		t.Errorf("Resolve(%q) = %+v, want Unique Go Slice.md", target, got)
	}
}
