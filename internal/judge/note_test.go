package judge

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/text/unicode/norm"
)

func TestParseNote(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		content         string
		wantNoFM        bool
		wantBadFM       bool
		wantFrontmatter map[string]fmValue
	}{
		{
			name:     "no frontmatter block is legal",
			content:  "just a transcript\nno fence\n",
			wantNoFM: true,
		},
		{
			name:      "present but unparseable is flagged bad",
			content:   "---\ntitle: \"unclosed\ndomain: x\n---\nbody\n",
			wantBadFM: true,
		},
		{
			name:    "scalars and a list are read raw",
			content: "---\ntitle: X\ntype: concept\ntags:\n  - a\n  - b/c\n---\nbody\n",
			wantFrontmatter: map[string]fmValue{
				"title": {scalar: "X", scalarIsString: true},
				"type":  {scalar: "concept", scalarIsString: true},
				"tags":  {list: []string{"a", "b/c"}, stringList: []string{"a", "b/c"}, isList: true},
			},
		},
		{
			name:    "an explicit null reads as an empty scalar",
			content: "---\nstatus:\ntype: concept\n---\n",
			wantFrontmatter: map[string]fmValue{
				"status": {scalar: ""},
				"type":   {scalar: "concept", scalarIsString: true},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parseNote("x.md", []byte(tt.content))
			if got.noFrontmatter != tt.wantNoFM {
				t.Errorf("parseNote(%q) noFrontmatter = %v, want %v", tt.content, got.noFrontmatter, tt.wantNoFM)
			}
			if got.badFrontmatter != tt.wantBadFM {
				t.Errorf("parseNote(%q) badFrontmatter = %v, want %v", tt.content, got.badFrontmatter, tt.wantBadFM)
			}
			if diff := cmp.Diff(tt.wantFrontmatter, got.frontmatter, cmp.AllowUnexported(fmValue{})); diff != "" {
				t.Errorf("parseNote(%q) frontmatter mismatch (-want +got):\n%s", tt.content, diff)
			}
		})
	}
}

func TestFmValue(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		value        fmValue
		wantScalar   string
		wantIsScalar bool
		wantPresent  bool
		wantStrings  []string
	}{
		{name: "non-empty scalar", value: fmValue{scalar: "x", scalarIsString: true}, wantScalar: "x", wantIsScalar: true, wantPresent: true, wantStrings: []string{"x"}},
		{name: "empty scalar", value: fmValue{scalar: ""}, wantScalar: "", wantIsScalar: true, wantPresent: false},
		{name: "coerced scalar", value: fmValue{scalar: "7"}, wantScalar: "7", wantIsScalar: true, wantPresent: true},
		{name: "non-empty list", value: fmValue{list: []string{"a", "7"}, stringList: []string{"a"}, isList: true}, wantScalar: "", wantIsScalar: false, wantPresent: true, wantStrings: []string{"a"}},
		{name: "empty list", value: fmValue{list: nil, isList: true}, wantScalar: "", wantIsScalar: false, wantPresent: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotScalar, gotIsScalar := tt.value.asScalar()
			if gotScalar != tt.wantScalar || gotIsScalar != tt.wantIsScalar {
				t.Errorf("asScalar() = (%q, %v), want (%q, %v)", gotScalar, gotIsScalar, tt.wantScalar, tt.wantIsScalar)
			}
			if got := tt.value.present(); got != tt.wantPresent {
				t.Errorf("present() = %v, want %v", got, tt.wantPresent)
			}
			if got := tt.value.stringValues(); !slices.Equal(got, tt.wantStrings) {
				t.Errorf("stringValues() = %v, want %v", got, tt.wantStrings)
			}
		})
	}
}

// TestCollectNotesScanBoundary pins the scan boundary the diagnostics rely on,
// which must match the resolver's: dot-prefixed directories and files are
// skipped, a .gitignore is never honored (a gitignored file is still scanned),
// only markdown files become notes (an image is a resource, not a note), the
// Diary is scanned like any other directory, and every path comes back in
// Obsidian's canonical NFC form.
func TestCollectNotesScanBoundary(t *testing.T) {
	root := t.TempDir()
	writeTestContract(t, root, nil)
	write(t, root, "notes/keep.md", "---\ntype: concept\n---\n")
	write(t, root, "notes/image.png", "not markdown, a linkable resource\n")
	write(t, root, ".obsidian/config.md", "hidden directory, skipped\n")
	write(t, root, ".hidden.md", "hidden file, skipped\n")
	write(t, root, ".gitignore", "secret.md\n")
	write(t, root, "secret.md", "---\ntype: x\n---\n")
	write(t, root, "Diary/2026-07-04.md", "a day, scanned like any other note\n")

	want := []string{
		"Diary/2026-07-04.md",
		"notes/keep.md",
		"secret.md",
	}
	// A note whose filename is stored decomposed comes back composed while its
	// captured entry retains the raw spelling needed to read it on a byte-exact
	// filesystem.
	const composed = "だ.md"
	if decomposed := norm.NFD.String(composed); decomposed != composed {
		write(t, root, "notes/"+decomposed, "---\ntype: x\n---\n")
		want = append(want, "notes/"+composed)
	}

	notes, err := collectNotes(t.Context(), root)
	if err != nil {
		t.Fatalf("collectNotes(t.Context(), %q) = %v", root, err)
	}
	got := make([]string, len(notes))
	for i, n := range notes {
		got[i] = n.path
	}
	slices.Sort(got)
	slices.Sort(want)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("collectNotes(t.Context(), %q) note paths mismatch (-want +got):\n%s", root, diff)
	}
}

// write creates a file at root/rel (rel in slash form), making parents.
func write(tb testing.TB, root, rel, content string) {
	tb.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		tb.Fatalf("mkdir for %q: %v", rel, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil { // #nosec G703 -- test fixture paths are owned by each caller's temporary root
		tb.Fatalf("write %q: %v", rel, err)
	}
}
