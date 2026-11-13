package main

import (
	"bytes"
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

	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/mark"
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
	exit := runOpenThoughts(t.Context(), argv, config{configDir: configDir}, &stdout, &stderr, false)
	return leftOpenResult{stdout: stdout.String(), stderr: stderr.String(), exit: exit}
}

// leftOpenMarks keeps marks for root under configDir, as the reading room
// would, admitting any location so a lost one can be stored too.
func leftOpenMarks(t *testing.T, root, configDir string, marks ...mark.Uncertainty) *mark.File {
	t.Helper()
	source, err := vault.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	name := source.Name()
	if closeErr := source.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	file, err := mark.New(configDir, name)
	if err != nil {
		t.Fatal(err)
	}
	for i := range marks {
		if _, err := file.ToggleUncertainty(&marks[i], admitMark); err != nil {
			t.Fatal(err)
		}
	}
	return file
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
		mark.Uncertainty{RelPath: "Source.md", Anchor: "chapter", At: leftOpenDay(7)},
		// Marked west of UTC on the evening before: the list says the moment in
		// UTC, and so the day UTC gives it, never the zone the reader was in.
		mark.Uncertainty{RelPath: "Source.md", At: time.Date(2026, 9, 4, 20, 4, 5, 0, time.FixedZone("", -7*60*60))},
		mark.Uncertainty{RelPath: "Source.md", Anchor: "vanished", At: leftOpenDay(4)},
		mark.Uncertainty{RelPath: "Source.md", Anchor: "twice", At: leftOpenDay(3)},
		mark.Uncertainty{RelPath: "Gone.md", Anchor: "x", At: leftOpenDay(2)},
		mark.Uncertainty{RelPath: "Titled.md", Anchor: "titled", At: leftOpenDay(1)},
		mark.Uncertainty{RelPath: "Titled.md", Anchor: "gone-heading", At: leftOpenDay(0)},
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
			want, err := os.ReadFile(filepath.Join("testdata", "left-open", tt.golden))
			if err != nil {
				t.Fatal(err)
			}
			if got.exit != 0 || got.stderr != "" {
				t.Fatalf("left-open --format %s: exit %d, stderr %q", tt.format, got.exit, got.stderr)
			}
			if diff := cmp.Diff(string(want), got.stdout); diff != "" {
				t.Errorf("caught: left-open --format %s differs from %s (-want +got):\n%s", tt.format, tt.golden, diff)
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
	leftOpenMarks(t, root, configDir, mark.Uncertainty{RelPath: "Plain.md", Anchor: "part", At: leftOpenDay(1)})
	got := leftOpen(t, root, configDir, "--format", "json")
	if got.exit != 0 {
		t.Fatalf("left-open exit %d: %s", got.exit, got.stderr)
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
	for _, kept := range []mark.Uncertainty{
		{RelPath: "Source.md", Anchor: "chapter", At: leftOpenDay(5)},
		{RelPath: "Other.md", At: leftOpenDay(4)},
		{RelPath: "Source.md", At: leftOpenDay(1)},
	} {
		if _, err := marks.ToggleUncertainty(&kept, admitMark); err != nil {
			t.Fatal(err)
		}
	}
	page := openThoughtPage(t, site, "/open-thoughts", "en")
	var shelf []string
	for _, row := range regexp.MustCompile(`<a [^>]*href="/notes/([^"]*)"[^>]*data-index-row`).FindAllStringSubmatch(page, -1) {
		address, err := url.PathUnescape(row[1])
		if err != nil {
			t.Fatal(err)
		}
		shelf = append(shelf, address)
	}
	if len(shelf) == 0 {
		t.Fatalf("no shelf row was read from the page:\n%s", page)
	}
	got := leftOpen(t, root, filepath.Dir(filepath.Dir(filepath.Dir(marks.UncertaintyPath()))), "--format", "json")
	if got.exit != 0 {
		t.Fatalf("left-open exit %d: %s", got.exit, got.stderr)
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
		t.Errorf("caught: left-open and the shelf disagree on the open items or their order (-shelf +left-open):\n%s", diff)
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
	var marks []mark.Uncertainty
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
		marks = append(marks, mark.Uncertainty{RelPath: fmt.Sprintf("Raw %d.md", i), Anchor: "sec", At: leftOpenDay(7)})
	}
	// The heading the anchor was made from has been rewritten since; the
	// anchor still spells the withheld note it once cited.
	putOpenThoughtFile(t, root, "Renamed.md", "# Renamed\n\n## Other\n\nwords\n")
	// The note is gone, and with it any way to tell what its anchor's words cited.
	leftOpenMarks(t, root, configDir, append(marks,
		mark.Uncertainty{RelPath: "Source.md", Anchor: "chapter", At: leftOpenDay(4)},
		mark.Uncertainty{RelPath: "Private/Secret plan.md", Anchor: "hidden-heading", At: leftOpenDay(5)},
		mark.Uncertainty{RelPath: "Public.md", Anchor: "about-private-secret-plan", At: leftOpenDay(6)},
		mark.Uncertainty{RelPath: "Retitled.md", Anchor: "sec", At: leftOpenDay(7)},
		mark.Uncertainty{RelPath: "Renamed.md", Anchor: "about-private-secret-plan", At: leftOpenDay(8)},
		mark.Uncertainty{RelPath: "Deleted.md", Anchor: "about-private-secret-plan", At: leftOpenDay(9)},
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
	request := openRequest{root: root, configDir: configDir, format: judge.FormatJSON, afterAssemble: func() {
		putOpenThoughtFile(t, root, schema.ContractRelPath, narrowed)
	}}
	exit := request.run(t.Context(), &stdout, &stderr)
	if exit != 2 || stdout.Len() != 0 || strings.Contains(stdout.String(), "Inside/") {
		t.Errorf("caught: a contract narrowed after assembly still got the list: exit %d, stdout %q", exit, stdout.String())
	}
	if !strings.Contains(stderr.String(), judge.ErrPrivacyAuthorityUnavailable.Error()) {
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
		{name: "no contract", want: "yomihon: this folder has no vault contract, System/schemas/vault-schema.toml, and left-open lists nothing for a program to read until one declares [privacy]\n"},
		{name: "no privacy section", contract: undeclared, want: "yomihon: the vault contract declares no privacy policy, so left-open lists nothing for a program to read; add a [privacy] table with never_egress_dirs = [] to System/schemas/vault-schema.toml, naming in that list any directory whose notes must never leave this machine\n"},
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
			leftOpenMarks(t, root, configDir, mark.Uncertainty{RelPath: "Source.md", At: leftOpenDay(1)})
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

// leftOpenNoLocationListed is the one line for a marks file that lists no
// location, whatever the reason: cleared marks and withheld ones read alike.
const leftOpenNoLocationListed = "yomihon: no location is listed from the \"Not sure yet\" marks file for this folder\n"

// No marks file and one that lists nothing both list nothing; stderr says
// which, and only that. A damaged one is refused without naming where it
// lives, since that path spells the home directory.
func TestLeftOpenTellsTheStatesOfTheMarksFileApart(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	putOpenThoughtFile(t, root, schema.ContractRelPath, leftOpenContract)
	putOpenThoughtFile(t, root, "Source.md", "# Source\n")

	missing := t.TempDir()
	cleared := t.TempDir()
	file := leftOpenMarks(t, root, cleared, mark.Uncertainty{RelPath: "Source.md", At: leftOpenDay(1)})
	if _, err := file.ToggleUncertainty(&mark.Uncertainty{RelPath: "Source.md", At: leftOpenDay(1)}, admitMark); err != nil {
		t.Fatal(err)
	}
	// Marks that all lie in withheld ground list nothing, and must say exactly
	// what a file whose marks were all cleared says.
	withheld := t.TempDir()
	leftOpenMarks(t, root, withheld,
		mark.Uncertainty{RelPath: "Private/Secret plan.md", Anchor: "hidden-heading", At: leftOpenDay(1)},
		mark.Uncertainty{RelPath: "Private/Gone.md", At: leftOpenDay(2)})
	damaged := t.TempDir()
	broken := leftOpenMarks(t, root, damaged)
	if err := os.MkdirAll(filepath.Dir(broken.UncertaintyPath()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(broken.UncertaintyPath(), []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	// A file the machine refuses to open is the case whose own error names
	// the file's absolute path, and so the configuration directory's.
	locked := t.TempDir()
	unopenable := leftOpenMarks(t, root, locked, mark.Uncertainty{RelPath: "Source.md", At: leftOpenDay(1)})
	if err := os.Chmod(unopenable.UncertaintyPath(), 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(unopenable.UncertaintyPath(), 0o600) }) //nolint:errcheck // TempDir removal reports what is left
	_, lockedErr := os.ReadFile(unopenable.UncertaintyPath())

	for _, tt := range []struct {
		name      string
		configDir string
		want      leftOpenResult
	}{
		{name: "no marks file", configDir: missing, want: leftOpenResult{stderr: "yomihon: no location is listed; there is no \"Not sure yet\" marks file for this folder as spelled\n"}},
		{name: "every mark cleared", configDir: cleared, want: leftOpenResult{stderr: leftOpenNoLocationListed}},
		{name: "only withheld marks", configDir: withheld, want: leftOpenResult{stderr: leftOpenNoLocationListed}},
		{name: "damaged marks file", configDir: damaged, want: leftOpenResult{exit: 2, stderr: "yomihon: the \"Not sure yet\" marks for this folder cannot be read; nothing was listed\n"}},
		{name: "unopenable marks file", configDir: locked, want: leftOpenResult{exit: 2, stderr: "yomihon: the \"Not sure yet\" marks for this folder cannot be read; nothing was listed\n"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.configDir == locked && lockedErr == nil {
				t.Skip("this user can read a mode-0 file, so no read fails")
			}
			got := leftOpen(t, root, tt.configDir, "--format", "json")
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
	if data, err := os.ReadFile(broken.UncertaintyPath()); err != nil || string(data) != "{broken" {
		t.Errorf("caught: listing changed a damaged marks file: %q, %v", data, err)
	}
}

func TestLeftOpenRefusesWithoutAConfigurationDirectory(t *testing.T) {
	t.Parallel()
	root, _ := leftOpenVault(t)
	var stdout, stderr bytes.Buffer
	exit := runOpenThoughts(t.Context(), []string{"--root", root}, config{noConfigDir: "neither $XDG_CONFIG_HOME nor $HOME are defined"}, &stdout, &stderr, false)
	want := "yomihon: no configuration directory, so the \"Not sure yet\" marks for this folder cannot be found: neither $XDG_CONFIG_HOME nor $HOME are defined; nothing was listed\n"
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
		{args: []string{"--all"}, want: "yomihon: left-open does not accept --all\n"},
		{args: []string{"--deny", "error"}, want: "yomihon: left-open does not accept --deny\n"},
		{args: []string{"--baseline", "x"}, want: "yomihon: left-open does not accept --baseline\n"},
		{args: []string{"Notes"}, want: "yomihon: left-open takes no positional arguments\n"},
		{args: []string{"--format", "csv"}, want: "yomihon: invalid --format \"csv\"; use json, human, or md\n"},
		{args: []string{"--sort", "path"}, want: "yomihon: unknown flag \"--sort\"\n"},
	} {
		got := leftOpen(t, root, configDir, tt.args...)
		if got.exit != 2 || got.stdout != "" || got.stderr != tt.want {
			t.Errorf("left-open %v: exit %d, stdout %q, stderr %q; want 2, nothing, %q", tt.args, got.exit, got.stdout, got.stderr, tt.want)
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
		t.Errorf("caught: left-open changed files (-before +after):\n%s", diff)
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
	if !handled || err != nil || !strings.HasPrefix(text, "Usage: yomihon left-open [--root <dir>] [--format json|human|md]\n") {
		t.Errorf("left-open --help = %q, %t, %v", text, handled, err)
	}
	if !strings.Contains(topLevelHelp, "\n  yomihon left-open [options]\n") {
		t.Errorf("top-level help does not list left-open:\n%s", topLevelHelp)
	}
	if command, _ := dispatch([]string{openCommand}); command != openCommand {
		t.Errorf("dispatch(%q) = %q", openCommand, command)
	}
}

func FuzzParseOpenArgs(f *testing.F) {
	for _, seed := range []string{"--root\x00v", "--format=md", "--all", "--\x00x", "--format\x00csv", "x", ""} {
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
		stderr: "yomihon: no location is listed; there is no \"Not sure yet\" marks file for this folder as spelled\n",
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
