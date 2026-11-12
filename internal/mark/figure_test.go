package mark_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/mark"
)

var approvedAt = time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)

// TestAnApprovalNamesExactBytes is the approval's whole meaning: it holds for
// the path and the digest it was given, and for nothing else. One byte
// changed in the figure is a different digest, and the approval no longer
// names the file.
func TestAnApprovalNamesExactBytes(t *testing.T) {
	t.Parallel()
	file := newFile(t)
	approved := sha256.Sum256([]byte("<script>draw()</script>"))
	edited := sha256.Sum256([]byte("<script>draw() </script>"))

	if ok, lapsed, err := file.FigureApproved("Figures/a.html", approved); err != nil || ok || lapsed {
		t.Fatalf("FigureApproved before any approval = %v, %v, %v; want false, false, nil", ok, lapsed, err)
	}
	if err := file.ApproveFigure("Figures/a.html", approved, approvedAt); err != nil {
		t.Fatalf("ApproveFigure = %v", err)
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
		got, lapsed, err := file.FigureApproved(tc.path, tc.sum)
		if err != nil || got != tc.want || lapsed != tc.lapsed {
			t.Errorf("FigureApproved(%s) = %v, %v, %v; want %v, %v, nil", tc.name, got, lapsed, err, tc.want, tc.lapsed)
		}
	}

	// Approving the edited bytes replaces the approval of the old ones.
	if err := file.ApproveFigure("Figures/a.html", edited, approvedAt.Add(time.Hour)); err != nil {
		t.Fatalf("ApproveFigure(edited) = %v", err)
	}
	if ok, _, _ := file.FigureApproved("Figures/a.html", approved); ok {
		t.Error("the earlier bytes are still approved after the path was approved for new ones")
	}
	held, err := file.FigureApprovals()
	if err != nil {
		t.Fatal(err)
	}
	if len(held) != 1 {
		t.Errorf("FigureApprovals() holds %d approvals for one path, want 1: %v", len(held), held)
	}

	revoked, err := file.RevokeFigure("Figures/a.html")
	if err != nil || !revoked {
		t.Fatalf("RevokeFigure = %v, %v; want true, nil", revoked, err)
	}
	if ok, _, _ := file.FigureApproved("Figures/a.html", edited); ok {
		t.Error("a revoked figure is still approved")
	}
	if again, err := file.RevokeFigure("Figures/a.html"); err != nil || again {
		t.Errorf("RevokeFigure of nothing = %v, %v; want false, nil", again, err)
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
			file := newFile(t)
			if err := os.MkdirAll(filepath.Dir(file.ApprovalsPath()), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(file.ApprovalsPath(), []byte(tc.data), 0o600); err != nil {
				t.Fatal(err)
			}
			ok, _, err := file.FigureApproved("a.html", sum)
			if ok || !errors.Is(err, mark.ErrUnreadableApprovals) {
				t.Errorf("FigureApproved over %s = %v, %v; want false, ErrUnreadableApprovals", tc.name, ok, err)
			}
			if err := file.ApproveFigure("a.html", sum, approvedAt); !errors.Is(err, mark.ErrUnreadableApprovals) {
				t.Errorf("ApproveFigure over %s = %v; want ErrUnreadableApprovals", tc.name, err)
			}
			after, err := os.ReadFile(file.ApprovalsPath())
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != tc.data {
				t.Errorf("the unreadable file was replaced: %q", after)
			}
		})
	}
}

// TestApprovalsAreOwnerOnly: the record decides what runs in the browser, so
// it is created readable and writable by its owner alone.
func TestApprovalsAreOwnerOnly(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no POSIX permission bits to read back")
	}
	file := newFile(t)
	if err := file.ApproveFigure("a.html", sha256.Sum256(nil), approvedAt); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(file.ApprovalsPath())
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("approvals file mode = %o, want 600", got)
	}
	dir, err := os.Stat(filepath.Dir(file.ApprovalsPath()))
	if err != nil {
		t.Fatal(err)
	}
	if got := dir.Mode().Perm(); got != 0o700 {
		t.Errorf("approvals directory mode = %o, want 700", got)
	}
	leftovers, err := filepath.Glob(filepath.Join(filepath.Dir(file.ApprovalsPath()), ".figures-*"))
	if err != nil || len(leftovers) != 0 {
		t.Errorf("temporary files left behind: %v, %v", leftovers, err)
	}
}

func TestApproveFigureRefusesAPathItCouldNotHandBack(t *testing.T) {
	t.Parallel()
	file := newFile(t)
	for _, relPath := range []string{"", "/abs.html", "../up.html", "a\nb.html", "Café.html"} {
		if err := file.ApproveFigure(relPath, sha256.Sum256(nil), approvedAt); !errors.Is(err, mark.ErrInvalid) {
			t.Errorf("ApproveFigure(%q) = %v, want ErrInvalid", relPath, err)
		}
	}
	held, err := file.FigureApprovals()
	if err != nil || len(held) != 0 {
		t.Errorf("FigureApprovals() after refusals = %v, %v; want empty", held, err)
	}
	if diff := cmp.Diff([]mark.FigureApproval{}, held); diff != "" {
		t.Errorf("FigureApprovals() mismatch (-want +got):\n%s", diff)
	}
}

func lowerHex(sum [sha256.Size]byte) string { return hex.EncodeToString(sum[:]) }

func upperHex(sum [sha256.Size]byte) string { return strings.ToUpper(hex.EncodeToString(sum[:])) }
