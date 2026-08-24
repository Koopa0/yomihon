package note_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// outOfEnumVault seeds one folder whose distribution carries both a declared
// status and one no group's enum lists for its carrier ("reviewing" on a
// concept note).
func outOfEnumVault(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	write("Concepts/legal.md", "---\ntitle: Legal\ntype: concept\ndomain: golang\nstatus: draft\n---\n\nbody\n")
	write("Concepts/outside.md", "---\ntitle: Outside\ntype: concept\ndomain: golang\nstatus: reviewing\n---\n\nbody\n")
	write("Concepts/outside-too.md", "---\ntitle: Outside Too\ntype: concept\ndomain: golang\nstatus: reviewing\n---\n\nbody\n")
	return root
}

// homeChip cuts the status-distribution chip showing the given status name
// out of a rendered Home body. The returned markup starts at the tail of the
// chip's class attribute, so a caller sees both the flag modifier and the
// link target.
func homeChip(t *testing.T, body, name string) string {
	t.Helper()
	for _, chunk := range strings.Split(body, `<a class="y-homechip`)[1:] {
		chip, _, terminated := strings.Cut(chunk, "</a>")
		if !terminated {
			t.Fatalf("unterminated home chip: %q", chunk)
		}
		if strings.Contains(chip, ">"+name+"</span>") {
			return chip
		}
	}
	t.Fatalf("the distribution has no %q chip; body = %q", name, body)
	return ""
}

// TestHomeFlagsAStatusOutsideEveryCarriersEnum locks the distribution's
// honesty: a chip whose status no carrying type declares renders with the
// same amber flag family the note page uses, and stays a link — flagged,
// never hidden. A declared status keeps its plain chip.
func TestHomeFlagsAStatusOutsideEveryCarriersEnum(t *testing.T) {
	t.Parallel()
	srv := newServerWithContract(t, outOfEnumVault(t), loadHomeContract(t))
	code, body := get(t, srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if !strings.Contains(body, `data-home-block="lifecycle"`) {
		t.Fatalf("home carries no lifecycle block; body = %q", body)
	}

	reviewing := homeChip(t, body, "reviewing")
	if !strings.Contains(reviewing, "y-homechip--unknown") {
		t.Errorf("the reviewing chip renders identically to declared ones, want the out-of-enum flag; chip = %q", reviewing)
	}
	if !strings.Contains(reviewing, `href="`) {
		t.Errorf("the flagged chip lost its link; chip = %q", reviewing)
	}
	if !strings.Contains(reviewing, "不在 schema 允許清單中") {
		t.Errorf("the flagged chip does not say what the flag means; chip = %q", reviewing)
	}

	draft := homeChip(t, body, "draft")
	if strings.Contains(draft, "y-homechip--unknown") {
		t.Errorf("a declared status is flagged; chip = %q", draft)
	}
}

// TestHealthCountsStatusesOutsideTheEnum locks the whole-folder line: the
// count of notes whose status is outside their type's declared list, and
// nothing at all when there are none.
func TestHealthCountsStatusesOutsideTheEnum(t *testing.T) {
	t.Parallel()
	srv := newServerWithContract(t, outOfEnumVault(t), loadHomeContract(t))
	code, body := get(t, srv.URL+"/health")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if !strings.Contains(body, "狀態值不在允許清單的筆記") {
		t.Fatalf("the health page carries no out-of-enum status line; body = %q", body)
	}
	section := healthSectionBody(t, body, "狀態值不在允許清單的筆記")
	if !strings.Contains(section, ">2</span>") {
		t.Errorf("the out-of-enum count is not 2; section = %q", section)
	}
}

func TestHealthStaysSilentWhenEveryStatusIsDeclared(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	full := filepath.Join(root, "Concepts", "legal.md")
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := "---\ntitle: Legal\ntype: concept\ndomain: golang\nstatus: draft\n---\n\nbody\n"
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	srv := newServerWithContract(t, root, loadHomeContract(t))
	code, body := get(t, srv.URL+"/health")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if strings.Contains(body, "狀態值不在允許清單的筆記") {
		t.Errorf("zero out-of-enum notes still render a line; body = %q", body)
	}
}
