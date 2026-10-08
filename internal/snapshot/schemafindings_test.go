package snapshot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/schema"
)

// TestAGenerationCarriesTheSchemaVerdictForEachNote holds the generation to the same
// verdict the check command reaches. The oracle is the judge seam over the
// same bytes rather than a list written here, so the two cannot drift apart
// while both stay green.
func TestAGenerationCarriesTheSchemaVerdictForEachNote(t *testing.T) {
	t.Parallel()

	const faulty = "Concepts/golang/Bad.md"
	const clean = "Concepts/golang/Fine.md"
	faultyBody := "---\ntitle: Bad\ntype: concept\ndomain: golang\nstatus: bogus\ncreated: 2026-06-01\nupdated: 2026-06-01\nextra: 1\n---\n\nbody\n"
	cleanBody := "---\ntitle: Fine\ntype: concept\ndomain: golang\nstatus: seedling\ncreated: 2026-06-01\nupdated: 2026-06-01\nbased_on: \"[[Something]]\"\n---\n\nbody\n"

	root := t.TempDir()
	contract := testContract(t, root)
	writeNote(t, root, faulty, faultyBody)
	writeNote(t, root, clean, cleanBody)
	store, _ := newTestStore(t, root, contract)
	gen := store.Current()

	want, err := judge.LintFrontmatter(faulty, []byte(faultyBody), contract)
	if err != nil {
		t.Fatalf("judge.LintFrontmatter() error = %v", err)
	}
	if len(want) == 0 {
		t.Fatal("the fixture note draws no schema findings, so this test would pass on any implementation")
	}
	var ruleIDs []string
	for _, f := range want {
		ruleIDs = append(ruleIDs, string(f.RuleID))
	}
	t.Logf("the fixture draws: %s", strings.Join(ruleIDs, ", "))

	if diff := cmp.Diff(judge.FrontmatterResult{Findings: want, EnumNoteType: "concept"}, gen.SchemaResult(faulty)); diff != "" {
		t.Errorf("Generation.SchemaResult(%q) differs from captured verdict (-want +got):\n%s", faulty, diff)
	}
	if diff := cmp.Diff(want, gen.SchemaFindings(faulty)); diff != "" {
		t.Errorf("Generation.SchemaFindings(%q) differs from the seam (-seam +generation):\n%s", faulty, diff)
	}
	if got := gen.SchemaFindings(clean); len(got) != 0 {
		t.Errorf("Generation.SchemaFindings(%q) = %v, want nothing for a note that satisfies the schema", clean, got)
	}
	for _, rel := range []string{clean, "Concepts/golang/Absent.md"} {
		if diff := cmp.Diff(judge.FrontmatterResult{}, gen.SchemaResult(rel)); diff != "" {
			t.Errorf("Generation.SchemaResult(%q) (-zero +got):\n%s", rel, diff)
		}
	}
	if got := gen.SchemaFindings("Concepts/golang/Absent.md"); len(got) != 0 {
		t.Errorf("Generation.SchemaFindings(absent) = %v, want nothing", got)
	}
}

// TestSchemaFindingsAreTheCallersOwnSlice holds the accessor to the promise
// every other collection accessor here makes. Handing out the generation's own
// backing array would let one reader's append reach another reader's page.
func TestSchemaFindingsAreTheCallersOwnSlice(t *testing.T) {
	t.Parallel()

	const faulty = "Concepts/golang/Bad.md"
	body := "---\ntitle: Bad\ntype: concept\ndomain: golang\nstatus: bogus\ncreated: 2026-06-01\nupdated: 2026-06-01\n---\n\nbody\n"

	root := t.TempDir()
	contract := testContract(t, root)
	writeNote(t, root, faulty, body)
	store, _ := newTestStore(t, root, contract)
	gen := store.Current()

	first := gen.SchemaResult(faulty)
	if len(first.Findings) == 0 {
		t.Fatal("the fixture note draws no schema findings, so this test would prove nothing")
	}
	first.Findings[0] = judge.Finding{RuleID: "scribbled.over"}
	first.EnumNoteType = "scribbled.over"
	wantFindings, err := judge.LintFrontmatter(faulty, []byte(body), contract)
	if err != nil {
		t.Fatal(err)
	}
	want := judge.FrontmatterResult{Findings: wantFindings, EnumNoteType: "concept"}
	if diff := cmp.Diff(want, gen.SchemaResult(faulty)); diff != "" {
		t.Errorf("caught: modifying caller result changed captured verdict (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(wantFindings, gen.SchemaFindings(faulty)); diff != "" {
		t.Errorf("caught: modifying caller result changed compatibility findings (-want +got):\n%s", diff)
	}
}

func TestGenerationDomainFolder(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	contract := domainRootContract(t, root, `["Writing/lessons"]`)
	store, _ := newTestStore(t, root, contract)
	gen := store.Current().Capture()
	ungoverned, _ := newTestStore(t, t.TempDir(), nil)
	emptyRoot := t.TempDir()
	emptyContract := domainRootContract(t, emptyRoot, `[]`)
	empty, _ := newTestStore(t, emptyRoot, emptyContract)

	for _, tt := range []struct {
		name  string
		gen   *Generation
		path  string
		want  string
		found bool
	}{
		{name: "nil generation", path: "Writing/lessons/japanese/L1.md"},
		{name: "zero generation", gen: &Generation{}, path: "Writing/lessons/japanese/L1.md"},
		{name: "absent contract", gen: ungoverned.Current().Capture(), path: "Writing/lessons/japanese/L1.md"},
		{name: "empty roots", gen: empty.Current().Capture(), path: "Writing/lessons/japanese/L1.md"},
		{name: "declared root", gen: gen, path: "Writing/lessons/japanese/L1.md", want: "japanese", found: true},
		{name: "deeper note", gen: gen, path: "Writing/lessons/japanese/drills/L1.md", want: "japanese", found: true},
		{name: "direct child", gen: gen, path: "Writing/lessons/Overview.md"},
		{name: "unmatched", gen: gen, path: "Concepts/japanese/L1.md"},
		{name: "sibling prefix", gen: gen, path: "Writing/lessons-extra/japanese/L1.md"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, found := tt.gen.DomainFolder(tt.path)
			if got != tt.want || found != tt.found {
				t.Errorf("DomainFolder(%q) = (%q, %t), want (%q, %t)", tt.path, got, found, tt.want, tt.found)
			}
		})
	}
}

// TestCapturedDomainRoots keeps one path's interpretation tied to its captured
// declaration even after a caller edits a copy and the contract file is replaced.
func TestCapturedDomainRoots(t *testing.T) {
	t.Parallel()
	const rel = "Writing/lessons/japanese/nested/Deep.md"
	const body = "---\ntitle: Deep\ntype: concept\ndomain: golang\nstatus: draft\ncreated: 2026-06-01\nupdated: 2026-06-01\nbased_on: \"[[x]]\"\n---\n\nbody\n"
	root := t.TempDir()
	firstContract := domainRootContract(t, root, `["Writing/lessons"]`)
	writeNote(t, root, rel, body)
	firstStore, _ := newTestStore(t, root, firstContract)
	first := firstStore.Current().Capture()

	copyBeforeBuild := firstContract.Definition()
	copyBeforeBuild.Rules.DomainEqualsFolderUnder[0] = "Writing"
	fromOriginalContract, _ := newTestStore(t, root, firstContract)
	copyAfterBuild := firstContract.Definition()
	copyAfterBuild.Rules.DomainEqualsFolderUnder[0] = "Writing"

	secondContract := domainRootContract(t, root, `["Writing"]`)
	secondStore, _ := newTestStore(t, root, secondContract)
	second := secondStore.Current().Capture()

	for _, tt := range []struct {
		name    string
		gen     *Generation
		folder  string
		message string
	}{
		{name: "captured first after replacement", gen: first, folder: "japanese", message: `domain "golang" does not match its folder japanese`},
		{name: "original contract after caller edits", gen: fromOriginalContract.Current().Capture(), folder: "japanese", message: `domain "golang" does not match its folder japanese`},
		{name: "captured replacement", gen: second, folder: "lessons", message: `domain "golang" does not match its folder lessons`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			folder, found := tt.gen.DomainFolder(rel)
			if folder != tt.folder || !found {
				t.Errorf("captured DomainFolder(%q) = (%q, %t), want (%q, true)", rel, folder, found, tt.folder)
			}
			var got []judge.Finding
			for _, finding := range tt.gen.SchemaFindings(rel) {
				if finding.RuleID == "schema.domain_folder" {
					got = append(got, finding)
				}
			}
			if len(got) != 1 {
				t.Fatalf("captured domain findings = %v, want exactly one", got)
			}
			want := judge.Finding{
				RuleID: "schema.domain_folder", Path: rel, Severity: judge.SeverityError,
				Field: new("domain"), Target: new("golang"), Message: tt.message,
				Evidence: "frontmatter validated against vault-schema.toml", SuggestedAction: "fix the frontmatter to match the schema",
				SourceRule: "vault-schema.toml#rules", Fingerprint: "v1:1b6dee4516d7c953",
			}
			if diff := cmp.Diff(want, got[0]); diff != "" {
				t.Errorf("captured domain finding mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func domainRootContract(t *testing.T, root, roots string) *schema.Contract {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("read contract fixture: %v", err)
	}
	const declaration = `domain_equals_folder_under = ["Concepts"]`
	if strings.Count(string(data), declaration) != 1 {
		t.Fatal("fixture must have exactly one domain-root declaration")
	}
	data = []byte(strings.Replace(string(data), declaration, "domain_equals_folder_under = "+roots, 1))
	tree, openErr := os.OpenRoot(root)
	if openErr != nil {
		t.Fatalf("open fixture root: %v", openErr)
	}
	t.Cleanup(func() {
		if closeErr := tree.Close(); closeErr != nil {
			t.Errorf("close fixture root: %v", closeErr)
		}
	})
	contractPath := filepath.FromSlash(schema.ContractRelPath)
	if mkdirErr := tree.MkdirAll(filepath.Dir(contractPath), 0o750); mkdirErr != nil {
		t.Fatalf("mkdir contract: %v", mkdirErr)
	}
	if writeErr := tree.WriteFile(contractPath, data, 0o600); writeErr != nil {
		t.Fatalf("write contract: %v", writeErr)
	}
	contract, err := schema.Load(root)
	if err != nil {
		t.Fatalf("load contract: %v", err)
	}
	return contract
}

// TestSchemaResultKeepsItsCapturedSelector binds advice to the verdict's read,
// even after another generation reads a different scalar spelling from disk.
func TestSchemaResultKeepsItsCapturedSelector(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	testContract(t, root)
	contractPath := filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))
	data, err := os.ReadFile(contractPath)
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, pair := range [][2]string{
		{`"study-path", "topic-map"]`, `"study-path", "topic-map", "1", "true"]`},
		{`system = ["system", "template", "guide"]`, `system = ["system", "template", "guide", "1", "true"]`},
	} {
		if strings.Count(source, pair[0]) != 1 {
			t.Fatalf("fixture selector needle %q is not unique", pair[0])
		}
		source = strings.Replace(source, pair[0], pair[1], 1)
	}
	writeNote(t, root, schema.ContractRelPath, source)
	contract, err := schema.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	lint, err := judge.NewFrontmatterLinter(contract)
	if err != nil {
		t.Fatal(err)
	}
	const rel = "Writing/Scalar.md"
	firstBody := "---\ntitle: Scalar\ntype: 1\nstatus: draft\n---\n"
	writeNote(t, root, rel, firstBody)
	firstStore, _ := newTestStore(t, root, contract)
	first := firstStore.Current().Capture()
	secondBody := strings.Replace(firstBody, "type: 1", "type: true", 1)
	writeNote(t, root, rel, secondBody)
	secondStore, _ := newTestStore(t, root, contract)
	second := secondStore.Current().Capture()
	for _, tt := range []struct {
		name     string
		gen      *Generation
		body     string
		selector string
	}{
		{name: "retained first", gen: first, body: firstBody, selector: "1"},
		{name: "replacement", gen: second, body: secondBody, selector: "true"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			want := lint.LintResult(rel, []byte(tt.body))
			if want.EnumNoteType != tt.selector || len(want.Findings) != 1 {
				t.Fatalf("fixture verdict = %+v, want selector %q and one finding", want, tt.selector)
			}
			if diff := cmp.Diff(want, tt.gen.SchemaResult(rel)); diff != "" {
				t.Errorf("caught: captured SchemaResult (%s) (-want +got):\n%s", tt.name, diff)
			}
			reading, found := tt.gen.Note(rel)
			if !found {
				t.Fatal("fixture reading absent")
			}
			if reading.Type != "" {
				t.Errorf("caught: scalar reading Type = %q, want empty", reading.Type)
			}
		})
	}
	for _, gen := range []*Generation{nil, {}, first, second} {
		if diff := cmp.Diff(judge.FrontmatterResult{}, gen.SchemaResult("Writing/Absent.md")); diff != "" {
			t.Errorf("caught: absent SchemaResult (-zero +got):\n%s", diff)
		}
	}
}
