package judge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

func judgeFixtureRoot(tb testing.TB, root string) string {
	tb.Helper()
	return judgeFixtureRootWithPrivacy(tb, root)
}

func judgeFixtureRootWithPrivacy(tb testing.TB, root string, privateDirs ...string) string {
	tb.Helper()
	if len(privateDirs) == 0 {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))); err == nil {
			return root
		}
	}
	copyRoot := tb.TempDir()
	if err := os.CopyFS(copyRoot, os.DirFS(root)); err != nil {
		tb.Fatalf("CopyFS(%q) error = %v", root, err)
	}
	if _, err := os.Stat(filepath.Join(copyRoot, filepath.FromSlash(schema.ContractRelPath))); err == nil {
		tb.Fatalf("fixture %q already has a contract; custom privacy replacement is ambiguous", root)
	}
	writeTestContract(tb, copyRoot, privateDirs)
	return copyRoot
}

func testScanAuthority(tb testing.TB, privateDirs ...string) scanAuthority {
	tb.Helper()
	root := tb.TempDir()
	writeTestContract(tb, root, privateDirs)
	return loadTestAuthority(tb, root)
}

func loadTestAuthority(tb testing.TB, root string) scanAuthority {
	tb.Helper()
	reader, err := vault.Open(root)
	if err != nil {
		tb.Fatalf("vault.Open() error = %v", err)
	}
	tb.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			tb.Errorf("Reader.Close() error = %v", closeErr)
		}
	})
	authority, err := loadScanAuthority(tb.Context(), reader)
	if err != nil {
		tb.Fatalf("loadScanAuthority() error = %v", err)
	}
	return authority
}

func collectNotes(ctx context.Context, root string) ([]note, error) {
	a, err := openAction(ctx, root, actionHooks{})
	if err != nil {
		return nil, err
	}
	if err := a.finish(); err != nil {
		return nil, err
	}
	return a.notes, nil
}

func writeTestContract(tb testing.TB, root string, privateDirs []string) {
	tb.Helper()
	dirs := fixtureKnowledgeDirs(tb, root)
	quoted := make([]string, len(dirs))
	for i, dir := range dirs {
		quoted[i] = strconv.Quote(dir)
	}
	write(tb, root, schema.ContractRelPath, contractFixture(tb, privateDirs,
		[2]string{
			`checked = ["Concepts", "Sources", "Maps", "Writing", "Synthesis", "Inbox"]`,
			"checked = [" + strings.Join(quoted, ", ") + "]",
		}))
}

// fixtureKnowledgeDirs lists the top-level directories the fixture already
// holds. That set is the knowledge layer the contract should declare: naming
// something else is a lint-off trick, and naming nothing when the fixture has
// folders is the empty-list polarity, not a description of the vault.
func fixtureKnowledgeDirs(tb testing.TB, root string) []string {
	tb.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		tb.Fatalf("ReadDir(%q) error = %v", root, err)
	}
	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry.Name())
		}
	}
	slices.Sort(dirs)
	return dirs
}

// contractFixture is the loader's own contract with each old-to-new
// substitution applied in turn and a privacy section appended, so a test that
// needs a vault declaring something different can vary one line of it instead
// of carrying a second copy that will drift from this one.
//
// A substitution matching nothing is fatal. Without that, a test naming a vault
// which declares no concepts would go on quietly exercising a vault that does,
// and would pass for the wrong reason for as long as nobody read it. The file
// is internal/schema's testdata, so reshaping one of its lines breaks the tests
// here that substitute into it.
func contractFixture(tb testing.TB, privateDirs []string, replacements ...[2]string) string {
	tb.Helper()
	data, err := os.ReadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		tb.Fatalf("ReadFile(schema fixture) error = %v", err)
	}
	text := string(data)
	for _, replacement := range replacements {
		next := strings.Replace(text, replacement[0], replacement[1], 1)
		if next == text {
			tb.Fatalf("the contract fixture does not contain %q", replacement[0])
		}
		text = next
	}
	quoted := make([]string, len(privateDirs))
	for i, dir := range privateDirs {
		quoted[i] = strconv.Quote(dir)
	}
	return text + "\n[privacy]\nnever_egress_dirs = [" + strings.Join(quoted, ", ") + "]\n"
}

// runPrepared drives one adjudicating engine the way the binary's front end
// does — prepare a payload, then re-validate the contract authority and close
// the observation before a byte is published — with the seams the front end
// cannot offer.
//
// Those seams name moments inside the engine, so no caller outside this package
// can reach them, and the tests that pin what happens at each of them have to
// live here. beforeEmission is separate from the observation's own hooks
// because it names a moment after the observation is over: the payload exists
// and nothing has been written yet, which is where the authority is asked a
// second time.
func runPrepared(
	ctx context.Context,
	tb testing.TB,
	command, root, name string,
	hooks actionHooks,
	beforeEmission func(),
) ([]byte, error) {
	tb.Helper()
	var prepared preparedCommand
	var err error
	switch command {
	case "check":
		prepared, err = prepareCheckWithHooks(ctx, &CheckOptions{Root: root, Format: FormatJSON}, hooks)
	case "coverage":
		prepared, err = prepareCoverageWithHooks(ctx, &CoverageOptions{Root: root, Format: FormatJSON}, hooks)
	case "exists":
		prepared, err = prepareExistsWithHooks(ctx, &ExistsOptions{Root: root, Name: name, Format: FormatJSON}, hooks)
	default:
		tb.Fatalf("runPrepared: unknown command %q", command)
	}
	if err != nil {
		return nil, err
	}
	if beforeEmission != nil {
		beforeEmission()
	}
	if _, err := prepared.seal(); err != nil {
		return nil, err
	}
	return prepared.stdout, nil
}

// refuse runs one adjudicating engine against root and returns the refusal it
// answered with, failing the test when it answered at all. The three engines
// classify a folder they cannot judge identically, so a test about a folder
// states its case once and runs it through each.
func refuse(ctx context.Context, tb testing.TB, command, root string) error {
	tb.Helper()
	payload, err := runPrepared(ctx, tb, command, root, "candidate", actionHooks{}, nil)
	if err == nil {
		// Fatal rather than an error: every caller reads the refusal's own
		// words, so returning a nil here would end the run in a dereference
		// panic instead of the report that says what changed.
		tb.Fatalf("%s produced %d bytes for a folder it cannot judge, and no refusal", command, len(payload))
	}
	if payload != nil {
		tb.Errorf("%s produced a payload alongside its refusal: %q", command, payload)
	}
	return err
}

// extractPathRefs returns every checkable file reference in body: Markdown
// [text](path.md) links, decoded once, and backticked path.md tokens, outside
// comments.
func extractPathRefs(body string, bodyStartLine int) []pathRef {
	facts := inspectBody(body, nil)
	refs, _, _ := extractReferencesFrom(body, bodyStartLine, facts.comments)
	return refs
}

// anchorSurface reads one body into what its page answers a fragment with:
// the section ids a link can land on, the sections an excerpt can cut to, and
// the lines that can carry a "^name" block address.
func anchorSurface(body string) (sections, excerptSections map[string]bool, blockAddresses []string) {
	facts := inspectBody(body, nil)
	return anchorSurfaceFrom(body, facts.comments)
}

// write creates a file at root/rel (rel in slash form), making parents.
func write(tb testing.TB, root, rel, content string) {
	tb.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		tb.Fatalf("mkdir for %q: %v", rel, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil { // #nosec G703 -- test fixture paths are owned by each caller's temporary root
		tb.Fatalf("write %q: %v", rel, err)
	}
}

// unreadable takes every permission from path for the rest of the test. It
// fails rather than skips where the filesystem ignores the request, as it does
// for root or on a filesystem without permissions: a test there would assert
// against a vault it never broke.
func unreadable(tb testing.TB, path string) {
	tb.Helper()
	if err := os.Chmod(path, 0); err != nil {
		tb.Fatalf("Chmod(%q, 0) error = %v", path, err)
	}
	tb.Cleanup(func() {
		if err := os.Chmod(path, 0o700); err != nil && !os.IsNotExist(err) { // #nosec G302 -- a directory needs owner execute permission so TempDir cleanup can traverse it
			tb.Errorf("restore %q: %v", path, err)
		}
	})
	info, err := os.Stat(path)
	if err != nil {
		tb.Fatalf("Stat(%q) error = %v", path, err)
	}
	if info.IsDir() {
		_, err = os.ReadDir(path)
	} else {
		_, err = os.ReadFile(path) // #nosec G304 -- the path is this test's own temporary file
	}
	if err == nil {
		tb.Fatalf("this process can still read %q after taking every permission from it", path)
	}
}

// runCheck runs the whole check engine over root and returns the wire bytes.
func runCheck(t *testing.T, root string) []byte {
	t.Helper()
	root = judgeFixtureRoot(t, root)
	findings, err := runCheckAction(t.Context(), root, nil, false)
	if err != nil {
		t.Fatalf("Check(%q): %v", root, err)
	}
	var buf bytes.Buffer
	if err := writeJSONL(&buf, findings); err != nil {
		t.Fatalf("WriteJSONL: %v", err)
	}
	return buf.Bytes()
}

// runSchema runs the frontmatter engine alone over root and returns the wire
// bytes.
func runSchema(t *testing.T, root string) []byte {
	t.Helper()
	notes, err := collectNotes(t.Context(), root)
	if err != nil {
		t.Fatalf("collectNotes(%q): %v", root, err)
	}
	s, err := schema.Load(root)
	if err != nil {
		t.Fatalf("schema.Load(%q): %v", root, err)
	}
	findings := checkSchema(notes, s)
	sortFindings(findings)
	var buf bytes.Buffer
	if err := writeJSONL(&buf, findings); err != nil {
		t.Fatalf("WriteJSONL: %v", err)
	}
	return buf.Bytes()
}

// The three commands return a sealed payload that can only be written; these
// write it into a buffer so an assertion can read it.

func writtenBytes(payload io.WriterTo, err error) ([]byte, error) {
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if _, err := payload.WriteTo(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func checkBytes(ctx context.Context, o *CheckOptions) (out []byte, exit int, err error) {
	payload, exit, err := RunCheck(ctx, o)
	out, err = writtenBytes(payload, err)
	return out, exit, err
}

func coverageBytes(ctx context.Context, o *CoverageOptions) (out []byte, exit int, err error) {
	payload, exit, err := RunCoverage(ctx, o)
	out, err = writtenBytes(payload, err)
	return out, exit, err
}

func existsBytes(ctx context.Context, o *ExistsOptions) (out []byte, exit int, err error) {
	payload, exit, err := RunExists(ctx, o)
	out, err = writtenBytes(payload, err)
	return out, exit, err
}

// decodeFindings reads JSONL check output back into findings.
func decodeFindings(tb testing.TB, data []byte) []Finding {
	tb.Helper()
	var findings []Finding
	decoder := json.NewDecoder(bytes.NewReader(data))
	for {
		var wire struct {
			Finding

			Severity string `json:"severity"`
		}
		err := decoder.Decode(&wire)
		if errors.Is(err, io.EOF) {
			return findings
		}
		if err != nil {
			tb.Fatalf("decode findings: %v", err)
		}
		f := wire.Finding
		switch wire.Severity {
		case "info":
			f.Severity = SeverityInfo
		case "warn":
			f.Severity = SeverityWarn
		case "error":
			f.Severity = SeverityError
		default:
			tb.Fatalf("decode findings: severity %q", wire.Severity)
		}
		findings = append(findings, f)
	}
}

// findFinding returns the first finding with rule whose path and target are
// the ones given, an empty one matching any, failing the test when there is
// none.
func findFinding(tb testing.TB, findings []Finding, rule RuleID, path, target string) Finding {
	tb.Helper()
	for i := range findings {
		f := &findings[i]
		if f.RuleID == rule && (path == "" || f.Path == path) && (target == "" || f.Target != nil && *f.Target == target) {
			return *f
		}
	}
	tb.Fatalf("no %s finding at %q for %q among %+v", rule, path, target, findings)
	return Finding{}
}

// findingKeys reduces each finding to its rule, field and target joined by
// spaces, leaving out a part the finding lacks: the identity most table tests
// compare, without the prose.
func findingKeys(findings []Finding) []string {
	var out []string
	for i := range findings {
		key := string(findings[i].RuleID)
		if findings[i].Field != nil {
			key += " " + *findings[i].Field
		}
		if findings[i].Target != nil {
			key += " " + *findings[i].Target
		}
		out = append(out, key)
	}
	return out
}

// noteVault writes notes beside a rules file that declares one knowledge
// directory, Notes, one note type and no field beyond type.
func noteVault(t *testing.T, notes map[string]string) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, schema.ContractRelPath, `[folders]
checked = ["Notes"]
templates = []

[enums]

[fields]
common = ["type"]
required = ["type"]

[types.note]

[privacy]
never_egress_dirs = []
`)
	for path, body := range notes {
		write(t, root, path, body)
	}
	return root
}

// coverageOf runs the coverage report over root and parses it back into the
// coverage value the assertions read.
func coverageOf(t *testing.T, root string) *coverageReport {
	t.Helper()
	out, _, err := coverageBytes(t.Context(), &CoverageOptions{Root: root, Format: FormatJSON})
	if err != nil {
		t.Fatalf("RunCoverage(%q): %v", root, err)
	}
	var cov coverageReport
	if err := json.Unmarshal(out, &cov); err != nil {
		t.Fatalf("parse coverage %q: %v", out, err)
	}
	return &cov
}

// wantMounts compares the concept tally of a coverage report: each domain's
// counts, the concepts pending a map, and the orphans.
func wantMounts(t *testing.T, cov *coverageReport, domains []domainCoverage, pending, orphans []string) {
	t.Helper()
	if diff := cmp.Diff(domains, cov.Domains, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("coverage domains (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(pending, cov.PendingMount, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("coverage pending_mount (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(orphans, cov.Orphans, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("coverage orphans (-want +got):\n%s", diff)
	}
}

// parseNote reads one note against the default marks.
func parseNote(rel string, data []byte) note {
	return parseNoteWithMarks(rel, data, defaultPlannedMarks())
}

// extractPlannedNames harvests one body's planned names against the default
// marks.
func extractPlannedNames(body string) []string {
	return extractPlannedNamesWith(body, defaultPlannedMarks())
}

// lintFrontmatter is what the reading server's linter says about one note.
func lintFrontmatter(relPath string, data []byte, contract *schema.Contract) []Finding {
	return NewFrontmatterLinter(contract).Lint(relPath, data)
}

// runCheckAction runs check over root the way RunCheck does and returns its
// findings in the wire order, scoped to paths and widened by all.
func runCheckAction(ctx context.Context, root string, paths []string, all bool) ([]Finding, error) {
	a, err := openAction(ctx, root, actionHooks{})
	if err != nil {
		return nil, err
	}
	findings, err := checkAction(a, paths, all)
	if err != nil {
		return nil, a.abort(err)
	}
	if err := a.finish(); err != nil {
		return nil, err
	}
	return findings, nil
}
