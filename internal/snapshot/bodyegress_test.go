package snapshot

import (
	"html"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/egress"
	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestBodyMasksWhatTheGenerationSaysReachesWithheldGround holds the gate's
// masking against the two readers it is built from in production: the judge's
// extractor, and a generation that resolves each name the way the page does. The
// fakes in the egress package stand for both, so this is where they are checked
// against the real ones, on names a fake would not think of: one a withheld
// note and a readable one share, and a path that climbs. A link in a code span
// or an Obsidian comment is not one to the reader the page uses, and stays as
// the text it is.
func TestBodyMasksWhatTheGenerationSaysReachesWithheldGround(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	contract := testContractDeclaringPrivacy(t, root, "Private")
	writeNote(t, root, "Private/Secret Plan.md", "---\ntype: note\n---\nthe plan\n")
	writeNote(t, root, "Private/Shared.md", "---\ntype: note\n---\nwithheld\n")
	writeNote(t, root, "Notes/Shared.md", "---\ntype: note\n---\nreadable\n")
	writeNote(t, root, "Notes/Plain.md", "---\ntype: note\n---\nplain\n")
	const note = "Notes/Sub/Citing.md"
	writeNote(t, root, note, "---\ntype: note\n---\nbody\n")
	store, reader := newTestStore(t, root, contract)
	gate, err := egress.Open(t.Context(), reader, judge.LinkTargets)
	if err != nil {
		t.Fatalf("egress.Open() error = %v", err)
	}

	body := strings.Join([]string{
		"[[Secret Plan]] [[Secret Plan|the plan]] ![[Secret Plan#Goals]] [[secret plan]]",
		"[[Shared]] [[Notes/Shared]]",
		"[a](../../Private/Secret%20Plan.md) [b](<../../Private/Secret Plan.md>) ![c](/Private/fig.png)",
		"`[[Secret Plan]]` %% [[Secret Plan]] %%",
		"[[Plain]] [[Plain|shown]] [d](../Plain.md)",
		"`[[Plain]] [[Secret Plan]]`",
	}, "\n")
	want := strings.Join([]string{
		egress.Masked + " " + egress.Masked + " " + egress.Masked + " " + egress.Masked,
		egress.Masked + " [[Notes/Shared]]",
		egress.Masked + " " + egress.Masked + " " + egress.Masked,
		"`[[Secret Plan]]` %% [[Secret Plan]] %%",
		"[[Plain]] [[Plain|shown]] [d](../Plain.md)",
		"`[[Plain]] [[Secret Plan]]`",
	}, "\n")
	got, places, err := gate.Body(note, body, store.Current())
	if err != nil {
		t.Fatalf("Body() error = %v", err)
	}
	if got != want {
		t.Errorf("Body() mismatch:\n got: %q\nwant: %q", got, want)
	}
	if places != 8 {
		t.Errorf("Body() masked %d places, want 8", places)
	}
}

var (
	pageAddress = regexp.MustCompile(`(?:href|src)="(/(?:notes|raw)/[^"]*)"`)
	pageMissing = regexp.MustCompile(`There is no file called &#34;([^&]*(?:&[^#][^&]*)*)&#34; yet`)
)

// FuzzBodyLeavesNoLinkToWithheldGround holds the promise end to end: whatever
// body Body is given, rendering what it returns with the page's own renderer
// leaves no link, image or note address that goes into withheld ground, and no
// word of the page's saying that something is missing there.
func FuzzBodyLeavesNoLinkToWithheldGround(f *testing.F) {
	root := f.TempDir()
	contract := testContractDeclaringPrivacy(f, root, "Private")
	writeNote(f, root, "Private/Secret Plan.md", "---\ntype: note\n---\nthe plan\n")
	writeNote(f, root, "Private/Shared.md", "---\ntype: note\n---\nwithheld\n")
	writeNote(f, root, "Notes/Shared.md", "---\ntype: note\n---\nreadable\n")
	writeNote(f, root, "Notes/Plain.md", "---\ntype: note\n---\nplain\n")
	const note = "Notes/Sub/Citing.md"
	writeNote(f, root, note, "---\ntype: note\n---\nbody\n")
	store, reader := newTestStore(f, root, contract)
	gate, err := egress.Open(f.Context(), reader, judge.LinkTargets)
	if err != nil {
		f.Fatalf("egress.Open() error = %v", err)
	}
	generation := store.Current()
	readable := gate.Readable(generation)

	for _, seed := range []string{
		"[[Secret Plan]] ![[Secret Plan#Goals]] [[Shared]]",
		"[a](../../Private/Secret%20Plan.md) ![c](/Private/fig.png) [t](Private/Missing.md)",
		"> [q][r]\n>\n> [r]: Private/Secret.md",
		"- [q][r]\n\n  [r]: Private/Secret.md",
		"[r3]:\n  Private/Secret.md\n\n[r3]",
		"[dup][d]\n\n[d]: Private/Secret.md\n[d]: Notes/Plain.md",
		"[multi\nline text](Private/Secret.md)",
		"[a `]` b](Private/Secret.md)",
		`[t](Private\Secret.md)`,
		`<img srcset="Private/pic.png 2x"> <video poster="Private/pic.png"></video> <object data="Private/Secret.md"></object>`,
		"[t][r]\n\n[r]: <../../Private/Secret Plan.md>",
		"[ok](../Plain.md) [[Plain]] <https://example.com/Private/x>",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, body string) {
		masked, _, err := gate.Body(note, body, generation)
		if err != nil {
			t.Skip("a body with its frontmatter is refused, not masked")
		}
		page := generation.Render(note, masked, wording.En).HTML
		for _, match := range pageAddress.FindAllStringSubmatch(page, -1) {
			address := strings.TrimPrefix(strings.TrimPrefix(htmlUnescape(match[1]), "/notes/"), "/raw/")
			if decoded, err := url.PathUnescape(address); err == nil && (!readable(decoded) && inWithheldFolder(decoded)) {
				t.Fatalf("Body(%q) = %q, which renders a link to %q", body, masked, decoded)
			}
		}
		for _, match := range pageMissing.FindAllStringSubmatch(page, -1) {
			name := htmlUnescape(match[1])
			if gate.Spells(name, "", generation) ||
				graph.ResolveMarkdown(note, name, readable, func(string) bool { return false }).Withheld {
				t.Fatalf("Body(%q) = %q, which renders that %q is missing", body, masked, name)
			}
		}
	})
}

func htmlUnescape(s string) string { return html.UnescapeString(s) }

// inWithheldFolder is whether a vault path is under Private, the folder these
// tests withhold, in any case. A path the gate refuses for being malformed is
// not ground anyone withheld.
func inWithheldFolder(path string) bool {
	first, _, _ := strings.Cut(path, "/")
	return strings.EqualFold(first, "Private")
}
