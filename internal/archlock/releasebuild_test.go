package archlock

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// releaseWorkflow is the workflow that builds the binaries a reader downloads.
const releaseWorkflow = ".github/workflows/release.yml"

// TestTheReleaseBuildTrimsPathsAndStripsSymbols holds the published binaries to
// the two flags that make them what a reader can check and carry.
//
// Without -trimpath every binary records the absolute directory it was built in,
// so two clones of one commit in different directories produce different
// SHA-256 values, and nobody can rebuild a release to the digest that was
// published. Without -ldflags="-s -w" the binary carries the symbol table and
// DWARF data, about a quarter of its size, which a reader never uses; a panic
// still prints its file and line and `go version -m` still reads the build
// information without them.
//
// The check reads every `go build` the workflow runs and asks each for both
// flags, rather than looking for the one line that ships today: a second build
// added to the workflow without them is the same mistake. A workflow that runs
// no `go build` fails too, because then the check would be reading nothing and
// would pass whatever the release did.
func TestTheReleaseBuildTrimsPathsAndStripsSymbols(t *testing.T) {
	t.Parallel()

	builds := releaseBuilds(t)
	if len(builds) == 0 {
		t.Fatalf("%s runs no `go build`, so there is no release build for this check to read; if the build moved, move this check with it", releaseWorkflow)
	}
	for _, build := range builds {
		if missing := missingReleaseFlags(build.args); len(missing) > 0 {
			t.Errorf("%s: the `go build` in %s lacks %s (-trimpath keeps the runner's checkout path out of the binary; -s -w drops symbol data nobody reads)\n\tbuild: go build %q",
				releaseWorkflow, build.where, strings.Join(missing, " and "), build.args)
		}
	}
}

// TestTheReleaseBuildCheckReadsTheCommandNotItsSpelling pins how the check above
// reads a script, so a build that is spelled differently is neither missed nor
// mistaken for one that carries the flags.
func TestTheReleaseBuildCheckReadsTheCommandNotItsSpelling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		script      string
		wantBuilds  int
		wantMissing []string
	}{
		{
			name:       "both flags on the line",
			script:     `CGO_ENABLED=0 GOOS="$goos" go build -trimpath -ldflags="-s -w" -o "$out" ./cmd/yomihon`,
			wantBuilds: 1,
		},
		{
			name:       "the flags in either order, the ldflags value as a separate word",
			script:     `go build -ldflags '-w -s' -trimpath -o out ./cmd/yomihon`,
			wantBuilds: 1,
		},
		{
			name:       "more linker flags beside the two",
			script:     `go build -trimpath -ldflags="-s -w -X main.version=v1" ./cmd/yomihon`,
			wantBuilds: 1,
		},
		{
			name:       "the flags on a continued line",
			script:     "go build \\\n  -trimpath \\\n  -ldflags=\"-s -w\" \\\n  ./cmd/yomihon",
			wantBuilds: 1,
		},
		{
			name:        "no flags",
			script:      `go build -o "$out" ./cmd/yomihon`,
			wantBuilds:  1,
			wantMissing: []string{"-trimpath", `-ldflags="-s -w"`},
		},
		{
			name:        "only -trimpath",
			script:      `go build -trimpath -o "$out" ./cmd/yomihon`,
			wantBuilds:  1,
			wantMissing: []string{`-ldflags="-s -w"`},
		},
		{
			name:        "only -ldflags",
			script:      `go build -ldflags="-s -w" -o "$out" ./cmd/yomihon`,
			wantBuilds:  1,
			wantMissing: []string{"-trimpath"},
		},
		{
			name:        "-ldflags that strips only one of the two",
			script:      `go build -trimpath -ldflags="-s" ./cmd/yomihon`,
			wantBuilds:  1,
			wantMissing: []string{`-ldflags="-s -w"`},
		},
		{
			name:        "-trimpath switched off",
			script:      `go build -trimpath=false -ldflags="-s -w" ./cmd/yomihon`,
			wantBuilds:  1,
			wantMissing: []string{"-trimpath"},
		},
		{
			name:        "the last -ldflags wins, as it does for go",
			script:      `go build -trimpath -ldflags="-s -w" -ldflags="-X main.version=v1" ./cmd/yomihon`,
			wantBuilds:  1,
			wantMissing: []string{`-ldflags="-s -w"`},
		},
		{
			name:        "flags that belong to the next command",
			script:      `go build ./cmd/yomihon && go vet -trimpath -ldflags="-s -w" ./...`,
			wantBuilds:  1,
			wantMissing: []string{"-trimpath", `-ldflags="-s -w"`},
		},
		{
			name:       "words that only mention a build",
			script:     "# go build -o out ./cmd/yomihon\necho \"go build -o out\"\ngo test ./...",
			wantBuilds: 0,
		},
		{
			name:       "no build at all",
			script:     "go vet ./...",
			wantBuilds: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			builds := goBuilds(tt.script)
			if len(builds) != tt.wantBuilds {
				t.Fatalf("goBuilds(%q) found %d build(s), want %d", tt.script, len(builds), tt.wantBuilds)
			}
			if len(builds) == 0 {
				return
			}
			got := missingReleaseFlags(builds[0])
			if !slices.Equal(got, tt.wantMissing) {
				t.Errorf("missingReleaseFlags(%q) = %q, want %q", tt.script, got, tt.wantMissing)
			}
		})
	}
}

// releaseBuild is one `go build` the release workflow runs, with where it runs.
type releaseBuild struct {
	where string
	args  []string
}

// releaseBuilds reads every `go build` out of the run scripts of every job in
// the release workflow, in the order the file declares its jobs and steps.
func releaseBuilds(t *testing.T) []releaseBuild {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(releaseWorkflow))) // #nosec G304 -- a fixed path under the repository root
	if err != nil {
		t.Fatalf("read %s: %v", releaseWorkflow, err)
	}
	var workflow struct {
		Jobs yaml.Node `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		t.Fatalf("parse %s: %v", releaseWorkflow, err)
	}
	type step struct {
		Name string `yaml:"name"`
		Run  string `yaml:"run"`
	}
	type job struct {
		Steps []step `yaml:"steps"`
	}

	// A mapping node keeps the order the file wrote its jobs in, which a Go map
	// would not.
	var found []releaseBuild
	for i := 0; i+1 < len(workflow.Jobs.Content); i += 2 {
		id := workflow.Jobs.Content[i].Value
		var j job
		if err := workflow.Jobs.Content[i+1].Decode(&j); err != nil {
			t.Fatalf("parse job %q of %s: %v", id, releaseWorkflow, err)
		}
		for n, s := range j.Steps {
			label := s.Name
			if label == "" {
				label = "step " + strconv.Itoa(n+1)
			}
			for _, args := range goBuilds(s.Run) {
				found = append(found, releaseBuild{where: "job " + id + ", step " + label, args: args})
			}
		}
	}
	return found
}

// goBuilds returns the arguments of each `go build` a shell script runs, up to
// the operator that ends the command. A line continued with a backslash is one
// line, a comment is not a command, and a quoted string is one word, so text
// that only mentions a build is not read as one.
func goBuilds(script string) [][]string {
	var builds [][]string
	for line := range strings.SplitSeq(strings.ReplaceAll(script, "\\\n", " "), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		words := shellWords(line)
		for i := 0; i+1 < len(words); i++ {
			if words[i] != "go" || words[i+1] != "build" {
				continue
			}
			args := words[i+2:]
			if end := slices.IndexFunc(args, isShellOperator); end >= 0 {
				args = args[:end]
			}
			builds = append(builds, args)
		}
	}
	return builds
}

func isShellOperator(word string) bool {
	switch word {
	case ";", "&&", "||", "|", "&":
		return true
	}
	return false
}

// shellWords splits a line the way a shell does for the purposes above:
// whitespace separates words, single and double quotes group and are removed,
// and a backslash outside single quotes escapes the next character.
func shellWords(line string) []string {
	var (
		words  []string
		word   strings.Builder
		inWord bool
		quote  rune
		escape bool
	)
	for _, r := range line {
		switch {
		case escape:
			word.WriteRune(r)
			escape = false
		case r == '\\' && quote != '\'':
			escape, inWord = true, true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				word.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote, inWord = r, true
		case r == ' ' || r == '\t':
			if inWord {
				words = append(words, word.String())
				word.Reset()
				inWord = false
			}
		default:
			word.WriteRune(r)
			inWord = true
		}
	}
	if inWord {
		words = append(words, word.String())
	}
	return words
}

// missingReleaseFlags names, in the spelling the workflow uses, each flag the
// release build is required to carry and does not. -ldflags counts only when
// its value strips both the symbol table (-s) and the DWARF data (-w); when the
// flag is given twice the last one is the one go uses.
func missingReleaseFlags(args []string) []string {
	var (
		trimmed bool
		ldflags []string
	)
	for i, arg := range args {
		switch {
		case arg == "-trimpath" || arg == "-trimpath=true":
			trimmed = true
		case arg == "-trimpath=false":
			trimmed = false
		case strings.HasPrefix(arg, "-ldflags="):
			ldflags = strings.Fields(strings.TrimPrefix(arg, "-ldflags="))
		case arg == "-ldflags" && i+1 < len(args):
			ldflags = strings.Fields(args[i+1])
		}
	}

	var missing []string
	if !trimmed {
		missing = append(missing, "-trimpath")
	}
	if !slices.Contains(ldflags, "-s") || !slices.Contains(ldflags, "-w") {
		missing = append(missing, `-ldflags="-s -w"`)
	}
	return missing
}
