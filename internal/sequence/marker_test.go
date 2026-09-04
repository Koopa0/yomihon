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
