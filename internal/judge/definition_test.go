package judge

import (
	"bytes"
	"encoding/json"
	"os"
	"slices"
	"strconv"
	"strings"
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

// TestUnusedDefinitionFindings pins the fixture's findings to the golden twice:
// as decoded records, so a difference names the field, and as the exact bytes
// the pipelines read.
func TestUnusedDefinitionFindings(t *testing.T) {
	t.Parallel()
	root := unusedDefinitionVault(t)
	wantBytes, err := os.ReadFile("testdata/golden/unused-definitions.jsonl")
	if err != nil {
		t.Fatalf("read unused definition golden: %v", err)
	}
	var want []Finding
	for line := range strings.SplitSeq(strings.TrimSuffix(string(wantBytes), "\n"), "\n") {
		var wire struct {
			Finding

			Severity string `json:"severity"`
		}
		if decodeErr := json.Unmarshal([]byte(line), &wire); decodeErr != nil {
			t.Fatalf("decode golden: %v", decodeErr)
		}
		if wire.Severity != "warn" {
			t.Fatalf("golden severity %q, want every record at warn", wire.Severity)
		}
		wire.Finding.Severity = SeverityWarn
		want = append(want, wire.Finding)
	}
	if len(want) != 3 {
		t.Fatalf("golden holds %d records, want 3", len(want))
	}
	got, err := Check(t.Context(), root)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Check findings (-want +got):\n%s", diff)
	}
	gotBytes, exit, err := RunCheck(t.Context(), &CheckOptions{Root: root, Format: FormatJSON})
	if err != nil || exit != 0 {
		t.Fatalf("RunCheck: exit=%d error=%v", exit, err)
	}
	if !bytes.Equal(wantBytes, gotBytes) {
		t.Errorf("RunCheck JSONL:\nwant %s\ngot  %s", wantBytes, gotBytes)
	}
	_, exit, err = RunCheck(t.Context(), &CheckOptions{Root: root, Format: FormatJSON, Deny: []string{"link.unused.definition"}})
	if err != nil || exit != 1 {
		t.Errorf("RunCheck --deny link.unused.definition: exit=%d error=%v, want exit 1", exit, err)
	}
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
			findings, err := Check(t.Context(), root)
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

// TestUnusedDefinitionScope pins that the rule goes through the filters every
// finding does: the default scope keeps the knowledge layer, all widens it, and
// ground the contract withholds is never named in either.
func TestUnusedDefinitionScope(t *testing.T) {
	t.Parallel()
	root := unusedDefinitionVault(t)
	write(t, root, "Notes/Refs.md", "[kept]: https://example.com\n")
	write(t, root, "Other/Outside.md", "[outside]: https://example.com\n")
	write(t, root, "Private/Author.md", "[secret]: https://example.com\n")
	for _, tt := range []struct {
		name    string
		all     bool
		present []string
	}{
		{name: "default scope", present: []string{`"target":"kept"`}},
		{name: "all", all: true, present: []string{`"target":"kept"`, `"target":"outside"`}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			out, _, err := RunCheck(t.Context(), &CheckOptions{Root: root, All: tt.all, Format: FormatJSON})
			if err != nil {
				t.Fatalf("RunCheck: %v", err)
			}
			for _, needle := range []string{`"target":"kept"`, `"target":"outside"`} {
				if got, want := bytes.Contains(out, []byte(needle)), slices.Contains(tt.present, needle); got != want {
					t.Errorf("report contains %s = %v, want %v:\n%s", needle, got, want, out)
				}
			}
			for _, forbidden := range []string{"Private", "secret"} {
				if bytes.Contains(out, []byte(forbidden)) {
					t.Errorf("the report names withheld ground %q: %s", forbidden, out)
				}
			}
		})
	}
}
