package judge

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/sequence"
)

func TestSupersessionRulesUseConfiguredVocabulary(t *testing.T) {
	t.Parallel()

	findings, err := Check(t.Context(), "testdata/vault-supersession")
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	var gotRules []string
	var gotProvenanceFields []string
	for i := range findings {
		finding := &findings[i]
		if finding.RuleID == "supersession.predecessor_not_archived" ||
			finding.RuleID == "supersession.archived_navigation_target" {
			gotRules = append(gotRules, string(finding.RuleID)+"@"+finding.Path)
		}
		if finding.RuleID == "provenance.unresolved" && finding.Field != nil {
			gotProvenanceFields = append(gotProvenanceFields, *finding.Field)
			if (*finding.Field == "lineage" || *finding.Field == "successors") &&
				finding.SourceRule != "vault-schema.toml#supersession" {
				t.Errorf("configured provenance field %q SourceRule = %q, want vault-schema.toml#supersession",
					*finding.Field, finding.SourceRule)
			}
		}
		if finding.Path == "System/templates/Template.md" ||
			(finding.ResolvedTo != nil && *finding.ResolvedTo == "System/templates/Archived Template.md") {
			t.Errorf("supersession finding includes non-instance artifact: %+v", *finding)
		}
	}

	wantRules := []string{
		"supersession.archived_navigation_target@Maps/Map.md",
		"supersession.archived_navigation_target@Maps/Path.md",
		"supersession.predecessor_not_archived@Writing/Old Lesson.md",
	}
	if !slices.Equal(gotRules, wantRules) {
		t.Errorf("supersession rules = %v, want %v", gotRules, wantRules)
	}
	wantFields := []string{"lineage", "predecessor", "successors"}
	slices.Sort(gotProvenanceFields)
	if !slices.Equal(gotProvenanceFields, wantFields) {
		t.Errorf("configured provenance fields = %v, want %v", gotProvenanceFields, wantFields)
	}
}

func TestSupersessionCapabilityCrossProduct(t *testing.T) {
	t.Parallel()

	notes := []note{
		{
			path:     "Writing/Old.md",
			noteType: "lesson",
			status:   "ready",
			frontmatter: map[string]fmValue{
				"successors": {list: []string{"new"}, stringList: []string{"new"}, isList: true},
			},
		},
		{
			path:     "Writing/Numeric.md",
			noteType: "lesson",
			status:   "ready",
			frontmatter: map[string]fmValue{
				"successors": {list: []string{"7"}, isList: true},
			},
		},
		{
			path:     "Writing/Invalid.md",
			noteType: "lesson",
			status:   "bogus",
			frontmatter: map[string]fmValue{
				"successors": {scalar: "new", scalarIsString: true},
			},
		},
		courseNote("Maps/Path.md", "## 主線 {sequence=primary}\n\n- [[Archived]]\n"),
		{
			path:     "Writing/Archived.md",
			noteType: "lesson",
			status:   "archived",
		},
	}
	idx := buildIndex(notes, nil)

	tests := []struct {
		name         string
		replacements [][2]string
		wantRules    []string
	}{
		{
			name:      "all capabilities",
			wantRules: []string{predecessorNotArchivedRule, archivedNavigationRule},
		},
		{
			name: "supersession absent",
			replacements: [][2]string{
				{supersessionFixtureSection, ""},
			},
		},
		{
			name: "navigation unavailable",
			replacements: [][2]string{
				{navigationFixtureSection, ""},
			},
			wantRules: []string{predecessorNotArchivedRule},
		},
		{
			name: "navigation incomplete",
			replacements: [][2]string{
				{navigationFixtureSection, "[navigation]\npath_types = [\"study-path\"]\n\n"},
			},
			wantRules: []string{predecessorNotArchivedRule},
		},
		{
			name: "navigation invalid",
			replacements: [][2]string{
				{navigationFixtureSection, "[navigation]\npath_types = [\"unknown\"]\nmap_types = [\"topic-map\"]\n\n"},
			},
			wantRules: []string{predecessorNotArchivedRule},
		},
		{
			name: "artifact policy missing",
			replacements: [][2]string{
				{artifactFixtureSection, ""},
			},
		},
		{
			name: "artifact policy incomplete",
			replacements: [][2]string{
				{artifactFixtureSection, "[artifacts]\n"},
			},
		},
		{
			name: "artifact policy invalid",
			replacements: [][2]string{
				{artifactFixtureSection, "[artifacts]\nnon_instance_dirs = [\".\"]\n\n"},
			},
		},
		{
			name: "privacy policy missing",
			replacements: [][2]string{
				{privacyFixtureSection, ""},
			},
		},
		{
			name: "privacy policy incomplete",
			replacements: [][2]string{
				{privacyFixtureSection, "[privacy]\n\n"},
			},
		},
		{
			name: "privacy policy invalid",
			replacements: [][2]string{
				{privacyFixtureSection, "[privacy]\nnever_egress_dirs = [\".\"]\n\n"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			contract := loadSupersessionContract(t, tt.replacements)
			authority := scanAuthority{contract: contract, privacy: contract.PrivacyPolicy()}
			findings := supersessionFindings(notes, idx, authority)
			gotRules := make([]string, len(findings))
			for i := range findings {
				gotRules[i] = string(findings[i].RuleID)
			}
			if !slices.Equal(gotRules, tt.wantRules) {
				t.Errorf("rules = %v, want %v", gotRules, tt.wantRules)
			}
		})
	}
}

func TestArchivedNavigationTargetResolutionDomain(t *testing.T) {
	t.Parallel()

	contract := loadSupersessionContract(t, nil)
	authority := scanAuthority{contract: contract, privacy: contract.PrivacyPolicy()}
	tests := []struct {
		name       string
		targetName string
		targets    []note
		want       int
	}{
		{
			name:       "unique governable archived target",
			targetName: "Archived",
			targets:    []note{{path: "Writing/Archived.md", noteType: "lesson", status: "archived"}},
			want:       1,
		},
		{
			name:       "unresolved target",
			targetName: "Missing",
		},
		{
			name:       "ambiguous target",
			targetName: "Archived",
			targets: []note{
				{path: "Writing/First.md", noteType: "lesson", status: "archived", aliases: []string{"Archived"}},
				{path: "Writing/Second.md", noteType: "lesson", status: "archived", aliases: []string{"Archived"}},
			},
		},
		{
			name:       "unknown target type",
			targetName: "Archived",
			targets:    []note{{path: "Writing/Archived.md", noteType: "unknown", status: "archived"}},
		},
		{
			name:       "invalid target status",
			targetName: "Archived",
			targets:    []note{{path: "Writing/Archived.md", noteType: "lesson", status: "bogus"}},
		},
		{
			name:       "non-instance target",
			targetName: "Archived Template",
			targets:    []note{{path: "System/templates/Archived Template.md", noteType: "lesson", status: "archived"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			notes := append([]note{
				courseNote("Maps/Path.md", "## 主線 {sequence=primary}\n\n- [["+tt.targetName+"]]\n"),
			}, tt.targets...)
			idx := buildIndex(notes, nil)
			findings := supersessionFindings(notes, idx, authority)
			count := 0
			for i := range findings {
				if findings[i].RuleID == archivedNavigationRule {
					count++
				}
			}
			if count != tt.want {
				t.Errorf("archived navigation findings = %d, want %d", count, tt.want)
			}
		})
	}
}

func loadSupersessionContract(t *testing.T, replacements [][2]string) *schema.Contract {
	t.Helper()

	data, err := os.ReadFile("testdata/vault-supersession/System/schemas/vault-schema.toml")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	text := string(data)
	for _, replacement := range replacements {
		if count := strings.Count(text, replacement[0]); count != 1 {
			t.Fatalf("contract mutation match count = %d, want 1 for %q", count, replacement[0])
		}
		text = strings.Replace(text, replacement[0], replacement[1], 1)
	}
	path := filepath.Join(t.TempDir(), "vault-schema.toml")
	if err = os.WriteFile(path, []byte(text), 0o600); err != nil { // #nosec G703 -- path is rooted in t.TempDir
		t.Fatalf("WriteFile() error = %v", err)
	}
	contract, err := schema.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}
	return contract
}

const navigationFixtureSection = `[navigation]
path_types = ["study-path"]
map_types = ["topic-map"]

`

const artifactFixtureSection = `[artifacts]
non_instance_dirs = ["System/templates"]

`

const privacyFixtureSection = `[privacy]
never_egress_dirs = []

`

const supersessionFixtureSection = `[supersession]
predecessor_field = "predecessor"
successor_field = "successors"
general_link_field = "lineage"
archived_status = "archived"

`

// courseNote is a study path built the way the scanner builds one: its links
// and its declared structure both come from the same body, so a test cannot
// assert a course lists something the grammar never read.
func courseNote(path, body string) note {
	return note{
		path:      path,
		noteType:  "study-path",
		status:    "ready",
		wikilinks: extractWikilinks(body, 1),
		sequence:  sequence.Parse(body, 1),
	}
}

// TestSupersessionFoldsTheNoteStatusSpelling holds the replacement-ledger
// verdicts to one answer per status word. A contract's values and a note's own
// value reach this code from different places — a filesystem hands over a
// decomposed spelling, an editor and the search index a composed one — so one
// word can arrive as two byte strings. A contract folds its own values as it
// reads them, and is refused at load if it archives with a spelling those
// values do not carry, so the note's value is the only side left to fold;
// while it was not folded, a superseded lesson and an archived link target
// both went unreported for the spelling their own vault happened to use.
//
// Both rules are reached by their own rows, and each comparison the rules make
// against a status has one: the status set a note's type declares, and the
// archived word itself, on the note being judged and on the note it links to.
func TestSupersessionFoldsTheNoteStatusSpelling(t *testing.T) {
	t.Parallel()

	// One word in two spellings, twice: a live status and the archived one.
	// Both are written from code points, so nothing between the keyboard and
	// the compiler can fold one form into the other. The decomposed form
	// carries the voiced-sound mark as its own code point.
	const (
		liveComposed       = "\u6e96\u5099\u305a\u307f"
		liveDecomposed     = "\u6e96\u5099\u3059\u3099\u307f"
		archivedComposed   = "\u4fdd\u7ba1\u305a\u307f"
		archivedDecomposed = "\u4fdd\u7ba1\u3059\u3099\u307f"
	)
	for _, pair := range [][2]string{{liveComposed, liveDecomposed}, {archivedComposed, archivedDecomposed}} {
		if pair[0] == pair[1] || schema.NormalizeStatus(pair[1]) != pair[0] {
			t.Fatalf("the fixture spellings are not one word in two forms: %q and %q", pair[0], pair[1])
		}
	}

	contract := loadSupersessionContract(t, statusSpellingReplacements(liveComposed, archivedComposed))
	vocabulary, ok := contract.Supersession()
	if !ok {
		t.Fatal("the fixture contract declares no replacement ledger")
	}
	if vocabulary.ArchivedStatus != archivedComposed {
		t.Fatalf("archived status = %q, want %q", vocabulary.ArchivedStatus, archivedComposed)
	}
	// A lesson and a study path read against different status groups, and a
	// row expecting no finding would pass for the wrong reason if either word
	// were missing from either of them.
	for _, noteType := range []string{"lesson", "study-path"} {
		for _, word := range []string{liveComposed, archivedComposed} {
			if declared := contract.Statuses(noteType); !slices.Contains(declared, word) {
				t.Fatalf("Statuses(%q) = %q, want it to declare %q", noteType, declared, word)
			}
		}
	}
	authority := scanAuthority{contract: contract, privacy: contract.PrivacyPolicy()}

	superseded := func(status string) []note {
		return []note{{
			path:     "Writing/Old.md",
			noteType: "lesson",
			status:   status,
			frontmatter: map[string]fmValue{
				"successors": {list: []string{"New"}, stringList: []string{"New"}, isList: true},
			},
		}}
	}
	navigation := func(sourceStatus, targetStatus string) []note {
		source := courseNote("Maps/Path.md", "## 主線 {sequence=primary}\n\n- [[Target]]\n")
		source.status = sourceStatus
		return []note{source, {path: "Writing/Target.md", noteType: "lesson", status: targetStatus}}
	}

	tests := []struct {
		name      string
		notes     []note
		wantRules []string
	}{
		{
			name: "predecessor/decomposed live status", notes: superseded(liveDecomposed),
			wantRules: []string{predecessorNotArchivedRule},
		},
		{
			name: "predecessor/composed live status", notes: superseded(liveComposed),
			wantRules: []string{predecessorNotArchivedRule},
		},
		{name: "predecessor/composed archived status", notes: superseded(archivedComposed)},
		{name: "predecessor/decomposed archived status", notes: superseded(archivedDecomposed)},
		{
			name: "navigation/decomposed live source", notes: navigation(liveDecomposed, archivedComposed),
			wantRules: []string{archivedNavigationRule},
		},
		{
			name: "navigation/decomposed archived target", notes: navigation(liveComposed, archivedDecomposed),
			wantRules: []string{archivedNavigationRule},
		},
		{
			name: "navigation/composed throughout", notes: navigation(liveComposed, archivedComposed),
			wantRules: []string{archivedNavigationRule},
		},
		{name: "navigation/live target", notes: navigation(liveComposed, liveComposed)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			findings := supersessionFindings(tt.notes, buildIndex(tt.notes, nil), authority)
			var gotRules []string
			for i := range findings {
				gotRules = append(gotRules, string(findings[i].RuleID))
			}
			if !slices.Equal(gotRules, tt.wantRules) {
				t.Errorf("rules = %v, want %v", gotRules, tt.wantRules)
			}
		})
	}
}

// statusSpellingReplacements rewrites the fixture contract to speak a status
// vocabulary that has two spellings: both words join every status group, the
// replacement ledger archives with the second of them, and each gets the
// lifecycle row the ledger's own validation requires of an archived status.
func statusSpellingReplacements(live, archived string) [][2]string {
	const archivedLifecycleRow = `[[lifecycle]]
status = "archived"
applies_to = ["*"]
from = ["*"]
owner = ["koopa"]
`
	return [][2]string{
		{
			`[enums.status]
note = ["draft", "ready", "archived"]
lesson = ["draft", "ready", "archived"]
`,
			`[enums.status]
note = ["draft", "ready", "archived", "` + live + `", "` + archived + `"]
lesson = ["draft", "ready", "archived", "` + live + `", "` + archived + `"]
`,
		},
		{`archived_status = "archived"`, `archived_status = "` + archived + `"`},
		{
			archivedLifecycleRow,
			archivedLifecycleRow + `
[[lifecycle]]
status = "` + live + `"
applies_to = ["*"]
from = []
owner = ["koopa"]

[[lifecycle]]
status = "` + archived + `"
applies_to = ["*"]
from = ["*"]
owner = ["koopa"]
`,
		},
	}
}
