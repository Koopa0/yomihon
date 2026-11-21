package schema

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/koopa0/yomihon/internal/vault"
)

// A folder with no rules file reads everything and withholds nothing. These
// tests hold the other half: every way a rules file can be missing for a wrong
// reason keeps the old, closed answer, because that vault may well have
// withheld ground this process never learned.

func openAbsentReader(t *testing.T, files map[string]string) (reader *vault.Reader, root string) {
	t.Helper()
	root = t.TempDir()
	for rel, body := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", path, err)
		}
	}
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open(%q) error = %v", root, err)
	}
	t.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Errorf("Reader.Close() error = %v", closeErr)
		}
	})
	return reader, root
}

func TestAFolderWithNoRulesFileIsAbsentAndOneWithAnUnreadableFileIsNot(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		files map[string]string
		want  bool
	}{
		{name: "no rules file", files: map[string]string{"Notes/a.md": "# a\n"}, want: true},
		{name: "no System directory at all", files: map[string]string{}, want: true},
		{name: "a rules file for a version this build does not read", files: map[string]string{ContractRelPath: "schema_version = \"2\"\n"}},
		{name: "a rules file that is not TOML", files: map[string]string{ContractRelPath: "this is = = not toml"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			reader, _ := openAbsentReader(t, tt.files)
			_, err := LoadReader(t.Context(), reader)
			if err == nil {
				t.Fatalf("LoadReader() error = nil, want one for %s", tt.name)
			}
			if got := ContractAbsent(err); got != tt.want {
				t.Errorf("ContractAbsent(%v) = %t, want %t", err, got, tt.want)
			}
		})
	}
}

func TestARulesFileThatIsNotARegularFileIsNotAbsent(t *testing.T) {
	t.Parallel()
	reader, root := openAbsentReader(t, map[string]string{"Notes/a.md": "# a\n"})
	if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(ContractRelPath)), 0o750); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	_, err := LoadReader(t.Context(), reader)
	if err == nil || ContractAbsent(err) {
		t.Errorf("LoadReader() error = %v, ContractAbsent = %t, want a refusal that is not absence", err, ContractAbsent(err))
	}
}

// A save by rename leaves a gap between the lookup that finds the rules file
// and the read of it. The folder has rules, so it must not read as one with
// none: that would withhold nothing from it.
func TestARulesFileThatWentMissingAfterItWasFoundIsNotAbsent(t *testing.T) {
	t.Parallel()
	reader, root := openAbsentReader(t, map[string]string{ContractRelPath: "schema_version = \"1\"\n"})
	entry, err := reader.Lookup(ContractRelPath)
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	// A save by rename leaves this gap between the lookup and the read.
	if rmErr := os.Remove(filepath.Join(root, filepath.FromSlash(ContractRelPath))); rmErr != nil {
		t.Fatalf("Remove() error = %v", rmErr)
	}
	_, err = loadEntry(t.Context(), reader, entry)
	if err == nil {
		t.Fatal("loadEntry() error = nil, want one for a file that is gone")
	}
	if ContractAbsent(err) {
		t.Errorf("ContractAbsent(%v) = true: a vault with a rules file was taken for one with none, which withholds nothing", err)
	}
}

func TestAnAbsentFoldersPrivacyWithholdsNothingAndSaysWhy(t *testing.T) {
	t.Parallel()
	reader, _ := openAbsentReader(t, map[string]string{"Notes/a.md": "# a\n"})
	policy := PrivacyWithoutRules(reader).ValidateSource()
	if !policy.Available() || !policy.WithoutRules() {
		t.Fatalf("Available() = %t, WithoutRules() = %t, want both true", policy.Available(), policy.WithoutRules())
	}
	for _, rel := range []string{"Notes/a.md", "Diary/2026-01-01.md", "Private/x.md"} {
		if !policy.EgressAllowed(rel) {
			t.Errorf("EgressAllowed(%q) = false, want true where no rules file withholds anything", rel)
		}
	}
	if dirs := policy.WithheldDirs(); len(dirs) != 0 {
		t.Errorf("WithheldDirs() = %v, want none", dirs)
	}
	if policy.EgressAllowed("../outside.md") {
		t.Error(`EgressAllowed("../outside.md") = true, want a path outside the vault refused whatever the policy`)
	}
}

func TestARulesFileThatAppearsWithdrawsAnAbsentPolicy(t *testing.T) {
	t.Parallel()
	reader, root := openAbsentReader(t, map[string]string{"Notes/a.md": "# a\n"})
	policy := PrivacyWithoutRules(reader)
	if !policy.ValidateSource().Available() {
		t.Fatal("the policy was unavailable before any rules file appeared")
	}
	path := filepath.Join(root, filepath.FromSlash(ContractRelPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte("schema_version = \"1\"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	got := policy.ValidateSource()
	if got.Available() || got.EgressAllowed("Notes/a.md") {
		t.Errorf("after a rules file appeared: Available() = %t, EgressAllowed() = %t, want the output withdrawn", got.Available(), got.EgressAllowed("Notes/a.md"))
	}
	if policy.Available() {
		t.Error("the latch did not hold: a copy of the policy is still available after the rules file appeared")
	}
}

// A vault moved into the folder after the first look and before the walk that
// reads the notes is read as ordinary ground unless that walk asks too. The
// policy is withdrawn by what the walks entered, which is what the output was
// read from; a folder no walk entered gave the output nothing.
func TestAFolderAWalkEntersThatHoldsARulesFileWithdrawsAnAbsentPolicy(t *testing.T) {
	t.Parallel()
	appear := func(t *testing.T, root string) {
		t.Helper()
		for rel, content := range map[string]string{
			"Later/" + ContractRelPath: "schema_version = \"1\"\n",
			"Later/Diary/secret.md":    "# secret\n",
		} {
			full := filepath.Join(root, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
				t.Fatalf("MkdirAll() error = %v", err)
			}
			if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}
		}
	}

	t.Run("a walk that entered it", func(t *testing.T) {
		t.Parallel()
		reader, root := openAbsentReader(t, map[string]string{"Notes/a.md": "# a\n"})
		_, policy, err := LoadForAgents(t.Context(), reader)
		if err != nil || !policy.WithoutRules() {
			t.Fatalf("LoadForAgents() = %v, WithoutRules() = %t, want the policy of a folder with no rules file", err, policy.WithoutRules())
		}
		appear(t, root)
		if _, scanErr := reader.ScanComplete(t.Context()); scanErr != nil {
			t.Fatalf("ScanComplete() error = %v", scanErr)
		}
		got := policy.ValidateSource()
		if got.Available() || got.EgressAllowed("Later/Diary/secret.md") {
			t.Errorf("Available() = %t, EgressAllowed() = %t, want the output withdrawn: a walk read notes from a folder that holds a rules file", got.Available(), got.EgressAllowed("Later/Diary/secret.md"))
		}
		if !got.RulesFileNearby() {
			t.Error("RulesFileNearby() = false, want true: the withdrawal is the one a command may name")
		}
	})

	t.Run("no walk entered it", func(t *testing.T) {
		t.Parallel()
		reader, root := openAbsentReader(t, map[string]string{"Notes/a.md": "# a\n"})
		_, policy, err := LoadForAgents(t.Context(), reader)
		if err != nil {
			t.Fatalf("LoadForAgents() error = %v", err)
		}
		appear(t, root)
		if got := policy.ValidateSource(); !got.Available() || got.RulesFileNearby() {
			t.Errorf("Available() = %t, RulesFileNearby() = %t, want the policy to stand: nothing has been read from the new folder", got.Available(), got.RulesFileNearby())
		}
	})
}

func TestAnAbsentPolicyIsWithdrawnWhenTheRulesPathCannotBeTold(t *testing.T) {
	t.Parallel()
	reader, root := openAbsentReader(t, map[string]string{"Notes/a.md": "# a\n"})
	policy := PrivacyWithoutRules(reader)
	if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(ContractRelPath)), 0o750); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if got := policy.ValidateSource(); got.Available() {
		t.Error("ValidateSource() left the policy available though the rules path is not a regular file")
	}
}

func TestOnlyAnAbsentFoldersPrivacyIsPermissive(t *testing.T) {
	t.Parallel()
	for name, policy := range map[string]PrivacyPolicy{
		"the zero value":              {},
		"a nil contract":              (*Contract)(nil).PrivacyPolicy(),
		"a contract nothing declared": (&Contract{}).PrivacyPolicy(),
	} {
		if policy.Available() || policy.EgressAllowed("Notes/a.md") || policy.WithoutRules() {
			t.Errorf("%s: Available() = %t, EgressAllowed() = %t, WithoutRules() = %t, want a policy that says nothing may go out",
				name, policy.Available(), policy.EgressAllowed("Notes/a.md"), policy.WithoutRules())
		}
	}
}

func TestAFolderWithNoRulesFileHoldsItsNotesOpenAndAnUnreadableOneClosesThem(t *testing.T) {
	t.Parallel()
	absent := CaptureAuthority(nil, Ungoverned())
	if absent.Closed() {
		t.Errorf("Closed() = true for a folder with no rules file, want its notes read as instances")
	}
	for _, word := range []string{"unit", "lesson"} {
		if !absent.IsLessonType(word) {
			t.Errorf("IsLessonType(%q) = false for a folder with no rules file", word)
		}
	}
	if absent.IsLessonType("note") {
		t.Error(`IsLessonType("note") = true, want an ordinary type left alone`)
	}

	broken := CaptureAuthority(nil, Unreadable(errors.New("unsupported schema_version")))
	if !broken.Closed() {
		t.Error("Closed() = false for a rules file that could not be read, want its notes held back")
	}
	if broken.IsLessonType("unit") {
		t.Error(`IsLessonType("unit") = true for a rules file that could not be read: the built-in names are for a folder that has none`)
	}
}

func TestAnUnreadableRulesFileClosesEveryProjectionABuiltInSetWouldOpen(t *testing.T) {
	t.Parallel()
	var none *Contract
	caps := none.Capabilities(Unreadable(errors.New("unsupported schema_version")))
	if caps.Navigation.Available() || caps.Navigation.IsPathType("study-path") || caps.Navigation.IsLessonType("lesson") {
		t.Error("Capabilities(Unreadable) left the built-in course and unit types open")
	}
	if caps.Navigation.AnswerType() != "" {
		t.Errorf("AnswerType() = %q, want none", caps.Navigation.AnswerType())
	}
	if caps.Artifacts.Available() {
		t.Error("Capabilities(Unreadable) left the artifact policy available")
	}
}

// LoadForAgents is where a folder is told apart as having no rules file, so it
// is held directly: only a lookup that finds nothing gives the permissive
// policy, and a rules file that is there and is refused gives none, however it
// is refused.
func TestLoadForAgentsGivesNothingBackForAFileItCannotUse(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		files map[string]string
		// wantErr is whether the rules file was refused.
		wantErr     bool
		withoutRule bool
	}{
		{name: "no rules file", files: map[string]string{"Notes/a.md": "# a\n"}, withoutRule: true},
		{name: "a version this build does not read", files: map[string]string{ContractRelPath: "schema_version = \"2\"\n"}, wantErr: true},
		{name: "not TOML", files: map[string]string{ContractRelPath: "this is = = not toml"}, wantErr: true},
		{name: "a rules file that declares no privacy", files: map[string]string{ContractRelPath: "schema_version = \"1\"\n"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			reader, _ := openAbsentReader(t, tt.files)
			contract, policy, err := LoadForAgents(t.Context(), reader)
			if (err != nil) != tt.wantErr {
				t.Fatalf("LoadForAgents() error = %v, want an error: %t", err, tt.wantErr)
			}
			if got := policy.WithoutRules(); got != tt.withoutRule {
				t.Errorf("WithoutRules() = %t, want %t", got, tt.withoutRule)
			}
			if tt.wantErr && (contract != nil || policy.Available() || policy.EgressAllowed("Notes/a.md")) {
				t.Errorf("a refused rules file left a contract %v or a policy that says a path may go out", contract)
			}
			if tt.name == "a rules file that declares no privacy" && (policy.Available() || policy.WithoutRules()) {
				t.Errorf("a rules file with no [privacy] section was given a permissive policy: Available() = %t", policy.Available())
			}
		})
	}
}

// A command run from a folder inside a vault, or from one above it, is not run
// from a folder with no rules file: the rules that govern what it reads are
// somewhere this folder's own lookup does not reach. Every shape of that is
// refused whole, since the alternative is to read a vault's withheld folders as
// ordinary ones.
func TestLoadForAgentsRefusesAFolderThatIsInsideOrAboveAVault(t *testing.T) {
	t.Parallel()
	rules := "schema_version = \"1\"\n"
	tests := []struct {
		name  string
		files map[string]string
		open  string
	}{
		{name: "inside the folder the rules withhold", files: map[string]string{"v/" + ContractRelPath: rules, "v/Diary/Plan.md": "# Plan\n"}, open: "v/Diary"},
		{name: "two folders inside the vault", files: map[string]string{"v/" + ContractRelPath: rules, "v/Diary/2026/Plan.md": "# Plan\n"}, open: "v/Diary/2026"},
		{name: "one folder above a vault", files: map[string]string{"p/v/" + ContractRelPath: rules, "p/v/Diary/Plan.md": "# Plan\n"}, open: "p"},
		{name: "three folders above a vault", files: map[string]string{"p/a/b/v/" + ContractRelPath: rules, "p/a/b/v/Diary/Plan.md": "# Plan\n"}, open: "p"},
		{name: "above a vault whose rules path is spelled at other letter cases", files: map[string]string{"p/v/system/SCHEMAS/Vault-Schema.toml": rules}, open: "p"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			for rel, body := range tt.files {
				path := filepath.Join(root, filepath.FromSlash(rel))
				if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
					t.Fatalf("MkdirAll() error = %v", err)
				}
				if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
					t.Fatalf("WriteFile() error = %v", err)
				}
			}
			dir := filepath.Join(root, filepath.FromSlash(tt.open))
			if err := os.MkdirAll(dir, 0o750); err != nil {
				t.Fatalf("MkdirAll() error = %v", err)
			}
			reader, err := vault.Open(dir)
			if err != nil {
				t.Fatalf("vault.Open() error = %v", err)
			}
			t.Cleanup(func() { _ = reader.Close() }) //nolint:errcheck // closing a read-only test reader

			contract, policy, err := LoadForAgents(t.Context(), reader)
			if !errors.Is(err, ErrRulesFileNearby) {
				t.Fatalf("LoadForAgents() error = %v, want ErrRulesFileNearby", err)
			}
			if contract != nil || policy.Available() || policy.WithoutRules() || policy.EgressAllowed("Diary/Plan.md") {
				t.Errorf("a refused folder left a contract or a policy that says a path may go out")
			}
		})
	}
}

// The same rules path at another letter case is the rules file on a disk that
// ignores case and an unrelated file on one that does not. Either way the
// folder is not one that is known to have no rules.
func TestARulesPathAtAnotherLetterCaseIsNeverTakenForNoRulesFile(t *testing.T) {
	t.Parallel()
	reader, _ := openAbsentReader(t, map[string]string{"system/schemas/vault-schema.toml": "schema_version = \"1\"\n"})
	_, policy, err := LoadForAgents(t.Context(), reader)
	if err == nil {
		return // read as the rules file, which is what a disk that ignores case does
	}
	if !errors.Is(err, ErrRulesFileNearby) || policy.WithoutRules() {
		t.Errorf("LoadForAgents() = %v, WithoutRules() = %t, want the folder refused", err, policy.WithoutRules())
	}
}

func TestAFolderWithAnUnrelatedSystemFolderIsStillOneWithNoRulesFile(t *testing.T) {
	t.Parallel()
	reader, _ := openAbsentReader(t, map[string]string{"Notes/System/schemas/other.toml": "x = 1\n", "System/notes.md": "# n\n"})
	_, policy, err := LoadForAgents(t.Context(), reader)
	if err != nil || !policy.WithoutRules() {
		t.Errorf("LoadForAgents() error = %v, WithoutRules() = %t, want a folder with no rules file", err, policy.WithoutRules())
	}
}
