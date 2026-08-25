package note_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
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

// TestMixedCarriersSplitTheChipAndTheCount pins the one value on which the
// two faces deliberately disagree: a status some carrying type declares and
// another does not. The chip speaks for the aggregated value — one carrier
// declares it, so it stays plain vocabulary — while health counts per note,
// and the carrier whose own type does not declare it is a finding.
func TestMixedCarriersSplitTheChipAndTheCount(t *testing.T) {
	t.Parallel()
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
	// The guide type sits in the system status group, which declares active;
	// the concept type sits in the default group, which does not.
	write("Concepts/handbook.md", "---\ntitle: Handbook\ntype: guide\ndomain: meta\nstatus: active\n---\n\nbody\n")
	write("Concepts/misfiled.md", "---\ntitle: Misfiled\ntype: concept\ndomain: golang\nstatus: active\n---\n\nbody\n")
	srv := newServerWithContract(t, root, loadHomeContract(t))

	code, home := get(t, srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("home status = %d, want 200", code)
	}
	if chip := homeChip(t, home, "active"); strings.Contains(chip, "y-homechip--unknown") {
		t.Errorf("a status one carrier declares is flagged as outside every enum; chip = %q", chip)
	}

	code, health := get(t, srv.URL+"/health")
	if code != http.StatusOK {
		t.Fatalf("health status = %d, want 200", code)
	}
	section := healthSectionBody(t, health, "狀態值不在允許清單的筆記")
	if !strings.Contains(section, ">1</span>") {
		t.Errorf("health does not count the one carrier whose type never declared the value; section = %q", section)
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

// TestHealthReachesEveryOutOfEnumNote locks the page's usefulness, not just
// its arithmetic. The section used to state a number and stop there, so the
// one reader who could act on it had to guess a query to find out which files
// it meant, while every other section on the same page named its findings and
// linked to them. The heading's number is the length of that list, so the two
// can never drift.
func TestHealthReachesEveryOutOfEnumNote(t *testing.T) {
	t.Parallel()
	srv := newServerWithContract(t, outOfEnumVault(t), loadHomeContract(t))
	code, body := get(t, srv.URL+"/health")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	section := healthSectionBody(t, body, "狀態值不在允許清單的筆記")

	for _, want := range []string{
		`href="/notes/Concepts/outside.md"`,
		`href="/notes/Concepts/outside-too.md"`,
	} {
		if !strings.Contains(section, want) {
			t.Errorf("the section does not reach the offending note %q; section = %q", want, section)
		}
	}
	// The control: the folder's declared note must not be dragged in with them.
	if strings.Contains(section, `href="/notes/Concepts/legal.md"`) {
		t.Errorf("a note whose status is declared is listed as a finding; section = %q", section)
	}
	// The offending value is named beside each row: the reader has to know
	// which word to edit, and the list is the only place that says it.
	if got := strings.Count(section, "reviewing"); got != 2 {
		t.Errorf("the section names the offending value %d times, want once per row (2); section = %q", got, section)
	}
	// One note, one name: these rows name notes exactly as every other section
	// of this page does — by file name — so the same note cannot appear here
	// as "Outside" and in the island list below as "outside". A title is not a
	// name this vault resolves links by, and the section above this one exists
	// to report readers who believed otherwise.
	for _, want := range []string{">outside</a>", ">outside-too</a>"} {
		if !strings.Contains(section, want) {
			t.Errorf("the row does not name the note the way the rest of the page does (%q missing); section = %q", want, section)
		}
	}

	// One derivation: the heading counts the rows it renders.
	rows := strings.Count(section, "<li>")
	if !strings.Contains(section, ">"+strconv.Itoa(rows)+"</span>") {
		t.Errorf("the heading's number is not the %d rows below it; section = %q", rows, section)
	}
}

// homeRecentRow cuts the recent-changes row for one note title out of a
// rendered Home body, so an assertion about one row cannot be answered by
// another row on the same page.
func homeRecentRow(t *testing.T, body, title string) string {
	t.Helper()
	for _, chunk := range strings.Split(body, `<a class="y-homenote"`)[1:] {
		row, _, terminated := strings.Cut(chunk, "</a>")
		if !terminated {
			t.Fatalf("unterminated recent row: %q", chunk)
		}
		if strings.Contains(row, ">"+title+"</span>") {
			return row
		}
	}
	t.Fatalf("the recent list has no row for %q; body = %q", title, body)
	return ""
}

// visibleOnly strips the stretches carried out of sight, so an assertion about
// what a reader sees cannot be satisfied by text placed where only a screen
// reader reaches it.
func visibleOnly(markup string) string {
	var b strings.Builder
	rest := markup
	for {
		before, after, found := strings.Cut(rest, `<span class="y-offscreen">`)
		b.WriteString(before)
		if !found {
			return b.String()
		}
		_, rest, found = strings.Cut(after, "</span>")
		if !found {
			return b.String()
		}
	}
}

// The distribution chip flagged a value no carrier declares with an amber
// edge and carried the words explaining it out of sight, so a reader looking
// straight at it saw a colour and nothing else — and a colour is not a
// statement. The words are on the page now, in the same form the search row
// and the note page use.
func TestHomeChipStatesItsFlagInWordsAReaderCanSee(t *testing.T) {
	t.Parallel()
	srv := newServerWithContract(t, outOfEnumVault(t), loadHomeContract(t))
	code, body := get(t, srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}

	reviewing := homeChip(t, body, "reviewing")
	if !strings.Contains(visibleOnly(reviewing), "不在 schema 允許清單中") {
		t.Errorf("the flagged chip says nothing a reader can see; chip = %q", reviewing)
	}
	// The control: a declared status gains no such words, so the assertion
	// above cannot be satisfied by a line the page prints beside every chip.
	if strings.Contains(visibleOnly(homeChip(t, body, "draft")), "不在 schema 允許清單中") {
		t.Errorf("a declared status is accused; chip = %q", homeChip(t, body, "draft"))
	}
}

// The recent list showed a note's status as an ordinary chip whatever the
// contract said about it, so the value the whole-folder page counts as a fault
// sat on the landing page looking exactly like every legal one beside it.
func TestHomeRecentRowNamesAStatusOutsideItsTypesEnum(t *testing.T) {
	t.Parallel()
	srv := newServerWithContract(t, outOfEnumVault(t), loadHomeContract(t))
	code, body := get(t, srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}

	outside := homeRecentRow(t, body, "Outside")
	if !strings.Contains(visibleOnly(outside), "不在 schema 允許清單中") {
		t.Errorf("the row shows a value the schema disallows as ordinary vocabulary; row = %q", outside)
	}
	if !strings.Contains(outside, "reviewing") {
		t.Errorf("the flagged row lost the value it is flagging; row = %q", outside)
	}

	legal := homeRecentRow(t, body, "Legal")
	if strings.Contains(legal, "不在 schema 允許清單中") {
		t.Errorf("a declared status is flagged; row = %q", legal)
	}
	if !strings.Contains(legal, "draft") {
		t.Errorf("the declared row lost its status; row = %q", legal)
	}
}

// A folder with no contract has no vocabulary to measure against, so the
// landing page names statuses there without ruling on any of them — the same
// restraint the search row and the whole-folder page keep.
func TestHomeRecentRowAccusesNothingWithoutAContract(t *testing.T) {
	t.Parallel()
	srv := newServer(t, outOfEnumVault(t))
	code, body := get(t, srv.URL+"/")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if strings.Contains(body, "不在 schema 允許清單中") {
		t.Errorf("an ungoverned folder ruled on a status value; body = %q", body)
	}
	// The control: the rows are on the page, so the assertion above is not
	// passing over a landing page that rendered nothing at all.
	if !strings.Contains(body, `data-home-recent-note`) {
		t.Errorf("the recent list is missing, so the assertion above proves nothing; body = %q", body)
	}
}
