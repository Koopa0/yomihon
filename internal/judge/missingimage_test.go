package judge

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
)

func missingImageVault(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/vault-missing-images")); err != nil {
		t.Fatalf("copy missing image fixture: %v", err)
	}
	return root
}

func missingImageGolden(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/golden/missing-images.jsonl")
	if err != nil {
		t.Fatalf("read missing image golden: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("missing image golden is empty")
	}
	return data
}

func TestMissingImageFixtureAuthority(t *testing.T) {
	t.Parallel()
	source, err := os.ReadFile("../schema/testdata/contract.toml")
	if err != nil {
		t.Fatalf("read source contract: %v", err)
	}
	const original = `knowledge_dirs = ["Concepts", "Sources", "Maps", "Writing", "Synthesis", "Inbox"]`
	if strings.Count(string(source), original) != 1 {
		t.Fatal("source contract must contain exactly one knowledge directory declaration")
	}
	want := strings.Replace(string(source), original, `knowledge_dirs = ["Notes"]`, 1) + "\n[privacy]\nnever_egress_dirs = [\"Private\"]\n"
	got, err := os.ReadFile("testdata/vault-missing-images/System/schemas/vault-schema.toml")
	if err != nil {
		t.Fatalf("read isolated contract: %v", err)
	}
	if diff := cmp.Diff(want, string(got)); diff != "" {
		t.Errorf("isolated contract substitutions (-want +got):\n%s", diff)
	}
}

func TestMissingImageFindings(t *testing.T) {
	t.Parallel()
	root := missingImageVault(t)
	wantBytes := missingImageGolden(t)
	var want []Finding
	for line := range strings.SplitSeq(strings.TrimSuffix(string(wantBytes), "\n"), "\n") {
		var wire struct {
			Finding

			Severity string `json:"severity"`
		}
		if err := json.Unmarshal([]byte(line), &wire); err != nil {
			t.Fatalf("decode literal golden: %v", err)
		}
		switch wire.Severity {
		case "error":
			wire.Finding.Severity = SeverityError
		case "warn":
			wire.Finding.Severity = SeverityWarn
		case "info":
			wire.Finding.Severity = SeverityInfo
		default:
			t.Fatalf("unknown literal golden severity %q", wire.Severity)
		}
		want = append(want, wire.Finding)
	}
	if len(want) != 8 {
		t.Fatalf("literal golden holds %d records, want 8", len(want))
	}
	got, err := Check(t.Context(), root)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	t.Log("invoked: missing-image occurrence contract")
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("caught: missing-image occurrence contract (-want +got):\n%s", diff)
	}
	gotBytes, exit, err := RunCheck(t.Context(), &CheckOptions{Root: root, Format: FormatJSON})
	if err != nil || exit != 0 {
		t.Fatalf("RunCheck: exit=%d error=%v", exit, err)
	}
	if !bytes.Equal(wantBytes, gotBytes) {
		t.Errorf("caught: missing-image occurrence contract JSONL:\nwant %s\ngot %s", wantBytes, gotBytes)
	}
}

func TestMissingImageOccurrences(t *testing.T) {
	t.Parallel()
	type occurrence struct {
		target string
		line   int
	}
	type classifiedOccurrence struct {
		rule     RuleID
		severity Severity
		target   *string
		line     *int
	}
	for _, tt := range []struct {
		name string
		body string
		want []occurrence
	}{
		{name: "empty and nested alt", body: "![](a.png) ![**words**](b.png)\n", want: []occurrence{{"Notes/a.png", 1}, {"Notes/b.png", 1}}},
		{name: "all reference forms", body: "![full][id]\n![collapsed][]\n![shortcut]\n\n[id]: a.png\n[collapsed]: b.png\n[shortcut]: c.png\n", want: []occurrence{{"Notes/a.png", 1}, {"Notes/b.png", 2}, {"Notes/c.png", 3}}},
		{name: "quoted list and duplicate", body: "> ![](a.png)\n\n- ![](b.png) ![](b.png)\n", want: []occurrence{{"Notes/a.png", 1}, {"Notes/b.png", 3}, {"Notes/b.png", 3}}},
		{name: "code and comments", body: "`![](inline.png)`\n\n```md\n![](fenced.png)\n```\n\n    ![](indented.png)\n\n%% ![](percent.png) %%\n<!-- ![](html.png) -->\n\\![](escaped.png)\n![](live.png)\n", want: []occurrence{{"Notes/live.png", 12}}},
		{name: "local image without recognized extension", body: "![](asset.bin)\n", want: []occurrence{{"Notes/asset.bin", 1}}},
		{name: "root and dot suffix", body: "![](/root.png?size=2#view)\n![](../top.png)\n![](./dir/../next.png)\n", want: []occurrence{{"root.png", 1}, {"top.png", 2}, {"Notes/next.png", 3}}},
		{name: "frontmatter line offset", body: "---\ntitle: Images\ntype: writing\ndomain: meta\nstatus: draft\ncreated: 2026-01-01\nupdated: 2026-01-01\n---\n![](a.png)\n", want: []occurrence{{"Notes/a.png", 9}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := missingImageVault(t)
			write(t, root, "Notes/Images.md", tt.body)
			got, err := Check(t.Context(), root)
			if err != nil {
				t.Fatalf("Check: %v", err)
			}
			t.Log("invoked: missing-image parsed occurrences")
			var actual, expected []classifiedOccurrence
			for i := range got {
				f := &got[i]
				actual = append(actual, classifiedOccurrence{rule: f.RuleID, severity: f.Severity, target: f.Target, line: f.Line})
			}
			for _, item := range tt.want {
				expected = append(expected, classifiedOccurrence{rule: "link.broken.image", severity: SeverityWarn, target: new(item.target), line: new(item.line)})
			}
			if diff := cmp.Diff(expected, actual, cmp.AllowUnexported(classifiedOccurrence{})); diff != "" {
				t.Errorf("caught: missing-image occurrence contract (-want +got):\n%s", diff)
			}
		})
	}
}

func TestMissingImageMembership(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		body string
		hold map[string]string
		want []string
	}{
		{name: "held percent decoded and NFC", body: "![](held.png) ![](held%2Epng) ![](caf%C3%A9.png) ![](cafe%CC%81.png)\n![[held.png]]\n", hold: map[string]string{"Notes/caf\u00e9.png": "held"}},
		{name: "directory membership", body: "![](folder)\n", hold: map[string]string{"Notes/folder/held.png": "held"}},
		{name: "same basename elsewhere is not rescue", body: "![](elsewhere.png)\n", hold: map[string]string{"Other/elsewhere.png": "held"}, want: []string{"Notes/elsewhere.png"}},
		{name: "nonlocal routed outside hidden", body: "![](https://example.invalid/a.png)\n![](//example.invalid/a.png)\n![](data:image/png;base64,AAAA)\n![](/raw/a.png)\n![](/notes/a.png)\n![](/static/a.png)\n![](../../outside.png)\n![](.hidden/a.png)\n![]()\n"},
		{name: "malformed percent is refused by page admission", body: "![](bad%ZZ.png)\n"},
		{name: "encoded percent page spelling", body: "![](bad%25ZZ.png)\n", want: []string{"Notes/bad%ZZ.png"}},
		{name: "unique wiki suffix", body: "![[held.png]] ![[Notes/held.png]]\n"},
		{name: "ambiguous file name", body: "![[same.png]]\n", hold: map[string]string{"A/same.png": "held", "B/same.png": "held"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := missingImageVault(t)
			write(t, root, "Notes/Images.md", tt.body)
			for path, content := range tt.hold {
				write(t, root, path, content)
			}
			findings, err := Check(t.Context(), root)
			if err != nil {
				t.Fatalf("Check: %v", err)
			}
			var targets []string
			for _, f := range findings {
				if f.RuleID == "link.broken.image" {
					if f.Target == nil {
						t.Fatal("caught: missing-image membership: no target")
					}
					targets = append(targets, *f.Target)
				}
			}
			if diff := cmp.Diff(tt.want, targets); diff != "" {
				t.Errorf("caught: missing-image membership (-want +got):\n%s", diff)
			}
		})
	}
}

func TestMissingImagePrivacy(t *testing.T) {
	t.Parallel()
	for _, destination := range []string{
		"![](../Private/secret.png)",
		"![](/Private/secret.png)",
		"![](/Priv%61te/secret.png)",
		"![](/Private/cafe%CC%81.png)",
		"![[Private/secret.png]]",
	} {
		t.Run(destination, func(t *testing.T) {
			t.Parallel()
			absent := missingImageVault(t)
			present := missingImageVault(t)
			for _, root := range []string{absent, present} {
				write(t, root, "Notes/Images.md", "![](public.png) ![[public-wiki.png]]\n"+destination+"\n")
				write(t, root, "Private/Author.md", "![](private-author.png) ![[private-author-wiki.png]]\n")
				notes, err := collectNotes(t.Context(), root)
				if err != nil {
					t.Fatalf("collectNotes: %v", err)
				}
				parsedPrivate := false
				for _, n := range notes {
					if n.path == "Private/Author.md" && n.body == "![](private-author.png) ![[private-author-wiki.png]]\n" && len(n.wikilinks) == 1 {
						parsedPrivate = true
					}
				}
				if !parsedPrivate {
					t.Fatal("denied author stimulus was not parsed")
				}
			}
			write(t, present, "Private/secret.png", "held")
			write(t, present, "Private/caf\u00e9.png", "held")
			baseline := filepath.Join(t.TempDir(), "baseline.jsonl")
			base, baseExit, err := RunCheck(t.Context(), &CheckOptions{Root: absent, Format: FormatJSON})
			if err != nil || baseExit != 0 {
				t.Fatalf("positive RunCheck: exit=%d error=%v", baseExit, err)
			}
			if !bytes.Contains(base, []byte(`"target":"Notes/public.png"`)) || !bytes.Contains(base, []byte(`"target":"public-wiki.png"`)) {
				t.Fatalf("caught: missing-image privacy positive producers: %s", base)
			}
			if err := os.WriteFile(baseline, base, 0o600); err != nil {
				t.Fatalf("write baseline: %v", err)
			}
			for _, format := range []Format{FormatJSON, FormatHuman, FormatMarkdown} {
				for _, scope := range []struct {
					all   bool
					paths []string
				}{{}, {all: true}, {paths: []string{"Notes"}}} {
					for _, deny := range [][]string{nil, {"warn"}, {"link.broken.image"}} {
						for _, prior := range []string{"", baseline} {
							options := CheckOptions{Root: absent, All: scope.all, Paths: scope.paths, Deny: deny, Baseline: prior, Format: format}
							a, aExit, aErr := RunCheck(t.Context(), &options)
							if aErr != nil {
								t.Fatalf("absent RunCheck: %v", aErr)
							}
							options.Root = present
							b, bExit, bErr := RunCheck(t.Context(), &options)
							if bErr != nil {
								t.Fatalf("present RunCheck: %v", bErr)
							}
							t.Log("invoked: missing-image denied destination")
							wantExit := 0
							if prior == "" && len(deny) != 0 {
								wantExit = 1
							}
							if aExit != wantExit || bExit != wantExit || !bytes.Equal(a, b) {
								t.Errorf("caught: missing-image denied destination observable: format=%s all=%v paths=%v deny=%v baseline=%v exit=%d/%d want=%d\nabsent=%s\npresent=%s", format, scope.all, scope.paths, deny, prior != "", aExit, bExit, wantExit, a, b)
							}
							for _, forbidden := range []string{"Private", "secret.png", "private-author", "cafe", "caf\u00e9"} {
								if bytes.Contains(a, []byte(forbidden)) || bytes.Contains(b, []byte(forbidden)) {
									t.Errorf("caught: missing-image denied destination observable: leaked %q", forbidden)
								}
							}
						}
					}
				}
			}
		})
	}
}

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
		t.Fatal("partial corpus stimulus did not produce an unreadable source")
	}
	findings, err := checkAction(a, nil, false)
	if err != nil {
		t.Fatalf("checkAction: %v", err)
	}
	var imageTargets []string
	unreadable := 0
	for _, f := range findings {
		if f.RuleID == "link.broken.image" && f.Target != nil {
			imageTargets = append(imageTargets, *f.Target)
		}
		if f.RuleID == unreadableRule {
			unreadable++
		}
	}
	if diff := cmp.Diff([]string{"Notes/direct.png"}, imageTargets); diff != "" {
		t.Errorf("caught: missing-image partial producer certainty (-want +got):\n%s", diff)
	}
	if unreadable != 1 {
		t.Errorf("caught: missing-image partial notice: got %d want 1", unreadable)
	}
}

func TestMissingImageAuthority(t *testing.T) {
	t.Parallel()
	for _, format := range []Format{FormatJSON, FormatHuman, FormatMarkdown} {
		t.Run(format.String(), func(t *testing.T) {
			t.Parallel()
			root := missingImageVault(t)
			write(t, root, schema.ContractRelPath, "[malformed\n")
			out, exit, err := RunCheck(t.Context(), &CheckOptions{Root: root, Format: format})
			if err == nil || exit != 0 || len(out) != 0 {
				t.Errorf("caught: missing-image invalid authority: error=%v exit=%d output=%q", err, exit, out)
			}
		})
	}
	root := missingImageVault(t)
	prepared, err := prepareCheckWithHooks(t.Context(), &CheckOptions{Root: root, Format: FormatJSON}, actionHooks{afterNoteRead: func(string) {
		write(t, root, schema.ContractRelPath, "[malformed\n")
	}})
	if err == nil {
		finishErr := prepared.finish()
		if finishErr == nil {
			t.Fatal("caught: missing-image stale authority published")
		}
	} else if !errors.Is(err, ErrPrivacyAuthorityUnavailable) {
		t.Fatalf("stale authority refused with unexpected class: %v", err)
	}
}
