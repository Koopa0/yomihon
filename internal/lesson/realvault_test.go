package lesson_test

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/koopa0/yomihon/internal/lesson"
)

// slotSlug is the namespace the Minna-no-Nihongo slot sidecars declare
// (jp-minna-lNN) — the slug policy's lesson namespace, checked here so the join
// key stays well-formed against the real files.
var slotSlug = regexp.MustCompile(`^jp-minna-l\d+$`)

// TestRealVaultSlotsLoadAndValidate exercises the slot loader directly against
// the actual lesson sidecars, not fixtures: it builds the slug index from the
// real vault's System/slots/ directory and asserts
// every sidecar parses, validates clean (the sidecars are known-good curated
// data, so any Validate problem is a real regression), declares a well-formed
// jp-minna-lNN slug, and round-trips through the index by that slug. It follows
// the same t.Skipf-when-vault-absent pattern as render's real-vault guard, so
// it runs loudly whenever the vault is present and is skipped — never vacuously
// passing — when it is not.
func TestRealVaultSlotsLoadAndValidate(t *testing.T) {
	t.Parallel()

	root := os.Getenv("YOMIHON_ROOT")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Skipf("no home dir: %v", err)
		}
		root = filepath.Join(home, "obsidian")
	}
	slotsDir := filepath.Join(root, "System", "slots")
	if _, err := os.Stat(slotsDir); err != nil { // #nosec G703 -- probing the operator's own vault to decide whether to skip
		t.Skipf("real vault slots not available: %v", err)
	}

	idx, err := lesson.BuildSlotIndex(slotsDir)
	if err != nil {
		t.Fatalf("BuildSlotIndex(%q) = %v", slotsDir, err)
	}

	// The lesson set spans L01–L20 today and grows as lessons are
	// added; 20 is the floor that proves the sweep ran against the real
	// directory rather than a near-empty stand-in.
	const minExpectedSidecars = 20
	if len(idx) < minExpectedSidecars {
		t.Errorf("indexed %d slot sidecars under %s, want at least %d — is this the real vault?",
			len(idx), slotsDir, minExpectedSidecars)
	}

	for slug, s := range idx {
		if !slotSlug.MatchString(slug) {
			t.Errorf("sidecar %s declares slug %q, want jp-minna-lNN form", s.Lesson, slug)
		}
		if problems := s.Validate(); len(problems) != 0 {
			t.Errorf("slot sidecar %s (%s) has %d validation problems: %v", s.Lesson, slug, len(problems), problems)
		}
	}

	// Round-trip the canonical join: L01's note carries slug jp-minna-l01, and
	// that slug must resolve to L01's sidecar through the index.
	if got, ok := idx.Lookup("jp-minna-l01"); !ok {
		t.Error("Lookup(jp-minna-l01) not found — the L01 slot sidecar is missing or mis-slugged")
	} else if got.Lesson != "L01" {
		t.Errorf("Lookup(jp-minna-l01).Lesson = %q, want %q", got.Lesson, "L01")
	}
}
