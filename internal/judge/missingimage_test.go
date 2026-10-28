package judge

import (
	"bytes"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func missingImageVault(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/vault-missing-images")); err != nil {
		t.Fatalf("copy missing image fixture: %v", err)
	}
	return root
}

// TestMissingImageFindings pins the fixture's findings to the golden twice: as
// decoded records, so a difference names the field, and as the exact bytes the
// pipelines read.
func TestMissingImageFindings(t *testing.T) {
	t.Parallel()
	root := missingImageVault(t)
	wantBytes, err := os.ReadFile("testdata/golden/missing-images.jsonl")
	if err != nil {
		t.Fatalf("read missing image golden: %v", err)
	}
	var want []Finding
	for line := range strings.SplitSeq(strings.TrimSuffix(string(wantBytes), "\n"), "\n") {
		var wire struct {
			Finding

			Severity string `json:"severity"`
		}
		if err := json.Unmarshal([]byte(line), &wire); err != nil {
			t.Fatalf("decode golden: %v", err)
		}
		if wire.Severity != "warn" {
			t.Fatalf("golden severity %q, want every record at warn", wire.Severity)
		}
		wire.Finding.Severity = SeverityWarn
		want = append(want, wire.Finding)
	}
	if len(want) != 8 {
		t.Fatalf("golden holds %d records, want 8", len(want))
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
	if _, exit, err := RunCheck(t.Context(), &CheckOptions{Root: root, Format: FormatJSON, Deny: []string{"link.broken.image"}}); err != nil || exit != 1 {
		t.Errorf("RunCheck --deny link.broken.image: exit=%d error=%v, want exit 1", exit, err)
	}
}

// TestMissingImageTargets pins which occurrences the rule reports, and that a
// missing file is reported once, by this rule, while a link that is not an
// embed keeps the broken-link rule's meaning.
func TestMissingImageTargets(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		body string
		hold []string
		want []string
	}{
		{
			name: "every reference form",
			body: "![full][id]\n![collapsed][]\n![shortcut]\n\n[id]: a.png\n[collapsed]: b.png\n[shortcut]: c.png\n",
			want: []string{"link.broken.image Notes/a.png 1", "link.broken.image Notes/b.png 2", "link.broken.image Notes/c.png 3"},
		},
		{
			name: "code, comments and escapes are not images",
			body: "`![](inline.png)`\n\n```md\n![](fenced.png)\n```\n\n%% ![](percent.png) %%\n<!-- ![](html.png) -->\n\\![](escaped.png)\n![](live.png)\n",
			want: []string{"link.broken.image Notes/live.png 10"},
		},
		{
			name: "rooted, climbing and suffixed destinations",
			body: "![](/root.png?size=2#view)\n![](../top.png)\n![](./dir/../next.png)\n",
			want: []string{"link.broken.image root.png 1", "link.broken.image top.png 2", "link.broken.image Notes/next.png 3"},
		},
		{
			name: "a held file under any spelling the page decodes",
			body: "![](held.png) ![](held%2Epng) ![](caf%C3%A9.png) ![](cafe%CC%81.png)\n![[held.png]] ![[Notes/held.png]]\n",
			hold: []string{"Notes/café.png"},
		},
		{
			name: "the same name elsewhere does not hold a relative path",
			body: "![](elsewhere.png)\n",
			hold: []string{"Other/elsewhere.png"},
			want: []string{"link.broken.image Notes/elsewhere.png 1"},
		},
		{
			name: "what the page does not show from the vault",
			body: "![](https://example.invalid/a.png)\n![](//example.invalid/a.png)\n![](data:image/png;base64,AAAA)\n![](/raw/a.png)\n![](../../outside.png)\n![](.hidden/a.png)\n![](bad%ZZ.png)\n",
		},
		{
			name: "an encoded percent names a literal one",
			body: "![](bad%25ZZ.png)\n",
			want: []string{"link.broken.image Notes/bad%ZZ.png 1"},
		},
		{
			name: "a shared file name is the collision rule's",
			body: "![[same.png]]\n",
			hold: []string{"A/same.png", "B/same.png"},
		},
		{
			name: "a file link that is not an embed stays a broken link",
			body: "[[gone.png]] ![[gone.png]]\n",
			want: []string{"link.broken gone.png 1", "link.broken.image gone.png 1"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := missingImageVault(t)
			write(t, root, "Notes/Images.md", tt.body)
			for _, path := range tt.hold {
				write(t, root, path, "held")
			}
			findings, err := Check(t.Context(), root)
			if err != nil {
				t.Fatalf("Check: %v", err)
			}
			var got []string
			for _, f := range findings {
				if !strings.HasPrefix(string(f.RuleID), "link.broken") {
					continue
				}
				if f.Target == nil || f.Line == nil {
					t.Fatalf("%s finding without target or line: %+v", f.RuleID, f)
				}
				got = append(got, string(f.RuleID)+" "+*f.Target+" "+strconv.Itoa(*f.Line))
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("link.broken findings (-want +got):\n%s", diff)
			}
		})
	}
}

// TestMissingImageWithheldGround pins that an image pointing under a directory
// the contract withholds says nothing either way: whether the file is there
// must not change a byte of the report, and the path must not appear in it.
func TestMissingImageWithheldGround(t *testing.T) {
	t.Parallel()
	for _, destination := range []string{
		"![](../Private/secret.png)",
		"![](/Priv%61te/secret.png)",
		"![](/Private/cafe%CC%81.png)",
		"![[Private/secret.png]]",
	} {
		t.Run(destination, func(t *testing.T) {
			t.Parallel()
			var reports [2][]byte
			for i, held := range []bool{false, true} {
				root := missingImageVault(t)
				write(t, root, "Notes/Images.md", "![](public.png) ![[public-wiki.png]]\n"+destination+"\n")
				write(t, root, "Private/Author.md", "![](private-author.png)\n")
				if held {
					write(t, root, "Private/secret.png", "held")
					write(t, root, "Private/café.png", "held")
				}
				out, _, err := RunCheck(t.Context(), &CheckOptions{Root: root, All: true, Format: FormatJSON})
				if err != nil {
					t.Fatalf("RunCheck: %v", err)
				}
				reports[i] = out
			}
			if !bytes.Contains(reports[0], []byte(`"target":"Notes/public.png"`)) || !bytes.Contains(reports[0], []byte(`"target":"public-wiki.png"`)) {
				t.Fatalf("the public images beside the withheld one went unreported: %s", reports[0])
			}
			if !bytes.Equal(reports[0], reports[1]) {
				t.Errorf("the report depends on a withheld file:\nabsent  %s\npresent %s", reports[0], reports[1])
			}
			for _, forbidden := range []string{"Private", "secret", "private-author", "caf"} {
				if bytes.Contains(reports[0], []byte(forbidden)) {
					t.Errorf("the report names withheld ground %q: %s", forbidden, reports[0])
				}
			}
		})
	}
}

// TestMissingImagePartialCorpus pins the split a run that could not read a
// file makes: an embed's name could be that file's alias, so it is withheld,
// while a Markdown image names a path the completed scan already answers.
func TestMissingImagePartialCorpus(t *testing.T) {
	t.Parallel()
	root := missingImageVault(t)
	write(t, root, "Notes/Images.md", "![](direct.png)\n![[alias.png]]\n")
	write(t, root, "Notes/Unread.md", "---\naliases: [alias.png]\n---\nbody\n")
	a, err := openAction(t.Context(), root, actionHooks{parseNote: func(rel string, data []byte, marks plannedMarks) note {
		if rel == "Notes/Unread.md" {
			panic("controlled unreadable alias source")
		}
		return parseNoteWithMarks(rel, data, marks)
	}})
	if err != nil {
		t.Fatalf("openAction: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := a.close(); closeErr != nil {
			t.Errorf("close action: %v", closeErr)
		}
	})
	if !a.partialCorpus() {
		t.Fatal("the controlled panic left the corpus whole")
	}
	findings, err := checkAction(a, nil, false)
	if err != nil {
		t.Fatalf("checkAction: %v", err)
	}
	var targets []string
	for _, f := range findings {
		if f.RuleID == "link.broken.image" && f.Target != nil {
			targets = append(targets, *f.Target)
		}
	}
	if diff := cmp.Diff([]string{"Notes/direct.png"}, targets); diff != "" {
		t.Errorf("link.broken.image targets on a partial corpus (-want +got):\n%s", diff)
	}
}
