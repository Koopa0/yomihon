package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/egress"
	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/mark/confdir"
	"github.com/koopa0/yomihon/internal/mark/machinemarks"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

// leftOpenContract is the shelf tests' contract with one withheld directory.
var leftOpenContract = strings.Replace(openThoughtContract, "never_egress_dirs = []", `never_egress_dirs = ["Private"]`, 1)

type leftOpenResult struct {
	stdout, stderr string
	exit           int
}

// leftOpen runs the command the way main does, through its argument parser.
func leftOpen(t *testing.T, root, configDir string, args ...string) leftOpenResult {
	t.Helper()
	var stdout, stderr bytes.Buffer
	argv := append([]string{"--root", root}, args...)
	exit := runOpenThoughts(t.Context(), argv, config{configDir: confdir.At(configDir)}, &stdout, &stderr, false)
	return leftOpenResult{stdout: stdout.String(), stderr: stderr.String(), exit: exit}
}

// leftOpenMarks keeps marks for root under configDir, as the reading room
// would: in the vault's own machine files, admitting any location so a lost one
// can be kept too.
func leftOpenMarks(t *testing.T, root, configDir string, marks ...machinemarks.Mark) {
	t.Helper()
	keptMarks{t: t, root: root, config: configDir}.keep(marks...)
}

func leftOpenDay(day int) time.Time { return time.Date(2026, 9, day, 3, 4, 5, 0, time.UTC) }

// leftOpenVault is the fixture the golden lists: every kind of item and every
// shape a location can resolve to, and nothing withheld.
func leftOpenVault(t *testing.T) (root, configDir string) {
	t.Helper()
	root, configDir = t.TempDir(), t.TempDir()
	putOpenThoughtFile(t, root, schema.ContractRelPath, leftOpenContract)
	putOpenThoughtFile(t, root, "Source.md", "# Source\n\n## Chapter\n\nwords\n\n## Twice\n\none\n\n## Twice\n\ntwo\n")
	putOpenThoughtFile(t, root, "Titled.md", "# Titled\n\nbody\n")
	putOpenThoughtFile(t, root, "Inside/Thought.md", openThoughtNote("A thought", "reflection", draftFlag, "2026-09-06"))
	putOpenThoughtFile(t, root, "Outside/Alternate.md", openThoughtNote("Outside the layer", "reflection", draftFlag, "2026-09-08"))
	putOpenThoughtFile(t, root, "Inside/Done.md", openThoughtNote("Done note", "reflection", "draft: false\n", "2026-09-10"))
	putOpenThoughtFile(t, root, "Inside/Undated.md", "---\ntype: reflection\ndraft: true\n---\n# Undated\n\nA thought.\n")
	leftOpenMarks(t, root, configDir,
		machinemarks.Mark{Kind: machinemarks.Question, Path: "Source.md", Anchor: "chapter", At: leftOpenDay(7)},
		// Marked west of UTC on the evening before: the list says the moment in
		// UTC, and so the day UTC gives it, never the zone the reader was in.
		machinemarks.Mark{Kind: machinemarks.Question, Path: "Source.md", At: time.Date(2026, 9, 4, 20, 4, 5, 0, time.FixedZone("", -7*60*60))},
		machinemarks.Mark{Kind: machinemarks.Question, Path: "Source.md", Anchor: "vanished", At: leftOpenDay(4)},
		machinemarks.Mark{Kind: machinemarks.Question, Path: "Source.md", Anchor: "twice", At: leftOpenDay(3)},
		machinemarks.Mark{Kind: machinemarks.Question, Path: "Gone.md", Anchor: "x", At: leftOpenDay(2)},
		machinemarks.Mark{Kind: machinemarks.Question, Path: "Titled.md", Anchor: "titled", At: leftOpenDay(1)},
		machinemarks.Mark{Kind: machinemarks.Question, Path: "Titled.md", Anchor: "gone-heading", At: leftOpenDay(0)},
	)
	return root, configDir
}

// The golden is the wire: field names, their order, every value shape, and the
// order of the items. It is compared byte for byte against a file written by
// hand from the fixture above, so a change to any of them shows as a diff.
func TestLeftOpenWritesTheGoldenList(t *testing.T) {
	t.Parallel()
	root, configDir := leftOpenVault(t)
	for _, tt := range []struct {
		format, golden string
	}{
		{format: "json", golden: "list.jsonl"},
		{format: "human", golden: "list.txt"},
		{format: "md", golden: "list.txt"},
	} {
		t.Run(tt.format, func(t *testing.T) {
			t.Parallel()
			got := leftOpen(t, root, configDir, "--format", tt.format)
			want, err := os.ReadFile(filepath.Join("testdata", "marks", tt.golden))
			if err != nil {
				t.Fatal(err)
			}
			if got.exit != 0 || got.stderr != "" {
				t.Fatalf("marks --format %s: exit %d, stderr %q", tt.format, got.exit, got.stderr)
			}
			if diff := cmp.Diff(string(want), got.stdout); diff != "" {
				t.Errorf("caught: marks --format %s differs from %s (-want +got):\n%s", tt.format, tt.golden, diff)
			}
		})
	}
}

// A title is the note's first level-one heading and nothing else: not a
// heading inside a fence, not a lesser heading, not a title the frontmatter
// writes, and not the file name when the note has no such heading.
func TestLeftOpenTitlesANoteOnlyByItsFirstHeading(t *testing.T) {
	t.Parallel()
	root, configDir := t.TempDir(), t.TempDir()
	putOpenThoughtFile(t, root, schema.ContractRelPath, leftOpenContract)
	putOpenThoughtFile(t, root, "Inside/Unheaded.md", "---\ntype: reflection\ndraft: true\ncreated: 2026-09-04\n---\nWords only.\n\n## A lesser heading\n")
	putOpenThoughtFile(t, root, "Inside/Fenced.md", "---\ntype: reflection\ndraft: true\ncreated: 2026-09-03\n---\n```\n# Inside a fence\n```\n\n# Past the fence\n")
	putOpenThoughtFile(t, root, "Inside/Written.md", "---\ntitle: Written above\ntype: reflection\ndraft: true\ncreated: 2026-09-02\n---\n# Said in the body\n")
	putOpenThoughtFile(t, root, "Plain.md", "Words.\n\n## Part\n\nmore\n")
	leftOpenMarks(t, root, configDir, machinemarks.Mark{Kind: machinemarks.Question, Path: "Plain.md", Anchor: "part", At: leftOpenDay(1)})
	got := leftOpen(t, root, configDir, "--format", "json")
	if got.exit != 0 {
		t.Fatalf("marks exit %d: %s", got.exit, got.stderr)
	}
	titles := map[string]any{}
	for _, item := range leftOpenItems(t, got.stdout) {
		path, _ := item["path"].(string) //nolint:errcheck // a missing path shows in the diff
		titles[path] = item["title"]
	}
	want := map[string]any{
		"Inside/Unheaded.md": nil,
		"Inside/Fenced.md":   "Past the fence",
		"Inside/Written.md":  "Said in the body",
		"Plain.md":           nil,
	}
	if diff := cmp.Diff(want, titles); diff != "" {
		t.Errorf("caught: a title came from somewhere other than the first level-one heading (-want +got):\n%s", diff)
	}
}

// The shelf and the command answer from one selection. Read in shelf order,
// every row the page links to and every item the command lists name the same
// note, in the same order.
func TestLeftOpenListsWhatTheShelfShowsInItsOrder(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	putOpenThoughtFile(t, root, schema.ContractRelPath, leftOpenContract)
	putOpenThoughtFile(t, root, "Source.md", "# Source\n\n## Chapter\n\nwords\n")
	putOpenThoughtFile(t, root, "Other.md", "# Other\n")
	for i, rel := range []string{"Inside/A.md", "Inside/B.md", "Outside/C.md"} {
		putOpenThoughtFile(t, root, rel, openThoughtNote("Thought "+rel, "reflection", draftFlag, leftOpenDay(2*i+2).Format(time.DateOnly)))
	}
	putOpenThoughtFile(t, root, "Inside/Same day.md", openThoughtNote("Same day", "reflection", draftFlag, leftOpenDay(4).Format(time.DateOnly)))
	site, marks := openThoughtSite(t, root)
	marks.keep(
		machinemarks.Mark{Kind: machinemarks.Question, Path: "Source.md", Anchor: "chapter", At: leftOpenDay(5)},
		machinemarks.Mark{Kind: machinemarks.Question, Path: "Other.md", At: leftOpenDay(4)},
		machinemarks.Mark{Kind: machinemarks.Question, Path: "Source.md", At: leftOpenDay(1)},
	)
	page := openThoughtPage(t, site, "/marks", "en")
	var shelf []string
	name := regexp.MustCompile(`class="y-openitem__name" href="/notes/([^"]*)"`)
	for _, row := range strings.Split(page, "data-index-row")[1:] {
		found := name.FindStringSubmatch(row)
		if found == nil {
			t.Fatalf("a shelf row leads nowhere, which none of this fixture's rows does:\n%s", row)
		}
		address, err := url.PathUnescape(found[1])
		if err != nil {
			t.Fatal(err)
		}
		shelf = append(shelf, address)
	}
	if len(shelf) == 0 {
		t.Fatalf("no shelf row was read from the page:\n%s", page)
	}
	got := leftOpen(t, root, marks.config, "--format", "json")
	if got.exit != 0 {
		t.Fatalf("marks exit %d: %s", got.exit, got.stderr)
	}
	var listed []string
	for _, item := range leftOpenItems(t, got.stdout) {
		at, ok := item["path"].(string)
		if !ok {
			t.Fatalf("item without a path: %v", item)
		}
		if anchor, ok := item["anchor"].(string); ok {
			at += "#" + anchor
		}
		listed = append(listed, at)
	}
	if diff := cmp.Diff(shelf, listed); diff != "" {
		t.Errorf("caught: marks and the shelf disagree on the open items or their order (-shelf +marks):\n%s", diff)
	}
}

// leftOpenItems decodes the JSON Lines output.
func leftOpenItems(t *testing.T, stdout string) []map[string]any {
	t.Helper()
	var items []map[string]any
	for line := range strings.SplitSeq(strings.TrimSuffix(stdout, "\n"), "\n") {
		if line == "" {
			continue
		}
		var item map[string]any
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			t.Fatalf("output line %q is not JSON: %v", line, err)
		}
		items = append(items, item)
	}
	return items
}

// leftOpenPrivateVault holds open items in withheld ground and items that
// point into it from outside, beside one plainly allowed thought and mark.
func leftOpenPrivateVault(t *testing.T) (root, configDir string) {
	t.Helper()
	var marks []machinemarks.Mark
	root, configDir = t.TempDir(), t.TempDir()
	putOpenThoughtFile(t, root, schema.ContractRelPath, leftOpenContract)
	putOpenThoughtFile(t, root, "Source.md", "# Source\n\n## Chapter\n\nwords\n")
	putOpenThoughtFile(t, root, "Inside/Visible.md", openThoughtNote("Visible thought", "reflection", draftFlag, "2026-09-01"))
	putOpenThoughtFile(t, root, "Private/Secret plan.md", "# Secret plan\n\n## Hidden heading\n\nwords\n")
	putOpenThoughtFile(t, root, "Private/Hidden thought.md", openThoughtNote("Hidden thought", "reflection", draftFlag, "2026-09-02"))
	putOpenThoughtFile(t, root, "Inside/Cites private.md",
		"---\ntype: reflection\ndraft: true\ncreated: 2026-09-03\nbased_on: '[[Private/Secret plan#Hidden heading]]'\n---\n# Cites quietly\n\nA thought.\n")
	putOpenThoughtFile(t, root, "Public.md", "# Public\n\n## About [[Private/Secret plan]]\n\nwords\n")
	// The only link to withheld ground is the one the title writes, and it
	// cites no heading the anchor could be made from.
	putOpenThoughtFile(t, root, "Inside/Titled cite.md",
		"---\ntype: reflection\ndraft: true\ncreated: 2026-09-07\n---\n# On [[Private/Secret plan]]\n\nA thought.\n")
	putOpenThoughtFile(t, root, "Retitled.md", "# Re [[Private/Secret plan]]\n\n## Sec\n\nwords\n")
	// A title is never rendered, so Markdown that would hide a link on a page
	// — a code span, a backslash — hides nothing in the list, and a Markdown
	// link names its destination as plainly as a wikilink does.
	putOpenThoughtFile(t, root, "Private/Caf\u00e9.md", "# Caf\u00e9\n")
	for i, title := range leftOpenRawTitles {
		putOpenThoughtFile(t, root, fmt.Sprintf("Inside/Raw %d.md", i),
			fmt.Sprintf("---\ntype: reflection\ndraft: true\ncreated: 2026-09-07\n---\n# %s\n\nA thought.\n", title))
		putOpenThoughtFile(t, root, fmt.Sprintf("Raw %d.md", i), fmt.Sprintf("# %s\n\n## Sec\n\nwords\n", title))
		marks = append(marks, machinemarks.Mark{Kind: machinemarks.Question, Path: fmt.Sprintf("Raw %d.md", i), Anchor: "sec", At: leftOpenDay(7)})
	}
	// The heading the anchor was made from has been rewritten since; the
	// anchor still spells the withheld note it once cited.
	putOpenThoughtFile(t, root, "Renamed.md", "# Renamed\n\n## Other\n\nwords\n")
	// The note is gone, and with it any way to tell what its anchor's words cited.
	leftOpenMarks(t, root, configDir, append(marks,
		machinemarks.Mark{Kind: machinemarks.Question, Path: "Source.md", Anchor: "chapter", At: leftOpenDay(4)},
		machinemarks.Mark{Kind: machinemarks.Question, Path: "Private/Secret plan.md", Anchor: "hidden-heading", At: leftOpenDay(5)},
		machinemarks.Mark{Kind: machinemarks.Question, Path: "Public.md", Anchor: "about-private-secret-plan", At: leftOpenDay(6)},
		machinemarks.Mark{Kind: machinemarks.Question, Path: "Retitled.md", Anchor: "sec", At: leftOpenDay(7)},
		machinemarks.Mark{Kind: machinemarks.Question, Path: "Renamed.md", Anchor: "about-private-secret-plan", At: leftOpenDay(8)},
		machinemarks.Mark{Kind: machinemarks.Question, Path: "Deleted.md", Anchor: "about-private-secret-plan", At: leftOpenDay(9)},
	)...)
	return root, configDir
}

// leftOpenRawTitles are titles that cite withheld ground, each written once
// as a thought's title and once as a marked location's note title. A title is
// written out as it stands, so none of them may reach the list however a
// Markdown reading of it would treat it. The first group names withheld ground
// through link syntax and holds the destination to CommonMark's own rules;
// the second spells a withheld directory followed by a slash, which no link
// syntax is needed for, through encodings and invisible characters.
var leftOpenRawTitles = []string{
	// Link syntax: wikilinks, inline links, reference definitions and HTML.
	"On `[[Private/Secret plan]]`",
	`On \[[Private/Secret plan]]`,
	"On [x](Private/Secret plan.md)",
	"On [x](../Private/Secret%20plan.md)",
	"On [x](Pri%76ate/Secret%20plan.md)",
	"On `[[Secret plan#Hidden heading|the plan]]`",
	"On [[private/secret plan]]",
	"On [[Secret plan]]",
	"On [[secret plan]]",
	"On [x](Private%2FSecret%20plan.md)",
	"On [x](Secret%20plan.md)",
	"On [[Cafe\u0301]]",
	"On [[Private/Cafe\u0301]]",
	"On `[[Private/Secret plan|alias]]`",
	"On [[Private/Se\ncret plan]]",
	"On [[ Private/Secret plan ]]",
	`On [x](Private/Secret%20plan.md "t")`,
	"On [x](Private/(a)/Secret plan.md)",
	`On [[Private\Secret plan]]`,
	"On [x](<Secret plan.md>)",
	"On [x](<Private/Secret plan.md>)",
	"On [x][r]\n\n[r]: Private/Secret plan.md",
	"On [x][r]\n\n[r]: <Secret plan.md>",
	"On [r]\n\n[r]: Secret%20plan.md \"t\"",
	`On <a href="Private/Secret plan.md">x</a>`,
	`On <a href="Secret plan.md">x</a>`,
	`On <a href='Secret%20plan.md'>x</a>`,
	// A withheld directory and a slash, spelled through what hides it.
	"On Private/Secret plan.md",
	"On <Private/Secret plan.md>",
	"On [[Private%2FSecret plan]]",
	"On &#91;&#91;Private/Secret plan&#93;&#93;",
	"On &#80;rivate&#x2F;Secret plan",
	"On \uff3b\uff3bPrivate/Secret plan\uff3d\uff3d",
	"On [[Pri\u200bvate/Secret plan]]",
	"On Pri\u200dvate\ufeff/Secret plan",
	"On PRIVATE/Secret plan",
	"On Pri%2576ate/Secret plan",
	"On \uff30\uff52\uff49\uff56\uff41\uff54\uff45\uff0fSecret plan",
	`On Private\Secret plan`,
	// Characters that draw nothing, or draw over another, inside the name:
	// format characters, a variation selector, a default-ignorable letter and
	// combining marks; and the solidus's look-alikes in place of the slash.
	"On Pri\u200evate/Secret plan",
	"On Pri\u2062vate/Secret plan",
	"On Pri\U000e0041vate/Secret plan",
	"On Pri\ufe0fvate/Secret plan",
	"On Pri\u034fvate/Secret plan",
	"On Pri\u115fvate/Secret plan",
	"On Pri\u0307vate/Secret plan",
	"On Private\u2215Secret plan",
	"On Private\u2044Secret plan",
	"On Private\u29f8Secret plan",
	"On semi-Private/public",
}

// The words that would describe withheld ground: its directory, its file
// names, its headings and the anchors made from them.
var leftOpenPrivateWords = []string{"Private", "PRIVATE", "Pri", "Secret", "secret", "Hidden", "hidden", "Cites", "Caf"}

func TestLeftOpenSaysNothingOfWithheldGround(t *testing.T) {
	t.Parallel()
	root, configDir := leftOpenPrivateVault(t)
	for _, format := range []string{"json", "human"} {
		got := leftOpen(t, root, configDir, "--format", format)
		if got.exit != 0 {
			t.Fatalf("--format %s: exit %d: %s", format, got.exit, got.stderr)
		}
		for _, word := range leftOpenPrivateWords {
			if strings.Contains(got.stdout, word) || strings.Contains(got.stderr, word) {
				t.Errorf("caught: --format %s says %q about withheld ground:\nstdout %s\nstderr %s", format, word, got.stdout, got.stderr)
			}
		}
		for _, want := range []string{"Inside/Visible.md", "Source.md", "Renamed.md", "Deleted.md"} {
			if !strings.Contains(got.stdout, want) {
				t.Errorf("--format %s lost the allowed item %s:\n%s", format, want, got.stdout)
			}
		}
	}
}

// Every key an item may carry, by kind. A key outside these — based_on, a
// body excerpt — is a second path or vault text the list does not hand out.
var leftOpenKeys = map[string][]string{
	"location": {"anchor", "anchor_text", "create_time", "kind", "lost", "path", "title", "wikilink"},
	"thought":  {"create_date", "kind", "path", "title", "wikilink"},
}

func TestLeftOpenCarriesNoFieldBeyondItsOwn(t *testing.T) {
	t.Parallel()
	root, configDir := leftOpenVault(t)
	got := leftOpen(t, root, configDir, "--format", "json")
	items := leftOpenItems(t, got.stdout)
	if len(items) == 0 {
		t.Fatal("the fixture listed nothing, so no key was looked at")
	}
	for _, item := range items {
		kind, ok := item["kind"].(string)
		if !ok {
			t.Fatalf("item without a kind: %v", item)
		}
		allowed, known := leftOpenKeys[kind]
		if !known {
			t.Errorf("caught: unknown kind in %v", item)
			continue
		}
		for key := range item {
			if !slices.Contains(allowed, key) {
				t.Errorf("caught: %s item carries %q: %v", item["kind"], key, item)
			}
		}
	}
}

// A list assembled under one contract is not handed over under another: when
// the contract changes between assembly and output, the command refuses.
func TestLeftOpenChecksAuthorityAgainAfterAssembly(t *testing.T) {
	t.Parallel()
	root, configDir := leftOpenPrivateVault(t)
	narrowed := strings.Replace(leftOpenContract, `never_egress_dirs = ["Private"]`, `never_egress_dirs = ["Private", "Inside"]`, 1)
	var stdout, stderr bytes.Buffer
	request := openRequest{root: root, configDir: confdir.At(configDir), format: judge.FormatJSON, afterAssemble: func() {
		putOpenThoughtFile(t, root, schema.ContractRelPath, narrowed)
	}}
	exit := request.run(t.Context(), &stdout, &stderr)
	if exit != 2 || stdout.Len() != 0 || strings.Contains(stdout.String(), "Inside/") {
		t.Errorf("caught: a contract narrowed after assembly still got the list: exit %d, stdout %q", exit, stdout.String())
	}
	if !strings.Contains(stderr.String(), egress.ErrUnavailable.Error()) {
		t.Errorf("stderr = %q, want the privacy refusal", stderr.String())
	}
}

// The marks command started from a folder inside a vault, or above one, is not
// started from a folder with no rules file. Each shape is refused before any
// list is built, since the withheld folder the vault's rules name would
// otherwise be listed as an ordinary one.
func TestLeftOpenRefusesAFolderInsideOrAboveAVault(t *testing.T) {
	t.Parallel()
	// A thought is what a folder with no rules file lists, so a leak shows up as
	// this one's path on stdout.
	hidden := openThoughtNote("Hidden thought", "thought", draftFlag, "2026-09-02")
	shapes := []struct {
		name string
		// unknown is a shape whose surroundings cannot be looked at, which is
		// refused as such rather than as a rules file found.
		unknown bool
		build   func(t *testing.T) string
	}{
		{"started inside the folder the rules withhold", false, func(t *testing.T) string {
			t.Helper()
			wrap := t.TempDir()
			putOpenThoughtFile(t, wrap, "v/"+schema.ContractRelPath, leftOpenContract)
			putOpenThoughtFile(t, wrap, "v/Private/Hidden thought.md", hidden)
			return filepath.Join(wrap, "v", "Private")
		}},
		{"started two folders inside the vault", false, func(t *testing.T) string {
			t.Helper()
			wrap := t.TempDir()
			putOpenThoughtFile(t, wrap, "v/"+schema.ContractRelPath, leftOpenContract)
			putOpenThoughtFile(t, wrap, "v/Private/2026/Hidden thought.md", hidden)
			return filepath.Join(wrap, "v", "Private", "2026")
		}},
		{"started one folder above a vault", false, func(t *testing.T) string {
			t.Helper()
			wrap := t.TempDir()
			putOpenThoughtFile(t, wrap, "v/"+schema.ContractRelPath, leftOpenContract)
			putOpenThoughtFile(t, wrap, "v/Private/Hidden thought.md", hidden)
			return wrap
		}},
		{"started above a vault nested below the root", false, func(t *testing.T) string {
			t.Helper()
			wrap := t.TempDir()
			putOpenThoughtFile(t, wrap, "a/b/v/"+schema.ContractRelPath, leftOpenContract)
			putOpenThoughtFile(t, wrap, "a/b/v/Private/Hidden thought.md", hidden)
			return wrap
		}},
		// What the scan leaves out is where a rules file hides. The withheld
		// folder is an ordinary one to a command that never learned the rules.
		{"above a vault whose rules file is a symlink", false, func(t *testing.T) string {
			t.Helper()
			wrap, shared := t.TempDir(), t.TempDir()
			putOpenThoughtFile(t, shared, "vault-schema.toml", leftOpenContract)
			putOpenThoughtFile(t, wrap, "Vs/Private/Hidden thought.md", hidden)
			linkTo(t, filepath.Join(shared, "vault-schema.toml"), filepath.Join(wrap, "Vs", "System", "schemas", "vault-schema.toml"))
			return wrap
		}},
		{"above a vault whose schemas folder is a symlink", false, func(t *testing.T) string {
			t.Helper()
			wrap, shared := t.TempDir(), t.TempDir()
			putOpenThoughtFile(t, shared, "vault-schema.toml", leftOpenContract)
			putOpenThoughtFile(t, wrap, "Vs/Private/Hidden thought.md", hidden)
			linkTo(t, shared, filepath.Join(wrap, "Vs", "System", "schemas"))
			return wrap
		}},
		{"above a vault whose System folder is a symlink", false, func(t *testing.T) string {
			t.Helper()
			wrap, shared := t.TempDir(), t.TempDir()
			putOpenThoughtFile(t, shared, "schemas/vault-schema.toml", leftOpenContract)
			putOpenThoughtFile(t, wrap, "Vs/Private/Hidden thought.md", hidden)
			linkTo(t, shared, filepath.Join(wrap, "Vs", "System"))
			return wrap
		}},
		{"above a vault whose System is a file", false, func(t *testing.T) string {
			t.Helper()
			wrap := t.TempDir()
			putOpenThoughtFile(t, wrap, "Vf/System", "not a folder\n")
			putOpenThoughtFile(t, wrap, "Vf/Private/Hidden thought.md", hidden)
			return wrap
		}},
		{"above a vault whose rules file an ignore rule hides from the scan", false, func(t *testing.T) string {
			t.Helper()
			wrap := t.TempDir()
			putOpenThoughtFile(t, wrap, ".gitignore", "schemas/\n")
			putOpenThoughtFile(t, wrap, "Vg/"+schema.ContractRelPath, leftOpenContract)
			putOpenThoughtFile(t, wrap, "Vg/Private/Hidden thought.md", hidden)
			return wrap
		}},
		{"above a vault whose System folder cannot be listed", true, func(t *testing.T) string {
			t.Helper()
			if os.Geteuid() == 0 {
				t.Skip("directory permissions do not hold for root")
			}
			wrap := t.TempDir()
			putOpenThoughtFile(t, wrap, "Vu/"+schema.ContractRelPath, leftOpenContract)
			putOpenThoughtFile(t, wrap, "Vu/Private/Hidden thought.md", hidden)
			lockDir(t, filepath.Join(wrap, "Vu", "System"))
			return wrap
		}},
	}
	for _, shape := range shapes {
		for _, format := range []judge.Format{judge.FormatJSON, judge.FormatHuman} {
			t.Run(fmt.Sprintf("%s/%v", shape.name, format), func(t *testing.T) {
				t.Parallel()
				root := shape.build(t)
				var stdout, stderr bytes.Buffer
				request := openRequest{root: root, configDir: confdir.At(t.TempDir()), format: format}
				exit := request.run(t.Context(), &stdout, &stderr)
				if exit != 2 || stdout.Len() != 0 {
					t.Errorf("caught: marks listed from a folder inside or above a vault: exit %d, stdout %q", exit, stdout.String())
				}
				want := egress.ErrRulesFileNearby
				if shape.unknown {
					want = egress.ErrRulesFileUnknown
				}
				if !strings.Contains(stderr.String(), want.Error()) {
					t.Errorf("stderr = %q, want %q", stderr.String(), want)
				}
				for _, named := range []string{root, "Vu", "Vs", "Vf", "System"} {
					if strings.Contains(strings.ReplaceAll(stderr.String(), want.Error(), ""), named) {
						t.Errorf("stderr names a path (%q): %q", named, stderr.String())
					}
				}
			})
		}
	}
}

func linkTo(t *testing.T, target, link string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(link), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks are not available here: %v", err)
	}
}

// lockDir makes a directory impossible to list and puts its mode back when the
// test ends, so the temp dir can be removed.
func lockDir(t *testing.T, dir string) {
	t.Helper()
	if err := os.Chmod(dir, 0o000); err != nil { // #nosec G302 -- the test's own temp dir
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(dir, 0o750); err != nil { // #nosec G302 -- restoring the execute bit lets cleanup remove the temp dir
			t.Errorf("restore %q: %v", dir, err)
		}
	})
}

// The first look at the folder ends before the list is read, and a vault moved
// in between would be listed as ordinary ground. The hook puts one there at
// that moment: after the rules are read and before the vault is read for the
// list. The refusal has to come from what the walk the list is read from entered.
func TestLeftOpenRefusesAVaultMovedInAfterTheFirstLook(t *testing.T) {
	t.Parallel()
	for _, format := range []judge.Format{judge.FormatJSON, judge.FormatHuman} {
		t.Run(format.String(), func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			putOpenThoughtFile(t, root, "Notes/a.md", "# a\n")
			request := openRequest{root: root, configDir: confdir.At(t.TempDir()), format: format, afterRules: func() {
				putOpenThoughtFile(t, root, "Later/"+schema.ContractRelPath, leftOpenContract)
				putOpenThoughtFile(t, root, "Later/Private/Hidden thought.md", openThoughtNote("Hidden thought", "thought", draftFlag, "2026-09-02"))
			}}
			var stdout, stderr bytes.Buffer
			exit := request.run(t.Context(), &stdout, &stderr)
			if exit != 2 || stdout.Len() != 0 {
				t.Errorf("caught: marks listed a note read from a folder that holds a rules file: exit %d, stdout %q", exit, stdout.String())
			}
			if !strings.Contains(stderr.String(), egress.ErrRulesFileNearby.Error()) {
				t.Errorf("stderr = %q, want the refusal that names a rules file above or inside", stderr.String())
			}
		})
	}
}

// A stopped command is the reader's own doing and gets its own words, not the
// refusal about a rules file that never failed.
func TestLeftOpenCancelledIsNotAPrivacyRefusal(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	putOpenThoughtFile(t, root, "Notes/a.md", "# a\n")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	var stdout, stderr bytes.Buffer
	exit := (&openRequest{root: root, configDir: confdir.At(t.TempDir()), format: judge.FormatJSON}).run(ctx, &stdout, &stderr)
	if exit != 2 || stdout.Len() != 0 {
		t.Errorf("exit %d, stdout %q, want a refusal and nothing listed", exit, stdout.String())
	}
	if strings.Contains(stderr.String(), egress.ErrUnavailable.Error()) || !strings.Contains(stderr.String(), "context canceled") {
		t.Errorf("stderr = %q, want the stopped command's own words and not the privacy refusal", stderr.String())
	}
}

// A folder with no rules file withheld nothing when the list was assembled. A
// rules file written since may withhold exactly what was listed, so the list is
// not handed over: the command refuses, as it does when a rules file changes.
func TestLeftOpenRefusesWhenARulesFileAppearsAfterAssembly(t *testing.T) {
	t.Parallel()
	root, configDir := t.TempDir(), t.TempDir()
	putOpenThoughtFile(t, root, "Source.md", "# Source\n")
	putOpenThoughtFile(t, root, "Private/Thought.md", openThoughtNote("A private thought", "thought", draftFlag, "2026-09-01"))
	leftOpenMarks(t, root, configDir, machinemarks.Mark{Kind: machinemarks.Question, Path: "Source.md", At: leftOpenDay(1)})
	var stdout, stderr bytes.Buffer
	request := openRequest{root: root, configDir: confdir.At(configDir), format: judge.FormatJSON, afterAssemble: func() {
		putOpenThoughtFile(t, root, schema.ContractRelPath, leftOpenContract)
	}}
	exit := request.run(t.Context(), &stdout, &stderr)
	if exit != 2 || stdout.Len() != 0 {
		t.Errorf("caught: a rules file that appeared after assembly still got the list: exit %d, stdout %q", exit, stdout.String())
	}
	if !strings.Contains(stderr.String(), egress.ErrUnavailable.Error()) {
		t.Errorf("stderr = %q, want the privacy refusal", stderr.String())
	}
}

// A withheld note that cannot be read changes nothing: not the list, not the
// exit, and no line names it. The vault generation logs such a read by path,
// so its logger must say nothing anywhere this process writes.
//
// Not parallel: it swaps the process's stderr and the standard logger's
// output, which is where a logger that was not discarded would write.
func TestLeftOpenWithheldReadFailureIsSilent(t *testing.T) {
	root, configDir := leftOpenPrivateVault(t)
	before := leftOpen(t, root, configDir, "--format", "json")
	locked := filepath.Join(root, "Private", "Locked plan.md")
	putOpenThoughtFile(t, root, "Private/Locked plan.md", openThoughtNote("Locked", "reflection", draftFlag, "2026-09-09"))
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o600) }) //nolint:errcheck // TempDir removal reports what is left
	if data, err := os.ReadFile(locked); err == nil { // #nosec G304 -- a path under this test's TempDir
		t.Skipf("this user can read a mode-0 file (%d bytes), so no read fails", len(data))
	}

	process := captureProcessOutput(t)
	after := leftOpen(t, root, configDir, "--format", "json")
	written := process()

	if diff := cmp.Diff(before, after, cmp.AllowUnexported(leftOpenResult{})); diff != "" {
		t.Errorf("caught: an unreadable withheld note changed the answer (-readable +unreadable):\n%s", diff)
	}
	for _, word := range []string{"Locked", "Private"} {
		if strings.Contains(after.stderr, word) || strings.Contains(written, word) {
			t.Errorf("caught: %q reached stderr:\ncommand stderr %q\nprocess output %q", word, after.stderr, written)
		}
	}
}

// captureProcessOutput points the process's stderr and the standard logger at
// a pipe until the returned function is called, which restores both and
// returns what was written.
func captureProcessOutput(t *testing.T) func() string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr, logOut := os.Stderr, log.Writer()
	os.Stderr = writer
	log.SetOutput(writer)
	done := make(chan string, 1)
	go func() {
		data, _ := io.ReadAll(reader) //nolint:errcheck // a short read still shows what leaked
		done <- string(data)
	}()
	return func() string {
		os.Stderr = stderr
		log.SetOutput(logOut)
		if err := writer.Close(); err != nil {
			t.Error(err)
		}
		return <-done
	}
}

// A note that cannot be read outside withheld ground might be a thought still
// open, so the list is refused rather than handed over without it.
func TestLeftOpenRefusesAListMissingAnUnreadableNote(t *testing.T) {
	t.Parallel()
	root, configDir := leftOpenPrivateVault(t)
	locked := filepath.Join(root, "Inside", "Locked.md")
	putOpenThoughtFile(t, root, "Inside/Locked.md", openThoughtNote("Locked", "reflection", draftFlag, "2026-09-09"))
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o600) }) //nolint:errcheck // TempDir removal reports what is left
	if data, err := os.ReadFile(locked); err == nil { // #nosec G304 -- a path under this test's TempDir
		t.Skipf("this user can read a mode-0 file (%d bytes), so no read fails", len(data))
	}
	got := leftOpen(t, root, configDir, "--format", "json")
	want := leftOpenResult{
		exit:   2,
		stderr: "yomihon: Inside/Locked.md could not be read, so whether it is a thought still open is unknown; nothing was listed\n",
	}
	if diff := cmp.Diff(want, got, cmp.AllowUnexported(leftOpenResult{})); diff != "" {
		t.Errorf("caught: unreadable note (-want +got):\n%s", diff)
	}
}

func TestLeftOpenRefusesWithoutPrivacyAuthority(t *testing.T) {
	t.Parallel()
	undeclared := regexp.MustCompile(`(?s)\[privacy\]\nnever_egress_dirs = \[\]\n`).ReplaceAllString(openThoughtContract, "")
	if undeclared == openThoughtContract {
		t.Fatal("the fixture contract no longer declares [privacy] the way this test removes it")
	}
	for _, tt := range []struct {
		name     string
		contract string // empty writes no contract at all
		want     string
	}{
		{name: "no privacy section", contract: undeclared, want: "yomihon: the rules file declares no privacy policy, so marks lists nothing for a program to read; add a [privacy] table with never_egress_dirs = [] to System/schemas/vault-schema.toml, naming in that list any directory whose notes must never leave this machine\n"},
		{name: "invalid privacy section", contract: strings.Replace(openThoughtContract, "never_egress_dirs = []", `never_egress_dirs = ["../Quoted secret"]`, 1), want: "yomihon: privacy authority unavailable; agent-facing output disabled\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root, configDir := t.TempDir(), t.TempDir()
			if tt.contract != "" {
				putOpenThoughtFile(t, root, schema.ContractRelPath, tt.contract)
			}
			putOpenThoughtFile(t, root, "Source.md", "# Source\n")
			putOpenThoughtFile(t, root, "Inside/Thought.md", openThoughtNote("A thought", "reflection", draftFlag, "2026-09-01"))
			leftOpenMarks(t, root, configDir, machinemarks.Mark{Kind: machinemarks.Question, Path: "Source.md", At: leftOpenDay(1)})
			got := leftOpen(t, root, configDir, "--format", "json")
			if got.exit != 2 || got.stdout != "" {
				t.Errorf("caught: %s: exit %d, stdout %q; want 2 and nothing", tt.name, got.exit, got.stdout)
			}
			if first, _, _ := strings.Cut(got.stderr, "\n"); first+"\n" != tt.want {
				t.Errorf("%s: stderr first line = %q, want %q", tt.name, first, tt.want)
			}
			if strings.Contains(got.stderr, "Quoted secret") {
				t.Errorf("caught: the refusal quotes the contract back: %q", got.stderr)
			}
		})
	}
}

// A rules file that cannot be used is a refusal, never a folder with no rules
// file: what it withholds is unknown, so a note under it must not be listed,
// and nothing the file wrote comes back out. That holds for the key every
// earlier rules file began with, which is one more key nothing reads.
func TestLeftOpenRefusesARulesFileItCannotUse(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name  string
		rules string
		want  string
	}{
		{"a version key", "schema_version = \"1\"\n[privacy]\nnever_egress_dirs = [\"Private\"]\n", "could not use it"},
		{"a key nothing reads", "unknown_key = true\n[privacy]\nnever_egress_dirs = [\"Private\"]\n", "could not use it"},
		{"a key whose value is a secret", "SecretKey = \"SECRET-token\"\n[privacy]\nnever_egress_dirs = [\"Private\"]\n", "could not use it"},
	} {
		for _, format := range []string{"json", "human"} {
			t.Run(tt.name+"/"+format, func(t *testing.T) {
				t.Parallel()
				root, configDir := t.TempDir(), t.TempDir()
				putOpenThoughtFile(t, root, schema.ContractRelPath, tt.rules)
				putOpenThoughtFile(t, root, "Private/Hidden thought.md", openThoughtNote("Hidden thought", "thought", draftFlag, "2026-09-02"))
				got := leftOpen(t, root, configDir, "--format", format)
				if got.exit != 2 || got.stdout != "" {
					t.Errorf("caught: marks read a folder whose rules file cannot be used: exit %d, stdout %q; want 2 and nothing", got.exit, got.stdout)
				}
				if first, _, _ := strings.Cut(got.stderr, "\n"); first != "yomihon: "+egress.ErrUnavailable.Error() {
					t.Errorf("stderr first line = %q, want the unavailable-authority refusal", first)
				}
				if !strings.Contains(got.stderr, tt.want) {
					t.Errorf("stderr = %q, want it to say %q", got.stderr, tt.want)
				}
				for _, leaked := range []string{"SecretKey", "SECRET-token", "Private", "Hidden thought", egress.NoRulesFile} {
					if strings.Contains(got.stdout+got.stderr, leaked) {
						t.Errorf("caught: the refusal or the output carries %q: %q", leaked, got.stdout+got.stderr)
					}
				}
			})
		}
	}
}

// A folder with no rules file is read: it declared nothing, so nothing is
// withheld, and the list says so first where a person reads it. The machine
// form is the list alone.
func TestLeftOpenReadsAFolderWithNoRulesFile(t *testing.T) {
	t.Parallel()
	root, configDir := t.TempDir(), t.TempDir()
	putOpenThoughtFile(t, root, "Source.md", "# Source\n")
	putOpenThoughtFile(t, root, "Private/Thought.md", openThoughtNote("Under a directory a rules file might withhold", "thought", draftFlag, "2026-09-01"))
	leftOpenMarks(t, root, configDir, machinemarks.Mark{Kind: machinemarks.Question, Path: "Source.md", At: leftOpenDay(1)})

	machine := leftOpen(t, root, configDir, "--format", "json")
	if machine.exit != 0 {
		t.Fatalf("exit = %d, stderr %q; want 0", machine.exit, machine.stderr)
	}
	for _, want := range []string{"Source.md", "Private/Thought.md"} {
		if !strings.Contains(machine.stdout, want) {
			t.Errorf("the list is missing %q although no rules file withholds it:\n%s", want, machine.stdout)
		}
	}
	if strings.Contains(machine.stdout, egress.NoRulesFile) {
		t.Errorf("the sentence for a person is in the machine form:\n%s", machine.stdout)
	}

	human := leftOpen(t, root, configDir, "--format", "human")
	if first, _, _ := strings.Cut(human.stdout, "\n"); first != egress.NoRulesFile {
		t.Errorf("the list opens with %q, want %q", first, egress.NoRulesFile)
	}
}

// leftOpenNoLocationListed is the one line for a marks file that lists no
// location, whatever the reason: cleared marks and withheld ones read alike.
const leftOpenNoLocationListed = "yomihon: no question is listed from the marks file for this folder\n"

// No marks file and one that lists nothing both list nothing; stderr says
// which, and only that. A damaged one is refused without naming where it
// lives, since that path spells the home directory.
func TestLeftOpenTellsTheStatesOfTheMarksFileApart(t *testing.T) {
	t.Parallel()
	// The reader's marks live in the vault's own files, so each state has a
	// vault of its own: one vault shared between them would hand every state
	// the marks of the others.
	fresh := func() string {
		root := t.TempDir()
		putOpenThoughtFile(t, root, schema.ContractRelPath, leftOpenContract)
		putOpenThoughtFile(t, root, "Source.md", "# Source\n")
		return root
	}

	missingRoot, missing := fresh(), t.TempDir()
	clearedRoot, cleared := fresh(), t.TempDir()
	leftOpenMarks(t, clearedRoot, cleared, machinemarks.Mark{ID: markID(1), Kind: machinemarks.Question, Path: "Source.md", At: leftOpenDay(1)})
	keptMarks{t: t, root: clearedRoot, config: cleared}.forget(markID(1))
	// Marks that all lie in withheld ground list nothing, and must say exactly
	// what a file whose marks were all cleared says.
	withheldRoot, withheld := fresh(), t.TempDir()
	leftOpenMarks(t, withheldRoot, withheld,
		machinemarks.Mark{Kind: machinemarks.Question, Path: "Private/Secret plan.md", Anchor: "hidden-heading", At: leftOpenDay(1)},
		machinemarks.Mark{Kind: machinemarks.Question, Path: "Private/Gone.md", At: leftOpenDay(2)})
	// The earlier file of marks, which is read until the vault's own files take
	// it in, is where a reader's marks can still be damaged or out of reach.
	damagedRoot, damaged := fresh(), t.TempDir()
	damagedDir := earlierMarksDir(t, damaged, damagedRoot)
	if err := os.MkdirAll(damagedDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(damagedDir, "marks.json"), []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	// A file the machine refuses to open is the case whose own error names
	// the file's absolute path, and so the configuration directory's.
	lockedRoot, locked := fresh(), t.TempDir()
	writeEarlierMarks(t, locked, vaultNameOf(t, lockedRoot), machinemarks.Mark{ID: markID(1), Kind: machinemarks.Question, Path: "Source.md", At: leftOpenDay(1)})
	lockedFile := filepath.Join(earlierMarksDir(t, locked, lockedRoot), "marks.json")
	if err := os.Chmod(lockedFile, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(lockedFile, 0o600) }) //nolint:errcheck // TempDir removal reports what is left
	_, lockedErr := os.ReadFile(lockedFile)               // #nosec G304 -- a test's own path

	for _, tt := range []struct {
		name      string
		root      string
		configDir string
		want      leftOpenResult
	}{
		{name: "no marks file", root: missingRoot, configDir: missing, want: leftOpenResult{stderr: "yomihon: no question is listed; there is no marks file for this folder as spelled\n"}},
		{name: "every mark cleared", root: clearedRoot, configDir: cleared, want: leftOpenResult{stderr: leftOpenNoLocationListed}},
		{name: "only withheld marks", root: withheldRoot, configDir: withheld, want: leftOpenResult{stderr: leftOpenNoLocationListed}},
		{name: "damaged marks file", root: damagedRoot, configDir: damaged, want: leftOpenResult{exit: 2, stderr: "yomihon: the marks for this folder cannot be read; nothing was listed\n"}},
		{name: "unopenable marks file", root: lockedRoot, configDir: locked, want: leftOpenResult{exit: 2, stderr: "yomihon: the marks for this folder cannot be read; nothing was listed\n"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.configDir == locked && lockedErr == nil {
				t.Skip("this user can read a mode-0 file, so no read fails")
			}
			got := leftOpen(t, tt.root, tt.configDir, "--format", "json")
			if strings.Contains(got.stderr, tt.configDir) || strings.Contains(got.stderr, filepath.Base(tt.configDir)) {
				t.Errorf("caught: %s: stderr names the configuration directory: %q", tt.name, got.stderr)
			}
			if diff := cmp.Diff(tt.want, got, cmp.AllowUnexported(leftOpenResult{})); diff != "" {
				t.Errorf("caught: %s (-want +got):\n%s", tt.name, diff)
			}
		})
	}
	if entries, err := os.ReadDir(missing); err != nil || len(entries) != 0 {
		t.Errorf("caught: listing created %d entries under a configuration directory with no marks (err %v)", len(entries), err)
	}
	if _, err := os.Stat(filepath.Join(missingRoot, "reader-notes")); !os.IsNotExist(err) {
		t.Errorf("caught: listing made the reader folder in a vault with no marks (err %v)", err)
	}
	if data, err := os.ReadFile(filepath.Join(damagedDir, "marks.json")); err != nil || string(data) != "{broken" { // #nosec G304 -- a test's own path
		t.Errorf("caught: listing changed a damaged marks file: %q, %v", data, err)
	}
}

func TestLeftOpenRefusesWithoutAConfigurationDirectory(t *testing.T) {
	t.Parallel()
	root, _ := leftOpenVault(t)
	var stdout, stderr bytes.Buffer
	exit := runOpenThoughts(t.Context(), []string{"--root", root}, config{noConfigDir: "neither $XDG_CONFIG_HOME nor $HOME are defined"}, &stdout, &stderr, false)
	want := "yomihon: no configuration directory, so the marks for this folder cannot be found: neither $XDG_CONFIG_HOME nor $HOME are defined; nothing was listed\n"
	if exit != 2 || stdout.Len() != 0 || stderr.String() != want {
		t.Errorf("caught: exit %d, stdout %q, stderr %q; want 2, nothing, %q", exit, stdout.String(), stderr.String(), want)
	}
}

func TestLeftOpenRefusesWhatItDoesNotTake(t *testing.T) {
	t.Parallel()
	root, configDir := leftOpenVault(t)
	for _, tt := range []struct {
		args []string
		want string
	}{
		{args: []string{"--all"}, want: "yomihon: marks does not accept --all\n"},
		{args: []string{"--deny", "error"}, want: "yomihon: marks does not accept --deny\n"},
		{args: []string{"--baseline", "x"}, want: "yomihon: marks does not accept --baseline\n"},
		{args: []string{"Notes"}, want: "yomihon: marks takes no positional arguments\n"},
		{args: []string{"--format", "csv"}, want: "yomihon: invalid --format \"csv\"; use json, human, or md\n"},
		{args: []string{"--sort", "path"}, want: "yomihon: unknown flag \"--sort\"\n"},
	} {
		got := leftOpen(t, root, configDir, tt.args...)
		if got.exit != 2 || got.stdout != "" || got.stderr != tt.want {
			t.Errorf("marks %v: exit %d, stdout %q, stderr %q; want 2, nothing, %q", tt.args, got.exit, got.stdout, got.stderr, tt.want)
		}
	}
}

// The command reads and writes nothing: the vault and the marks file are the
// same bytes after it runs.
func TestLeftOpenChangesNothing(t *testing.T) {
	t.Parallel()
	root, configDir := leftOpenVault(t)
	before := treeBytes(t, root, configDir)
	if got := leftOpen(t, root, configDir, "--format", "json"); got.exit != 0 {
		t.Fatalf("exit %d: %s", got.exit, got.stderr)
	}
	if diff := cmp.Diff(before, treeBytes(t, root, configDir)); diff != "" {
		t.Errorf("caught: marks changed files (-before +after):\n%s", diff)
	}
}

func treeBytes(t *testing.T, roots ...string) map[string]string {
	t.Helper()
	files := make(map[string]string)
	for _, root := range roots {
		tree := os.DirFS(root)
		err := fs.WalkDir(tree, ".", func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			data, err := fs.ReadFile(tree, path)
			files[filepath.Join(root, path)] = string(data)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return files
}

// The help names the command at the top level and describes it in full.
func TestLeftOpenHasHelp(t *testing.T) {
	t.Parallel()
	text, handled, err := helpRequest([]string{openCommand, "--help"})
	if !handled || err != nil || !strings.HasPrefix(text, "Usage: yomihon marks ") || !strings.Contains(text, "--adopt") {
		t.Errorf("marks --help = %q, %t, %v", text, handled, err)
	}
	if !strings.Contains(topLevelHelp, "\n  yomihon marks [options]\n") {
		t.Errorf("top-level help does not list marks:\n%s", topLevelHelp)
	}
	if command, _ := dispatch([]string{openCommand}); command != openCommand {
		t.Errorf("dispatch(%q) = %q", openCommand, command)
	}
}

func FuzzParseOpenArgs(f *testing.F) {
	for _, seed := range []string{"--root\x00v", "--format=md", "--all", "--\x00x", "--format\x00csv", "--adopt\x00/x", "--adopt\x00x", "x", ""} {
		f.Add(seed, false)
	}
	f.Fuzz(func(t *testing.T, joined string, terminal bool) {
		var args []string
		if joined != "" {
			args = strings.Split(joined, "\x00")
		}
		request, err := parseOpenArgs(args, terminal)
		if err != nil {
			return
		}
		if request.format != judge.FormatJSON && request.format != judge.FormatHuman {
			t.Errorf("parseOpenArgs(%q) accepted format %v, which this command does not write", args, request.format)
		}
		if request.root == "" {
			t.Errorf("parseOpenArgs(%q) left the root unresolved", args)
		}
		if request.adopt != "" && !filepath.IsAbs(request.adopt) {
			t.Errorf("parseOpenArgs(%q) accepted the earlier path %q, which is not absolute", args, request.adopt)
		}
	})
}

// A note in allowed ground whose frontmatter cannot be read might be a thought
// still open, so the list is refused as it is for a note that cannot be read
// at all. The same note in withheld ground changes nothing.
func TestLeftOpenRefusesANoteWhoseFrontmatterCannotBeRead(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, content string
	}{
		{name: "unclosed quotation", content: "---\naliases: \"Broken\ntype: reflection\ndraft: true\n---\n# Broken\n\nA thought.\n"},
		{name: "unclosed draft list", content: "---\ntype: reflection\ndraft: [true\n---\n# Broken\n\nA thought.\n"},
		{name: "unclosed fence", content: "---\ncreated: 2026-09-01\ntype: reflection\ndraft: true\nA thought.\n"},
		{name: "draft not a boolean", content: "---\ntype: reflection\ndraft: \"true\"\n---\n# Broken\n\nA thought.\n"},
		{name: "type a list", content: "---\ntype: [reflection]\ndraft: true\n---\n# Broken\n\nA thought.\n"},
		{name: "type a number", content: "---\ntype: 1\ndraft: true\n---\n# Broken\n\nA thought.\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root, configDir := leftOpenPrivateVault(t)
			before := leftOpen(t, root, configDir, "--format", "json")
			putOpenThoughtFile(t, root, "Private/Broken.md", tt.content)
			if got := leftOpen(t, root, configDir, "--format", "json"); !cmp.Equal(before, got, cmp.AllowUnexported(leftOpenResult{})) {
				t.Errorf("caught: a withheld note changed the answer:\n%s", cmp.Diff(before, got, cmp.AllowUnexported(leftOpenResult{})))
			}
			putOpenThoughtFile(t, root, "Inside/Broken.md", tt.content)
			got := leftOpen(t, root, configDir, "--format", "json")
			want := leftOpenResult{
				exit:   2,
				stderr: "yomihon: Inside/Broken.md has frontmatter that could not be read, so whether it is a thought still open is unknown; nothing was listed\n",
			}
			if diff := cmp.Diff(want, got, cmp.AllowUnexported(leftOpenResult{})); diff != "" {
				t.Errorf("caught: %s (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

// The terminal view is one line per item whatever the vault's names and
// titles hold: a line break, an escape sequence or a direction override is
// shown as an escape, never written to the terminal as itself.
func TestLeftOpenHumanViewEscapesControlCharacters(t *testing.T) {
	t.Parallel()
	root, configDir := t.TempDir(), t.TempDir()
	putOpenThoughtFile(t, root, schema.ContractRelPath, leftOpenContract)
	putOpenThoughtFile(t, root, "Source.md", "# Source\n")
	putOpenThoughtFile(t, root, "Inside/Loud.md",
		"---\ntype: reflection\ndraft: true\ncreated: 2026-09-02\n---\n# \x1b]0;pwned\a \x1b[2J\u202Eevil\u200E\u200F\u061C\n\nA thought.\n")
	// A title's whitespace is collapsed, so a next-line character reaches the
	// list through a path instead.
	putOpenThoughtFile(t, root, "Inside/b\u0085c.md", openThoughtNote("Next line", "reflection", draftFlag, "2026-09-03"))
	putOpenThoughtFile(t, root, "Inside/a\n2099-01-01  thought   Inside/Forged.md  Forged.md",
		openThoughtNote("Two lines", "reflection", draftFlag, "2026-09-01"))
	got := leftOpen(t, root, configDir, "--format", "human")
	want := leftOpenResult{
		stdout: "2026-09-03  thought   Inside/b\\u0085c.md  Next line\n" +
			"2026-09-02  thought   Inside/Loud.md  \\u001b]0;pwned\\u0007 \\u001b[2J\\u202eevil\\u200e\\u200f\\u061c\n" +
			"2026-09-01  thought   Inside/a\\n2099-01-01  thought   Inside/Forged.md  Forged.md  Two lines\n",
		stderr: "yomihon: no question is listed; there is no marks file for this folder as spelled\n",
	}
	if diff := cmp.Diff(want, got, cmp.AllowUnexported(leftOpenResult{})); diff != "" {
		t.Errorf("caught: the terminal view wrote a control character as itself (-want +got):\n%s", diff)
	}
}

// A root that cannot be opened is refused without repeating the path the
// machine's error spells.
func TestLeftOpenRefusesAnUnopenableRootWithoutNamingIt(t *testing.T) {
	t.Parallel()
	missing := filepath.Join(t.TempDir(), "nobody-here")
	got := leftOpen(t, missing, t.TempDir(), "--format", "json")
	want := leftOpenResult{exit: 2, stderr: "yomihon: this folder cannot be opened as a vault; nothing was listed\n"}
	if diff := cmp.Diff(want, got, cmp.AllowUnexported(leftOpenResult{})); diff != "" {
		t.Errorf("caught: an unopenable root (-want +got):\n%s", diff)
	}
}

// Words an author writes without link syntax are the author's own and are
// listed unfiltered, a withheld note's bare file name included: filtering
// every title that happens to share a word with a withheld note would empty
// lists for reasons nobody could see. A title is its heading's words with
// their whitespace collapsed, so a tab arrives as one space and a heading
// ends at its line. These are the boundary the documentation states, so a
// change to it shows here.
func TestLeftOpenListsAnAuthorsPlainWordsUnfiltered(t *testing.T) {
	t.Parallel()
	root, configDir := t.TempDir(), t.TempDir()
	putOpenThoughtFile(t, root, schema.ContractRelPath, leftOpenContract)
	putOpenThoughtFile(t, root, "Private/Secret plan.md", "# Secret plan\n")
	written := []string{
		"Secret plan",
		"On [x](Secret plan.md)",
		"On Priv\nate/Secret plan",
		"On Private\t/Secret plan",
		"Private matters",
	}
	for i, title := range written {
		putOpenThoughtFile(t, root, fmt.Sprintf("Inside/Plain %d.md", i),
			fmt.Sprintf("---\ntype: reflection\ndraft: true\ncreated: 2026-09-0%d\n---\n# %s\n\nA thought.\n", i+1, title))
	}
	got := leftOpen(t, root, configDir, "--format", "json")
	var titles []string
	for _, item := range leftOpenItems(t, got.stdout) {
		title, _ := item["title"].(string) //nolint:errcheck // a missing title shows in the diff
		titles = append(titles, title)
	}
	slices.Reverse(titles)
	listed := []string{
		"Secret plan",
		"On [x](Secret plan.md)",
		"On Priv",
		"On Private /Secret plan",
		"Private matters",
	}
	if diff := cmp.Diff(listed, titles); diff != "" {
		t.Errorf("caught: an author's plain words were filtered (-want +got):\n%s", diff)
	}
}

// A withheld directory is matched as a name, not as letters inside another
// word: it counts only at the start of the text or after a character that is
// neither a letter nor a digit, judged once what draws nothing is gone.
func TestLeftOpenMatchesAWithheldDirectoryOnlyAsAName(t *testing.T) {
	t.Parallel()
	root, configDir := t.TempDir(), t.TempDir()
	putOpenThoughtFile(t, root, schema.ContractRelPath, strings.Replace(openThoughtContract, "never_egress_dirs = []", `never_egress_dirs = ["AI", "Caf\u00e9"]`, 1))
	for i, title := range []string{
		"OpenAI/GPT notes",
		"MAI/x",
		"AI/models",
		"On semi-ai/public",
		"On (AI/x)",
		// A filler letter draws nothing, so it puts no letter before the name;
		// a space that draws nothing leaves the name inside the word it joins.
		"On \u3164AI/models",
		"On x\u200bAI/models",
		// A variation selector between a letter and its accent would keep the
		// two from composing into the letter the directory's name is spelled with.
		"On Cafe\ufe0f\u0301/menu",
	} {
		putOpenThoughtFile(t, root, fmt.Sprintf("Inside/Named %d.md", i),
			fmt.Sprintf("---\ntype: reflection\ndraft: true\ncreated: 2026-09-%02d\n---\n# %s\n\nA thought.\n", i+1, title))
	}
	got := leftOpen(t, root, configDir, "--format", "json")
	var titles []string
	for _, item := range leftOpenItems(t, got.stdout) {
		title, _ := item["title"].(string) //nolint:errcheck // a missing title shows in the diff
		titles = append(titles, title)
	}
	slices.Reverse(titles)
	if diff := cmp.Diff([]string{"OpenAI/GPT notes", "MAI/x", "On x\u200bAI/models"}, titles); diff != "" {
		t.Errorf("caught: a withheld directory was matched inside a word, or missed as a name (-want +got):\n%s", diff)
	}
}

// movedFolder is a folder that moved. It withholds Private, and what the
// reader kept was kept for the path it had before: a question on a note it
// still has, one on a note in withheld ground that it still has, and two on
// notes it has not got, one of which was in withheld ground.
func movedFolder(t *testing.T) (root, configDir, earlier string) {
	t.Helper()
	root, configDir = t.TempDir(), t.TempDir()
	earlier = filepath.Join(t.TempDir(), "earlier", "vault")
	putOpenThoughtFile(t, root, schema.ContractRelPath, leftOpenContract)
	putOpenThoughtFile(t, root, "Source.md", "# Source\n\n## Chapter\n\nwords\n")
	putOpenThoughtFile(t, root, "Private/Secret plan.md", "# Secret plan\n\n## Hidden heading\n\nwords\n")
	writeEarlierMarks(t, configDir, earlier,
		machinemarks.Mark{ID: markID(1), Kind: machinemarks.Question, Path: "Source.md", Anchor: "chapter", At: leftOpenDay(1)},
		machinemarks.Mark{ID: markID(2), Kind: machinemarks.Question, Path: "Private/Secret plan.md", Anchor: "hidden-heading", At: leftOpenDay(2)},
		machinemarks.Mark{ID: markID(3), Kind: machinemarks.Question, Path: "Private/Gone.md", Anchor: "lost", At: leftOpenDay(3)},
		machinemarks.Mark{ID: markID(4), Kind: machinemarks.Question, Path: "NotHere.md", Anchor: "elsewhere", At: leftOpenDay(4)},
	)
	return root, configDir, earlier
}

func TestMarksAdoptListsWhatWasKeptForTheEarlierPath(t *testing.T) {
	t.Parallel()
	root, configDir, earlier := movedFolder(t)
	if got := leftOpen(t, root, configDir, "--format", "json"); got.exit != 0 || strings.Contains(got.stdout, "Source.md") {
		t.Fatalf("before adopting the folder already lists the marks: exit %d, %s", got.exit, got.stdout)
	}
	got := leftOpen(t, root, configDir, "--adopt", earlier, "--format", "json")
	if got.exit != 0 {
		t.Fatalf("exit %d: %s", got.exit, got.stderr)
	}
	if !strings.Contains(got.stdout, `"path":"Source.md"`) || !strings.Contains(got.stdout, `"anchor":"chapter"`) {
		t.Errorf("the question kept for the earlier path is not listed:\n%s", got.stdout)
	}
	if want := "yomihon: " + openAdoptedNotice + "\n"; got.stderr != want {
		t.Errorf("stderr = %q, want %q", got.stderr, want)
	}
	// Once adopted they are the folder's own, listed without being asked again.
	if again := leftOpen(t, root, configDir, "--format", "json"); again.exit != 0 || again.stdout != got.stdout {
		t.Errorf("after adopting, the list is %q, want %q", again.stdout, got.stdout)
	}
}

// What the folder withholds, and what it has not got, stays where it was kept.
// A path a list prints for a mark whose note is gone would otherwise be the way
// another folder's withheld ground reached this folder's output.
func TestMarksAdoptBringsAcrossNothingThisFolderWithholdsOrHasNot(t *testing.T) {
	t.Parallel()
	root, configDir, earlier := movedFolder(t)
	for _, format := range []string{"json", "human"} {
		got := leftOpen(t, root, configDir, "--adopt", earlier, "--format", format)
		if got.exit != 0 {
			t.Fatalf("--format %s: exit %d: %s", format, got.exit, got.stderr)
		}
		for _, word := range append([]string{"Gone", "NotHere", "elsewhere", "lost"}, leftOpenPrivateWords...) {
			if strings.Contains(got.stdout, word) || strings.Contains(got.stderr, word) {
				t.Errorf("caught: --format %s says %q of what the folder withholds or has not got:\nstdout %s\nstderr %s", format, word, got.stdout, got.stderr)
			}
		}
	}
	held := heldQuestions(t, root, configDir)
	if len(held) != 1 || held[0].Path != "Source.md" {
		t.Errorf("the folder holds %+v after adopting, want only the question on Source.md", held)
	}
}

// Adopting takes the marks into the vault's own machine files, so the reader
// folder is where it writes in the vault, and nowhere else: not a note, not the
// rules, and not the earlier marks.
func TestMarksAdoptLeavesTheEarlierMarksAndTheNotesAsTheyWere(t *testing.T) {
	t.Parallel()
	root, configDir, earlier := movedFolder(t)
	earlierDir := nameMarksDir(t, configDir, earlier)
	readerFolder := filepath.Join(root, "reader-notes")
	outsideReaderFolder := func() map[string]string {
		files := treeBytes(t, root)
		for path := range files {
			if strings.HasPrefix(path, readerFolder+string(filepath.Separator)) {
				delete(files, path)
			}
		}
		return files
	}
	beforeEarlier, beforeVault := treeBytes(t, earlierDir), outsideReaderFolder()
	if got := leftOpen(t, root, configDir, "--adopt", earlier); got.exit != 0 {
		t.Fatalf("exit %d: %s", got.exit, got.stderr)
	}
	if diff := cmp.Diff(beforeEarlier, treeBytes(t, earlierDir)); diff != "" {
		t.Errorf("the marks kept for the earlier path changed (-before +after):\n%s", diff)
	}
	if diff := cmp.Diff(beforeVault, outsideReaderFolder()); diff != "" {
		t.Errorf("caught: adopting wrote into the folder beyond the reader folder (-before +after):\n%s", diff)
	}
	if _, err := os.Stat(filepath.Join(readerFolder, ".yomihon", "marks")); err != nil {
		t.Errorf("adopting left no machine file of marks in the reader folder: %v", err)
	}
}

func TestMarksAdoptTwiceChangesNothingTheSecondTime(t *testing.T) {
	t.Parallel()
	root, configDir, earlier := movedFolder(t)
	first := leftOpen(t, root, configDir, "--adopt", earlier, "--format", "json")
	if first.exit != 0 {
		t.Fatalf("exit %d: %s", first.exit, first.stderr)
	}
	after := treeBytes(t, configDir)
	second := leftOpen(t, root, configDir, "--adopt", earlier, "--format", "json")
	if second.exit != 0 || second.stdout != first.stdout {
		t.Errorf("the second run is exit %d, %q; want exit 0, %q", second.exit, second.stdout, first.stdout)
	}
	if diff := cmp.Diff(after, treeBytes(t, configDir)); diff != "" {
		t.Errorf("the second adopting changed a file (-first +second):\n%s", diff)
	}
}

// Each refusal is fixed text that names no path, and writes nothing.
func TestMarksAdoptRefusesWhatItCannotAdoptWithoutNamingAPath(t *testing.T) {
	t.Parallel()
	root, configDir, earlier := movedFolder(t)
	source, err := vault.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	own := source.Name()
	if err = source.Close(); err != nil {
		t.Fatal(err)
	}
	// A directory copied by hand to the name another path gives records the path
	// it was made for.
	copiedRoot := filepath.Join(t.TempDir(), "copied")
	copiedDir := nameMarksDir(t, configDir, copiedRoot)
	data, err := os.ReadFile(filepath.Join(nameMarksDir(t, configDir, earlier), "marks.json")) // #nosec G304 -- a file of this test's own temporary directory
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(copiedDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(copiedDir, "marks.json"), data, 0o600); err != nil { // #nosec G703 -- fixture path under t.TempDir
		t.Fatal(err)
	}

	for _, tt := range []struct {
		name string
		path string
		want error
	}{
		{"a relative path", filepath.Join("earlier", "vault"), errOpenAdoptRelative},
		{"this folder's own path", own, errOpenAdoptOwn},
		{"a path with no marks kept for it", filepath.Join(t.TempDir(), "never"), errOpenAdoptNothing},
		{"a directory that records another folder", copiedRoot, errOpenAdoptOther},
	} {
		before := treeBytes(t, configDir)
		got := leftOpen(t, root, configDir, "--adopt", tt.path)
		if want := "yomihon: " + tt.want.Error() + "\n"; got.exit != 2 || got.stdout != "" || got.stderr != want {
			t.Errorf("%s: exit %d, stdout %q, stderr %q; want 2, nothing, %q", tt.name, got.exit, got.stdout, got.stderr, want)
		}
		if strings.Contains(got.stderr, filepath.Base(tt.path)) {
			t.Errorf("%s: the refusal names the path it was given: %q", tt.name, got.stderr)
		}
		if diff := cmp.Diff(before, treeBytes(t, configDir)); diff != "" {
			t.Errorf("%s: a refusal changed a file (-before +after):\n%s", tt.name, diff)
		}
	}

	var stdout, stderr bytes.Buffer
	exit := runOpenThoughts(t.Context(), []string{"--root", root, "--adopt", earlier}, config{noConfigDir: "the environment names none"}, &stdout, &stderr, false)
	if exit != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "no configuration directory") {
		t.Errorf("with no configuration directory: exit %d, stdout %q, stderr %q", exit, stdout.String(), stderr.String())
	}
}

func TestOnlyMarksTakesAdopt(t *testing.T) {
	t.Parallel()
	const want = "yomihon: usage: yomihon [dir] — or yomihon serve [dir] — or yomihon serve --root <dir>\n"
	if _, err := serveRoot([]string{"--adopt", filepath.Join(t.TempDir(), "x")}); err == nil || "yomihon: "+err.Error()+"\n" != want {
		t.Errorf("serve --adopt: %v, want the usage line", err)
	}
	for _, command := range []string{"check", "coverage", "exists"} {
		args := []string{"--adopt", filepath.Join(t.TempDir(), "x")}
		if command == "exists" {
			args = append(args, "candidate")
		}
		var stdout, stderr bytes.Buffer
		exit := runCommand(t.Context(), command, args, &stdout, &stderr, false)
		if wantLine := "yomihon: " + command + " does not accept --adopt\n"; exit != 2 || stdout.Len() != 0 || stderr.String() != wantLine {
			t.Errorf("%s --adopt: exit %d, stdout %q, stderr %q; want 2, nothing, %q", command, exit, stdout.String(), stderr.String(), wantLine)
		}
	}
}

// A reader who has not marked anything since the upgrade still has the marks the
// earlier file kept, and the command lists them: they are not hidden until a
// first write takes them into the vault, and the listing writes nothing, not
// even the reader folder.
func TestLeftOpenListsTheEarlierFilesQuestionsBeforeAnyWrite(t *testing.T) {
	t.Parallel()
	root, configDir := t.TempDir(), t.TempDir()
	putOpenThoughtFile(t, root, schema.ContractRelPath, leftOpenContract)
	putOpenThoughtFile(t, root, "Source.md", "# Source\n\n## Chapter\n\nwords\n")
	writeEarlierMarks(t, configDir, vaultNameOf(t, root),
		machinemarks.Mark{ID: markID(1), Kind: machinemarks.Question, Path: "Source.md", Anchor: "chapter", At: leftOpenDay(1)})
	before := treeBytes(t, root, configDir)
	got := leftOpen(t, root, configDir, "--format", "json")
	if got.exit != 0 || !strings.Contains(got.stdout, `"Source.md"`) || !strings.Contains(got.stdout, "chapter") {
		t.Fatalf("marks: exit %d, stdout %q, stderr %q; want the earlier file's question listed", got.exit, got.stdout, got.stderr)
	}
	if diff := cmp.Diff(before, treeBytes(t, root, configDir)); diff != "" {
		t.Errorf("caught: listing the earlier file's marks wrote files (-before +after):\n%s", diff)
	}
	if _, err := os.Stat(filepath.Join(root, "reader-notes")); !os.IsNotExist(err) {
		t.Errorf("caught: listing made the reader folder (err %v)", err)
	}
}
