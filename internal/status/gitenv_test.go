package status_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestVaultSeedingIgnoresAnInheritedGitEnvironment holds the seeding helpers to
// the repository they were pointed at. git exports GIT_DIR, GIT_WORK_TREE and
// GIT_INDEX_FILE to every command it runs on your behalf, so a suite started
// from a rebase --exec, a bisect run or a hook inherits the name of the
// repository git is working in. The seeding here configures an identity and
// turns commit signing off; carried into an inherited repository those writes
// land in somebody's own checkout, which is how this test came to exist.
//
// The decoy stands in for that checkout. Its config is compared byte for byte,
// because the failure was not a broken repository — it was a working one that
// quietly started committing under another name.
func TestVaultSeedingIgnoresAnInheritedGitEnvironment(t *testing.T) {
	decoy := t.TempDir()
	runGit(t, decoy, "init", "--initial-branch=main")
	decoyConfig := filepath.Join(decoy, ".git", "config")
	before, err := os.ReadFile(decoyConfig) // #nosec G304 -- a path this test just created under t.TempDir()
	if err != nil {
		t.Fatalf("read decoy config: %v", err)
	}

	t.Setenv("GIT_DIR", filepath.Join(decoy, ".git"))
	t.Setenv("GIT_WORK_TREE", decoy)
	t.Setenv("GIT_INDEX_FILE", filepath.Join(decoy, ".git", "index"))
	// Config injection is the other half of the same inheritance. These three
	// set a config value on every git command in the environment without
	// writing it anywhere, so a helper that keeps them seeds a repository
	// carrying settings nobody in this file asked for.
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "yomihon.probe")
	t.Setenv("GIT_CONFIG_VALUE_0", "leaked")

	seeded := t.TempDir()
	initVault(t, seeded)

	after, err := os.ReadFile(decoyConfig) // #nosec G304 -- the same path read a second time
	if err != nil {
		t.Fatalf("re-read decoy config: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("seeding a scratch vault rewrote the inherited repository's config:\n--- before ---\n%s\n--- after ---\n%s", before, after)
	}

	// An injected setting is not written to any file, so the comparison above
	// cannot see it. The seeded repository is asked directly instead, with a
	// default so the question is answerable when the setting is absent.
	if probe := runGit(t, seeded, "config", "--default", "none", "--get", "yomihon.probe"); probe != "none\n" {
		t.Errorf("seeded vault carries an injected setting: yomihon.probe = %q, want it unset", probe)
	}

	// Without this the test would also pass if the seeding stopped working
	// altogether, which is a different bug wearing the same green.
	if name := runGit(t, seeded, "config", "user.name"); name != "Test Vault\n" {
		t.Errorf("seeded vault's user.name = %q, want the seeding identity", name)
	}
}
