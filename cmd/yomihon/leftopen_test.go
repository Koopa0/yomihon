package main

import (
	"bytes"
	"encoding/json"
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
// would, admitting any place so a lost one can be stored too.
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
	putOpenThoughtFile(t, root, "Titled.md", "---\ntitle: Titled\n---\n# Titled\n\nbody\n")
	putOpenThoughtFile(t, root, "Inside/Thought.md", openThoughtNote("A thought", "reflection", "started", "2026-09-06"))
	putOpenThoughtFile(t, root, "Outside/Alternate.md", openThoughtNote("Alternative initial", "reflection", "revisit", "2026-09-08"))
	putOpenThoughtFile(t, root, "Inside/Done.md", openThoughtNote("Done note", "reflection", "settled", "2026-09-10"))
	putOpenThoughtFile(t, root, "Inside/Undated.md", "---\ntitle: Undated\ntype: reflection\nstatus: started\n---\nA thought.\n")
	leftOpenMarks(t, root, configDir,
		mark.Uncertainty{RelPath: "Source.md", Anchor: "chapter", At: leftOpenDay(7)},
		mark.Uncertainty{RelPath: "Source.md", At: leftOpenDay(5)},
		mark.Uncertainty{RelPath: "Source.md", Anchor: "vanished", At: leftOpenDay(4)},
		mark.Uncertainty{RelPath: "Source.md", Anchor: "twice", At: leftOpenDay(3)},
		mark.Uncertainty{RelPath: "Gone.md", Anchor: "x", At: leftOpenDay(2)},
		mark.Uncertainty{RelPath: "Titled.md", Anchor: "titled", At: leftOpenDay(1)},
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
		putOpenThoughtFile(t, root, rel, openThoughtNote("Thought "+rel, "reflection", "started", leftOpenDay(2*i+2).Format(time.DateOnly)))
	}
	putOpenThoughtFile(t, root, "Inside/Same day.md", openThoughtNote("Same day", "reflection", "revisit", leftOpenDay(4).Format(time.DateOnly)))
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
	root, configDir = t.TempDir(), t.TempDir()
	putOpenThoughtFile(t, root, schema.ContractRelPath, leftOpenContract)
	putOpenThoughtFile(t, root, "Source.md", "# Source\n\n## Chapter\n\nwords\n")
	putOpenThoughtFile(t, root, "Inside/Visible.md", openThoughtNote("Visible thought", "reflection", "started", "2026-09-01"))
	putOpenThoughtFile(t, root, "Private/Secret plan.md", "# Secret plan\n\n## Hidden heading\n\nwords\n")
	putOpenThoughtFile(t, root, "Private/Hidden thought.md", openThoughtNote("Hidden thought", "reflection", "started", "2026-09-02"))
	putOpenThoughtFile(t, root, "Inside/Cites private.md",
		"---\ntitle: Cites quietly\ntype: reflection\nstatus: started\nupdated: 2026-09-03\nbased_on: '[[Private/Secret plan#Hidden heading]]'\n---\nA thought.\n")
	putOpenThoughtFile(t, root, "Public.md", "# Public\n\n## About [[Private/Secret plan]]\n\nwords\n")
	leftOpenMarks(t, root, configDir,
		mark.Uncertainty{RelPath: "Source.md", Anchor: "chapter", At: leftOpenDay(4)},
		mark.Uncertainty{RelPath: "Private/Secret plan.md", Anchor: "hidden-heading", At: leftOpenDay(5)},
		mark.Uncertainty{RelPath: "Public.md", Anchor: "about-private-secret-plan", At: leftOpenDay(6)},
	)
	return root, configDir
}

// The words that would describe withheld ground: its directory, its file
// names, its headings and the anchors made from them.
var leftOpenPrivateWords = []string{"Private", "Secret", "secret", "Hidden", "hidden", "Cites"}

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
		for _, want := range []string{"Inside/Visible.md", "Source.md"} {
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
	"thought":  {"kind", "path", "status", "title", "update_date", "wikilink"},
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
	putOpenThoughtFile(t, root, "Private/Locked plan.md", openThoughtNote("Locked", "reflection", "started", "2026-09-09"))
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
	putOpenThoughtFile(t, root, "Inside/Locked.md", openThoughtNote("Locked", "reflection", "started", "2026-09-09"))
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
			putOpenThoughtFile(t, root, "Inside/Thought.md", openThoughtNote("A thought", "reflection", "started", "2026-09-01"))
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

// No marks file and an emptied one both list nothing; stderr says which. A
// damaged one is refused without naming where it lives, since that path
// spells the home directory.
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
		{name: "no marks file", configDir: missing, want: leftOpenResult{stderr: "yomihon: no location in this folder has been marked \"Not sure yet\" on this machine; there is no marks file yet\n"}},
		{name: "every mark cleared", configDir: cleared, want: leftOpenResult{stderr: "yomihon: the \"Not sure yet\" marks file for this folder holds no location; every mark has been cleared\n"}},
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
