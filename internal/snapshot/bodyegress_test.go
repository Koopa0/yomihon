package snapshot

import (
	"html"
	"net/url"
	"regexp"
	"slices"
	"strconv"
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
	pageMissing = regexp.MustCompile(`There is no (file|note) called &#34;([^&]*(?:&[^#][^&]*)*)&#34; yet`)
	// wikilinkTitle is the sentence a wikilink the page could not follow says
	// when a pointer rests on it, which names what the link was reaching for.
	wikilinkTitle = regexp.MustCompile(`class="wikilink[^"]*" title="([^"]*)"`)
	// spellsWithheldFolder is a withheld folder's name, or the last part of
	// one, written as a folder: at the start or after neither a letter nor a
	// digit, and before a slash or a backslash.
	spellsWithheldFolder = regexp.MustCompile(`(?i)(?:^|[^\pL\pN])(?:private|hidden)[/\\]`)
	quotedName           = regexp.MustCompile(`"((?:[^"\\]|\\.)*)"`)
)

// FuzzBodyLeavesNoLinkToWithheldGround holds the promise end to end: whatever
// body Body is given, rendering what it returns with the page's own renderer
// leaves no link, image or note address that goes into withheld ground, and no
// word of the page's saying that something is missing there.
func FuzzBodyLeavesNoLinkToWithheldGround(f *testing.F) {
	root := f.TempDir()
	contract := testContractDeclaringPrivacy(f, root, "Private", "Notes/Hidden")
	writeNote(f, root, "Private/Secret Plan.md", "---\ntype: note\n---\nthe plan\n")
	writeNote(f, root, "Private/Shared.md", "---\ntype: note\n---\nwithheld\n")
	writeNote(f, root, "Notes/Hidden/Deep.md", "---\ntype: note\n---\nwithheld below the top\n")
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
	sectionMissing, _, _ := strings.Cut(wording.SectionNotFoundFmt.In(wording.En), "%q")
	fileMissing, _, _ := strings.Cut(wording.UnwrittenFileFmt.In(wording.En), "%q")
	headingRead, _, _ := strings.Cut(wording.UnwrittenHeadingFmt.In(wording.En), "%q")

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
		"[[Sec%%x%%ret Plan]]",
		"[[Sec\ue000ret Plan]]",
		"[[Sec<!--x-->ret Plan]]",
		"[a](Pri%%x%%vate/Secret.md)",
		"[r]\n\n[r]: Pri%%c%%vate/p.md",
		"[[Deep]] [[Hidden/Deep]] [d](../Hidden/Deep.md) [m](../Hidden/Missing.md) [[Hidden/Missing]]",
		"[[De%%x%%ep]] [d](../Hid%%x%%den/Deep.md)",
		"[[Private/Missing]] [[Missing]] [[Plain#No Such Section]]",
		"<a href=\"Priv\tate/Secret.md\">x</a> <a href=\"Private\n/Secret.md\">y</a> <a href=\"Priv&Tab;ate/x.md\">z</a>",
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
		// A file is named by a path from the note, and a note by a name a
		// wikilink finds. A file is withheld ground when the path lands in a
		// withheld folder or spells one; the resolver also calls a path the
		// gate refuses for being malformed withheld, which is not ground anyone
		// withheld. A note is when the gate says its name spells withheld
		// ground or the name places a withheld note.
		withheldFile := func(path string) bool {
			resolved := graph.ResolveMarkdown(note, path, readable, func(string) bool { return false })
			return resolved.Withheld && inWithheldFolder(resolved.Relative) || spellsWithheldFolder.MatchString(path)
		}
		withheldName := func(name string) bool {
			if gate.Spells(name, "", generation) || inWithheldFolder(name) {
				return true
			}
			resolved := generation.Graph().Resolve(strings.TrimSpace(name))
			return slices.ContainsFunc(append(resolved.Candidates, resolved.RelPath), func(rel string) bool {
				return rel != "" && !readable(rel)
			})
		}
		for _, match := range pageMissing.FindAllStringSubmatch(page, -1) {
			name := htmlUnescape(match[2])
			if unquoted, err := strconv.Unquote(`"` + name + `"`); err == nil {
				name = unquoted
			}
			if match[1] == "file" && withheldFile(name) || match[1] == "note" && withheldName(name) {
				t.Fatalf("Body(%q) = %q, which renders that %q is missing", body, masked, name)
			}
		}
		for _, match := range wikilinkTitle.FindAllStringSubmatch(page, -1) {
			title := htmlUnescape(match[1])
			if strings.HasPrefix(title, sectionMissing) {
				// What this one quotes is a heading of the note the link
				// reached, which is the author's words and not a name a link
				// is followed by; the link's own address is read above.
				continue
			}
			// The same holds for the section an unwritten name's fragment was
			// read as, said after the name.
			title, _, _ = strings.Cut(title, headingRead)
			// The names the sentence quotes are read one by one below, unquoted,
			// since the quoting escapes a quote with a backslash that would
			// read as a slash; what is left, such as the files an ambiguous
			// name places, is read whole.
			if gate.Spells(quotedName.ReplaceAllString(title, `""`), "", generation) {
				t.Fatalf("Body(%q) = %q, which renders a wikilink titled %q", body, masked, title)
			}
			for _, quoted := range quotedName.FindAllStringSubmatch(title, -1) {
				name := quoted[1]
				if unquoted, err := strconv.Unquote(quoted[0]); err == nil {
					name = unquoted
				}
				if strings.HasPrefix(title, fileMissing) && withheldFile(name) ||
					!strings.HasPrefix(title, fileMissing) && withheldName(name) {
					t.Fatalf("Body(%q) = %q, which renders a wikilink titled %q", body, masked, title)
				}
			}
		}
	})
}

func htmlUnescape(s string) string { return html.UnescapeString(s) }

// inWithheldFolder is whether a vault path is under Private or Notes/Hidden,
// the folders the fuzz target withholds, in any case. A path the gate refuses
// for being malformed is not ground anyone withheld.
func inWithheldFolder(path string) bool {
	folded := strings.ToLower(path)
	return strings.HasPrefix(folded, "private/") || strings.HasPrefix(folded, "notes/hidden/")
}
