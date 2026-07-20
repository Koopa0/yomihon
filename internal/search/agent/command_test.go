package agent

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/search/semantic"
)

const searchTestContractBase = `schema_version = "1"

[enums]
type = ["concept", "lesson"]

[enums.status]
note = ["draft", "ready"]

[fields]
required = ["title", "type"]
known = ["title", "type", "based_on"]

[rules]
concept_requires_provenance = ["based_on"]
slug_pattern = "^[a-z]+$"

[scan]
knowledge_dirs = ["Writing", "Private"]

[[lifecycle]]
status = "draft"
applies_to = ["*"]
from = []
owner = ["koopa"]
`

type fakeGenerationBuilder struct {
	run func(context.Context) (semantic.BuildReport, error)
}

func (f fakeGenerationBuilder) Build(ctx context.Context, _ semantic.BuildRequest) (semantic.BuildReport, error) {
	return f.run(ctx)
}

type recordingGenerationBuilder struct {
	request semantic.BuildRequest
	report  semantic.BuildReport
}

func (b *recordingGenerationBuilder) Build(_ context.Context, request semantic.BuildRequest) (semantic.BuildReport, error) {
	b.request = request
	return b.report, nil
}

func TestResolveSearchRoot(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error: %v", err)
	}
	tests := []struct {
		name        string
		explicit    string
		configured  string
		defaultRoot string
		defaultErr  error
		wantCalls   int
		want        string
		wantErr     string
	}{
		{name: "explicit", explicit: "/chosen", configured: "/configured", defaultErr: errors.New("must stay lazy"), want: "/chosen"},
		{name: "explicit relative and unclean", explicit: "fixtures/../vault", want: filepath.Join(cwd, "vault")},
		{name: "configured", configured: "/configured", defaultErr: errors.New("must stay lazy"), want: "/configured"},
		{name: "configured relative and unclean", configured: "fixtures/../configured", want: filepath.Join(cwd, "configured")},
		{name: "absolute unclean", explicit: "/chosen/one/../two", want: "/chosen/two"},
		{name: "command-owned default", defaultRoot: "/home/reader/obsidian", wantCalls: 1, want: "/home/reader/obsidian"},
		{name: "default unavailable", defaultErr: errors.New("home directory is unavailable"), wantCalls: 1, wantErr: "resolve vault root: home directory is unavailable"},
		{name: "no default", wantErr: "resolve vault root: home directory is unavailable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			var defaultRoot func() (string, error)
			if tt.defaultRoot != "" || tt.defaultErr != nil {
				defaultRoot = func() (string, error) {
					calls++
					return tt.defaultRoot, tt.defaultErr
				}
			}
			got, err := resolveRoot(tt.explicit, rootConfig{
				rootEnv:     tt.configured,
				defaultRoot: defaultRoot,
			})
			if calls != tt.wantCalls {
				t.Fatalf("default root calls = %d, want %d", calls, tt.wantCalls)
			}
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("resolveSearchRoot() error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveSearchRoot() error: %v", err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("resolveSearchRoot() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestRunCommandLexicalDoesNotResolveSemanticDependencies(t *testing.T) {
	root := writeSearchTestVault(t, `
[artifacts]
non_instance_dirs = []

[privacy]
never_egress_dirs = []
`, map[string]string{
		"Writing/public.md": "---\ntitle: Public\ntype: concept\nstatus: draft\n---\nbody token\n",
	})
	home := t.TempDir()
	cache := isolateUserCache(t, home)

	var stdout, stderr bytes.Buffer
	exit := (CLI{
		Root:             root,
		Stdout:           &stdout,
		Stderr:           &stderr,
		ReadEmbeddingKey: func() string { panic("lexical search read the embedding key") },
	}).Run(t.Context(), "search", []string{"--json", "token"})
	if exit != 0 {
		t.Fatalf("RunCommand() exit = %d, want 0; stderr = %q", exit, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, `"semantic":"off"`) {
		t.Errorf("RunCommand() stdout = %q, want lexical-only answer", got)
	}
	if stderr.Len() != 0 {
		t.Errorf("RunCommand() stderr = %q, want empty", stderr.Bytes())
	}
	if _, err := os.Stat(filepath.Join(cache, "yomihon")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("lexical search semantic cache stat error = %v, want not-exist", err)
	}
}

func isolateUserCache(t *testing.T, home string) string {
	t.Helper()
	cache := filepath.Join(home, "cache")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CACHE_HOME", cache)
	t.Setenv("LOCALAPPDATA", cache)
	resolved, err := os.UserCacheDir()
	if err != nil {
		t.Fatalf("os.UserCacheDir() error: %v", err)
	}
	return resolved
}

func TestRunCommandSemanticColdStoreStopsBeforeCredential(t *testing.T) {
	root := writeSearchTestVault(t, `
[artifacts]
non_instance_dirs = []

[privacy]
never_egress_dirs = []
`, map[string]string{
		"Writing/public.md": "---\ntitle: Public\ntype: concept\nstatus: draft\n---\nbody token\n",
	})
	isolateUserCache(t, t.TempDir())
	keyReads := 0
	capabilities, err := loadVaultCapabilities(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := capabilities.reader.Close(); closeErr != nil {
			t.Errorf("close vault reader: %v", closeErr)
		}
	})
	if _, err := newProductionIndexer(semanticActionConfig{
		reader:   capabilities.reader,
		artifact: capabilities.artifact,
		privacy:  capabilities.privacy,
	}, func() string {
		keyReads++
		return "must-not-be-read"
	}, semantic.NewIndexer); err != nil {
		t.Fatalf("newProductionIndexer() error: %v", err)
	}
	var stdout, stderr bytes.Buffer
	exit := (CLI{
		Root:   root,
		Stdout: &stdout,
		Stderr: &stderr,
		ReadEmbeddingKey: func() string {
			keyReads++
			return "must-not-be-read"
		},
	}).Run(t.Context(), "search", []string{"--json", "--semantic", "token"})
	if exit != 3 {
		t.Fatalf("RunCommand() exit = %d, want 3; stderr = %q", exit, stderr.String())
	}
	if got, want := stdout.String(), "{\"mode\":\"lexical\",\"semantic\":\"unavailable\",\"coverage\":{\"reason\":\"cache-cold\"},\"results\":[{\"rank\":1,\"rel_path\":\"Writing/public.md\",\"title\":\"Public\",\"status\":\"draft\",\"snippet\":\"body token\",\"channels\":[\"lexical\"],\"channel_ranks\":{\"lexical\":1}}]}\n"; got != want {
		t.Errorf("RunCommand() stdout = %q, want %q", got, want)
	}
	if got, want := stderr.String(), "yomihon search: cache-cold: no semantic index exists; run yomihon search-index build\n"; got != want {
		t.Errorf("RunCommand() stderr = %q, want %q", got, want)
	}
	if keyReads != 0 {
		t.Errorf("RunCommand() embedding-key reads = %d, want zero", keyReads)
	}
}

func TestSearchCommandUsageFailuresArePreCapabilityAndRedacted(t *testing.T) {
	const sentinel = "sentinel-private-query key-sentinel"
	t.Run("killed best-effort spelling", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		exit := runSearch(t.Context(), []string{"--semantic=best-effort", sentinel}, rootConfig{}, searchDeps{
			stdout: &stdout,
			stderr: &stderr,
		})
		if exit != 2 {
			t.Errorf("runSearch() exit = %d, want 2", exit)
		}
		if stdout.Len() != 0 {
			t.Errorf("runSearch() stdout = %q, want empty", stdout.Bytes())
		}
		want := "yomihon search: flag --semantic takes no value\n"
		if got := stderr.String(); got != want {
			t.Errorf("runSearch() stderr = %q, want %q", got, want)
		}
		if bytes.Contains(stderr.Bytes(), []byte(sentinel)) {
			t.Errorf("runSearch() stderr echoed sentinel: %q", stderr.Bytes())
		}
	})
	t.Run("search-index positional", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		exit := runSearchIndex(t.Context(), []string{"build", sentinel}, rootConfig{}, buildCommandDeps{
			stdout: &stdout,
			stderr: &stderr,
		})
		if exit != 2 {
			t.Errorf("runSearchIndex() exit = %d, want 2", exit)
		}
		if stdout.Len() != 0 {
			t.Errorf("runSearchIndex() stdout = %q, want empty", stdout.Bytes())
		}
		want := "yomihon search-index: build takes no arguments\n"
		if got := stderr.String(); got != want {
			t.Errorf("runSearchIndex() stderr = %q, want %q", got, want)
		}
		if bytes.Contains(stderr.Bytes(), []byte(sentinel)) {
			t.Errorf("runSearchIndex() stderr echoed sentinel: %q", stderr.Bytes())
		}
	})
	t.Run("search-index renewal takes no value", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		exit := runSearchIndex(t.Context(), []string{"build", "--json", "--renew-attempt-budget=true"}, rootConfig{}, buildCommandDeps{
			stdout: &stdout,
			stderr: &stderr,
		})
		if exit != 2 {
			t.Errorf("runSearchIndex() exit = %d, want 2", exit)
		}
		if stdout.Len() != 0 {
			t.Errorf("runSearchIndex() stdout = %q, want empty", stdout.Bytes())
		}
		want := "yomihon search-index: flag --renew-attempt-budget takes no value\n"
		if got := stderr.String(); got != want {
			t.Errorf("runSearchIndex() stderr = %q, want %q", got, want)
		}
	})
}

func TestRunSearchPrivacyGate(t *testing.T) {
	root := writeSearchTestVault(t, `
[artifacts]
non_instance_dirs = []
`, map[string]string{
		"Writing/public.md": "---\ntitle: Public\ntype: concept\nstatus: draft\n---\nbody token\n",
	})
	var stdout, stderr bytes.Buffer
	exit := runSearch(t.Context(), []string{"--json", "token"}, rootConfig{rootEnv: root}, searchDeps{
		stdout: &stdout,
		stderr: &stderr,
	})
	if exit != 3 {
		t.Errorf("runSearch() exit = %d, want 3", exit)
	}
	wantStdout := "{\"error\":{\"reason\":\"privacy-capability-unavailable\"}}\n"
	if got := stdout.String(); got != wantStdout {
		t.Errorf("runSearch() stdout = %q, want %q", got, wantStdout)
	}
	wantStderr := "yomihon search: privacy-capability-unavailable: the vault contract declares no valid privacy policy, so agent-facing search output is closed\n"
	if got := stderr.String(); got != wantStderr {
		t.Errorf("runSearch() stderr = %q, want %q", got, wantStderr)
	}
}

func TestRunSearchMetadataGate(t *testing.T) {
	root := writeSearchTestVault(t, `
[privacy]
never_egress_dirs = ["Private"]
`, map[string]string{
		"Writing/public.md": "---\ntitle: Public\ntype: concept\nstatus: draft\n---\nbody token\n",
	})
	var stdout, stderr bytes.Buffer
	exit := runSearch(t.Context(), []string{"--json", "type:concept", "token"}, rootConfig{rootEnv: root}, searchDeps{
		stdout: &stdout,
		stderr: &stderr,
	})
	if exit != 3 {
		t.Errorf("runSearch() exit = %d, want 3", exit)
	}
	wantStdout := "{\"error\":{\"reason\":\"metadata-filters-unavailable\"}}\n"
	if got := stdout.String(); got != wantStdout {
		t.Errorf("runSearch() stdout = %q, want %q", got, wantStdout)
	}
	wantStderr := "yomihon search: metadata-filters-unavailable: the vault contract declares no valid artifact policy, so metadata filters cannot be evaluated\n"
	if got := stderr.String(); got != wantStderr {
		t.Errorf("runSearch() stderr = %q, want %q", got, wantStderr)
	}
}

func TestRunSearchPreservesLexicalAnswerWhenChunkingFails(t *testing.T) {
	root := writeSearchTestVault(t, `
[artifacts]
non_instance_dirs = ["Exports"]

[privacy]
never_egress_dirs = []
`, map[string]string{
		"Writing/oversize.md": "---\ntitle: " + strings.Repeat("界", semantic.ChunkTokenCap+1) + "\ntype: concept\n---\nbody\n",
		"Exports/result.md":   "---\ntitle: Result\n---\ntoken\n",
	})
	var stdout, stderr bytes.Buffer
	providerOpened := false
	exit := runSearch(t.Context(), []string{"--json", "--semantic", "token"}, rootConfig{rootEnv: root}, searchDeps{
		stdout: &stdout,
		stderr: &stderr,
		openSemantic: func(semanticActionConfig) (reconcileSearch, error) {
			providerOpened = true
			return nil, errors.New("must not open semantic provider")
		},
	})
	if exit != 3 {
		t.Errorf("runSearch() exit = %d, want 3", exit)
	}
	wantStdout := "{\"mode\":\"lexical\",\"semantic\":\"unavailable\",\"coverage\":{\"reason\":\"index-incomplete\"},\"results\":[{\"rank\":1,\"rel_path\":\"Exports/result.md\",\"title\":\"Result\",\"snippet\":\"token\",\"channels\":[\"lexical\"],\"channel_ranks\":{\"lexical\":1}}]}\n"
	if got := stdout.String(); got != wantStdout {
		t.Errorf("runSearch() stdout = %q, want %q", got, wantStdout)
	}
	wantStderr := "yomihon search: index-incomplete: one or more current chunks could not be indexed\n"
	if got := stderr.String(); got != wantStderr {
		t.Errorf("runSearch() stderr = %q, want %q", got, wantStderr)
	}
	if providerOpened {
		t.Error("runSearch() opened provider after local chunking failure")
	}
}

func TestRunSearchFiltersPrivateResultsBeforeLimitAndRank(t *testing.T) {
	root := writeSearchTestVault(t, `
[artifacts]
non_instance_dirs = []

[privacy]
never_egress_dirs = ["Private"]
`, map[string]string{
		"Private/first.md":  "---\ntitle: Secret\ntype: concept\nstatus: draft\n---\nbody token\n",
		"Writing/public.md": "---\ntitle: Public\ntype: concept\nstatus: draft\n---\nbody token\n",
	})
	var stdout, stderr bytes.Buffer
	exit := runSearch(t.Context(), []string{"--json", "--limit=1", "token"}, rootConfig{rootEnv: root}, searchDeps{
		stdout: &stdout,
		stderr: &stderr,
	})
	if exit != 0 {
		t.Errorf("runSearch() exit = %d, want 0", exit)
	}
	want := "{\"mode\":\"lexical\",\"semantic\":\"off\",\"results\":[{\"rank\":1,\"rel_path\":\"Writing/public.md\",\"title\":\"Public\",\"status\":\"draft\",\"snippet\":\"body token\",\"channels\":[\"lexical\"],\"channel_ranks\":{\"lexical\":1}}]}\n"
	if got := stdout.String(); got != want {
		t.Errorf("runSearch() stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Errorf("runSearch() stderr = %q, want empty", stderr.Bytes())
	}
}

func TestPrivateSourceDoesNotChangePublicHybridOrder(t *testing.T) {
	publicNotes := map[string]string{
		"Writing/a.md": "---\ntitle: A\ntype: concept\nstatus: ready\n---\ntoken\n",
		"Writing/b.md": "---\ntitle: B\ntype: concept\nstatus: ready\n---\ntoken\n",
	}
	withPrivate := make(map[string]string, len(publicNotes)+1)
	maps.Copy(withPrivate, publicNotes)
	withPrivate["Private/first.md"] = "---\ntitle: Private\ntype: concept\nstatus: ready\n---\ntoken\n"

	want := runPublicHybridPaths(t, publicNotes)
	got := runPublicHybridPaths(t, withPrivate)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("public hybrid order changed after adding a private source (-want +got):\n%s", diff)
	}
}

func runPublicHybridPaths(t *testing.T, notes map[string]string) []string {
	t.Helper()
	root := writeSearchTestVault(t, `
[artifacts]
non_instance_dirs = []

[privacy]
never_egress_dirs = ["Private"]
`, notes)
	var stdout, stderr bytes.Buffer
	exit := runSearch(t.Context(), []string{"--json", "--semantic", "--limit=2", "token"}, rootConfig{rootEnv: root}, searchDeps{
		stdout: &stdout,
		stderr: &stderr,
		openSemantic: func(semanticActionConfig) (reconcileSearch, error) {
			return func(_ context.Context, corpus semantic.Corpus) (SemanticSearch, error) {
				return newFakeSemanticSearch(corpus), nil
			}, nil
		},
	})
	if exit != 0 {
		t.Fatalf("runSearch() exit = %d, want 0; stderr = %q", exit, stderr.String())
	}
	var answer searchAnswerEnvelope
	if err := json.Unmarshal(stdout.Bytes(), &answer); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}
	paths := make([]string, 0, len(answer.Results))
	for i := range answer.Results {
		result := &answer.Results[i]
		if strings.HasPrefix(result.RelPath, "Writing/") {
			paths = append(paths, result.RelPath)
		}
	}
	return paths
}

func TestRunSearchPartialOutputIsToolFailure(t *testing.T) {
	root := writeSearchTestVault(t, `
[artifacts]
non_instance_dirs = []

[privacy]
never_egress_dirs = ["Private"]
`, map[string]string{
		"Writing/public.md": "---\ntitle: Public\ntype: concept\nstatus: draft\n---\nbody token\n",
	})
	var stdout partialWriter
	var stderr bytes.Buffer
	exit := runSearch(t.Context(), []string{"--json", "token"}, rootConfig{rootEnv: root}, searchDeps{
		stdout: &stdout,
		stderr: &stderr,
	})
	if exit != 2 {
		t.Errorf("runSearch() exit = %d, want 2", exit)
	}
	if stdout.Len() == 0 || stdout.Bytes()[stdout.Len()-1] == '\n' {
		t.Errorf("runSearch() partial stdout = %q, want nonempty incomplete frame", stdout.Bytes())
	}
	if got, want := stderr.String(), "yomihon search: write output\n"; got != want {
		t.Errorf("runSearch() stderr = %q, want %q", got, want)
	}
}

func TestRunSearchSemanticNotApplicable(t *testing.T) {
	root := writeSearchTestVault(t, `
[artifacts]
non_instance_dirs = []

[privacy]
never_egress_dirs = ["Private"]
`, map[string]string{
		"Writing/public.md": "---\ntitle: Public\ntype: concept\nstatus: draft\n---\nbody\n",
	})
	var stdout, stderr bytes.Buffer
	exit := runSearch(t.Context(), []string{"--json", "--semantic", "type:concept"}, rootConfig{rootEnv: root}, searchDeps{
		stdout: &stdout,
		stderr: &stderr,
	})
	if exit != 0 {
		t.Errorf("runSearch() exit = %d, want 0", exit)
	}
	want := "{\"mode\":\"lexical\",\"semantic\":\"not-applicable\",\"coverage\":{\"reason\":\"not-applicable\"},\"results\":[{\"rank\":1,\"rel_path\":\"Writing/public.md\",\"title\":\"Public\",\"status\":\"draft\",\"channels\":[\"lexical\"],\"channel_ranks\":{\"lexical\":1}}]}\n"
	if got := stdout.String(); got != want {
		t.Errorf("runSearch() stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Errorf("runSearch() stderr = %q, want empty", stderr.Bytes())
	}
}

func TestRunSearchFolderOnlySemanticDoesNotRequireArtifactPolicy(t *testing.T) {
	root := writeSearchTestVault(t, `
[privacy]
never_egress_dirs = ["Private"]
`, map[string]string{
		"Writing/public.md": "---\ntitle: Public\ntype: concept\nstatus: draft\n---\nbody\n",
	})
	var stdout, stderr bytes.Buffer
	exit := runSearch(t.Context(), []string{"--json", "--semantic", "folder:Writing"}, rootConfig{rootEnv: root}, searchDeps{
		stdout: &stdout,
		stderr: &stderr,
	})
	if exit != 0 {
		t.Errorf("runSearch() exit = %d, want 0", exit)
	}
	want := "{\"mode\":\"lexical\",\"semantic\":\"not-applicable\",\"coverage\":{\"reason\":\"not-applicable\"},\"results\":[{\"rank\":1,\"rel_path\":\"Writing/public.md\",\"title\":\"Public\",\"channels\":[\"lexical\"],\"channel_ranks\":{\"lexical\":1}}]}\n"
	if got := stdout.String(); got != want {
		t.Errorf("runSearch() stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Errorf("runSearch() stderr = %q, want empty", stderr.Bytes())
	}
}

func TestRunSearchSemanticArtifactUnavailablePreservesLexicalAnswer(t *testing.T) {
	root := writeSearchTestVault(t, `
[privacy]
never_egress_dirs = ["Private"]
`, map[string]string{
		"Writing/public.md": "---\ntitle: Public\ntype: concept\nstatus: draft\n---\nbody token\n",
	})
	var stdout, stderr bytes.Buffer
	exit := runSearch(t.Context(), []string{"--json", "--semantic", "token"}, rootConfig{rootEnv: root}, searchDeps{
		stdout: &stdout,
		stderr: &stderr,
	})
	if exit != 3 {
		t.Errorf("runSearch() exit = %d, want 3", exit)
	}
	wantStdout := "{\"mode\":\"lexical\",\"semantic\":\"unavailable\",\"coverage\":{\"reason\":\"artifact-policy-unavailable\"},\"results\":[{\"rank\":1,\"rel_path\":\"Writing/public.md\",\"title\":\"Public\",\"snippet\":\"body token\",\"channels\":[\"lexical\"],\"channel_ranks\":{\"lexical\":1}}]}\n"
	if got := stdout.String(); got != wantStdout {
		t.Errorf("runSearch() stdout = %q, want %q", got, wantStdout)
	}
	wantStderr := "yomihon search: artifact-policy-unavailable: the vault contract declares no valid artifact policy, so the semantic corpus cannot exist\n"
	if got := stderr.String(); got != wantStderr {
		t.Errorf("runSearch() stderr = %q, want %q", got, wantStderr)
	}
}

func TestRunSearchHumanUnavailablePrintsLexicalAnswerAndDiagnostic(t *testing.T) {
	root := writeSearchTestVault(t, `
[artifacts]
non_instance_dirs = []

[privacy]
never_egress_dirs = ["Private"]
`, map[string]string{
		"Writing/public.md": "---\ntitle: Public\ntype: concept\nstatus: draft\n---\nbody token\n",
	})
	var stdout, stderr bytes.Buffer
	exit := runSearch(t.Context(), []string{"--semantic", "token"}, rootConfig{rootEnv: root}, searchDeps{
		stdout: &stdout,
		stderr: &stderr,
		openSemantic: func(semanticActionConfig) (reconcileSearch, error) {
			return func(context.Context, semantic.Corpus) (SemanticSearch, error) {
				return nil, semantic.ErrStoreNotFound
			}, nil
		},
	})
	if exit != 3 {
		t.Errorf("runSearch() exit = %d, want 3", exit)
	}
	wantStdout := "1. Public — Writing/public.md [draft] (lexical)\n   body token\n"
	if got := stdout.String(); got != wantStdout {
		t.Errorf("runSearch() stdout = %q, want %q", got, wantStdout)
	}
	wantStderr := "yomihon search: cache-cold: no semantic index exists; run yomihon search-index build\n"
	if got := stderr.String(); got != wantStderr {
		t.Errorf("runSearch() stderr = %q, want %q", got, wantStderr)
	}
}

func TestRunSearchSemanticSuccess(t *testing.T) {
	root := writeSearchTestVault(t, `
[artifacts]
non_instance_dirs = []

[privacy]
never_egress_dirs = ["Private"]
	`, map[string]string{
		"Writing/semantic.md": "---\ntitle: Semantic\ntype: concept\nstatus: ready\n---\n## Evidence\nsemantic evidence\n",
	})
	var stdout, stderr bytes.Buffer
	exit := runSearch(t.Context(), []string{"--json", "--semantic", "conceptual"}, rootConfig{rootEnv: root}, searchDeps{
		stdout: &stdout,
		stderr: &stderr,
		openSemantic: func(semanticActionConfig) (reconcileSearch, error) {
			return func(_ context.Context, corpus semantic.Corpus) (SemanticSearch, error) {
				return newFakeSemanticSearch(corpus), nil
			}, nil
		},
	})
	if exit != 0 {
		t.Errorf("runSearch() exit = %d, want 0", exit)
	}
	want := "{\"mode\":\"hybrid\",\"semantic\":\"ok\",\"results\":[{\"rank\":1,\"rel_path\":\"Writing/semantic.md\",\"title\":\"Semantic\",\"status\":\"ready\",\"snippet\":\"semantic evidence\",\"heading\":\"Evidence\",\"channels\":[\"semantic\"],\"channel_ranks\":{\"semantic\":1}}]}\n"
	if got := stdout.String(); got != want {
		t.Errorf("runSearch() stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Errorf("runSearch() stderr = %q, want empty", stderr.Bytes())
	}
}

func TestRunSearchFiltersSemanticAllowedPathsBeforeRanking(t *testing.T) {
	root := writeSearchTestVault(t, `
[artifacts]
non_instance_dirs = []

[privacy]
never_egress_dirs = ["Private"]
`, map[string]string{
		"Private/private-note.md": "---\ntitle: Private Note\ntype: concept\nstatus: ready\n---\nsemantic evidence\n",
	})
	var stdout, stderr bytes.Buffer
	exit := runSearch(t.Context(), []string{"--json", "--semantic", "conceptual"}, rootConfig{rootEnv: root}, searchDeps{
		stdout: &stdout,
		stderr: &stderr,
		openSemantic: func(semanticActionConfig) (reconcileSearch, error) {
			return func(_ context.Context, corpus semantic.Corpus) (SemanticSearch, error) {
				return newFakeSemanticSearch(corpus), nil
			}, nil
		},
	})
	if exit != 0 {
		t.Errorf("runSearch() exit = %d, want 0", exit)
	}
	want := "{\"mode\":\"hybrid\",\"semantic\":\"ok\",\"results\":[]}\n"
	if got := stdout.String(); got != want {
		t.Errorf("runSearch() stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Errorf("runSearch() stderr = %q, want empty", stderr.Bytes())
	}
}

func TestRunSearchRechecksPrivacyBeforeEveryAnswerableOutput(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		semantic func(*testing.T) func(semanticActionConfig) (reconcileSearch, error)
	}{
		{name: "lexical off", args: []string{"--json", "token"}},
		{name: "lexical metadata", args: []string{"--json", "type:concept", "token"}},
		{name: "semantic not applicable", args: []string{"--json", "--semantic", "type:concept"}},
		{
			name: "hybrid success",
			args: []string{"--json", "--semantic", "conceptual"},
			semantic: func(t *testing.T) func(semanticActionConfig) (reconcileSearch, error) {
				t.Helper()
				return func(semanticActionConfig) (reconcileSearch, error) {
					return func(_ context.Context, corpus semantic.Corpus) (SemanticSearch, error) {
						return newFakeSemanticSearch(corpus), nil
					}, nil
				}
			},
		},
		{
			name: "semantic failure",
			args: []string{"--json", "--semantic", "token"},
			semantic: func(*testing.T) func(semanticActionConfig) (reconcileSearch, error) {
				return func(semanticActionConfig) (reconcileSearch, error) {
					return func(context.Context, semantic.Corpus) (SemanticSearch, error) {
						return nil, semantic.ErrPolicySourceChanged
					}, nil
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := writeSearchTestVault(t, `
[artifacts]
non_instance_dirs = []

[privacy]
never_egress_dirs = ["Private"]
`, map[string]string{
				"Writing/public.md": "---\ntitle: Public\ntype: concept\nstatus: draft\n---\nbody token\n",
			})
			var stdout, stderr bytes.Buffer
			deps := searchDeps{
				stdout: &stdout,
				stderr: &stderr,
				beforeCapabilityRecheck: func() {
					writeSearchTestFile(t, root, "System/schemas/vault-schema.toml", searchTestContractBase+`
[artifacts]
non_instance_dirs = []
`)
				},
			}
			if tt.semantic != nil {
				deps.openSemantic = tt.semantic(t)
			}
			exit := runSearch(t.Context(), tt.args, rootConfig{rootEnv: root}, deps)
			if exit != 3 {
				t.Errorf("runSearch() exit = %d, want 3", exit)
			}
			wantStdout := "{\"error\":{\"reason\":\"privacy-capability-unavailable\"}}\n"
			if got := stdout.String(); got != wantStdout {
				t.Errorf("runSearch() stdout = %q, want %q", got, wantStdout)
			}
			wantStderr := "yomihon search: privacy-capability-unavailable: the vault contract declares no valid privacy policy, so agent-facing search output is closed\n"
			if got := stderr.String(); got != wantStderr {
				t.Errorf("runSearch() stderr = %q, want %q", got, wantStderr)
			}
		})
	}
}

func TestSearchCommandsClassifySetupSourceDriftThroughCapabilityGate(t *testing.T) {
	newRoot := func(t *testing.T) string {
		t.Helper()
		return writeSearchTestVault(t, `
[artifacts]
non_instance_dirs = []

[privacy]
never_egress_dirs = ["Private"]
`, map[string]string{
			"Writing/public.md": "---\ntitle: Public\ntype: concept\nstatus: draft\n---\nbody token\n",
		})
	}
	drift := func(t *testing.T, root string) {
		t.Helper()
		writeSearchTestFile(t, root, "System/schemas/vault-schema.toml", searchTestContractBase+`
[artifacts]
non_instance_dirs = []
`)
	}
	t.Run("search", func(t *testing.T) {
		root := newRoot(t)
		var stdout, stderr bytes.Buffer
		exit := runSearch(t.Context(), []string{"--json", "--semantic", "token"}, rootConfig{rootEnv: root}, searchDeps{
			stdout: &stdout,
			stderr: &stderr,
			openSemantic: func(semanticActionConfig) (reconcileSearch, error) {
				drift(t, root)
				return nil, semantic.ErrPolicySourceChanged
			},
		})
		assertPrivacyUnavailable(t, "runSearch", exit, stdout.String(), stderr.String())
	})
	t.Run("search-index", func(t *testing.T) {
		root := newRoot(t)
		var stdout, stderr bytes.Buffer
		exit := runSearchIndex(t.Context(), []string{"build", "--json"}, rootConfig{rootEnv: root}, buildCommandDeps{
			stdout: &stdout,
			stderr: &stderr,
			openBuilder: func(semanticActionConfig) (generationBuilder, error) {
				drift(t, root)
				return nil, semantic.ErrPolicySourceChanged
			},
		})
		assertPrivacyUnavailable(t, "runSearchIndex", exit, stdout.String(), stderr.String())
	})
}

func TestSearchCommandsDoNotMislabelSetupDefectsAsRuntimeState(t *testing.T) {
	root := writeSearchTestVault(t, `
[artifacts]
non_instance_dirs = []

[privacy]
never_egress_dirs = ["Private"]
`, map[string]string{
		"Writing/public.md": "---\ntitle: Public\ntype: concept\nstatus: draft\n---\nbody token\n",
	})
	t.Run("search", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		exit := runSearch(t.Context(), []string{"--json", "--semantic", "token"}, rootConfig{rootEnv: root}, searchDeps{
			stdout: &stdout,
			stderr: &stderr,
			openSemantic: func(semanticActionConfig) (reconcileSearch, error) {
				return nil, semantic.ErrInvalidIdentity
			},
		})
		if exit != 2 || stdout.Len() != 0 || stderr.String() != "yomihon search: semantic command setup failed\n" {
			t.Errorf("runSearch() = exit %d, stdout %q, stderr %q", exit, stdout.Bytes(), stderr.Bytes())
		}
	})
	t.Run("search-index", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		exit := runSearchIndex(t.Context(), []string{"build", "--json"}, rootConfig{rootEnv: root}, buildCommandDeps{
			stdout: &stdout,
			stderr: &stderr,
			openBuilder: func(semanticActionConfig) (generationBuilder, error) {
				return nil, semantic.ErrInvalidIdentity
			},
		})
		if exit != 2 || stdout.Len() != 0 || stderr.String() != "yomihon search-index: semantic command setup failed\n" {
			t.Errorf("runSearchIndex() = exit %d, stdout %q, stderr %q", exit, stdout.Bytes(), stderr.Bytes())
		}
	})
}

func assertPrivacyUnavailable(t *testing.T, command string, exit int, stdout, stderr string) {
	t.Helper()
	if exit != 3 {
		t.Errorf("%s() exit = %d, want 3", command, exit)
	}
	wantStdout := "{\"error\":{\"reason\":\"privacy-capability-unavailable\"}}\n"
	if command == "runSearchIndex" {
		wantStdout = "{\"error\":{\"reason\":\"privacy-capability-unavailable\",\"active_generation\":\"not-inspected\",\"staging_generation\":\"not-inspected\",\"retry_safe\":false,\"next_action\":\"repair-vault-contract\"}}\n"
	}
	if stdout != wantStdout {
		t.Errorf("%s() stdout = %q, want %q", command, stdout, wantStdout)
	}
	wantStderr := commandPrefix(command) + ": privacy-capability-unavailable: the vault contract declares no valid privacy policy, so agent-facing search output is closed\n"
	if stderr != wantStderr {
		t.Errorf("%s() stderr = %q, want %q", command, stderr, wantStderr)
	}
}

func commandPrefix(command string) string {
	if command == "runSearchIndex" {
		return "yomihon search-index"
	}
	return "yomihon search"
}

func TestClassifySearchSemanticError(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		source semanticFailureSource
		want   commandFailure
	}{
		{name: "store absent", err: semantic.ErrStoreNotFound, source: semanticFailureReconcile, want: unavailableFailure(searchReasonCacheCold)},
		{name: "no active generation", err: semantic.ErrNoActiveGeneration, source: semanticFailureReconcile, want: unavailableFailure(searchReasonCacheCold)},
		{name: "store corrupt", err: semantic.ErrStoreCorrupt, source: semanticFailureReconcile, want: unavailableFailure(searchReasonCacheCorrupt)},
		{name: "invalid chunk", err: semantic.ErrInvalidChunk, source: semanticFailureReconcile, want: unavailableFailure(searchReasonCacheCorrupt)},
		{name: "dimension mismatch while loading", err: semantic.ErrDimensionMismatch, source: semanticFailureReconcile, want: unavailableFailure(searchReasonCacheCorrupt)},
		{name: "zero vector while loading", err: semantic.ErrZeroVector, source: semanticFailureReconcile, want: unavailableFailure(searchReasonCacheCorrupt)},
		{name: "duplicate chunk while loading", err: semantic.ErrDuplicateChunk, source: semanticFailureReconcile, want: unavailableFailure(searchReasonCacheCorrupt)},
		{name: "unmeasured active generation", err: semantic.ErrGenerationUnmeasured, source: semanticFailureReconcile, want: unavailableFailure(searchReasonCacheCorrupt)},
		{name: "schema mismatch", err: semantic.ErrStoreSchemaMismatch, source: semanticFailureReconcile, want: unavailableFailure(searchReasonCacheMismatch)},
		{name: "vector format mismatch", err: semantic.ErrVectorFormatMismatch, source: semanticFailureReconcile, want: unavailableFailure(searchReasonCacheMismatch)},
		{name: "identity mismatch", err: semantic.ErrGenerationMismatch, source: semanticFailureReconcile, want: unavailableFailure(searchReasonCacheMismatch)},
		{name: "invalid identity", err: semantic.ErrInvalidIdentity, source: semanticFailureReconcile, want: unavailableFailure(searchReasonCacheMismatch)},
		{name: "retired model", err: semantic.ErrEmbedderRetired, source: semanticFailureReconcile, want: unavailableFailure(searchReasonEmbedderRetired)},
		{name: "capacity", err: semantic.ErrIndexCapacity, source: semanticFailureReconcile, want: unavailableFailure(searchReasonCapacity)},
		{name: "unsupported store platform", err: semantic.ErrStoreUnsupportedPlatform, source: semanticFailureReconcile, want: unavailableFailure(searchReasonUnsupportedPlatform)},
		{name: "key absent", err: semantic.ErrEmbedderUnconfigured, source: semanticFailureReconcile, want: unavailableFailure(searchReasonEmbedderUnconfigured)},
		{name: "rebuild required", err: semantic.ErrRebuildRequired, source: semanticFailureReconcile, want: unavailableFailure(searchReasonRebuildRequired)},
		{name: "writer held", err: semantic.ErrWriterHeld, source: semanticFailureReconcile, want: unavailableFailure(searchReasonIndexRefreshing)},
		{name: "staging handle lost", err: semantic.ErrStagingClosed, source: semanticFailureReconcile, want: unavailableFailure(searchReasonIndexRefreshing)},
		{name: "corpus read", err: semantic.ErrCorpusRead, source: semanticFailureReconcile, want: unavailableFailure(searchReasonIndexIncomplete)},
		{name: "corpus chunking", err: semantic.ErrCorpusChunking, source: semanticFailureReconcile, want: unavailableFailure(searchReasonIndexIncomplete)},
		{name: "invalid corpus", err: semantic.ErrInvalidCorpus, source: semanticFailureReconcile, want: unavailableFailure(searchReasonIndexIncomplete)},
		{name: "generation incomplete", err: semantic.ErrGenerationIncomplete, source: semanticFailureReconcile, want: unavailableFailure(searchReasonIndexIncomplete)},
		{name: "vault changed", err: semantic.ErrVaultChanged, source: semanticFailureReconcile, want: unavailableFailure(searchReasonVaultChanged)},
		{name: "document changed", err: semantic.ErrSourceNoteChanged, source: semanticFailureReconcile, want: unavailableFailure(searchReasonVaultChanged)},
		{name: "document unavailable", err: semantic.ErrSourceNoteUnavailable, source: semanticFailureReconcile, want: unavailableFailure(searchReasonVaultChanged)},
		{name: "document denied", err: semantic.ErrChunkEgressDenied, source: semanticFailureReconcile, want: unavailableFailure(searchReasonVaultChanged)},
		{name: "retry deferred", err: semantic.ErrRetryNotReady, source: semanticFailureReconcile, want: unavailableFailure(searchReasonRateLimited)},
		{name: "attempts exhausted", err: semantic.ErrAttemptLimit, source: semanticFailureReconcile, want: unavailableFailure(searchReasonAttemptBudgetExhausted)},
		{name: "last slot provider terminal", err: errors.Join(semantic.ErrAttemptLimit, &semantic.EmbedError{Kind: semantic.EmbedFailureProvider}), source: semanticFailureReconcile, want: unavailableFailure(searchReasonAttemptBudgetExhausted)},
		{name: "transport", err: &semantic.EmbedError{Kind: semantic.EmbedFailureUnreachable}, source: semanticFailureQuery, want: unavailableFailure(searchReasonEmbedderUnreachable)},
		{name: "throttle", err: &semantic.EmbedError{Kind: semantic.EmbedFailureRateLimited}, source: semanticFailureQuery, want: unavailableFailure(searchReasonRateLimited)},
		{name: "credential", err: &semantic.EmbedError{Kind: semantic.EmbedFailureRejected}, source: semanticFailureQuery, want: unavailableFailure(searchReasonEmbedderRejected)},
		{name: "provider", err: &semantic.EmbedError{Kind: semantic.EmbedFailureProvider}, source: semanticFailureQuery, want: unavailableFailure(searchReasonEmbedderFailed)},
		{name: "unknown provider kind", err: &semantic.EmbedError{Kind: semantic.EmbedFailureKind("unknown")}, source: semanticFailureQuery, want: unavailableFailure(searchReasonEmbedderFailed)},
		{name: "confirmed malformed", err: &semantic.EmbedError{Kind: semantic.EmbedFailureMalformedRequest}, source: semanticFailureQuery, want: commandFailure{kind: commandFailureInternal}},
		{name: "request formation", err: &semantic.EmbedError{Kind: semantic.EmbedFailureInternal}, source: semanticFailureQuery, want: commandFailure{kind: commandFailureInternal}},
		{name: "invalid query capability", err: semantic.ErrInvalidSearchState, source: semanticFailureQuery, want: commandFailure{kind: commandFailureInternal}},
		{name: "query capability reused", err: semantic.ErrQueryAlreadyAttempted, source: semanticFailureQuery, want: commandFailure{kind: commandFailureInternal}},
		{name: "provider applicability disagreement", err: semantic.ErrQueryNotApplicable, source: semanticFailureQuery, want: commandFailure{kind: commandFailureInternal}},
		{name: "invalid snapshot", err: ErrInvalidSnapshot, source: semanticFailureQuery, want: commandFailure{kind: commandFailureInternal}},
		{name: "invalid search", err: ErrInvalidSearch, source: semanticFailureQuery, want: commandFailure{kind: commandFailureInternal}},
		{name: "privacy capability", err: semantic.ErrPrivacyPolicyUnavailable, source: semanticFailureReconcile, want: commandFailure{kind: commandFailureUnanswerable, reason: searchReasonPrivacyUnavailable}},
		{name: "policy source", err: semantic.ErrPolicySourceChanged, source: semanticFailureQuery, want: commandFailure{kind: commandFailureUnanswerable, reason: searchReasonPrivacyUnavailable}},
		{name: "artifact capability", err: semantic.ErrArtifactPolicyUnavailable, source: semanticFailureReconcile, want: unavailableFailure(searchReasonArtifactPolicyUnavailable)},
		{name: "provider configuration", err: semantic.ErrEmbedderConfiguration, source: semanticFailureReconcile, want: commandFailure{kind: commandFailureTool}},
		{name: "store permissions", err: semantic.ErrStorePermissions, source: semanticFailureReconcile, want: commandFailure{kind: commandFailureTool}},
		{name: "cancelled", err: context.Canceled, source: semanticFailureReconcile, want: commandFailure{kind: commandFailureTool}},
		{name: "evidence mismatch", err: ErrEvidenceMismatch, source: semanticFailureQuery, want: commandFailure{kind: commandFailureInternal}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifySearchSemanticError(tt.err, tt.source)
			if diff := cmp.Diff(tt.want, got, cmp.AllowUnexported(commandFailure{})); diff != "" {
				t.Errorf("classifySearchSemanticError() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestClassifySearchIndexError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want commandFailure
	}{
		{name: "provider", err: &semantic.EmbedError{Kind: semantic.EmbedFailureProvider}, want: unavailableFailure(searchReasonEmbedderFailed)},
		{name: "unconfigured", err: semantic.ErrEmbedderUnconfigured, want: unavailableFailure(searchReasonEmbedderUnconfigured)},
		{name: "writer held", err: semantic.ErrWriterHeld, want: unavailableFailure(searchReasonIndexRefreshing)},
		{name: "build closed", err: semantic.ErrStagingClosed, want: unavailableFailure(searchReasonIndexRefreshing)},
		{name: "corpus read", err: semantic.ErrCorpusRead, want: unavailableFailure(searchReasonIndexIncomplete)},
		{name: "corpus incomplete", err: semantic.ErrGenerationIncomplete, want: unavailableFailure(searchReasonIndexIncomplete)},
		{name: "vault changed", err: semantic.ErrVaultChanged, want: unavailableFailure(searchReasonVaultChanged)},
		{name: "document denied", err: semantic.ErrChunkEgressDenied, want: unavailableFailure(searchReasonVaultChanged)},
		{name: "retry deferred", err: semantic.ErrRetryNotReady, want: unavailableFailure(searchReasonRateLimited)},
		{name: "attempts exhausted", err: semantic.ErrAttemptLimit, want: unavailableFailure(searchReasonAttemptBudgetExhausted)},
		{name: "last slot provider terminal", err: errors.Join(semantic.ErrAttemptLimit, &semantic.EmbedError{Kind: semantic.EmbedFailureProvider}), want: unavailableFailure(searchReasonAttemptBudgetExhausted)},
		{name: "attempt budget not renewable", err: semantic.ErrAttemptBudgetNotRenewable, want: unavailableFailure(searchReasonAttemptBudgetNotRenewable)},
		{name: "capacity", err: semantic.ErrIndexCapacity, want: unavailableFailure(searchReasonCapacity)},
		{name: "unsupported store platform", err: semantic.ErrStoreUnsupportedPlatform, want: unavailableFailure(searchReasonUnsupportedPlatform)},
		{name: "local store failure", err: semantic.ErrStorePermissions, want: commandFailure{kind: commandFailureTool}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifySearchIndexError(tt.err)
			if diff := cmp.Diff(tt.want, got, cmp.AllowUnexported(commandFailure{})); diff != "" {
				t.Errorf("classifySearchIndexError() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestRunSearchSemanticFailureShapes(t *testing.T) {
	root := writeSearchTestVault(t, `
[artifacts]
non_instance_dirs = []

[privacy]
never_egress_dirs = ["Private"]
`, map[string]string{
		"Writing/public.md": "---\ntitle: Public\ntype: concept\nstatus: draft\n---\nbody token\n",
	})
	lexicalCold := "{\"mode\":\"lexical\",\"semantic\":\"unavailable\",\"coverage\":{\"reason\":\"cache-cold\"},\"results\":[{\"rank\":1,\"rel_path\":\"Writing/public.md\",\"title\":\"Public\",\"status\":\"draft\",\"snippet\":\"body token\",\"channels\":[\"lexical\"],\"channel_ranks\":{\"lexical\":1}}]}\n"
	lexicalFailed := "{\"mode\":\"lexical\",\"semantic\":\"unavailable\",\"coverage\":{\"reason\":\"embedder-failed\"},\"results\":[{\"rank\":1,\"rel_path\":\"Writing/public.md\",\"title\":\"Public\",\"status\":\"draft\",\"snippet\":\"body token\",\"channels\":[\"lexical\"],\"channel_ranks\":{\"lexical\":1}}]}\n"
	lexicalUnsupported := "{\"mode\":\"lexical\",\"semantic\":\"unavailable\",\"coverage\":{\"reason\":\"unsupported-platform\"},\"results\":[{\"rank\":1,\"rel_path\":\"Writing/public.md\",\"title\":\"Public\",\"status\":\"draft\",\"snippet\":\"body token\",\"channels\":[\"lexical\"],\"channel_ranks\":{\"lexical\":1}}]}\n"
	lexicalExhausted := "{\"mode\":\"lexical\",\"semantic\":\"unavailable\",\"coverage\":{\"reason\":\"attempt-budget-exhausted\"},\"results\":[{\"rank\":1,\"rel_path\":\"Writing/public.md\",\"title\":\"Public\",\"status\":\"draft\",\"snippet\":\"body token\",\"channels\":[\"lexical\"],\"channel_ranks\":{\"lexical\":1}}]}\n"
	tests := []struct {
		name       string
		err        error
		wantExit   int
		wantStdout string
		wantStderr string
	}{
		{
			name:       "answerable unavailable",
			err:        semantic.ErrStoreNotFound,
			wantExit:   3,
			wantStdout: lexicalCold,
			wantStderr: "yomihon search: cache-cold: no semantic index exists; run yomihon search-index build\n",
		},
		{
			name:       "unanswerable privacy",
			err:        semantic.ErrPrivacyPolicyUnavailable,
			wantExit:   3,
			wantStdout: "{\"error\":{\"reason\":\"privacy-capability-unavailable\"}}\n",
			wantStderr: "yomihon search: privacy-capability-unavailable: the vault contract declares no valid privacy policy, so agent-facing search output is closed\n",
		},
		{
			name:       "provider unknown",
			err:        &semantic.EmbedError{Kind: semantic.EmbedFailureKind("future-provider-status")},
			wantExit:   3,
			wantStdout: lexicalFailed,
			wantStderr: "yomihon search: embedder-failed: the embedding API returned an error search could not recover from\n",
		},
		{
			name:       "unsupported store platform",
			err:        semantic.ErrStoreUnsupportedPlatform,
			wantExit:   3,
			wantStdout: lexicalUnsupported,
			wantStderr: "yomihon search: unsupported-platform: the semantic generation store is not supported on this platform\n",
		},
		{
			name:       "document attempt budget exhausted",
			err:        semantic.ErrAttemptLimit,
			wantExit:   3,
			wantStdout: lexicalExhausted,
			wantStderr: "yomihon search: attempt-budget-exhausted: semantic document sends need renewed authorization; run yomihon search-index build --renew-attempt-budget\n",
		},
		{
			name:       "internal malformed",
			err:        &semantic.EmbedError{Kind: semantic.EmbedFailureMalformedRequest},
			wantExit:   1,
			wantStdout: "{\"internal_error\":{\"detail\":\"the request could not be formed correctly\"}}\n",
			wantStderr: "yomihon search: internal: the request could not be formed correctly\n",
		},
		{
			name:       "unknown local tool error is redacted",
			err:        errors.New("sentinel-private-query key-sentinel"),
			wantExit:   2,
			wantStderr: "yomihon search: semantic action failed\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			exit := runSearch(t.Context(), []string{"--json", "--semantic", "token"}, rootConfig{rootEnv: root}, searchDeps{
				stdout: &stdout,
				stderr: &stderr,
				openSemantic: func(semanticActionConfig) (reconcileSearch, error) {
					return func(context.Context, semantic.Corpus) (SemanticSearch, error) {
						return nil, tt.err
					}, nil
				},
			})
			if exit != tt.wantExit {
				t.Errorf("runSearch() exit = %d, want %d", exit, tt.wantExit)
			}
			if got := stdout.String(); got != tt.wantStdout {
				t.Errorf("runSearch() stdout = %q, want %q", got, tt.wantStdout)
			}
			if got := stderr.String(); got != tt.wantStderr {
				t.Errorf("runSearch() stderr = %q, want %q", got, tt.wantStderr)
			}
		})
	}
}

func TestRunSearchNilReadyIsLocalToolFailure(t *testing.T) {
	root := writeSearchTestVault(t, `
[artifacts]
non_instance_dirs = []

[privacy]
never_egress_dirs = ["Private"]
`, map[string]string{
		"Writing/public.md": "---\ntitle: Public\ntype: concept\nstatus: draft\n---\nbody token\n",
	})
	var stdout, stderr bytes.Buffer
	exit := runSearch(t.Context(), []string{"--json", "--semantic", "token"}, rootConfig{rootEnv: root}, searchDeps{
		stdout: &stdout,
		stderr: &stderr,
		openSemantic: func(semanticActionConfig) (reconcileSearch, error) {
			return func(context.Context, semantic.Corpus) (SemanticSearch, error) {
				return nil, nil //nolint:nilnil // malicious fixture proves a nil capability cannot become a successful action
			}, nil
		},
	})
	if exit != 2 {
		t.Errorf("runSearch() exit = %d, want 2", exit)
	}
	if stdout.Len() != 0 {
		t.Errorf("runSearch() stdout = %q, want empty", stdout.Bytes())
	}
	want := "yomihon search: semantic action failed\n"
	if got := stderr.String(); got != want {
		t.Errorf("runSearch() stderr = %q, want %q", got, want)
	}
}

func TestRunSearchIndexSuccess(t *testing.T) {
	root := writeSearchTestVault(t, `
[artifacts]
non_instance_dirs = []

[privacy]
never_egress_dirs = ["Private"]
`, nil)
	tests := []struct {
		name       string
		args       []string
		result     semantic.BuildReport
		wantStdout string
	}{
		{
			name:       "current JSON",
			args:       []string{"build", "--json"},
			result:     semantic.BuildReport{Status: semantic.BuildCurrent, Chunks: 8, Reused: 8, TopKP95: 241 * time.Microsecond},
			wantStdout: "{\"status\":\"current\",\"chunks\":8,\"embedded\":0,\"reused\":8,\"top_k_p95_us\":241}\n",
		},
		{
			name:       "built human",
			args:       []string{"build"},
			result:     semantic.BuildReport{Status: semantic.BuildPublished, Chunks: 8, Embedded: 3, Reused: 5, TopKP95: 317 * time.Microsecond},
			wantStdout: "semantic index built: 8 chunks (3 embedded, 5 reused)\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			exit := runSearchIndex(t.Context(), tt.args, rootConfig{rootEnv: root}, buildCommandDeps{
				stdout: &stdout,
				stderr: &stderr,
				openBuilder: func(semanticActionConfig) (generationBuilder, error) {
					return fakeGenerationBuilder{run: func(context.Context) (semantic.BuildReport, error) {
							return tt.result, nil
						}},

						nil
				},
			})
			if exit != 0 {
				t.Errorf("runSearchIndex() exit = %d, want 0", exit)
			}
			if got := stdout.String(); got != tt.wantStdout {
				t.Errorf("runSearchIndex() stdout = %q, want %q", got, tt.wantStdout)
			}
			if stderr.Len() != 0 {
				t.Errorf("runSearchIndex() stderr = %q, want empty", stderr.Bytes())
			}
		})
	}
}

func TestRunSearchIndexPassesExplicitAttemptBudgetRenewal(t *testing.T) {
	root := writeSearchTestVault(t, `
[artifacts]
non_instance_dirs = []

[privacy]
never_egress_dirs = ["Private"]
`, nil)
	builder := &recordingGenerationBuilder{report: semantic.BuildReport{
		Status:  semantic.BuildCurrent,
		Chunks:  1,
		Reused:  1,
		TopKP95: time.Microsecond,
	}}
	var stdout, stderr bytes.Buffer
	exit := runSearchIndex(t.Context(), []string{
		"build",
		"--renew-attempt-budget",
		"--renew-attempt-budget",
	}, rootConfig{rootEnv: root}, buildCommandDeps{
		stdout: &stdout,
		stderr: &stderr,
		openBuilder: func(semanticActionConfig) (generationBuilder, error) {
			return builder, nil
		},
	})
	if exit != 0 {
		t.Fatalf("runSearchIndex() exit = %d, want 0; stderr = %q", exit, stderr.Bytes())
	}
	if !builder.request.RenewAttemptBudget {
		t.Error("BuildRequest.RenewAttemptBudget = false, want true")
	}
}

func TestRunSearchIndexProgressIsHumanOnlyAndByteExact(t *testing.T) {
	root := writeSearchTestVault(t, `
[artifacts]
non_instance_dirs = []

[privacy]
never_egress_dirs = ["Private"]
`, nil)
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{name: "human", args: []string{"build"}, wantStderr: "yomihon search-index: embedded 100/205 chunks\nyomihon search-index: embedded 205/205 chunks\n"},
		{name: "JSON", args: []string{"build", "--json"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			exit := runSearchIndex(t.Context(), tt.args, rootConfig{rootEnv: root}, buildCommandDeps{
				stdout: &stdout,
				stderr: &stderr,
				openBuilder: func(action semanticActionConfig) (generationBuilder, error) {
					return fakeGenerationBuilder{run: func(context.Context) (semantic.BuildReport, error) {
						if action.buildProgress != nil {
							if err := action.buildProgress(100, 205); err != nil {
								return semantic.BuildReport{}, err
							}
							if err := action.buildProgress(205, 205); err != nil {
								return semantic.BuildReport{}, err
							}
						}
						return semantic.BuildReport{Status: semantic.BuildPublished, Chunks: 205, Embedded: 205, TopKP95: time.Microsecond}, nil
					}}, nil
				},
			})
			if exit != 0 {
				t.Fatalf("runSearchIndex() exit = %d, want 0", exit)
			}
			if got := stderr.String(); got != tt.wantStderr {
				t.Errorf("runSearchIndex() stderr = %q, want %q", got, tt.wantStderr)
			}
		})
	}
}

func TestRunSearchIndexRejectsInvalidBuildReport(t *testing.T) {
	root := writeSearchTestVault(t, `
[artifacts]
non_instance_dirs = []

[privacy]
never_egress_dirs = ["Private"]
`, nil)
	var stdout, stderr bytes.Buffer
	exit := runSearchIndex(t.Context(), []string{"build", "--json"}, rootConfig{rootEnv: root}, buildCommandDeps{
		stdout: &stdout,
		stderr: &stderr,
		openBuilder: func(semanticActionConfig) (generationBuilder, error) {
			return fakeGenerationBuilder{run: func(context.Context) (semantic.BuildReport, error) {
					return semantic.BuildReport{Status: semantic.BuildStatus("future-status")}, nil
				}},

				nil
		},
	})
	if exit != 2 {
		t.Errorf("runSearchIndex() exit = %d, want 2", exit)
	}
	if stdout.Len() != 0 {
		t.Errorf("runSearchIndex() stdout = %q, want empty", stdout.Bytes())
	}
	want := "yomihon search-index: semantic build returned an invalid result\n"
	if got := stderr.String(); got != want {
		t.Errorf("runSearchIndex() stderr = %q, want %q", got, want)
	}
}

func TestRunSearchIndexRejectsUnmeasuredBuildReport(t *testing.T) {
	root := writeSearchTestVault(t, `
[artifacts]
non_instance_dirs = []

[privacy]
never_egress_dirs = ["Private"]
`, nil)
	for _, topKP95 := range []time.Duration{0, time.Nanosecond} {
		var stdout, stderr bytes.Buffer
		exit := runSearchIndex(t.Context(), []string{"build", "--json"}, rootConfig{rootEnv: root}, buildCommandDeps{
			stdout: &stdout,
			stderr: &stderr,
			openBuilder: func(semanticActionConfig) (generationBuilder, error) {
				return fakeGenerationBuilder{run: func(context.Context) (semantic.BuildReport, error) {
						return semantic.BuildReport{Status: semantic.BuildCurrent, TopKP95: topKP95}, nil
					}},

					nil
			},
		})
		if exit != 2 || stdout.Len() != 0 {
			t.Errorf("TopKP95 %s: runSearchIndex() = exit %d, stdout %q", topKP95, exit, stdout.Bytes())
		}
		want := "yomihon search-index: semantic build returned an invalid result\n"
		if got := stderr.String(); got != want {
			t.Errorf("TopKP95 %s: stderr = %q, want %q", topKP95, got, want)
		}
	}
}

func TestRunSearchIndexCapabilityGatesBeforeBuilder(t *testing.T) {
	tests := []struct {
		name       string
		sections   string
		wantStdout string
		wantStderr string
	}{
		{
			name: "privacy",
			sections: `
[artifacts]
non_instance_dirs = []
`,
			wantStdout: "{\"error\":{\"reason\":\"privacy-capability-unavailable\",\"active_generation\":\"not-inspected\",\"staging_generation\":\"not-inspected\",\"retry_safe\":false,\"next_action\":\"repair-vault-contract\"}}\n",
			wantStderr: "yomihon search-index: privacy-capability-unavailable: the vault contract declares no valid privacy policy, so agent-facing search output is closed\n",
		},
		{
			name: "artifact",
			sections: `
[privacy]
never_egress_dirs = ["Private"]
`,
			wantStdout: "{\"error\":{\"reason\":\"artifact-policy-unavailable\",\"active_generation\":\"not-inspected\",\"staging_generation\":\"not-inspected\",\"retry_safe\":false,\"next_action\":\"repair-vault-contract\"}}\n",
			wantStderr: "yomihon search-index: artifact-policy-unavailable: the vault contract declares no valid artifact policy, so the semantic corpus cannot exist\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := writeSearchTestVault(t, tt.sections, nil)
			var stdout, stderr bytes.Buffer
			opened := false
			exit := runSearchIndex(t.Context(), []string{"build", "--json"}, rootConfig{rootEnv: root}, buildCommandDeps{
				stdout: &stdout,
				stderr: &stderr,
				openBuilder: func(semanticActionConfig) (generationBuilder, error) {
					opened = true
					return nil, errors.New("must not open")
				},
			})
			if exit != 3 {
				t.Errorf("runSearchIndex() exit = %d, want 3", exit)
			}
			if opened {
				t.Error("runSearchIndex() opened semantic builder before capability gate")
			}
			if got := stdout.String(); got != tt.wantStdout {
				t.Errorf("runSearchIndex() stdout = %q, want %q", got, tt.wantStdout)
			}
			if got := stderr.String(); got != tt.wantStderr {
				t.Errorf("runSearchIndex() stderr = %q, want %q", got, tt.wantStderr)
			}
		})
	}
}

func TestRunSearchIndexRechecksPrivacyBeforeSuccessOutput(t *testing.T) {
	root := writeSearchTestVault(t, `
[artifacts]
non_instance_dirs = []

[privacy]
never_egress_dirs = ["Private"]
`, nil)
	var stdout, stderr bytes.Buffer
	exit := runSearchIndex(t.Context(), []string{"build", "--json"}, rootConfig{rootEnv: root}, buildCommandDeps{
		stdout: &stdout,
		stderr: &stderr,
		openBuilder: func(semanticActionConfig) (generationBuilder, error) {
			return fakeGenerationBuilder{run: func(context.Context) (semantic.BuildReport, error) {
					return semantic.BuildReport{Status: semantic.BuildCurrent}, nil
				}},

				nil
		},
		beforeCapabilityRecheck: func() {
			writeSearchTestFile(t, root, "System/schemas/vault-schema.toml", searchTestContractBase+`
[artifacts]
non_instance_dirs = []
`)
		},
	})
	if exit != 3 {
		t.Errorf("runSearchIndex() exit = %d, want 3", exit)
	}
	wantStdout := "{\"error\":{\"reason\":\"privacy-capability-unavailable\",\"active_generation\":\"preserved-unusable\",\"staging_generation\":\"not-inspected\",\"retry_safe\":false,\"next_action\":\"repair-vault-contract\"}}\n"
	if got := stdout.String(); got != wantStdout {
		t.Errorf("runSearchIndex() stdout = %q, want %q", got, wantStdout)
	}
	wantStderr := "yomihon search-index: privacy-capability-unavailable: the vault contract declares no valid privacy policy, so agent-facing search output is closed\n"
	if got := stderr.String(); got != wantStderr {
		t.Errorf("runSearchIndex() stderr = %q, want %q", got, wantStderr)
	}
}

func TestRunSearchIndexRechecksPolicyAfterPublishedBuild(t *testing.T) {
	root := writeSearchTestVault(t, `
[artifacts]
non_instance_dirs = []

[privacy]
never_egress_dirs = ["Private"]
`, nil)
	var stdout, stderr bytes.Buffer
	exit := runSearchIndex(t.Context(), []string{"build", "--json"}, rootConfig{rootEnv: root}, buildCommandDeps{
		stdout: &stdout,
		stderr: &stderr,
		openBuilder: func(semanticActionConfig) (generationBuilder, error) {
			return fakeGenerationBuilder{run: func(context.Context) (semantic.BuildReport, error) {
				return semantic.BuildReport{Status: semantic.BuildPublished}, nil
			}}, nil
		},
		beforeCapabilityRecheck: func() {
			writeSearchTestFile(t, root, "System/schemas/vault-schema.toml", searchTestContractBase+`
[privacy]
never_egress_dirs = ["Private"]
`)
		},
	})
	if exit != 3 {
		t.Errorf("runSearchIndex() exit = %d, want 3", exit)
	}
	wantStdout := "{\"error\":{\"reason\":\"privacy-capability-unavailable\",\"active_generation\":\"preserved-unusable\",\"staging_generation\":\"absent\",\"retry_safe\":false,\"next_action\":\"repair-vault-contract\"}}\n"
	if got := stdout.String(); got != wantStdout {
		t.Errorf("runSearchIndex() stdout = %q, want %q", got, wantStdout)
	}
	wantStderr := "yomihon search-index: privacy-capability-unavailable: the vault contract declares no valid privacy policy, so agent-facing search output is closed\n"
	if got := stderr.String(); got != wantStderr {
		t.Errorf("runSearchIndex() stderr = %q, want %q", got, wantStderr)
	}
}

func TestRunSearchIndexFailureShapes(t *testing.T) {
	root := writeSearchTestVault(t, `
[artifacts]
non_instance_dirs = []

[privacy]
never_egress_dirs = ["Private"]
`, nil)
	tests := []struct {
		name       string
		args       []string
		err        error
		wantExit   int
		wantStdout string
		wantStderr string
	}{
		{
			name:       "writer held JSON",
			args:       []string{"build", "--json"},
			err:        semantic.ErrWriterHeld,
			wantExit:   3,
			wantStdout: "{\"error\":{\"reason\":\"index-refreshing\",\"active_generation\":\"not-inspected\",\"staging_generation\":\"not-inspected\",\"retry_safe\":false,\"next_action\":\"wait-and-retry\"}}\n",
			wantStderr: "yomihon search-index: index-refreshing: another process is updating the semantic index\n",
		},
		{
			name:       "provider unknown human",
			args:       []string{"build"},
			err:        &semantic.EmbedError{Kind: semantic.EmbedFailureKind("future-provider-status")},
			wantExit:   3,
			wantStderr: "yomihon search-index: embedder-failed: the embedding API returned an error search could not recover from\n",
		},
		{
			name:       "capacity JSON",
			args:       []string{"build", "--json"},
			err:        semantic.ErrIndexCapacity,
			wantExit:   3,
			wantStdout: "{\"error\":{\"reason\":\"capacity\",\"active_generation\":\"not-inspected\",\"staging_generation\":\"not-inspected\",\"retry_safe\":false,\"next_action\":\"review-capacity\"}}\n",
			wantStderr: "yomihon search-index: capacity: the semantic index could not be loaded into memory\n",
		},
		{
			name:       "unsupported store platform JSON",
			args:       []string{"build", "--json"},
			err:        semantic.ErrStoreUnsupportedPlatform,
			wantExit:   3,
			wantStdout: "{\"error\":{\"reason\":\"unsupported-platform\",\"active_generation\":\"not-inspected\",\"staging_generation\":\"not-inspected\",\"retry_safe\":false,\"next_action\":\"use-supported-platform\"}}\n",
			wantStderr: "yomihon search-index: unsupported-platform: the semantic generation store is not supported on this platform\n",
		},
		{
			name:       "internal malformed JSON",
			args:       []string{"build", "--json"},
			err:        &semantic.EmbedError{Kind: semantic.EmbedFailureMalformedRequest},
			wantExit:   1,
			wantStdout: "{\"internal_error\":{\"detail\":\"the request could not be formed correctly\",\"active_generation\":\"not-inspected\",\"staging_generation\":\"not-inspected\",\"retry_safe\":false,\"next_action\":\"repair-yomihon\"}}\n",
			wantStderr: "yomihon search-index: internal: the request could not be formed correctly\n",
		},
		{
			name:       "attempt budget exhausted JSON",
			args:       []string{"build", "--json"},
			err:        exhaustedBuildFailure(),
			wantExit:   3,
			wantStdout: readWireGolden(t, "index-attempt-budget-exhausted.jsonl"),
			wantStderr: readWireGolden(t, "index-attempt-budget-exhausted.stderr"),
		},
		{
			name:       "attempt budget not renewable JSON",
			args:       []string{"build", "--json", "--renew-attempt-budget"},
			err:        semantic.ErrAttemptBudgetNotRenewable,
			wantExit:   3,
			wantStdout: "{\"error\":{\"reason\":\"attempt-budget-not-renewable\",\"active_generation\":\"not-inspected\",\"staging_generation\":\"not-inspected\",\"retry_safe\":false,\"next_action\":\"retry-build\"}}\n",
			wantStderr: readWireGolden(t, "index-attempt-budget-not-renewable.stderr"),
		},
		{
			name:       "unknown local error redacted",
			args:       []string{"build", "--json"},
			err:        errors.New("sentinel-private-query key-sentinel"),
			wantExit:   2,
			wantStderr: "yomihon search-index: semantic build failed\n",
		},
		{
			name:       "SQLite or filesystem failure stays local",
			args:       []string{"build", "--json"},
			err:        semantic.ErrStorePermissions,
			wantExit:   2,
			wantStderr: "yomihon search-index: semantic build failed\n",
		},
		{
			name:       "interruption stays local",
			args:       []string{"build", "--json"},
			err:        context.Canceled,
			wantExit:   2,
			wantStderr: "yomihon search-index: semantic build failed\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			exit := runSearchIndex(t.Context(), tt.args, rootConfig{rootEnv: root}, buildCommandDeps{
				stdout: &stdout,
				stderr: &stderr,
				openBuilder: func(semanticActionConfig) (generationBuilder, error) {
					return fakeGenerationBuilder{run: func(context.Context) (semantic.BuildReport, error) {
							return semantic.BuildReport{}, tt.err
						}},

						nil
				},
			})
			if exit != tt.wantExit {
				t.Errorf("runSearchIndex() exit = %d, want %d", exit, tt.wantExit)
			}
			if got := stdout.String(); got != tt.wantStdout {
				t.Errorf("runSearchIndex() stdout = %q, want %q", got, tt.wantStdout)
			}
			if got := stderr.String(); got != tt.wantStderr {
				t.Errorf("runSearchIndex() stderr = %q, want %q", got, tt.wantStderr)
			}
		})
	}
}

func TestRunSearchIndexPartialRecoveryOutputIsToolFailure(t *testing.T) {
	root := writeSearchTestVault(t, `
[artifacts]
non_instance_dirs = []

[privacy]
never_egress_dirs = ["Private"]
`, nil)
	var stdout partialWriter
	var stderr bytes.Buffer
	exit := runSearchIndex(t.Context(), []string{"build", "--json"}, rootConfig{rootEnv: root}, buildCommandDeps{
		stdout: &stdout,
		stderr: &stderr,
		openBuilder: func(semanticActionConfig) (generationBuilder, error) {
			return fakeGenerationBuilder{run: func(context.Context) (semantic.BuildReport, error) {
				return semantic.BuildReport{}, exhaustedBuildFailure()
			}}, nil
		},
	})
	if exit != 2 {
		t.Errorf("runSearchIndex() exit = %d, want 2", exit)
	}
	if stdout.Len() == 0 || stdout.Bytes()[stdout.Len()-1] == '\n' {
		t.Errorf("runSearchIndex() partial stdout = %q, want nonempty incomplete frame", stdout.Bytes())
	}
	if got, want := stderr.String(), "yomihon search-index: write output\n"; got != want {
		t.Errorf("runSearchIndex() stderr = %q, want %q", got, want)
	}
}

type fakeSemanticSearch struct {
	corpus semantic.Corpus
}

type partialWriter struct {
	bytes.Buffer
}

func (w *partialWriter) Write(p []byte) (int, error) {
	written := len(p) / 2
	_, _ = w.Buffer.Write(p[:written])
	return written, nil
}

func newFakeSemanticSearch(corpus semantic.Corpus) *fakeSemanticSearch {
	return &fakeSemanticSearch{corpus: corpus}
}

func (f *fakeSemanticSearch) CorpusFingerprint() [sha256.Size]byte {
	return f.corpus.Fingerprint
}

func (f *fakeSemanticSearch) Search(
	_ context.Context,
	_ string,
	allowed map[string]struct{},
	_ int,
) ([]semantic.Rank, error) {
	if len(f.corpus.Chunks) == 0 {
		return nil, nil
	}
	document := f.corpus.Chunks[0]
	if _, ok := allowed[document.RelPath]; !ok {
		return nil, nil
	}
	return []semantic.Rank{{RelPath: document.RelPath, ChunkOrdinal: document.Ordinal, Score: 1}}, nil
}

func writeSearchTestVault(t *testing.T, capabilitySections string, notes map[string]string) string {
	t.Helper()
	root := t.TempDir()
	writeSearchTestFile(t, root, "System/schemas/vault-schema.toml", searchTestContractBase+capabilitySections)
	for relPath, body := range notes {
		writeSearchTestFile(t, root, relPath, body)
	}
	return root
}

func writeSearchTestFile(t *testing.T, root, relPath, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("MkdirAll(%q) error: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil { // #nosec G703 -- path is rooted in t.TempDir
		t.Fatalf("WriteFile(%q) error: %v", path, err)
	}
}
