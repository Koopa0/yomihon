package judge

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

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
		if len(privateDirs) == 0 {
			return copyRoot
		}
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
	write(tb, root, schema.ContractRelPath, contractFixture(tb, privateDirs,
		[2]string{
			`knowledge_dirs = ["Concepts", "Sources", "Maps", "Writing", "Synthesis", "Inbox"]`,
			"knowledge_dirs = []",
		}))
}

// contractFixture is the loader's own contract with each old-to-new
// substitution applied in turn and a privacy section appended, so a test that
// needs a vault declaring something different can vary one line of it instead
// of carrying a second copy that will drift from this one.
//
// A substitution matching nothing is fatal. Without that, a test naming a vault
// which declares no concepts would go on quietly exercising a vault that does,
// and would pass for the wrong reason for as long as nobody read it.
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
	if err := prepared.finish(); err != nil {
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
		tb.Errorf("%s produced %d bytes for a folder it cannot judge, and no refusal", command, len(payload))
	}
	if payload != nil {
		tb.Errorf("%s produced a payload alongside its refusal: %q", command, payload)
	}
	return err
}
