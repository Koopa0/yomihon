package judge

import (
	"bytes"
	"testing"
)

// hiddenTitleNotes is a vault whose notes each carry a heading-shaped line the
// reader is never shown, inside an Obsidian comment or a math block, beside the
// heading they are titled by.
var hiddenTitleNotes = map[string]string{
	"Notes/Commented.md": "---\ntype: note\n---\n%%\n# secret draft heading\n%%\n\n# Real Title\n\nBody.\n",
	"Notes/Maths.md":     "---\ntype: note\n---\n$$\n# not a heading\n$$\n\n# Other Title\n\nBody.\n",
	"Notes/Links.md":     "---\ntype: note\n---\n# Links\n\n[[Real Title]] [[secret draft heading]] [[not a heading]]\n",
	"Notes/Overlap.md":   "---\ntype: note\n---\n# Overlap\n\n%%\n# Real Title\n%%\n",
}

// TestExistsAnswersToTheTitleTheReaderSees pins exists to the title a note is
// shown with: a title hidden in a comment or a math block neither matches a
// name nor leaves the note unfindable by the title the page prints, and the
// words of a hidden heading never reach the answer.
func TestExistsAnswersToTheTitleTheReaderSees(t *testing.T) {
	t.Parallel()
	root := writeLeftoverLockVault(t, leftoverLockContract, hiddenTitleNotes)

	tests := []struct {
		name      string
		search    string
		wantExit  int
		wantPath  string
		wantField string
	}{
		{name: "the heading shown", search: "Real Title", wantExit: 0, wantPath: "Notes/Commented.md", wantField: `"field":"title"`},
		{name: "the heading shown beside a math block", search: "Other Title", wantExit: 0, wantPath: "Notes/Maths.md", wantField: `"field":"title"`},
		{name: "a heading hidden in a comment", search: "secret draft heading", wantExit: 1},
		{name: "a heading hidden in a math block", search: "not a heading", wantExit: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			out, exit, err := RunExists(t.Context(), &ExistsOptions{Root: root, Name: tt.search, Format: FormatJSON})
			if err != nil {
				t.Fatalf("RunExists(%q) error = %v", tt.search, err)
			}
			if exit != tt.wantExit {
				t.Fatalf("RunExists(%q) exit = %d, want %d\n%s", tt.search, exit, tt.wantExit, out)
			}
			if tt.wantExit == 0 {
				for _, want := range []string{tt.wantPath, tt.wantField} {
					if !bytes.Contains(out, []byte(want)) {
						t.Errorf("RunExists(%q) = %s, want it to carry %s", tt.search, out, want)
					}
				}
			}
			for _, leaked := range []string{"secret draft heading", "not a heading"} {
				if tt.search != leaked && bytes.Contains(out, []byte(leaked)) {
					t.Errorf("RunExists(%q) carried the hidden heading %q:\n%s", tt.search, leaked, out)
				}
			}
		})
	}
}

// TestCheckReadsTheTitleTheReaderSees runs the same notes through check: a
// link to a name only a hidden heading holds stays broken, a link to the
// heading shown is told it names a title, and a comment repeating another
// note's title does not put the two in collision.
func TestCheckReadsTheTitleTheReaderSees(t *testing.T) {
	t.Parallel()
	root := writeLeftoverLockVault(t, leftoverLockContract, hiddenTitleNotes)
	findings, err := Check(t.Context(), root)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	rules := make(map[string]Finding)
	for i := range findings {
		f := findings[i]
		if f.Target == nil || f.Path != "Notes/Links.md" {
			continue
		}
		rules[*f.Target] = f
	}
	for _, target := range []string{"secret draft heading", "not a heading"} {
		got, ok := rules[target]
		if !ok || got.RuleID != "link.broken" {
			t.Errorf("link [[%s]] (a hidden heading) drew %+v, want link.broken", target, got)
		}
	}
	got, ok := rules["Real Title"]
	if !ok || got.RuleID != "link.title_not_alias" {
		t.Fatalf("link [[Real Title]] drew %+v, want link.title_not_alias", got)
	}
	if len(got.CollisionMembers) != 0 {
		t.Errorf("title Real Title is held by %v, want one holder: the copy in Notes/Overlap.md is in a comment", got.CollisionMembers)
	}
	if want := "the target is the title of Notes/Commented.md but not one of its aliases"; got.Evidence != want {
		t.Errorf("link [[Real Title]] evidence = %q, want %q", got.Evidence, want)
	}
}
