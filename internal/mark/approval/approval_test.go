package approval_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/mark/approval"
	"github.com/koopa0/yomihon/internal/mark/confdir"
)

var approvedAt = time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)

func newStore(t *testing.T) *approval.Store {
	t.Helper()
	store, err := approval.Open(confdir.At(t.TempDir()), "/vaults/notes")
	if err != nil {
		t.Fatalf("approval.Open: %v", err)
	}
	return store
}

// TestAnApprovalNamesExactBytes is the approval's whole meaning: it holds for
// the path and the digest it was given, and for nothing else. One byte
// changed in the figure is a different digest, and the approval no longer
// names the file.
func TestAnApprovalNamesExactBytes(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	approved := sha256.Sum256([]byte("<script>draw()</script>"))
	edited := sha256.Sum256([]byte("<script>draw() </script>"))

	if ok, lapsed, err := store.Approved("Figures/a.html", approved); err != nil || ok || lapsed {
		t.Fatalf("Approved before any approval = %v, %v, %v; want false, false, nil", ok, lapsed, err)
	}
	if err := store.Approve("Figures/a.html", approved, approvedAt); err != nil {
		t.Fatalf("Approve = %v", err)
	}
	for _, tc := range []struct {
		name   string
		path   string
		sum    [sha256.Size]byte
		want   bool
		lapsed bool
	}{
		{"the approved bytes", "Figures/a.html", approved, true, false},
		{"one byte changed", "Figures/a.html", edited, false, true},
		{"the same bytes elsewhere", "Figures/b.html", approved, false, false},
	} {
		got, lapsed, err := store.Approved(tc.path, tc.sum)
		if err != nil || got != tc.want || lapsed != tc.lapsed {
			t.Errorf("Approved(%s) = %v, %v, %v; want %v, %v, nil", tc.name, got, lapsed, err, tc.want, tc.lapsed)
		}
	}

	// Approving the edited bytes replaces the approval of the old ones.
	if err := store.Approve("Figures/a.html", edited, approvedAt.Add(time.Hour)); err != nil {
		t.Fatalf("Approve(edited) = %v", err)
	}
	if ok, _, err := store.Approved("Figures/a.html", approved); err != nil || ok {
		t.Error("the earlier bytes are still approved after the path was approved for new ones")
	}
	held, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(held) != 1 {
		t.Errorf("List() holds %d approvals for one path, want 1: %v", len(held), held)
	}

	revoked, err := store.Revoke("Figures/a.html")
	if err != nil || !revoked {
		t.Fatalf("Revoke = %v, %v; want true, nil", revoked, err)
	}
	if ok, _, err := store.Approved("Figures/a.html", edited); err != nil || ok {
		t.Error("a revoked figure is still approved")
	}
	if again, err := store.Revoke("Figures/a.html"); err != nil || again {
		t.Errorf("Revoke of nothing = %v, %v; want false, nil", again, err)
	}
}

// TestUnreadableApprovalsFailClosed: damage is never an empty list. The figure
// is not approved, the caller is told why, and a new approval does not
// replace the file a person has to look at.
func TestUnreadableApprovalsFailClosed(t *testing.T) {
	t.Parallel()
	sum := sha256.Sum256([]byte("x"))
	for _, tc := range []struct {
		name string
		data string
	}{
		{"not json", "{"},
		{"another version", `{"version":2,"vault":"/v","approvals":[]}`},
		{"no list", `{"version":1,"vault":"/v"}`},
		{"unknown field", `{"version":1,"vault":"/v","approvals":[],"yes":true}`},
		{"upper case digest", `{"version":1,"vault":"/v","approvals":[{"path":"a.html","sha256":"` + upperHex(sum) + `","time":"2026-11-12T09:00:00Z"}]}`},
		{"escaping path", `{"version":1,"vault":"/v","approvals":[{"path":"../a.html","sha256":"` + lowerHex(sum) + `","time":"2026-11-12T09:00:00Z"}]}`},
		{"trailing bytes", `{"version":1,"vault":"/v","approvals":[]} {}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			store := newStore(t)
			if err := os.MkdirAll(filepath.Dir(store.Path()), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(store.Path(), []byte(tc.data), 0o600); err != nil {
				t.Fatal(err)
			}
			ok, _, err := store.Approved("a.html", sum)
			if ok || !errors.Is(err, approval.ErrUnreadable) {
				t.Errorf("Approved over %s = %v, %v; want false, ErrUnreadable", tc.name, ok, err)
			}
			if approveErr := store.Approve("a.html", sum, approvedAt); !errors.Is(approveErr, approval.ErrUnreadable) {
				t.Errorf("Approve over %s = %v; want ErrUnreadable", tc.name, approveErr)
			}
			after, err := os.ReadFile(store.Path())
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != tc.data {
				t.Errorf("the unreadable file was replaced: %q", after)
			}
		})
	}
}

func TestApproveRefusesAPathItCouldNotHandBack(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	for _, relPath := range []string{"", "/abs.html", "../up.html", "a\nb.html", "Cafe\u0301.html"} {
		if err := store.Approve(relPath, sha256.Sum256(nil), approvedAt); !errors.Is(err, approval.ErrInvalid) {
			t.Errorf("Approve(%q) = %v, want ErrInvalid", relPath, err)
		}
	}
	held, err := store.List()
	if err != nil || len(held) != 0 {
		t.Errorf("List() after refusals = %v, %v; want empty", held, err)
	}
	if diff := cmp.Diff([]approval.Approval{}, held); diff != "" {
		t.Errorf("List() mismatch (-want +got):\n%s", diff)
	}
}

func lowerHex(sum [sha256.Size]byte) string { return hex.EncodeToString(sum[:]) }

func upperHex(sum [sha256.Size]byte) string { return strings.ToUpper(hex.EncodeToString(sum[:])) }

// Asking to revoke what was never approved is not approving, and a directory
// made to hold the lock would be the first thing left behind.
func TestRevokingWhatWasNeverApprovedLeavesNoDirectory(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	store, err := approval.Open(confdir.At(config), "/vaults/notes")
	if err != nil {
		t.Fatal(err)
	}
	if revoked, err := store.Revoke("Figures/a.html"); err != nil || revoked {
		t.Fatalf("Revoke = %v, %v", revoked, err)
	}
	if _, err := os.Stat(filepath.Join(config, "yomihon")); !os.IsNotExist(err) {
		t.Errorf("asking created %s", filepath.Join(config, "yomihon"))
	}
}
