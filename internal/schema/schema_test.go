package schema_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/kurodo/internal/schema"
)

// The testdata contract is a loader fixture, not a second schema: runtime
// code only ever reads the real vault contract.
func loadFixture(t *testing.T) *schema.Schema {
	t.Helper()
	s, err := schema.LoadFile(filepath.Join("testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("LoadFile(testdata/contract.toml) = %v", err)
	}
	return s
}

func TestStatusGroup(t *testing.T) {
	t.Parallel()
	s := loadFixture(t)

	tests := []struct {
		noteType string
		want     string
	}{
		{"lesson", "lesson"},
		{"system", "system"},
		{"guide", "system"},
		{"concept", "note"},
		{"writing", "note"},
	}
	for _, tt := range tests {
		t.Run(tt.noteType, func(t *testing.T) {
			t.Parallel()
			if got := s.StatusGroup(tt.noteType); got != tt.want {
				t.Errorf("StatusGroup(%q) = %q, want %q", tt.noteType, got, tt.want)
			}
		})
	}
}

func TestStatuses(t *testing.T) {
	t.Parallel()
	s := loadFixture(t)

	got := s.Statuses("lesson")
	want := []string{"imported", "draft", "ready", "archived"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Statuses(\"lesson\") mismatch (-want +got):\n%s", diff)
	}
}

func TestTransition(t *testing.T) {
	t.Parallel()
	s := loadFixture(t)

	tests := []struct {
		name    string
		typ     string
		from    string
		to      string
		actor   string
		wantErr error
	}{
		{"lesson imported→draft by claude", "lesson", "imported", "draft", "claude", nil},
		{"lesson draft→ready by koopa", "lesson", "draft", "ready", "koopa", nil},
		{"ready is koopa-only", "lesson", "draft", "ready", "claude", schema.ErrOwnerForbidden},
		{"skip a stage", "lesson", "imported", "ready", "koopa", schema.ErrIllegalTransition},
		{"archived from anywhere", "concept", "seedling", "archived", "koopa", nil},
		{"initial captured", "transcript", "", "captured", "hermes", nil},
		{"non-initial needs predecessor", "source-note", "", "cleaned", "claude", schema.ErrIllegalTransition},
		{"undefined status for type", "concept", "seedling", "ready", "koopa", schema.ErrUnknownStatus},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := s.Transition(tt.typ, tt.from, tt.to, tt.actor)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Transition(%q, %q, %q, %q) = %v, want %v",
					tt.typ, tt.from, tt.to, tt.actor, err, tt.wantErr)
			}
		})
	}
}

// TestLoadRealContract locks the assumptions this repo makes about the real
// vault contract. Skips when the vault is absent (fresh clone, CI).
func TestLoadRealContract(t *testing.T) {
	t.Parallel()

	root := os.Getenv("KURODO_ROOT")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Skipf("no home dir: %v", err)
		}
		root = filepath.Join(home, "obsidian")
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))); err != nil { // #nosec G703 -- probing the operator's own vault to decide whether to skip
		t.Skipf("real vault contract not available: %v", err)
	}

	s, err := schema.Load(root)
	if err != nil {
		t.Fatalf("Load(%q) = %v", root, err)
	}
	if s.Version != "1" {
		t.Errorf("Version = %q, want %q", s.Version, "1")
	}
	st, ok := s.Stage("lesson", "ready")
	if !ok {
		t.Fatal("Stage(\"lesson\", \"ready\") not found in real contract")
	}
	want := []string{"koopa"}
	if diff := cmp.Diff(want, st.Owner); diff != "" {
		t.Errorf("ready owner mismatch (-want +got):\n%s", diff)
	}
}
