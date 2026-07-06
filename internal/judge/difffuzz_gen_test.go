package judge

// A seeded, deterministic vault generator for the differential harness. Given a
// seed it plans one small synthetic vault — a contract file and a handful of
// notes — biased toward the judge's rule surfaces rather than uniform noise, so
// the bytes actually exercise the schema, graph, and disk-reference checks and
// not only the malformed-frontmatter path. Every choice is drawn from one
// seeded generator in a fixed order and no clock is read, so the same seed
// plans a byte-identical vault and a failure needs only the seed to reproduce.
//
// Alongside the vault it returns a manifest recording the constructs whose
// two-engine treatment deliberately differs: a path reference sealed in a
// comment, a public note linking a journal note's title, a journal note typed
// as a concept. Those departures are invisible in a finding's structured
// fields, so the manifest — not a guess from the output — is what lets the
// comparison drop exactly the reference lines this engine is expected to omit.

import (
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/koopa0/kurodo/internal/schema"
)

// genNote is one planned file: its vault-relative slash path and its full
// bytes. The fields are exported so the determinism check can compare two plans
// with a structural diff.
type genNote struct {
	Path string
	Body string
}

// pathTarget names one finding site by the two fields the comparison keys on:
// the note that carries it and the reference text it cites.
type pathTarget struct {
	path   string
	target string
}

// diaryConcept is one journal note typed as a concept. It is always planted so
// that nothing links to it, which fixes its coverage state as an orphan; the
// domain is the coverage row its removal must adjust, since a concept's domain
// is not visible in the coverage list entries.
type diaryConcept struct {
	domain string
}

// genManifest records what a plan planted that the two engines treat
// differently, plus the material an existence query draws on. It is the precise
// alternative to filtering the reference output by heuristic.
type genManifest struct {
	seed int64

	// commentPathRefs are path references planted inside a %%...%% comment. The
	// reference binary reports each as a dead-path finding; this engine does not
	// check a path reference in a comment, so the reference's line at each site
	// is dropped before the byte-diff.
	commentPathRefs []pathTarget

	// diaryTitleLinks are public notes whose wikilink names a journal note's
	// title. This engine drops such a finding at its source so the journal path
	// never reaches the evidence, and the drop leaves no trace in the finding's
	// structured fields, so each site is recorded here by note and link text.
	diaryTitleLinks []pathTarget

	// diaryConcepts are journal notes typed as a concept. Coverage counts them
	// on the reference and drops them here.
	diaryConcepts []diaryConcept

	// hasDiary is true when the plan placed any note under the journal, which
	// switches the coverage and existence comparisons from a raw byte-diff to
	// the normalized form.
	hasDiary bool

	// emptyDomainConcepts counts concepts written with an explicit empty domain;
	// coverage folds them into the "(none)" group here and the reference keeps
	// them as a separate empty-named group.
	emptyDomainConcepts int

	// publicNames resolve to a public note (existence queries that should hit);
	// diaryNames name journal notes, whose existence this engine hides.
	publicNames []string
	diaryNames  []string

	// categories tallies which planted constructs a plan carries, so a run can
	// confirm the generator reached every surface — including the no-frontmatter
	// classification, which emits no finding to count in the output.
	categories map[string]int
}

func (m *genManifest) note(cat string) { m.categories[cat]++ }

// The focus categories. Each round guarantees one of these by round index, so a
// run of at least focusCount rounds plants every one at least once; the rest of
// each vault is drawn from the safe subset for breadth.
const (
	focusEnumType = iota
	focusEnumStatus
	focusEnumValue
	focusRequired
	focusUnknownKey
	focusSlug
	focusDomainFolder
	focusLegacyTag
	focusProvenance
	focusBadYAML
	focusDupKey
	focusNoFrontmatter
	focusLinkBrokenWarn
	focusLinkBrokenInfo
	focusTitleNotAlias
	focusCollisionAlias
	focusProvenanceUnresolved
	focusMapMismatch
	focusMapUnlisted
	focusDeadPath
	focusExternalPath
	focusCommentPathRef
	focusDiaryChannels
	focusDiaryConcept
	focusEmptyDomain
	focusCount
)

// safeFocuses are the categories with no engine-specific treatment, so they are
// free to appear as breadth in any round without a manifest entry. The
// journal, comment, and empty-domain categories are excluded: they carry a
// deliberate divergence and appear only as a round's guaranteed focus, which
// keeps a share of rounds free of them for the strict byte-diff path.
var safeFocuses = []int{
	focusEnumType, focusEnumStatus, focusEnumValue, focusRequired, focusUnknownKey,
	focusSlug, focusDomainFolder, focusLegacyTag, focusProvenance, focusBadYAML,
	focusDupKey, focusNoFrontmatter, focusLinkBrokenWarn, focusLinkBrokenInfo,
	focusTitleNotAlias, focusCollisionAlias, focusProvenanceUnresolved,
	focusMapMismatch, focusMapUnlisted, focusDeadPath, focusExternalPath,
}

// planner threads the seeded generator, the growing note list, the manifest,
// and a monotonic id through one plan. The id makes every planted name globally
// unique within the vault, so a construct meant to resolve to nothing is never
// resolved by an unrelated note.
type planner struct {
	rng   *rand.Rand
	gc    genContract
	notes []genNote
	man   genManifest
	uid   int
}

func (p *planner) id() string {
	p.uid++
	return fmt.Sprintf("u%03d", p.uid)
}

func (p *planner) add(path, body string) {
	p.notes = append(p.notes, genNote{Path: path, Body: body})
}

// planVault plans one vault for a seed: a varied contract, a few baseline valid
// notes, the round's guaranteed focus plus breadth, and the manifest. It is
// pure — no disk, one seeded generator, a fixed draw order — so two calls with
// the same seed return equal plans.
func planVault(seed int64) (contract string, notes []genNote, man genManifest) {
	// #nosec G404 G115 -- a deterministic reproduction from a seed needs a seeded, non-cryptographic generator; the seed is a small test index, so the unsigned conversion cannot overflow in practice
	rng := rand.New(rand.NewPCG(uint64(seed), 0x9e3779b97f4a7c15))
	p := &planner{
		rng: rng,
		man: genManifest{seed: seed, categories: map[string]int{}},
	}
	p.gc = buildContract(rng)

	p.baseline()

	round := int(seed)
	if round < 0 {
		round = -round
	}
	guaranteed := round % focusCount
	p.plant(guaranteed)
	for extra := 2 + rng.IntN(4); extra > 0; extra-- {
		p.plant(safeFocuses[rng.IntN(len(safeFocuses))])
	}

	return contractTOML(&p.gc), p.notes, p.man
}

// baseline plants a valid concept mounted by a map, a valid concept referenced
// only by another concept, and a true orphan, so every run has resolvable link
// targets and a non-trivial coverage report to compare on the strict path.
func (p *planner) baseline() {
	dom := p.gc.domains[0]
	mounted := "Anchor-" + p.id()
	pending := "Anchor-" + p.id()
	orphan := "Anchor-" + p.id()

	p.add("Concepts/"+dom+"/"+mounted+".md", validConcept(&p.gc, dom, mounted))
	p.add("Concepts/"+dom+"/"+pending+".md", validConcept(&p.gc, dom, pending))
	p.add("Concepts/"+dom+"/"+orphan+".md", validConcept(&p.gc, dom, orphan))
	p.man.publicNames = append(p.man.publicNames, mounted, pending, orphan)

	// A map note mounts the first concept; a second concept references the
	// second, leaving it pending-mount and the third an orphan.
	moc := "Map-" + p.id()
	p.add("Maps/"+moc+".md", frontmatter(
		"title: "+moc,
		"type: moc",
		"domain: "+dom,
		"status: evergreen",
	)+"\nlinks [["+mounted+"]]\n")

	ref := "Concepts/" + dom + "/Ref-" + p.id() + ".md"
	p.add(ref, validConcept(&p.gc, dom, "Ref")+"\nsee [["+pending+"]]\n")
}

// plant grows the vault with the construct that reaches one focus category and
// records any manifest entry the construct needs.
func (p *planner) plant(focus int) {
	dom := p.gc.domains[0]
	switch focus {
	case focusEnumType:
		id := p.id()
		p.add("Concepts/"+dom+"/Type-"+id+".md", frontmatter(
			"title: Bad Type "+id,
			"type: "+p.gc.absentToken("type"),
			"domain: "+dom,
			"status: "+p.gc.noteStatus[0],
		)+"\nbody\n")
		p.man.note("schema.enum")

	case focusEnumStatus:
		id := p.id()
		// A concept whose only fault is a status outside the note status set.
		p.add("Concepts/"+dom+"/Status-"+id+".md", frontmatter(
			"title: Bad Status "+id,
			"type: concept",
			"domain: "+dom,
			"status: "+p.gc.absentToken("status"),
			"source_locator: somewhere",
		)+"\nbody\n")
		p.man.note("schema.enum")

	case focusEnumValue:
		id := p.id()
		// Placed under Sources/ so a bad domain reaches the value-enum rule
		// without also tripping the folder rule, which is scoped to Concepts/.
		p.add("Sources/Val-"+id+".md", frontmatter(
			"title: Bad Value "+id,
			"type: source-note",
			"domain: "+p.gc.absentToken("domain"),
			"status: "+p.gc.noteStatus[0],
		)+"\nbody\n")
		p.man.note("schema.enum")

	case focusRequired:
		id := p.id()
		// A concept with provenance but no status: the status requirement is
		// the only fault, so the required rule is reached cleanly.
		p.add("Concepts/"+dom+"/Req-"+id+".md", frontmatter(
			"title: Missing Status "+id,
			"type: concept",
			"domain: "+dom,
			"source_locator: somewhere",
		)+"\nbody\n")
		p.man.note("schema.required")

	case focusUnknownKey:
		id := p.id()
		p.add("Concepts/"+dom+"/Unknown-"+id+".md", validConceptFields(&p.gc, dom, "Unknown "+id,
			p.gc.absentToken("field")+": x")+"\nbody\n")
		p.man.note("schema.unknown_key")

	case focusSlug:
		id := p.id()
		p.add("Writing/Slug-"+id+".md", frontmatter(
			"title: Bad Slug "+id,
			"type: lesson",
			"domain: "+dom,
			"status: "+p.gc.lessonStatus[0],
			"slug: NOT a valid slug "+id,
		)+"\nbody\n")
		p.man.note("schema.slug")

	case focusDomainFolder:
		id := p.id()
		other := p.gc.otherDomain(dom)
		// Filed under one domain's folder but declaring another valid domain.
		p.add("Concepts/"+dom+"/Folder-"+id+".md", frontmatter(
			"title: Folder Mismatch "+id,
			"type: concept",
			"domain: "+other,
			"status: "+p.gc.noteStatus[0],
			"source_locator: somewhere",
		)+"\nbody\n")
		p.man.note("schema.domain_folder")

	case focusLegacyTag:
		id := p.id()
		p.add("Concepts/"+dom+"/Tag-"+id+".md", validConceptFields(&p.gc, dom, "Legacy Tag "+id,
			"tags:\n  - type/concept")+"\nbody\n")
		p.man.note("schema.legacy_tag")

	case focusProvenance:
		id := p.id()
		// A concept with neither based_on nor source_locator.
		p.add("Concepts/"+dom+"/Prov-"+id+".md", frontmatter(
			"title: No Provenance "+id,
			"type: concept",
			"domain: "+dom,
			"status: "+p.gc.noteStatus[0],
		)+"\nbody\n")
		p.man.note("schema.provenance")

	case focusBadYAML:
		id := p.id()
		p.add("Concepts/"+dom+"/BadYAML-"+id+".md",
			"---\ntitle: unterminated ["+id+"\n---\nbody\n")
		p.man.note("schema.frontmatter")

	case focusDupKey:
		id := p.id()
		p.add("Concepts/"+dom+"/Dup-"+id+".md",
			"---\ntitle: A "+id+"\ntitle: B "+id+"\ntype: concept\n---\nbody\n")
		p.man.note("schema.frontmatter")

	case focusNoFrontmatter:
		id := p.id()
		p.add("Concepts/"+dom+"/Raw-"+id+".md",
			"a raw transcript body with no frontmatter block "+id+"\n")
		p.man.note("noFrontmatter")

	case focusLinkBrokenWarn:
		id := p.id()
		target := "Missing Target " + id
		p.add("Concepts/"+dom+"/Broken-"+id+".md", validConcept(&p.gc, dom, "Broken "+id)+
			"\nlinks [["+target+"]]\n")
		p.man.note("link.broken")

	case focusLinkBrokenInfo:
		id := p.id()
		target := "Planned Target " + id
		p.add("Concepts/"+dom+"/Planned-"+id+".md", validConcept(&p.gc, dom, "Planned "+id)+
			"\n## 缺口\n\nlinks [["+target+"]]\n")
		p.man.note("link.broken")

	case focusTitleNotAlias:
		id := p.id()
		title := "Owned Title " + id
		// One note owns the title by frontmatter; a filename different from the
		// title means the linker resolves neither the filename nor an alias.
		p.add("Concepts/"+dom+"/Owner-"+id+".md", frontmatter(
			"title: "+title,
			"type: concept",
			"domain: "+dom,
			"status: "+p.gc.noteStatus[0],
			"source_locator: somewhere",
		)+"\nbody\n")
		p.add("Concepts/"+dom+"/Linker-"+id+".md", validConcept(&p.gc, dom, "Linker "+id)+
			"\nlinks [["+title+"]]\n")
		p.man.publicNames = append(p.man.publicNames, title)
		p.man.note("link.title_not_alias")

	case focusCollisionAlias:
		id := p.id()
		alias := "Shared Alias " + id
		p.add("Concepts/"+dom+"/AliasA-"+id+".md", validConceptFields(&p.gc, dom, "Alias A "+id,
			"aliases:\n  - "+alias)+"\nbody\n")
		p.add("Concepts/"+dom+"/AliasB-"+id+".md", validConceptFields(&p.gc, dom, "Alias B "+id,
			"aliases:\n  - "+alias)+"\nbody\n")
		p.man.note("collision.alias")

	case focusProvenanceUnresolved:
		id := p.id()
		p.add("Concepts/"+dom+"/ProvRef-"+id+".md", validConceptFields(&p.gc, dom, "Prov Ref "+id,
			"based_on:\n  - Missing Provenance "+id)+"\nbody\n")
		p.man.note("provenance.unresolved")

	case focusMapMismatch:
		id := p.id()
		p.add("Maps/Syllabus-"+id+".md", frontmatter(
			"title: Syllabus "+id,
			"type: study-path",
			"domain: "+dom,
			"status: evergreen",
		)+"\nlists [[Missing Lesson "+id+"]]\n")
		p.man.note("map.disk_mismatch")

	case focusMapUnlisted:
		id := p.id()
		ld := p.gc.domains[0]
		// A syllabus for a domain and a ready lesson of that domain it omits.
		p.add("Maps/PathU-"+id+".md", frontmatter(
			"title: Path "+id,
			"type: study-path",
			"domain: "+ld,
			"status: evergreen",
		)+"\nno links here\n")
		p.add("Writing/Lesson-"+id+".md", frontmatter(
			"title: Lesson "+id,
			"type: lesson",
			"domain: "+ld,
			"status: "+p.gc.lessonReady(),
			"slug: lesson-"+strings.ToLower(id),
		)+"\nbody\n")
		p.man.note("map.disk_unlisted")

	case focusDeadPath:
		id := p.id()
		p.add("Concepts/"+dom+"/Dead-"+id+".md", validConcept(&p.gc, dom, "Dead Path "+id)+
			"\nsee [file](./missing-"+id+".md)\n")
		p.man.note("link.broken.path")

	case focusExternalPath:
		id := p.id()
		p.add("Concepts/"+dom+"/External-"+id+".md", validConcept(&p.gc, dom, "External Path "+id)+
			"\nsee [file](../../../outside-"+id+".md)\n")
		p.man.note("link.broken.path")

	case focusCommentPathRef:
		id := p.id()
		target := "missing-in-comment-" + id + ".md"
		path := "Concepts/" + dom + "/Comment-" + id + ".md"
		p.add(path, validConcept(&p.gc, dom, "Comment Scope "+id)+
			"\n%% sealed [file]("+target+") %%\n\nreal [ok](../"+dom+"/Comment-"+id+".md)\n")
		p.man.commentPathRefs = append(p.man.commentPathRefs, pathTarget{path: path, target: target})
		p.man.note("commentPathRef")

	case focusDiaryChannels:
		p.plantDiaryChannels(dom)

	case focusDiaryConcept:
		p.plantDiaryConcept()

	case focusEmptyDomain:
		id := p.id()
		// A concept declaring an explicit empty domain, folded into "(none)"
		// here and kept as an empty group by the reference in coverage.
		p.add("Concepts/"+dom+"/Empty-"+id+".md", frontmatter(
			"title: Empty Domain "+id,
			"type: concept",
			`domain: ""`,
			"status: "+p.gc.noteStatus[0],
			"source_locator: somewhere",
		)+"\nbody\n")
		p.man.emptyDomainConcepts++
		p.man.note("emptyDomain")

	default:
		panic("difffuzz: unknown focus: " + strconv.Itoa(focus))
	}
}

// plantDiaryChannels reaches the three journal channels that are visible in a
// finding's structured fields — the note that carries it, a collision member,
// and the note a link resolved to — plus the title-owner channel, which is not,
// and so is recorded in the manifest.
func (p *planner) plantDiaryChannels(dom string) {
	p.man.hasDiary = true

	// Citing channel: a journal note with a broken link. The reference reports
	// it against the journal path; this engine drops it.
	c := p.id()
	p.add("Diary/2026-01-"+c+".md", "a journal entry linking [[Missing Journal "+c+"]]\n")

	// Collision channel: a public note and a journal note declaring one alias.
	// The collision finding lists a journal member, so this engine drops it.
	a := p.id()
	alias := "Journal Alias " + a
	p.add("Concepts/"+dom+"/PubAlias-"+a+".md", validConceptFields(&p.gc, dom, "Pub Alias "+a,
		"aliases:\n  - "+alias)+"\nbody\n")
	p.add("Diary/2026-02-"+a+".md", frontmatter(
		"title: Journal Alias Owner "+a,
		"type: transcript",
		"aliases:\n  - "+alias,
	)+"\nbody\n")

	// Resolved-to channel: a journal syllabus and a public lesson it omits. The
	// unlisted finding resolves to the journal syllabus, so this engine drops it.
	r := p.id()
	p.add("Diary/2026-03-"+r+".md", frontmatter(
		"title: Journal Path "+r,
		"type: study-path",
		"domain: "+dom,
		"status: evergreen",
	)+"\nno links\n")
	p.add("Writing/JLesson-"+r+".md", frontmatter(
		"title: Journal-domain Lesson "+r,
		"type: lesson",
		"domain: "+dom,
		"status: "+p.gc.lessonReady(),
		"slug: jlesson-"+strings.ToLower(r),
	)+"\nbody\n")

	// Title-owner channel: a journal note owns a title a public note links; the
	// finding lands on the public note with the link text as its target, so the
	// manifest records the site by (public path, link text).
	t := p.id()
	title := "Journal Title " + t
	p.add("Diary/2026-04-"+t+".md", frontmatter(
		"title: "+title,
		"type: transcript",
	)+"\nbody\n")
	linker := "Concepts/" + dom + "/TitleLinker-" + t + ".md"
	p.add(linker, validConcept(&p.gc, dom, "Title Linker "+t)+"\nlinks [["+title+"]]\n")
	p.man.diaryTitleLinks = append(p.man.diaryTitleLinks, pathTarget{path: linker, target: title})

	p.man.diaryNames = append(p.man.diaryNames, "Journal Alias Owner "+a, title)
	p.man.note("diaryChannels")
}

// plantDiaryConcept places a journal note typed as a concept, with a globally
// unique domain and no inbound edge, so coverage counts it as an orphan on the
// reference and drops it here. The domain is recorded so the count adjustment
// is exact.
func (p *planner) plantDiaryConcept() {
	p.man.hasDiary = true
	id := p.id()
	dc := p.gc.domains[0]
	p.add("Diary/2026-05-"+id+".md", frontmatter(
		"title: Journal Concept "+id,
		"type: concept",
		"domain: "+dc,
	)+"\na journal entry mistyped as a concept\n")
	p.man.diaryConcepts = append(p.man.diaryConcepts, diaryConcept{domain: dc})
	p.man.diaryNames = append(p.man.diaryNames, "Journal Concept "+id)
	p.man.note("diaryConcept")
}

// validConcept renders a fully valid concept: every required field, valid
// enums, and provenance.
func validConcept(gc *genContract, domain, title string) string {
	return frontmatter(
		"title: "+title,
		"type: concept",
		"domain: "+domain,
		"status: "+gc.noteStatus[0],
		"source_locator: somewhere",
	) + "\nbody of " + title + "\n"
}

// validConceptFields renders a valid concept with one extra frontmatter line
// spliced in before the body, for a construct that adds a single fault or field
// on an otherwise clean note.
func validConceptFields(gc *genContract, domain, title, extra string) string {
	return frontmatter(
		"title: "+title,
		"type: concept",
		"domain: "+domain,
		"status: "+gc.noteStatus[0],
		"source_locator: somewhere",
		extra,
	)
}

// frontmatter renders an ordered set of frontmatter lines between fences. A
// line may itself carry newlines (a block sequence), which join preserves.
func frontmatter(lines ...string) string {
	return "---\n" + strings.Join(lines, "\n") + "\n---\n"
}

// writeVault plans a vault for the seed and materializes it under dir, returning
// the manifest. The contract goes to its fixed path and each note to its own,
// creating parent directories as needed.
func writeVault(tb testing.TB, dir string, seed int64) genManifest {
	tb.Helper()
	contract, notes, man := planVault(seed)
	writeFile(tb, dir, filepath.FromSlash(schema.ContractRelPath), contract)
	for _, n := range notes {
		writeFile(tb, dir, filepath.FromSlash(n.Path), n.Body)
	}
	return man
}

func writeFile(tb testing.TB, dir, rel, content string) {
	tb.Helper()
	full := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		tb.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		tb.Fatalf("write %s: %v", rel, err)
	}
}

// TestDiffFuzzGeneratorDeterministic pins the reproduction contract: the same
// seed plans a byte-identical vault, so a failing round is fully recoverable
// from its seed alone.
func TestDiffFuzzGeneratorDeterministic(t *testing.T) {
	t.Parallel()
	for _, seed := range []int64{0, 1, 7, 42, 1000, focusCount + 3} {
		c1, n1, _ := planVault(seed)
		c2, n2, _ := planVault(seed)
		if c1 != c2 {
			t.Errorf("planVault(%d) contract not deterministic:\nfirst:\n%s\nsecond:\n%s", seed, c1, c2)
		}
		if len(n1) != len(n2) {
			t.Fatalf("planVault(%d) note count = %d then %d", seed, len(n1), len(n2))
		}
		for i := range n1 {
			if n1[i] != n2[i] {
				t.Errorf("planVault(%d) note %d differs:\nfirst:  %q => %q\nsecond: %q => %q",
					seed, i, n1[i].Path, n1[i].Body, n2[i].Path, n2[i].Body)
			}
		}
	}
}
