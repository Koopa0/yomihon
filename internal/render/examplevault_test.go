package render

import (
	"fmt"
	"math"
	"path"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/sequence"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/vaultfs"
	"github.com/koopa0/yomihon/internal/wording"
)

// The vault under examples/ is the one yomihon ships and the one a reader
// copies, so anything it never exercises is something nobody can see working.
// Until this test existed nothing said so: an editorial pass over the prose
// could delete the sentence that was also a capability's only demonstration,
// and the person making that edit had no way to tell. Twenty-two callout types
// went from two uses to none exactly that way. What follows is the signal that
// was missing — a failure here names the capability whose last demonstration
// just went out of the vault.
//
// It lives in this package because the reading dialect is the largest part of
// what an author writes and the part a prose rewrite touches, and because the
// callout vocabulary it ranges over is unexported here. The contract, the
// course grammar and the reading faces are read through the packages that own
// them, so no closed set is written down a second time.
//
// Adding a capability to yomihon means adding a demonstration here. Where a
// capability has a registration point in code, this test ranges over that
// point rather than over a list, so the list cannot fall behind.

const exampleVaultRoot = "../../examples/vault"

// surface is which of a note's two texts a capability is visible in. Some are
// only in what the author typed — an Obsidian comment is written to disappear
// — and the rest have to survive all the way to the reader.
type surface uint8

const (
	inSource surface = iota
	inReading
)

func (s surface) String() string {
	if s == inSource {
		return "what the author typed"
	}
	return "what the reader receives"
}

// authorFace is one thing a vault's author can write, named the way an editor
// about to delete the last one needs to hear it.
type authorFace struct {
	name  string
	where surface
	found func(text string) bool
}

// contains is the ordinary detector: a fragment the construct always produces.
func contains(fragment string) func(string) bool {
	return func(text string) bool { return strings.Contains(text, fragment) }
}

// dialectFaces is everything an author writes into a note body. The callout
// entries come from the renderer's own vocabulary, so a new bucket arrives
// here without anyone remembering to add it; the rest have no registration
// point in code to range over, and are named one per line.
func dialectFaces() []authorFace {
	faces := []authorFace{
		{"a foldable callout, written with - or +", inReading, contains(`<details class="callout`)},
		{"a [[wikilink]] that resolves", inReading, contains(`class="wikilink"`)},
		{"a link whose target nobody has written", inReading, contains(`class="wikilink-broken"`)},
		{"a link into a section, [[Note#Heading]]", inReading, linksIntoASection},
		{"a link to a named block, [[Note#^id]]", inReading, linksToABlock},
		{"a fragment address that missed, kept and reported", inReading, contains(`wikilink-degraded`)},
		{"a link written with an alias, [[Note|other words]]", inSource, aliasedWikilink},
		{"an embed, ![[Note]]", inReading, contains(`class="embed"`)},
		{"an embed of one section, ![[Note#Heading]]", inSource, fragmentEmbed},
		{"a named block, written as ^id at the end of a line", inReading, contains(`<span id="^`)},
		{"a mermaid diagram", inReading, contains(`class="mermaid-diagram"`)},
		{"an %%Obsidian comment%%, which the reader never sees", inSource, obsidianComment},
		{"==highlighted== words", inReading, contains(`<mark>`)},
		{"ruby, for furigana", inReading, contains(`<ruby>`)},
		{"a code fence whose language is coloured", inReading, contains(`class="chroma"`)},
		{"a table", inReading, contains(`class="y-tablewrap"`)},
		{"~~struck-out~~ words", inReading, contains(`<del>`)},
		{"a task list", inReading, contains(`type="checkbox"`)},
		{"a footnote", inReading, contains(`class="footnotes"`)},
		{"a heading, which a link can then address", inReading, contains(`<h2 id="`)},
		{"a picture from off the machine, kept as a link", inReading, contains(`referrerpolicy="no-referrer"`)},
		{"a paragraph marked to be read aloud", inReading, contains(`class="y-tts"`)},
	}
	// One per bucket rather than one per type, deliberately. The vocabulary's
	// twenty-two types render as three things a reader can tell apart, and
	// asking a twenty-note vault for twenty-two callouts would fill it with
	// callouts written for this test rather than for a reader. The buckets come
	// from the vocabulary itself, so a fourth one is demanded here the day it is
	// added. What this cannot see is a type that moved between groups, since
	// both spellings still produce a callout: that is held by the vocabulary's
	// own unit test beside it, which is where a per-type question belongs.
	seen := map[calloutBucket]bool{}
	for _, group := range calloutVocabulary {
		if seen[group.bucket] {
			continue
		}
		seen[group.bucket] = true
		faces = append(faces, authorFace{
			name:  fmt.Sprintf("a callout that reads as a %s, such as [!%s]", calloutClass(group.bucket), group.types[0]),
			where: inReading,
			found: contains(`class="callout callout-` + calloutClass(group.bucket) + `"`),
		})
	}
	return faces
}

// linksIntoASection and linksToABlock read the href the renderer writes rather
// than the address the author typed, so a form that stopped resolving stops
// counting. The two are told apart by the escaped caret a block address keeps.
func linksIntoASection(text string) bool { return hasResolvedFragment(text, false) }

func linksToABlock(text string) bool { return hasResolvedFragment(text, true) }

// hasResolvedFragment wants an address that landed, which is why it reads the
// class and not the href alone: a section the target does not have keeps its
// fragment in the href too, and is marked degraded instead. Without the class
// check one broken address answers for both the working form and the failing
// one, and a vault that had never shown a fragment resolve would look as
// though it had.
func hasResolvedFragment(text string, block bool) bool {
	for _, anchor := range noteAnchors(text) {
		if anchor.class != "wikilink" {
			continue
		}
		_, fragment, ok := strings.Cut(anchor.href, "#")
		if !ok || fragment == "" {
			continue
		}
		if strings.HasPrefix(fragment, "%5E") == block {
			return true
		}
	}
	return false
}

// noteAnchor is one reading-page link as this package emitted it.
type noteAnchor struct{ href, class string }

// noteAnchors collects every reading-page destination in one body's HTML with
// the classes it was given. It reads back the exact shape renderWikilink
// writes — href first, class second — rather than parsing arbitrary HTML.
func noteAnchors(text string) []noteAnchor {
	var out []noteAnchor
	for rest := text; ; {
		_, after, ok := strings.Cut(rest, `<a href="/notes/`)
		if !ok {
			return out
		}
		href, afterHref, ok := strings.Cut(after, `"`)
		if !ok {
			return out
		}
		rest = afterHref
		// An anchor this package wrote with no class is not a wikilink — the
		// line naming an embed's source note is one — so it carries no class
		// to read and is passed over.
		afterAttr, ok := strings.CutPrefix(afterHref, ` class="`)
		if !ok {
			continue
		}
		class, tail, ok := strings.Cut(afterAttr, `"`)
		if !ok {
			return out
		}
		out = append(out, noteAnchor{href: href, class: class})
		rest = tail
	}
}

// authorProse is the source with everything the dialect passes leave inert
// taken out: fenced blocks dropped, inline code spans blanked to spaces so
// every other offset stays where it was. The source detectors read this rather
// than raw bytes, because syntax an author quoted to show what it looks like is
// not syntax the vault uses — and the note that documents this dialect quotes
// every form it describes, so without this the documentation stands in for the
// demonstration. Fences and code spans are read with this package's own
// readers, the ones the wikilink and comment passes consult, so the test and
// the renderer never disagree about what is quoted.
func authorProse(source string) string {
	var kept []string
	inFence := false
	var fenceByte byte
	for line := range strings.SplitSeq(source, "\n") {
		if inFence {
			if fenceCloses(line, fenceByte) {
				inFence = false
			}
			continue
		}
		if marker, _, ok := fenceOpen(line); ok {
			inFence, fenceByte = true, marker
			continue
		}
		kept = append(kept, blankCodeSpans(line))
	}
	return strings.Join(kept, "\n")
}

func blankCodeSpans(line string) string {
	spans := codeSpanRanges(line)
	if len(spans) == 0 {
		return line
	}
	out := []byte(line)
	for _, span := range spans {
		for i := span[0]; i < span[1]; i++ {
			out[i] = ' '
		}
	}
	return string(out)
}

// aliasedWikilink and fragmentEmbed read the author's own text: both forms
// render as an ordinary link or embed, so the reading page cannot tell which
// spelling produced it.
func aliasedWikilink(text string) bool {
	return matchesWikilink(text, "[[", func(inner string) bool { return strings.Contains(inner, "|") })
}

func fragmentEmbed(text string) bool {
	return matchesWikilink(text, "![[", func(inner string) bool { return strings.Contains(inner, "#") })
}

func matchesWikilink(text, opener string, accept func(inner string) bool) bool {
	for rest := text; ; {
		_, after, ok := strings.Cut(rest, opener)
		if !ok {
			return false
		}
		inner, remainder, ok := strings.Cut(after, "]]")
		if !ok {
			return false
		}
		if !strings.Contains(inner, "\n") && accept(inner) {
			return true
		}
		rest = remainder
	}
}

// obsidianComment wants a closed one on a single line: an unclosed marker runs
// to the end of the file and is reported rather than demonstrated.
func obsidianComment(text string) bool {
	for line := range strings.SplitSeq(text, "\n") {
		if _, after, ok := strings.Cut(line, "%%"); ok && strings.Contains(after, "%%") {
			return true
		}
	}
	return false
}

// exampleVault is one reading of the shipped vault: every file as its author
// wrote it, every note as a reader receives it, and the contract that governs
// both.
type exampleVault struct {
	contract *schema.Contract
	notes    map[string]*vault.Note
	entries  []vaultfs.Entry
	source   string
	reading  string
}

func loadExampleVault(t *testing.T) *exampleVault {
	t.Helper()

	reader, err := vaultfs.Open(exampleVaultRoot)
	if err != nil {
		t.Fatalf("open the example vault at %s: %v", exampleVaultRoot, err)
	}
	t.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Errorf("close the example vault: %v", closeErr)
		}
	})

	contract, err := schema.Load(exampleVaultRoot)
	if err != nil {
		t.Fatalf("load the example vault's contract: %v", err)
	}
	scan, err := reader.ScanComplete(t.Context())
	if err != nil {
		t.Fatalf("scan the example vault: %v", err)
	}

	loaded := &exampleVault{contract: contract, notes: map[string]*vault.Note{}, entries: scan.Files()}
	notes := make([]*vault.Note, 0, len(loaded.entries))
	resources := make([]string, 0, len(loaded.entries))
	bodies := map[string]string{}
	var sources strings.Builder
	for _, entry := range loaded.entries {
		if path.Ext(entry.Path()) != ".md" {
			resources = append(resources, entry.Path())
			continue
		}
		data, readErr := reader.ReadFile(t.Context(), entry)
		if readErr != nil {
			t.Fatalf("read %s: %v", entry.Path(), readErr)
		}
		note := vault.Parse(entry.Path(), data)
		notes = append(notes, note)
		loaded.notes[entry.Path()] = note
		bodies[entry.Path()] = note.Body
		sources.WriteString(authorProse(string(data)))
		sources.WriteString("\n")
	}
	loaded.source = sources.String()

	pipeline := New(graph.New(notes, resources), transcluded(bodies), noDeclaredTitles{})
	var pages strings.Builder
	for _, note := range notes {
		result := pipeline.HTML(note.RelPath, note.Title(), note.Body, wording.En)
		pages.WriteString(result.HTML)
		// Read-aloud reaches a reader on a governed lesson and nowhere else,
		// so the marker only counts where the note page would honour it.
		if note.Type() == "lesson" && !contract.ArtifactPolicy().IsNonInstance(note.RelPath) {
			pages.WriteString(InjectTTS(result.HTML, wording.En))
		}
		pages.WriteString("\n")
	}
	loaded.reading = pages.String()
	return loaded
}

func (v *exampleVault) text(where surface) string {
	if where == inSource {
		return v.source
	}
	return v.reading
}

// frontmatterKeys is every key any note in the vault writes down.
func (v *exampleVault) frontmatterKeys() map[string]bool {
	keys := map[string]bool{}
	for _, note := range v.notes {
		for key := range note.Frontmatter {
			keys[key] = true
		}
	}
	return keys
}

// frontmatterValues gathers every value any note writes under one frontmatter
// key, reading a scalar and a list alike so an enum governing either shape is
// answered the same way.
func (v *exampleVault) frontmatterValues(key string) map[string]bool {
	values := map[string]bool{}
	for _, note := range v.notes {
		if value, ok := note.Text(key); ok {
			values[value] = true
		}
		for _, value := range note.Strings(key) {
			values[value] = true
		}
	}
	return values
}

// declaredEnumValues flattens one enum declaration. A type-conditional enum
// arrives as a map of groups, and every group's values are equally declared.
// A shape it has no reading for stops the test: returning nothing would make
// that enum demand nothing, and an enum silently exempted from the rule every
// other enum is held to is the failure this whole check exists to prevent.
func declaredEnumValues(t *testing.T, key string, enum reflect.Value) []string {
	t.Helper()
	switch declared := enum.Interface().(type) {
	case []string:
		return declared
	case map[string][]string:
		var out []string
		for _, group := range declared {
			out = append(out, group...)
		}
		return out
	default:
		t.Fatalf("the contract's %s enum is declared as %s, which this check has no reading for; "+
			"give it one, because as it stands that enum is asked for nothing at all", key, enum.Type())
		return nil
	}
}

func (v *exampleVault) noteTypes() map[string]bool {
	types := map[string]bool{}
	for _, note := range v.notes {
		types[note.Type()] = true
	}
	return types
}

type transcluded map[string]string

func (t transcluded) Transclusion(p string) (string, bool) {
	body, ok := t[p]
	return body, ok
}

type noDeclaredTitles struct{}

func (noDeclaredTitles) TitledBy(string) []string { return nil }

func TestTheExampleVaultStillDemonstratesEveryAuthorFace(t *testing.T) {
	t.Parallel()
	loaded := loadExampleVault(t)

	t.Run("reading dialect", func(t *testing.T) {
		t.Parallel()
		faces := dialectFaces()
		probes := probeCorpus(t)
		for _, face := range faces {
			// A detector that cannot match anything would report every
			// capability present, so each one answers a body written to
			// carry the construct before its answer about the vault counts.
			if !face.found(probes.text(face.where)) {
				t.Errorf("the detector for %q does not match %s in a body written to carry it, "+
					"so its answer about the example vault means nothing", face.name, face.where)
				continue
			}
			if !face.found(loaded.text(face.where)) {
				t.Errorf("nothing in examples/vault is %s any more.\n"+
					"\tIf you just removed one, it was the last: yomihon supports this and the "+
					"shipped vault no longer shows it working.\n"+
					"\tPut it back where an author would really write it, or delete the capability.",
					face.name)
			}
		}
		// The other half of that question, for the detectors that read what the
		// author typed: a body where each of those constructs appears only
		// inside a code span or a fence. One that answers yes to it counts a
		// description of a form as a use of it, and would stay green through
		// the deletion of the vault's last real one.
		//
		// Both halves are needed. Asking only that the detectors say no to the
		// stripped body is an assertion over whatever that body happens to
		// hold, and it holds nothing the day a quoted line is mistyped or a
		// fourth source detector arrives without one — so the construct is
		// required in the raw constant first, and refused only after stripping.
		quoted := authorProse(quotedProbeBody)
		for _, face := range faces {
			if face.where != inSource {
				continue
			}
			if !face.found(quotedProbeBody) {
				t.Errorf("the quoted body carries no %q for the stripper to take out, so refusing it "+
					"afterwards proves nothing about %q", face.name, face.name)
				continue
			}
			if face.found(quoted) {
				t.Errorf("the detector for %q matches syntax that is only quoted, so it cannot tell "+
					"a demonstration from a description of one", face.name)
			}
		}
	})

	t.Run("vault contract", func(t *testing.T) {
		t.Parallel()
		assertContractDeclaresEverythingItCanRead(t, loaded.contract)
		assertEveryDeclarationIsUsed(t, loaded)
	})

	t.Run("course grammar", func(t *testing.T) {
		t.Parallel()
		assertEveryWritableRoleIsUsed(t, loaded)
	})

	t.Run("reading faces", func(t *testing.T) {
		t.Parallel()
		assertEveryFaceHasSomethingOnIt(t, loaded)
	})
}

// assertContractDeclaresEverythingItCanRead walks the decoded contract's
// struct fields, so a key added to [enums], [fields], [rules] or [scan] is
// demanded here without this test being edited. Anything the example contract
// leaves at its zero value is a capability the vault cannot exercise whatever
// its notes contain.
//
// The reach stops at that struct, which is less than it looks. [navigation],
// [artifacts], [privacy], [supersession] and the lifecycle rows are decoded
// beside it rather than as fields of it, and reach a caller only through
// accessors, so each is named one at a time below — and a section added the
// same way would go undemanded until somebody adds it here too.
//
// A bool key is the one shape the zero-value question cannot ask properly: a
// key written "false" and a key left out decode alike. Both bools this
// contract carries are written true, which is the value that opens the
// capability behind them, so the walk is right about them; a contract that
// deliberately wrote false would need a different question.
func assertContractDeclaresEverythingItCanRead(t *testing.T, contract *schema.Contract) {
	t.Helper()

	definition := reflect.ValueOf(contract.Definition())
	for _, section := range reflect.VisibleFields(definition.Type()) {
		if len(section.Index) != 1 {
			continue
		}
		sectionValue := definition.FieldByIndex(section.Index)
		for _, key := range reflect.VisibleFields(sectionValue.Type()) {
			if sectionValue.FieldByIndex(key.Index).IsZero() {
				t.Errorf("the example contract declares no %s.%s, so no note in the vault can exercise it",
					strings.ToLower(section.Name), key.Tag.Get("toml"))
			}
		}
	}

	if _, declared := contract.Supersession(); !declared {
		t.Error("the example contract declares no [supersession], so the replacement ledger stays shut")
	}
	for _, section := range []struct {
		name      string
		available bool
	}{
		{"[navigation]", contract.NavigationRoles().Available()},
		{"[artifacts]", contract.ArtifactPolicy().Available()},
		{"[privacy]", contract.PrivacyPolicy().Available()},
	} {
		if !section.available {
			t.Errorf("the example contract's %s is unusable, so the capability behind it is off", section.name)
		}
	}
	if contract.StageCount() == 0 {
		t.Error("the example contract declares no [[lifecycle]] rows, so no status can be written")
	}
}

// assertEveryDeclarationIsUsed is the other half of the ruling this vault is
// held to: a contract may not name vocabulary the vault never writes. Every
// check below reads the contract for the set it walks, so widening the
// contract widens the demand rather than silently passing.
func assertEveryDeclarationIsUsed(t *testing.T, loaded *exampleVault) {
	t.Helper()

	definition := loaded.contract.Definition()
	written := loaded.frontmatterKeys()
	for _, key := range slices.Concat(definition.Fields.Known, definition.Fields.LessonOnly) {
		if !written[key] {
			t.Errorf("the contract allows the frontmatter key %q and no note in examples/vault writes it", key)
		}
	}

	// Every value of every enum, not merely every enum. The ruling this vault
	// is held to is that a contract may not name vocabulary no note writes, and
	// a declared value nobody has ever written is exactly that. Which
	// frontmatter key an enum governs is the enum's own toml name, so the
	// pairing is read off the decoder's tags rather than kept in a list here.
	enums := reflect.ValueOf(definition.Enums)
	for _, enum := range reflect.VisibleFields(enums.Type()) {
		key := enum.Tag.Get("toml")
		written := loaded.frontmatterValues(key)
		for _, value := range declaredEnumValues(t, key, enums.FieldByIndex(enum.Index)) {
			if !written[value] {
				t.Errorf("the contract lists %q among the values a note's %s may take, "+
					"and no note in examples/vault takes it", value, key)
			}
		}
	}

	// [privacy] and [artifacts] hand out no list, so each is asked what its
	// policy says about the paths this vault actually holds. That is all this
	// observes: whether the policy answers about a real file, not what any
	// page or response then does with the answer. A directory named in the
	// contract and absent from the vault fails here; a directory that is
	// present but wired into nothing would not.
	privacy := loaded.contract.PrivacyPolicy()
	if !slices.ContainsFunc(loaded.entries, func(e vaultfs.Entry) bool { return !privacy.EgressAllowed(e.Path()) }) {
		t.Error("the privacy policy denies egress to no path in examples/vault: every file the scan " +
			"found is allowed out, so never_egress_dirs names directories this vault does not have")
	}
	artifacts := loaded.contract.ArtifactPolicy()
	if !slices.ContainsFunc(loaded.entries, func(e vaultfs.Entry) bool { return artifacts.IsNonInstance(e.Path()) }) {
		t.Error("the artifact policy claims no path in examples/vault: no file the scan found sits " +
			"under non_instance_dirs, so it names a directory this vault does not have")
	}

	types := loaded.noteTypes()
	for _, exempt := range definition.Fields.DomainExempt {
		if !types[exempt] {
			t.Errorf("the contract exempts type %q from a domain and examples/vault holds no such note", exempt)
		}
	}
	if conceptType, declared := loaded.contract.ConceptType(); declared && !types[conceptType] {
		t.Errorf("the contract declares the type %q, which opens the concept sheets, "+
			"and examples/vault holds no note of that type", conceptType)
	}
	if inboxType, _, declared := loaded.contract.InboxRequiredFields(); declared && !types[inboxType] {
		t.Errorf("the contract states what a %q note must carry and examples/vault holds none", inboxType)
	}

	for _, root := range definition.Rules.DomainEqualsFolderUnder {
		if !slices.ContainsFunc(loaded.entries, func(e vaultfs.Entry) bool {
			return strings.HasPrefix(e.Path(), root+"/") && strings.Contains(strings.TrimPrefix(e.Path(), root+"/"), "/")
		}) {
			t.Errorf("the contract ties a note's domain to its folder under %q and examples/vault "+
				"files nothing in a folder there", root)
		}
	}
}

// assertEveryWritableRoleIsUsed ranges over every value the role type can hold
// and asks the parser which ones an author may write, so the three-value set
// comes from the grammar rather than from a list kept here.
func assertEveryWritableRoleIsUsed(t *testing.T, loaded *exampleVault) {
	t.Helper()

	used := map[sequence.Role]bool{}
	for _, note := range loaded.notes {
		if !loaded.contract.NavigationRoles().IsPathType(note.Type()) {
			continue
		}
		for _, group := range sequence.Parse(note.Body, 1).Groups {
			markUsedRoles(group, used)
		}
	}

	for value := 0; value <= math.MaxUint8; value++ {
		role := sequence.Role(value)
		if !role.Declared() || used[role] {
			continue
		}
		t.Errorf("no study path in examples/vault declares {sequence=%s}, so that branch kind is "+
			"documented in code and shown nowhere", role)
	}
}

func markUsedRoles(group *sequence.Group, used map[sequence.Role]bool) {
	used[group.Role] = true
	for _, item := range group.Items {
		if item.Branch != nil {
			markUsedRoles(item.Branch, used)
		}
	}
}

// assertEveryFaceHasSomethingOnIt builds the navigation model the server
// serves and asks it what a reader would find. A face with nothing on it is a
// page yomihon renders empty for anyone who copies this vault.
func assertEveryFaceHasSomethingOnIt(t *testing.T, loaded *exampleVault) {
	t.Helper()

	notes := make([]*vault.Note, 0, len(loaded.notes))
	for _, note := range loaded.notes {
		notes = append(notes, note)
	}
	slices.SortFunc(notes, func(a, b *vault.Note) int { return strings.Compare(a.RelPath, b.RelPath) })

	model := nav.New(
		loaded.entries,
		loaded.notes,
		graph.New(notes, nil),
		loaded.contract.NavigationRoles(),
		loaded.contract.KnowledgeScope(),
		loaded.contract.ArtifactPolicy(),
	)
	for _, face := range []struct {
		name string
		rows int
	}{
		{"study paths", model.PathCount()},
		{"maps", model.MapCount()},
		{"reports", len(model.Reports())},
		{"journal", len(model.Journal())},
		{"folders", len(model.Folders())},
	} {
		if face.rows == 0 {
			t.Errorf("the %s face has nothing on it in examples/vault, so that page opens empty "+
				"for anyone who starts from this vault", face.name)
		}
	}
}

// probeCorpus is a body written to carry every construct the dialect table
// looks for, rendered through this package the way a note is. It is what each
// detector is tried against first: a check that cannot fail proves nothing,
// and a detector reading for a fragment the renderer stopped emitting would
// otherwise report the vault empty of a capability it still demonstrates.
func probeCorpus(t *testing.T) *exampleVault {
	t.Helper()

	var body strings.Builder
	body.WriteString(probeBody)
	for _, group := range calloutVocabulary {
		fmt.Fprintf(&body, "\n> [!%s] A probe\n> Its body.\n", group.types[0])
	}
	fmt.Fprintf(&body, "\n> [!%s]- Folded shut\n> Its body.\n", calloutVocabulary[0].types[0])

	idx := graph.BuildFromNotes([]graph.NoteInput{{RelPath: "Notes/Probe Target.md"}}, nil)
	bodies := transcluded{"Notes/Probe Target.md": "## Probe Section\n\nA paragraph the probe names. ^probe-block\n"}
	rendered := New(idx, bodies, noDeclaredTitles{}).HTML("Notes/Probe.md", "Probe", body.String(), wording.En)
	return &exampleVault{source: authorProse(body.String()), reading: InjectTTS(rendered.HTML, wording.En)}
}

const probeBody = "" +
	"A [[Probe Target]] link, an aliased [[Probe Target|other words]] one, and a\n" +
	"[[Name nobody wrote]] that resolves to nothing.\n" +
	"\n" +
	"Into a section: [[Probe Target#Probe Section]]. To a block:\n" +
	"[[Probe Target#^probe-block]]. Missing: [[Probe Target#Nowhere]].\n" +
	"\n" +
	"![[Probe Target]]\n" +
	"\n" +
	"![[Probe Target#Probe Section]]\n" +
	"\n" +
	"## A heading\n" +
	"\n" +
	"==Highlighted== words, ~~struck-out~~ words, a footnote[^p], a\n" +
	"<ruby>漢<rt>かん</rt></ruby> reading, and %%a comment nobody reads%%.\n" +
	"\n" +
	"[^p]: The note at the foot of the page.\n" +
	"\n" +
	"- [ ] something to do\n" +
	"- [x] something done\n" +
	"\n" +
	"| Column | Value |\n" +
	"| --- | --- |\n" +
	"| one | 1 |\n" +
	"\n" +
	"```go\nfunc probe() int { return 1 }\n```\n" +
	"\n" +
	"```mermaid\nflowchart LR\n  a --> b\n```\n" +
	"\n" +
	"![a picture off the machine](https://example.invalid/probe.png)\n" +
	"\n" +
	"A paragraph this probe names. ^probe-block\n" +
	"\n" +
	"<!-- read-aloud: ja -->\n" +
	"古池や\n"

// quotedProbeBody writes every construct the source detectors look for, and
// writes each one only as something being shown: once in a code span, once
// inside a fence. Nothing in it is a use.
const quotedProbeBody = "" +
	"Shown rather than used: `[[Probe Target|other words]]`, `![[Probe Target#Probe Section]]`,\n" +
	"and `%%a comment nobody reads%%`.\n" +
	"\n" +
	"```\n" +
	"[[Probe Target|other words]]\n" +
	"![[Probe Target#Probe Section]]\n" +
	"%%a comment nobody reads%%\n" +
	"```\n"
