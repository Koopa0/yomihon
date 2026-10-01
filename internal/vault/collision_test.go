package vault

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The pair every test below collides: one file name as a Linux keyboard or an
// NFC-normalizing sync writes it, and the same name as a macOS-to-Linux copy or
// a git checkout leaves it. They print as the same glyph, which is the whole
// difficulty of repairing the folder that holds both.
const (
	collidingComposed   = "Notes/\u304c.md"
	collidingDecomposed = "Notes/\u304b\u3099.md"
)

// TestACanonicalCollisionNamesBothSpellings pins what the refusal carries. The
// repair for two names with one canonical path is to delete or rename one of
// them, and nobody can do that from "vault contains canonically colliding
// paths" alone. The pair is reported in byte order whichever name the walk met
// first, so the same folder reads the same way every time it is refused.
func TestACanonicalCollisionNamesBothSpellings(t *testing.T) {
	t.Parallel()

	// The decomposed spelling sorts first: its third byte is 0x8b where the
	// composed one has 0x8c.
	want := [2]string{collidingDecomposed, collidingComposed}
	for _, tt := range []struct {
		name  string
		first string
		then  string
	}{
		{name: "the composed name first", first: collidingComposed, then: collidingDecomposed},
		{name: "the decomposed name first", first: collidingDecomposed, then: collidingComposed},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			seen := make(map[string]string)
			if err := recordCanonicalPath(seen, tt.first, collidingComposed); err != nil {
				t.Fatalf("record %q: %v", tt.first, err)
			}
			err := recordCanonicalPath(seen, tt.then, collidingComposed)
			if !errors.Is(err, ErrCanonicalCollision) {
				t.Fatalf("recording the second spelling = %v, want ErrCanonicalCollision", err)
			}
			collision, ok := errors.AsType[*CollisionError](err)
			if !ok {
				t.Fatalf("the refusal %T carries no CollisionError, so it names no file", err)
			}
			if collision.Paths != want {
				t.Errorf("collision paths = %q, want %q", collision.Paths, want)
			}
			for _, raw := range want {
				if !strings.Contains(err.Error(), Spelled(raw)) {
					t.Errorf("the refusal %q does not name %q", err, raw)
				}
			}
		})
	}
}

// TestSpelledTellsTwoNamesThatPrintAlikeApart holds the reason the refusal is
// not just two quoted names. A composed が and a decomposed か + U+3099 are one
// glyph on every screen, so a message quoting them plainly shows the reader the
// same word twice; the escapes are where the two differ.
func TestSpelledTellsTwoNamesThatPrintAlikeApart(t *testing.T) {
	t.Parallel()

	// The two strings are different bytes either way; what matters is that a
	// person reading them is shown the difference, which is the escapes.
	for raw, escapes := range map[string]string{
		collidingComposed:   `\u304c`,
		collidingDecomposed: `\u304b\u3099`,
	} {
		if got := Spelled(raw); !strings.Contains(got, escapes) {
			t.Errorf("Spelled(%q) = %s, want the escapes %s beside it so it reads differently from its twin", raw, got, escapes)
		}
	}
	if plain := Spelled("Notes/plain.md"); plain != `"Notes/plain.md"` {
		t.Errorf("Spelled(ASCII path) = %s, want it quoted and nothing more", plain)
	}
}

// TestAScanRefusedForCollidingNamesNamesBothFiles drives the same refusal
// through the real walk of a real directory, for both scans. The two cases
// above prove the error is built right; this proves the walk is what hands it
// to a caller, wrapped as it is on the way out.
func TestAScanRefusedForCollidingNamesNamesBothFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeReaderFixture(t, root, collidingComposed, "composed")
	writeReaderFixture(t, root, collidingDecomposed, "decomposed")
	entries, err := os.ReadDir(filepath.Join(root, "Notes"))
	if err != nil {
		t.Fatalf("ReadDir(Notes) error = %v", err)
	}
	if len(entries) != 2 {
		t.Skip("this filesystem folds the two spellings into one file, so there is no pair to refuse")
	}
	reader := openTestReader(t, root)

	for name, scan := range map[string]func(context.Context) (Scan, error){
		"ScanAvailable": reader.ScanAvailable,
		"ScanComplete":  reader.ScanComplete,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := scan(t.Context())
			collision, ok := errors.AsType[*CollisionError](err)
			if !ok {
				t.Fatalf("%s error = %v, want a CollisionError naming the pair", name, err)
			}
			if want := [2]string{collidingDecomposed, collidingComposed}; collision.Paths != want {
				t.Errorf("%s collision paths = %q, want %q", name, collision.Paths, want)
			}
			if !errors.Is(err, ErrCanonicalCollision) {
				t.Errorf("%s error = %v, want it to stay an ErrCanonicalCollision", name, err)
			}
		})
	}
}
