package judge

// The varied contract half of the generator. Each plan draws its own machine
// contract — enum sets, field lists, status groups, slug pattern — from the
// seeded generator, so the schema checks run against a moving target rather
// than one fixed shape. The data lists vary freely: both engines read the same
// contract file, so any disagreement they reach on a varied contract is a real
// divergence, not a quirk of one shape. The structural policy flags stay at
// their live values, keeping the fuzzing on the enum and field data where the
// checks actually branch.

import (
	"math/rand/v2"
	"strings"
)

// genContract is one plan's chosen contract, kept as parsed lists so a plant
// can pick a value the contract admits or one it does not.
type genContract struct {
	types         []string
	domains       []string
	sourceKind    []string
	sourceProv    []string
	levels        []string
	mapKinds      []string
	noteStatus    []string
	systemStatus  []string
	lessonStatus  []string
	required      []string
	requiredInbox []string
	domainExempt  []string
	known         []string
	lessonOnly    []string
	groupSystem   []string
	groupLesson   []string
	slugPattern   string
}

// buildContract selects one plan's contract. The value lists are sampled and
// shuffled for variety, with the load-bearing members always kept so a valid
// note stays valid: the primary domain the notes are filed under, the statuses
// the plants use, and the fields a valid note writes.
func buildContract(rng *rand.Rand) genContract {
	lessonStatus := shuffled(rng, []string{"imported", "draft", "ready", "archived"})
	// The note status set is a superset of the lesson set, so a lesson's status
	// is accepted by both the lesson rule and the general note rule; "evergreen"
	// is always present because the map and syllabus notes use it.
	noteStatus := shuffled(rng, append([]string{"seedling", "growing", "evergreen", "captured", "cleaned"}, lessonStatus...))

	gc := genContract{
		types: shuffled(rng, []string{
			"inbox", "transcript", "source-note", "concept", "moc",
			"source-map", "topic-map", "study-path", "synthesis", "research-brief",
			"writing", "lesson", "system", "template", "guide",
		}),
		domains:       sample(rng, []string{"docker", "k8s", "database", "networking", "algorithms", "meta"}, []string{"golang", "rust", "japanese"}),
		sourceKind:    sample(rng, []string{"book", "personal", "official-doc"}, []string{"course"}),
		sourceProv:    sample(rng, []string{"oreilly", "notion", "hermes"}, []string{"personal"}),
		levels:        sample(rng, []string{"intermediate", "advanced"}, []string{"fundamental"}),
		mapKinds:      shuffled(rng, []string{"source", "topic", "path"}),
		noteStatus:    noteStatus,
		systemStatus:  []string{"active", "archived"},
		lessonStatus:  lessonStatus,
		required:      []string{"title", "type", "domain", "status"},
		requiredInbox: []string{"title", "type", "status"},
		domainExempt:  []string{"system", "guide", "template", "research-brief"},
		known:         sample(rng, []string{"topics", "level", "map_kind", "source_kind", "source_provider", "created", "updated", "kind", "area"}, []string{"title", "aliases", "type", "domain", "status", "source_locator", "based_on", "related", "tags"}),
		lessonOnly:    []string{"slug", "title_en"},
		groupSystem:   []string{"system", "template", "guide"},
		groupLesson:   []string{"lesson"},
		slugPattern:   pick(rng, []string{`^[a-z0-9]+(-[a-z0-9]+)*$`, `^[a-z][a-z0-9-]*$`}),
	}
	return gc
}

// absentToken returns a token the contract's set for kind does not contain, so
// a plant can reach an enum, folder, or unknown-key fault with certainty. The
// tokens are outside every value pool the generator draws from.
func (gc *genContract) absentToken(kind string) string {
	switch kind {
	case "type":
		return "zzztypeabsent"
	case "status":
		return "zzzstatusabsent"
	case "domain":
		return "zzzdomainabsent"
	case "field":
		return "zzzfieldabsent"
	default:
		panic("difffuzz: unknown absent kind: " + kind)
	}
}

// otherDomain returns a valid domain other than dom, for the folder-mismatch
// plant where both the folder and the declared domain must be legal.
func (gc *genContract) otherDomain(dom string) string {
	for _, d := range gc.domains {
		if d != dom {
			return d
		}
	}
	panic("difffuzz: contract needs at least two domains")
}

// lessonReady returns a lesson status other than draft, so a lesson used in the
// map-vs-disk plant is not skipped as expected work-in-progress.
func (gc *genContract) lessonReady() string {
	for _, s := range gc.lessonStatus {
		if s != "draft" {
			return s
		}
	}
	panic("difffuzz: lesson status set has only draft")
}

// contractTOML renders the contract to the exact file both engines load. The
// lifecycle block is fixed and minimal: it satisfies the loader's demand for at
// least one lifecycle entry but never gates check, coverage, or exists, which
// read no transition.
func contractTOML(gc *genContract) string {
	var b strings.Builder
	b.WriteString("schema_version = \"1\"\n")
	b.WriteString("[enums]\n")
	line := func(key string, vals []string) {
		b.WriteString(key + " = " + tomlArray(vals) + "\n")
	}
	line("type", gc.types)
	line("domain", gc.domains)
	line("source_kind", gc.sourceKind)
	line("source_provider", gc.sourceProv)
	line("level", gc.levels)
	line("map_kind", gc.mapKinds)
	b.WriteString("[enums.status]\n")
	line("note", gc.noteStatus)
	line("system", gc.systemStatus)
	line("lesson", gc.lessonStatus)
	b.WriteString("[fields]\n")
	line("required", gc.required)
	line("required_inbox", gc.requiredInbox)
	line("domain_exempt_types", gc.domainExempt)
	line("known", gc.known)
	line("lesson_only", gc.lessonOnly)
	b.WriteString("[fields.status_group]\n")
	line("system", gc.groupSystem)
	line("lesson", gc.groupLesson)
	b.WriteString("[rules]\n")
	b.WriteString(`domain_equals_folder_under = ["Concepts"]` + "\n")
	b.WriteString(`concept_requires_provenance = ["based_on", "source_locator"]` + "\n")
	b.WriteString("slug_pattern = " + tomlString(gc.slugPattern) + "\n")
	b.WriteString("forbid_tag_with_slash = true\n")
	b.WriteString("[scan]\n")
	b.WriteString(`knowledge_dirs = ["Concepts", "Sources", "Maps", "Writing", "Synthesis", "Inbox"]` + "\n")
	b.WriteString(`skip_basenames = ["README.md"]` + "\n")
	b.WriteString("no_frontmatter_is_legal = true\n")
	b.WriteString("[[lifecycle]]\n")
	b.WriteString(`status = "seedling"` + "\n")
	b.WriteString(`applies_to = ["concept"]` + "\n")
	b.WriteString("from = []\n")
	b.WriteString(`owner = ["claude"]` + "\n")
	b.WriteString("[[lifecycle]]\n")
	b.WriteString(`status = "archived"` + "\n")
	b.WriteString(`applies_to = ["*"]` + "\n")
	b.WriteString(`from = ["*"]` + "\n")
	b.WriteString(`owner = ["claude", "koopa"]` + "\n")
	b.WriteString("[[lifecycle]]\n")
	b.WriteString(`status = "active"` + "\n")
	b.WriteString(`applies_to = ["system", "template", "guide"]` + "\n")
	b.WriteString("from = []\n")
	b.WriteString(`owner = ["claude"]` + "\n")
	return b.String()
}

// tomlArray renders a string slice as a TOML inline array.
func tomlArray(vals []string) string {
	parts := make([]string, len(vals))
	for i, v := range vals {
		parts[i] = tomlString(v)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// tomlString renders one TOML basic string, escaping the two characters a basic
// string may not carry raw.
func tomlString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

// shuffled returns a deterministic permutation of items drawn from the seeded
// generator, leaving the input untouched.
func shuffled(rng *rand.Rand, items []string) []string {
	out := append([]string(nil), items...)
	rng.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}

// sample returns the kept members followed by a deterministic subset of the
// pool, so the load-bearing values are always present and the rest vary.
func sample(rng *rand.Rand, pool, keep []string) []string {
	out := append([]string(nil), keep...)
	for _, v := range pool {
		if rng.IntN(2) == 0 {
			out = append(out, v)
		}
	}
	return out
}

// pick returns one deterministic choice from options.
func pick(rng *rand.Rand, options []string) string {
	return options[rng.IntN(len(options))]
}
