package archlock

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"go.yaml.in/yaml/v3"
)

type mutationWorkflowJob struct {
	Name            string `yaml:"name"`
	ContinueOnError any    `yaml:"continue-on-error"`
	Strategy        struct {
		FailFast    *bool            `yaml:"fail-fast"`
		Matrix      map[string][]any `yaml:"matrix"`
		MaxParallel *int             `yaml:"max-parallel"`
	} `yaml:"strategy"`
	Steps []struct {
		Run             string `yaml:"run"`
		If              string `yaml:"if"`
		ContinueOnError any    `yaml:"continue-on-error"`
	} `yaml:"steps"`
}

func TestMutationShardWorkflow(t *testing.T) {
	var workflow struct {
		Jobs map[string]mutationWorkflowJob `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(mutationArtifact(t, ".github/workflows/ci.yml"), &workflow); err != nil {
		t.Fatalf("decode actual workflow: %v", err)
	}
	var contract struct {
		Prerequisites []string `json:"verify_prerequisites"`
		Jobs          []struct {
			Name     string   `json:"name"`
			Matrix   []string `json:"matrix"`
			Owns     []string `json:"owns"`
			Advisory bool     `json:"advisory"`
		} `json:"ci_jobs"`
	}
	if err := json.Unmarshal(mutationArtifact(t, ".github/gate-contract.json"), &contract); err != nil {
		t.Fatalf("decode contract: %v", err)
	}
	var ruleset struct {
		Rules []struct {
			Type       string `json:"type"`
			Parameters struct {
				Checks []struct {
					Context       string `json:"context"`
					IntegrationID int    `json:"integration_id"`
				} `json:"required_status_checks"`
			} `json:"parameters"`
		} `json:"rules"`
	}
	if err := json.Unmarshal(mutationArtifact(t, ".github/rulesets/main.json"), &ruleset); err != nil {
		t.Fatalf("decode tracked ruleset: %v", err)
	}
	t.Log("invoked: actual mutation workflow and complete context lock")
	job, ok := workflow.Jobs["e2e-mutations"]
	if !ok {
		t.Fatal("caught: no actual mutation job")
	}
	wantMatrix := map[string][]any{"shard": {1, 2, 3, 4}}
	if diff := cmp.Diff(wantMatrix, job.Strategy.Matrix); diff != "" {
		t.Errorf("caught: actual four-shard matrix (-want +got):\n%s", diff)
	}
	if job.Strategy.FailFast == nil || *job.Strategy.FailFast || job.Strategy.MaxParallel != nil || job.ContinueOnError != nil {
		t.Error("caught: mutation siblings can be cancelled or excused")
	}
	if job.Name != "e2e-mutations (${{ matrix.shard }})" {
		t.Errorf("caught: actual shard context name=%q", job.Name)
	}
	var commands []string
	for _, step := range job.Steps {
		if strings.Contains(step.Run, "mutation-check") {
			commands = append(commands, step.Run)
			if step.If != "" || step.ContinueOnError != nil {
				t.Error("caught: mutation step is conditional or excused")
			}
		}
	}
	if diff := cmp.Diff([]string{"make mutation-check MUTATION_SHARD=${{ matrix.shard }}"}, commands); diff != "" {
		t.Errorf("caught: actual matrix selector forwarding (-want +got):\n%s", diff)
	}
	actualJobs := []string{}
	for name := range workflow.Jobs {
		actualJobs = append(actualJobs, name)
	}
	declaredJobs := []string{}
	actualContexts := []string{}
	declaredContexts := []string{}
	owners := []string{}
	for _, decl := range contract.Jobs {
		declaredJobs = append(declaredJobs, decl.Name)
		for _, prerequisite := range decl.Owns {
			if prerequisite == "mutation-check" {
				owners = append(owners, decl.Name)
			}
		}
		if decl.Name == "e2e-mutations" {
			if diff := cmp.Diff([]string{"1", "2", "3", "4"}, decl.Matrix); diff != "" {
				t.Errorf("caught: declared shard contexts (-want +got):\n%s", diff)
			}
		}
		if decl.Advisory {
			continue
		}
		if len(decl.Matrix) == 0 {
			declaredContexts = append(declaredContexts, decl.Name)
		} else {
			for _, value := range decl.Matrix {
				declaredContexts = append(declaredContexts, decl.Name+" ("+value+")")
			}
		}
		actual, exists := workflow.Jobs[decl.Name]
		if !exists {
			t.Errorf("caught: declaration names missing actual job %q", decl.Name)
			continue
		}
		if len(actual.Strategy.Matrix) == 0 {
			actualContexts = append(actualContexts, decl.Name)
			continue
		}
		for axis, values := range actual.Strategy.Matrix {
			for _, value := range values {
				text := mutationMatrixValue(t, value)
				name := actual.Name
				if name == "" {
					name = decl.Name + " (" + text + ")"
				} else {
					name = strings.ReplaceAll(name, "${{ matrix."+axis+" }}", text)
				}
				actualContexts = append(actualContexts, name)
			}
		}
	}
	slices.Sort(actualJobs)
	slices.Sort(declaredJobs)
	if diff := cmp.Diff(actualJobs, declaredJobs); diff != "" {
		t.Errorf("caught: complete job multiplicity (-actual +declared):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"e2e-mutations"}, owners); diff != "" {
		t.Errorf("caught: mutation prerequisite owner multiplicity: %s", diff)
	}
	if countMutationName(contract.Prerequisites, "mutation-check") != 1 {
		t.Error("caught: full gate mutation prerequisite missing or repeated")
	}
	required := []string{}
	requiredRules := 0
	for _, rule := range ruleset.Rules {
		if rule.Type != "required_status_checks" {
			continue
		}
		requiredRules++
		for _, check := range rule.Parameters.Checks {
			required = append(required, check.Context)
			if check.IntegrationID != 15368 {
				t.Errorf("caught: required context integration changed: %q", check.Context)
			}
		}
	}
	if requiredRules != 1 {
		t.Errorf("caught: required-status rule multiplicity=%d", requiredRules)
	}
	slices.Sort(actualContexts)
	slices.Sort(declaredContexts)
	slices.Sort(required)
	if diff := cmp.Diff(actualContexts, declaredContexts); diff != "" {
		t.Errorf("caught: actual/declared complete contexts (-actual +declared):\n%s", diff)
	}
	if diff := cmp.Diff(actualContexts, required); diff != "" {
		t.Errorf("caught: actual/required complete contexts (-actual +required):\n%s", diff)
	}
	if countMutationName(required, "commit-attribution") != 0 {
		t.Error("caught: advisory attribution became required")
	}
	t.Logf("invoked: actual workflow, Make and complete required contexts %d", len(actualContexts))
}

func mutationMatrixValue(t *testing.T, value any) string {
	t.Helper()
	switch v := value.(type) {
	case string:
		return v
	case int:
		return strconv.Itoa(v)
	default:
		t.Fatalf("unexpected matrix value %T", value)
		return ""
	}
}

func countMutationName(names []string, want string) int {
	count := 0
	for _, name := range names {
		if name == want {
			count++
		}
	}
	return count
}

func mutationArtifact(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(path))) // #nosec G304 -- fixed repository artifact names
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// This drives the copied Make recipe with process-boundary doubles. The actual
// recipe owns selection, quoting, build failure and the serve exit status.
func TestMutationShardMake(t *testing.T) {
	cases := []struct {
		name, selector                   string
		buildStatus, serveStatus, status int
		fullGate                         bool
		args                             []string
	}{
		{name: "full", args: []string{"--mutate"}},
		{name: "full gate ignores selector", selector: "3", fullGate: true, args: []string{"--mutate"}},
		{name: "selected", selector: "3", args: []string{"--mutate", "--shard", "3"}},
		{name: "metacharacters stay one argument", selector: "3;touch forbidden", args: []string{"--mutate", "--shard", "3;touch forbidden"}},
		{name: "build failure", selector: "2", buildStatus: 7, status: 2},
		{name: "serve failure", selector: "4", serveStatus: 9, status: 2, args: []string{"--mutate", "--shard", "4"}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			bin := filepath.Join(dir, "bin")
			serve := filepath.Join(dir, ".github", "e2e")
			for _, path := range []string{bin, serve} {
				if err := os.MkdirAll(path, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			writeProbeFixture(t, filepath.Join(dir, "Makefile"), mutationArtifact(t, "Makefile"), 0o600)
			trace := filepath.Join(dir, "trace")
			writeProbeFixture(t, trace, nil, 0o600)
			writeProbeFixture(t, filepath.Join(bin, "go"), []byte("#!/bin/sh\nprintf 'build\\n' >> \"$MUTATION_MAKE_TRACE\"\nexit \"$MUTATION_BUILD_STATUS\"\n"), 0o700)
			writeProbeFixture(t, filepath.Join(serve, "serve.sh"), []byte("#!/bin/sh\nshift 3\nprintf '%s\\n' \"$@\" >> \"$MUTATION_MAKE_TRACE\"\nexit \"$MUTATION_SERVE_STATUS\"\n"), 0o700)
			args := []string{"--no-print-directory", "-o", "frontend-deps"}
			goal := "mutation-check"
			if tt.fullGate {
				goal = "verify"
				args = append(args, mutationVerifySkips(t)...)
			}
			args = append(args, goal, "MUTATION_SHARD="+tt.selector)
			cmd := exec.CommandContext(t.Context(), "make", args...) // #nosec G204 -- fixed make target and test-controlled selector in an isolated copy
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "MUTATION_MAKE_TRACE="+trace, "MUTATION_BUILD_STATUS="+strconv.Itoa(tt.buildStatus), "MUTATION_SERVE_STATUS="+strconv.Itoa(tt.serveStatus))
			output, err := cmd.CombinedOutput()
			status := 0
			if err != nil {
				if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
					status = exitErr.ExitCode()
				} else {
					t.Fatal(err)
				}
			}
			data, readErr := os.ReadFile(trace) // #nosec G304 -- child receipt in a test-owned temporary directory
			if readErr != nil {
				t.Fatal(readErr)
			}
			got := strings.Split(strings.TrimSpace(string(data)), "\n")
			want := []string{"build"}
			if tt.buildStatus == 0 {
				want = append(want, "bash", ".github/e2e/probes.sh")
				want = append(want, tt.args...)
			}
			if status != tt.status {
				t.Errorf("caught: Make status=%d, want %d; %s", status, tt.status, output)
			}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("caught: Make selector/build/serve trace (-want +got):\n%s; %s", diff, output)
			}
			if _, err := os.Stat(filepath.Join(dir, "forbidden")); !os.IsNotExist(err) {
				t.Errorf("caught: selector executed shell input: %v", err)
			}
			t.Logf("invoked: actual copied Make mutation recipe %s status=%d", tt.name, status)
		})
	}
}

func mutationVerifySkips(t *testing.T) []string {
	t.Helper()
	for line := range strings.SplitSeq(string(mutationArtifact(t, "Makefile")), "\n") {
		if !strings.HasPrefix(line, "verify:") {
			continue
		}
		var args []string
		for prerequisite := range strings.FieldsSeq(strings.TrimPrefix(line, "verify:")) {
			if prerequisite != "mutation-check" {
				args = append(args, "-o", prerequisite)
			}
		}
		return args
	}
	t.Fatal("copied Makefile has no complete verify target")
	return nil
}
