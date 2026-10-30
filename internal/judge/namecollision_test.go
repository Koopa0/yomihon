package judge

import (
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// nameCollisionTargets runs the whole-vault check over root and returns the
// name of every collision.name finding, in report order.
func nameCollisionTargets(tb testing.TB, root string) []string {
	tb.Helper()
	findings, err := runCheckAction(tb.Context(), root, nil, true)
	if err != nil {
		tb.Fatalf("runCheckAction(%q) error = %v", root, err)
	}
	var names []string
	for i := range findings {
		if findings[i].RuleID != "collision.name" {
			continue
		}
		if findings[i].Target == nil {
			tb.Fatalf("collision.name finding has no target: %+v", findings[i])
		}
		names = append(names, *findings[i].Target)
	}
	return names
}

// TestNameCollisionNeedsALinkWrittenAgainstTheName holds the point of the rule:
// two files sharing a name break nothing until a link resolves through it. A
// course vault repeats main.go in every lab and links each copy by its path;
// reporting the repeated name would draw a finding per file name and bury the
// rest. The control cases show the same vault does report once a wikilink or an
// embed names the file, so a silent run cannot be a rule that never fires.
func TestNameCollisionNeedsALinkWrittenAgainstTheName(t *testing.T) {
	t.Parallel()

	const (
		lessonHead = "---\ntitle: Lesson\ntype: note\nstatus: draft\n---\n\n# Lesson\n\n"
		pathLinks  = "The program is [lab A](lab/a/main.go) and the second is [lab B](lab/b/main.go).\n"
	)
	tests := []struct {
		name    string
		lesson  string
		private string // when set, the lesson is filed under the withheld directory
		partial bool   // when set, one note exists that cannot be read
		want    []string
	}{
		{name: "markdown path links resolve by path, not by name", lesson: lessonHead + pathLinks},
		{name: "a wikilink to the shared name", lesson: lessonHead + pathLinks + "\nSee [[main.go]].\n", want: []string{"main.go"}},
		{name: "an embed of the shared name", lesson: lessonHead + pathLinks + "\n![[main.go]]\n", want: []string{"main.go"}},
		{name: "the link folds like the index", lesson: lessonHead + "See [[MAIN.GO#run]].\n", want: []string{"main.go"}},
		{name: "a wikilink to a path resolves uniquely", lesson: lessonHead + "See [[lab/a/main.go]].\n"},
		{name: "a link quoted in a code fence is not written", lesson: lessonHead + "```\n[[main.go]]\n```\n"},
		{
			name:    "a link only a withheld note writes does not make the name reportable",
			lesson:  lessonHead + "See [[main.go]].\n",
			private: "Private",
		},
		{
			name:    "a note nobody could read may hold the link",
			lesson:  lessonHead + pathLinks,
			partial: true,
			want:    []string{"main.go"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			lessonAt := "Notes/Lesson.md"
			var private []string
			if tt.private != "" {
				lessonAt = tt.private + "/Lesson.md"
				private = []string{tt.private}
			}
			write(t, root, lessonAt, tt.lesson)
			write(t, root, "Notes/lab/a/main.go", "package main\n")
			write(t, root, "Notes/lab/b/main.go", "package main\n")
			if tt.partial {
				write(t, root, "Notes/Sealed.md", "---\ntitle: Sealed\n---\n")
			}
			writeTestContract(t, root, private)
			if tt.partial && !unreadable(t, filepath.Join(root, "Notes", "Sealed.md")) {
				t.Fatal("this process can still read a file it took every permission from, so no vault here has a hole in it to judge")
			}

			got := nameCollisionTargets(t, root)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("collision.name targets mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestNameCollisionReportsOnlyTheSpellingsALinkUses pins how the extension
// pair is settled. Two notes sharing a filename share its extension-less form
// too, and the pair is one repair; which spelling the finding carries is the
// one a link wrote, and a link to the other spelling alone must still be
// reported rather than lost to the restatement drop.
func TestNameCollisionReportsOnlyTheSpellingsALinkUses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cite string
		want []string
	}{
		{name: "the stem", cite: "[[dup]]", want: []string{"dup"}},
		{name: "the filename", cite: "[[dup.md]]", want: []string{"dup.md"}},
		{name: "both spellings are one repair", cite: "[[dup]] and [[dup.md]]", want: []string{"dup"}},
		{name: "neither", cite: "nothing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			write(t, root, "Notes/dup.md", "---\ntitle: One\n---\n")
			write(t, root, "Lessons/dup.md", "---\ntitle: Two\n---\n")
			write(t, root, "Notes/cite.md", "---\ntitle: Cite\n---\n\n"+tt.cite+"\n")
			writeTestContract(t, root, nil)

			got := nameCollisionTargets(t, root)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("collision.name targets mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
