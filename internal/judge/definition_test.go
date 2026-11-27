package judge

import (
	"os"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func unusedDefinitionVault(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/vault-unused-definitions")); err != nil {
		t.Fatalf("copy unused definition fixture: %v", err)
	}
	return root
}

// TestUnusedDefinitions pins which definitions the rule reports: those the
// parser resolved no link or image through, whatever form the reference took.
func TestUnusedDefinitions(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		body string
		want []string
	}{
		{
			name: "a definition, a renamed use and an adjacent pair",
			body: "# Refs\n\n[壓力測試] and [the guide][guide].\n\nTwo sources. [E1][E2]\n\n[壓測]: https://example.com/a\n[guide-old]: https://example.com/b\n[E1]: https://example.com/e1\n[E2]: https://example.com/e2\n",
			want: []string{"壓測 7", "guide-old 8", "E1 9"},
		},
		{
			name: "a space between adjacent references uses both",
			body: "Two sources. [E1] [E2]\n\n[E1]: https://example.com/e1\n[E2]: https://example.com/e2\n",
		},
		{
			name: "every reference form and an image use their definition",
			body: "[full][id] [collapsed][] [shortcut] ![pic][img]\n\n[id]: a\n[collapsed]: b\n[shortcut]: c\n[img]: d.png\n",
		},
		{
			name: "the label folds case as the page does",
			body: "see [x][FOLD case]\n\n[Fold Case]: https://example.com\n",
		},
		{
			name: "a footnote is not a link definition",
			body: "Text[^n].\n\n[^n]: note\n[^lone]: cited nowhere\n",
		},
		{
			name: "the first of two definitions of a label is the one judged",
			body: "[a]: first\n[a]: second\n",
			want: []string{"a 1"},
		},
		{
			name: "two definitions of a used label are used",
			body: "[a]\n\n[a]: first\n[a]: second\n",
		},
		{
			name: "code is not a definition",
			body: "```\n[a]: https://example.com\n```\n\n    [b]: https://example.com\n",
		},
		{
			name: "a reference in a comment does not use a definition, and a definition in one is not judged",
			body: "[a]: u1\n[b]: u2\n\n%% see [a] %%\n\n%%\n\n[c]: u3\n\n%%\n",
			want: []string{"a 1", "b 2"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := unusedDefinitionVault(t)
			write(t, root, "Notes/Refs.md", tt.body)
			findings, err := runCheckAction(t.Context(), root, nil, false)
			if err != nil {
				t.Fatalf("Check: %v", err)
			}
			var got []string
			for _, f := range findings {
				if f.RuleID != "link.unused.definition" {
					continue
				}
				if f.Target == nil || f.Line == nil {
					t.Fatalf("%s finding without target or line: %+v", f.RuleID, f)
				}
				got = append(got, *f.Target+" "+strconv.Itoa(*f.Line))
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("link.unused.definition findings (-want +got):\n%s", diff)
			}
		})
	}
}
