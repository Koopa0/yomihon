package note_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// probeStatuses are seven notes alike but for how each writes its status, which
// is the whole point: the page owes each shape a different sentence, and the
// two ordinary-text notes are the controls — an assertion about the others
// means nothing unless a note that reads normally behaves differently under the
// same fixture.
var probeStatuses = []struct {
	name    string
	written string
	// says is the sentence the status panel owes this shape. Measured against
	// this repository's YAML reader rather than assumed: "no" is text under the
	// YAML 1.2 core schema, which is why it sits beside the quoted one as a
	// control and not beside the dates.
	says statusSentence
	// quoted is whether the judging side quotes the value back in the panel
	// under it. It does so for a scalar it can read as characters; for a list
	// it reports nothing about the status at all, and for a mapping it reports
	// the status as one the note did not write.
	quoted bool
}{
	{name: "date", written: "2026-08-30", says: sentenceNotText, quoted: true},
	{name: "number", written: "12345", says: sentenceNotText, quoted: true},
	{name: "float", written: "1.5", says: sentenceNotText, quoted: true},
	{name: "quoted", written: `"nearly-ready"`, says: sentenceValue, quoted: true},
	{name: "boolish", written: "no", says: sentenceValue, quoted: true},
	{name: "list", written: "\n  - draft\n  - ready", says: sentenceNotSingle},
	{name: "mapping", written: "\n  a: b", says: sentenceNotSingle},
}

// statusSentence is which of the three things the status panel says about a
// note's status: it carries the value, it says the value is not text, or it
// says nothing single was written there.
type statusSentence int

const (
	sentenceValue statusSentence = iota
	sentenceNotText
	sentenceNotSingle
)

// TestTheTwoPanelsAgreeAboutAStatusThatIsNotText holds the reading page and the
// judging commands to one account of one field. They read the same file through
// different readers: this side asks YAML for a Go value and gets a date back
// where the author typed an unquoted one, the judging side takes the scalar as
// the characters typed. The page therefore used to report the field as one it
// could not read at all — the same sentence a note that wrote no status gets —
// directly above a schema diagnostic quoting the value the author wrote.
//
// The two are not required to say the same words. They are required not to
// disagree about whether the note wrote anything: a reader looking at "no
// status value could be read" and "status is written as 2026-08-30" on one page
// has no way to tell which panel to believe.
func TestTheTwoPanelsAgreeAboutAStatusThatIsNotText(t *testing.T) {
	t.Parallel()

	const (
		notText    = "那個值不是文字"
		unreadable = "frontmatter 裡讀不出 status 值"
	)

	root := t.TempDir()
	dir := filepath.Join(root, "Concepts", "golang")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for _, probe := range probeStatuses {
		note := "---\ntitle: " + probe.name + " probe\ntype: concept\ndomain: golang\nstatus: " + probe.written + "\n---\n\nbody\n"
		if err := os.WriteFile(filepath.Join(dir, probe.name+".md"), []byte(note), 0o600); err != nil {
			t.Fatalf("write %s: %v", probe.name, err)
		}
	}

	srv := newServerWithContract(t, root, loadHomeContract(t))
	for _, probe := range probeStatuses {
		t.Run(probe.name, func(t *testing.T) {
			t.Parallel()
			_, page := get(t, srv.Client(), srv.URL+"/notes/Concepts/golang/"+probe.name+".md")

			// Where the judging side quotes the value, say so first: without
			// it the assertions below would hold on a page that had stopped
			// judging the note at all.
			if probe.quoted && !strings.Contains(page, "不在 schema 的允許清單裡") {
				t.Fatal("the page carries no schema diagnostic for a status outside the declared list")
			}

			switch probe.says {
			case sentenceValue:
				if !strings.Contains(page, probe.written) {
					t.Errorf("the page never quotes the status the note wrote (%s)", probe.written)
				}
				if strings.Contains(page, notText) || strings.Contains(page, unreadable) {
					t.Error("a status written as text is reported as one that could not be read")
				}
			case sentenceNotText:
				if !strings.Contains(page, notText) {
					t.Error("the page does not say the status was written as something other than text")
				}
				if strings.Contains(page, unreadable) {
					t.Error("the page still reports a written status as one the note never wrote")
				}
			case sentenceNotSingle:
				// A list and a mapping are not one value, and the judging side
				// quotes neither — it says nothing at all about a list's status
				// and reports a mapping's as absent. Saying "not text" here
				// would promise a diagnostic quoting a value that no panel
				// carries, which is the same fault this test exists for,
				// inverted.
				if !strings.Contains(page, unreadable) {
					t.Error("the page does not say that nothing single was written where the status goes")
				}
				if strings.Contains(page, notText) {
					t.Error("a shape no panel quotes is reported as a value that is not text")
				}
			}
		})
	}
}
