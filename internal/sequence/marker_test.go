package sequence

import (
	"strings"
	"testing"
)

// A branch's name is read in two places that cannot be allowed to drift: here,
// where the course lists the branch under it, and on the reading page, which
// shows the same heading, lists it in the contents and stamps the id a citation
// reaches it by. The page cannot ask the parser — it works over assembled
// markup that no one parse tree holds — so it asks [HeadingName], and this
// holds that answer to the one a parse of the same heading gives.
func TestHeadingNameAnswersWhatTheParserCallsTheBranch(t *testing.T) {
	t.Parallel()

	headings := []string{
		"基本觀念 {sequence=primary}",
		"Doing it {sequence=local}",
		"不分先後 {sequence=none}",
		"沒有宣告",
		"宣告 `{sequence=primary}`",
		"{sequence=primary} 開頭",
		"基本觀念 {sequence=whatever}",
		"基本觀念 {sequence=primary} {sequence=local}",
		"基本觀念 {sequence}",
	}
	for _, heading := range headings {
		t.Run(heading, func(t *testing.T) {
			t.Parallel()
			body := "## " + heading + "\n\n- [[L01]]\n"
			doc := Parse(body, 1)
			if len(doc.Groups) != 1 {
				t.Fatalf("Parse(%q) opened %d branches, want one", body, len(doc.Groups))
			}
			if got, want := strings.TrimSpace(HeadingName(heading, 2)), doc.Groups[0].Name; got != want {
				t.Errorf("HeadingName(%q, 2) = %q; the parser calls that branch %q", heading, got, want)
			}
		})
	}
}

// A level-1 heading is the note's own title, not a part of the course, so a
// role written on one declares nothing and is reported. The words the report
// quotes have to stay where the author can see them, which is why the level is
// asked for rather than assumed.
func TestHeadingNameLeavesALevelOneHeadingWhole(t *testing.T) {
	t.Parallel()

	heading := "基本觀念 {sequence=primary}"
	if got := HeadingName(heading, 1); got != heading {
		t.Errorf("HeadingName(%q, 1) = %q, want the heading as written", heading, got)
	}

	doc := Parse("# "+heading+"\n\n- [[L01]]\n", 1)
	var quoted bool
	for _, d := range doc.Diagnostics {
		if d.Rule == RuleRoleMisplaced && d.Evidence == heading {
			quoted = true
		}
	}
	if !quoted {
		t.Errorf("no report quotes %q, so nothing depends on it staying visible: %+v", heading, doc.Diagnostics)
	}
}

// A heading that is nothing but a declaration has no words behind it, and a
// page cannot show a reader nothing: with the declaration off it would print a
// blank heading, list a blank row in the contents, and answer only to the name
// every nameless heading shares. So the line stays as written. This is the one
// shape where the page's name for a branch and the course's part company, and
// it parts because the course has no name to list, not because the two read
// the line differently.
func TestAHeadingThatIsOnlyADeclarationKeepsItsLine(t *testing.T) {
	t.Parallel()

	headings := []string{
		"{sequence=primary}",
		"　{sequence=primary}",
	}
	for _, heading := range headings {
		t.Run(heading, func(t *testing.T) {
			t.Parallel()
			if got := HeadingName(heading, 2); got != heading {
				t.Errorf("HeadingName(%q, 2) = %q, want the heading as written", heading, got)
			}
			doc := Parse("## "+heading+"\n\n- [[L01]]\n", 1)
			if len(doc.Groups) != 1 {
				t.Fatalf("the parse opened %d branches, want one", len(doc.Groups))
			}
			if doc.Groups[0].Name != "" {
				t.Errorf("the course calls that branch %q; this rule exists because it has no name", doc.Groups[0].Name)
			}
		})
	}
}

// A branch's name ends with the author's last word. The space they left in
// front of the declaration is spacing, and a name carrying it would fold to
// the same id anyway while every report quoting it showed the gap. Until now
// only a reading-page test noticed the trim, and the page is not where the
// rule lives.
func TestABranchNameEndsWithItsLastWord(t *testing.T) {
	t.Parallel()

	headings := []string{
		"基本觀念 {sequence=primary}",
		"基本觀念   {sequence=primary}",
		"基本觀念\t{sequence=primary}",
		"基本觀念 \t {sequence=primary}",
	}
	for _, heading := range headings {
		t.Run(heading, func(t *testing.T) {
			t.Parallel()
			if got, want := HeadingName(heading, 2), "基本觀念"; got != want {
				t.Errorf("HeadingName(%q, 2) = %q, want %q", heading, got, want)
			}
		})
	}
}

// One vocabulary, written and read. The syllabus shows an author the three
// declarations it will accept, and this package spells them; the parser reads
// a declaration back off a line. A round trip holds the two halves together,
// because a page offering a marker the parser rejects would be teaching the
// author to write a fault.
func TestAMarkerThisPackageWritesIsOneItReads(t *testing.T) {
	t.Parallel()

	for _, role := range []Role{RolePrimary, RoleLocal, RoleNone} {
		marker := Marker(role)
		got, ok := markerValue(marker)
		if !ok || got != role {
			t.Errorf("markerValue(Marker(%v)) = %v, %t, want %v, true", role, got, ok, role)
		}
		heading := "標題 " + marker
		if name := HeadingName(heading, 2); name != "標題" {
			t.Errorf("HeadingName(%q, 2) = %q, want the branch called 標題", heading, name)
		}
	}
	// A branch that declared nothing carries one of these, and no line can say
	// either: offering one as a declaration would name a value the parser reports.
	for _, role := range []Role{RoleStructural, RoleUnclassified} {
		if got := Marker(role); got != "" {
			t.Errorf("Marker(%v) = %q, want no marker at all", role, got)
		}
	}
}

// The three values an author may write are the parser's own, and they are shown
// to a reader twice on the same page: once as the whole declaration to copy,
// built from those values, and once as the bare value, taken from Role.String()
// — which wrote them out again by hand. Changing one spelling left the other
// teaching the reader a value this parser rejects, and nothing in this package
// noticed: only a handful of display-string assertions in other packages did,
// which is the wrong place to hear it.
func TestADeclarableRoleSpellsItselfTheWayTheParserReadsIt(t *testing.T) {
	t.Parallel()

	declarable := []Role{RolePrimary, RoleLocal, RoleNone}
	for _, role := range declarable {
		marker := Marker(role)
		if marker == "" {
			t.Errorf("%v is declarable and has no marker to write", role)
			continue
		}
		read, ok := markerValue(marker)
		if !ok || read != role {
			t.Errorf("the parser reads %q as (%v, %v), want (%v, true)", marker, read, ok, role)
		}
		inner := strings.TrimSuffix(strings.TrimPrefix(marker, markerOpen+"="), markerClose)
		if name := role.String(); name != inner {
			t.Errorf("%v is shown as %q and read as %q; a reader shown one and parsed by the other cannot write a marker that works", role, name, inner)
		}
	}

	// The two a branch is left with when it declared nothing have no spelling,
	// so their names answer to nobody and are free to read as they like.
	for _, role := range []Role{RoleStructural, RoleUnclassified} {
		if marker := Marker(role); marker != "" {
			t.Errorf("%v cannot be declared and offers the marker %q", role, marker)
		}
	}
}
