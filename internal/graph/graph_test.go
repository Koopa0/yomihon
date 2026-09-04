package graph_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/text/unicode/norm"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/vaultfs"
)

func capturedGraph(t *testing.T, root string) *graph.Index {
	t.Helper()
	reader, err := vaultfs.Open(root)
	if err != nil {
		t.Fatalf("vaultfs.Open() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Errorf("Reader.Close() error = %v", closeErr)
		}
	})
	scan, err := reader.ScanComplete(t.Context())
	if err != nil {
		t.Fatalf("ScanComplete() error = %v", err)
	}
	notes := make([]*vault.Note, 0, len(scan.Files()))
	resources := make([]string, 0, len(scan.Files()))
	for _, entry := range scan.Files() {
		if !strings.HasSuffix(entry.Path(), ".md") {
			resources = append(resources, entry.Path())
			continue
		}
		data, readErr := reader.ReadFile(t.Context(), entry)
		if readErr != nil {
			t.Fatalf("ReadFile() error = %v", readErr)
		}
		notes = append(notes, vault.Parse(entry.Path(), data))
	}
	return graph.New(notes, resources)
}

func TestResolveCaseInsensitive(t *testing.T) {
	t.Parallel()
	idx := graph.BuildFromNotes([]graph.NoteInput{{RelPath: "a/Go Slice.md"}}, nil)

	for _, name := range []string{"go slice", "GO SLICE", "Go Slice"} {
		got := idx.Resolve(name)
		if got.Kind != graph.KindUnique || got.RelPath != "a/Go Slice.md" {
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

	idx := graph.BuildFromNotes([]graph.NoteInput{{RelPath: "x.md", Aliases: []string{decomposed}}}, nil)

	got := idx.Resolve(composed)
	if got.Kind != graph.KindUnique || got.RelPath != "x.md" {
		t.Errorf("Resolve(%q) = %+v, want Unique x.md", composed, got)
	}
}

// TestCapturedGraphResolvedPathIsNFCEvenWhenDiskFilenameIsNFD guards the property
// normalize()'s own doc comment claims but earlier only held for lookup
// keys, not the stored path value: a note whose filename arrived on disk
// as raw NFD bytes (macOS filesystems can hold either form regardless of
// how it was typed) must still resolve to an NFC Resolution.RelPath.
// normalize() alone cannot fix this because it only normalizes at lookup time;
// this test exercises the rooted reader capture where path bytes enter the
// generation.
func TestCapturedGraphResolvedPathIsNFCEvenWhenDiskFilenameIsNFD(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	const composed = "だ体.md" // だ = U+3060; decomposes to た (U+305F) + combining voiced sound mark (U+3099)
	decomposed := norm.NFD.String(composed)
	if decomposed == composed {
		t.Fatalf("test setup invalid: NFD form of %q did not change", composed)
	}
	if err := os.WriteFile(filepath.Join(root, decomposed), []byte("body\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	idx := capturedGraph(t, root)

	got := idx.Resolve("だ体") // an NFC-typed target, as any human keyboard/IME input would be
	if got.Kind != graph.KindUnique {
		t.Fatalf("Resolve(%q) = %+v, want Unique", "だ体", got)
	}
	if got.RelPath != composed {
		t.Errorf("Resolve(%q).RelPath = %q (% x), want NFC-normalized %q (% x)",
			"だ体", got.RelPath, []byte(got.RelPath), composed, []byte(composed))
	}
}

func TestNewOwnsAliasExtractionFromParsedNotes(t *testing.T) {
	t.Parallel()

	idx := graph.New([]*vault.Note{
		{
			RelPath: "Concepts/Go Slice.md",
			Frontmatter: map[string]any{
				"title":   "A Title Is Not A Resolution Key",
				"aliases": []any{"slice header"},
			},
		},
	}, []string{"Diagrams/overview.svg"})

	for _, target := range []string{"Go Slice", "slice header", "overview.svg"} {
		if got := idx.Resolve(target); got.Kind != graph.KindUnique {
			t.Errorf("Resolve(%q).Kind = %v, want Unique", target, got.Kind)
		}
	}
	if got := idx.Resolve("A Title Is Not A Resolution Key"); got.Kind != graph.KindUnresolved {
		t.Errorf("frontmatter title resolved as a key: %+v", got)
	}
}

// TestAliasesContributeOnlyFromAStringList pins the tolerant reading of the
// frontmatter aliases field: a plain list is the one shape that contributes
// resolution keys, and inside it the string members. Any other shape — a bare
// scalar, a mapping — silently costs the note its alias keys and nothing else;
// the note's own filename keys survive whatever the field holds.
func TestAliasesContributeOnlyFromAStringList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		aliases    any
		resolved   []string
		unresolved []string
	}{
		{
			name:       "a bare scalar contributes nothing",
			aliases:    "solo name",
			unresolved: []string{"solo name"},
		},
		{
			name:       "a mapping contributes nothing",
			aliases:    map[string]any{"alias": "mapped name"},
			unresolved: []string{"alias", "mapped name"},
		},
		{
			name:       "string members of a mixed list contribute and the rest are skipped",
			aliases:    []any{"kept name", 42, true, []any{"nested name"}},
			resolved:   []string{"kept name"},
			unresolved: []string{"42", "true", "nested name"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			idx := graph.New([]*vault.Note{{
				RelPath:     "Concepts/Aliased.md",
				Frontmatter: map[string]any{"aliases": tt.aliases},
			}}, nil)

			if got := idx.Resolve("Aliased"); got.Kind != graph.KindUnique {
				t.Fatalf("Resolve(filename) = %+v, want Unique — a malformed aliases field costs alias keys, never the note's own keys", got)
			}
			for _, name := range tt.resolved {
				if got := idx.Resolve(name); got.Kind != graph.KindUnique {
					t.Errorf("Resolve(%q) = %+v, want Unique", name, got)
				}
			}
			for _, name := range tt.unresolved {
				if got := idx.Resolve(name); got.Kind != graph.KindUnresolved {
					t.Errorf("Resolve(%q) = %+v, want Unresolved", name, got)
				}
			}
		})
	}
}

func TestResolveAliasSameAsFilename(t *testing.T) {
	t.Parallel()
	idx := graph.BuildFromNotes([]graph.NoteInput{
		{RelPath: "Concepts/golang/Go Slice.md", Aliases: []string{"Slice Header"}},
	}, nil)

	byFilename := idx.Resolve("Go Slice")
	byAlias := idx.Resolve("Slice Header")
	if byFilename.Kind != graph.KindUnique || byFilename.RelPath != "Concepts/golang/Go Slice.md" {
		t.Errorf("Resolve(filename) = %+v, want Unique Concepts/golang/Go Slice.md", byFilename)
	}
	if diff := cmp.Diff(byFilename, byAlias); diff != "" {
		t.Errorf("alias resolves differently than filename (-byFilename +byAlias):\n%s", diff)
	}
}

// TestCapturedGraphTitleIsNotAResolutionKey is the single most important negative test
// in this package: a link written against a note's frontmatter title (not
// its filename or an alias) silently fails to resolve in real Obsidian,
// and the resolver must reproduce that failure mode. This goes through a
// rooted capture and graph.New because the property under test is that parsed
// frontmatter never promotes title into a key.
func TestCapturedGraphTitleIsNotAResolutionKey(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, "Concepts", "golang")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := "---\ntitle: \"Go Slice 內部結構\"\naliases:\n  - Slice Header\n---\nbody\n"
	if err := os.WriteFile(filepath.Join(dir, "Go Slice.md"), []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	idx := capturedGraph(t, root)

	const path = "Concepts/golang/Go Slice.md"
	if got := idx.Resolve("Go Slice"); got.Kind != graph.KindUnique || got.RelPath != path {
		t.Errorf("Resolve(filename) = %+v, want Unique %s", got, path)
	}
	if got := idx.Resolve("Slice Header"); got.Kind != graph.KindUnique || got.RelPath != path {
		t.Errorf("Resolve(alias) = %+v, want Unique %s", got, path)
	}
	if got := idx.Resolve("Go Slice 內部結構"); got.Kind != graph.KindUnresolved {
		t.Errorf("Resolve(title) = %+v, want Unresolved — title must never be a resolution key", got)
	}
}

func TestResolveDuplicateAliasIsAmbiguous(t *testing.T) {
	t.Parallel()
	idx := graph.BuildFromNotes([]graph.NoteInput{
		{RelPath: "Concepts/golang/A.md", Aliases: []string{"Mechanical Sympathy"}},
		{RelPath: "Concepts/golang/B.md", Aliases: []string{"Mechanical Sympathy"}},
	}, nil)

	got := idx.Resolve("Mechanical Sympathy")
	want := graph.Resolution{
		Kind:       graph.KindAmbiguous,
		Candidates: []string{"Concepts/golang/A.md", "Concepts/golang/B.md"},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Resolve(duplicate alias) mismatch (-want +got):\n%s", diff)
	}
}

func TestResolveSameFilenameDifferentFolderIsAmbiguous(t *testing.T) {
	t.Parallel()
	idx := graph.BuildFromNotes([]graph.NoteInput{
		{RelPath: "golang/Foo.md"},
		{RelPath: "rust/Foo.md"},
	}, nil)

	got := idx.Resolve("Foo")
	if got.Kind != graph.KindAmbiguous {
		t.Fatalf("Resolve(Foo) = %+v, want Ambiguous", got)
	}
	want := []string{"golang/Foo.md", "rust/Foo.md"}
	if diff := cmp.Diff(want, got.Candidates); diff != "" {
		t.Errorf("Candidates mismatch (-want +got):\n%s", diff)
	}
}

func TestResolveReturnsIndependentCandidates(t *testing.T) {
	t.Parallel()
	idx := graph.BuildFromNotes([]graph.NoteInput{
		{RelPath: "A/Foo.md"},
		{RelPath: "B/Foo.md"},
	}, nil)

	first := idx.Resolve("Foo")
	if first.Kind != graph.KindAmbiguous || len(first.Candidates) != 2 {
		t.Fatalf("Resolve(Foo) = %+v, want two ambiguous candidates", first)
	}
	first.Candidates[0] = "mutated"
	second := idx.Resolve("Foo")
	want := []string{"A/Foo.md", "B/Foo.md"}
	if diff := cmp.Diff(want, second.Candidates); diff != "" {
		t.Errorf("Resolve(Foo) after caller mutation mismatch (-want +got):\n%s", diff)
	}
}

func TestResolveNonMarkdownResourceNeedsExtension(t *testing.T) {
	t.Parallel()
	idx := graph.BuildFromNotes(
		[]graph.NoteInput{{RelPath: "Sources/DDIA.md"}},
		[]string{"Diagrams/canvas/DDIA-Ch1-Overview.canvas"},
	)

	got := idx.Resolve("DDIA-Ch1-Overview.canvas")
	if got.Kind != graph.KindUnique || got.RelPath != "Diagrams/canvas/DDIA-Ch1-Overview.canvas" {
		t.Errorf("Resolve(with extension) = %+v, want Unique Diagrams/canvas/DDIA-Ch1-Overview.canvas", got)
	}
	// Without the extension, a resource does not resolve — Obsidian
	// itself requires the extension to link a non-note file.
	if got := idx.Resolve("DDIA-Ch1-Overview"); got.Kind != graph.KindUnresolved {
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

// TestParseWikilink covers the fragments SplitWikilink drops. The table above
// is what holds the unchanged-semantics promise for the callers that only want
// a name and a label: it carries its own hand-written answers, so it still
// fails if this parse ever starts reading those forms differently.
func TestParseWikilink(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		inner string
		want  graph.Wikilink
		ok    bool
	}{
		{
			name:  "a name alone addresses the whole note",
			inner: "Go Slice", ok: true,
			want: graph.Wikilink{Target: "Go Slice", Display: "Go Slice"},
		},
		{
			name:  "a heading is kept apart from the name it follows",
			inner: "Go Slice#Internals", ok: true,
			want: graph.Wikilink{Target: "Go Slice", Display: "Go Slice#Internals", Heading: "Internals"},
		},
		{
			name:  "display text replaces the label and leaves the heading intact",
			inner: "玻璃潮初稿#第三節：失約的燈|回到那一段", ok: true,
			want: graph.Wikilink{Target: "玻璃潮初稿", Display: "回到那一段", Heading: "第三節：失約的燈"},
		},
		{
			name:  "a block address is its own kind of fragment",
			inner: "Go Slice^abc123", ok: true,
			want: graph.Wikilink{Target: "Go Slice", Display: "Go Slice^abc123", Block: "abc123"},
		},
		{
			// The form Obsidian actually writes. Read as a heading, the
			// caret would become part of a section name and a caller would
			// build an anchor for a paragraph that has none.
			name:  "a block address written after the fragment marker is still a block",
			inner: "Go Slice#^abc123", ok: true,
			want: graph.Wikilink{Target: "Go Slice", Display: "Go Slice#^abc123", Block: "abc123"},
		},
		{
			name:  "a block address beside a section name stays a block address",
			inner: "Go Slice^abc123#Internals", ok: true,
			want: graph.Wikilink{Target: "Go Slice", Display: "Go Slice^abc123#Internals", Heading: "Internals", Block: "abc123"},
		},
		{
			name:  "an escaped table-cell pipe still separates the display text",
			inner: `Go Slice#Internals\|see`, ok: true,
			want: graph.Wikilink{Target: "Go Slice", Display: "see", Heading: "Internals"},
		},
		{
			name:  "the display separator is the first pipe, so a second pipe belongs to the display text",
			inner: "Note|a|b", ok: true,
			want: graph.Wikilink{Target: "Note", Display: "a|b"},
		},
		{
			// A cell can stack escapes; every trailing backslash comes off the
			// name, not just the last one, so the deepest escape still yields
			// the same target as no escape at all.
			name:  "every trailing backslash ahead of the pipe is stripped",
			inner: `Go Slice\\\|see`, ok: true,
			want: graph.Wikilink{Target: "Go Slice", Display: "see"},
		},
		{
			name:  "surrounding space belongs to neither the name nor the heading",
			inner: "  Go Slice #  Internals  ", ok: true,
			want: graph.Wikilink{Target: "Go Slice", Display: "Go Slice #  Internals", Heading: "Internals"},
		},
		{
			name:  "a bare heading names no other file",
			inner: "#Heading", ok: false,
			want: graph.Wikilink{Target: "", Display: "#Heading", Heading: "Heading"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := graph.ParseWikilink(tt.inner)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("ParseWikilink(%q) mismatch (-want +got):\n%s", tt.inner, diff)
			}
			if ok != tt.ok {
				t.Errorf("ParseWikilink(%q) ok = %v, want %v", tt.inner, ok, tt.ok)
			}
		})
	}
}

func TestWikilinkFragmentsResolveOnNameAloneRegardlessOfHeadingExistence(t *testing.T) {
	t.Parallel()
	// Anchors are never verified: [[Go Slice#NoSuchHeading]] must resolve
	// exactly like [[Go Slice]] — only the target file's existence
	// matters.
	idx := graph.BuildFromNotes([]graph.NoteInput{{RelPath: "Go Slice.md"}}, nil)

	target, _, ok := graph.SplitWikilink("Go Slice#This Heading Does Not Exist Anywhere")
	if !ok {
		t.Fatalf("SplitWikilink() ok = false, want true")
	}
	got := idx.Resolve(target)
	if got.Kind != graph.KindUnique || got.RelPath != "Go Slice.md" {
		t.Errorf("Resolve(%q) = %+v, want Unique Go Slice.md", target, got)
	}
}

// A name several files answer to resolves to nothing, and that is true from
// the moment the second file lands — not from the moment somebody writes the
// citation that will fail. Reporting the pair is therefore a question about
// the index, which this answers, and every key a file is reachable by counts:
// the stem, the filename, and a resource's own name.
func TestCollisionsAreEveryNameMoreThanOneFileClaims(t *testing.T) {
	t.Parallel()

	idx := graph.BuildFromNotes([]graph.NoteInput{
		{RelPath: "golang/Foo.md"},
		{RelPath: "rust/Foo.md"},
		{RelPath: "notes/Alone.md", Aliases: []string{"solo"}},
	}, []string{"img/shot.png", "old/shot.png"})

	// The absences carry as much as the entries: a name one file holds, an
	// alias, and the full paths are all reachable and none of them collide.
	want := map[string][]string{
		"foo":      {"golang/Foo.md", "rust/Foo.md"},
		"foo.md":   {"golang/Foo.md", "rust/Foo.md"},
		"shot.png": {"img/shot.png", "old/shot.png"},
	}
	if diff := cmp.Diff(want, idx.Collisions()); diff != "" {
		t.Errorf("Collisions() mismatch (-want +got):\n%s", diff)
	}
}

// TestDistinctCollisionsDropsOnlyRestatedNames pins the difference between the
// two answers. Every surface that counts collisions counts one repair once —
// two files sharing "Foo.md" necessarily share "Foo", and a page or a gate
// reporting both would have the reader fix one thing twice — but a name that
// carries the extension and is claimed by a different set of files is its own
// repair and survives.
func TestDistinctCollisionsDropsOnlyRestatedNames(t *testing.T) {
	t.Parallel()

	idx := graph.BuildFromNotes([]graph.NoteInput{
		{RelPath: "golang/Foo.md"},
		{RelPath: "rust/Foo.md"},
		{RelPath: "a/Bar.md"},
		{RelPath: "b/Bar.md"},
	}, []string{"legacy/Bar"})

	// "foo.md" is absent because its claimants are exactly "foo"'s. "bar.md"
	// stays because the extension-less name is also claimed by a file that
	// does not carry the extension, so the two name different repairs.
	want := map[string][]string{
		"foo":    {"golang/Foo.md", "rust/Foo.md"},
		"bar":    {"a/Bar.md", "b/Bar.md", "legacy/Bar"},
		"bar.md": {"a/Bar.md", "b/Bar.md"},
	}
	if diff := cmp.Diff(want, idx.DistinctCollisions()); diff != "" {
		t.Errorf("DistinctCollisions() mismatch (-want +got):\n%s", diff)
	}
}

// The answer belongs to whoever asked for it: a caller sorting or trimming the
// paths it was handed must not reach back into the resolver everyone shares.
func TestCollisionsReturnsIndependentCandidates(t *testing.T) {
	t.Parallel()

	idx := graph.BuildFromNotes([]graph.NoteInput{{RelPath: "A/Foo.md"}, {RelPath: "B/Foo.md"}}, nil)

	idx.Collisions()["foo"][0] = "mutated"
	want := []string{"A/Foo.md", "B/Foo.md"}
	if diff := cmp.Diff(want, idx.Collisions()["foo"]); diff != "" {
		t.Errorf("Collisions()[foo] after caller mutation mismatch (-want +got):\n%s", diff)
	}
}

// A resolution kind reaches a log line, a panic and a rendered fault, and in
// each of those places a number is a lookup the reader has to perform against
// a constant block they do not have open.
func TestKindNamesItself(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		kind graph.Kind
		want string
	}{
		{graph.KindUnresolved, "unresolved"},
		{graph.KindUnique, "unique"},
		{graph.KindAmbiguous, "ambiguous"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()

			if got := tc.kind.String(); got != tc.want {
				t.Errorf("Kind.String() = %q, want %q", got, tc.want)
			}
		})
	}
}

// A kind the constant block does not declare has no name, and answering with
// one of the three would report a resolution that never happened.
func TestKindRefusesToNameAValueItDoesNotDeclare(t *testing.T) {
	t.Parallel()

	defer func() {
		recovered := recover()
		text, isText := recovered.(string)
		if !isText || !strings.Contains(text, "99") {
			t.Errorf("panic = %v, want a message naming the value 99", recovered)
		}
	}()
	_ = graph.Kind(99).String()
	t.Error("Kind(99).String() returned instead of panicking")
}

// TestSectionIDNamesAPlaceTheSameWayForEveryFace pins the id a section name
// folds to. Both the page that stamps the id and the adjudicator that asks
// whether a note answers one read it from here, so a change made for either of
// them moves every anchor and every fragment in the vault at once.
//
// The rows are the spellings that separate one plausible fold from another:
// letter case, characters that are letters without being ASCII, characters that
// look like letters and are not, characters that are counted as digits without
// looking like one, and names that fold to nothing at all. The two
// Unicode forms of one word are built rather than typed, since a source file
// cannot show which form it is carrying.
func TestSectionIDNamesAPlaceTheSameWayForEveryFace(t *testing.T) {
	t.Parallel()

	tests := []struct{ name, want string }{
		{"Plain Heading", "plain-heading"},
		{"  Trailing and leading  ", "trailing-and-leading"},
		{"CJK 標題", "cjk-標題"},
		{"Heading -- with punctuation!", "heading-with-punctuation"},
		{"under_score", "under-score"},
		{"x²", "x²"},
		{"½", "½"},
		{"!!!", "section"},
		{"", "section"},
		{"   ", "section"},
		{"ＦＵＬＬＷＩＤＴＨ", "ｆｕｌｌｗｉｄｔｈ"},
	}
	for _, tt := range tests {
		if got := graph.SectionID(tt.name); got != tt.want {
			t.Errorf("SectionID(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}

	for _, name := range []string{"Mañana", "がん", "Å"} {
		composed, decomposed := norm.NFC.String(name), norm.NFD.String(name)
		if composed == decomposed {
			t.Fatalf("%q has one Unicode form, so it says nothing about folding the other", name)
		}
		if graph.SectionID(composed) != graph.SectionID(decomposed) {
			t.Errorf("the two Unicode forms of %q stamp two ids: %q and %q",
				name, graph.SectionID(composed), graph.SectionID(decomposed))
		}
	}
}
